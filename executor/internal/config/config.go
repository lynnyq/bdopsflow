package config

import (
	"log/slog"

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

	executorName := cfg.GetString("app.executor_name", "executor-default")
	hostname := cfg.GetString("app.hostname", "localhost")

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

func defaultConfig() *Config {
	return &Config{
		ExecutorName:  "executor-default",
		Hostname:      "localhost",
		Capacity:      10,
		SchedulerAddr: "localhost:50051",
		Timeout:       30,
		LogLevel:      "info",
		LogFormat:     "json",
	}
}
