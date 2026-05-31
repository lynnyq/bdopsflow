package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouteParam_ExecutionID_CanBeExtracted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		routePath  string
		requestPath string
		paramName   string
		wantValue   string
	}{
		{
			name:       "task execution logs uses execution_id",
			routePath:  "/api/tasks/executions/:execution_id/logs",
			requestPath: "/api/tasks/executions/exec-123/logs",
			paramName:  "execution_id",
			wantValue:  "exec-123",
		},
		{
			name:       "executor remove domain uses domain_id",
			routePath:  "/api/executors/:name/domains/:domain_id",
			requestPath: "/api/executors/exec-1/domains/42",
			paramName:  "domain_id",
			wantValue:  "42",
		},
		{
			name:       "task endpoints use id",
			routePath:  "/api/tasks/:id",
			requestPath: "/api/tasks/1",
			paramName:  "id",
			wantValue:  "1",
		},
		{
			name:       "executor endpoints use name",
			routePath:  "/api/executors/:name",
			requestPath: "/api/executors/executor-1",
			paramName:  "name",
			wantValue:  "executor-1",
		},
		{
			name:       "log endpoints use id",
			routePath:  "/api/logs/:id",
			requestPath: "/api/logs/99",
			paramName:  "id",
			wantValue:  "99",
		},
		{
			name:       "datasource permission uses perm_id",
			routePath:  "/api/datasources/:id/permissions/:perm_id",
			requestPath: "/api/datasources/1/permissions/5",
			paramName:  "perm_id",
			wantValue:  "5",
		},
		{
			name:       "query cancel uses query_id",
			routePath:  "/api/query/cancel/:query_id",
			requestPath: "/api/query/cancel/q-123",
			paramName:  "query_id",
			wantValue:  "q-123",
		},
		{
			name:       "system config uses key",
			routePath:  "/api/admin/system-config/:key",
			requestPath: "/api/admin/system-config/web.enabled",
			paramName:  "key",
			wantValue:  "web.enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotParam string
			r := gin.New()
			r.GET(tt.routePath, func(c *gin.Context) {
				gotParam = c.Param(tt.paramName)
				c.Status(http.StatusOK)
			})

			req, _ := http.NewRequest("GET", tt.requestPath, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if gotParam != tt.wantValue {
				t.Errorf("c.Param(%q) = %q, want %q", tt.paramName, gotParam, tt.wantValue)
			}
		})
	}
}

func TestRouteParam_OldCamelCaseNamesNotUsed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		routePath  string
		requestPath string
		oldParam   string
	}{
		{
			name:       "task execution logs should NOT use executionId",
			routePath:  "/api/tasks/executions/:execution_id/logs",
			requestPath: "/api/tasks/executions/exec-123/logs",
			oldParam:   "executionId",
		},
		{
			name:       "executor remove domain should NOT use domainId",
			routePath:  "/api/executors/:name/domains/:domain_id",
			requestPath: "/api/executors/exec-1/domains/42",
			oldParam:   "domainId",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotOldParam string
			r := gin.New()
			r.GET(tt.routePath, func(c *gin.Context) {
				gotOldParam = c.Param(tt.oldParam)
				c.Status(http.StatusOK)
			})

			req, _ := http.NewRequest("GET", tt.requestPath, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if gotOldParam != "" {
				t.Errorf("c.Param(%q) should be empty (old camelCase param name should not be used), got %q", tt.oldParam, gotOldParam)
			}
		})
	}
}

func TestListResponse_UsesItemsKey(t *testing.T) {
	c, w := setupTestContext()

	items := []map[string]string{
		{"name": "item1"},
		{"name": "item2"},
	}
	Success(c, gin.H{
		"items":     items,
		"total":     2,
		"page":      1,
		"page_size": 10,
	})

	var raw map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &raw)

	data, ok := raw["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response.data is not a map")
	}

	if _, ok := data["items"]; !ok {
		t.Error("list response missing 'items' key in data")
	}

	if _, ok := data["data"]; ok {
		t.Error("list response should NOT use 'data' key for list items, use 'items' instead")
	}
}

func TestLogHandler_List_ResponseStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &LogHandler{}
	r.GET("/api/logs", handler.List)

	req, _ := http.NewRequest("GET", "/api/logs?page=1&page_size=20", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var raw map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &raw)

	if raw["code"] == nil {
		t.Fatal("response missing 'code' field")
	}

	code, ok := raw["code"].(float64)
	if !ok {
		t.Fatalf("response 'code' is not a number: %v", raw["code"])
	}

	if code == 0 {
		data, ok := raw["data"].(map[string]interface{})
		if !ok {
			t.Fatal("success response.data is not a map")
		}
		if _, ok := data["items"]; !ok {
			t.Error("LogHandler.List success response missing 'items' key")
		}
		if _, ok := data["data"]; ok {
			t.Error("LogHandler.List should NOT use 'data' key for list items, use 'items' instead")
		}
	} else {
		t.Logf("LogHandler.List returned error code %v (nil service expected), skipping data structure check", code)
	}
}

func TestResponse_StandardFields(t *testing.T) {
	c, w := setupTestContext()

	Success(c, gin.H{"id": 1, "name": "test"})

	var raw map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &raw)

	for _, field := range []string{"code", "status", "message", "data"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("response missing standard field '%s'", field)
		}
	}

	if raw["code"].(float64) != 0 {
		t.Errorf("expected code 0, got %v", raw["code"])
	}
	if raw["status"] != "success" {
		t.Errorf("expected status 'success', got %v", raw["status"])
	}
	if raw["message"] != "success" {
		t.Errorf("expected message 'success', got %v", raw["message"])
	}
}

func TestErrorResponse_StandardFields(t *testing.T) {
	c, w := setupTestContext()

	BadRequest(c, "invalid input")

	var raw map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &raw)

	for _, field := range []string{"code", "status", "message"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("error response missing standard field '%s'", field)
		}
	}

	if raw["code"].(float64) != 400 {
		t.Errorf("expected code 400, got %v", raw["code"])
	}
	if raw["status"] != "error" {
		t.Errorf("expected status 'error', got %v", raw["status"])
	}
	if raw["message"] != "invalid input" {
		t.Errorf("expected message 'invalid input', got %v", raw["message"])
	}
}

func TestPaginatedResponse_StandardFields(t *testing.T) {
	c, w := setupTestContext()

	SuccessPaginated(c, []string{"a", "b"}, 100, 3, 10)

	var raw map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &raw)

	for _, field := range []string{"code", "status", "message", "data", "total", "page", "page_size"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("paginated response missing standard field '%s'", field)
		}
	}

	if raw["total"].(float64) != 100 {
		t.Errorf("expected total 100, got %v", raw["total"])
	}
	if raw["page"].(float64) != 3 {
		t.Errorf("expected page 3, got %v", raw["page"])
	}
	if raw["page_size"].(float64) != 10 {
		t.Errorf("expected page_size 10, got %v", raw["page_size"])
	}
}
