package config

import (
	"log/slog"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/config"
)

type Config struct {
	HTTPPort      string
	GRPCPort      string
	RQLiteDSN     string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	JWTSecret     string
	JWTExpiry     int
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

	return &Config{
		HTTPPort:      cfg.GetString("app.http_port", "8080"),
		GRPCPort:      cfg.GetString("app.grpc_port", "50051"),
		RQLiteDSN:     cfg.GetString("database.rqlite_dsn", "http://localhost:4001"),
		RedisAddr:     cfg.GetString("redis.addr", "localhost:6379"),
		RedisPassword: cfg.GetString("redis.password", ""),
		RedisDB:       cfg.GetInt("redis.db", 0),
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
		RQLiteDSN:     "http://localhost:4001",
		RedisAddr:     "localhost:6379",
		RedisPassword: "",
		RedisDB:       0,
		JWTSecret:     "your-secret-key-change-in-production",
		JWTExpiry:     24,
		LogLevel:      "info",
		LogFormat:     "json",
	}
}
