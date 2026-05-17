package config

import (
	"log/slog"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/config"
)

type Config struct {
	HTTPPort      string
	GRPCPort      string
	RQLiteAddrs    []string // rqlite 多节点地址列表
	RQLiteUser     string
	RQLitePass     string
	RQLiteTLS      bool
	RedisMode      string // "single" 或 "sentinel"
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	RedisMaster    string
	RedisSentinelAddrs []string
	RedisSentinelPassword string
	JWTSecret      string
	JWTExpiry      int
	LogLevel       string
	LogFormat      string
	ConfigFile     string
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

	return &Config{
		HTTPPort:      cfg.GetString("app.http_port", "8080"),
		GRPCPort:      cfg.GetString("app.grpc_port", "50051"),
		RQLiteAddrs:   cfg.GetStringSlice("database.rqlite_addrs", []string{"http://localhost:4001"}),
		RQLiteUser:    cfg.GetString("database.rqlite_user", ""),
		RQLitePass:    cfg.GetString("database.rqlite_password", ""),
		RQLiteTLS:     cfg.GetBool("database.rqlite_tls", false),
		RedisMode:     cfg.GetString("redis.mode", "single"),
		RedisAddr:     cfg.GetString("redis.addr", "localhost:6379"),
		RedisPassword: cfg.GetString("redis.password", ""),
		RedisDB:       cfg.GetInt("redis.db", 0),
		RedisMaster:   cfg.GetString("redis.master_name", "mymaster"),
		RedisSentinelAddrs: cfg.GetStringSlice("redis.sentinel_addrs", []string{}),
		RedisSentinelPassword: cfg.GetString("redis.sentinel_password", ""),
		JWTSecret:     cfg.GetString("jwt.secret", "your-secret-key-change-in-production"),
		JWTExpiry:     cfg.GetInt("jwt.expiry_hours", 24),
		LogLevel:      cfg.GetString("log.level", "info"),
		LogFormat:     cfg.GetString("log.format", "json"),
		ConfigFile:    configured,
	}
}

func defaultConfig() *Config {
	return &Config{
		HTTPPort:      "8080",
		GRPCPort:      "50051",
		RQLiteAddrs:   []string{"http://localhost:4001"},
		RQLiteUser:    "",
		RQLitePass:    "",
		RQLiteTLS:     false,
		RedisMode:     "single",
		RedisAddr:     "localhost:6379",
		RedisPassword: "",
		RedisDB:       0,
		RedisMaster:   "mymaster",
		RedisSentinelAddrs: []string{},
		RedisSentinelPassword: "",
		JWTSecret:     "your-secret-key-change-in-production",
		JWTExpiry:     24,
		LogLevel:      "info",
		LogFormat:     "json",
	}
}
