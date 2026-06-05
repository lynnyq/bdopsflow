package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig("test-host")
	if cfg.Hostname != "test-host" {
		t.Errorf("expected hostname test-host, got %s", cfg.Hostname)
	}
	if cfg.Capacity != 10 {
		t.Errorf("expected capacity 10, got %d", cfg.Capacity)
	}
	if cfg.Timeout != 30 {
		t.Errorf("expected timeout 30, got %d", cfg.Timeout)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected log level info, got %s", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("expected log format json, got %s", cfg.LogFormat)
	}
}

func TestParseSchedulerAddrs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single address",
			input:    "localhost:50051",
			expected: []string{"localhost:50051"},
		},
		{
			name:     "multiple addresses",
			input:    "localhost:50051, 192.168.1.1:50051,  10.0.0.1:50051  ",
			expected: []string{"localhost:50051", "192.168.1.1:50051", "10.0.0.1:50051"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSchedulerAddrs(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d addresses, got %d", len(tt.expected), len(result))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("expected address %s at index %d, got %s", tt.expected[i], i, result[i])
				}
			}
		})
	}
}

func TestConfigMerge(t *testing.T) {
	cfg := defaultConfig("default-host")
	cfg.Merge("test-executor", 20, "scheduler:50051", []string{"s1:50051", "s2:50051"}, 60, "new-host", "debug", "text")

	if cfg.ExecutorName != "test-executor" {
		t.Errorf("expected executor name test-executor, got %s", cfg.ExecutorName)
	}
	if cfg.Capacity != 20 {
		t.Errorf("expected capacity 20, got %d", cfg.Capacity)
	}
	if cfg.SchedulerAddr != "scheduler:50051" {
		t.Errorf("expected scheduler addr scheduler:50051, got %s", cfg.SchedulerAddr)
	}
	if len(cfg.SchedulerAddrs) != 2 {
		t.Errorf("expected 2 scheduler addrs, got %d", len(cfg.SchedulerAddrs))
	}
	if cfg.Timeout != 60 {
		t.Errorf("expected timeout 60, got %d", cfg.Timeout)
	}
	if cfg.Hostname != "new-host" {
		t.Errorf("expected hostname new-host, got %s", cfg.Hostname)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("expected log format text, got %s", cfg.LogFormat)
	}
}

func TestGetSchedulerAddresses(t *testing.T) {
	cfg := defaultConfig("test-host")
	if len(cfg.GetSchedulerAddresses()) != 0 {
		t.Errorf("expected 0 scheduler addresses, got %d", len(cfg.GetSchedulerAddresses()))
	}

	cfg.SchedulerAddr = "single:50051"
	addrs := cfg.GetSchedulerAddresses()
	if len(addrs) != 1 || addrs[0] != "single:50051" {
		t.Errorf("expected [single:50051], got %v", addrs)
	}

	cfg.SchedulerAddrs = []string{"multi1:50051", "multi2:50051"}
	addrs = cfg.GetSchedulerAddresses()
	if len(addrs) != 2 {
		t.Errorf("expected 2 scheduler addresses, got %d", len(addrs))
	}
	if addrs[0] != "multi1:50051" || addrs[1] != "multi2:50051" {
		t.Errorf("expected [multi1:50051 multi2:50051], got %v", addrs)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "empty config",
			cfg:     defaultConfig("test-host"),
			wantErr: true,
		},
		{
			name: "only executor name",
			cfg: &Config{
				ExecutorName: "test-exec",
			},
			wantErr: true,
		},
		{
			name: "valid config",
			cfg: &Config{
				ExecutorName:  "test-exec",
				SchedulerAddr: "scheduler:50051",
			},
			wantErr: false,
		},
		{
			name: "valid with multiple addrs",
			cfg: &Config{
				ExecutorName:   "test-exec",
				SchedulerAddrs: []string{"s1:50051", "s2:50051"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequiredError(t *testing.T) {
	err := &RequiredError{Field: "test-field"}
	expected := "test-field is required"
	if err.Error() != expected {
		t.Errorf("expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	cfg := Load("/non/existent/file.yaml")
	if cfg == nil {
		t.Fatalf("expected config, got nil")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	cfg := Load(tmpFile.Name())
	if cfg == nil {
		t.Fatalf("expected config, got nil")
	}
}

func TestMergeNoChanges(t *testing.T) {
	cfg := defaultConfig("test-host")
	original := *cfg

	cfg.Merge("", 0, "", nil, 0, "", "", "")

	if cfg.ExecutorName != original.ExecutorName {
		t.Errorf("executor name should not change")
	}
	if cfg.Capacity != original.Capacity {
		t.Errorf("capacity should not change")
	}
	if cfg.SchedulerAddr != original.SchedulerAddr {
		t.Errorf("scheduler addr should not change")
	}
	if cfg.Timeout != original.Timeout {
		t.Errorf("timeout should not change")
	}
	if cfg.Hostname != original.Hostname {
		t.Errorf("hostname should not change")
	}
	if cfg.LogLevel != original.LogLevel {
		t.Errorf("log level should not change")
	}
	if cfg.LogFormat != original.LogFormat {
		t.Errorf("log format should not change")
	}
}

func TestDefaultConfigEmptyHostname(t *testing.T) {
	cfg := defaultConfig("")
	if cfg.Hostname != "localhost" {
		t.Errorf("expected hostname localhost for empty input, got %s", cfg.Hostname)
	}
}
