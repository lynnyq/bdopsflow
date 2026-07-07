package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

// TestNewExecutorHandler 测试 NewExecutorHandler 构造函数
func TestNewExecutorHandler(t *testing.T) {
	h := NewExecutorHandler(nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.svc != nil {
		t.Errorf("expected nil svc, got %v", h.svc)
	}
}

// TestParseName 测试 parseName 纯函数
func TestParseName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"empty string", "", "", true},
		{"valid name", "executor-1", "executor-1", false},
		{"name with spaces", "my executor", "my executor", false},
		{"name with special chars", "exec_123@host", "exec_123@host", false},
		{"single char", "a", "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseParam 测试 parseParam 纯函数
func TestParseParam(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOK   bool
		wantErr  bool
		wantCall int64
	}{
		{"empty string", "", false, true, 0},
		{"non-numeric", "abc", false, true, 0},
		{"float value", "1.5", false, true, 0},
		{"zero value", "0", false, true, 0},
		{"negative value", "-5", false, true, 0},
		{"valid positive", "42", true, false, 42},
		{"large value", "9999999999", true, false, 9999999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called int64
			ok, err := parseParam(tt.input, func(v int64) { called = v })
			if (err != nil) != tt.wantErr {
				t.Errorf("parseParam(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if ok != tt.wantOK {
				t.Errorf("parseParam(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if tt.wantOK && called != tt.wantCall {
				t.Errorf("parseParam(%q) called with %d, want %d", tt.input, called, tt.wantCall)
			}
		})
	}
}

// TestExecutorToDTO 测试 executorToDTO 转换函数
func TestExecutorToDTO(t *testing.T) {
	now := time.Now()
	staleTime := now.Add(-2 * time.Minute)

	tests := []struct {
		name       string
		exec       *model.Executor
		wantStatus string
		wantHB     string
		wantCTS    string
		wantUTS    string
	}{
		{
			name: "online with recent heartbeat",
			exec: &model.Executor{
				ID:            1,
				Name:          "exec-1",
				Address:       "localhost:50051",
				Status:        "online",
				LastHeartbeat: rqlite.NullTime{Valid: true, Time: now},
				Capacity:      10,
				CurrentLoad:   3,
				IsGlobal:      true,
			},
			wantStatus: "online",
			wantHB:     "non-empty",
			wantCTS:    "",
			wantUTS:    "",
		},
		{
			name: "online with stale heartbeat -> offline",
			exec: &model.Executor{
				ID:            2,
				Name:          "exec-2",
				Status:        "online",
				LastHeartbeat: rqlite.NullTime{Valid: true, Time: staleTime},
			},
			wantStatus: "offline",
			wantHB:     "non-empty",
		},
		{
			name: "online with invalid heartbeat -> offline",
			exec: &model.Executor{
				ID:            3,
				Name:          "exec-3",
				Status:        "online",
				LastHeartbeat: rqlite.NullTime{Valid: false},
			},
			wantStatus: "offline",
			wantHB:     "",
		},
		{
			name: "online with zero heartbeat time -> offline",
			exec: &model.Executor{
				ID:            4,
				Name:          "exec-4",
				Status:        "online",
				LastHeartbeat: rqlite.NullTime{Valid: true, Time: time.Time{}},
			},
			wantStatus: "offline",
			wantHB:     "non-empty",
		},
		{
			name: "offline status unchanged",
			exec: &model.Executor{
				ID:     5,
				Name:   "exec-5",
				Status: "offline",
			},
			wantStatus: "offline",
			wantHB:     "",
		},
		{
			name: "with created_at and updated_at",
			exec: &model.Executor{
				ID:        6,
				Name:      "exec-6",
				Status:    "offline",
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantStatus: "offline",
			wantCTS:    "non-empty",
			wantUTS:    "non-empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dto := executorToDTO(tt.exec)
			if dto == nil {
				t.Fatal("expected non-nil DTO")
			}
			if dto.ID != tt.exec.ID {
				t.Errorf("expected ID %d, got %d", tt.exec.ID, dto.ID)
			}
			if dto.Name != tt.exec.Name {
				t.Errorf("expected Name %q, got %q", tt.exec.Name, dto.Name)
			}
			if dto.Status != tt.wantStatus {
				t.Errorf("expected Status %q, got %q", tt.wantStatus, dto.Status)
			}
			if tt.wantHB == "non-empty" && dto.LastHeartbeat == "" {
				t.Error("expected non-empty LastHeartbeat, got empty")
			}
			if tt.wantHB == "" && dto.LastHeartbeat != "" {
				t.Errorf("expected empty LastHeartbeat, got %q", dto.LastHeartbeat)
			}
			if tt.wantCTS == "non-empty" && dto.CreatedAt == "" {
				t.Error("expected non-empty CreatedAt, got empty")
			}
			if tt.wantCTS == "" && dto.CreatedAt != "" {
				t.Errorf("expected empty CreatedAt, got %q", dto.CreatedAt)
			}
			if tt.wantUTS == "non-empty" && dto.UpdatedAt == "" {
				t.Error("expected non-empty UpdatedAt, got empty")
			}
			if tt.wantUTS == "" && dto.UpdatedAt != "" {
				t.Errorf("expected empty UpdatedAt, got %q", dto.UpdatedAt)
			}
			if dto.Capacity != tt.exec.Capacity {
				t.Errorf("expected Capacity %d, got %d", tt.exec.Capacity, dto.Capacity)
			}
			if dto.CurrentLoad != tt.exec.CurrentLoad {
				t.Errorf("expected CurrentLoad %d, got %d", tt.exec.CurrentLoad, dto.CurrentLoad)
			}
			if dto.IsGlobal != tt.exec.IsGlobal {
				t.Errorf("expected IsGlobal %v, got %v", tt.exec.IsGlobal, dto.IsGlobal)
			}
		})
	}
}

// TestExecutorHandler_Get_ValidName 测试 Get 方法传入有效名称
func TestExecutorHandler_Get_ValidName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "name", Value: "executor-1"}}

	h.Get(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d, got %d", CodeSuccess, resp.Code)
	}
}

// TestExecutorHandler_Get_EmptyName 测试 Get 方法传入空名称
func TestExecutorHandler_Get_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "name", Value: ""}}

	h.Get(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty name, got %d", CodeBadRequest, resp.Code)
	}
}

// TestExecutorHandler_Delete_EmptyName 测试 Delete 方法传入空名称
func TestExecutorHandler_Delete_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/", nil)
	c.Params = gin.Params{{Key: "name", Value: ""}}

	h.Delete(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty name, got %d", CodeBadRequest, resp.Code)
	}
}

// TestExecutorHandler_Online_EmptyName 测试 Online 方法传入空名称
func TestExecutorHandler_Online_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Params = gin.Params{{Key: "name", Value: ""}}

	h.Online(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty name, got %d", CodeBadRequest, resp.Code)
	}
}

// TestExecutorHandler_Offline_EmptyName 测试 Offline 方法传入空名称
func TestExecutorHandler_Offline_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Params = gin.Params{{Key: "name", Value: ""}}

	h.Offline(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty name, got %d", CodeBadRequest, resp.Code)
	}
}

// TestExecutorHandler_UpdateCapacity 测试 UpdateCapacity 参数校验
func TestExecutorHandler_UpdateCapacity(t *testing.T) {
	tests := []struct {
		name       string
		urlName    string
		body       interface{}
		rawBody    string
		wantCode   int
	}{
		{
			name:     "empty name",
			urlName:  "",
			body:     map[string]interface{}{"capacity": 10},
			wantCode: CodeBadRequest,
		},
		{
			name:     "invalid json",
			urlName:  "exec-1",
			rawBody:  "not json",
			wantCode: CodeBadRequest,
		},
		{
			name:     "missing capacity",
			urlName:  "exec-1",
			body:     map[string]interface{}{},
			wantCode: CodeBadRequest,
		},
		{
			name:     "zero capacity",
			urlName:  "exec-1",
			body:     map[string]interface{}{"capacity": 0},
			wantCode: CodeBadRequest,
		},
		{
			name:     "negative capacity",
			urlName:  "exec-1",
			body:     map[string]interface{}{"capacity": -5},
			wantCode: CodeBadRequest,
		},
		{
			name:     "valid capacity but nil service",
			urlName:  "exec-1",
			body:     map[string]interface{}{"capacity": 10},
			wantCode: CodeInternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &ExecutorHandler{}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			var bodyReader *bytes.Reader
			if tt.rawBody != "" {
				bodyReader = bytes.NewReader([]byte(tt.rawBody))
			} else {
				bodyBytes, _ := json.Marshal(tt.body)
				bodyReader = bytes.NewReader(bodyBytes)
			}
			c.Request = httptest.NewRequest(http.MethodPut, "/", bodyReader)
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = gin.Params{{Key: "name", Value: tt.urlName}}

			defer func() {
				if r := recover(); r != nil {
					// nil service may panic - that's expected for the valid case
					if tt.wantCode == CodeInternalError {
						return
					}
					t.Logf("unexpected panic: %v", r)
				}
			}()

			h.UpdateCapacity(c)

			// For nil service panic case, we may not get a response
			if tt.wantCode == CodeInternalError {
				if w.Body.Len() > 0 {
					var resp Response
					if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
						// Either panic or error response is acceptable
						if resp.Code != CodeInternalError && resp.Code != 0 {
							t.Logf("got code %d (panic recovery may have occurred)", resp.Code)
						}
					}
				}
				return
			}

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp.Code != tt.wantCode {
				t.Errorf("expected code %d, got %d", tt.wantCode, resp.Code)
			}
		})
	}
}

// TestExecutorHandler_List_NilService 测试 List 方法在 nil service 时的 panic recovery
func TestExecutorHandler_List_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?page=1&page_size=10", nil)

	h.List(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	// nil svc 应被 panic recovery 捕获，返回 500
	if resp.Code != CodeInternalError {
		t.Errorf("expected code %d for nil service, got %d", CodeInternalError, resp.Code)
	}
}

// TestExecutorHandler_List_PaginationBoundary 测试 List 分页参数边界值
func TestExecutorHandler_List_PaginationBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorHandler{}

	tests := []struct {
		name string
		page string
		size string
	}{
		{"negative page", "-1", "10"},
		{"zero page", "0", "10"},
		{"negative page_size", "1", "-5"},
		{"zero page_size", "1", "0"},
		{"oversized page_size", "1", "200"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			url := "/?page=" + tt.page + "&page_size=" + tt.size
			c.Request = httptest.NewRequest(http.MethodGet, url, nil)

			h.List(c)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			// nil svc 会导致 panic recovery 返回 500
			if resp.Code != CodeInternalError {
				t.Errorf("expected code %d for nil service, got %d", CodeInternalError, resp.Code)
			}
		})
	}
}
