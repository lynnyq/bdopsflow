package config

import (
	"log/slog"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/config"
)

type Config struct {
	ExecutorID    string
	ExecutorName  string
	Hostname      string
	Capacity      int32
	SchedulerAddr string
	Timeout       int
	LogLevel      string
	LogFormat     string
	ConfigFile    string
}

func Load(configFile string) *Config {
	cfg, err := config.New(config.Options{
		ConfigFile: configFile,
	})
	if err != nil {
		slog.Warn("failed to load config file, using defaults", "error", err)
		return defaultConfig()
	}

	if cfg == nil {
		return defaultConfig()
	}

	configured := cfg.ConfigFile()
	if configured != "" {
		slog.Info("loaded config from file", "file", configured)
	}

	executorID := cfg.GetString("app.executor_id", "executor-1")

	return &Config{
		ExecutorID:    executorID,
		ExecutorName:  cfg.GetString("app.executor_name", executorID),
		Hostname:      cfg.GetString("app.hostname", executorID),
		Capacity:      cfg.GetInt32("app.capacity", 10),
		SchedulerAddr: cfg.GetString("scheduler.addr", "localhost:50051"),
		Timeout:       cfg.GetInt("scheduler.timeout", 30),
		LogLevel:      cfg.GetString("log.level", "info"),
		LogFormat:     cfg.GetString("log.format", "json"),
		ConfigFile:    configured,
	}
}

func defaultConfig() *Config {
	return &Config{
		ExecutorID:    "executor-1",
		ExecutorName:  "executor-1",
		Hostname:      "executor-1",
		Capacity:      10,
		SchedulerAddr: "localhost:50051",
		Timeout:       30,
		LogLevel:      "info",
		LogFormat:     "json",
	}
}
