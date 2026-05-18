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
	// 配置文件参数
	configFile := flag.String("config", "", "path to config file (default: config.yaml in current directory)")
	
	// 必需参数
	executorName := flag.String("executor-name", "", "executor name (required)")
	schedulerAddr := flag.String("scheduler-addr", "", "scheduler gRPC address (required)")
	
	// 可选参数，有默认值
	capacity := flag.Int("capacity", 0, "task execution capacity (default: 10)")
	timeout := flag.Int("timeout", 0, "gRPC request timeout in seconds (default: 30)")
	hostname := flag.String("hostname", "", "override hostname or IP for executor registration (default: system hostname)")
	
	// 日志可选参数
	logLevel := flag.String("log-level", "", "log level: debug, info, warn, error (default: info)")
	logFormat := flag.String("log-format", "", "log format: json, text (default: json)")
	
	flag.Parse()
	
	logger.Init()
	
	// 从配置文件加载
	cfg := config.Load(*configFile)
	
	// 合并命令行参数（优先级高于配置文件）
	cfg.Merge(
		*executorName,
		int32(*capacity),
		*schedulerAddr,
		*timeout,
		*hostname,
		*logLevel,
		*logFormat,
	)
	
	// 验证必需参数
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		fmt.Printf("\nUsage: %s [OPTIONS]\n\n", os.Args[0])
		fmt.Println("Required options:")
		fmt.Println("  --executor-name     executor name (or via config file)")
		fmt.Println("  --scheduler-addr    scheduler gRPC address (or via config file)")
		fmt.Println("\nOptional options:")
		fmt.Println("  --config            path to config file")
		fmt.Println("  --capacity          task execution capacity (default: 10)")
		fmt.Println("  --timeout           gRPC request timeout in seconds (default: 30)")
		fmt.Println("  --hostname          override hostname or IP for executor registration")
		fmt.Println("  --log-level         log level: debug, info, warn, error (default: info)")
		fmt.Println("  --log-format        log format: json, text (default: json)")
		os.Exit(1)
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
