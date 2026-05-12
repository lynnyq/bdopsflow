package main

import (
	"context"
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
)

func main() {
	logger.Init()

	cfg := config.Load()

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

	grpcSrv := grpcserver.NewServer(cfg.GRPCPort, schedulerService)

	cronScheduler := cron.NewCronScheduler(schedulerService, redisClient)
	if err := cronScheduler.Start(); err != nil {
		slog.Error("failed to start cron scheduler", "error", err)
		os.Exit(1)
	}
	defer cronScheduler.Stop()

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(corsMiddleware())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authHandler := handler.NewAuthHandler(db)
	router.POST("/api/auth/login", authHandler.Login)
	router.POST("/api/auth/register", authHandler.Register)

	protected := router.Group("/api")
	protected.Use(middleware.JWTAuthMiddleware())
	{
		protected.GET("/auth/current", authHandler.GetCurrentUser)

		taskHandler := handler.NewTaskHandler(schedulerService)
		tasks := protected.Group("/tasks")
		{
			tasks.GET("", taskHandler.List)
			tasks.POST("", middleware.RBACMiddleware("admin", "operator"), taskHandler.Create)
			tasks.GET("/:id", taskHandler.Get)
			tasks.PUT("/:id", middleware.RBACMiddleware("admin", "operator"), taskHandler.Update)
			tasks.DELETE("/:id", middleware.RBACMiddleware("admin"), taskHandler.Delete)
			tasks.POST("/:id/trigger", middleware.RBACMiddleware("admin", "operator"), taskHandler.Trigger)
			tasks.GET("/:id/executions", taskHandler.Executions)
			tasks.GET("/executions/:executionId/logs", taskHandler.ExecutionLogs)
		}

		workflowHandler := handler.NewWorkflowHandler(schedulerService)
		workflows := protected.Group("/workflows")
		{
			workflows.GET("", workflowHandler.List)
			workflows.POST("", middleware.RBACMiddleware("admin", "operator"), workflowHandler.Create)
			workflows.GET("/:id", workflowHandler.Get)
			workflows.PUT("/:id", middleware.RBACMiddleware("admin", "operator"), workflowHandler.Update)
			workflows.DELETE("/:id", middleware.RBACMiddleware("admin"), workflowHandler.Delete)
			workflows.POST("/:id/trigger", middleware.RBACMiddleware("admin", "operator"), workflowHandler.TriggerWorkflow)
			workflows.GET("/:id/executions", workflowHandler.GetWorkflowExecutions)
			workflows.GET("/executions/:executionId", workflowHandler.GetWorkflowExecution)
			workflows.GET("/executions/:executionId/logs", workflowHandler.GetExecutionLogs)
		}

		executorHandler := handler.NewExecutorHandler(schedulerService)
		executors := protected.Group("/executors")
		{
			executors.GET("", executorHandler.List)
			executors.GET("/:id", executorHandler.Get)
			executors.DELETE("/:id", middleware.RBACMiddleware("admin"), executorHandler.Delete)
		}

		logHandler := handler.NewLogHandler(schedulerService)
		logs := protected.Group("/logs")
		{
			logs.GET("", logHandler.List)
			logs.DELETE("/:id", logHandler.Delete)
			logs.POST("/batch-delete", logHandler.BatchDelete)
		}

		protected.GET("/logs/stream", taskHandler.StreamLogs)
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