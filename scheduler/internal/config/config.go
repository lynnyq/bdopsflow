package config

import (
	"log/slog"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/config"
)

type Config struct {
	HTTPPort              string
	GRPCPort              string
	AdvertiseAddr         string
	NodeID                string
	RQLiteAddrs           []string
	RQLiteUser            string
	RQLitePass            string
	RQLiteTLS             bool
	RedisMode             string
	RedisAddr             string
	RedisPassword         string
	RedisDB               int
	RedisMaster           string
	RedisSentinelAddrs    []string
	RedisSentinelPassword string
	JWTSecret             string
	JWTExpiry             int
	JWTRefreshExpiry      int
	AllowRegister         bool
	CORSAllowOrigins      []string
	LogLevel              string
	LogFormat             string
	ConfigFile            string
	RSAPublicKey          string
	RSAPrivateKey         string
	SSOEnabled            bool
	SSOUrl                string
	SSOPublicKey          string
	SSOTimeout            int
	DatasourceCrypto      DatasourceCryptoConfig
}

type DatasourceCryptoConfig struct {
	EncryptionKey  string
	KeySource      string
	KeyEnvVar      string
	KeyFile        string
	AutoRotateDays int
}

func Load(configFile string) *Config {
	slog.Info("attempting to load config file", "config_file", configFile)

	cfg, err := config.New(config.Options{
		ConfigFile: configFile,
	})
	if err != nil {
		slog.Warn("failed to load config file, using defaults", "error", err)
		return defaultConfig()
	}

	if cfg == nil {
		slog.Warn("config is nil, using defaults")
		return defaultConfig()
	}

	configured := cfg.ConfigFile()
	if configured != "" {
		slog.Info("loaded config from file", "file", configured)
	} else {
		slog.Warn("no config file loaded, using defaults")
	}

	return &Config{
		HTTPPort:              cfg.GetString("app.http_port", "8080"),
		GRPCPort:              cfg.GetString("app.grpc_port", "50051"),
		AdvertiseAddr:         cfg.GetString("app.advertise_addr", ""),
		NodeID:                cfg.GetString("app.node_id", ""),
		RQLiteAddrs:           cfg.GetStringSlice("database.rqlite_addrs", []string{"http://localhost:4001"}),
		RQLiteUser:            cfg.GetString("database.rqlite_user", ""),
		RQLitePass:            cfg.GetString("database.rqlite_password", ""),
		RQLiteTLS:             cfg.GetBool("database.rqlite_tls", false),
		RedisMode:             cfg.GetString("redis.mode", "single"),
		RedisAddr:             cfg.GetString("redis.addr", "localhost:6379"),
		RedisPassword:         cfg.GetString("redis.password", ""),
		RedisDB:               cfg.GetInt("redis.db", 0),
		RedisMaster:           cfg.GetString("redis.master_name", "mymaster"),
		RedisSentinelAddrs:    cfg.GetStringSlice("redis.sentinel_addrs", []string{}),
		RedisSentinelPassword: cfg.GetString("redis.sentinel_password", ""),
		JWTSecret:             cfg.GetString("jwt.secret", "your-secret-key-change-in-production"),
		JWTExpiry:             cfg.GetInt("jwt.expiry_hours", 2),
		JWTRefreshExpiry:      cfg.GetInt("jwt.refresh_expiry_hours", 168),
		AllowRegister:         cfg.GetBool("app.allow_register", false),
		CORSAllowOrigins:      cfg.GetStringSlice("app.cors_allow_origins", []string{}),
		LogLevel:              cfg.GetString("log.level", "info"),
		LogFormat:             cfg.GetString("log.format", "json"),
		ConfigFile:            configured,
		RSAPublicKey:          cfg.GetString("rsa.public_key", ""),
		RSAPrivateKey:         cfg.GetString("rsa.private_key", ""),
		SSOEnabled:            cfg.GetBool("sso.enabled", false),
		SSOUrl:                cfg.GetString("sso.url", ""),
		SSOPublicKey:          cfg.GetString("sso.public_key", ""),
		SSOTimeout:            cfg.GetInt("sso.timeout", 10),
		DatasourceCrypto: DatasourceCryptoConfig{
			EncryptionKey:  cfg.GetString("datasource.encryption_key", "change-in-prod-32byte-key1-here1"),
			KeySource:      cfg.GetString("datasource.key_source", "direct"),
			KeyEnvVar:      cfg.GetString("datasource.key_env_var", "BDOPSFLOW_ENCRYPTION_KEY"),
			KeyFile:        cfg.GetString("datasource.key_file", ""),
			AutoRotateDays: cfg.GetInt("datasource.auto_rotate_days", 0),
		},
	}
}

func defaultConfig() *Config {
	return &Config{
		HTTPPort:              "8080",
		GRPCPort:              "50051",
		AdvertiseAddr:         "",
		NodeID:                "",
		RQLiteAddrs:           []string{"http://localhost:4001"},
		RQLiteUser:            "",
		RQLitePass:            "",
		RQLiteTLS:             false,
		RedisMode:             "single",
		RedisAddr:             "localhost:6379",
		RedisPassword:         "",
		RedisDB:               0,
		RedisMaster:           "mymaster",
		RedisSentinelAddrs:    []string{},
		RedisSentinelPassword: "",
		JWTSecret:             "your-secret-key-change-in-production",
		JWTExpiry:             2,
		JWTRefreshExpiry:      168,
		AllowRegister:         false,
		CORSAllowOrigins:      []string{},
		LogLevel:              "info",
		LogFormat:             "json",
		DatasourceCrypto: DatasourceCryptoConfig{
			EncryptionKey:  "change-in-prod-32byte-key1-here1",
			KeySource:      "direct",
			KeyEnvVar:      "BDOPSFLOW_ENCRYPTION_KEY",
			AutoRotateDays: 0,
		},
	}
}
