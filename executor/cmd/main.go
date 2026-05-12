package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lynnyq/bdopsflow/executor/internal/config"
	"github.com/lynnyq/bdopsflow/executor/internal/executor"
	"github.com/lynnyq/bdopsflow/executor/internal/grpcclient"
	"github.com/lynnyq/bdopsflow/executor/internal/logger"
)

func main() {
	logger.Init()

	cfg := config.Load()

	slog.Info("executor starting",
		"executor_id", cfg.ExecutorID,
		"scheduler_addr", cfg.SchedulerAddr,
	)

	exec := executor.NewTaskExecutor(cfg.ExecutorID)

	client, err := grpcclient.NewClient(cfg.SchedulerAddr)
	if err != nil {
		slog.Error("failed to create gRPC client", "error", err)
		os.Exit(1)
	}

	go func() {
		if err := client.Subscribe(cfg.ExecutorID, cfg.ExecutorName, cfg.ExecutorName, cfg.Capacity, exec); err != nil {
			slog.Error("gRPC subscription failed", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("executor running",
		"executor_id", cfg.ExecutorID,
		"name", cfg.ExecutorName,
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("executor shutting down")
	client.Close()
}