package main

import (
	"flag"
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
	configFile := flag.String("config", "", "path to config file (default: config.yaml in current directory)")
	flag.Parse()

	logger.Init()

	cfg := config.Load(*configFile)

	slog.Info("executor starting",
		"executor_id", cfg.ExecutorID,
		"executor_name", cfg.ExecutorName,
		"scheduler_addr", cfg.SchedulerAddr,
		"capacity", cfg.Capacity,
		"config_file", cfg.ConfigFile,
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
