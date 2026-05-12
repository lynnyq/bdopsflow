package config

import (
	"os"
)

type Config struct {
	HTTPPort     string
	GRPCPort     string
	RQLiteDSN    string
	RedisAddr    string
	RedisPassword string
	RedisDB      int
}

func Load() *Config {
	return &Config{
		HTTPPort:     getEnv("HTTP_PORT", "8080"),
		GRPCPort:     getEnv("GRPC_PORT", "50051"),
		RQLiteDSN:    getEnv("RQLITE_DSN", "http://localhost:4001"),
		RedisAddr:    getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:      0,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
