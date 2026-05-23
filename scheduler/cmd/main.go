package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	rqlite "github.com/rqlite/gorqlite"
	"github.com/go-playground/validator/v10"

	"github.com/lynnyq/bdopsflow/scheduler/internal/config"
	"github.com/lynnyq/bdopsflow/scheduler/internal/cron"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource"
	"github.com/lynnyq/bdopsflow/scheduler/internal/grpcserver"
	"github.com/lynnyq/bdopsflow/scheduler/internal/handler"
	"github.com/lynnyq/bdopsflow/scheduler/internal/logger"
	"github.com/lynnyq/bdopsflow/scheduler/internal/middleware"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/election"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/rsautil"
	"github.com/lynnyq/bdopsflow/scheduler/web"
)

// RQLiteClient rqlite 多节点客户端
type RQLiteClient struct {
	addrs    []string
	user     string
	password string
	useTLS   bool
	mu       sync.RWMutex
	current  *rqlite.Connection
	index    int
}

// NewRQLiteClient 创建 rqlite 客户端
func NewRQLiteClient(addrs []string, user, password string, useTLS bool) *RQLiteClient {
	return &RQLiteClient{
		addrs:    addrs,
		user:     user,
		password: password,
		useTLS:   useTLS,
		index:    0,
	}
}

// Connect 连接到 rqlite 节点
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

		// 测试连接 - 执行一个简单查询
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

// Connection 获取当前连接
func (c *RQLiteClient) Connection() *rqlite.Connection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

// Close 关闭连接
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

	// 设置 TLS
	if c.useTLS {
		u.Scheme = "https"
	}

	// 设置认证
	if c.user != "" && c.password != "" {
		u.User = url.UserPassword(c.user, c.password)
	}

	return u.String()
}

func printHelp() {
	fmt.Fprintln(os.Stderr, `BDopsFlow Scheduler - 任务调度和执行引擎

用法:
  scheduler [命令] [选项]

命令:
  keygen                     生成 RSA 密钥对
  encrypt-password           加密密码
  decrypt-password           解密密码

选项:
  -config string             配置文件路径 (默认: 当前目录的 config.yaml)
  -h, --help                 显示帮助信息

示例:
  scheduler                  启动调度器
  scheduler -config my.yml   使用指定配置启动
  scheduler keygen           生成密钥对
  scheduler encrypt-password --config config.yml --password mypass
`)
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "keygen":
			runKeygen()
			return
		case "encrypt-password":
			runEncryptPassword()
			return
		case "decrypt-password":
			runDecryptPassword()
			return
		case "-h", "--help", "help":
			printHelp()
			return
		}
	}

	flag.Usage = printHelp
	configFile := flag.String("config", "", "path to config file (default: config.yaml in current directory)")
	flag.Parse()

	logger.Init()

	cfg := config.Load(*configFile)

	rsaUtil, err := rsautil.NewFromConfig(cfg.RSAPublicKey, cfg.RSAPrivateKey)
	if err != nil {
		slog.Error("failed to initialize RSA", "error", err)
		os.Exit(1)
	}

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

	// 生成节点ID（如果配置中没有提供）
	nodeID := cfg.NodeID
	if nodeID == "" {
		nodeID = uuid.New().String()
		slog.Info("generated node ID", "node_id", nodeID)
	} else {
		slog.Info("using configured node ID", "node_id", nodeID)
	}

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

	// 创建 Redis 客户端
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to Redis")

	// 创建 rqlite 多节点客户端
	rqliteClient := NewRQLiteClient(cfg.RQLiteAddrs, cfg.RQLiteUser, cfg.RQLitePass, cfg.RQLiteTLS)
	if err := rqliteClient.Connect(ctx); err != nil {
		slog.Error("failed to connect to rqlite", "error", err)
		os.Exit(1)
	}
	defer rqliteClient.Close()

	db := rqliteClient.Connection()

	// 初始化主节点选举（先声明变量）
	leaderElection := election.NewLeaderElection(redisClient, "bdopsflow:leader", nodeID, 15*time.Second)

	schedulerService := service.NewSchedulerService(db, redisClient)

	permissionService := service.NewPermissionService(db, redisClient)
	userAdminService := service.NewUserAdminService(db, permissionService, rsaUtil)
	roleAdminService := service.NewRoleAdminService(db, permissionService)
	domainAdminService := service.NewDomainAdminService(db)
	executorDomainService := service.NewExecutorDomainService(db)
	auditLogService := service.NewAuditLogService(db)

	schedulerService.ExecutorDomainService = executorDomainService

	dsCrypto, err := datasource.NewCrypto(cfg.DatasourceCrypto.EncryptionKey)
	if err != nil {
		slog.Error("failed to initialize datasource crypto", "error", err)
		os.Exit(1)
	}
	dsConfigService := datasource.NewConfigService(db)
	dsConfigService.StartReloadTicker(5 * time.Minute)
	dsManager := datasource.NewManager(dsCrypto, dsConfigService)
	defer dsManager.Close()
	dsService := datasource.NewDatasourceService(db, dsCrypto, dsConfigService, dsManager)
	dsCacheService := datasource.NewCacheService(redisClient, dsConfigService)
	dsConcurrentService := datasource.NewConcurrentService(redisClient, dsConfigService)

	schedulerService.StartCleanupRoutine()

	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			retentionDays := auditLogService.GetRetentionDays()
			deleted, err := auditLogService.CleanExpired(context.Background(), retentionDays)
			if err != nil {
				slog.Error("failed to clean expired audit logs", "error", err)
			} else if deleted > 0 {
				slog.Info("cleaned expired audit logs", "deleted_count", deleted, "retention_days", retentionDays)
			}
		}
	}()

	webhookSvc := service.NewWebhookService(db)
	schedulerService.SetWebhookService(webhookSvc)

	grpcSrv := grpcserver.NewServer(cfg.GRPCPort, schedulerService)
	grpcSrv.SetNodeId(nodeID) // 设置节点 ID

	cronScheduler := cron.NewCronScheduler(schedulerService, redisClient)
	schedulerService.SetCronScheduler(cronScheduler)

	if err := cronScheduler.Start(); err != nil {
		slog.Error("failed to start cron scheduler", "error", err)
		os.Exit(1)
	}
	defer cronScheduler.Stop()
	defer schedulerService.StopCleanupRoutine()

	// 设置主节点选举回调
	leaderElection.OnAcquire(func() {
		slog.Info("this node became the leader", "node_id", nodeID)
		// 标记 gRPC 服务为新 leader，需要执行器同步任务
		grpcSrv.MarkAsNewLeader()
		grpcSrv.SetLeader(true)
		cronScheduler.OnBecomeLeader()
	})
	leaderElection.OnRelease(func() {
		slog.Info("this node lost leadership", "node_id", nodeID)
		grpcSrv.SetLeader(false)
		cronScheduler.OnLoseLeader()
	})

	// 创建一个可取消的context
	mainCtx, mainCancel := context.WithCancel(context.Background())
	leaderElection.Start(mainCtx)
	defer mainCancel()

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

	router := gin.Default()

	router.Use(corsMiddleware())

	router.GET("/health", func(c *gin.Context) {
		result := schedulerService.HealthCheck(c.Request.Context())
		// 添加节点ID和主节点状态到健康检查结果
		healthData := map[string]interface{}{
			"status":    result.Status,
			"timestamp": result.Timestamp,
			"node_id":   nodeID,
			"is_leader": leaderElection.IsLeader(),
		}
		if result.Components != nil {
			healthData["components"] = result.Components
		}
		if result.Status == "healthy" {
			c.JSON(http.StatusOK, healthData)
		} else {
			c.JSON(http.StatusServiceUnavailable, healthData)
		}
	})

	authHandler := handler.NewAuthHandler(db, permissionService, rsaUtil, cfg.SSOEnabled, cfg.SSOUrl, ssoRsaUtil, cfg.SSOTimeout)
	userAdminHandler := handler.NewUserAdminHandler(userAdminService)
	router.POST("/api/auth/login", middleware.AuditMiddleware(auditLogService), authHandler.Login)
	router.POST("/api/auth/sso-login", middleware.AuditMiddleware(auditLogService), authHandler.SSOLogin)
	router.POST("/api/auth/register", middleware.AuditMiddleware(auditLogService), authHandler.Register)
	router.GET("/api/auth/public-key", authHandler.GetPublicKey)

	protected := router.Group("/api")
	protected.Use(middleware.JWTAuthMiddleware())
	protected.Use(middleware.AuditMiddleware(auditLogService))
	{
		protected.GET("/auth/current", authHandler.GetCurrentUser)
		protected.PUT("/auth/profile", userAdminHandler.UpdateCurrentUser)
		protected.POST("/auth/change-password", userAdminHandler.ChangePassword)

		taskHandler := handler.NewTaskHandler(schedulerService)
		tasks := protected.Group("/tasks")
		{
			tasks.GET("", taskHandler.List)
			tasks.POST("", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), taskHandler.Create)
			tasks.GET("/:id", taskHandler.Get)
			tasks.PUT("/:id", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), taskHandler.Update)
			tasks.DELETE("/:id", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), taskHandler.Delete)
			tasks.POST("/:id/trigger", middleware.RBACMiddleware("admin", "system_admin", "domain_admin", "user"), taskHandler.Trigger)
			tasks.GET("/:id/executions", taskHandler.Executions)
			tasks.GET("/executions/:execution_id/logs", taskHandler.ExecutionLogs)
		}

		workflowHandler := handler.NewWorkflowHandler(schedulerService)
		workflows := protected.Group("/workflows")
		{
			workflows.GET("", workflowHandler.List)
			workflows.POST("", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), workflowHandler.Create)
			workflows.GET("/:id", workflowHandler.Get)
			workflows.PUT("/:id", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), workflowHandler.Update)
			workflows.DELETE("/:id", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), workflowHandler.Delete)
			workflows.POST("/:id/trigger", middleware.RBACMiddleware("admin", "system_admin", "domain_admin", "user"), workflowHandler.TriggerWorkflow)
			workflows.GET("/:id/executions", workflowHandler.GetWorkflowExecutions)
			workflows.GET("/executions/:execution_id", workflowHandler.GetWorkflowExecution)
			workflows.GET("/executions/:execution_id/logs", workflowHandler.GetExecutionLogs)
		}

		executorHandler := handler.NewExecutorHandler(schedulerService)
		executorDomainHandler := handler.NewExecutorDomainHandler(executorDomainService, permissionService, userAdminService)
		executors := protected.Group("/executors")
		{
			executors.GET("", executorDomainHandler.GetExecutorsWithDomains)
			executors.GET("/:name", executorHandler.Get)
			executors.GET("/:name/domains", middleware.RequireSystemAdmin(permissionService), executorDomainHandler.GetExecutorDomains)
			executors.POST("/:name/domains", middleware.RequireSystemAdmin(permissionService), executorDomainHandler.AssignDomains)
			executors.DELETE("/:name/domains/:domain_id", middleware.RequireSystemAdmin(permissionService), executorDomainHandler.RemoveDomain)
			executors.GET("/:name/tasks", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), executorDomainHandler.GetAssignedTasks)
			executors.GET("/:name/can-delete", executorDomainHandler.CanDeleteExecutor)
			executors.POST("/:name/online", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), executorHandler.Online)
			executors.POST("/:name/offline", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), executorHandler.Offline)
			executors.PUT("/:name/capacity", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), executorHandler.UpdateCapacity)
			executors.DELETE("/:name", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), executorHandler.Delete)
		}

		logHandler := handler.NewLogHandler(schedulerService)
		logs := protected.Group("/logs")
		{
			logs.GET("", logHandler.List)
			logs.GET("/stats", logHandler.GetStats)
			logs.DELETE("/:id", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), logHandler.Delete)
			logs.POST("/batch-delete", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), logHandler.BatchDelete)
		}

		protected.GET("/logs/stream", taskHandler.StreamLogs)

		dashboardHandler := handler.NewDashboardHandler(schedulerService)
		dashboard := protected.Group("/dashboard")
		{
			dashboard.GET("/stats", dashboardHandler.GetStats)
			dashboard.GET("/trends", dashboardHandler.GetTrends)
			dashboard.GET("/scheduler/status", dashboardHandler.GetSchedulerStatus)
			dashboard.POST("/scheduler/pause", middleware.RequireSystemAdmin(permissionService), dashboardHandler.PauseScheduler)
			dashboard.POST("/scheduler/resume", middleware.RequireSystemAdmin(permissionService), dashboardHandler.ResumeScheduler)
			dashboard.GET("/health", dashboardHandler.HealthCheck)
		}

		admin := protected.Group("/admin")
		{
			permissionHandler := handler.NewPermissionHandler(permissionService)
			admin.GET("/permissions", middleware.RequireSystemAdmin(permissionService), permissionHandler.GetAllPermissions)

			admin.GET("/users", middleware.RequireSystemAdmin(permissionService), userAdminHandler.ListUsers)
			admin.GET("/users/by-domain", middleware.RequireAdminOrDomainAdmin(), userAdminHandler.ListUsersByDomain)
			admin.GET("/users/:id", middleware.RequireSystemAdmin(permissionService), userAdminHandler.GetUser)
			admin.POST("/users", middleware.RequireSystemAdmin(permissionService), userAdminHandler.CreateUser)
			admin.PUT("/users/:id", middleware.RequireAdminOrDomainAdmin(), userAdminHandler.UpdateUser)
			admin.DELETE("/users/:id", middleware.RequireSystemAdmin(permissionService), userAdminHandler.DeleteUser)
			admin.POST("/users/:id/roles", middleware.RequireSystemAdmin(permissionService), userAdminHandler.AssignUserRoles)
			admin.GET("/users/:id/roles", middleware.RequireSystemAdmin(permissionService), userAdminHandler.GetUserRoles)
			admin.POST("/users/:id/domains", middleware.RequireSystemAdmin(permissionService), userAdminHandler.AssignUserDomains)
			admin.POST("/users/:id/reset-password", middleware.RequireAdminOrDomainAdmin(), userAdminHandler.ResetUserPassword)

			roleAdminHandler := handler.NewRoleAdminHandler(roleAdminService)
			admin.GET("/roles", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.ListRoles)
			admin.GET("/roles/:id", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.GetRole)
			admin.POST("/roles", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.CreateRole)
			admin.PUT("/roles/:id", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.UpdateRole)
			admin.DELETE("/roles/:id", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.DeleteRole)
			admin.GET("/roles/:id/permissions", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.GetRolePermissions)
			admin.POST("/roles/:id/permissions", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.AssignPermissions)

			domainAdminHandler := handler.NewDomainAdminHandler(domainAdminService)
			admin.GET("/domains", middleware.RequireSystemAdmin(permissionService), domainAdminHandler.ListDomains)
			admin.GET("/domains/:id", middleware.RequireSystemAdmin(permissionService), domainAdminHandler.GetDomain)
			admin.POST("/domains", middleware.RequireSystemAdmin(permissionService), domainAdminHandler.CreateDomain)
			admin.PUT("/domains/:id", middleware.RequireSystemAdmin(permissionService), domainAdminHandler.UpdateDomain)
			admin.DELETE("/domains/:id", middleware.RequireSystemAdmin(permissionService), domainAdminHandler.DeleteDomain)

			systemConfigHandler := handler.NewSystemConfigHandler(dsConfigService)
			admin.GET("/system-config", middleware.RequireSystemAdmin(permissionService), systemConfigHandler.List)
			admin.PUT("/system-config/:key", middleware.RequireSystemAdmin(permissionService), systemConfigHandler.Update)

			auditLogHandler := handler.NewAuditLogHandler(auditLogService)
			admin.GET("/audit-logs", middleware.RequireSystemAdmin(permissionService), auditLogHandler.List)
			admin.GET("/audit-logs/stats", middleware.RequireSystemAdmin(permissionService), auditLogHandler.GetStats)
			admin.POST("/audit-logs/clean", middleware.RequireSystemAdmin(permissionService), auditLogHandler.CleanExpired)
			admin.GET("/audit-logs/retention", middleware.RequireSystemAdmin(permissionService), auditLogHandler.GetRetentionDays)
			admin.PUT("/audit-logs/retention", middleware.RequireSystemAdmin(permissionService), auditLogHandler.UpdateRetentionDays)
		}

		webhookHandler := handler.NewWebhookHandler(webhookSvc)
		webhooks := protected.Group("/webhooks")
		{
			webhooks.GET("", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), webhookHandler.List)
			webhooks.POST("", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), webhookHandler.Create)
			webhooks.PUT("/:id", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), webhookHandler.Update)
			webhooks.DELETE("/:id", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), webhookHandler.Delete)
			webhooks.POST("/:id/test", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), webhookHandler.Test)
		}

		dsHandler := handler.NewDatasourceHandler(dsService, dsManager, dsConfigService)
		queryHandler := handler.NewQueryHandler(dsService, dsManager, dsConfigService, dsCacheService, dsConcurrentService)

		datasources := protected.Group("/datasources")
		{
			datasources.GET("", middleware.RBACMiddleware("admin", "system_admin", "domain_admin", "user"), dsHandler.List)
			datasources.GET("/types", dsHandler.SupportedTypes)
			datasources.POST("/test", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), dsHandler.TestConnectionByParams)
			datasources.POST("", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), dsHandler.Create)
			datasources.GET("/:id", middleware.DatasourcePermissionMiddleware(dsService, "read"), dsHandler.Get)
			datasources.PUT("/:id", middleware.DatasourcePermissionMiddleware(dsService, "update"), dsHandler.Update)
			datasources.DELETE("/:id", middleware.DatasourcePermissionMiddleware(dsService, "delete"), dsHandler.Delete)
			datasources.POST("/:id/test", middleware.DatasourcePermissionMiddleware(dsService, "read"), dsHandler.TestConnection)
			datasources.POST("/:id/permissions", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), dsHandler.GrantPermission)
			datasources.PUT("/:id/permissions/:perm_id", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), dsHandler.UpdatePermission)
			datasources.DELETE("/:id/permissions/:perm_id", middleware.RBACMiddleware("admin", "system_admin", "domain_admin"), dsHandler.RevokePermission)
			datasources.GET("/:id/permissions", middleware.DatasourcePermissionMiddleware(dsService, "manage"), dsHandler.GetPermissions)
			datasources.GET("/:id/metadata", middleware.DatasourcePermissionMiddleware(dsService, "query"), queryHandler.GetMetadata)
		}

		query := protected.Group("/query")
		{
			query.POST("/execute", middleware.DatasourcePermissionMiddleware(dsService, "query"), queryHandler.Execute)
			query.POST("/cancel/:query_id", middleware.DatasourcePermissionMiddleware(dsService, "query"), queryHandler.Cancel)
			query.POST("/export", middleware.DatasourcePermissionMiddleware(dsService, "download"), queryHandler.ExportCSV)
			query.GET("/history", queryHandler.GetHistory)
			query.DELETE("/history/:id", queryHandler.DeleteQueryHistory)
			query.POST("/history/batch-delete", queryHandler.BatchDeleteQueryHistory)
			query.GET("/saved-sql", queryHandler.ListSavedSQL)
			query.POST("/saved-sql", queryHandler.SaveSQL)
			query.DELETE("/saved-sql/:id", queryHandler.DeleteSavedSQL)
		}
	}

	// Serve static files and SPA after all API routes
	setupStaticRoutes(router, dsConfigService)

	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler: router,
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcSrv.Start(); err != nil {
			slog.Error("failed to start gRPC server", "error", err)
			os.Exit(1)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("HTTP server listening", "port", cfg.HTTPPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to start HTTP server", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("scheduler started", "http_port", cfg.HTTPPort, "grpc_port", cfg.GRPCPort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down servers")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	grpcSrv.Stop()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("servers exited cleanly")
	case <-time.After(5 * time.Second):
		slog.Info("servers force exited")
	}
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

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
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

func setupStaticRoutes(router *gin.Engine, dsConfigService *datasource.ConfigService) {
	webEnabled := func() bool {
		return dsConfigService.GetBool("web.enabled")
	}

	// 检查是否有前端资源
	hasWebAssets := func() bool {
		staticFS, _ := web.GetStaticFS()
		if staticFS == nil {
			return false
		}
		_, err := staticFS.Open("index.html")
		return err == nil
	}

	// 处理根路径
	router.GET("/", func(c *gin.Context) {
		if !webEnabled() {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"status":  "error",
				"message": "Not found. Built-in web service is disabled.",
			})
			return
		}
		if !hasWebAssets() {
			c.HTML(http.StatusOK, "", `
<!DOCTYPE html>
<html>
<head>
    <title>BDopsFlow</title>
    <style>
        body { font-family: system-ui, -apple-system, sans-serif; max-width: 600px; margin: 100px auto; padding: 0 20px; }
        h1 { color: #333; }
        .info { background: #f5f5f5; padding: 20px; border-radius: 8px; margin-top: 20px; }
    </style>
</head>
<body>
    <h1>BDopsFlow</h1>
    <div class="info">
        <p>Built-in web UI is not available.</p>
        <p>Please run <code>make build-frontend</code> to build the frontend first.</p>
        <p>Or use <code>make run-dev</code> for development mode.</p>
    </div>
</body>
</html>`)
			return
		}
		staticFS, _ := web.GetStaticFS()
		http.FileServer(staticFS).ServeHTTP(c.Writer, c.Request)
	})

	// 处理静态资源路径
	router.GET("/assets/*filepath", func(c *gin.Context) {
		if !webEnabled() {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"status":  "error",
				"message": "Not found. Built-in web service is disabled.",
			})
			return
		}
		if !hasWebAssets() {
			c.String(http.StatusNotFound, "Not found")
			return
		}
		staticFS, _ := web.GetStaticFS()
		http.FileServer(staticFS).ServeHTTP(c.Writer, c.Request)
	})

	// 处理其他特定的静态文件
	router.GET("/favicon.ico", func(c *gin.Context) {
		if !webEnabled() {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"status":  "error",
				"message": "Not found. Built-in web service is disabled.",
			})
			return
		}
		if !hasWebAssets() {
			c.String(http.StatusNotFound, "Not found")
			return
		}
		staticFS, _ := web.GetStaticFS()
		http.FileServer(staticFS).ServeHTTP(c.Writer, c.Request)
	})

	// NoRoute 处理：API 404 和 SPA 回退
	router.NoRoute(func(c *gin.Context) {
		// 1. 如果是 /api 或 /health 请求，直接返回 404
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"status":  "error",
				"message": "API endpoint not found",
			})
			return
		}
		if c.Request.URL.Path == "/health" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"status":  "error",
				"message": "Not found",
			})
			return
		}

		// 2. 检查是否启用了内置 Web
		if !webEnabled() {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"status":  "error",
				"message": "Not found. Built-in web service is disabled.",
			})
			return
		}

		// 3. 检查是否有前端资源
		if !hasWebAssets() {
			c.HTML(http.StatusOK, "", `
<!DOCTYPE html>
<html>
<head>
    <title>BDopsFlow</title>
    <style>
        body { font-family: system-ui, -apple-system, sans-serif; max-width: 600px; margin: 100px auto; padding: 0 20px; }
        h1 { color: #333; }
        .info { background: #f5f5f5; padding: 20px; border-radius: 8px; margin-top: 20px; }
    </style>
</head>
<body>
    <h1>BDopsFlow</h1>
    <div class="info">
        <p>Built-in web UI is not available.</p>
        <p>Please run <code>make build-frontend</code> to build the frontend first.</p>
        <p>Or use <code>make run-dev</code> for development mode.</p>
    </div>
</body>
</html>`)
			return
		}

		// 4. 启用了 Web 服务且有资源，处理 SPA 路由回退
		staticFS, _ := web.GetStaticFS()
		file, err := staticFS.Open("index.html")
		if err != nil {
			c.String(http.StatusNotFound, "Page not found")
			return
		}
		defer file.Close()

		http.ServeContent(c.Writer, c.Request, "index.html", time.Now(), file)
	})
}
