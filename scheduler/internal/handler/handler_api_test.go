package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// newTestQueryHandler 创建用于测试的 QueryHandler（依赖为 nil，仅测试参数校验路径）
func newTestQueryHandler() *QueryHandler {
	return &QueryHandler{
		registry: NewQueryRegistry(),
	}
}

// newTestQueryHandlerWithRegistry 创建带自定义 registry 的 QueryHandler
func newTestQueryHandlerWithRegistry(registry *QueryRegistry) *QueryHandler {
	return &QueryHandler{
		registry: registry,
	}
}

// setupQueryContext 创建带 JSON 请求体的测试 gin.Context
func setupQueryContext(method, path string, body interface{}) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var req *http.Request
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	c.Request = req
	return c, w
}

// setupQueryContextWithParams 创建带 URL 参数和可选请求体的测试 gin.Context
func setupQueryContextWithParams(method, path string, params gin.Params, body interface{}) (*gin.Context, *httptest.ResponseRecorder) {
	c, w := setupQueryContext(method, path, body)
	c.Params = params
	return c, w
}

// assertResponseCode 断言响应码
func assertResponseCode(t *testing.T, w *httptest.ResponseRecorder, expectedCode int) {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v, body: %s", err, w.Body.String())
	}
	code, ok := resp["code"].(float64)
	if !ok {
		t.Fatalf("expected code to be a number, got %T: %v", resp["code"], resp["code"])
	}
	if int(code) != expectedCode {
		t.Errorf("expected code %d, got %d, body: %s", expectedCode, int(code), w.Body.String())
	}
}

// === QueryHandler.Execute 参数校验测试 ===

func TestQueryHandler_Execute_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestQueryHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/query/execute", bytes.NewReader([]byte("invalid json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Execute(c)

	assertResponseCode(t, w, CodeBadRequest)
}

func TestQueryHandler_Execute_MissingRequiredFields(t *testing.T) {
	h := newTestQueryHandler()

	tests := []struct {
		name string
		body interface{}
	}{
		{
			name: "empty body",
			body: nil,
		},
		{
			name: "missing datasource_id",
			body: map[string]string{"sql": "SELECT 1"},
		},
		{
			name: "missing sql",
			body: map[string]interface{}{"datasource_id": 1},
		},
		{
			name: "both missing",
			body: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupQueryContext(http.MethodPost, "/api/v1/query/execute", tt.body)
			h.Execute(c)
			assertResponseCode(t, w, CodeBadRequest)
		})
	}
}

// === QueryHandler.GetMetadata 参数校验测试 ===

func TestQueryHandler_GetMetadata_InvalidID(t *testing.T) {
	h := newTestQueryHandler()

	c, w := setupQueryContextWithParams(http.MethodGet, "/api/v1/query/metadata/abc", gin.Params{
		{Key: "id", Value: "abc"},
	}, nil)

	h.GetMetadata(c)

	assertResponseCode(t, w, CodeBadRequest)
}

func TestQueryHandler_GetMetadata_ZeroID(t *testing.T) {
	h := newTestQueryHandler()

	c, w := setupQueryContextWithParams(http.MethodGet, "/api/v1/query/metadata/0", gin.Params{
		{Key: "id", Value: "0"},
	}, nil)

	// 0 是有效整数，会通过 ID 校验进入后续 service 调用（service 为 nil 会 panic）
	// 因此这里只验证 ID 解析成功（不返回 BadRequest）
	defer func() {
		if r := recover(); r != nil {
			// panic 说明 ID 解析成功并进入了 service 调用，这是预期行为
		}
	}()
	h.GetMetadata(c)

	// 不应该返回 BadRequest（ID 解析成功）
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if code, ok := resp["code"].(float64); ok && int(code) == CodeBadRequest {
			t.Errorf("ID=0 不应返回 BadRequest")
		}
	}
}

func TestQueryHandler_GetMetadata_InvalidLevel(t *testing.T) {
	h := newTestQueryHandler()

	// 由于 service 为 nil，我们无法测试完整的 level 校验路径
	// 但可以验证无效 ID 的快速失败路径
	c, w := setupQueryContextWithParams(http.MethodGet, "/api/v1/query/metadata/invalid", gin.Params{
		{Key: "id", Value: "invalid"},
	}, nil)

	h.GetMetadata(c)

	assertResponseCode(t, w, CodeBadRequest)
}

// === QueryHandler.ExportCSV 参数校验测试 ===

func TestQueryHandler_ExportCSV_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestQueryHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/query/export", bytes.NewReader([]byte("invalid")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ExportCSV(c)

	assertResponseCode(t, w, CodeBadRequest)
}

func TestQueryHandler_ExportCSV_MissingRequiredFields(t *testing.T) {
	h := newTestQueryHandler()

	tests := []struct {
		name string
		body interface{}
	}{
		{
			name: "empty body",
			body: nil,
		},
		{
			name: "missing datasource_id",
			body: map[string]string{"sql": "SELECT 1"},
		},
		{
			name: "missing sql",
			body: map[string]interface{}{"datasource_id": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupQueryContext(http.MethodPost, "/api/v1/query/export", tt.body)
			h.ExportCSV(c)
			assertResponseCode(t, w, CodeBadRequest)
		})
	}
}

// === QueryHandler.DeleteQueryHistory 参数校验测试 ===

func TestQueryHandler_DeleteQueryHistory_InvalidID(t *testing.T) {
	h := newTestQueryHandler()

	tests := []struct {
		name  string
		idVal string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupQueryContextWithParams(http.MethodDelete, "/api/v1/query/history/"+tt.idVal, gin.Params{
				{Key: "id", Value: tt.idVal},
			}, nil)
			h.DeleteQueryHistory(c)
			assertResponseCode(t, w, CodeBadRequest)
		})
	}
}

// === QueryHandler.BatchDeleteQueryHistory 参数校验测试 ===

func TestQueryHandler_BatchDeleteQueryHistory_MissingIDs(t *testing.T) {
	h := newTestQueryHandler()

	tests := []struct {
		name string
		body interface{}
	}{
		{
			name: "empty body",
			body: nil,
		},
		{
			name: "missing ids field",
			body: map[string]string{"other": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupQueryContext(http.MethodPost, "/api/v1/query/history/batch-delete", tt.body)
			h.BatchDeleteQueryHistory(c)
			assertResponseCode(t, w, CodeBadRequest)
		})
	}
}

func TestQueryHandler_BatchDeleteQueryHistory_EmptyIDs(t *testing.T) {
	h := newTestQueryHandler()

	c, w := setupQueryContext(http.MethodPost, "/api/v1/query/history/batch-delete", map[string]interface{}{
		"ids": []int64{},
	})

	h.BatchDeleteQueryHistory(c)

	assertResponseCode(t, w, CodeBadRequest)
}

// === QueryHandler.SaveSQL 参数校验测试 ===

func TestQueryHandler_SaveSQL_MissingRequiredFields(t *testing.T) {
	h := newTestQueryHandler()

	tests := []struct {
		name string
		body interface{}
	}{
		{
			name: "empty body",
			body: nil,
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"datasource_id": 1,
				"sql_text":      "SELECT 1",
			},
		},
		{
			name: "missing datasource_id",
			body: map[string]interface{}{
				"name":     "test",
				"sql_text": "SELECT 1",
			},
		},
		{
			name: "missing sql_text",
			body: map[string]interface{}{
				"name":          "test",
				"datasource_id": 1,
			},
		},
		{
			name: "all missing",
			body: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupQueryContext(http.MethodPost, "/api/v1/query/saved-sql", tt.body)
			h.SaveSQL(c)
			assertResponseCode(t, w, CodeBadRequest)
		})
	}
}

// === QueryHandler.UpdateSavedSQL 参数校验测试（补充已有测试） ===

func TestQueryHandler_UpdateSavedSQL_InvalidID(t *testing.T) {
	h := newTestQueryHandler()

	tests := []struct {
		name  string
		idVal string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupQueryContextWithParams(http.MethodPut, "/api/v1/query/saved-sql/"+tt.idVal, gin.Params{
				{Key: "id", Value: tt.idVal},
			}, nil)
			h.UpdateSavedSQL(c)
			assertResponseCode(t, w, CodeBadRequest)
		})
	}
}

// === QueryHandler.DeleteSavedSQL 参数校验测试 ===

func TestQueryHandler_DeleteSavedSQL_InvalidID(t *testing.T) {
	h := newTestQueryHandler()

	tests := []struct {
		name  string
		idVal string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupQueryContextWithParams(http.MethodDelete, "/api/v1/query/saved-sql/"+tt.idVal, gin.Params{
				{Key: "id", Value: tt.idVal},
			}, nil)
			h.DeleteSavedSQL(c)
			assertResponseCode(t, w, CodeBadRequest)
		})
	}
}

// === QueryHandler.GetResult 参数校验测试 ===

func TestQueryHandler_GetResult_EmptyQueryID(t *testing.T) {
	h := newTestQueryHandler()

	c, w := setupQueryContextWithParams(http.MethodGet, "/api/v1/query/result/", gin.Params{
		{Key: "query_id", Value: ""},
	}, nil)

	h.GetResult(c)

	assertResponseCode(t, w, CodeBadRequest)
}

func TestQueryHandler_GetResult_NotFound(t *testing.T) {
	h := newTestQueryHandler()

	c, w := setupQueryContextWithParams(http.MethodGet, "/api/v1/query/result/q_nonexistent", gin.Params{
		{Key: "query_id", Value: "q_nonexistent"},
	}, nil)

	h.GetResult(c)

	assertResponseCode(t, w, CodeNotFound)
}

// === QueryHandler.StreamResult 参数校验测试 ===

func TestQueryHandler_StreamResult_EmptyQueryID(t *testing.T) {
	h := newTestQueryHandler()

	c, w := setupQueryContextWithParams(http.MethodGet, "/api/v1/query/stream/", gin.Params{
		{Key: "query_id", Value: ""},
	}, nil)

	h.StreamResult(c)

	assertResponseCode(t, w, CodeBadRequest)
}

// === QueryHandler.Cancel 参数校验测试 ===

func TestQueryHandler_Cancel_EmptyQueryID(t *testing.T) {
	h := newTestQueryHandler()

	c, w := setupQueryContextWithParams(http.MethodPost, "/api/v1/query/cancel/", gin.Params{
		{Key: "query_id", Value: ""},
	}, nil)

	h.Cancel(c)

	assertResponseCode(t, w, CodeBadRequest)
}

func TestQueryHandler_Cancel_NotFound(t *testing.T) {
	h := newTestQueryHandler()

	c, w := setupQueryContextWithParams(http.MethodPost, "/api/v1/query/cancel/q_nonexistent", gin.Params{
		{Key: "query_id", Value: "q_nonexistent"},
	}, nil)

	h.Cancel(c)

	assertResponseCode(t, w, CodeQueryError)
}

func TestQueryHandler_Cancel_AlreadyCompleted(t *testing.T) {
	registry := NewQueryRegistry()
	queryID := "q_completed"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusCompleted,
		CancelFunc: func() {},
	})
	h := newTestQueryHandlerWithRegistry(registry)

	c, w := setupQueryContextWithParams(http.MethodPost, "/api/v1/query/cancel/"+queryID, gin.Params{
		{Key: "query_id", Value: queryID},
	}, nil)

	h.Cancel(c)

	assertResponseCode(t, w, CodeQueryError)
}

func TestQueryHandler_Cancel_AlreadyFailed(t *testing.T) {
	registry := NewQueryRegistry()
	queryID := "q_failed"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusFailed,
		CancelFunc: func() {},
	})
	h := newTestQueryHandlerWithRegistry(registry)

	c, w := setupQueryContextWithParams(http.MethodPost, "/api/v1/query/cancel/"+queryID, gin.Params{
		{Key: "query_id", Value: queryID},
	}, nil)

	h.Cancel(c)

	assertResponseCode(t, w, CodeQueryError)
}

func TestQueryHandler_Cancel_AlreadyCancelled(t *testing.T) {
	registry := NewQueryRegistry()
	queryID := "q_cancelled"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusCancelled,
		CancelFunc: func() {},
	})
	h := newTestQueryHandlerWithRegistry(registry)

	c, w := setupQueryContextWithParams(http.MethodPost, "/api/v1/query/cancel/"+queryID, gin.Params{
		{Key: "query_id", Value: queryID},
	}, nil)

	h.Cancel(c)

	assertResponseCode(t, w, CodeQueryError)
}

func TestQueryHandler_Cancel_Success(t *testing.T) {
	registry := NewQueryRegistry()
	queryID := "q_running"

	cancelCalled := false
	registry.Register(&RunningQuery{
		QueryID: queryID,
		Status:  QueryStatusRunning,
		CancelFunc: func() {
			cancelCalled = true
		},
	})
	h := newTestQueryHandlerWithRegistry(registry)

	c, w := setupQueryContextWithParams(http.MethodPost, "/api/v1/query/cancel/"+queryID, gin.Params{
		{Key: "query_id", Value: queryID},
	}, nil)

	h.Cancel(c)

	assertResponseCode(t, w, CodeSuccess)

	if !cancelCalled {
		t.Error("expected CancelFunc to be called")
	}

	// 验证审计字段已设置
	auditDetail, exists := c.Get("audit_detail")
	if !exists {
		t.Error("expected audit_detail to be set")
	}
	if auditDetailStr, ok := auditDetail.(string); !ok || auditDetailStr == "" {
		t.Errorf("expected non-empty audit_detail string, got %v", auditDetail)
	}
}

// === QueryHandler.GetPoolStats 参数校验测试 ===

func TestQueryHandler_GetPoolStats_InvalidID(t *testing.T) {
	h := newTestQueryHandler()

	tests := []struct {
		name  string
		idVal string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupQueryContextWithParams(http.MethodGet, "/api/v1/query/pool-stats/"+tt.idVal, gin.Params{
				{Key: "id", Value: tt.idVal},
			}, nil)
			h.GetPoolStats(c)
			assertResponseCode(t, w, CodeBadRequest)
		})
	}
}

// === QueryHandler.ClearCache 参数校验测试 ===

func TestQueryHandler_ClearCache_InvalidID(t *testing.T) {
	h := newTestQueryHandler()

	tests := []struct {
		name  string
		idVal string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupQueryContextWithParams(http.MethodDelete, "/api/v1/query/cache/"+tt.idVal, gin.Params{
				{Key: "id", Value: tt.idVal},
			}, nil)
			h.ClearCache(c)
			assertResponseCode(t, w, CodeBadRequest)
		})
	}
}

// === QueryHandler.GetHistory 分页参数测试 ===

func TestQueryHandler_GetHistory_DefaultPagination(t *testing.T) {
	h := newTestQueryHandler()

	// 由于 dsService 为 nil，调用会 panic，但我们只需验证分页参数解析路径
	// 通过设置 admin 角色和 domain_id 参数来测试参数解析
	c, w := setupQueryContext(http.MethodGet, "/api/v1/query/history", nil)
	c.Set("role", "system_admin")
	c.Set("current_domain_id", int64(1))

	defer func() {
		if r := recover(); r != nil {
			// panic 说明参数解析成功并进入了 service 调用，这是预期行为
		}
	}()

	h.GetHistory(c)

	// 不应该返回 BadRequest（参数解析成功）
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if code, ok := resp["code"].(float64); ok && int(code) == CodeBadRequest {
			t.Errorf("不应返回 BadRequest")
		}
	}
}

func TestQueryHandler_GetHistory_AdminWithDomainID(t *testing.T) {
	h := newTestQueryHandler()

	// 测试 admin 角色带 domain_id 参数解析
	c, w := setupQueryContext(http.MethodGet, "/api/v1/query/history?domain_id=5&page=2&page_size=10", nil)
	c.Set("role", "admin")
	c.Set("current_domain_id", int64(1))

	defer func() {
		if r := recover(); r != nil {
			// panic 说明参数解析成功并进入了 service 调用
		}
	}()

	h.GetHistory(c)

	// 不应该返回 BadRequest
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if code, ok := resp["code"].(float64); ok && int(code) == CodeBadRequest {
			t.Errorf("不应返回 BadRequest")
		}
	}
}

func TestQueryHandler_GetHistory_NonAdminUsesCurrentDomain(t *testing.T) {
	h := newTestQueryHandler()

	// 非 admin 角色应使用 current_domain_id
	c, w := setupQueryContext(http.MethodGet, "/api/v1/query/history?domain_id=999", nil)
	c.Set("role", "user")
	c.Set("current_domain_id", int64(42))

	defer func() {
		if r := recover(); r != nil {
			// panic 说明参数解析成功并进入了 service 调用
		}
	}()

	h.GetHistory(c)

	// 不应该返回 BadRequest
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if code, ok := resp["code"].(float64); ok && int(code) == CodeBadRequest {
			t.Errorf("不应返回 BadRequest")
		}
	}
}

// === QueryHandler.ListSavedSQL 分页参数测试 ===

func TestQueryHandler_ListSavedSQL_DefaultPagination(t *testing.T) {
	h := newTestQueryHandler()

	c, w := setupQueryContext(http.MethodGet, "/api/v1/query/saved-sql", nil)
	c.Set("role", "system_admin")
	c.Set("current_domain_id", int64(1))
	c.Set("user_id", int64(100))

	defer func() {
		if r := recover(); r != nil {
			// panic 说明参数解析成功并进入了 service 调用
		}
	}()

	h.ListSavedSQL(c)

	// 不应该返回 BadRequest
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if code, ok := resp["code"].(float64); ok && int(code) == CodeBadRequest {
			t.Errorf("不应返回 BadRequest")
		}
	}
}

// === 工具函数测试 ===

func TestTruncateSQLPreview(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "empty string",
			sql:  "",
			want: "",
		},
		{
			name: "short sql",
			sql:  "SELECT 1",
			want: "SELECT 1",
		},
		{
			name: "sql with leading/trailing whitespace",
			sql:  "  SELECT 1  ",
			want: "SELECT 1",
		},
		{
			name: "sql exactly at limit (200 runes)",
			sql:  string([]rune("SELECT ")[:0]) + string(make([]rune, 200)),
			want: string(make([]rune, 200)),
		},
		{
			name: "sql exceeding limit gets truncated with ellipsis",
			sql:  string(make([]rune, 250)),
			want: string(make([]rune, 200)) + "...",
		},
		{
			name: "multibyte characters preserved",
			sql:  "SELECT '中文测试' FROM dual",
			want: "SELECT '中文测试' FROM dual",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateSQLPreview(tt.sql)
			if got != tt.want {
				t.Errorf("truncateSQLPreview() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateSQLPreview_MultibyteTruncation(t *testing.T) {
	// 测试多字节字符在截断时不被破坏
	// 每个中文字符占 1 个 rune，构造超过 200 rune 的字符串
	longSQL := "SELECT '"
	for i := 0; i < 100; i++ {
		longSQL += "中文"
	}
	longSQL += "'"

	got := truncateSQLPreview(longSQL)

	// 应该被截断为 200 rune + "..."
	if len([]rune(got)) != 203 { // 200 + 3 for "..."
		t.Errorf("expected 203 runes, got %d", len([]rune(got)))
	}

	// 应该以 "..." 结尾
	if got[len(got)-3:] != "..." {
		t.Errorf("expected to end with '...', got %q", got[len(got)-3:])
	}
}

func TestDefaultDBName(t *testing.T) {
	tests := []struct {
		name string
		db   string
		want string
	}{
		{
			name: "empty returns default",
			db:   "",
			want: "(默认)",
		},
		{
			name: "non-empty returns as-is",
			db:   "mydb",
			want: "mydb",
		},
		{
			name: "whitespace returns as-is",
			db:   " ",
			want: " ",
		},
		{
			name: "special characters preserved",
			db:   "test-db_123",
			want: "test-db_123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultDBName(tt.db)
			if got != tt.want {
				t.Errorf("defaultDBName(%q) = %q, want %q", tt.db, got, tt.want)
			}
		})
	}
}

// === QueryHandler.OnConfigChanged 测试 ===
// 注：OnConfigChanged 调用 refreshRuntimeConfig，后者依赖 configService。
// NewQueryHandler 保证注入非 nil configService，此处无法用 nil configService 测试。
// 该方法逻辑简单（委托 refreshRuntimeConfig + 日志），在生产环境中通过配置观察者链测试。

// === QueryRegistry 观察者测试 ===

func TestQueryRegistry_RegisterAndUnregisterObserver(t *testing.T) {
	registry := NewQueryRegistry()

	observer := &mockQueryObserver{}
	registry.RegisterObserver(observer)
	registry.UnregisterObserver(observer)

	// 验证注册和注销不 panic
}

func TestQueryRegistry_SetRunning(t *testing.T) {
	registry := NewQueryRegistry()
	queryID := "q_set_running"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusPending,
		CancelFunc: func() {},
	})

	registry.SetRunning(queryID)

	q, ok := registry.Get(queryID)
	if !ok {
		t.Fatal("query should exist")
	}
	if q.Status != QueryStatusRunning {
		t.Errorf("status = %v, want %v", q.Status, QueryStatusRunning)
	}
}

func TestQueryRegistry_SetRunning_NonExistent(t *testing.T) {
	registry := NewQueryRegistry()

	// 设置不存在的查询不应 panic
	registry.SetRunning("q_nonexistent")
}

func TestQueryRegistry_Cancel_NonExistent(t *testing.T) {
	registry := NewQueryRegistry()

	if registry.Cancel("q_nonexistent") {
		t.Error("expected Cancel to return false for non-existent query")
	}
}

func TestQueryRegistry_Cancel_AlreadyTerminal(t *testing.T) {
	registry := NewQueryRegistry()
	queryID := "q_terminal"

	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusCompleted,
		CancelFunc: func() {},
	})

	if registry.Cancel(queryID) {
		t.Error("expected Cancel to return false for completed query")
	}
}

func TestQueryRegistry_Cleanup(t *testing.T) {
	registry := NewQueryRegistry()
	queryID := "q_old"

	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusCompleted,
		CancelFunc: func() {},
	})

	// Cleanup 会删除超过 maxAge 的查询
	registry.Cleanup(0) // maxAge=0 会立即清理

	_, ok := registry.Get(queryID)
	if ok {
		t.Error("expected query to be cleaned up")
	}
}

// mockQueryObserver 用于测试观察者模式（并发安全）
type mockQueryObserver struct {
	mu      sync.RWMutex
	updates []string
}

func (m *mockQueryObserver) OnQueryUpdate(queryID string, query *RunningQuery) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates = append(m.updates, queryID)
}

func TestQueryRegistry_NotifyObserver(t *testing.T) {
	registry := NewQueryRegistry()
	observer := &mockQueryObserver{}
	registry.RegisterObserver(observer)

	queryID := "q_observe"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusRunning,
		CancelFunc: func() {},
	})

	registry.UpdateResult(queryID, nil, 1.0)

	// notifyObservers 通过 goroutine 异步通知，等待一小段时间确保 goroutine 完成
	// 使用轮询方式检查，最多等待 100ms
	for i := 0; i < 100; i++ {
		observer.mu.RLock()
		count := len(observer.updates)
		observer.mu.RUnlock()
		if count >= 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	observer.mu.RLock()
	defer observer.mu.RUnlock()
	if len(observer.updates) != 1 {
		t.Errorf("expected 1 update, got %d", len(observer.updates))
	}
	if len(observer.updates) > 0 && observer.updates[0] != queryID {
		t.Errorf("expected update for %q, got %q", queryID, observer.updates[0])
	}
}
