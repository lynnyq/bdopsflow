package config

import (
	"log/slog"
	"os"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/config"
)

type Config struct {
	ExecutorName  string
	Hostname      string
	Capacity      int32
	SchedulerAddr string
	Timeout       int
	LogLevel      string
	LogFormat     string
	ConfigFile    string
}

func getSystemHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		slog.Warn("failed to get system hostname, using default", "error", err)
		return "localhost"
	}
	return hostname
}

func Load(configFile string) *Config {
	defaultHostname := getSystemHostname()
	
	cfg, err := config.New(config.Options{
		ConfigFile: configFile,
	})
	if err != nil {
		slog.Warn("failed to load config file, using defaults", "error", err)
		return defaultConfig(defaultHostname)
	}

	if cfg == nil {
		return defaultConfig(defaultHostname)
	}

	configured := cfg.ConfigFile()
	if configured != "" {
		slog.Info("loaded config from file", "file", configured)
	}

	executorName := cfg.GetString("app.executor_name", "executor-default")
	hostname := cfg.GetString("app.hostname", defaultHostname)

	return &Config{
		ExecutorName:  executorName,
		Hostname:      hostname,
		Capacity:      cfg.GetInt32("app.capacity", 10),
		SchedulerAddr: cfg.GetString("scheduler.addr", "localhost:50051"),
		Timeout:       cfg.GetInt("scheduler.timeout", 30),
		LogLevel:      cfg.GetString("log.level", "info"),
		LogFormat:     cfg.GetString("log.format", "json"),
		ConfigFile:    configured,
	}
}

func defaultConfig(hostname string) *Config {
	if hostname == "" {
		hostname = "localhost"
	}
	return &Config{
		ExecutorName:  "executor-default",
		Hostname:      hostname,
		Capacity:      10,
		SchedulerAddr: "localhost:50051",
		Timeout:       30,
		LogLevel:      "info",
		LogFormat:     "json",
	}
}
