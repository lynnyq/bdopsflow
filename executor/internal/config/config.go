package config

import (
	"log/slog"
	"os"
	"strings"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/config"
)

type Config struct {
	ExecutorName   string
	Hostname       string
	Capacity       int32
	SchedulerAddr  string
	SchedulerAddrs []string
	Timeout        int
	LogLevel       string
	LogFormat      string
	ConfigFile     string
}

func getSystemHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		slog.Warn("failed to get system hostname, using default", "error", err)
		return "localhost"
	}
	return hostname
}

func parseSchedulerAddrs(addrsStr string) []string {
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

// Load loads configuration from config file
func Load(configFile string) *Config {
	defaultHostname := getSystemHostname()

	cfg, err := config.New(config.Options{
		ConfigFile: configFile,
	})
	if err != nil {
		// 如果显式指定了配置文件但加载失败，应该记录错误并使用默认配置
		// 但不阻止启动，因为可能通过命令行参数提供必要配置
		if configFile != "" {
			slog.Error("failed to load specified config file", "file", configFile, "error", err)
		} else {
			slog.Warn("failed to load default config file, using defaults", "error", err)
		}
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
		ExecutorName:   executorName,
		Hostname:       hostname,
		Capacity:       cfg.GetInt32("app.capacity", 10),
		SchedulerAddr:  cfg.GetString("scheduler.addr", ""),
		SchedulerAddrs: parseSchedulerAddrs(cfg.GetString("scheduler.addrs", "")),
		Timeout:        cfg.GetInt("scheduler.timeout", 30),
		LogLevel:       cfg.GetString("log.level", "info"),
		LogFormat:      cfg.GetString("log.format", "json"),
		ConfigFile:     configured,
	}
}

func defaultConfig(hostname string) *Config {
	if hostname == "" {
		hostname = "localhost"
	}
	return &Config{
		ExecutorName:   "",
		Hostname:       hostname,
		Capacity:       10,
		SchedulerAddr:  "",
		SchedulerAddrs: nil,
		Timeout:        30,
		LogLevel:       "info",
		LogFormat:      "json",
	}
}

// Merge merges command line arguments into config, command line args take precedence
func (c *Config) Merge(
	executorName string,
	capacity int32,
	schedulerAddr string,
	schedulerAddrs []string,
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
	if len(schedulerAddrs) > 0 {
		c.SchedulerAddrs = schedulerAddrs
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

// GetSchedulerAddresses returns the list of scheduler addresses to connect to
// Priority: SchedulerAddrs > SchedulerAddr > first address from config file
func (c *Config) GetSchedulerAddresses() []string {
	if len(c.SchedulerAddrs) > 0 {
		return c.SchedulerAddrs
	}
	if c.SchedulerAddr != "" {
		return []string{c.SchedulerAddr}
	}
	return nil
}

// Validate validates that required configuration is present
func (c *Config) Validate() error {
	if c.ExecutorName == "" {
		return newRequiredError("executor_name")
	}
	addrs := c.GetSchedulerAddresses()
	if len(addrs) == 0 {
		return newRequiredError("scheduler.addr or scheduler.addrs")
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
