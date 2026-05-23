package config

import (
	"os"
	"reflect"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	expected := &Config{
		HTTPPort:       "8080",
		GRPCPort:       "50051",
		RQLiteAddrs:    []string{"http://localhost:4001"},
		RQLiteUser:     "",
		RQLitePass:     "",
		RQLiteTLS:      false,
		RedisMode:      "single",
		RedisAddr:      "localhost:6379",
		RedisPassword:  "",
		RedisDB:        0,
		RedisMaster:    "mymaster",
		RedisSentinelAddrs: []string{},
		RedisSentinelPassword: "",
		JWTSecret:      "your-secret-key-change-in-production",
		JWTExpiry:      24,
		LogLevel:       "info",
		LogFormat:      "json",
	}

	if cfg.HTTPPort != expected.HTTPPort {
		t.Errorf("HTTPPort = %v, want %v", cfg.HTTPPort, expected.HTTPPort)
	}
	if cfg.GRPCPort != expected.GRPCPort {
		t.Errorf("GRPCPort = %v, want %v", cfg.GRPCPort, expected.GRPCPort)
	}
	if !reflect.DeepEqual(cfg.RQLiteAddrs, expected.RQLiteAddrs) {
		t.Errorf("RQLiteAddrs = %v, want %v", cfg.RQLiteAddrs, expected.RQLiteAddrs)
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
  rqlite_addrs:
    - "http://rqlite1:4001"
    - "http://rqlite2:4001"
  rqlite_user: "admin"
  rqlite_password: "secret"
  rqlite_tls: true

redis:
  mode: "sentinel"
  master_name: "mymaster"
  sentinel_addrs:
    - "sentinel1:26379"
    - "sentinel2:26379"
  sentinel_password: "sentinel-pass"
  password: "redis-pass"
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
	expectedRQLiteAddrs := []string{"http://rqlite1:4001", "http://rqlite2:4001"}
	if !reflect.DeepEqual(cfg.RQLiteAddrs, expectedRQLiteAddrs) {
		t.Errorf("RQLiteAddrs = %v, want %v", cfg.RQLiteAddrs, expectedRQLiteAddrs)
	}
	if cfg.RQLiteUser != "admin" {
		t.Errorf("RQLiteUser = %v, want %v", cfg.RQLiteUser, "admin")
	}
	if cfg.RQLitePass != "secret" {
		t.Errorf("RQLitePass = %v, want %v", cfg.RQLitePass, "secret")
	}
	if cfg.RQLiteTLS != true {
		t.Errorf("RQLiteTLS = %v, want %v", cfg.RQLiteTLS, true)
	}
	if cfg.RedisMode != "sentinel" {
		t.Errorf("RedisMode = %v, want %v", cfg.RedisMode, "sentinel")
	}
	if cfg.RedisMaster != "mymaster" {
		t.Errorf("RedisMaster = %v, want %v", cfg.RedisMaster, "mymaster")
	}
	expectedSentinelAddrs := []string{"sentinel1:26379", "sentinel2:26379"}
	if !reflect.DeepEqual(cfg.RedisSentinelAddrs, expectedSentinelAddrs) {
		t.Errorf("RedisSentinelAddrs = %v, want %v", cfg.RedisSentinelAddrs, expectedSentinelAddrs)
	}
	if cfg.RedisSentinelPassword != "sentinel-pass" {
		t.Errorf("RedisSentinelPassword = %v, want %v", cfg.RedisSentinelPassword, "sentinel-pass")
	}
	if cfg.RedisPassword != "redis-pass" {
		t.Errorf("RedisPassword = %v, want %v", cfg.RedisPassword, "redis-pass")
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
