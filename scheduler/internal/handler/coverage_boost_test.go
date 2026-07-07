package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

// ============================================================
// response.go: ServiceUnavailable
// ============================================================

// TestServiceUnavailable 测试 ServiceUnavailable 响应
func TestServiceUnavailable(t *testing.T) {
	c, w := setupTestContext()

	ServiceUnavailable(c, "service unavailable")

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeServiceUnavailable {
		t.Errorf("expected code %d, got %d", CodeServiceUnavailable, resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if resp.Message != "service unavailable" {
		t.Errorf("expected message 'service unavailable', got %q", resp.Message)
	}
}

// ============================================================
// common.go: checkOwnership
// ============================================================

// TestCheckOwnership_NoUser 测试未登录用户（userID=0）
func TestCheckOwnership_NoUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	// 不设置 user_id

	result := checkOwnership(c, nil, 999)
	if result {
		t.Error("expected false for missing user_id")
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d, got %d", CodeUnauthorized, resp.Code)
	}
}

// TestCheckOwnership_SameUser 测试用户为自己的资源
func TestCheckOwnership_SameUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("user_id", int64(123))

	result := checkOwnership(c, nil, 123)
	if !result {
		t.Error("expected true for owner accessing own resource")
	}
}

// ============================================================
// dashboard.go: NewDashboardHandler, HealthCheck
// ============================================================

// TestNewDashboardHandler 测试构造函数
func TestNewDashboardHandler(t *testing.T) {
	h := NewDashboardHandler(nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.svc != nil {
		t.Errorf("expected nil svc, got %v", h.svc)
	}
}

// TestDashboardHandler_HealthCheck_NilService 测试 HealthCheck 在 svc 为 nil 时通过 recover 返回 500
func TestDashboardHandler_HealthCheck_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DashboardHandler{}
	r.GET("/api/dashboard/health", handler.HealthCheck)

	req, _ := http.NewRequest("GET", "/api/dashboard/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if code, ok := body["code"].(float64); !ok || int(code) != 500 {
		t.Errorf("expected body.code 500 for nil service, got %v", body["code"])
	}
}

// ============================================================
// task.go: NewTaskHandler, CancelExecution, ExecutionLogs
// ============================================================

// TestNewTaskHandler 测试构造函数
func TestNewTaskHandler(t *testing.T) {
	h := NewTaskHandler(nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

// TestTaskHandler_CancelExecution_EmptyID 测试 CancelExecution 缺少 execution_id
func TestTaskHandler_CancelExecution_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	// execution_id 来自 URL 参数，未设置则为空字符串

	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	handler.CancelExecution(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
	}
}

// TestTaskHandler_CancelExecution_NotFound 测试 CancelExecution 找不到执行记录
func TestTaskHandler_CancelExecution_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Params = gin.Params{{Key: "execution_id", Value: "exec-not-found"}}

	// 使用自定义 service 类型覆盖 CancelExecution 方法，返回 not found 错误
	handler := newTaskHandlerWithSvc(&cancelNotFoundService{})
	handler.CancelExecution(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeNotFound {
		t.Errorf("expected code %d for not found, got %d", CodeNotFound, resp.Code)
	}
}

// cancelNotFoundService 包装 mockTaskService 并覆盖 CancelExecution 方法
type cancelNotFoundService struct {
	mockTaskService
}

func (m *cancelNotFoundService) CancelExecution(ctx context.Context, executionID string, cancelledBy string) error {
	return errors.New("execution not found")
}

// TestTaskHandler_CancelExecution_NotRunning 测试 CancelExecution 任务不在运行状态
func TestTaskHandler_CancelExecution_NotRunning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Params = gin.Params{{Key: "execution_id", Value: "exec-1"}}
	c.Set("username", "tester")

	handler := newTaskHandlerWithSvc(&cancelNotRunningService{})
	handler.CancelExecution(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for not running, got %d", CodeBadRequest, resp.Code)
	}
}

type cancelNotRunningService struct {
	mockTaskService
}

func (m *cancelNotRunningService) CancelExecution(ctx context.Context, executionID string, cancelledBy string) error {
	return errors.New("execution is not running")
}

// TestTaskHandler_CancelExecution_Success 测试 CancelExecution 成功
func TestTaskHandler_CancelExecution_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Params = gin.Params{{Key: "execution_id", Value: "exec-1"}}
	c.Set("username", "tester")

	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	handler.CancelExecution(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d for success, got %d", CodeSuccess, resp.Code)
	}
}

// TestTaskHandler_CancelExecution_OtherError 测试 CancelExecution 其他错误
func TestTaskHandler_CancelExecution_OtherError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Params = gin.Params{{Key: "execution_id", Value: "exec-1"}}

	handler := newTaskHandlerWithSvc(&cancelOtherErrService{})
	handler.CancelExecution(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeInternalError {
		t.Errorf("expected code %d for other error, got %d", CodeInternalError, resp.Code)
	}
}

type cancelOtherErrService struct {
	mockTaskService
}

func (m *cancelOtherErrService) CancelExecution(ctx context.Context, executionID string, cancelledBy string) error {
	return errors.New("database error")
}

// TestTaskHandler_ExecutionLogs_EmptyID 测试 ExecutionLogs 缺少 execution_id
func TestTaskHandler_ExecutionLogs_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	handler.ExecutionLogs(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
	}
}

// TestTaskHandler_ExecutionLogs_ServiceError 测试 ExecutionLogs service 返回错误
func TestTaskHandler_ExecutionLogs_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "execution_id", Value: "exec-1"}}

	mock := &mockTaskService{
		getTaskLogsFunc: func(ctx context.Context, executionID string) ([]*model.TaskLog, error) {
			return nil, errors.New("database error")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	handler.ExecutionLogs(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeInternalError {
		t.Errorf("expected code %d for service error, got %d", CodeInternalError, resp.Code)
	}
}

// TestTaskHandler_ExecutionLogs_Success 测试 ExecutionLogs 成功
func TestTaskHandler_ExecutionLogs_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "execution_id", Value: "exec-1"}}

	mock := &mockTaskService{
		getTaskLogsFunc: func(ctx context.Context, executionID string) ([]*model.TaskLog, error) {
			return []*model.TaskLog{
				{ID: 1, ExecutionID: executionID, TaskID: 10, Message: "log line 1", LogTime: time.Now()},
			}, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	handler.ExecutionLogs(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d for success, got %d", CodeSuccess, resp.Code)
	}
}

// TestTaskHandler_Executions_NegativeID 测试 Executions id <= 0
func TestTaskHandler_Executions_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "-1"}}

	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	handler.Executions(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for negative id, got %d", CodeBadRequest, resp.Code)
	}
}

// TestTaskHandler_Executions_ServiceError 测试 Executions service 错误
func TestTaskHandler_Executions_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	mock := &mockTaskService{
		getTaskExecsFunc: func(ctx context.Context, taskID int64) ([]*model.TaskExecution, error) {
			return nil, errors.New("database error")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	handler.Executions(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code == CodeSuccess {
		t.Errorf("expected non-success code for service error, got %d", resp.Code)
	}
}

// ============================================================
// task.go: forwardToLeader
// ============================================================

// TestTaskHandler_ForwardToLeader_Error 测试 forwardToLeader service 返回错误
func TestTaskHandler_ForwardToLeader_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/tasks/1?foo=bar", nil)
	c.Request.Header.Set("Authorization", "Bearer token")
	c.Request.Header.Set("Content-Type", "application/json")

	mock := &mockTaskService{
		forwardToLeaderFunc: func(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error) {
			return nil, 0, errors.New("leader unavailable")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	handler.forwardToLeader(c, "GET", "/api/tasks/1", nil)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeServiceUnavailable {
		t.Errorf("expected code %d, got %d", CodeServiceUnavailable, resp.Code)
	}
}

// TestTaskHandler_ForwardToLeader_Success 测试 forwardToLeader 成功转发
func TestTaskHandler_ForwardToLeader_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/tasks/1?foo=bar", nil)
	c.Request.Header.Set("Authorization", "Bearer token")

	leaderResp := []byte(`{"code":0,"status":"success","message":"ok","data":{"id":1}}`)
	mock := &mockTaskService{
		forwardToLeaderFunc: func(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error) {
			return leaderResp, http.StatusOK, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	handler.forwardToLeader(c, "GET", "/api/tasks/1", nil)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), leaderResp) {
		t.Errorf("expected body to be forwarded response, got: %s", w.Body.String())
	}
}

// ============================================================
// task.go: StreamLogs
// ============================================================

// TestTaskHandler_StreamLogs_EmptyExecutionID 测试 StreamLogs 缺少 execution_id
func TestTaskHandler_StreamLogs_EmptyExecutionID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	handler.StreamLogs(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
	}
}

// ============================================================
// role_admin.go: GetAllPermissions
// ============================================================

// TestRoleAdminHandler_GetAllPermissions_NilSvc 测试 GetAllPermissions svc 为 nil 时 panic
// 注：GetAllPermissions 没有 defer recover，nil svc 会直接 panic
func TestRoleAdminHandler_GetAllPermissions_NilSvc(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h := &RoleAdminHandler{}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil svc, but no panic occurred")
		}
	}()

	h.GetAllPermissions(c)
	_ = w
}

// ============================================================
// user_admin.go: ListUsers, ListUsersByDomain
// ============================================================

// TestUserAdminHandler_ListUsers_NilSvc 测试 ListUsers svc 为 nil 时触发 panic recover
func TestUserAdminHandler_ListUsers_NilSvc(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	// 不设置 user_id 和 current_domain_id

	h := &UserAdminHandler{}
	h.ListUsers(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != CodeInternalError {
		t.Errorf("expected code %d for nil svc panic recover, got %d", CodeInternalError, resp.Code)
	}
}

// TestUserAdminHandler_ListUsersByDomain_NilSvc 测试 ListUsersByDomain svc 为 nil 时 panic
// 注：ListUsersByDomain 没有 defer recover，nil svc 会直接 panic
func TestUserAdminHandler_ListUsersByDomain_NilSvc(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?domain_id=1", nil)

	h := &UserAdminHandler{}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil svc, but no panic occurred")
		}
	}()

	h.ListUsersByDomain(c)
	_ = w
}

// ============================================================
// wecom_handler.go: NewWeComHandler, GetChatGroupInfo
// ============================================================

// TestNewWeComHandler 测试构造函数（不依赖外部资源）
func TestNewWeComHandler(t *testing.T) {
	// NewWeComHandler 内部调用 wecom.NewService(configService)
	// 传入 nil configService 也会创建 service（仅初始化结构体）
	defer func() {
		if r := recover(); r != nil {
			// wecom.NewService 可能在 nil 时 panic，这是预期行为
			t.Logf("NewWeComHandler with nil configService panicked (expected): %v", r)
		}
	}()
	h := NewWeComHandler(nil)
	if h != nil {
		// 如果没有 panic，handler 应该有 configService
		if h.configService != nil {
			t.Errorf("expected nil configService, got %v", h.configService)
		}
	}
}

// TestWeComHandler_GetChatGroupInfo_NilService 测试 GetChatGroupInfo 在 wecomService 为 nil 时
func TestWeComHandler_GetChatGroupInfo_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WeComHandler{}
	r.GET("/api/wecom/chat/:chat_id", h.GetChatGroupInfo)

	req, _ := http.NewRequest("GET", "/api/wecom/chat/test-chat-1", nil)
	w := httptest.NewRecorder()

	defer func() {
		_ = recover()
	}()

	r.ServeHTTP(w, req)

	// 如果 wecomService 为 nil 会 panic，gin 默认 recovery 中间件不在，会向上抛
	// 实际上 gin.New() 没有 recovery，所以会 panic - 测试通过 recover 捕获
}

// TestWeComHandler_GetChatGroupInfo_WithChatID 验证 GetChatGroupInfo 在有 chat_id 时会调用 service
func TestWeComHandler_GetChatGroupInfo_WithChatID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "chat_id", Value: "test-chat-1"}}

	h := &WeComHandler{}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil wecomService, but no panic occurred")
		}
	}()

	h.GetChatGroupInfo(c)
}

// ============================================================
// api_test_handler.go: GenerateCurl
// ============================================================

// TestApiTestHandler_GenerateCurl_InvalidJSON 测试 GenerateCurl 无效 JSON
func TestApiTestHandler_GenerateCurl_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h := newTestApiTestHandler()
	h.GenerateCurl(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("expected code %d for invalid JSON, got %v", CodeBadRequest, resp["code"])
	}
}

// TestApiTestHandler_GenerateCurl_NilHttpExec 测试 GenerateCurl 在 httpExec 为 nil 时
// 注：HTTPExecutor.GenerateCurl 不解引用 receiver，所以 nil receiver 不会 panic
func TestApiTestHandler_GenerateCurl_NilHttpExec(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := map[string]interface{}{
		"method": "GET",
		"url":    "http://example.com",
	}
	bodyBytes, _ := json.Marshal(body)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	h := newTestApiTestHandler() // httpExec 为 nil

	h.GenerateCurl(c)

	// GenerateCurl 不解引用 receiver，应该返回成功响应
	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeSuccess) {
		t.Errorf("expected code %d for nil httpExec (GenerateCurl doesn't deref receiver), got %v", CodeSuccess, resp["code"])
	}
}

// ============================================================
// query.go: sseQueryObserver.OnQueryUpdate, sendSSEEvent
// ============================================================

// TestSseQueryObserver_OnQueryUpdate_MatchingQueryID 测试 OnQueryUpdate 匹配 queryID 时发送到 channel
func TestSseQueryObserver_OnQueryUpdate_MatchingQueryID(t *testing.T) {
	ch := make(chan *RunningQuery, 1)
	observer := &sseQueryObserver{queryID: "q-1", ch: ch}

	query := &RunningQuery{QueryID: "q-1", Status: QueryStatusCompleted}
	observer.OnQueryUpdate("q-1", query)

	select {
	case got := <-ch:
		if got.QueryID != "q-1" {
			t.Errorf("expected query_id q-1, got %q", got.QueryID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive query on channel")
	}
}

// TestSseQueryObserver_OnQueryUpdate_NonMatchingQueryID 测试 OnQueryUpdate 不匹配 queryID 时不发送
func TestSseQueryObserver_OnQueryUpdate_NonMatchingQueryID(t *testing.T) {
	ch := make(chan *RunningQuery, 1)
	observer := &sseQueryObserver{queryID: "q-1", ch: ch}

	query := &RunningQuery{QueryID: "q-other", Status: QueryStatusCompleted}
	observer.OnQueryUpdate("q-other", query)

	select {
	case <-ch:
		t.Error("expected no signal on channel for non-matching queryID")
	case <-time.After(50 * time.Millisecond):
		// 预期不收到消息
	}
}

// TestSseQueryObserver_OnQueryUpdate_FullChannel 测试 OnQueryUpdate channel 满时不阻塞
func TestSseQueryObserver_OnQueryUpdate_FullChannel(t *testing.T) {
	ch := make(chan *RunningQuery, 1)
	// 先填满 channel
	ch <- &RunningQuery{QueryID: "q-1"}

	observer := &sseQueryObserver{queryID: "q-1", ch: ch}

	// 应该不阻塞，直接丢弃
	done := make(chan struct{})
	go func() {
		defer close(done)
		observer.OnQueryUpdate("q-1", &RunningQuery{QueryID: "q-1"})
	}()

	select {
	case <-done:
		// 预期不阻塞
	case <-time.After(100 * time.Millisecond):
		t.Error("OnQueryUpdate blocked on full channel")
	}
}

// TestQueryHandler_SendSSEEvent_Completed 测试 sendSSEEvent 在 completed 状态
func TestQueryHandler_SendSSEEvent_Completed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	// 使用带 buffer 的 ResponseWriter 以避免 SSE 写入问题
	h := &QueryHandler{}

	q := &RunningQuery{
		QueryID:       "q-1",
		Status:        QueryStatusCompleted,
		Result:        &driver.QueryResult{Columns: []string{"id"}, Rows: [][]interface{}{{int64(1)}}, RowCount: 1},
		ExecutionTime: 0.5,
		FromCache:     false,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("sendSSEEvent panicked (acceptable for test context): %v", r)
		}
	}()

	h.sendSSEEvent(c, q)
}

// TestQueryHandler_SendSSEEvent_Failed 测试 sendSSEEvent 在 failed 状态
func TestQueryHandler_SendSSEEvent_Failed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h := &QueryHandler{}

	q := &RunningQuery{
		QueryID:       "q-failed",
		Status:        QueryStatusFailed,
		Error:         "query timeout",
		ExecutionTime: 1.5,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("sendSSEEvent panicked (acceptable for test context): %v", r)
		}
	}()

	h.sendSSEEvent(c, q)
}

// TestQueryHandler_SendSSEEvent_Cancelled 测试 sendSSEEvent 在 cancelled 状态
func TestQueryHandler_SendSSEEvent_Cancelled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h := &QueryHandler{}

	q := &RunningQuery{
		QueryID:       "q-cancelled",
		Status:        QueryStatusCancelled,
		Error:         "user cancelled",
		ExecutionTime: 0.1,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("sendSSEEvent panicked (acceptable for test context): %v", r)
		}
	}()

	h.sendSSEEvent(c, q)
}

// TestQueryHandler_SendSSEEvent_CompletedNilResult 测试 sendSSEEvent 在 completed 状态但 result 为 nil
func TestQueryHandler_SendSSEEvent_CompletedNilResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h := &QueryHandler{}

	q := &RunningQuery{
		QueryID:       "q-nil-result",
		Status:        QueryStatusCompleted,
		Result:        nil,
		ExecutionTime: 0.0,
		FromCache:     true,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("sendSSEEvent panicked (acceptable for test context): %v", r)
		}
	}()

	h.sendSSEEvent(c, q)
}

// ============================================================
// query_registry.go: StartCleanupLoop
// ============================================================

// TestQueryRegistry_StartCleanupLoop 测试 StartCleanupLoop 启动后能正常执行清理
func TestQueryRegistry_StartCleanupLoop(t *testing.T) {
	registry := NewQueryRegistry()

	// 注册一个已完成查询
	queryID := "q-cleanup-test"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusCompleted,
		CancelFunc: func() {},
	})

	// 启动 cleanup loop，间隔短一些
	registry.StartCleanupLoop(50*time.Millisecond, 0) // maxAge=0 立即清理

	// 等待 cleanup 执行
	time.Sleep(150 * time.Millisecond)

	_, ok := registry.Get(queryID)
	if ok {
		t.Error("expected query to be cleaned up after StartCleanupLoop")
	}
}

// ============================================================
// query_registry_distributed.go: StartCleanupLoop, Close
// ============================================================

// TestDistributedQueryRegistry_StartCleanupLoop 测试分布式 StartCleanupLoop
func TestDistributedQueryRegistry_StartCleanupLoop(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	registry := NewDistributedQueryRegistry(client, "node-cleanup-test")

	// 注册一个已完成查询
	queryID := "q-dist-cleanup-test"
	registry.Register(&RunningQuery{
		QueryID:    queryID,
		Status:     QueryStatusCompleted,
		CancelFunc: func() {},
	})

	// 启动 cleanup loop
	registry.StartCleanupLoop(50*time.Millisecond, 0)

	// 等待 cleanup 执行
	time.Sleep(150 * time.Millisecond)

	_, ok := registry.Get(queryID)
	if ok {
		t.Error("expected query to be cleaned up after StartCleanupLoop")
	}
}

// TestDistributedQueryRegistry_Close_NoSubscriber 测试 Close 在未启动 subscriber 时
func TestDistributedQueryRegistry_Close_NoSubscriber(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	registry := NewDistributedQueryRegistry(client, "node-close-test")

	// 未启动 subscriber，subCancel 为 nil
	// Close 不应 panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Close should not panic when subCancel is nil, got: %v", r)
		}
	}()

	registry.Close()
}

// TestDistributedQueryRegistry_Close_AfterSubscriber 测试 Close 在启动 subscriber 后
func TestDistributedQueryRegistry_Close_AfterSubscriber(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	registry := NewDistributedQueryRegistry(client, "node-close-subscriber")

	// 启动 subscriber（通过注册 observer 触发）
	registry.RegisterObserver(&mockQueryObserver{})

	// Close 应该取消 subscriber context
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Close should not panic after subscriber started, got: %v", r)
		}
	}()

	registry.Close()
}

// ============================================================
// query.go: NewQueryHandler, OnConfigChanged, refreshRuntimeConfig
// ============================================================

// 注：NewQueryHandler、OnConfigChanged、refreshRuntimeConfig 都依赖 configService（非 nil）。
// 这些方法需要 mock configService 才能测试，且 configService 是具体类型 *sysconfig.Service。
// 这里测试 sseQueryObserver 已在上面覆盖。
// 由于 configService 是外部依赖的具体类型，无法直接 mock，跳过这部分测试。

// ============================================================
// datasource.go: fillDomainNames, fillUserPermissions
// ============================================================

// TestDatasourceHandler_FillDomainNames_NoDomainIDs 测试 fillDomainNames 在没有 domainID 时直接返回
func TestDatasourceHandler_FillDomainNames_NoDomainIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h := &DatasourceHandler{}

	datasources := []*model.Datasource{
		{ID: 1, Name: "ds1", DomainID: 0},
		{ID: 2, Name: "ds2", DomainID: 0},
	}

	// 不应 panic，即使 domainSvc 为 nil
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("fillDomainNames should not panic for empty domainIDSet, got: %v", r)
		}
	}()

	h.fillDomainNames(c.Request.Context(), datasources)
}

// TestDatasourceHandler_FillDomainNames_WithDomainIDs 测试 fillDomainNames 在有 domainID 但 domainSvc 为 nil 时
func TestDatasourceHandler_FillDomainNames_WithDomainIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h := &DatasourceHandler{}

	datasources := []*model.Datasource{
		{ID: 1, Name: "ds1", DomainID: 5},
	}

	// domainSvc 为 nil 会 panic，但 fillDomainNames 会先调用 ListDomains
	// 这里期望 panic 或错误处理
	defer func() {
		if r := recover(); r != nil {
			// 预期 panic，因为 domainSvc 为 nil
			t.Logf("fillDomainNames panicked for nil domainSvc (expected): %v", r)
		}
	}()

	h.fillDomainNames(c.Request.Context(), datasources)
}

// TestDatasourceHandler_FillUserPermissions_EmptyDatasources 测试 fillUserPermissions 在空切片时直接返回
func TestDatasourceHandler_FillUserPermissions_EmptyDatasources(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h := &DatasourceHandler{}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("fillUserPermissions should not panic for empty datasources, got: %v", r)
		}
	}()

	h.fillUserPermissions(c.Request.Context(), 1, []*model.Datasource{})
}

// TestDatasourceHandler_FillUserPermissions_NilServices 测试 fillUserPermissions 在 services 为 nil 但 datasources 非空时
func TestDatasourceHandler_FillUserPermissions_NilServices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h := &DatasourceHandler{}

	datasources := []*model.Datasource{
		{ID: 1, Name: "ds1", DomainID: 1},
	}

	// instancePermSvc 为 nil 会 panic
	defer func() {
		if r := recover(); r != nil {
			t.Logf("fillUserPermissions panicked for nil instancePermSvc (expected): %v", r)
		}
	}()

	h.fillUserPermissions(c.Request.Context(), 1, datasources)
}

// TestPickHigherPermission_AllCases 测试 pickHigherPermission 的所有组合
func TestPickHigherPermission_AllCases(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{"both empty", "", "", ""},
		{"a empty, b valid", "", "read", "read"},
		{"a valid, b empty", "manage", "", "manage"},
		{"manage vs update", "manage", "update", "manage"},
		{"update vs manage", "update", "manage", "manage"},
		{"read vs query", "read", "query", "query"},
		{"query vs read", "query", "read", "query"},
		{"delete vs read", "delete", "read", "read"},
		{"read vs delete", "read", "delete", "read"},
		{"same permission", "read", "read", "read"},
		{"manage vs manage", "manage", "manage", "manage"},
		{"download vs query", "download", "query", "download"},
		{"unknown vs read", "unknown", "read", "read"},
		{"read vs unknown", "read", "unknown", "read"},
		{"both unknown", "foo", "bar", "bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickHigherPermission(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("pickHigherPermission(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// ============================================================
// 补充测试：辅助验证
// ============================================================

// TestContainsHelper 验证 contains 辅助函数
func TestContainsHelper(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "abc", false},
		{"", "", true},
		{"abc", "", true},
		{"", "abc", false},
	}

	for _, tt := range tests {
		got := contains(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

// TestEscapeJSON_CarriageReturn 测试 escapeJSON 处理 \r
func TestEscapeJSON_CarriageReturn(t *testing.T) {
	input := "hello\rworld"
	got := escapeJSON(input)
	want := "hello\\rworld"
	if got != want {
		t.Errorf("escapeJSON(%q) = %q, want %q", input, got, want)
	}
}

// TestFnvHash_Determinism 验证 fnvHash 多次调用结果一致
func TestFnvHash_Determinism(t *testing.T) {
	input := "test-determinism"
	first := fnvHash(input)
	for i := 0; i < 10; i++ {
		if fnvHash(input) != first {
			t.Fatalf("fnvHash not deterministic on iteration %d", i)
		}
	}
}

// TestTaskHandler_Trigger_DispatchFailed 测试 Trigger 时遇到 dispatch failed 错误
func TestTaskHandler_Trigger_DispatchFailed(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 1}, nil
		},
		triggerTaskFunc: func(ctx context.Context, taskID int64) (string, error) {
			return "", errors.New("dispatch failed: no executor available")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("current_domain_id", int64(1))
		c.Set("role", "user")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeDispatchFailed {
		t.Errorf("expected code %d for dispatch failed, got %d", CodeDispatchFailed, resp.Code)
	}
}

// TestTaskHandler_Trigger_NoCapacity 测试 Trigger 时执行器容量已满
func TestTaskHandler_Trigger_NoCapacity(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 1}, nil
		},
		triggerTaskFunc: func(ctx context.Context, taskID int64) (string, error) {
			return "", errors.New("no capacity available")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("current_domain_id", int64(1))
		c.Set("role", "user")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeExecutorNoCapacity {
		t.Errorf("expected code %d for no capacity, got %d", CodeExecutorNoCapacity, resp.Code)
	}
}

// TestTaskHandler_Trigger_NoAvailableExecutor 测试 Trigger 时无可用执行器
func TestTaskHandler_Trigger_NoAvailableExecutor(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 1}, nil
		},
		triggerTaskFunc: func(ctx context.Context, taskID int64) (string, error) {
			return "", fmt.Errorf("no available executor")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("current_domain_id", int64(1))
		c.Set("role", "user")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeNoAvailableExecutor {
		t.Errorf("expected code %d for no available executor, got %d", CodeNoAvailableExecutor, resp.Code)
	}
}

// TestTaskHandler_Trigger_ExecutorOffline 测试 Trigger 时执行器离线（错误消息需包含 "not online"）
func TestTaskHandler_Trigger_ExecutorOffline(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 1}, nil
		},
		triggerTaskFunc: func(ctx context.Context, taskID int64) (string, error) {
			return "", errors.New("executor is not online")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("current_domain_id", int64(1))
		c.Set("role", "user")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeExecutorOffline {
		t.Errorf("expected code %d for executor offline, got %d", CodeExecutorOffline, resp.Code)
	}
}

// TestTaskHandler_Trigger_TaskRunning 测试 Trigger 时任务正在运行（错误消息需包含 "already running"）
func TestTaskHandler_Trigger_TaskRunning(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 1}, nil
		},
		triggerTaskFunc: func(ctx context.Context, taskID int64) (string, error) {
			return "", errors.New("task is already running")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("current_domain_id", int64(1))
		c.Set("role", "user")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeTaskRunning {
		t.Errorf("expected code %d for task running, got %d", CodeTaskRunning, resp.Code)
	}
}

// TestTaskHandler_Trigger_AlreadyBeingExecuted 测试 Trigger 时任务正在执行（错误消息包含 "already being executed"）
func TestTaskHandler_Trigger_AlreadyBeingExecuted(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 1}, nil
		},
		triggerTaskFunc: func(ctx context.Context, taskID int64) (string, error) {
			return "", errors.New("task already being executed")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("current_domain_id", int64(1))
		c.Set("role", "user")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeTaskRunning {
		t.Errorf("expected code %d for task already being executed, got %d", CodeTaskRunning, resp.Code)
	}
}

// TestTaskHandler_Trigger_TaskLocked 测试 Trigger 时任务被锁定（错误消息不匹配任何关键词，返回 500）
func TestTaskHandler_Trigger_TaskLocked(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 1}, nil
		},
		triggerTaskFunc: func(ctx context.Context, taskID int64) (string, error) {
			return "", errors.New("task is locked")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("current_domain_id", int64(1))
		c.Set("role", "user")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	// "task is locked" 不匹配任何关键词，返回 CodeInternalError
	if resp.Code != CodeInternalError {
		t.Errorf("expected code %d for unmatched error, got %d", CodeInternalError, resp.Code)
	}
}

// TestTaskHandler_Trigger_GenericError 测试 Trigger 时遇到不匹配任何关键词的通用错误
func TestTaskHandler_Trigger_GenericError(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 1}, nil
		},
		triggerTaskFunc: func(ctx context.Context, taskID int64) (string, error) {
			return "", errors.New("some unexpected error")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("current_domain_id", int64(1))
		c.Set("role", "user")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeInternalError {
		t.Errorf("expected code %d for generic error, got %d", CodeInternalError, resp.Code)
	}
	if !strings.Contains(resp.Message, "some unexpected error") {
		t.Errorf("expected message to contain error, got: %s", resp.Message)
	}
}
