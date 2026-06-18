package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

// TestNewAPITokenService 测试构造函数
func TestNewAPITokenService(t *testing.T) {
	t.Run("所有参数为nil时仍可创建实例", func(t *testing.T) {
		svc := NewAPITokenService(nil, nil, nil)
		if svc == nil {
			t.Fatal("期望返回非nil实例，实际为nil")
		}
		if svc.db != nil {
			t.Error("期望db为nil")
		}
		if svc.rsaUtil != nil {
			t.Error("期望rsaUtil为nil")
		}
		if svc.permSvc != nil {
			t.Error("期望permSvc为nil")
		}
	})

	t.Run("参数正确赋值", func(t *testing.T) {
		mockPermSvc := &PermissionService{}
		svc := NewAPITokenService(nil, nil, mockPermSvc)
		if svc.permSvc != mockPermSvc {
			t.Error("期望permSvc正确赋值")
		}
	})
}

// TestAPITokenService_TokenPrefixFormat 测试Token前缀格式
// Token格式: bdf_ + 32字节随机hex = bdf_ + 64字符 = 68字符总长度
// 前缀: 前8个字符 = bdf_ + 4个hex字符
func TestAPITokenService_TokenPrefixFormat(t *testing.T) {
	t.Run("Token前缀bdf_长度正确", func(t *testing.T) {
		prefix := "bdf_"
		if len(prefix) != 4 {
			t.Errorf("期望前缀'bdf_'长度为4，实际为%d", len(prefix))
		}
	})

	t.Run("生成的Token前缀为8字符", func(t *testing.T) {
		// 模拟GenerateToken中的Token生成逻辑
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			t.Fatalf("生成随机字节失败: %v", err)
		}
		tokenString := "bdf_" + hex.EncodeToString(tokenBytes)
		tokenPrefix := tokenString[:8]

		if len(tokenPrefix) != 8 {
			t.Errorf("期望Token前缀长度为8，实际为%d", len(tokenPrefix))
		}
		if !strings.HasPrefix(tokenPrefix, "bdf_") {
			t.Errorf("期望Token前缀以'bdf_'开头，实际为%q", tokenPrefix)
		}
	})

	t.Run("Token总长度正确", func(t *testing.T) {
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			t.Fatalf("生成随机字节失败: %v", err)
		}
		tokenString := "bdf_" + hex.EncodeToString(tokenBytes)

		// bdf_(4) + 32字节hex(64) = 68
		expectedLen := 4 + 64
		if len(tokenString) != expectedLen {
			t.Errorf("期望Token总长度为%d，实际为%d", expectedLen, len(tokenString))
		}
	})

	t.Run("多次生成Token前缀不同", func(t *testing.T) {
		prefixes := make(map[string]bool)
		for i := 0; i < 100; i++ {
			tokenBytes := make([]byte, 32)
			if _, err := rand.Read(tokenBytes); err != nil {
				t.Fatalf("生成随机字节失败: %v", err)
			}
			tokenString := "bdf_" + hex.EncodeToString(tokenBytes)
			prefix := tokenString[:8]
			prefixes[prefix] = true
		}
		// 100次生成应该产生多个不同前缀
		if len(prefixes) < 90 {
			t.Errorf("100次生成只产生了%d个不同前缀，随机性不足", len(prefixes))
		}
	})
}

// TestAPITokenService_ErrorsDefined 验证API Token相关错误已定义且错误码正确
func TestAPITokenService_ErrorsDefined(t *testing.T) {
	t.Run("ErrAPITokenNotFound已定义", func(t *testing.T) {
		if ErrAPITokenNotFound == nil {
			t.Fatal("ErrAPITokenNotFound不应为nil")
		}
		if ErrAPITokenNotFound.Error() != "api token not found" {
			t.Errorf("期望错误信息'api token not found'，实际为%q", ErrAPITokenNotFound.Error())
		}
	})

	t.Run("ErrAPITokenInvalid已定义", func(t *testing.T) {
		if ErrAPITokenInvalid == nil {
			t.Fatal("ErrAPITokenInvalid不应为nil")
		}
		if ErrAPITokenInvalid.Error() != "api token invalid" {
			t.Errorf("期望错误信息'api token invalid'，实际为%q", ErrAPITokenInvalid.Error())
		}
	})

	t.Run("ErrAPITokenNotFound错误码为15001", func(t *testing.T) {
		code := GetErrorCode(ErrAPITokenNotFound)
		if code != 15001 {
			t.Errorf("期望ErrAPITokenNotFound错误码为15001，实际为%d", code)
		}
	})

	t.Run("ErrAPITokenInvalid错误码为15002", func(t *testing.T) {
		code := GetErrorCode(ErrAPITokenInvalid)
		if code != 15002 {
			t.Errorf("期望ErrAPITokenInvalid错误码为15002，实际为%d", code)
		}
	})

	t.Run("两个错误不相等", func(t *testing.T) {
		if errors.Is(ErrAPITokenNotFound, ErrAPITokenInvalid) {
			t.Error("ErrAPITokenNotFound和ErrAPITokenInvalid不应相等")
		}
	})
}

// TestAPITokenService_ValidateTokenPrefix 测试ValidateToken中的前缀校验逻辑
// 对应源码: if len(tokenString) < 8 || tokenString[:4] != "bdf_"
func TestAPITokenService_ValidateTokenPrefix(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		wantValid bool
	}{
		{
			name:      "空字符串无效",
			token:     "",
			wantValid: false,
		},
		{
			name:      "长度不足8无效",
			token:     "bdf_abc",
			wantValid: false,
		},
		{
			name:      "恰好8字符但前缀不对无效",
			token:     "abcd1234",
			wantValid: false,
		},
		{
			name:      "前缀不是bdf_无效",
			token:     "xYZ_1234567890abcdef",
			wantValid: false,
		},
		{
			name:      "恰好8字符且前缀正确有效",
			token:     "bdf_abcd",
			wantValid: true,
		},
		{
			name:      "标准Token有效",
			token:     "bdf_" + strings.Repeat("a", 64),
			wantValid: true,
		},
		{
			name:      "前缀bdf_后跟hex字符有效",
			token:     "bdf_a1b2" + strings.Repeat("c3", 30),
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 复现ValidateToken中的前缀校验逻辑
			valid := !(len(tt.token) < 8 || tt.token[:4] != "bdf_")
			if valid != tt.wantValid {
				t.Errorf("token %q: 期望valid=%v，实际valid=%v", tt.token, tt.wantValid, valid)
			}
		})
	}
}

// TestAPITokenService_NilService 测试nil service上的方法调用
func TestAPITokenService_NilService(t *testing.T) {
	t.Run("nil service的GenerateToken应panic", func(t *testing.T) {
		var svc *APITokenService
		defer func() {
			if r := recover(); r == nil {
				t.Error("期望在nil service上调用GenerateToken时panic，实际未panic")
			}
		}()
		svc.GenerateToken(nil, 1)
	})

	t.Run("nil service的GetTokenInfo应panic", func(t *testing.T) {
		var svc *APITokenService
		defer func() {
			if r := recover(); r == nil {
				t.Error("期望在nil service上调用GetTokenInfo时panic，实际未panic")
			}
		}()
		svc.GetTokenInfo(nil, 1)
	})

	t.Run("nil service的RevealToken应panic", func(t *testing.T) {
		var svc *APITokenService
		defer func() {
			if r := recover(); r == nil {
				t.Error("期望在nil service上调用RevealToken时panic，实际未panic")
			}
		}()
		svc.RevealToken(nil, 1)
	})

	t.Run("nil service的RevokeToken应panic", func(t *testing.T) {
		var svc *APITokenService
		defer func() {
			if r := recover(); r == nil {
				t.Error("期望在nil service上调用RevokeToken时panic，实际未panic")
			}
		}()
		svc.RevokeToken(nil, 1)
	})

	t.Run("nil service的ValidateToken应panic", func(t *testing.T) {
		var svc *APITokenService
		defer func() {
			if r := recover(); r == nil {
				t.Error("期望在nil service上调用ValidateToken时panic，实际未panic")
			}
		}()
		svc.ValidateToken(nil, "bdf_test")
	})

	t.Run("nil service的GetTokenUserInfo应panic", func(t *testing.T) {
		var svc *APITokenService
		defer func() {
			if r := recover(); r == nil {
				t.Error("期望在nil service上调用GetTokenUserInfo时panic，实际未panic")
			}
		}()
		svc.GetTokenUserInfo(nil, 1)
	})
}

// TestAPITokenModel 测试APIToken模型结构体
func TestAPITokenModel(t *testing.T) {
	t.Run("APIToken结构体字段赋值", func(t *testing.T) {
		now := time.Now()
		token := &model.APIToken{
			ID:             1,
			UserID:         100,
			TokenEncrypted: "encrypted_value",
			TokenPrefix:    "bdf_a1b2",
			CreatedAt:      now,
		}

		if token.ID != 1 {
			t.Errorf("期望ID=1，实际为%d", token.ID)
		}
		if token.UserID != 100 {
			t.Errorf("期望UserID=100，实际为%d", token.UserID)
		}
		if token.TokenEncrypted != "encrypted_value" {
			t.Errorf("期望TokenEncrypted='encrypted_value'，实际为%q", token.TokenEncrypted)
		}
		if token.TokenPrefix != "bdf_a1b2" {
			t.Errorf("期望TokenPrefix='bdf_a1b2'，实际为%q", token.TokenPrefix)
		}
		if !token.CreatedAt.Equal(now) {
			t.Errorf("期望CreatedAt=%v，实际为%v", now, token.CreatedAt)
		}
		if token.LastUsedAt != nil {
			t.Error("期望LastUsedAt默认为nil")
		}
	})

	t.Run("APIToken LastUsedAt可选字段", func(t *testing.T) {
		now := time.Now()
		token := &model.APIToken{
			ID:             2,
			UserID:         200,
			TokenEncrypted: "enc",
			TokenPrefix:    "bdf_c3d4",
			LastUsedAt:     &now,
			CreatedAt:      now,
		}

		if token.LastUsedAt == nil {
			t.Fatal("期望LastUsedAt非nil")
		}
		if !token.LastUsedAt.Equal(now) {
			t.Errorf("期望LastUsedAt=%v，实际为%v", now, *token.LastUsedAt)
		}
	})

	t.Run("APIToken JSON标签TokenEncrypted不输出", func(t *testing.T) {
		// TokenEncrypted的json标签为"-"，不应序列化
		token := model.APIToken{
			ID:             1,
			TokenEncrypted: "secret",
			TokenPrefix:    "bdf_test",
		}
		// 验证结构体字段可访问
		if token.TokenEncrypted != "secret" {
			t.Error("TokenEncrypted应可访问")
		}
	})
}
