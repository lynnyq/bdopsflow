package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/lynnyq/bdopsflow/executor/internal/config"
	"github.com/lynnyq/bdopsflow/executor/internal/executor"
	"github.com/lynnyq/bdopsflow/executor/internal/grpcclient"
	"github.com/lynnyq/bdopsflow/executor/internal/logger"
	"github.com/lynnyq/bdopsflow/executor/internal/pool"
)

func parseAddrs(addrsStr string) []string {
	if addrsStr == "" {
		return nil
	}
	var addrs []string
	for _, addr := range strings.Split(addrsStr, ",") {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

func printExecutorHelp() {
	fmt.Fprint(os.Stderr, `BDopsFlow Executor - 任务执行引擎

用法:
  executor [选项]

选项:
  --config string           配置文件路径 (默认: 当前目录的 config.yaml)
  --executor-name string    执行器名称 (必需)
  --scheduler-addr string   调度器 gRPC 地址 (单个，向后兼容)
  --scheduler-addrs string  调度器 gRPC 地址 (逗号分隔，多个调度器)
  --capacity int            任务执行容量 (默认: 10)
  --timeout int             gRPC 请求超时秒数 (默认: 30)
  --hostname string         覆盖主机名或 IP 用于执行器注册
  --log-level string        日志级别: debug, info, warn, error (默认: info)
  --log-format string       日志格式: json, text (默认: json)
  -h, --help                显示帮助信息

示例:
  executor --executor-name my-exec --scheduler-addr localhost:50051
  executor --executor-name my-exec --scheduler-addrs host1:50051,host2:50051 --capacity 20
`)
}

func main() {
	flag.Usage = printExecutorHelp
	configFile := flag.String("config", "", "path to config file (default: config.yaml in current directory)")

	executorName := flag.String("executor-name", "", "executor name (required)")
	schedulerAddr := flag.String("scheduler-addr", "", "scheduler gRPC address (single, for backward compatibility)")
	schedulerAddrs := flag.String("scheduler-addrs", "", "scheduler gRPC addresses (comma-separated, for multiple schedulers)")

	capacity := flag.Int("capacity", 0, "task execution capacity (default: 10)")
	timeout := flag.Int("timeout", 0, "gRPC request timeout in seconds (default: 30)")
	hostname := flag.String("hostname", "", "override hostname or IP for executor registration (default: system hostname)")

	logLevel := flag.String("log-level", "", "log level: debug, info, warn, error (default: info)")
	logFormat := flag.String("log-format", "", "log format: json, text (default: json)")

	// 检查是否是 --help 或 -h
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			printExecutorHelp()
			return
		}
	}

	flag.Parse()

	logger.Init()

	cfg := config.Load(*configFile)

	cfg.Merge(
		*executorName,
		int32(*capacity),
		*schedulerAddr,
		parseAddrs(*schedulerAddrs),
		*timeout,
		*hostname,
		*logLevel,
		*logFormat,
	)

	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		fmt.Printf("\nUsage: %s [OPTIONS]\n\n", os.Args[0])
		fmt.Println("Required options:")
		fmt.Println("  --executor-name     executor name (or via config file)")
		fmt.Println("  --scheduler-addr     scheduler gRPC address (single, or via config file)")
		fmt.Println("  --scheduler-addrs   scheduler gRPC addresses (comma-separated, for multiple schedulers)")
		fmt.Println("\nOptional options:")
		fmt.Println("  --config            path to config file")
		fmt.Println("  --capacity          task execution capacity (default: 10)")
		fmt.Println("  --timeout           gRPC request timeout in seconds (default: 30)")
		fmt.Println("  --hostname          override hostname or IP for executor registration")
		fmt.Println("  --log-level         log level: debug, info, warn, error (default: info)")
		fmt.Println("  --log-format        log format: json, text (default: json)")
		os.Exit(1)
	}

	schedulerAddrsList := cfg.GetSchedulerAddresses()
	slog.Info("executor starting",
		"executor_name", cfg.ExecutorName,
		"scheduler_addrs", schedulerAddrsList,
		"capacity", cfg.Capacity,
		"hostname", cfg.Hostname,
		"config_file", cfg.ConfigFile,
	)

	taskPool := pool.NewPool(cfg.Capacity)
	taskPool.Start()

	exec := executor.NewTaskExecutor(taskPool)

	client, err := grpcclient.NewMultiClient(schedulerAddrsList)
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
