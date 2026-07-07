package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	rqlite "github.com/rqlite/gorqlite"

	"github.com/lynnyq/bdopsflow/scheduler/internal/config"
	"github.com/lynnyq/bdopsflow/scheduler/internal/cron"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource"
	"github.com/lynnyq/bdopsflow/scheduler/internal/grpcserver"
	"github.com/lynnyq/bdopsflow/scheduler/internal/logger"
	"github.com/lynnyq/bdopsflow/scheduler/internal/metrics"
	"github.com/lynnyq/bdopsflow/scheduler/internal/middleware"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
	"github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/election"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/rsautil"
)

type RQLiteClient struct {
	addrs    []string
	user     string
	password string
	useTLS   bool
	mu       sync.RWMutex
	current  *rqlite.Connection
	index    int
}

func NewRQLiteClient(addrs []string, user, password string, useTLS bool) *RQLiteClient {
	return &RQLiteClient{
		addrs:    addrs,
		user:     user,
		password: password,
		useTLS:   useTLS,
		index:    0,
	}
}

func (c *RQLiteClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < len(c.addrs); i++ {
		addr := c.addrs[(c.index+i)%len(c.addrs)]
		connURL := c.buildURL(addr)

		slog.Info("attempting to connect to rqlite", "addr", addr, "index", i)
		conn, err := rqlite.Open(connURL)
		if err != nil {
			slog.Warn("failed to connect to rqlite node", "addr", addr, "error", err)
			continue
		}

		stmt := rqlite.ParameterizedStatement{
			Query: "SELECT 1",
		}
		qr, err := conn.QueryOneParameterized(stmt)
		if err != nil || qr.Err != nil {
			slog.Warn("rqlite node test query failed", "addr", addr, "error", err)
			conn.Close()
			continue
		}

		c.current = conn
		c.index = (c.index + i) % len(c.addrs)
		slog.Info("successfully connected to rqlite", "addr", addr)
		return nil
	}

	return fmt.Errorf("failed to connect to any rqlite node")
}

func (c *RQLiteClient) Connection() *rqlite.Connection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

func (c *RQLiteClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current != nil {
		c.current.Close()
	}
}

func (c *RQLiteClient) buildURL(addr string) string {
	u, err := url.Parse(addr)
	if err != nil {
		slog.Warn("failed to parse rqlite address, using as is", "addr", addr)
		return addr
	}

	if c.useTLS {
		u.Scheme = "https"
	}

	if c.user != "" && c.password != "" {
		u.User = url.UserPassword(c.user, c.password)
	}

	return u.String()
}

type App struct {
	cfg          *config.Config
	db           *rqlite.Connection
	logDB        database.DB
	redisClient  *redis.Client
	rqliteClient *RQLiteClient
	rsaUtil      *rsautil.RSAUtil
	ssoRsaUtil   *rsautil.RSAUtil
	nodeID       string

	schedulerService      *service.SchedulerService
	permissionService     *service.PermissionService
	authService           *service.AuthService
	userAdminService      *service.UserAdminService
	roleAdminService      *service.RoleAdminService
	domainAdminService    *service.DomainAdminService
	executorDomainService *service.ExecutorDomainService
	auditLogService       *service.AuditLogService
	apiTokenService       *service.APITokenService
	webhookSvc            *service.WebhookService
	instancePermSvc       *service.InstancePermissionService

	dsCrypto            *datasource.Crypto
	dsManager           *datasource.Manager
	dsService           *datasource.DatasourceService
	dsCacheService      *datasource.CacheService
	dsConcurrentService *datasource.ConcurrentService

	sysConfigService *system_config.Service  // 全局系统配置服务

	apiTestSvc   *service.ApiTestService
	httpExecutor *service.HTTPExecutor
	grpcExecutor *service.GRPCExecutor
	protoSvc     *service.ProtoService
	certSvc      *service.CertificateService

	leaderElection *election.LeaderElection
	grpcSrv        *grpcserver.Server
	cronScheduler  *cron.CronScheduler

	httpSrv    *http.Server
	mainCancel context.CancelFunc
	wg         sync.WaitGroup
}

func NewApp(cfg *config.Config) *App {
	app := &App{cfg: cfg}

	if err := cfg.Validate(); err != nil {
		slog.Error("config validation failed", "error", err)
		os.Exit(1)
	}

	middleware.InitJWT(cfg.JWTSecret, cfg.JWTExpiry, cfg.JWTRefreshExpiry)

	rsaUtil, err := rsautil.NewFromConfig(cfg.RSAPublicKey, cfg.RSAPrivateKey)
	if err != nil {
		slog.Error("failed to initialize RSA", "error", err)
		os.Exit(1)
	}
	app.rsaUtil = rsaUtil

	var ssoRsaUtil *rsautil.RSAUtil
	if cfg.SSOEnabled && cfg.SSOPublicKey != "" {
		ssoRsaUtil, err = rsautil.NewFromConfig(cfg.SSOPublicKey, "")
		if err != nil {
			slog.Error("failed to initialize SSO RSA", "error", err)
			os.Exit(1)
		}
		slog.Info("SSO login enabled", "url", cfg.SSOUrl)
	} else {
		slog.Info("SSO login disabled")
	}
	app.ssoRsaUtil = ssoRsaUtil

	if cfg.RQLitePass != "" {
		decrypted, err := rsaUtil.DecryptConfigPassword(cfg.RQLitePass)
		if err != nil {
			slog.Error("failed to decrypt rqlite password", "error", err)
			os.Exit(1)
		}
		cfg.RQLitePass = decrypted
	}
	if cfg.RedisPassword != "" {
		decrypted, err := rsaUtil.DecryptConfigPassword(cfg.RedisPassword)
		if err != nil {
			slog.Error("failed to decrypt redis password", "error", err)
			os.Exit(1)
		}
		cfg.RedisPassword = decrypted
	}
	if cfg.RedisSentinelPassword != "" {
		decrypted, err := rsaUtil.DecryptConfigPassword(cfg.RedisSentinelPassword)
		if err != nil {
			slog.Error("failed to decrypt redis sentinel password", "error", err)
			os.Exit(1)
		}
		cfg.RedisSentinelPassword = decrypted
	}

	nodeID := cfg.NodeID
	if nodeID == "" {
		nodeID = uuid.New().String()
		slog.Info("generated node ID", "node_id", nodeID)
	} else {
		slog.Info("using configured node ID", "node_id", nodeID)
	}
	app.nodeID = nodeID

	slog.Info("scheduler starting",
		"http_port", cfg.HTTPPort,
		"grpc_port", cfg.GRPCPort,
		"config_file", cfg.ConfigFile,
		"redis_mode", cfg.RedisMode,
		"rqlite_addrs", cfg.RQLiteAddrs,
		"rqlite_tls", cfg.RQLiteTLS,
		"rqlite_has_auth", cfg.RQLiteUser != "" && cfg.RQLitePass != "",
		"node_id", nodeID,
	)

	var redisClient *redis.Client
	if cfg.RedisMode == "sentinel" {
		slog.Info("using Redis Sentinel mode",
			"master_name", cfg.RedisMaster,
			"sentinel_addrs", cfg.RedisSentinelAddrs,
		)
		redisClient = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       cfg.RedisMaster,
			SentinelAddrs:    cfg.RedisSentinelAddrs,
			SentinelPassword: cfg.RedisSentinelPassword,
			Password:         cfg.RedisPassword,
			DB:               cfg.RedisDB,
		})
	} else {
		slog.Info("using Redis single mode", "addr", cfg.RedisAddr)
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
	}
	app.redisClient = redisClient

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to Redis")

	rqliteClient := NewRQLiteClient(cfg.RQLiteAddrs, cfg.RQLiteUser, cfg.RQLitePass, cfg.RQLiteTLS)
	if err := rqliteClient.Connect(ctx); err != nil {
		slog.Error("failed to connect to rqlite", "error", err)
		os.Exit(1)
	}
	app.rqliteClient = rqliteClient

	db := rqliteClient.Connection()
	app.db = db
	logDB := database.NewLogDB(db)
	app.logDB = logDB

	leaderHTTPAddr := resolveAdvertiseAddr(cfg.AdvertiseAddr, cfg.HTTPPort)
	leaderElection := election.NewLeaderElection(redisClient, "bdopsflow:leader", nodeID, leaderHTTPAddr, 15*time.Second)
	app.leaderElection = leaderElection

	schedulerService := service.NewSchedulerService(logDB, redisClient)
	app.schedulerService = schedulerService

	permissionService := service.NewPermissionService(logDB, redisClient)
	app.permissionService = permissionService

	authService := service.NewAuthService(logDB)
	app.authService = authService

	userAdminService := service.NewUserAdminService(logDB, permissionService, rsaUtil)
	app.userAdminService = userAdminService

	roleAdminService := service.NewRoleAdminService(logDB, permissionService)
	app.roleAdminService = roleAdminService

	domainAdminService := service.NewDomainAdminService(logDB, permissionService)
	app.domainAdminService = domainAdminService

	executorDomainService := service.NewExecutorDomainService(logDB)
	app.executorDomainService = executorDomainService

	// 初始化全局系统配置服务（支持热更新）。
	// 必须在依赖配置服务的下游组件（AuditLogService、数据源 Manager 等）之前初始化。
	sysConfigService := system_config.InitGlobalService(logDB)
	sysConfigService.StartReloadTicker(5 * time.Minute)
	app.sysConfigService = sysConfigService

	auditLogService := service.NewAuditLogService(logDB, sysConfigService)
	app.auditLogService = auditLogService

	apiTokenService := service.NewAPITokenService(logDB, rsaUtil, permissionService)
	app.apiTokenService = apiTokenService

	schedulerService.ExecutorDomainService = executorDomainService

	dsCrypto, err := datasource.NewCrypto(cfg.DatasourceCrypto.EncryptionKey)
	if err != nil {
		slog.Error("failed to initialize datasource crypto", "error", err)
		os.Exit(1)
	}
	app.dsCrypto = dsCrypto

	dsManager := datasource.NewManager(dsCrypto, sysConfigService)
	app.dsManager = dsManager

	// 注册 Manager 为全局配置观察者，连接池配置变更时动态更新
	sysConfigService.RegisterObserver(dsManager)

	dsService := datasource.NewDatasourceService(logDB, dsCrypto, dsManager)
	app.dsService = dsService

	// 使用全局系统配置服务创建缓存和并发服务（支持热更新）
	dsCacheService := datasource.NewCacheService(redisClient, sysConfigService)
	app.dsCacheService = dsCacheService

	dsConcurrentService := datasource.NewConcurrentService(redisClient, sysConfigService)
	app.dsConcurrentService = dsConcurrentService

	// 启动并发控制计数器校准机制（每5分钟校准一次）
	// 校准函数从 QueryRegistry 获取实际运行的查询数量
	// 注意：这里暂时传入 nil，实际集成需要等待 QueryRegistry 初始化后设置
	// 后续在 setupRoutes 或 QueryHandler 初始化时完成集成
	slog.Info("concurrent service calibration will be started when query registry is available")

	schedulerService.StartCleanupRoutine()

	// 审计日志自动清理 goroutine：启动时立即执行一次，之后每 24 小时执行一次。
	// goroutine 内加 recover 防止 panic 导致清理永久停止；
	// 每次清理使用 30 秒超时 context，避免 DB 卡住时长时间阻塞。
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("audit log cleanup goroutine panicked", "recover", r)
			}
		}()

		runAuditCleanup := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			retentionDays := auditLogService.GetRetentionDays()
			deleted, err := auditLogService.CleanExpired(ctx, retentionDays)
			if err != nil {
				slog.Error("failed to clean expired audit logs", "error", err)
			} else if deleted > 0 {
				slog.Info("cleaned expired audit logs", "deleted_count", deleted, "retention_days", retentionDays)
			}
		}

		// 启动时立即执行一次，避免旧数据累积到次日
		runAuditCleanup()

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			runAuditCleanup()
		}
	}()

	webhookSvc := service.NewWebhookService(logDB)
	app.webhookSvc = webhookSvc
	schedulerService.SetWebhookService(webhookSvc)

	instancePermSvc := service.NewInstancePermissionService(logDB, permissionService)
	app.instancePermSvc = instancePermSvc

	// 接口测试模块服务初始化
	apiTestSvc := service.NewApiTestService(logDB)
	app.apiTestSvc = apiTestSvc

	httpExecutor := service.NewHTTPExecutor(app.sysConfigService)
	app.httpExecutor = httpExecutor

	grpcExecutor := service.NewGRPCExecutor(app.sysConfigService)
	app.grpcExecutor = grpcExecutor

	protoSvc := service.NewProtoService(logDB)
	app.protoSvc = protoSvc

	certSvc := service.NewCertificateService(logDB, rsaUtil)
	app.certSvc = certSvc

	grpcSrv := grpcserver.NewServer(cfg.GRPCPort, schedulerService)
	grpcSrv.SetNodeId(nodeID)
	schedulerService.SetConnectivityChecker(grpcSrv)
	schedulerService.SetLeaderAddrResolver(leaderElection)
	schedulerService.SetCancelNotifier(grpcSrv)
	app.grpcSrv = grpcSrv

	cronScheduler := cron.NewCronScheduler(schedulerService, redisClient)
	schedulerService.SetCronScheduler(cronScheduler)
	app.cronScheduler = cronScheduler

	if err := cronScheduler.Start(); err != nil {
		slog.Error("failed to start cron scheduler", "error", err)
		os.Exit(1)
	}

	leaderElection.OnAcquire(func() {
		slog.Info("this node became the leader", "node_id", nodeID)
		grpcSrv.MarkAsNewLeader()
		grpcSrv.SetLeader(true)
		schedulerService.SetLeader(true)
		cronScheduler.OnBecomeLeader()
		metrics.SetLeaderStatus(nodeID, true)
	})
	leaderElection.OnRelease(func() {
		slog.Info("this node lost leadership", "node_id", nodeID)
		grpcSrv.SetLeader(false)
		schedulerService.SetLeader(false)
		cronScheduler.OnLoseLeader()
		metrics.SetLeaderStatus(nodeID, false)
	})

	mainCtx, mainCancel := context.WithCancel(context.Background())
	leaderElection.Start(mainCtx)
	app.mainCancel = mainCancel

	// 初始化 Prometheus leader 状态指标（默认为 follower）
	metrics.SetLeaderStatus(nodeID, false)

	gin.SetMode(gin.ReleaseMode)

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("regexp", func(fl validator.FieldLevel) bool {
			param := fl.Param()
			if param == "" {
				return true
			}
			re, err := regexp.Compile("^" + param + "$")
			if err != nil {
				return false
			}
			return re.MatchString(fl.Field().String())
		})
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(slogGinLogger())
	router.Use(corsMiddleware(cfg.CORSAllowOrigins))

	setupRoutes(router, app)

	app.httpSrv = &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler: router,
	}

	return app
}

func (a *App) Run() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.grpcSrv.Start(); err != nil {
			slog.Error("failed to start gRPC server", "error", err)
			os.Exit(1)
		}
	}()

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		slog.Info("HTTP server listening", "port", a.cfg.HTTPPort)
		if err := a.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to start HTTP server", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("scheduler started", "http_port", a.cfg.HTTPPort, "grpc_port", a.cfg.GRPCPort)
}

func (a *App) Shutdown() {
	slog.Info("shutting down servers")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a.httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	a.grpcSrv.Stop()

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("servers exited cleanly")
	case <-time.After(5 * time.Second):
		slog.Info("servers force exited")
	}

	a.schedulerService.StopCleanupRoutine()
	a.cronScheduler.Stop()
	a.dsManager.Close()
	a.rqliteClient.Close()
	a.mainCancel()
}

func (a *App) ReloadConfig() error {
	slog.Info("initiating config reload")

	if err := a.cfg.Reload(); err != nil {
		slog.Error("failed to reload config", "error", err)
		return err
	}

	if err := logger.ReopenLogFile(); err != nil {
		slog.Error("failed to reopen log file", "error", err)
		return err
	}

	slog.Info("config reload completed successfully")
	return nil
}

func corsMiddleware(allowOrigins []string) gin.HandlerFunc {
	originsMap := make(map[string]bool)
	for _, o := range allowOrigins {
		originsMap[o] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if len(originsMap) == 0 {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		} else if originsMap[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func slogGinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		elapsed := time.Since(start)
		status := c.Writer.Status()

		attrs := []any{
			"status", status,
			"method", c.Request.Method,
			"path", path,
			"latency", elapsed.String(),
			"ip", c.ClientIP(),
		}
		if query != "" {
			attrs = append(attrs, "query", query)
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		msg := c.Request.Method + " " + path
		switch {
		case status >= 500:
			slog.Error(msg, attrs...)
		case status >= 400:
			slog.Warn(msg, attrs...)
		default:
			slog.Info(msg, attrs...)
		}
	}
}

func resolveAdvertiseAddr(advertiseAddr, httpPort string) string {
	if advertiseAddr == "" {
		addr := fmt.Sprintf("127.0.0.1:%s", httpPort)
		slog.Warn("app.advertise_addr not configured, using 127.0.0.1 as leader HTTP address; set app.advertise_addr to the externally reachable address for multi-node deployments",
			"effective_addr", addr,
		)
		return addr
	}

	if !strings.Contains(advertiseAddr, ":") {
		addr := fmt.Sprintf("%s:%s", advertiseAddr, httpPort)
		slog.Info("app.advertise_addr has no port, auto-appending http_port",
			"advertise_addr", advertiseAddr,
			"http_port", httpPort,
			"effective_addr", addr,
		)
		return addr
	}

	_, port, err := net.SplitHostPort(advertiseAddr)
	if err != nil {
		slog.Warn("app.advertise_addr format invalid, using as-is",
			"advertise_addr", advertiseAddr,
			"error", err,
		)
		return advertiseAddr
	}

	if port != httpPort {
		slog.Warn("app.advertise_addr port differs from app.http_port, ensure other nodes can reach the scheduler HTTP API at this address",
			"advertise_addr", advertiseAddr,
			"http_port", httpPort,
			"hint", "advertise_addr should point to the scheduler's direct HTTP port, not a reverse proxy port (e.g. nginx)",
		)
	}

	slog.Info("using configured advertise_addr", "advertise_addr", advertiseAddr)
	return advertiseAddr
}

func printHelp() {
	fmt.Fprint(os.Stderr, `BDopsFlow Scheduler - 任务调度和执行引擎

用法:
  ./scheduler [命令] [选项]

命令:
  keygen                     生成 RSA 密钥对
  encrypt-password           加密密码
  decrypt-password           解密密码

选项:
  -config string             配置文件路径 (默认: 当前目录的 config.yaml)
  -advertise-addr string     集群部署时节点对外可达的 HTTP 地址 (格式: host:port)
                             单节点部署可留空，多节点部署必须配置
                             等同于配置项 app.advertise_addr，优先级高于配置文件
  -h, --help                 显示帮助信息

集群部署说明:
  多节点部署时，每个调度中心节点必须配置 advertise_addr，指定其他节点可访问的
  HTTP 地址（如 10.0.1.5:8080）。未配置时默认使用 127.0.0.1:<http_port>，
  导致非主节点转发请求到 127.0.0.1 而非主节点，请求会失败。

  advertise_addr 必须指向调度器直接监听的 HTTP 端口，而非 Nginx 等反向代理端口。
  若仅填写 IP 或主机名未指定端口，系统将自动补全 http_port 配置的端口号。
  示例：-advertise-addr 10.0.1.5 等同于 -advertise-addr 10.0.1.5:8080

示例:
  ./scheduler                  启动调度器
  ./scheduler -config my.yml   使用指定配置启动
  ./scheduler -advertise-addr 10.0.1.5:8080  指定对外宣告地址
  ./scheduler keygen           生成密钥对
  ./scheduler encrypt-password --config config.yml --password mypass
`)
}

func runKeygen() {
	publicKeyB64, privateKeyB64, err := rsautil.GenerateKeyPair()
	if err != nil {
		fmt.Fprintf(os.Stderr, "生成密钥对失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("rsa:")
	fmt.Printf("  public_key: \"%s\"\n", publicKeyB64)
	fmt.Printf("  private_key: \"%s\"\n", privateKeyB64)
}

func runEncryptPassword() {
	configFile := ""
	password := ""

	fs := flag.NewFlagSet("encrypt-password", flag.ExitOnError)
	fs.StringVar(&configFile, "config", "", "path to config file")
	fs.StringVar(&password, "password", "", "password to encrypt")
	fs.Parse(os.Args[2:])

	if configFile == "" || password == "" {
		fmt.Fprintln(os.Stderr, "用法: scheduler encrypt-password --config <config_file> --password <password>")
		os.Exit(1)
	}

	cfg := config.Load(configFile)
	if cfg.RSAPublicKey == "" {
		fmt.Fprintln(os.Stderr, "配置文件中未找到RSA公钥 (rsa.public_key)")
		os.Exit(1)
	}

	rsaUtil, err := rsautil.NewFromConfig(cfg.RSAPublicKey, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化RSA失败: %v\n", err)
		os.Exit(1)
	}

	ciphertext, err := rsaUtil.Encrypt(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加密失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("RSA_ENCRYPTED:%s\n", ciphertext)
}

func runDecryptPassword() {
	configFile := ""
	ciphertext := ""

	fs := flag.NewFlagSet("decrypt-password", flag.ExitOnError)
	fs.StringVar(&configFile, "config", "", "path to config file")
	fs.StringVar(&ciphertext, "ciphertext", "", "encrypted ciphertext (with RSA_ENCRYPTED: prefix)")
	fs.Parse(os.Args[2:])

	if configFile == "" || ciphertext == "" {
		fmt.Fprintln(os.Stderr, "用法: scheduler decrypt-password --config <config_file> --ciphertext <ciphertext>")
		os.Exit(1)
	}

	cfg := config.Load(configFile)
	if cfg.RSAPrivateKey == "" {
		fmt.Fprintln(os.Stderr, "配置文件中未找到RSA私钥 (rsa.private_key)")
		os.Exit(1)
	}

	rsaUtil, err := rsautil.NewFromConfig(cfg.RSAPublicKey, cfg.RSAPrivateKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化RSA失败: %v\n", err)
		os.Exit(1)
	}

	ciphertext = strings.TrimPrefix(ciphertext, "RSA_ENCRYPTED:")
	plaintext, err := rsaUtil.Decrypt(ciphertext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解密失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(plaintext)
}
