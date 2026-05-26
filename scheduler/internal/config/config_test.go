package config

import (
	"os"
	"reflect"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	expected := &Config{
		HTTPPort:              "8080",
		GRPCPort:              "50051",
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
		AllowRegister:         false,
		CORSAllowOrigins:      []string{},
		LogLevel:              "info",
		LogFormat:             "json",
		SSOEnabled:            false,
		SSOUrl:                "",
		SSOPublicKey:          "",
		SSOTimeout:            0,
		DatasourceCrypto: DatasourceCryptoConfig{
			EncryptionKey:  "change-in-prod-32byte-key1-here1",
			KeySource:      "direct",
			KeyEnvVar:      "BDOPSFLOW_ENCRYPTION_KEY",
			KeyFile:        "",
			AutoRotateDays: 0,
		},
	}

	if cfg.HTTPPort != expected.HTTPPort {
		t.Errorf("HTTPPort = %v, want %v", cfg.HTTPPort, expected.HTTPPort)
	}
	if cfg.GRPCPort != expected.GRPCPort {
		t.Errorf("GRPCPort = %v, want %v", cfg.GRPCPort, expected.GRPCPort)
	}
	if cfg.NodeID != expected.NodeID {
		t.Errorf("NodeID = %v, want %v", cfg.NodeID, expected.NodeID)
	}
	if !reflect.DeepEqual(cfg.RQLiteAddrs, expected.RQLiteAddrs) {
		t.Errorf("RQLiteAddrs = %v, want %v", cfg.RQLiteAddrs, expected.RQLiteAddrs)
	}
	if cfg.RQLiteUser != expected.RQLiteUser {
		t.Errorf("RQLiteUser = %v, want %v", cfg.RQLiteUser, expected.RQLiteUser)
	}
	if cfg.RQLitePass != expected.RQLitePass {
		t.Errorf("RQLitePass = %v, want %v", cfg.RQLitePass, expected.RQLitePass)
	}
	if cfg.RQLiteTLS != expected.RQLiteTLS {
		t.Errorf("RQLiteTLS = %v, want %v", cfg.RQLiteTLS, expected.RQLiteTLS)
	}
	if cfg.RedisAddr != expected.RedisAddr {
		t.Errorf("RedisAddr = %v, want %v", cfg.RedisAddr, expected.RedisAddr)
	}
	if cfg.RedisMode != expected.RedisMode {
		t.Errorf("RedisMode = %v, want %v", cfg.RedisMode, expected.RedisMode)
	}
	if cfg.RedisPassword != expected.RedisPassword {
		t.Errorf("RedisPassword = %v, want %v", cfg.RedisPassword, expected.RedisPassword)
	}
	if cfg.RedisDB != expected.RedisDB {
		t.Errorf("RedisDB = %v, want %v", cfg.RedisDB, expected.RedisDB)
	}
	if cfg.RedisMaster != expected.RedisMaster {
		t.Errorf("RedisMaster = %v, want %v", cfg.RedisMaster, expected.RedisMaster)
	}
	if !reflect.DeepEqual(cfg.RedisSentinelAddrs, expected.RedisSentinelAddrs) {
		t.Errorf("RedisSentinelAddrs = %v, want %v", cfg.RedisSentinelAddrs, expected.RedisSentinelAddrs)
	}
	if cfg.RedisSentinelPassword != expected.RedisSentinelPassword {
		t.Errorf("RedisSentinelPassword = %v, want %v", cfg.RedisSentinelPassword, expected.RedisSentinelPassword)
	}
	if cfg.JWTSecret != expected.JWTSecret {
		t.Errorf("JWTSecret = %v, want %v", cfg.JWTSecret, expected.JWTSecret)
	}
	if cfg.JWTExpiry != expected.JWTExpiry {
		t.Errorf("JWTExpiry = %v, want %v", cfg.JWTExpiry, expected.JWTExpiry)
	}
	if cfg.AllowRegister != expected.AllowRegister {
		t.Errorf("AllowRegister = %v, want %v", cfg.AllowRegister, expected.AllowRegister)
	}
	if !reflect.DeepEqual(cfg.CORSAllowOrigins, expected.CORSAllowOrigins) {
		t.Errorf("CORSAllowOrigins = %v, want %v", cfg.CORSAllowOrigins, expected.CORSAllowOrigins)
	}
	if cfg.LogLevel != expected.LogLevel {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, expected.LogLevel)
	}
	if cfg.LogFormat != expected.LogFormat {
		t.Errorf("LogFormat = %v, want %v", cfg.LogFormat, expected.LogFormat)
	}
	if cfg.SSOEnabled != expected.SSOEnabled {
		t.Errorf("SSOEnabled = %v, want %v", cfg.SSOEnabled, expected.SSOEnabled)
	}
	if cfg.SSOUrl != expected.SSOUrl {
		t.Errorf("SSOUrl = %v, want %v", cfg.SSOUrl, expected.SSOUrl)
	}
	if cfg.SSOPublicKey != expected.SSOPublicKey {
		t.Errorf("SSOPublicKey = %v, want %v", cfg.SSOPublicKey, expected.SSOPublicKey)
	}
	if cfg.SSOTimeout != expected.SSOTimeout {
		t.Errorf("SSOTimeout = %v, want %v", cfg.SSOTimeout, expected.SSOTimeout)
	}
	if !reflect.DeepEqual(cfg.DatasourceCrypto, expected.DatasourceCrypto) {
		t.Errorf("DatasourceCrypto = %+v, want %+v", cfg.DatasourceCrypto, expected.DatasourceCrypto)
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

func TestLoad_AllowRegisterAndCORS(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
app:
  allow_register: true
  cors_allow_origins:
    - "https://example.com"
    - "https://app.example.com"
  node_id: "node-1"
`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config content: %v", err)
	}
	tmpFile.Close()

	cfg := Load(tmpFile.Name())

	if cfg.AllowRegister != true {
		t.Errorf("AllowRegister = %v, want true", cfg.AllowRegister)
	}
	expectedCORS := []string{"https://example.com", "https://app.example.com"}
	if !reflect.DeepEqual(cfg.CORSAllowOrigins, expectedCORS) {
		t.Errorf("CORSAllowOrigins = %v, want %v", cfg.CORSAllowOrigins, expectedCORS)
	}
	if cfg.NodeID != "node-1" {
		t.Errorf("NodeID = %v, want 'node-1'", cfg.NodeID)
	}
}

func TestLoad_SSOConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
sso:
  enabled: true
  url: "https://sso.example.com"
  public_key: "ssh-rsa AAAA..."
  timeout: 30
`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config content: %v", err)
	}
	tmpFile.Close()

	cfg := Load(tmpFile.Name())

	if cfg.SSOEnabled != true {
		t.Errorf("SSOEnabled = %v, want true", cfg.SSOEnabled)
	}
	if cfg.SSOUrl != "https://sso.example.com" {
		t.Errorf("SSOUrl = %v, want 'https://sso.example.com'", cfg.SSOUrl)
	}
	if cfg.SSOPublicKey != "ssh-rsa AAAA..." {
		t.Errorf("SSOPublicKey = %v, want 'ssh-rsa AAAA...'", cfg.SSOPublicKey)
	}
	if cfg.SSOTimeout != 30 {
		t.Errorf("SSOTimeout = %v, want 30", cfg.SSOTimeout)
	}
}

func TestLoad_DatasourceCryptoConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
datasource:
  encryption_key: "my-custom-32byte-encryption-key!!"
  key_source: "env"
  key_env_var: "MY_ENCRYPTION_KEY"
  key_file: "/etc/keys/encryption.key"
  auto_rotate_days: 90
`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config content: %v", err)
	}
	tmpFile.Close()

	cfg := Load(tmpFile.Name())

	if cfg.DatasourceCrypto.EncryptionKey != "my-custom-32byte-encryption-key!!" {
		t.Errorf("DatasourceCrypto.EncryptionKey = %v, want 'my-custom-32byte-encryption-key!!'", cfg.DatasourceCrypto.EncryptionKey)
	}
	if cfg.DatasourceCrypto.KeySource != "env" {
		t.Errorf("DatasourceCrypto.KeySource = %v, want 'env'", cfg.DatasourceCrypto.KeySource)
	}
	if cfg.DatasourceCrypto.KeyEnvVar != "MY_ENCRYPTION_KEY" {
		t.Errorf("DatasourceCrypto.KeyEnvVar = %v, want 'MY_ENCRYPTION_KEY'", cfg.DatasourceCrypto.KeyEnvVar)
	}
	if cfg.DatasourceCrypto.KeyFile != "/etc/keys/encryption.key" {
		t.Errorf("DatasourceCrypto.KeyFile = %v, want '/etc/keys/encryption.key'", cfg.DatasourceCrypto.KeyFile)
	}
	if cfg.DatasourceCrypto.AutoRotateDays != 90 {
		t.Errorf("DatasourceCrypto.AutoRotateDays = %v, want 90", cfg.DatasourceCrypto.AutoRotateDays)
	}
}

func TestLoad_RSAConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
rsa:
  public_key: "/etc/rsa/public.pem"
  private_key: "/etc/rsa/private.pem"
`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config content: %v", err)
	}
	tmpFile.Close()

	cfg := Load(tmpFile.Name())

	if cfg.RSAPublicKey != "/etc/rsa/public.pem" {
		t.Errorf("RSAPublicKey = %v, want '/etc/rsa/public.pem'", cfg.RSAPublicKey)
	}
	if cfg.RSAPrivateKey != "/etc/rsa/private.pem" {
		t.Errorf("RSAPrivateKey = %v, want '/etc/rsa/private.pem'", cfg.RSAPrivateKey)
	}
}

func TestLoad_DefaultsPreservedWhenNotInYAML(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
app:
  http_port: "3000"
`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config content: %v", err)
	}
	tmpFile.Close()

	cfg := Load(tmpFile.Name())

	if cfg.HTTPPort != "3000" {
		t.Errorf("HTTPPort = %v, want '3000'", cfg.HTTPPort)
	}
	if cfg.GRPCPort != "50051" {
		t.Errorf("GRPCPort should default to '50051', got %v", cfg.GRPCPort)
	}
	if cfg.AllowRegister != false {
		t.Errorf("AllowRegister should default to false, got %v", cfg.AllowRegister)
	}
	if cfg.RedisMode != "single" {
		t.Errorf("RedisMode should default to 'single', got %v", cfg.RedisMode)
	}
	if cfg.JWTExpiry != 2 {
		t.Errorf("JWTExpiry should default to 2, got %v", cfg.JWTExpiry)
	}
	if cfg.SSOEnabled != false {
		t.Errorf("SSOEnabled should default to false, got %v", cfg.SSOEnabled)
	}
	if cfg.SSOTimeout != 10 {
		t.Errorf("SSOTimeout should default to 10, got %v", cfg.SSOTimeout)
	}
	if cfg.DatasourceCrypto.KeySource != "direct" {
		t.Errorf("DatasourceCrypto.KeySource should default to 'direct', got %v", cfg.DatasourceCrypto.KeySource)
	}
	if cfg.DatasourceCrypto.AutoRotateDays != 0 {
		t.Errorf("DatasourceCrypto.AutoRotateDays should default to 0, got %v", cfg.DatasourceCrypto.AutoRotateDays)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `
app:
  http_port: "9090"
  invalid yaml content: [
`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config content: %v", err)
	}
	tmpFile.Close()

	cfg := Load(tmpFile.Name())

	if cfg == nil {
		t.Error("Load() should return default config for invalid YAML")
	}
	if cfg.HTTPPort != "8080" {
		t.Errorf("expected default HTTPPort '8080' for invalid YAML, got '%s'", cfg.HTTPPort)
	}
}

func TestDefaultConfig_AllowRegisterFalse(t *testing.T) {
	cfg := defaultConfig()
	if cfg.AllowRegister != false {
		t.Errorf("AllowRegister should be false by default, got %v", cfg.AllowRegister)
	}
}

func TestDefaultConfig_CORSAllowOriginsEmpty(t *testing.T) {
	cfg := defaultConfig()
	if len(cfg.CORSAllowOrigins) != 0 {
		t.Errorf("CORSAllowOrigins should be empty by default, got %v", cfg.CORSAllowOrigins)
	}
}

func TestDefaultConfig_NodeIDEmpty(t *testing.T) {
	cfg := defaultConfig()
	if cfg.NodeID != "" {
		t.Errorf("NodeID should be empty by default, got %v", cfg.NodeID)
	}
}

func TestDefaultConfig_SSODefaults(t *testing.T) {
	cfg := defaultConfig()
	if cfg.SSOEnabled != false {
		t.Errorf("SSOEnabled should be false by default, got %v", cfg.SSOEnabled)
	}
	if cfg.SSOUrl != "" {
		t.Errorf("SSOUrl should be empty by default, got %v", cfg.SSOUrl)
	}
	if cfg.SSOPublicKey != "" {
		t.Errorf("SSOPublicKey should be empty by default, got %v", cfg.SSOPublicKey)
	}
	if cfg.SSOTimeout != 0 {
		t.Errorf("SSOTimeout should be 0 in defaultConfig, got %v", cfg.SSOTimeout)
	}
}

func TestDefaultConfig_DatasourceCryptoDefaults(t *testing.T) {
	cfg := defaultConfig()
	if cfg.DatasourceCrypto.EncryptionKey != "change-in-prod-32byte-key1-here1" {
		t.Errorf("DatasourceCrypto.EncryptionKey default incorrect, got %v", cfg.DatasourceCrypto.EncryptionKey)
	}
	if cfg.DatasourceCrypto.KeySource != "direct" {
		t.Errorf("DatasourceCrypto.KeySource default incorrect, got %v", cfg.DatasourceCrypto.KeySource)
	}
	if cfg.DatasourceCrypto.KeyEnvVar != "BDOPSFLOW_ENCRYPTION_KEY" {
		t.Errorf("DatasourceCrypto.KeyEnvVar default incorrect, got %v", cfg.DatasourceCrypto.KeyEnvVar)
	}
	if cfg.DatasourceCrypto.KeyFile != "" {
		t.Errorf("DatasourceCrypto.KeyFile should be empty by default, got %v", cfg.DatasourceCrypto.KeyFile)
	}
	if cfg.DatasourceCrypto.AutoRotateDays != 0 {
		t.Errorf("DatasourceCrypto.AutoRotateDays should be 0 by default, got %v", cfg.DatasourceCrypto.AutoRotateDays)
	}
}
