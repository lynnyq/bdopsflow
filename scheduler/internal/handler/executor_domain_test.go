package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	rqlite "github.com/rqlite/gorqlite"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

// === NewExecutorDomainHandler 测试 ===

func TestNewExecutorDomainHandler(t *testing.T) {
	h := NewExecutorDomainHandler(nil, nil, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.svc != nil {
		t.Errorf("expected nil svc, got %v", h.svc)
	}
	if h.permissionSvc != nil {
		t.Errorf("expected nil permissionSvc, got %v", h.permissionSvc)
	}
	if h.userAdminSvc != nil {
		t.Errorf("expected nil userAdminSvc, got %v", h.userAdminSvc)
	}
}

// === parseInt64Param 测试 ===

func TestParseInt64Param(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"valid positive", "123", 123, false},
		{"valid one", "1", 1, false},
		{"empty string", "", 0, true},
		{"non-numeric", "abc", 0, true},
		{"float", "1.5", 0, true},
		{"zero", "0", 0, true},
		{"negative", "-1", 0, true},
		{"with spaces", " 123 ", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInt64Param(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInt64Param(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseInt64Param(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// === executorWithDomainsToDTO 测试 ===

func TestExecutorWithDomainsToDTO_OfflineStatus(t *testing.T) {
	exec := &model.ExecutorWithDomains{
		Executor: model.Executor{
			ID:            1,
			Name:          "test-executor",
			Address:       "localhost:50051",
			Status:        "offline",
			Capacity:      10,
			CurrentLoad:   3,
			LastHeartbeat: rqlite.NullTime{},
		},
		IsGlobal: true,
		Domains:  []*model.Domain{{ID: 1, Name: "domain1"}},
	}

	dto := executorWithDomainsToDTO(exec)

	if dto.ID != 1 {
		t.Errorf("expected ID 1, got %d", dto.ID)
	}
	if dto.Name != "test-executor" {
		t.Errorf("expected Name test-executor, got %s", dto.Name)
	}
	if dto.Status != "offline" {
		t.Errorf("expected Status offline, got %s", dto.Status)
	}
	if dto.Capacity != 10 {
		t.Errorf("expected Capacity 10, got %d", dto.Capacity)
	}
	if dto.CurrentLoad != 3 {
		t.Errorf("expected CurrentLoad 3, got %d", dto.CurrentLoad)
	}
	if !dto.IsGlobal {
		t.Errorf("expected IsGlobal true, got false")
	}
	if dto.LastHeartbeat != "" {
		t.Errorf("expected empty LastHeartbeat, got %s", dto.LastHeartbeat)
	}
	if len(dto.Domains) != 1 || dto.Domains[0].Name != "domain1" {
		t.Errorf("expected 1 domain 'domain1', got %v", dto.Domains)
	}
}

func TestExecutorWithDomainsToDTO_OnlineButNoHeartbeat(t *testing.T) {
	// Status="online" but LastHeartbeat invalid → should become "offline"
	exec := &model.ExecutorWithDomains{
		Executor: model.Executor{
			ID:            2,
			Name:          "exec-no-hb",
			Status:        "online",
			LastHeartbeat: rqlite.NullTime{Valid: false},
		},
	}

	dto := executorWithDomainsToDTO(exec)

	if dto.Status != "offline" {
		t.Errorf("expected Status offline (no heartbeat), got %s", dto.Status)
	}
}

func TestExecutorWithDomainsToDTO_OnlineWithStaleHeartbeat(t *testing.T) {
	// Status="online" but LastHeartbeat is old → should become "offline"
	oldTime := time.Now().Add(-2 * time.Minute) // 2 分钟前，超过 30 秒阈值
	exec := &model.ExecutorWithDomains{
		Executor: model.Executor{
			ID:            3,
			Name:          "exec-stale",
			Status:        "online",
			LastHeartbeat: rqlite.NullTime{Time: oldTime, Valid: true},
		},
	}

	dto := executorWithDomainsToDTO(exec)

	if dto.Status != "offline" {
		t.Errorf("expected Status offline (stale heartbeat), got %s", dto.Status)
	}
	if dto.LastHeartbeat == "" {
		t.Errorf("expected non-empty LastHeartbeat, got empty")
	}
}

func TestExecutorWithDomainsToDTO_EmptyTimestamps(t *testing.T) {
	exec := &model.ExecutorWithDomains{
		Executor: model.Executor{
			ID:        4,
			Name:      "exec-empty-ts",
			Status:    "offline",
			CreatedAt: time.Time{}, // zero time
			UpdatedAt: time.Time{}, // zero time
		},
	}

	dto := executorWithDomainsToDTO(exec)

	if dto.CreatedAt != "" {
		t.Errorf("expected empty CreatedAt for zero time, got %s", dto.CreatedAt)
	}
	if dto.UpdatedAt != "" {
		t.Errorf("expected empty UpdatedAt for zero time, got %s", dto.UpdatedAt)
	}
}

func TestExecutorWithDomainsToDTO_WithTimestamps(t *testing.T) {
	now := time.Now()
	exec := &model.ExecutorWithDomains{
		Executor: model.Executor{
			ID:        5,
			Name:      "exec-ts",
			Status:    "offline",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	dto := executorWithDomainsToDTO(exec)

	if dto.CreatedAt == "" {
		t.Errorf("expected non-empty CreatedAt, got empty")
	}
	if dto.UpdatedAt == "" {
		t.Errorf("expected non-empty UpdatedAt, got empty")
	}
}

// === ExecutorDomainHandler.GetExecutorDomains 测试 ===

func TestExecutorDomainHandler_GetExecutorDomains_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "name", Value: ""}}

	h.GetExecutorDomains(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty name, got %d", CodeBadRequest, resp.Code)
	}
}

// === ExecutorDomainHandler.AssignDomains 测试 ===

func TestExecutorDomainHandler_AssignDomains_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	body := map[string]interface{}{"domain_ids": []int64{1}}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "name", Value: ""}}

	h.AssignDomains(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty name, got %d", CodeBadRequest, resp.Code)
	}
}

func TestExecutorDomainHandler_AssignDomains_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "name", Value: "test-exec"}}

	h.AssignDomains(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

func TestExecutorDomainHandler_AssignDomains_MissingDomainIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	body := map[string]interface{}{} // 缺少 domain_ids
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "name", Value: "test-exec"}}

	h.AssignDomains(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for missing domain_ids, got %d", CodeBadRequest, resp.Code)
	}
}

func TestExecutorDomainHandler_AssignDomains_InvalidDomainID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	body := map[string]interface{}{"domain_ids": []int64{0, -1}} // 无效 domain_id
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "name", Value: "test-exec"}}

	h.AssignDomains(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid domain_id, got %d", CodeBadRequest, resp.Code)
	}
}

func TestExecutorDomainHandler_AssignDomains_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	body := map[string]interface{}{"domain_ids": []int64{1}}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "name", Value: "test-exec"}}

	h.AssignDomains(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for no auth, got %d", CodeUnauthorized, resp.Code)
	}
}

func TestExecutorDomainHandler_AssignDomains_InvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	body := map[string]interface{}{"domain_ids": []int64{1}}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "name", Value: "test-exec"}}
	c.Set("user_id", "invalid") // 类型无效

	h.AssignDomains(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for invalid user_id, got %d", CodeUnauthorized, resp.Code)
	}
}

// === ExecutorDomainHandler.RemoveDomain 测试 ===

func TestExecutorDomainHandler_RemoveDomain_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/", nil)
	c.Params = gin.Params{{Key: "name", Value: ""}, {Key: "domain_id", Value: "1"}}

	h.RemoveDomain(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty name, got %d", CodeBadRequest, resp.Code)
	}
}

func TestExecutorDomainHandler_RemoveDomain_InvalidDomainID(t *testing.T) {
	tests := []struct {
		name     string
		domainID string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"zero", "0"},
		{"negative", "-1"},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &ExecutorDomainHandler{}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodDelete, "/", nil)
			c.Params = gin.Params{{Key: "name", Value: "test-exec"}, {Key: "domain_id", Value: tt.domainID}}

			h.RemoveDomain(c)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
			}
		})
	}
}

// === ExecutorDomainHandler.GetExecutorsWithDomains 未授权测试 ===

func TestExecutorDomainHandler_GetExecutorsWithDomains_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h.GetExecutorsWithDomains(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for no auth, got %d", CodeUnauthorized, resp.Code)
	}
}

func TestExecutorDomainHandler_GetExecutorsWithDomains_InvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("user_id", "invalid")

	h.GetExecutorsWithDomains(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for invalid user_id, got %d", CodeUnauthorized, resp.Code)
	}
}

// === ExecutorDomainHandler.GetAssignedTasks 测试 ===

func TestExecutorDomainHandler_GetAssignedTasks_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "name", Value: ""}}

	h.GetAssignedTasks(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty name, got %d", CodeBadRequest, resp.Code)
	}
}

// === ExecutorDomainHandler.CanDeleteExecutor 测试 ===

func TestExecutorDomainHandler_CanDeleteExecutor_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "name", Value: ""}}

	h.CanDeleteExecutor(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty name, got %d", CodeBadRequest, resp.Code)
	}
}

func TestExecutorDomainHandler_CanDeleteExecutor_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "name", Value: "test-exec"}}

	h.CanDeleteExecutor(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for no auth, got %d", CodeUnauthorized, resp.Code)
	}
}

func TestExecutorDomainHandler_CanDeleteExecutor_InvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ExecutorDomainHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "name", Value: "test-exec"}}
	c.Set("user_id", "invalid")

	h.CanDeleteExecutor(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for invalid user_id, got %d", CodeUnauthorized, resp.Code)
	}
}
