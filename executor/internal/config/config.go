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

// Load loads configuration from config file
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

	executorName := cfg.GetString("app.executor_name", "")
	hostname := cfg.GetString("app.hostname", defaultHostname)

	return &Config{
		ExecutorName:  executorName,
		Hostname:      hostname,
		Capacity:      cfg.GetInt32("app.capacity", 10),
		SchedulerAddr: cfg.GetString("scheduler.addr", ""),
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
		ExecutorName:  "",
		Hostname:      hostname,
		Capacity:      10,
		SchedulerAddr: "",
		Timeout:       30,
		LogLevel:      "info",
		LogFormat:     "json",
	}
}

// Merge merges command line arguments into config, command line args take precedence
func (c *Config) Merge(
	executorName string,
	capacity int32,
	schedulerAddr string,
	timeout int,
	hostname string,
	logLevel string,
	logFormat string,
) {
	if executorName != "" {
		c.ExecutorName = executorName
	}
	if capacity > 0 {
		c.Capacity = capacity
	}
	if schedulerAddr != "" {
		c.SchedulerAddr = schedulerAddr
	}
	if timeout > 0 {
		c.Timeout = timeout
	}
	if hostname != "" {
		c.Hostname = hostname
	}
	if logLevel != "" {
		c.LogLevel = logLevel
	}
	if logFormat != "" {
		c.LogFormat = logFormat
	}
}

// Validate validates that required configuration is present
func (c *Config) Validate() error {
	if c.ExecutorName == "" {
		return newRequiredError("executor_name")
	}
	if c.SchedulerAddr == "" {
		return newRequiredError("scheduler.addr")
	}
	return nil
}

func newRequiredError(field string) error {
	return &RequiredError{Field: field}
}

type RequiredError struct {
	Field string
}

func (e *RequiredError) Error() string {
	return e.Field + " is required"
}
