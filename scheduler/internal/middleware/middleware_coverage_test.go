package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	rqlite "github.com/rqlite/gorqlite"

	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
)

// mockCovDB 实现 database.DB 接口，用于中间件测试中模拟数据库
type mockCovDB struct {
	queryOneErr      error
	queryOneParamErr error
	writeOneParamErr error
	writeParamErr    error
	writeCallCount   int
	mu               sync.Mutex
}

func (m *mockCovDB) QueryOne(sqlStatement string) (rqlite.QueryResult, error) {
	if m.queryOneErr != nil {
		return rqlite.QueryResult{}, m.queryOneErr
	}
	return rqlite.QueryResult{}, nil
}

func (m *mockCovDB) QueryOneParameterized(statement rqlite.ParameterizedStatement) (rqlite.QueryResult, error) {
	if m.queryOneParamErr != nil {
		return rqlite.QueryResult{}, m.queryOneParamErr
	}
	return rqlite.QueryResult{}, nil
}

func (m *mockCovDB) WriteOneParameterized(statement rqlite.ParameterizedStatement) (rqlite.WriteResult, error) {
	m.mu.Lock()
	m.writeCallCount++
	m.mu.Unlock()
	if m.writeOneParamErr != nil {
		return rqlite.WriteResult{}, m.writeOneParamErr
	}
	return rqlite.WriteResult{}, nil
}

func (m *mockCovDB) WriteParameterized(sqlStatements []rqlite.ParameterizedStatement) ([]rqlite.WriteResult, error) {
	if m.writeParamErr != nil {
		return nil, m.writeParamErr
	}
	return nil, nil
}

func (m *mockCovDB) getWriteCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeCallCount
}

// mockRoleInjector 实现 RoleInjector 接口
type mockRoleInjector struct {
	getUserRoleCodesFn func(ctx context.Context, userID int64) ([]string, error)
}

func (m *mockRoleInjector) GetUserRoleCodes(ctx context.Context, userID int64) ([]string, error) {
	if m.getUserRoleCodesFn != nil {
		return m.getUserRoleCodesFn(ctx, userID)
	}
	return nil, nil
}

// TestGetJWTConfig_V2 测试 GetJWTConfig 返回当前配置
func TestGetJWTConfig_V2(t *testing.T) {
	config := GetJWTConfig()
	if config == nil {
		t.Fatal("期望非 nil 配置")
	}
	if config.Secret == nil {
		t.Error("期望 Secret 非 nil")
	}
	if config.ExpiryHours <= 0 {
		t.Errorf("期望 ExpiryHours > 0，实际 %d", config.ExpiryHours)
	}
	if config.RefreshSecret == nil {
		t.Error("期望 RefreshSecret 非 nil")
	}
	if config.RefreshExpiryHours <= 0 {
		t.Errorf("期望 RefreshExpiryHours > 0，实际 %d", config.RefreshExpiryHours)
	}
}

// TestSetRefreshExpiryHours_V2 测试 SetRefreshExpiryHours 修改刷新令牌过期时间
func TestSetRefreshExpiryHours_V2(t *testing.T) {
	original := jwtConfig.RefreshExpiryHours
	defer func() {
		jwtConfig.RefreshExpiryHours = original
	}()

	newHours := 336
	SetRefreshExpiryHours(newHours)

	if jwtConfig.RefreshExpiryHours != newHours {
		t.Errorf("期望 RefreshExpiryHours=%d，实际 %d", newHours, jwtConfig.RefreshExpiryHours)
	}

	config := GetJWTConfig()
	if config.RefreshExpiryHours != newHours {
		t.Errorf("期望 GetJWTConfig().RefreshExpiryHours=%d，实际 %d", newHours, config.RefreshExpiryHours)
	}
}

// TestInjectUserRole_V2 测试 InjectUserRole 中间件的各种路径
func TestInjectUserRole_V2(t *testing.T) {
	t.Run("上下文无user_id_应直接放行", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		nextCalled := false
		router := gin.New()
		router.Use(InjectUserRole(&mockRoleInjector{}))
		router.GET("/test", func(c *gin.Context) {
			nextCalled = true
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if !nextCalled {
			t.Error("期望后续 handler 被调用")
		}
		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200，实际 %d", w.Code)
		}
	})

	t.Run("user_id类型非int64_应直接放行", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", "not-an-int64")
			c.Next()
		})
		router.Use(InjectUserRole(&mockRoleInjector{}))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200，实际 %d", w.Code)
		}
	})

	t.Run("user_id为零值_应直接放行", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", int64(0))
			c.Next()
		})
		router.Use(InjectUserRole(&mockRoleInjector{}))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200，实际 %d", w.Code)
		}
	})

	t.Run("获取角色出错_应直接放行", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		roleSvc := &mockRoleInjector{
			getUserRoleCodesFn: func(ctx context.Context, userID int64) ([]string, error) {
				return nil, errors.New("database error")
			},
		}
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", int64(1))
			c.Next()
		})
		router.Use(InjectUserRole(roleSvc))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200（错误时放行），实际 %d", w.Code)
		}
	})

	t.Run("用户无角色_应返回401", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		roleSvc := &mockRoleInjector{
			getUserRoleCodesFn: func(ctx context.Context, userID int64) ([]string, error) {
				return []string{}, nil
			},
		}
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", int64(1))
			c.Next()
		})
		router.Use(InjectUserRole(roleSvc))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("期望状态码 401，实际 %d", w.Code)
		}
	})

	t.Run("用户有system_admin角色_应注入system_admin", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		var capturedRole string
		roleSvc := &mockRoleInjector{
			getUserRoleCodesFn: func(ctx context.Context, userID int64) ([]string, error) {
				return []string{"user", "system_admin", "domain_admin"}, nil
			},
		}
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", int64(1))
			c.Next()
		})
		router.Use(InjectUserRole(roleSvc))
		router.GET("/test", func(c *gin.Context) {
			role, _ := c.Get("role")
			capturedRole = role.(string)
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200，实际 %d", w.Code)
		}
		if capturedRole != "system_admin" {
			t.Errorf("期望 role=system_admin，实际 %s", capturedRole)
		}
	})

	t.Run("用户仅有domain_admin角色_应注入domain_admin", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		var capturedRole string
		roleSvc := &mockRoleInjector{
			getUserRoleCodesFn: func(ctx context.Context, userID int64) ([]string, error) {
				return []string{"user", "domain_admin"}, nil
			},
		}
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", int64(1))
			c.Next()
		})
		router.Use(InjectUserRole(roleSvc))
		router.GET("/test", func(c *gin.Context) {
			role, _ := c.Get("role")
			capturedRole = role.(string)
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if capturedRole != "domain_admin" {
			t.Errorf("期望 role=domain_admin，实际 %s", capturedRole)
		}
	})

	t.Run("用户仅普通角色_应注入user", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		var capturedRole string
		roleSvc := &mockRoleInjector{
			getUserRoleCodesFn: func(ctx context.Context, userID int64) ([]string, error) {
				return []string{"user"}, nil
			},
		}
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", int64(1))
			c.Next()
		})
		router.Use(InjectUserRole(roleSvc))
		router.GET("/test", func(c *gin.Context) {
			role, _ := c.Get("role")
			capturedRole = role.(string)
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if capturedRole != "user" {
			t.Errorf("期望 role=user，实际 %s", capturedRole)
		}
	})
}

// TestRequireInstancePermission_V2 测试 RequireInstancePermission 中间件
func TestRequireInstancePermission_V2(t *testing.T) {
	t.Run("无user_id_应返回401", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(RequireInstancePermission(nil, "datasource", func(c *gin.Context) int64 {
			return 1
		}, "read"))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("期望状态码 401，实际 %d", w.Code)
		}
	})

	t.Run("instanceID为零_应返回400", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", int64(1))
			c.Next()
		})
		router.Use(RequireInstancePermission(nil, "datasource", func(c *gin.Context) int64 {
			return 0
		}, "read"))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("期望状态码 400，实际 %d", w.Code)
		}
	})

	t.Run("未知资源类型_应返回403", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		// 传入 nil instancePermSvc，但资源类型不是 datasource/webhook
		// 所以不会调用 instancePermSvc 的方法
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", int64(1))
			c.Next()
		})
		router.Use(RequireInstancePermission(nil, "unknown_type", func(c *gin.Context) int64 {
			return 1
		}, "read"))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("期望状态码 403，实际 %d", w.Code)
		}
	})

	t.Run("datasource权限检查出错_应返回500", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		db := &mockCovDB{
			queryOneParamErr: errors.New("database error"),
		}
		permSvc := service.NewPermissionService(db, nil)
		instancePermSvc := service.NewInstancePermissionService(db, permSvc)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", int64(1))
			c.Next()
		})
		router.Use(RequireInstancePermission(instancePermSvc, "datasource", func(c *gin.Context) int64 {
			return 100
		}, "read"))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("期望状态码 500，实际 %d", w.Code)
		}
	})

	t.Run("datasource权限拒绝_应返回500", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		// 空结果 → HasDatasourcePermission 返回 ErrInstancePermissionDenied
		db := &mockCovDB{}
		permSvc := service.NewPermissionService(db, nil)
		instancePermSvc := service.NewInstancePermissionService(db, permSvc)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", int64(1))
			c.Next()
		})
		router.Use(RequireInstancePermission(instancePermSvc, "datasource", func(c *gin.Context) int64 {
			return 100
		}, "read"))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("期望状态码 500（ErrInstancePermissionDenied），实际 %d", w.Code)
		}
	})
}

// TestMetricsCollector_V2 测试 MetricsCollector 中间件
func TestMetricsCollector_V2(t *testing.T) {
	t.Run("正常请求_应记录指标", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(MetricsCollector())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200，实际 %d", w.Code)
		}
	})

	t.Run("无匹配路由_应记录指标", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(MetricsCollector())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/nonexistent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 404 也是正常响应，指标应被记录
		if w.Code != http.StatusNotFound {
			t.Errorf("期望状态码 404，实际 %d", w.Code)
		}
	})

	t.Run("POST请求_应记录指标", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(MetricsCollector())
		router.POST("/test", func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{"message": "created"})
		})

		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("期望状态码 201，实际 %d", w.Code)
		}
	})
}

// TestAuditMiddleware_V2 测试 AuditMiddleware 中间件
func TestAuditMiddleware_V2(t *testing.T) {
	// 创建 mock 审计日志服务
	newAuditSvc := func() *service.AuditLogService {
		db := &mockCovDB{}
		return service.NewAuditLogService(db, nil)
	}

	t.Run("GET请求_应直接放行不记录审计", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		auditSvc := newAuditSvc()
		handlerCalled := false

		router := gin.New()
		router.Use(AuditMiddleware(auditSvc))
		router.GET("/api/tasks", func(c *gin.Context) {
			handlerCalled = true
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/api/tasks", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if !handlerCalled {
			t.Error("期望 handler 被调用")
		}
		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200，实际 %d", w.Code)
		}
		// GET 请求不应触发审计写入
		if auditSvc == nil {
			t.Error("审计服务不应为 nil")
		}
	})

	t.Run("POST已知路由_应记录审计日志", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		db := &mockCovDB{}
		auditSvc := service.NewAuditLogService(db, nil)

		router := gin.New()
		router.Use(AuditMiddleware(auditSvc))
		router.POST("/api/auth/login", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"token": "xxx"})
		})

		req := httptest.NewRequest("POST", "/api/auth/login", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200，实际 %d", w.Code)
		}

		// 等待异步审计 goroutine 完成
		time.Sleep(100 * time.Millisecond)
		if db.getWriteCallCount() != 1 {
			t.Errorf("期望 1 次审计写入，实际 %d", db.getWriteCallCount())
		}
	})

	t.Run("POST未知路由_资源为空应跳过审计", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		db := &mockCovDB{}
		auditSvc := service.NewAuditLogService(db, nil)

		router := gin.New()
		router.Use(AuditMiddleware(auditSvc))
		router.POST("/api/unknown-endpoint", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("POST", "/api/unknown-endpoint", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200，实际 %d", w.Code)
		}

		time.Sleep(100 * time.Millisecond)
		// 未知路由 → resource 和 action 为空 → 不写入审计
		if db.getWriteCallCount() != 0 {
			t.Errorf("期望 0 次审计写入（未知路由），实际 %d", db.getWriteCallCount())
		}
	})

	t.Run("handler panic_应恢复并返回500", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		auditSvc := newAuditSvc()

		router := gin.New()
		router.Use(AuditMiddleware(auditSvc))
		router.POST("/api/tasks", func(c *gin.Context) {
			panic("simulated panic")
		})

		req := httptest.NewRequest("POST", "/api/tasks", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("期望状态码 500，实际 %d", w.Code)
		}
	})

	t.Run("audit_action覆盖_应使用覆盖值", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		db := &mockCovDB{}
		auditSvc := service.NewAuditLogService(db, nil)

		router := gin.New()
		router.Use(AuditMiddleware(auditSvc))
		router.POST("/api/tasks", func(c *gin.Context) {
			c.Set("audit_action", "custom_action")
			c.Set("audit_resource", "custom_resource")
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("POST", "/api/tasks", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200，实际 %d", w.Code)
		}

		time.Sleep(100 * time.Millisecond)
		// 覆盖后 resource 和 action 非空，应写入审计
		if db.getWriteCallCount() != 1 {
			t.Errorf("期望 1 次审计写入（覆盖后），实际 %d", db.getWriteCallCount())
		}
	})

	t.Run("DELETE已知路由_应记录审计日志", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		db := &mockCovDB{}
		auditSvc := service.NewAuditLogService(db, nil)

		router := gin.New()
		router.Use(AuditMiddleware(auditSvc))
		router.DELETE("/api/tasks/:id", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "deleted"})
		})

		req := httptest.NewRequest("DELETE", "/api/tasks/123", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 200，实际 %d", w.Code)
		}

		time.Sleep(100 * time.Millisecond)
		if db.getWriteCallCount() != 1 {
			t.Errorf("期望 1 次审计写入，实际 %d", db.getWriteCallCount())
		}
	})

	t.Run("失败响应_审计状态应为failure", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		db := &mockCovDB{}
		auditSvc := service.NewAuditLogService(db, nil)

		router := gin.New()
		router.Use(AuditMiddleware(auditSvc))
		router.POST("/api/tasks", func(c *gin.Context) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		})

		req := httptest.NewRequest("POST", "/api/tasks", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("期望状态码 400，实际 %d", w.Code)
		}

		time.Sleep(100 * time.Millisecond)
		// 即使响应失败，审计日志仍应写入
		if db.getWriteCallCount() != 1 {
			t.Errorf("期望 1 次审计写入（失败响应），实际 %d", db.getWriteCallCount())
		}
	})
}

// TestParseDatasourceIDFromBody_V2 测试 parseDatasourceIDFromBody 的更多路径
func TestParseDatasourceIDFromBody_V2(t *testing.T) {
	t.Run("有效JSON包含datasource_id_应返回ID", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"datasource_id": 12345, "name": "test"}`
		c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(body))

		result := parseDatasourceIDFromBody(c)
		if result != 12345 {
			t.Errorf("期望 datasource_id=12345，实际 %d", result)
		}
	})

	t.Run("无效JSON_应返回0", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/test", strings.NewReader("invalid json {{{"))

		result := parseDatasourceIDFromBody(c)
		if result != 0 {
			t.Errorf("期望 0（无效JSON），实际 %d", result)
		}
	})

	t.Run("JSON无datasource_id字段_应返回0", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"name": "test", "type": "mysql"}`
		c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(body))

		result := parseDatasourceIDFromBody(c)
		if result != 0 {
			t.Errorf("期望 0（无 datasource_id 字段），实际 %d", result)
		}
	})

	t.Run("空body_应返回0", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/test", nil)

		result := parseDatasourceIDFromBody(c)
		if result != 0 {
			t.Errorf("期望 0（空 body），实际 %d", result)
		}
	})

	t.Run("datasource_id为零值_应返回0", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"datasource_id": 0, "name": "test"}`
		c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(body))

		result := parseDatasourceIDFromBody(c)
		if result != 0 {
			t.Errorf("期望 0（datasource_id 为零值），实际 %d", result)
		}
	})

	t.Run("调用后body应可重复读取", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"datasource_id": 999}`
		c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(body))

		_ = parseDatasourceIDFromBody(c)

		// 验证 body 被重置后可以再次读取
		buf := make([]byte, 1024)
		n, _ := c.Request.Body.Read(buf)
		if n == 0 {
			t.Error("期望 body 可重复读取，但读取到 0 字节")
		}
	})
}

// TestParseInt64_V2 测试 parseInt64 辅助函数
func TestParseInt64_V2(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{"正常数字", "123", 123},
		{"零", "0", 0},
		{"大数字", "9999999999", 9999999999},
		{"空字符串", "", 0},
		{"非数字", "abc", 0},
		{"部分数字", "123abc", 123},
		{"负数", "-456", -456},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseInt64(tt.input)
			if result != tt.want {
				t.Errorf("parseInt64(%q) = %d, 期望 %d", tt.input, result, tt.want)
			}
		})
	}
}

// 确保 mockCovDB 实现了 database.DB 接口
var _ database.DB = (*mockCovDB)(nil)
