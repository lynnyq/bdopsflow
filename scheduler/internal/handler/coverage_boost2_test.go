package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
)

// ============================================================
// query.go: StreamResult - 查询不存在路径
// ============================================================

// TestQueryHandler_StreamResult_NotFound_Registry 测试 StreamResult 查询不存在于 registry
// 注：handler_api_test.go 中的 StreamResult_EmptyQueryID 只测试空 query_id，
// 此测试覆盖 registry.Get 返回 false 的路径
func TestQueryHandler_StreamResult_NotFound_Registry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "query_id", Value: "non-existent-in-registry"}}

	h := &QueryHandler{registry: NewQueryRegistry()}
	h.StreamResult(c)

	// 查询不存在时，handler 通过 SSEvent 发送 error 事件并 flush
	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty SSE response body for not found query")
	}
}

// ============================================================
// query.go: StreamResult - 已完成查询路径
// ============================================================

// TestQueryHandler_StreamResult_CompletedQuery 测试 StreamResult 查询已完成时直接推送结果
func TestQueryHandler_StreamResult_CompletedQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "query_id", Value: "q-stream-completed"}}

	registry := NewQueryRegistry()
	registry.Register(&RunningQuery{
		QueryID:       "q-stream-completed",
		Status:        QueryStatusCompleted,
		Result:        &driver.QueryResult{Columns: []string{"id"}, Rows: [][]interface{}{{int64(1)}}, RowCount: 1},
		ExecutionTime: 0.5,
		FromCache:     false,
	})

	h := &QueryHandler{registry: registry}
	h.StreamResult(c)

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty SSE response body for completed query")
	}
}

// TestQueryHandler_StreamResult_FailedQuery 测试 StreamResult 查询失败时直接推送结果
func TestQueryHandler_StreamResult_FailedQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "query_id", Value: "q-stream-failed"}}

	registry := NewQueryRegistry()
	registry.Register(&RunningQuery{
		QueryID:       "q-stream-failed",
		Status:        QueryStatusFailed,
		Error:         "query execution failed",
		ExecutionTime: 1.2,
	})

	h := &QueryHandler{registry: registry}
	h.StreamResult(c)

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty SSE response body for failed query")
	}
}

// TestQueryHandler_StreamResult_CancelledQuery 测试 StreamResult 查询已取消时直接推送结果
func TestQueryHandler_StreamResult_CancelledQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "query_id", Value: "q-stream-cancelled"}}

	registry := NewQueryRegistry()
	registry.Register(&RunningQuery{
		QueryID:       "q-stream-cancelled",
		Status:        QueryStatusCancelled,
		Error:         "user cancelled",
		ExecutionTime: 0.1,
	})

	h := &QueryHandler{registry: registry}
	h.StreamResult(c)

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty SSE response body for cancelled query")
	}
}

// ============================================================
// query.go: GetResult - Cancelled 状态（补全 91.7% → 100%）
// ============================================================

// TestQueryHandler_GetResult_CancelledStatus 测试 GetResult 查询已取消状态
// 注：query_test.go 已测试 NilResult(Completed)、NonNilResult(Completed)、FailedStatus，
// 但未测试 Cancelled 状态
func TestQueryHandler_GetResult_CancelledStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "query_id", Value: "q-get-cancelled"}}

	registry := NewQueryRegistry()
	registry.Register(&RunningQuery{
		QueryID:       "q-get-cancelled",
		Status:        QueryStatusCancelled,
		Error:         "cancelled by user",
		ExecutionTime: 0.3,
	})

	h := &QueryHandler{registry: registry}
	h.GetResult(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response data should be a map")
	}

	if data["status"] != "cancelled" {
		t.Errorf("status = %v, want cancelled", data["status"])
	}
	if data["error"] != "cancelled by user" {
		t.Errorf("error = %v, want 'cancelled by user'", data["error"])
	}
}

// ============================================================
// task.go: StreamLogs - 上下文取消路径
// ============================================================

// TestTaskHandler_StreamLogs_ContextCancelled 测试 StreamLogs 在 context 取消时退出
// 注：coverage_boost_test.go 中的 StreamLogs_EmptyExecutionID 只测试空 execution_id，
// 此测试覆盖 context.Done() 分支
func TestTaskHandler_StreamLogs_ContextCancelled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodGet, "/?execution_id=exec-1", nil)
	c.Request = c.Request.WithContext(ctx)

	mock := &mockTaskService{
		isLeaderFunc: func() bool { return true },
	}
	handler := newTaskHandlerWithSvc(mock)

	// 立即取消 context，使 handler 在第一次 select 时走 ctx.Done() 分支
	cancel()

	// StreamLogs 是阻塞的，context 已取消应立即返回
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.StreamLogs(c)
	}()

	select {
	case <-done:
		// 预期快速返回
	case <-time.After(2 * time.Second):
		t.Fatal("StreamLogs did not return after context cancel within 2s")
	}

	// 验证 SSE headers 已设置
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got %q", ct)
	}
}

// ============================================================
// query.go: GetResult - Running/Pending 状态
// ============================================================

// TestQueryHandler_GetResult_RunningStatus 测试 GetResult 查询运行中状态
// Running 状态不匹配任何 switch case，只返回基本字段
func TestQueryHandler_GetResult_RunningStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "query_id", Value: "q-get-running"}}

	registry := NewQueryRegistry()
	registry.Register(&RunningQuery{
		QueryID: "q-get-running",
		Status:  QueryStatusRunning,
	})

	h := &QueryHandler{registry: registry}
	h.GetResult(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response data should be a map")
	}

	if data["status"] != "running" {
		t.Errorf("status = %v, want running", data["status"])
	}
}

// ============================================================
// query.go: GetResult - Pending 状态
// ============================================================

// TestQueryHandler_GetResult_PendingStatus 测试 GetResult 查询待处理状态
func TestQueryHandler_GetResult_PendingStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "query_id", Value: "q-get-pending"}}

	registry := NewQueryRegistry()
	registry.Register(&RunningQuery{
		QueryID: "q-get-pending",
		Status:  QueryStatusPending,
	})

	h := &QueryHandler{registry: registry}
	h.GetResult(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response data should be a map")
	}

	if data["status"] != "pending" {
		t.Errorf("status = %v, want pending", data["status"])
	}
}
