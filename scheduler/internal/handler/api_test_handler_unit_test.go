package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTestApiTestHandler 创建用于测试的 ApiTestHandler（services 为 nil，仅测试参数校验路径）
func newTestApiTestHandler() *ApiTestHandler {
	return NewApiTestHandler(nil, nil, nil, nil, nil, nil)
}

// setupApiTestContext 创建带请求的测试 gin.Context
func setupApiTestContext(method, path string, body interface{}) (*gin.Context, *httptest.ResponseRecorder) {
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

// decodeResponse 解码响应到 map
func decodeResponse(data []byte) map[string]interface{} {
	var resp map[string]interface{}
	_ = json.Unmarshal(data, &resp)
	return resp
}

// TestApiTestHandler_Create_MissingRequiredFields 测试 Create 缺少必填字段
func TestApiTestHandler_Create_MissingRequiredFields(t *testing.T) {
	h := newTestApiTestHandler()

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
			body: map[string]string{"type": "http"},
		},
		{
			name: "missing type",
			body: map[string]string{"name": "test"},
		},
		{
			name: "empty name",
			body: map[string]string{"name": "", "type": "http"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests", tt.body)
			h.Create(c)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			resp := decodeResponse(w.Body.Bytes())
			if resp["code"] != float64(CodeBadRequest) {
				t.Errorf("expected code %d, got %v", CodeBadRequest, resp["code"])
			}
		})
	}
}

// TestApiTestHandler_Create_InvalidType 测试 Create 无效的测试类型
func TestApiTestHandler_Create_InvalidType(t *testing.T) {
	h := newTestApiTestHandler()

	tests := []struct {
		name string
		body map[string]string
	}{
		{
			name: "type is websocket",
			body: map[string]string{"name": "test", "type": "websocket"},
		},
		{
			name: "type is tcp",
			body: map[string]string{"name": "test", "type": "tcp"},
		},
		{
			name: "type is empty",
			body: map[string]string{"name": "test", "type": ""},
		},
		{
			name: "type is HTTP uppercase",
			body: map[string]string{"name": "test", "type": "HTTP"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests", tt.body)
			c.Set("user_id", int64(1))
			h.Create(c)

			resp := decodeResponse(w.Body.Bytes())
			if resp["code"] != float64(CodeBadRequest) {
				t.Errorf("expected code %d for type %q, got %v", CodeBadRequest, tt.body["type"], resp["code"])
			}
		})
	}
}

// TestApiTestHandler_Create_NoUser 测试 Create 时用户未登录
func TestApiTestHandler_Create_NoUser(t *testing.T) {
	h := newTestApiTestHandler()

	body := map[string]string{"name": "test", "type": "http"}
	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests", body)
	// 不设置 user_id
	h.Create(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeUnauthorized) {
		t.Errorf("expected code %d for missing user, got %v", CodeUnauthorized, resp["code"])
	}
}

// TestApiTestHandler_Get_InvalidID 测试 Get 无效的测试ID
func TestApiTestHandler_Get_InvalidID(t *testing.T) {
	h := newTestApiTestHandler()

	tests := []struct {
		name    string
		idParam string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"special chars", "##"},
		{"float value", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupApiTestContext(http.MethodGet, "/api/v1/api-tests/test", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.idParam}}
			h.Get(c)

			resp := decodeResponse(w.Body.Bytes())
			if resp["code"] != float64(CodeBadRequest) {
				t.Errorf("expected code %d for id %q, got %v", CodeBadRequest, tt.idParam, resp["code"])
			}
		})
	}
}

// TestApiTestHandler_Update_InvalidID 测试 Update 无效的测试ID
func TestApiTestHandler_Update_InvalidID(t *testing.T) {
	h := newTestApiTestHandler()

	c, w := setupApiTestContext(http.MethodPut, "/api/v1/api-tests/abc", map[string]interface{}{"name": "test"})
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	h.Update(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("expected code %d, got %v", CodeBadRequest, resp["code"])
	}
}

// TestApiTestHandler_Delete_InvalidID 测试 Delete 无效的测试ID
func TestApiTestHandler_Delete_InvalidID(t *testing.T) {
	h := newTestApiTestHandler()

	c, w := setupApiTestContext(http.MethodDelete, "/api/v1/api-tests/invalid", nil)
	c.Params = gin.Params{{Key: "id", Value: "invalid"}}
	h.Delete(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("expected code %d, got %v", CodeBadRequest, resp["code"])
	}
}

// TestApiTestHandler_Execute_NoUser 测试 Execute 用户未登录
func TestApiTestHandler_Execute_NoUser(t *testing.T) {
	h := newTestApiTestHandler()

	body := map[string]interface{}{
		"type":   "http",
		"config": "{}",
	}
	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/execute", body)
	// 不设置 user_id
	h.Execute(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeUnauthorized) {
		t.Errorf("expected code %d, got %v", CodeUnauthorized, resp["code"])
	}
}

// TestApiTestHandler_Execute_MissingRequiredFields 测试 Execute 缺少必填字段
func TestApiTestHandler_Execute_MissingRequiredFields(t *testing.T) {
	h := newTestApiTestHandler()

	tests := []struct {
		name string
		body interface{}
	}{
		{
			name: "empty body",
			body: nil,
		},
		{
			name: "missing type",
			body: map[string]string{"config": "{}"},
		},
		{
			name: "missing config",
			body: map[string]string{"type": "http"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/execute", tt.body)
			c.Set("user_id", int64(1))
			h.Execute(c)

			resp := decodeResponse(w.Body.Bytes())
			if resp["code"] != float64(CodeBadRequest) {
				t.Errorf("expected code %d for %q, got %v", CodeBadRequest, tt.name, resp["code"])
			}
		})
	}
}

// TestApiTestHandler_Execute_UnsupportedType 测试 Execute 不支持的类型
func TestApiTestHandler_Execute_UnsupportedType(t *testing.T) {
	h := newTestApiTestHandler()

	body := map[string]interface{}{
		"type":   "websocket",
		"config": "{}",
	}
	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/execute", body)
	c.Set("user_id", int64(1))
	h.Execute(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("expected code %d for unsupported type, got %v", CodeBadRequest, resp["code"])
	}
}

// TestApiTestHandler_ExecuteSaved_InvalidID 测试 ExecuteSaved 无效ID
func TestApiTestHandler_ExecuteSaved_InvalidID(t *testing.T) {
	h := newTestApiTestHandler()

	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/notanumber/execute", nil)
	c.Params = gin.Params{{Key: "id", Value: "notanumber"}}
	h.ExecuteSaved(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("expected code %d, got %v", CodeBadRequest, resp["code"])
	}
}

// TestApiTestHandler_List_NoUser 测试 List 用户未登录
func TestApiTestHandler_List_NoUser(t *testing.T) {
	h := newTestApiTestHandler()

	c, w := setupApiTestContext(http.MethodGet, "/api/v1/api-tests", nil)
	h.List(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeUnauthorized) {
		t.Errorf("expected code %d for missing user, got %v", CodeUnauthorized, resp["code"])
	}
}

// TestApiTestHandler_ListResults_NoUser 测试 ListResults 用户未登录
func TestApiTestHandler_ListResults_NoUser(t *testing.T) {
	h := newTestApiTestHandler()

	c, w := setupApiTestContext(http.MethodGet, "/api/v1/api-test-results", nil)
	h.ListResults(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeUnauthorized) {
		t.Errorf("expected code %d for missing user, got %v", CodeUnauthorized, resp["code"])
	}
}

// TestApiTestHandler_DeleteResult_InvalidID 测试 DeleteResult 无效ID
func TestApiTestHandler_DeleteResult_InvalidID(t *testing.T) {
	h := newTestApiTestHandler()

	c, w := setupApiTestContext(http.MethodDelete, "/api/v1/api-test-results/invalid", nil)
	c.Params = gin.Params{{Key: "id", Value: "invalid"}}
	h.DeleteResult(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("expected code %d, got %v", CodeBadRequest, resp["code"])
	}
}

// TestApiTestHandler_GetResults_InvalidID 测试 GetResults 无效ID
func TestApiTestHandler_GetResults_InvalidID(t *testing.T) {
	h := newTestApiTestHandler()

	c, w := setupApiTestContext(http.MethodGet, "/api/v1/api-tests/bad/results", nil)
	c.Params = gin.Params{{Key: "id", Value: "bad"}}
	h.GetResults(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("expected code %d, got %v", CodeBadRequest, resp["code"])
	}
}

// TestApiTestHandler_LoadProtoContent_NilID 测试 loadProtoContent 当 protoFileID 为 nil
func TestApiTestHandler_LoadProtoContent_NilID(t *testing.T) {
	h := newTestApiTestHandler()

	c, _ := setupApiTestContext(http.MethodGet, "/", nil)

	content, deps := h.loadProtoContent(c, nil)
	if content != "" {
		t.Errorf("expected empty content for nil protoFileID, got %q", content)
	}
	if deps != nil {
		t.Errorf("expected nil deps for nil protoFileID, got %v", deps)
	}
}

// TestApiTestHandler_LoadProtoContent_ZeroID 测试 loadProtoContent 当 protoFileID 为 0
func TestApiTestHandler_LoadProtoContent_ZeroID(t *testing.T) {
	h := newTestApiTestHandler()

	c, _ := setupApiTestContext(http.MethodGet, "/", nil)

	zeroID := int64(0)
	content, deps := h.loadProtoContent(c, &zeroID)
	if content != "" {
		t.Errorf("expected empty content for zero protoFileID, got %q", content)
	}
	if deps != nil {
		t.Errorf("expected nil deps for zero protoFileID, got %v", deps)
	}
}
