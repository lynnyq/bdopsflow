package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

type mockTaskService struct {
	createTaskFunc      func(ctx context.Context, query string, args ...interface{}) (*model.Task, error)
	getTaskByIDFunc     func(ctx context.Context, id int64) (*model.Task, error)
	listTasksFunc       func(ctx context.Context) ([]*model.Task, error)
	updateTaskFunc      func(ctx context.Context, id int64, task *model.Task) error
	deleteTaskFunc      func(ctx context.Context, id int64) error
	triggerTaskFunc     func(ctx context.Context, taskID int64) (string, error)
	getTaskExecsFunc    func(ctx context.Context, taskID int64) ([]*model.TaskExecution, error)
	getTaskLogsFunc     func(ctx context.Context, executionID string) ([]*model.TaskLog, error)

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

func (m *mockTaskService) ListTasks(ctx context.Context) ([]*model.Task, error) {
	if m.listTasksFunc != nil {
		return m.listTasksFunc(ctx)
	}
	return []*model.Task{}, nil
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

func setupTestRouter(handler *TaskHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/tasks", handler.Create)
	r.GET("/api/tasks", handler.List)
	r.GET("/api/tasks/:id", handler.Get)
	r.PUT("/api/tasks/:id", handler.Update)
	r.DELETE("/api/tasks/:id", handler.Delete)
	r.POST("/api/tasks/:id/trigger", handler.Trigger)
	r.GET("/api/tasks/:id/executions", handler.Executions)
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
	req, _ := http.NewRequest("POST", "/api/tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Errorf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
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
	req, _ := http.NewRequest("POST", "/api/tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Errorf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
	}

	if !strings.Contains(mock.lastQuery, "workflow_id") {
		t.Errorf("INSERT should contain workflow_id when provided, got query: %s", mock.lastQuery)
	}

	if len(mock.lastArgs) != 13 {
		t.Errorf("expected 13 args with workflow_id, got %d", len(mock.lastArgs))
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
	req, _ := http.NewRequest("POST", "/api/tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Errorf("expected status 500 for service error, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "database connection refused") {
		t.Errorf("expected error message about database, got: %s", resp["error"])
	}
}

func TestCreateTask_InvalidJSON(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("POST", "/api/tasks", bytes.NewBuffer([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected status 400 for invalid JSON, got %d", w.Code)
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
	req, _ := http.NewRequest("POST", "/api/tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Errorf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
	}

	var defaultTimeoutFound, defaultRetryFound, defaultDomainFound bool
	for _, arg := range mock.lastArgs {
		switch v := arg.(type) {
		case int64:
			if v == 0 {
				defaultTimeoutFound = true
			}
			if v == 3 {
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
		t.Errorf("retry_count default 3 not applied, args: %v", mock.lastArgs)
	}
	if !defaultDomainFound {
		t.Errorf("domain_id default 1 not applied, args: %v", mock.lastArgs)
	}
}

func TestListTasks(t *testing.T) {
	mock := &mockTaskService{
		listTasksFunc: func(ctx context.Context) ([]*model.Task, error) {
			return []*model.Task{
				{ID: 1, Name: "task1", Type: "http", Status: "pending"},
				{ID: 2, Name: "task2", Type: "shell", Status: "success"},
			}, nil
		},
	}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Items []model.Task `json:"items"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(resp.Items))
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

	req, _ := http.NewRequest("GET", "/api/tasks/999", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteTask(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("DELETE", "/api/tasks/1", nil)
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

	req, _ := http.NewRequest("POST", "/api/tasks/1/trigger", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["message"] != "triggered" {
		t.Errorf("expected message 'triggered', got %v", resp["message"])
	}
	execID, ok := resp["execution_id"].(string)
	if !ok || execID == "" {
		t.Errorf("expected execution_id in response, got %v", resp["execution_id"])
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
	req, _ := http.NewRequest("PUT", "/api/tasks/1", bytes.NewBuffer(jsonBody))
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

	req, _ := http.NewRequest("PUT", "/api/tasks/invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
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

	req, _ := http.NewRequest("GET", "/api/tasks/1/executions", nil)
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

	req, _ := http.NewRequest("GET", "/api/tasks/invalid/executions", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}

func TestTaskHandler_Delete_InvalidID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("DELETE", "/api/tasks/invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}

func TestTaskHandler_Delete_NegativeID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("DELETE", "/api/tasks/-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative ID, got %d", w.Code)
	}
}

func TestTaskHandler_Trigger_InvalidID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("POST", "/api/tasks/invalid/trigger", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}

func TestTaskHandler_Get_InvalidID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("GET", "/api/tasks/invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}

func TestTaskHandler_Get_NegativeID(t *testing.T) {
	mock := &mockTaskService{}
	handler := newTaskHandlerWithSvc(mock)
	router := setupTestRouter(handler)

	req, _ := http.NewRequest("GET", "/api/tasks/-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative ID, got %d", w.Code)
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
	req, _ := http.NewRequest("POST", "/api/tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing name, got %d", w.Code)
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
	req, _ := http.NewRequest("POST", "/api/tasks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing type, got %d", w.Code)
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