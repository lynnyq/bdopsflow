package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
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

func main() {
	configFile := flag.String("config", "", "path to config file (default: config.yaml in current directory)")
	flag.Parse()

	logger.Init()

	cfg := config.Load(*configFile)

	slog.Info("scheduler starting",
		"http_port", cfg.HTTPPort,
		"grpc_port", cfg.GRPCPort,
		"config_file", cfg.ConfigFile,
	)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to Redis")

	db, err := rqlite.Open(cfg.RQLiteDSN)
	if err != nil {
		slog.Error("failed to connect to rqlite", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to rqlite")

	schedulerService := service.NewSchedulerService(*db, redisClient)

	permissionService := service.NewPermissionService(*db, redisClient)
	userAdminService := service.NewUserAdminService(*db, permissionService)
	roleAdminService := service.NewRoleAdminService(*db, permissionService)
	domainAdminService := service.NewDomainAdminService(*db)
	executorDomainService := service.NewExecutorDomainService(*db)

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
		bdopsflow_tasks := protected.Group("/bdopsflow_tasks")
		{
			bdopsflow_tasks.GET("", taskHandler.List)
			bdopsflow_tasks.POST("", middleware.RBACMiddleware("admin", "operator"), taskHandler.Create)
			bdopsflow_tasks.GET("/:id", taskHandler.Get)
			bdopsflow_tasks.PUT("/:id", middleware.RBACMiddleware("admin", "operator"), taskHandler.Update)
			bdopsflow_tasks.DELETE("/:id", middleware.RBACMiddleware("admin"), taskHandler.Delete)
			bdopsflow_tasks.POST("/:id/trigger", middleware.RBACMiddleware("admin", "operator"), taskHandler.Trigger)
			bdopsflow_tasks.GET("/:id/executions", taskHandler.Executions)
			bdopsflow_tasks.GET("/executions/:executionId/logs", taskHandler.ExecutionLogs)
		}

		workflowHandler := handler.NewWorkflowHandler(schedulerService)
		bdopsflow_workflows := protected.Group("/bdopsflow_workflows")
		{
			bdopsflow_workflows.GET("", workflowHandler.List)
			bdopsflow_workflows.POST("", middleware.RBACMiddleware("admin", "operator"), workflowHandler.Create)
			bdopsflow_workflows.GET("/:id", workflowHandler.Get)
			bdopsflow_workflows.PUT("/:id", middleware.RBACMiddleware("admin", "operator"), workflowHandler.Update)
			bdopsflow_workflows.DELETE("/:id", middleware.RBACMiddleware("admin"), workflowHandler.Delete)
			bdopsflow_workflows.POST("/:id/trigger", middleware.RBACMiddleware("admin", "operator"), workflowHandler.TriggerWorkflow)
			bdopsflow_workflows.GET("/:id/executions", workflowHandler.GetWorkflowExecutions)
			bdopsflow_workflows.GET("/executions/:executionId", workflowHandler.GetWorkflowExecution)
			bdopsflow_workflows.GET("/executions/:executionId/logs", workflowHandler.GetExecutionLogs)
		}

		executorHandler := handler.NewExecutorHandler(schedulerService)
		bdopsflow_executors := protected.Group("/bdopsflow_executors")
		{
			bdopsflow_executors.GET("", executorHandler.List)
			bdopsflow_executors.GET("/:id", executorHandler.Get)
			bdopsflow_executors.POST("/:id/online", middleware.RBACMiddleware("admin"), executorHandler.Online)
			bdopsflow_executors.POST("/:id/offline", middleware.RBACMiddleware("admin"), executorHandler.Offline)
			bdopsflow_executors.PUT("/:id/capacity", middleware.RBACMiddleware("admin"), executorHandler.UpdateCapacity)
			bdopsflow_executors.DELETE("/:id", middleware.RBACMiddleware("admin"), executorHandler.Delete)
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
			dashboard.POST("/scheduler/pause", middleware.RBACMiddleware("admin"), dashboardHandler.PauseScheduler)
			dashboard.POST("/scheduler/resume", middleware.RBACMiddleware("admin"), dashboardHandler.ResumeScheduler)
		}

		admin := protected.Group("/admin")
		{
			permissionHandler := handler.NewPermissionHandler(permissionService)
			admin.GET("/bdopsflow_permissions", middleware.RequireSystemAdmin(permissionService), permissionHandler.GetAllPermissions)

			admin.GET("/bdopsflow_users", middleware.RequireSystemAdmin(permissionService), userAdminHandler.ListUsers)
			admin.GET("/bdopsflow_users/:id", middleware.RequireSystemAdmin(permissionService), userAdminHandler.GetUser)
			admin.POST("/bdopsflow_users", middleware.RequireSystemAdmin(permissionService), userAdminHandler.CreateUser)
			admin.PUT("/bdopsflow_users/:id", middleware.RequireAdminOrDomainAdmin(), userAdminHandler.UpdateUser)
			admin.DELETE("/bdopsflow_users/:id", middleware.RequireSystemAdmin(permissionService), userAdminHandler.DeleteUser)
			admin.POST("/bdopsflow_users/:id/bdopsflow_roles", middleware.RequireSystemAdmin(permissionService), userAdminHandler.AssignUserRoles)
			admin.GET("/bdopsflow_users/:id/bdopsflow_roles", middleware.RequireSystemAdmin(permissionService), userAdminHandler.GetUserRoles)
			admin.POST("/bdopsflow_users/:id/bdopsflow_domains", middleware.RequireSystemAdmin(permissionService), userAdminHandler.AssignUserDomains)
			admin.POST("/bdopsflow_users/:id/reset-password", middleware.RequireAdminOrDomainAdmin(), userAdminHandler.ResetUserPassword)

			roleAdminHandler := handler.NewRoleAdminHandler(roleAdminService)
			admin.GET("/bdopsflow_roles", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.ListRoles)
			admin.GET("/bdopsflow_roles/:id", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.GetRole)
			admin.POST("/bdopsflow_roles", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.CreateRole)
			admin.PUT("/bdopsflow_roles/:id", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.UpdateRole)
			admin.DELETE("/bdopsflow_roles/:id", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.DeleteRole)
			admin.GET("/bdopsflow_roles/:id/bdopsflow_permissions", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.GetRolePermissions)
			admin.POST("/bdopsflow_roles/:id/bdopsflow_permissions", middleware.RequireSystemAdmin(permissionService), roleAdminHandler.AssignPermissions)

			domainAdminHandler := handler.NewDomainAdminHandler(domainAdminService)
			admin.GET("/bdopsflow_domains", middleware.RequireSystemAdmin(permissionService), domainAdminHandler.ListDomains)
			admin.GET("/bdopsflow_domains/:id", middleware.RequireSystemAdmin(permissionService), domainAdminHandler.GetDomain)
			admin.POST("/bdopsflow_domains", middleware.RequireSystemAdmin(permissionService), domainAdminHandler.CreateDomain)
			admin.PUT("/bdopsflow_domains/:id", middleware.RequireSystemAdmin(permissionService), domainAdminHandler.UpdateDomain)
			admin.DELETE("/bdopsflow_domains/:id", middleware.RequireSystemAdmin(permissionService), domainAdminHandler.DeleteDomain)

			executorDomainHandler := handler.NewExecutorDomainHandler(executorDomainService)
			admin.GET("/bdopsflow_executors/:id/bdopsflow_domains", middleware.RequireSystemAdmin(permissionService), executorDomainHandler.GetExecutorDomains)
			admin.POST("/bdopsflow_executors/:id/bdopsflow_domains", middleware.RequireSystemAdmin(permissionService), executorDomainHandler.AssignDomains)
			admin.DELETE("/bdopsflow_executors/:id/bdopsflow_domains/:domainId", middleware.RequireSystemAdmin(permissionService), executorDomainHandler.RemoveDomain)
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
