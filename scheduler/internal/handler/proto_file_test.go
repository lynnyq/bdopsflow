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
)

// === NewProtoFileHandler 测试 ===

func TestNewProtoFileHandler(t *testing.T) {
	h := NewProtoFileHandler(nil, nil, nil, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.protoSvc != nil {
		t.Errorf("expected nil protoSvc, got %v", h.protoSvc)
	}
}

// === jsonMarshal 测试 ===

func TestJsonMarshal(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    string
		wantErr bool
	}{
		{"nil value", nil, "null", false},
		{"empty string", "", `""`, false},
		{"simple string", "hello", `"hello"`, false},
		{"integer", 42, "42", false},
		{"int64 slice", []int64{1, 2, 3}, "[1,2,3]", false},
		{"string slice", []string{"a", "b"}, `["a","b"]`, false},
		{"map", map[string]int{"a": 1}, `{"a":1}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jsonMarshal(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("jsonMarshal(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("jsonMarshal(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// === protoFileToMap 测试 ===

func TestProtoFileToMap(t *testing.T) {
	now := time.Now()
	pf := &model.ProtoFile{
		ID:           1,
		Name:         "test.proto",
		Content:      "syntax = \"proto3\";",
		FileHash:     "abc123",
		ParsedResult: "{}",
		Dependencies: "[]",
		CreatedBy:    1,
		CreatedByName: "admin",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	m := protoFileToMap(pf)

	if m["id"] != int64(1) {
		t.Errorf("expected id 1, got %v", m["id"])
	}
	if m["name"] != "test.proto" {
		t.Errorf("expected name test.proto, got %v", m["name"])
	}
	if m["content"] != "syntax = \"proto3\";" {
		t.Errorf("expected content, got %v", m["content"])
	}
	if m["file_hash"] != "abc123" {
		t.Errorf("expected file_hash abc123, got %v", m["file_hash"])
	}
	if m["dependencies"] != "[]" {
		t.Errorf("expected dependencies [], got %v", m["dependencies"])
	}
	if m["created_by"] != int64(1) {
		t.Errorf("expected created_by 1, got %v", m["created_by"])
	}
	if m["created_by_name"] != "admin" {
		t.Errorf("expected created_by_name admin, got %v", m["created_by_name"])
	}
	if m["created_at"] == nil || m["created_at"] == "" {
		t.Errorf("expected non-empty created_at, got %v", m["created_at"])
	}
	if m["updated_at"] == nil || m["updated_at"] == "" {
		t.Errorf("expected non-empty updated_at, got %v", m["updated_at"])
	}
}

// === ProtoFileHandler.List 未授权测试 ===

func TestProtoFileHandler_List_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ProtoFileHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h.List(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for no auth, got %d", CodeUnauthorized, resp.Code)
	}
}

// === ProtoFileHandler.Create 测试 ===

func TestProtoFileHandler_Create_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ProtoFileHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

func TestProtoFileHandler_Create_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		body interface{}
	}{
		{
			name: "empty body",
			body: map[string]interface{}{},
		},
		{
			name: "missing content",
			body: map[string]interface{}{
				"name": "test.proto",
			},
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"content": "syntax = \"proto3\";",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &ProtoFileHandler{}

			bodyBytes, _ := json.Marshal(tt.body)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			h.Create(c)

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

// === ProtoFileHandler.Get ID 参数校验测试 ===

func TestProtoFileHandler_Get_InvalidID(t *testing.T) {
	// 注意：proto_file.go 的 Get/Update/Delete 只检查 parse 错误，不检查 id <= 0
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
			gin.SetMode(gin.TestMode)
			h := &ProtoFileHandler{}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.idVal}}

			h.Get(c)

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

// === ProtoFileHandler.Update 测试 ===

func TestProtoFileHandler_Update_InvalidID(t *testing.T) {
	// 注意：proto_file.go 的 Update 只检查 parse 错误，不检查 id <= 0
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
			gin.SetMode(gin.TestMode)
			h := &ProtoFileHandler{}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPut, "/", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.idVal}}

			h.Update(c)

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

func TestProtoFileHandler_Update_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ProtoFileHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.Update(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

// === ProtoFileHandler.Delete ID 参数校验测试 ===

func TestProtoFileHandler_Delete_InvalidID(t *testing.T) {
	// 注意：proto_file.go 的 Delete 只检查 parse 错误，不检查 id <= 0。
	// Delete 中先调用 GetByID，但若 userID<=0 则不会进入 admin 分支，
	// 在 GetByID 失败时（nil svc 会 panic），所以只测试 parse 错误场景。
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
			gin.SetMode(gin.TestMode)
			h := &ProtoFileHandler{}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodDelete, "/", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.idVal}}

			// 设置 user_id 为 int64(1)，使 handler 进入非 admin 分支
			c.Set("user_id", int64(1))

			h.Delete(c)

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

// === ProtoFileHandler.Parse 测试 ===

func TestProtoFileHandler_Parse_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ProtoFileHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Parse(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

func TestProtoFileHandler_Parse_MissingRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ProtoFileHandler{}

	body := map[string]interface{}{} // 缺少 content
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Parse(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for missing content, got %d", CodeBadRequest, resp.Code)
	}
}

// === ProtoFileHandler.Reflect 测试 ===

func TestProtoFileHandler_Reflect_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ProtoFileHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Reflect(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

func TestProtoFileHandler_Reflect_MissingRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ProtoFileHandler{}

	body := map[string]interface{}{} // 缺少 address
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Reflect(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for missing address, got %d", CodeBadRequest, resp.Code)
	}
}

// === ProtoFileHandler.Fields 测试 ===

func TestProtoFileHandler_Fields_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ProtoFileHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Fields(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

func TestProtoFileHandler_Fields_MissingRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ProtoFileHandler{}

	body := map[string]interface{}{} // 缺少 proto_file_id
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Fields(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for missing proto_file_id, got %d", CodeBadRequest, resp.Code)
	}
}

// === ProtoFileHandler.Template 测试 ===

func TestProtoFileHandler_Template_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ProtoFileHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Template(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

func TestProtoFileHandler_Template_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		body interface{}
	}{
		{
			name: "empty body",
			body: map[string]interface{}{},
		},
		{
			name: "missing service and method",
			body: map[string]interface{}{
				"proto_file_id": 1,
			},
		},
		{
			name: "missing method",
			body: map[string]interface{}{
				"proto_file_id": 1,
				"service":       "MyService",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &ProtoFileHandler{}

			bodyBytes, _ := json.Marshal(tt.body)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			h.Template(c)

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
