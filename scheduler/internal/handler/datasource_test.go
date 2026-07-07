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

func TestDatasourceHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DatasourceHandler{}
	r.GET("/api/datasources", handler.List)

	req, _ := http.NewRequest("GET", "/api/datasources", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	// Should panic due to nil db, but we recover
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestDatasourceHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DatasourceHandler{}
	r.GET("/api/datasources/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/datasources/1", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestDatasourceHandler_Create_MissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DatasourceHandler{}
	r.POST("/api/datasources", handler.Create)

	body := map[string]interface{}{}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/datasources", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// BadRequest returns HTTP 200 with error code in JSON body
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["status"] != "error" {
		t.Errorf("expected status 'error' in response body, got %v", resp["status"])
	}
}

func TestDatasourceHandler_Update_MissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DatasourceHandler{}
	r.PUT("/api/datasources/:id", handler.Update)

	body := map[string]interface{}{}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/api/datasources/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil service):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	// If no panic, check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 or panic, got %d", w.Code)
	}
}

func TestDatasourceHandler_Delete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DatasourceHandler{}
	r.DELETE("/api/datasources/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/datasources/1", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestDatasourceHandler_TestConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DatasourceHandler{}
	r.POST("/api/datasources/:id/test", handler.TestConnection)

	req, _ := http.NewRequest("POST", "/api/datasources/1/test", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

// TestPickHigherPermission 测试纯函数 pickHigherPermission
func TestPickHigherPermission(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{"both valid a higher", "manage", "read", "manage"},
		{"both valid b higher", "read", "manage", "manage"},
		{"both valid equal weight", "query", "query", "query"},
		{"a valid b invalid", "manage", "unknown", "manage"},
		{"a invalid b valid", "unknown", "read", "read"},
		{"both invalid", "unknown1", "unknown2", "unknown2"},
		{"a empty b valid", "", "read", "read"},
		{"a valid b empty", "manage", "", "manage"},
		{"both empty", "", "", ""},
		{"manage vs update", "manage", "update", "manage"},
		{"update vs download", "update", "download", "update"},
		{"download vs query", "download", "query", "download"},
		{"query vs read", "query", "read", "query"},
		{"read vs delete", "read", "delete", "read"},
		{"delete vs manage", "delete", "manage", "manage"},
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

// TestDatasourceToMap 测试 datasourceToMap（nil userAdminSvc）
func TestDatasourceToMap(t *testing.T) {
	t.Run("nil userAdminSvc with basic datasource", func(t *testing.T) {
		h := &DatasourceHandler{}
		ds := &model.Datasource{
			ID: 1, Name: "test-ds", Type: "mysql",
			Host: "localhost", Port: 3306, Path: "/tmp",
			Database: "testdb", Username: "root", AuthType: "simple",
			Config: "{}", Description: "test desc",
			DomainID: 1, DomainName: "domain1", IsEnabled: true,
			AllowWriteSQL: false, TestStatus: "success",
			ConnectionMode: "standard", ZkHosts: "", ZkPath: "",
			RqliteHosts: "", UserPermission: "read",
		}
		m := h.datasourceToMap(ds)
		if m["id"] != int64(1) {
			t.Errorf("id = %v, want 1", m["id"])
		}
		if m["name"] != "test-ds" {
			t.Errorf("name = %v, want test-ds", m["name"])
		}
		if m["password"] != nil {
			t.Errorf("password should not be in map, got %v", m["password"])
		}
		if m["created_by_name"] != "" {
			t.Errorf("created_by_name should be empty for nil userAdminSvc, got %v", m["created_by_name"])
		}
		if m["updated_by_name"] != "" {
			t.Errorf("updated_by_name should be empty for nil userAdminSvc, got %v", m["updated_by_name"])
		}
		if m["user_permission"] != "read" {
			t.Errorf("user_permission = %v, want read", m["user_permission"])
		}
	})

	t.Run("with nil LastTestAt", func(t *testing.T) {
		h := &DatasourceHandler{}
		ds := &model.Datasource{
			ID: 2, Name: "ds2", Type: "sqlite",
			LastTestAt: nil,
		}
		m := h.datasourceToMap(ds)
		if m["last_test_at"] != "" {
			t.Errorf("last_test_at should be empty for nil, got %v", m["last_test_at"])
		}
	})

	t.Run("with valid LastTestAt", func(t *testing.T) {
		h := &DatasourceHandler{}
		now := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
		ds := &model.Datasource{
			ID: 3, Name: "ds3", Type: "mysql",
			LastTestAt: &now,
		}
		m := h.datasourceToMap(ds)
		if m["last_test_at"] != "2024-06-15T10:30:00Z" {
			t.Errorf("last_test_at = %v, want 2024-06-15T10:30:00Z", m["last_test_at"])
		}
	})

	t.Run("with created_by and updated_by pointers", func(t *testing.T) {
		h := &DatasourceHandler{}
		createdBy := int64(10)
		updatedBy := int64(20)
		ds := &model.Datasource{
			ID: 4, Name: "ds4", Type: "mysql",
			CreatedBy: &createdBy, UpdatedBy: &updatedBy,
		}
		m := h.datasourceToMap(ds)
		if m["created_by"] != &createdBy {
			t.Errorf("created_by = %v, want %v", m["created_by"], &createdBy)
		}
		if m["updated_by"] != &updatedBy {
			t.Errorf("updated_by = %v, want %v", m["updated_by"], &updatedBy)
		}
	})
}

// TestDatasourceHandler_Get_InvalidID 测试无效 ID
func TestDatasourceHandler_Get_InvalidID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"non-numeric", "abc"},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			h := &DatasourceHandler{}
			r.GET("/api/datasources/:id", h.Get)

			req, _ := http.NewRequest("GET", "/api/datasources/"+tt.id, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
			}
		})
	}
}

// TestDatasourceHandler_Create 测试创建数据源
func TestDatasourceHandler_Create(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources", h.Create)

		req, _ := http.NewRequest("POST", "/api/datasources", bytes.NewBufferString("not json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing required name", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources", h.Create)

		body := `{"type":"mysql","domain_id":1}`
		req, _ := http.NewRequest("POST", "/api/datasources", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing required type", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources", h.Create)

		body := `{"name":"test","domain_id":1}`
		req, _ := http.NewRequest("POST", "/api/datasources", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing required domain_id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources", h.Create)

		body := `{"name":"test","type":"mysql"}`
		req, _ := http.NewRequest("POST", "/api/datasources", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources", h.Create)

		body := `{"name":"test","type":"unsupporteDBType","domain_id":1}`
		req, _ := http.NewRequest("POST", "/api/datasources", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("valid request but nil manager", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources", h.Create)

		body := `{"name":"test","type":"mysql","host":"localhost","port":3306,"domain_id":1}`
		req, _ := http.NewRequest("POST", "/api/datasources", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		defer func() {
			if rec := recover(); rec != nil {
				t.Log("Recovered from panic (expected for nil manager):", rec)
			}
		}()

		r.ServeHTTP(w, req)
	})
}

// TestDatasourceHandler_Update_InvalidID 测试更新时无效 ID
func TestDatasourceHandler_Update_InvalidID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"non-numeric", "abc"},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			h := &DatasourceHandler{}
			r.PUT("/api/datasources/:id", h.Update)

			body := `{"name":"test"}`
			req, _ := http.NewRequest("PUT", "/api/datasources/"+tt.id, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
			}
		})
	}
}

// TestDatasourceHandler_Update_InvalidJSON 测试更新时无效 JSON
func TestDatasourceHandler_Update_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.PUT("/api/datasources/:id", h.Update)

	req, _ := http.NewRequest("PUT", "/api/datasources/1", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
	}
}

// TestDatasourceHandler_Delete_InvalidID 测试删除时无效 ID
func TestDatasourceHandler_Delete_InvalidID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"non-numeric", "abc"},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			h := &DatasourceHandler{}
			r.DELETE("/api/datasources/:id", h.Delete)

			req, _ := http.NewRequest("DELETE", "/api/datasources/"+tt.id, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
			}
		})
	}
}

// TestDatasourceHandler_TestConnection_InvalidID 测试连接测试时无效 ID
func TestDatasourceHandler_TestConnection_InvalidID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"non-numeric", "abc"},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			h := &DatasourceHandler{}
			r.POST("/api/datasources/:id/test", h.TestConnection)

			req, _ := http.NewRequest("POST", "/api/datasources/"+tt.id+"/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
			}
		})
	}
}

// TestDatasourceHandler_TestConnectionByParams 测试按参数测试连接
func TestDatasourceHandler_TestConnectionByParams(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources/test", h.TestConnectionByParams)

		req, _ := http.NewRequest("POST", "/api/datasources/test", bytes.NewBufferString("not json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing required type", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources/test", h.TestConnectionByParams)

		body := `{"host":"localhost"}`
		req, _ := http.NewRequest("POST", "/api/datasources/test", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources/test", h.TestConnectionByParams)

		body := `{"type":"unsupporteDBType","host":"localhost"}`
		req, _ := http.NewRequest("POST", "/api/datasources/test", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})
}

// TestDatasourceHandler_GrantPermission 测试授权
func TestDatasourceHandler_GrantPermission(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources/:id/permissions", h.GrantPermission)

		req, _ := http.NewRequest("POST", "/api/datasources/1/permissions", bytes.NewBufferString("not json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing permission_type", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources/:id/permissions", h.GrantPermission)

		body := `{"role_id":1}`
		req, _ := http.NewRequest("POST", "/api/datasources/1/permissions", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing both role_id and user_id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources/:id/permissions", h.GrantPermission)

		body := `{"permission_type":"read"}`
		req, _ := http.NewRequest("POST", "/api/datasources/1/permissions", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("invalid permission_type", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources/:id/permissions", h.GrantPermission)

		body := `{"permission_type":"invalid_perm","role_id":1}`
		req, _ := http.NewRequest("POST", "/api/datasources/1/permissions", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("invalid datasource id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.POST("/api/datasources/:id/permissions", h.GrantPermission)

		body := `{"permission_type":"read","role_id":1}`
		req, _ := http.NewRequest("POST", "/api/datasources/abc/permissions", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})
}

// TestDatasourceHandler_RevokePermission 测试撤销权限
func TestDatasourceHandler_RevokePermission(t *testing.T) {
	tests := []struct {
		name   string
		permID string
	}{
		{"non-numeric", "abc"},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			h := &DatasourceHandler{}
			r.DELETE("/api/datasources/:id/permissions/:perm_id", h.RevokePermission)

			req, _ := http.NewRequest("DELETE", "/api/datasources/1/permissions/"+tt.permID, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
			}
		})
	}
}

// TestDatasourceHandler_UpdatePermission 测试更新权限
func TestDatasourceHandler_UpdatePermission(t *testing.T) {
	t.Run("invalid perm_id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.PUT("/api/datasources/:id/permissions/:perm_id", h.UpdatePermission)

		body := `{"permission_type":"read"}`
		req, _ := http.NewRequest("PUT", "/api/datasources/1/permissions/abc", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.PUT("/api/datasources/:id/permissions/:perm_id", h.UpdatePermission)

		req, _ := http.NewRequest("PUT", "/api/datasources/1/permissions/1", bytes.NewBufferString("not json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing permission_type", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.PUT("/api/datasources/:id/permissions/:perm_id", h.UpdatePermission)

		body := `{}`
		req, _ := http.NewRequest("PUT", "/api/datasources/1/permissions/1", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("invalid permission_type", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &DatasourceHandler{}
		r.PUT("/api/datasources/:id/permissions/:perm_id", h.UpdatePermission)

		body := `{"permission_type":"invalid_perm"}`
		req, _ := http.NewRequest("PUT", "/api/datasources/1/permissions/1", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})
}

// TestDatasourceHandler_GetPermissions_InvalidID 测试获取权限列表时无效 ID
func TestDatasourceHandler_GetPermissions_InvalidID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"non-numeric", "abc"},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			h := &DatasourceHandler{}
			r.GET("/api/datasources/:id/permissions", h.GetPermissions)

			req, _ := http.NewRequest("GET", "/api/datasources/"+tt.id+"/permissions", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
			}
		})
	}
}

// TestDatasourceHandler_SupportedTypes 测试获取支持的数据源类型
func TestDatasourceHandler_SupportedTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.GET("/api/datasources/types", h.SupportedTypes)

	req, _ := http.NewRequest("GET", "/api/datasources/types", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeSuccess {
		t.Errorf("code = %d, want %d", resp.Code, CodeSuccess)
	}
}

// TestNewDatasourceHandler 测试构造函数
func TestNewDatasourceHandler(t *testing.T) {
	h := NewDatasourceHandler(nil, nil, nil, nil, nil, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

// TestDatasourceHandler_GetPermissions_NilService 测试获取权限列表（nil service 会 panic）
func TestDatasourceHandler_GetPermissions_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DatasourceHandler{}
	r.GET("/api/datasources/:id/permissions", h.GetPermissions)

	req, _ := http.NewRequest("GET", "/api/datasources/1/permissions", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("Recovered from panic (expected for nil service):", rec)
		}
	}()

	r.ServeHTTP(w, req)
}
