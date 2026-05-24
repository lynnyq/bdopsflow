package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

type mockTaskService struct {
	createTaskFunc           func(ctx context.Context, query string, args ...interface{}) (*model.Task, error)
	getTaskByIDFunc          func(ctx context.Context, id int64) (*model.Task, error)
	listTasksFunc            func(ctx context.Context, domainID int64, role string, page, pageSize int) ([]*model.Task, int, error)
	updateTaskFunc           func(ctx context.Context, id int64, task *model.Task) error
	deleteTaskFunc           func(ctx context.Context, id int64) error
	triggerTaskFunc          func(ctx context.Context, taskID int64) (string, error)
	getTaskExecsFunc         func(ctx context.Context, taskID int64) ([]*model.TaskExecution, error)
	getTaskLogsFunc          func(ctx context.Context, executionID string) ([]*model.TaskLog, error)
	listExecutorsByDomainFunc func(ctx context.Context, domainID int64) ([]*model.Executor, error)
	getDomainNameFunc        func(ctx context.Context, domainID int64) string
	isLeaderFunc             func() bool
	forwardToLeaderFunc      func(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error)

	lastQuery string
	lastArgs  []interface{}
}

func (m *mockTaskService) CreateTask(ctx context.Context, query string, args ...interface{}) (*model.Task, error) {
	m.lastQuery = query
	m.lastArgs = args
	if m.createTaskFunc != nil {
		return m.createTaskFunc(ctx, query, args...)
	}
	return &model.Task{ID: 1, Name: "test", Type: "http", Status: "pending"}, nil
}

func (m *mockTaskService) GetTaskByID(ctx context.Context, id int64) (*model.Task, error) {
	if m.getTaskByIDFunc != nil {
		return m.getTaskByIDFunc(ctx, id)
	}
	return &model.Task{ID: id, Name: "test", Type: "http", Status: "pending"}, nil
}

func (m *mockTaskService) ListTasks(ctx context.Context, domainID int64, role string, page, pageSize int) ([]*model.Task, int, error) {
	if m.listTasksFunc != nil {
		return m.listTasksFunc(ctx, domainID, role, page, pageSize)
	}
	return []*model.Task{}, 0, nil
}

func (m *mockTaskService) UpdateTask(ctx context.Context, id int64, task *model.Task) error {
	if m.updateTaskFunc != nil {
		return m.updateTaskFunc(ctx, id, task)
	}
	return nil
}

func (m *mockTaskService) DeleteTask(ctx context.Context, id int64) error {
	if m.deleteTaskFunc != nil {
		return m.deleteTaskFunc(ctx, id)
	}
	return nil
}

func (m *mockTaskService) TriggerTask(ctx context.Context, taskID int64) (string, error) {
	if m.triggerTaskFunc != nil {
		return m.triggerTaskFunc(ctx, taskID)
	}
	return fmt.Sprintf("exec-%d-123456", taskID), nil
}

func (m *mockTaskService) GetTaskExecutions(ctx context.Context, taskID int64) ([]*model.TaskExecution, error) {
	if m.getTaskExecsFunc != nil {
		return m.getTaskExecsFunc(ctx, taskID)
	}
	return []*model.TaskExecution{}, nil
}

func (m *mockTaskService) GetTaskLogs(ctx context.Context, executionID string) ([]*model.TaskLog, error) {
	if m.getTaskLogsFunc != nil {
		return m.getTaskLogsFunc(ctx, executionID)
	}
	return []*model.TaskLog{}, nil
}

func (m *mockTaskService) ListExecutorsByDomain(ctx context.Context, domainID int64) ([]*model.Executor, error) {
	if m.listExecutorsByDomainFunc != nil {
		return m.listExecutorsByDomainFunc(ctx, domainID)
	}
	return []*model.Executor{}, nil
}

func (m *mockTaskService) GetDomainName(ctx context.Context, domainID int64) string {
	if m.getDomainNameFunc != nil {
		return m.getDomainNameFunc(ctx, domainID)
	}
	return fmt.Sprintf("领域 %d", domainID)
}

func (m *mockTaskService) IsLeader() bool {
	if m.isLeaderFunc != nil {
		return m.isLeaderFunc()
	}
	return true
}

func (m *mockTaskService) ForwardToLeader(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error) {
	if m.forwardToLeaderFunc != nil {
		return m.forwardToLeaderFunc(ctx, method, path, body)
	}
	return nil, 503, fmt.Errorf("not implemented")
}

func setupTestRouter(handler *TaskHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/bdopsflow_tasks", handler.Create)
	r.GET("/api/bdopsflow_tasks", handler.List)
	r.GET("/api/bdopsflow_tasks/:id", handler.Get)
	r.PUT("/api/bdopsflow_tasks/:id", handler.Update)
	r.DELETE("/api/bdopsflow_tasks/:id", handler.Delete)
	r.POST("/api/bdopsflow_tasks/:id/trigger", handler.Trigger)
	r.GET("/api/bdopsflow_tasks/:id/executions", handler.Executions)
	return r
}

func TestCreateTask_WithoutWorkflowID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	body := map[string]interface{}{
		"name":            "测试HTTP任务",
		"type":            "http",
		"config":          `{"url":"http://example.com","method":"GET"}`,
		"cron_expression": "",
		"timeout_seconds": 300,
		"retry_count":     3,
		"retry_interval":  5,
		"domain_id":       1,
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var createResp Response
	json.Unmarshal(w.Body.Bytes(), &createResp)
	if createResp.Code != CodeSuccess {
		t.Errorf("expected code 0, got %d, body: %s", createResp.Code, w.Body.String())
	}

	if strings.Contains(mock.lastQuery, "workflow_id") {
		t.Errorf("INSERT should NOT contain workflow_id when not provided, got query: %s", mock.lastQuery)
	}

	t.Logf("SQL: %s", mock.lastQuery)
	t.Logf("Args: %v", mock.lastArgs)
}

func TestCreateTask_WithWorkflowID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	wfID := int64(42)
	body := map[string]interface{}{
		"name":            "工作流任务",
		"type":            "shell",
		"config":          `{"script":"echo hello"}`,
		"cron_expression": "0 0 * * *",
		"timeout_seconds": 120,
		"retry_count":     2,
		"retry_interval":  10,
		"domain_id":       2,
		"workflow_id":     wfID,
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var createResp Response
	json.Unmarshal(w.Body.Bytes(), &createResp)
	if createResp.Code != CodeSuccess {
		t.Errorf("expected code 0, got %d, body: %s", createResp.Code, w.Body.String())
	}

	if !strings.Contains(mock.lastQuery, "workflow_id") {
		t.Errorf("INSERT should contain workflow_id when provided, got query: %s", mock.lastQuery)
	}

	if len(mock.lastArgs) != 15 {
		t.Errorf("expected 15 args with workflow_id, got %d", len(mock.lastArgs))
	}

	foundWFID := false
	for _, arg := range mock.lastArgs {
		if id, ok := arg.(int64); ok && id == 42 {
			foundWFID = true
			break
		}
	}
	if !foundWFID {
		t.Errorf("workflow_id=42 not found in args: %v", mock.lastArgs)
	}

	t.Logf("SQL: %s", mock.lastQuery)
	t.Logf("Args: %v", mock.lastArgs)
}

func TestCreateTask_ServiceError(t *testing.T) {
	mock := &mockTaskService{
		createTaskFunc: func(ctx context.Context, query string, args ...interface{}) (*model.Task, error) {
			return nil, fmt.Errorf("database connection refused")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	body := map[string]interface{}{
		"name":   "test",
		"type":   "http",
		"config": "{}",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeInternalError {
		t.Errorf("expected code 500 for service error, got %d", resp.Code)
	}

	var respMap map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &respMap)
	if !strings.Contains(respMap["message"].(string), "database connection refused") {
		t.Errorf("expected error message about database, got: %s", respMap["message"])
	}
}

func TestCreateTask_InvalidJSON(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks", bytes.NewBuffer([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for invalid JSON, got %d", resp.Code)
	}
}

func TestCreateTask_DefaultValues(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	body := map[string]interface{}{
		"name":   "minimal task",
		"type":   "http",
		"config": "{}",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var createResp Response
	json.Unmarshal(w.Body.Bytes(), &createResp)
	if createResp.Code != CodeSuccess {
		t.Errorf("expected code 0, got %d, body: %s", createResp.Code, w.Body.String())
	}

	var defaultTimeoutFound, defaultRetryFound, defaultDomainFound bool
	for _, arg := range mock.lastArgs {
		switch v := arg.(type) {
		case int64:
			if v == 0 {
				defaultTimeoutFound = true
				defaultRetryFound = true
			}
			if v == 1 {
				defaultDomainFound = true
			}
		}
	}

	if !defaultTimeoutFound {
		t.Errorf("timeout_seconds default 0 not applied, args: %v", mock.lastArgs)
	}
	if !defaultRetryFound {
		t.Errorf("retry_count default 0 not applied, args: %v", mock.lastArgs)
	}
	if !defaultDomainFound {
		t.Errorf("domain_id default 1 not applied, args: %v", mock.lastArgs)
	}
}

func TestListTasks(t *testing.T) {
	mock := &mockTaskService{
		listTasksFunc: func(ctx context.Context, domainID int64, role string, page, pageSize int) ([]*model.Task, int, error) {
			return []*model.Task{
				{ID: 1, Name: "task1", Type: "http", Status: "pending"},
				{ID: 2, Name: "task2", Type: "shell", Status: "success"},
			}, 2, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_tasks", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Code    int                 `json:"code"`
		Status  string              `json:"status"`
		Message string              `json:"message"`
		Data    struct {
			Items []model.Task `json:"items"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data.Items) != 2 {
		t.Errorf("expected 2 bdopsflow_tasks, got %d", len(resp.Data.Items))
	}
}

func TestGetTask_NotFound(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return nil, fmt.Errorf("task not found")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_tasks/999", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeNotFound {
		t.Errorf("expected code 404, got %d", resp.Code)
	}
}

func TestDeleteTask(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("DELETE", "/api/bdopsflow_tasks/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestTriggerTask(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	// 新的统一响应格式：data 字段包含实际数据
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Errorf("expected data field in response, got: %v", resp)
		return
	}
	if data["message"] != "triggered" {
		t.Errorf("expected message 'triggered', got %v", data["message"])
	}
	execID, ok := data["execution_id"].(string)
	if !ok || execID == "" {
		t.Errorf("expected execution_id in response, got %v", data["execution_id"])
	}
}

func TestTaskHandler_Update(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	body := map[string]interface{}{
		"name":   "updated task",
		"type":   "shell",
		"config": `{"script":"echo updated"}`,
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/api/bdopsflow_tasks/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestTaskHandler_Update_InvalidID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("PUT", "/api/bdopsflow_tasks/invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 for invalid ID, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for invalid ID, got %d", resp.Code)
	}
}

func TestTaskHandler_Executions(t *testing.T) {
	mock := &mockTaskService{
		getTaskExecsFunc: func(ctx context.Context, taskID int64) ([]*model.TaskExecution, error) {
			return []*model.TaskExecution{
				{ID: 1, TaskID: taskID, ExecutionID: "exec-1", Status: "success"},
				{ID: 2, TaskID: taskID, ExecutionID: "exec-2", Status: "failed"},
			}, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_tasks/1/executions", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestTaskHandler_Executions_InvalidID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_tasks/invalid/executions", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 for invalid ID, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for invalid ID, got %d", resp.Code)
	}
}

func TestTaskHandler_Delete_InvalidID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("DELETE", "/api/bdopsflow_tasks/invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 for invalid ID, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for invalid ID, got %d", resp.Code)
	}
}

func TestTaskHandler_Delete_NegativeID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("DELETE", "/api/bdopsflow_tasks/-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 for negative ID, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for negative ID, got %d", resp.Code)
	}
}

func TestTaskHandler_Trigger_InvalidID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/invalid/trigger", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 for invalid ID, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for invalid ID, got %d", resp.Code)
	}
}

func TestTaskHandler_Get_InvalidID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_tasks/invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 for invalid ID, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for invalid ID, got %d", resp.Code)
	}
}

func TestTaskHandler_Get_NegativeID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_tasks/-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 for negative ID, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for negative ID, got %d", resp.Code)
	}
}

func TestTaskHandler_Create_MissingName(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	body := map[string]interface{}{
		"type":   "http",
		"config": "{}",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 for missing name, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for missing name, got %d", resp.Code)
	}
}

func TestTaskHandler_Create_MissingType(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	body := map[string]interface{}{
		"name":   "test",
		"config": "{}",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 for missing type, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for missing type, got %d", resp.Code)
	}
}

func TestSafeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"whitespace", "   ", ""},
		{"normal string", "  hello  ", "hello"},
		{"already trimmed", "hello", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeString(tt.input)
			if result != tt.expected {
				t.Errorf("safeString(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal string", "hello", "hello"},
		{"string with quotes", `hello "world"`, `hello \"world\"`},
		{"string with backslash", `hello\world`, `hello\\world`},
		{"string with newline", "hello\nworld", `hello\nworld`},
		{"string with tab", "hello\tworld", `hello\tworld`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeJSON(tt.input)
			if result != tt.expected {
				t.Errorf("escapeJSON(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsExecutorOnline(t *testing.T) {
	now := time.Now()
	justNow := now.Add(-10 * time.Second)
	halfMinuteAgo := now.Add(-15 * time.Second)
	oneMinuteAgo := now.Add(-30 * time.Second)

	nullTime := rqlite.NullTime{Valid: false}

	tests := []struct {
		name     string
		exec     *model.Executor
		expected bool
	}{
		{
			name:     "status offline should return false",
			exec:     &model.Executor{Status: "offline", LastHeartbeat: rqlite.NullTime{Time: now, Valid: true}},
			expected: false,
		},
		{
			name:     "status online but no heartbeat should return false",
			exec:     &model.Executor{Status: "online", LastHeartbeat: nullTime},
			expected: false,
		},
		{
			name:     "status online with recent heartbeat should return true",
			exec:     &model.Executor{Status: "online", LastHeartbeat: rqlite.NullTime{Time: justNow, Valid: true}},
			expected: true,
		},
		{
			name:     "status online with heartbeat within timeout should return true",
			exec:     &model.Executor{Status: "online", LastHeartbeat: rqlite.NullTime{Time: halfMinuteAgo, Valid: true}},
			expected: true,
		},
		{
			name:     "status online with heartbeat beyond timeout should return false",
			exec:     &model.Executor{Status: "online", LastHeartbeat: rqlite.NullTime{Time: oneMinuteAgo, Valid: true}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isExecutorOnline(tt.exec)
			if result != tt.expected {
				t.Errorf("isExecutorOnline() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestCreateTask_NoAvailableExecutors(t *testing.T) {
	mock := &mockTaskService{
		listExecutorsByDomainFunc: func(ctx context.Context, domainID int64) ([]*model.Executor, error) {
			return []*model.Executor{}, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	body := map[string]interface{}{
		"name":      "任务无执行器",
		"type":      "http",
		"config":    `{"url":"http://example.com","method":"GET"}`,
		"domain_id": 1,
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var createResp Response
	json.Unmarshal(w.Body.Bytes(), &createResp)
	if createResp.Code != CodeSuccess {
		t.Errorf("expected code 0, got %d, body: %s", createResp.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Errorf("expected data field in response, got: %v", resp)
		return
	}

	if data["has_available_executors"] != false {
		t.Errorf("expected has_available_executors to be false, got %v", data["has_available_executors"])
	}
}

func TestCreateTask_WithAvailableExecutor(t *testing.T) {
	now := time.Now()
	mock := &mockTaskService{
		listExecutorsByDomainFunc: func(ctx context.Context, domainID int64) ([]*model.Executor, error) {
			return []*model.Executor{
				{
					ID:            1,
					Name:          "executor-1",
					Status:        "online",
					LastHeartbeat: rqlite.NullTime{Time: now.Add(-10 * time.Second), Valid: true},
					Capacity:      10,
					CurrentLoad:   5,
				},
			}, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	body := map[string]interface{}{
		"name":      "任务有执行器",
		"type":      "http",
		"config":    `{"url":"http://example.com","method":"GET"}`,
		"domain_id": 1,
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var createResp Response
	json.Unmarshal(w.Body.Bytes(), &createResp)
	if createResp.Code != CodeSuccess {
		t.Errorf("expected code 0, got %d, body: %s", createResp.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Errorf("expected data field in response, got: %v", resp)
		return
	}

	if data["has_available_executors"] != true {
		t.Errorf("expected has_available_executors to be true, got %v", data["has_available_executors"])
	}
}

func TestCreateTask_ExecutorAtCapacity(t *testing.T) {
	now := time.Now()
	mock := &mockTaskService{
		listExecutorsByDomainFunc: func(ctx context.Context, domainID int64) ([]*model.Executor, error) {
			return []*model.Executor{
				{
					ID:            1,
					Name:          "executor-1",
					Status:        "online",
					LastHeartbeat: rqlite.NullTime{Time: now.Add(-10 * time.Second), Valid: true},
					Capacity:      10,
					CurrentLoad:   10,
				},
			}, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	body := map[string]interface{}{
		"name":      "任务执行器满载",
		"type":      "http",
		"config":    `{"url":"http://example.com","method":"GET"}`,
		"domain_id": 1,
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var createResp Response
	json.Unmarshal(w.Body.Bytes(), &createResp)
	if createResp.Code != CodeSuccess {
		t.Errorf("expected code 0, got %d, body: %s", createResp.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Errorf("expected data field in response, got: %v", resp)
		return
	}

	if data["has_available_executors"] != false {
		t.Errorf("expected has_available_executors to be false when executor at capacity, got %v", data["has_available_executors"])
	}
}

func setupTestRouterWithAuth(handler *TaskHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authGroup := r.Group("/api/bdopsflow_tasks")
	authGroup.Use(func(c *gin.Context) {
		if domainID, exists := c.Get("inject_domain_id"); exists {
			c.Set("domain_id", domainID)
		}
		if role, exists := c.Get("inject_role"); exists {
			c.Set("role", role)
		}
		c.Next()
	})
	{
		authGroup.POST("/:id/trigger", handler.Trigger)
	}
	return r
}

func TestTrigger_PermissionDenied_DifferentDomain(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 2}, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("domain_id", int64(1))
		c.Set("role", "user")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeForbidden {
		t.Errorf("expected code %d for cross-domain access, got %d", CodeForbidden, resp.Code)
	}
}

func TestTrigger_PermissionAllowed_SameDomain(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 1}, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("domain_id", int64(1))
		c.Set("role", "user")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d for same-domain access, got %d, body: %s", CodeSuccess, resp.Code, w.Body.String())
	}
}

func TestTrigger_PermissionAllowed_SystemAdmin(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 99}, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("domain_id", int64(1))
		c.Set("role", "system_admin")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d for system_admin cross-domain access, got %d", CodeSuccess, resp.Code)
	}
}

func TestTrigger_PermissionAllowed_AdminRole(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 99}, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("domain_id", int64(1))
		c.Set("role", "admin")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d for admin cross-domain access, got %d", CodeSuccess, resp.Code)
	}
}

func TestTrigger_TaskNotFound(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return nil, fmt.Errorf("task not found")
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := gin.New()
	gin.SetMode(gin.TestMode)

	router.POST("/api/bdopsflow_tasks/:id/trigger", func(c *gin.Context) {
		c.Set("domain_id", int64(1))
		c.Set("role", "user")
		handler.Trigger(c)
	})

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/999/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for task not found, got %d", CodeBadRequest, resp.Code)
	}
}

func TestTrigger_NegativeID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/-1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for negative ID, got %d", CodeBadRequest, resp.Code)
	}
}

func TestTrigger_NoAuthContext(t *testing.T) {
	mock := &mockTaskService{
		getTaskByIDFunc: func(ctx context.Context, id int64) (*model.Task, error) {
			return &model.Task{ID: id, Name: "test", DomainID: 0}, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_tasks/1/trigger", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code == CodeForbidden {
		t.Errorf("should not be forbidden when no auth context (domain_id defaults to 0, task domain_id is 0)")
	}
}
