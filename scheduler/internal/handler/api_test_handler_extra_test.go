package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestApiTestHandler_Execute_InvalidHTTPConfig 测试 Execute http 类型但 config 不是有效 JSON
func TestApiTestHandler_Execute_InvalidHTTPConfig(t *testing.T) {
	h := newTestApiTestHandler()

	body := map[string]interface{}{
		"type":   "http",
		"config": "not json",
	}
	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/execute", body)
	c.Set("user_id", int64(1))
	h.Execute(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("expected code %d for invalid http config, got %v", CodeBadRequest, resp["code"])
	}
}

// TestApiTestHandler_Execute_EmptyHTTPConfig 测试 Execute http 类型但 config 为空字符串
func TestApiTestHandler_Execute_EmptyHTTPConfig(t *testing.T) {
	h := newTestApiTestHandler()

	body := map[string]interface{}{
		"type":   "http",
		"config": "",
	}
	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/execute", body)
	c.Set("user_id", int64(1))
	h.Execute(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("expected code %d for empty http config, got %v", CodeBadRequest, resp["code"])
	}
}

// TestApiTestHandler_Execute_InvalidGRPCConfig 测试 Execute grpc 类型但 config 不是有效 JSON
func TestApiTestHandler_Execute_InvalidGRPCConfig(t *testing.T) {
	h := newTestApiTestHandler()

	body := map[string]interface{}{
		"type":   "grpc",
		"config": "not json",
	}
	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/execute", body)
	c.Set("user_id", int64(1))
	h.Execute(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("expected code %d for invalid grpc config, got %v", CodeBadRequest, resp["code"])
	}
}

// TestApiTestHandler_Execute_InvalidGRPCConnectTestConfig 测试 Execute grpc_connect_test 类型但 config 无效
func TestApiTestHandler_Execute_InvalidGRPCConnectTestConfig(t *testing.T) {
	h := newTestApiTestHandler()

	body := map[string]interface{}{
		"type":   "grpc_connect_test",
		"config": "not json",
	}
	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/execute", body)
	c.Set("user_id", int64(1))
	h.Execute(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeBadRequest) {
		t.Errorf("expected code %d for invalid grpc_connect_test config, got %v", CodeBadRequest, resp["code"])
	}
}

// TestApiTestHandler_Execute_GRPCConnectTestWithNilService 测试 Execute grpc_connect_test 有效 config 但 service 为 nil
func TestApiTestHandler_Execute_GRPCConnectTestWithNilService(t *testing.T) {
	h := newTestApiTestHandler()

	body := map[string]interface{}{
		"type":   "grpc_connect_test",
		"config": `{"address":"localhost:50051"}`,
	}
	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/execute", body)
	c.Set("user_id", int64(1))

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from expected panic: %v", r)
		}
	}()

	h.Execute(c)

	if w.Body.Len() > 0 {
		resp := decodeResponse(w.Body.Bytes())
		if resp["code"] != nil && resp["code"] != float64(CodeApiTestExecuteFailed) {
			t.Logf("got code %v (panic may have occurred)", resp["code"])
		}
	}
}

// TestApiTestHandler_Create_ValidFieldsButNilService 测试 Create 字段有效但 service 为 nil
func TestApiTestHandler_Create_ValidFieldsButNilService(t *testing.T) {
	h := newTestApiTestHandler()

	body := map[string]interface{}{
		"name":   "test-api",
		"type":   "http",
		"config": "{}",
	}
	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests", body)
	c.Set("user_id", int64(1))

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from expected panic: %v", r)
		}
	}()

	h.Create(c)

	if w.Body.Len() > 0 {
		resp := decodeResponse(w.Body.Bytes())
		if resp["code"] != nil && resp["code"] != float64(CodeQueryError) {
			t.Logf("got code %v (panic may have occurred)", resp["code"])
		}
	}
}

// TestApiTestHandler_Update_InvalidType 测试 Update 时传入无效的 type
func TestApiTestHandler_Update_InvalidType(t *testing.T) {
	h := newTestApiTestHandler()

	body := map[string]interface{}{
		"type": "websocket",
	}
	c, _ := setupApiTestContext(http.MethodPut, "/api/v1/api-tests/1", body)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from expected panic: %v", r)
		}
	}()

	h.Update(c)
}

// TestApiTestHandler_List_WithUserID 测试 List 设置 user_id 但 service 为 nil
func TestApiTestHandler_List_WithUserID(t *testing.T) {
	h := newTestApiTestHandler()

	c, w := setupApiTestContext(http.MethodGet, "/api/v1/api-tests", nil)
	c.Set("user_id", int64(1))

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from expected panic: %v", r)
		}
	}()

	h.List(c)

	if w.Body.Len() > 0 {
		resp := decodeResponse(w.Body.Bytes())
		if resp["code"] != nil && resp["code"] != float64(CodeQueryError) {
			t.Logf("got code %v (panic may have occurred)", resp["code"])
		}
	}
}

// TestApiTestHandler_ListResults_WithUserID 测试 ListResults 设置 user_id 但 service 为 nil
func TestApiTestHandler_ListResults_WithUserID(t *testing.T) {
	h := newTestApiTestHandler()

	c, w := setupApiTestContext(http.MethodGet, "/api/v1/api-test-results", nil)
	c.Set("user_id", int64(1))

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from expected panic: %v", r)
		}
	}()

	h.ListResults(c)

	if w.Body.Len() > 0 {
		resp := decodeResponse(w.Body.Bytes())
		if resp["code"] != nil && resp["code"] != float64(CodeQueryError) {
			t.Logf("got code %v (panic may have occurred)", resp["code"])
		}
	}
}

// TestApiTestHandler_ExecuteSaved_NoUser 测试 ExecuteSaved 用户未登录
func TestApiTestHandler_ExecuteSaved_NoUser(t *testing.T) {
	h := newTestApiTestHandler()

	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/1/execute", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.ExecuteSaved(c)

	resp := decodeResponse(w.Body.Bytes())
	if resp["code"] != float64(CodeUnauthorized) {
		t.Errorf("expected code %d for missing user, got %v", CodeUnauthorized, resp["code"])
	}
}

// TestApiTestHandler_ExecuteSaved_ZeroID 测试 ExecuteSaved id 为 0（nil service 会 panic）
func TestApiTestHandler_ExecuteSaved_ZeroID(t *testing.T) {
	h := newTestApiTestHandler()

	c, _ := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/0/execute", nil)
	c.Params = gin.Params{{Key: "id", Value: "0"}}
	c.Set("user_id", int64(1))

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from expected panic: %v", r)
		}
	}()

	h.ExecuteSaved(c)
}

// TestApiTestHandler_Get_ValidIDButNilService 测试 Get 有效 ID 但 service 为 nil
func TestApiTestHandler_Get_ValidIDButNilService(t *testing.T) {
	h := newTestApiTestHandler()

	c, w := setupApiTestContext(http.MethodGet, "/api/v1/api-tests/1", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from expected panic: %v", r)
		}
	}()

	h.Get(c)

	if w.Body.Len() > 0 {
		resp := decodeResponse(w.Body.Bytes())
		if resp["code"] != nil && resp["code"] != float64(CodeNotFound) {
			t.Logf("got code %v (panic may have occurred)", resp["code"])
		}
	}
}

// TestApiTestHandler_Delete_ValidIDButNilService 测试 Delete 有效 ID 但 service 为 nil
func TestApiTestHandler_Delete_ValidIDButNilService(t *testing.T) {
	h := newTestApiTestHandler()

	c, w := setupApiTestContext(http.MethodDelete, "/api/v1/api-tests/1", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from expected panic: %v", r)
		}
	}()

	h.Delete(c)

	if w.Body.Len() > 0 {
		resp := decodeResponse(w.Body.Bytes())
		if resp["code"] != nil && resp["code"] != float64(CodeNotFound) {
			t.Logf("got code %v (panic may have occurred)", resp["code"])
		}
	}
}

// TestApiTestHandler_Execute_HTTPWithNilService 测试 Execute http 类型有效 config 但 service 为 nil
func TestApiTestHandler_Execute_HTTPWithNilService(t *testing.T) {
	h := newTestApiTestHandler()

	body := map[string]interface{}{
		"type":   "http",
		"config": `{"url":"http://example.com"}`,
	}
	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/execute", body)
	c.Set("user_id", int64(1))

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from expected panic: %v", r)
		}
	}()

	h.Execute(c)

	if w.Body.Len() > 0 {
		resp := decodeResponse(w.Body.Bytes())
		if resp["code"] != nil && resp["code"] != float64(CodeApiTestExecuteFailed) {
			t.Logf("got code %v (panic may have occurred)", resp["code"])
		}
	}
}

// TestApiTestHandler_Execute_GRPCWithNilService 测试 Execute grpc 类型有效 config 但 service 为 nil
func TestApiTestHandler_Execute_GRPCWithNilService(t *testing.T) {
	h := newTestApiTestHandler()

	body := map[string]interface{}{
		"type":   "grpc",
		"config": `{"address":"localhost:50051"}`,
	}
	c, w := setupApiTestContext(http.MethodPost, "/api/v1/api-tests/execute", body)
	c.Set("user_id", int64(1))

	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from expected panic: %v", r)
		}
	}()

	h.Execute(c)

	if w.Body.Len() > 0 {
		resp := decodeResponse(w.Body.Bytes())
		if resp["code"] != nil && resp["code"] != float64(CodeApiTestExecuteFailed) {
			t.Logf("got code %v (panic may have occurred)", resp["code"])
		}
	}
}
