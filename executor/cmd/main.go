package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lynnyq/bdopsflow/executor/internal/config"
	"github.com/lynnyq/bdopsflow/executor/internal/executor"
	"github.com/lynnyq/bdopsflow/executor/internal/grpcclient"
	"github.com/lynnyq/bdopsflow/executor/internal/logger"
	"github.com/lynnyq/bdopsflow/executor/internal/pool"
)

func main() {
	configFile := flag.String("config", "", "path to config file (default: config.yaml in current directory)")
	hostname := flag.String("hostname", "", "override hostname or IP for executor registration (default: system hostname)")
	flag.Parse()

	logger.Init()

	cfg := config.Load(*configFile)

	if *hostname != "" {
		cfg.Hostname = *hostname
	}

	slog.Info("executor starting",
		"executor_name", cfg.ExecutorName,
		"scheduler_addr", cfg.SchedulerAddr,
		"capacity", cfg.Capacity,
		"hostname", cfg.Hostname,
		"config_file", cfg.ConfigFile,
	)

	taskPool := pool.NewPool(cfg.Capacity)
	taskPool.Start()

	exec := executor.NewTaskExecutor(taskPool)

	client, err := grpcclient.NewClient(cfg.SchedulerAddr)
	if err != nil {
		slog.Error("failed to create gRPC client", "error", err)
		os.Exit(1)
	}

	go func() {
		address := fmt.Sprintf("%s#%d", cfg.Hostname, os.Getpid())
		if err := client.Subscribe(cfg.ExecutorName, address, cfg.Capacity, exec); err != nil {
			slog.Error("gRPC subscription failed", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("executor running",
		"name", cfg.ExecutorName,
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("executor shutting down")
	taskPool.Stop()
	client.Close()
}
