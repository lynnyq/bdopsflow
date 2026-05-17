package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	expected := &Config{
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

	if cfg.HTTPPort != expected.HTTPPort {
		t.Errorf("HTTPPort = %v, want %v", cfg.HTTPPort, expected.HTTPPort)
	}
	if cfg.GRPCPort != expected.GRPCPort {
		t.Errorf("GRPCPort = %v, want %v", cfg.GRPCPort, expected.GRPCPort)
	}
	if cfg.RQLiteDSN != expected.RQLiteDSN {
		t.Errorf("RQLiteDSN = %v, want %v", cfg.RQLiteDSN, expected.RQLiteDSN)
	}
	if cfg.RedisAddr != expected.RedisAddr {
		t.Errorf("RedisAddr = %v, want %v", cfg.RedisAddr, expected.RedisAddr)
	}
	if cfg.JWTSecret != expected.JWTSecret {
		t.Errorf("JWTSecret = %v, want %v", cfg.JWTSecret, expected.JWTSecret)
	}
	if cfg.LogLevel != expected.LogLevel {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, expected.LogLevel)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	cfg := Load("/non/existent/path/config.yaml")

	if cfg == nil {
		t.Error("Load() should return default config for non-existent file")
	}

	if cfg.HTTPPort != "8080" {
		t.Errorf("expected default HTTPPort '8080', got '%s'", cfg.HTTPPort)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "empty_config.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	cfg := Load(tmpFile.Name())

	if cfg == nil {
		t.Error("Load() should return default config for empty file")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
app:
  http_port: "9090"
  grpc_port: "50052"

database:
  rqlite_dsn: "http://rqlite:4001"

redis:
  addr: "redis:6379"
  password: "test-pass"
  db: 1

jwt:
  secret: "my-secret-key"
  expiry_hours: 12

log:
  level: "debug"
  format: "text"
`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config content: %v", err)
	}
	tmpFile.Close()

	cfg := Load(tmpFile.Name())

	if cfg.HTTPPort != "9090" {
		t.Errorf("HTTPPort = %v, want %v", cfg.HTTPPort, "9090")
	}
	if cfg.GRPCPort != "50052" {
		t.Errorf("GRPCPort = %v, want %v", cfg.GRPCPort, "50052")
	}
	if cfg.RQLiteDSN != "http://rqlite:4001" {
		t.Errorf("RQLiteDSN = %v, want %v", cfg.RQLiteDSN, "http://rqlite:4001")
	}
	if cfg.RedisAddr != "redis:6379" {
		t.Errorf("RedisAddr = %v, want %v", cfg.RedisAddr, "redis:6379")
	}
	if cfg.RedisPassword != "test-pass" {
		t.Errorf("RedisPassword = %v, want %v", cfg.RedisPassword, "test-pass")
	}
	if cfg.RedisDB != 1 {
		t.Errorf("RedisDB = %v, want %v", cfg.RedisDB, 1)
	}
	if cfg.JWTSecret != "my-secret-key" {
		t.Errorf("JWTSecret = %v, want %v", cfg.JWTSecret, "my-secret-key")
	}
	if cfg.JWTExpiry != 12 {
		t.Errorf("JWTExpiry = %v, want %v", cfg.JWTExpiry, 12)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, "debug")
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %v, want %v", cfg.LogFormat, "text")
	}
}