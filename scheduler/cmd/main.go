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
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	rqlite "github.com/rqlite/gorqlite"

	"github.com/lynnyq/bdopsflow/scheduler/internal/config"
	"github.com/lynnyq/bdopsflow/scheduler/internal/cron"
	"github.com/lynnyq/bdopsflow/scheduler/internal/grpcserver"
	"github.com/lynnyq/bdopsflow/scheduler/internal/handler"
	"github.com/lynnyq/bdopsflow/scheduler/internal/logger"
	"github.com/lynnyq/bdopsflow/scheduler/internal/middleware"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
	"github.com/lynnyq/bdopsflow/scheduler/internal/webhook"
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

func main() {
	configFile := flag.String("config", "", "path to config file (default: config.yaml in current directory)")
	flag.Parse()

	logger.Init()

	cfg := config.Load(*configFile)

	slog.Info("scheduler starting",
		"http_port", cfg.HTTPPort,
		"grpc_port", cfg.GRPCPort,
		"config_file", cfg.ConfigFile,
		"redis_mode", cfg.RedisMode,
		"rqlite_addrs", cfg.RQLiteAddrs,
		"rqlite_tls", cfg.RQLiteTLS,
		"rqlite_has_auth", cfg.RQLiteUser != "" && cfg.RQLitePass != "",
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

	schedulerService := service.NewSchedulerService(*db, redisClient)

	permissionService := service.NewPermissionService(*db, redisClient)
	userAdminService := service.NewUserAdminService(*db, permissionService)
	roleAdminService := service.NewRoleAdminService(*db, permissionService)
	domainAdminService := service.NewDomainAdminService(*db)
	executorDomainService := service.NewExecutorDomainService(*db)

	// 注入执行器领域服务到调度器服务
	schedulerService.ExecutorDomainService = executorDomainService

	schedulerService.StartCleanupRoutine()

	webhookSvc := webhook.NewService()
	schedulerService.SetWebhookService(webhookSvc)

	grpcSrv := grpcserver.NewServer(cfg.GRPCPort, schedulerService)

	cronScheduler := cron.NewCronScheduler(schedulerService, redisClient)
	schedulerService.SetCronScheduler(cronScheduler)

	if err := cronScheduler.Start(); err != nil {
		slog.Error("failed to start cron scheduler", "error", err)
		os.Exit(1)
	}
	defer cronScheduler.Stop()
	defer schedulerService.StopCleanupRoutine()

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(corsMiddleware())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authHandler := handler.NewAuthHandler(db)
	userAdminHandler := handler.NewUserAdminHandler(userAdminService)
	router.POST("/api/auth/login", authHandler.Login)
	router.POST("/api/auth/register", authHandler.Register)

	protected := router.Group("/api")
	protected.Use(middleware.JWTAuthMiddleware())
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
			tasks.GET("/executions/:executionId/logs", taskHandler.ExecutionLogs)
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
			workflows.GET("/executions/:executionId", workflowHandler.GetWorkflowExecution)
			workflows.GET("/executions/:executionId/logs", workflowHandler.GetExecutionLogs)
		}

		executorHandler := handler.NewExecutorHandler(schedulerService)
		executorDomainHandler := handler.NewExecutorDomainHandler(executorDomainService, permissionService, userAdminService)
		executors := protected.Group("/executors")
		{
			executors.GET("", executorDomainHandler.GetExecutorsWithDomains)
			executors.GET("/:name", executorHandler.Get)
			executors.GET("/:name/domains", middleware.RequireSystemAdmin(permissionService), executorDomainHandler.GetExecutorDomains)
			executors.POST("/:name/domains", middleware.RequireSystemAdmin(permissionService), executorDomainHandler.AssignDomains)
			executors.DELETE("/:name/domains/:domainId", middleware.RequireSystemAdmin(permissionService), executorDomainHandler.RemoveDomain)
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
			logs.DELETE("/:id", logHandler.Delete)
			logs.POST("/batch-delete", logHandler.BatchDelete)
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
		}

		admin := protected.Group("/admin")
		{
			permissionHandler := handler.NewPermissionHandler(permissionService)
			admin.GET("/permissions", middleware.RequireSystemAdmin(permissionService), permissionHandler.GetAllPermissions)

			admin.GET("/users", middleware.RequireSystemAdmin(permissionService), userAdminHandler.ListUsers)
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
		}
	}

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
