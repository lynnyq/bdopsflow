package config

import (
	"strings"
	"testing"
)

// TestValidate 测试 Config.Validate() 方法的各种场景
func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		jwtSecret string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "empty JWTSecret",
			jwtSecret: "",
			wantErr:   true,
			errSubstr: "JWTSecret 未配置",
		},
		{
			name:      "default JWTSecret",
			jwtSecret: "your-secret-key-change-in-production",
			wantErr:   true,
			errSubstr: "JWTSecret 未配置",
		},
		{
			name:      "short JWTSecret less than 32 chars",
			jwtSecret: "short-key",
			wantErr:   true,
			errSubstr: "JWTSecret 长度不足",
		},
		{
			name:      "JWTSecret exactly 31 chars",
			jwtSecret: strings.Repeat("a", 31),
			wantErr:   true,
			errSubstr: "JWTSecret 长度不足",
		},
		{
			name:      "valid JWTSecret exactly 32 chars",
			jwtSecret: strings.Repeat("a", 32),
			wantErr:   false,
		},
		{
			name:      "valid JWTSecret more than 32 chars",
			jwtSecret: strings.Repeat("b", 64),
			wantErr:   false,
		},
		{
			name:      "valid JWTSecret with special chars",
			jwtSecret: "my-super-secret-jwt-key-32bytes!@#",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				JWTSecret: tt.jwtSecret,
			}
			err := cfg.Validate()

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("expected error to contain %q, got %q", tt.errSubstr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

// TestValidate_DefaultConfigReturnsError 测试默认配置的 Validate 应返回错误
func TestValidate_DefaultConfigReturnsError(t *testing.T) {
	cfg := defaultConfig()
	err := cfg.Validate()
	if err == nil {
		t.Error("expected default config to fail validation, got nil")
	}
}

// TestValidate_ErrorMessages 测试 Validate 错误信息内容
func TestValidate_ErrorMessages(t *testing.T) {
	t.Run("empty secret error message", func(t *testing.T) {
		cfg := &Config{JWTSecret: ""}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for empty JWTSecret")
		}
		if !strings.Contains(err.Error(), "32 字符") {
			t.Errorf("error message should mention '32 字符', got %q", err.Error())
		}
	})

	t.Run("short secret error message includes length", func(t *testing.T) {
		cfg := &Config{JWTSecret: "abc"}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for short JWTSecret")
		}
		if !strings.Contains(err.Error(), "3") {
			t.Errorf("error message should include current length 3, got %q", err.Error())
		}
	})
}

// TestValidate_ValidConfigAfterReload 测试重载后配置的 Validate
func TestValidate_ValidConfigAfterReload(t *testing.T) {
	cfg := defaultConfig()

	// 默认配置应该校验失败
	if err := cfg.Validate(); err == nil {
		t.Error("default config should fail validation")
	}

	// 设置合法的 JWTSecret
	cfg.JWTSecret = strings.Repeat("x", 48)
	if err := cfg.Validate(); err != nil {
		t.Errorf("config with valid JWTSecret should pass validation, got %v", err)
	}
}
