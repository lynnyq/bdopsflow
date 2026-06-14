package config

import (
	"log/slog"
	"sync"

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
	LogPath               string
	ConfigFile            string
	RSAPublicKey          string
	RSAPrivateKey         string
	SSOEnabled            bool
	SSOUrl                string
	SSOPublicKey          string
	SSOTimeout            int
	DatasourceCrypto      DatasourceCryptoConfig

	mu sync.RWMutex
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
		LogPath:               cfg.GetString("log.path", ""),
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

func (c *Config) Reload() error {
	if c.ConfigFile == "" {
		slog.Warn("no config file configured, skipping reload")
		return nil
	}

	slog.Info("reloading config file", "config_file", c.ConfigFile)

	newCfg, err := config.New(config.Options{
		ConfigFile: c.ConfigFile,
	})
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 应用配置（可以热重载的配置）
	c.HTTPPort = newCfg.GetString("app.http_port", c.HTTPPort)
	c.GRPCPort = newCfg.GetString("app.grpc_port", c.GRPCPort)
	c.AdvertiseAddr = newCfg.GetString("app.advertise_addr", c.AdvertiseAddr)
	c.AllowRegister = newCfg.GetBool("app.allow_register", c.AllowRegister)
	c.CORSAllowOrigins = newCfg.GetStringSlice("app.cors_allow_origins", c.CORSAllowOrigins)

	// 日志配置
	c.LogLevel = newCfg.GetString("log.level", c.LogLevel)
	c.LogFormat = newCfg.GetString("log.format", c.LogFormat)
	c.LogPath = newCfg.GetString("log.path", c.LogPath)

	// JWT 配置（可以热重载，新 token 会使用新配置）
	c.JWTExpiry = newCfg.GetInt("jwt.expiry_hours", c.JWTExpiry)
	c.JWTRefreshExpiry = newCfg.GetInt("jwt.refresh_expiry_hours", c.JWTRefreshExpiry)

	// SSO 配置（可以热重载）
	c.SSOEnabled = newCfg.GetBool("sso.enabled", c.SSOEnabled)
	c.SSOUrl = newCfg.GetString("sso.url", c.SSOUrl)
	c.SSOPublicKey = newCfg.GetString("sso.public_key", c.SSOPublicKey)
	c.SSOTimeout = newCfg.GetInt("sso.timeout", c.SSOTimeout)

	// 数据源加密配置（可以热重载，新数据源会使用新配置）
	c.DatasourceCrypto.EncryptionKey = newCfg.GetString("datasource.encryption_key", c.DatasourceCrypto.EncryptionKey)
	c.DatasourceCrypto.KeySource = newCfg.GetString("datasource.key_source", c.DatasourceCrypto.KeySource)
	c.DatasourceCrypto.KeyEnvVar = newCfg.GetString("datasource.key_env_var", c.DatasourceCrypto.KeyEnvVar)
	c.DatasourceCrypto.KeyFile = newCfg.GetString("datasource.key_file", c.DatasourceCrypto.KeyFile)
	c.DatasourceCrypto.AutoRotateDays = newCfg.GetInt("datasource.auto_rotate_days", c.DatasourceCrypto.AutoRotateDays)

	slog.Info("config reloaded successfully",
		"log_level", c.LogLevel,
		"jwt_expiry_hours", c.JWTExpiry,
		"sso_enabled", c.SSOEnabled,
	)
	return nil
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
