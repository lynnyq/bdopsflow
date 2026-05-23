package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type mockDatasourceGetter struct {
	getDomainIDFn         func(dsID int64) (int64, error)
	checkPermissionFn func(userID int64, dsID int64, action string) (bool, error)
}

func (m *mockDatasourceGetter) GetDatasourceDomainID(dsID int64) (int64, error) {
	return m.getDomainIDFn(dsID)
}

func (m *mockDatasourceGetter) CheckDatasourcePermission(userID int64, dsID int64, action string) (bool, error) {
	return m.checkPermissionFn(userID, dsID, action)
}

func setupDatasourceRouter(dsSvc DatasourceGetter, action string, role string, userID int64, domainID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", role)
		c.Set("user_id", userID)
		c.Set("domain_id", domainID)
		c.Next()
	})
	router.Use(DatasourcePermissionMiddleware(dsSvc, action))
	return router
}

func TestDatasourcePermission_SystemAdmin(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			t.Error("GetDatasourceDomainID should not be called for system_admin")
			return 0, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			t.Error("CheckDatasourcePermission should not be called for system_admin")
			return false, nil
		},
	}

	router := setupDatasourceRouter(dsSvc, "read", "system_admin", 1, 1)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDatasourcePermission_Admin(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			t.Error("GetDatasourceDomainID should not be called for admin")
			return 0, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			t.Error("CheckDatasourcePermission should not be called for admin")
			return false, nil
		},
	}

	router := setupDatasourceRouter(dsSvc, "read", "admin", 1, 1)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDatasourcePermission_DomainAdmin_SameDomain(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			if dsID != 100 {
				t.Errorf("expected dsID 100, got %d", dsID)
			}
			return 5, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			t.Error("CheckDatasourcePermission should not be called for domain_admin with same domain")
			return false, nil
		},
	}

	router := setupDatasourceRouter(dsSvc, "read", "domain_admin", 1, 5)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDatasourcePermission_DomainAdmin_DifferentDomain(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			return 99, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			return false, nil
		},
	}

	router := setupDatasourceRouter(dsSvc, "read", "domain_admin", 1, 5)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestDatasourcePermission_DomainAdmin_DifferentDomain_WithPermission(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			return 99, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			return true, nil
		},
	}

	router := setupDatasourceRouter(dsSvc, "read", "domain_admin", 1, 5)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDatasourcePermission_User_WithPermission(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			return 5, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			if action != "read" {
				t.Errorf("expected action 'read', got '%s'", action)
			}
			return true, nil
		},
	}

	router := setupDatasourceRouter(dsSvc, "read", "user", 42, 5)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDatasourcePermission_User_WithoutPermission(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			return 5, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			return false, nil
		},
	}

	router := setupDatasourceRouter(dsSvc, "write", "user", 42, 5)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestDatasourcePermission_MissingDatasourceID(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			t.Error("GetDatasourceDomainID should not be called when datasource_id is missing")
			return 0, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			t.Error("CheckDatasourcePermission should not be called when datasource_id is missing")
			return false, nil
		},
	}

	router := setupDatasourceRouter(dsSvc, "read", "user", 42, 5)
	router.GET("/datasources", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestDatasourcePermission_DatasourceNotFound(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			return 0, fmt.Errorf("datasource not found")
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			t.Error("CheckDatasourcePermission should not be called when datasource not found")
			return false, nil
		},
	}

	router := setupDatasourceRouter(dsSvc, "read", "user", 42, 5)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDatasourcePermission_PermissionCheckError(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			return 5, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			return false, fmt.Errorf("internal error")
		},
	}

	router := setupDatasourceRouter(dsSvc, "read", "user", 42, 5)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestDatasourcePermission_SetsDatasourceID(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			return 5, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			return true, nil
		},
	}

	var capturedDsID int64
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Set("user_id", int64(42))
		c.Set("domain_id", int64(5))
		c.Next()
	})
	router.Use(DatasourcePermissionMiddleware(dsSvc, "read"))
	router.GET("/datasources/:id", func(c *gin.Context) {
		val, exists := c.Get("datasource_id")
		if !exists {
			t.Error("datasource_id should be set in context")
		}
		capturedDsID = val.(int64)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if capturedDsID != 100 {
		t.Errorf("expected datasource_id 100, got %d", capturedDsID)
	}
}

func TestDatasourcePermission_DomainAdmin_SetsDatasourceID(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			return 5, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			return false, nil
		},
	}

	var capturedDsID int64
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("role", "domain_admin")
		c.Set("user_id", int64(1))
		c.Set("domain_id", int64(5))
		c.Next()
	})
	router.Use(DatasourcePermissionMiddleware(dsSvc, "read"))
	router.GET("/datasources/:id", func(c *gin.Context) {
		val, exists := c.Get("datasource_id")
		if !exists {
			t.Error("datasource_id should be set in context")
		}
		capturedDsID = val.(int64)
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/200", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if capturedDsID != 200 {
		t.Errorf("expected datasource_id 200, got %d", capturedDsID)
	}
}

func TestGetDatasourceID_URLParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/datasources/42", nil)
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	dsID := getDatasourceID(c)
	if dsID != 42 {
		t.Errorf("expected datasource_id 42, got %d", dsID)
	}
}

func TestGetDatasourceID_URLParamInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/datasources/abc", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	dsID := getDatasourceID(c)
	if dsID != 0 {
		t.Errorf("expected datasource_id 0 for invalid param, got %d", dsID)
	}
}

func TestGetDatasourceID_QueryParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/datasources?datasource_id=55", nil)

	dsID := getDatasourceID(c)
	if dsID != 55 {
		t.Errorf("expected datasource_id 55, got %d", dsID)
	}
}

func TestGetDatasourceID_QueryParamInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/datasources?datasource_id=abc", nil)

	dsID := getDatasourceID(c)
	if dsID != 0 {
		t.Errorf("expected datasource_id 0 for invalid query param, got %d", dsID)
	}
}

func TestGetDatasourceID_URLParamTakesPrecedence(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/datasources/10?datasource_id=20", nil)
	c.Params = gin.Params{{Key: "id", Value: "10"}}

	dsID := getDatasourceID(c)
	if dsID != 10 {
		t.Errorf("expected datasource_id 10 (URL param takes precedence), got %d", dsID)
	}
}

func TestGetDatasourceID_PostBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := map[string]interface{}{"datasource_id": 77}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/datasources", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	dsID := getDatasourceID(c)
	if dsID != 77 {
		t.Errorf("expected datasource_id 77, got %d", dsID)
	}
}

func TestGetDatasourceID_PostBodyZeroValue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := map[string]interface{}{"datasource_id": 0}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/datasources", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	dsID := getDatasourceID(c)
	if dsID != 0 {
		t.Errorf("expected datasource_id 0 for zero value in body, got %d", dsID)
	}
}

func TestGetDatasourceID_PostBodyNotJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/datasources", bytes.NewReader([]byte("datasource_id=77")))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	dsID := getDatasourceID(c)
	if dsID != 0 {
		t.Errorf("expected datasource_id 0 for non-JSON content type, got %d", dsID)
	}
}

func TestGetDatasourceID_GetRequestNoBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/datasources", nil)

	dsID := getDatasourceID(c)
	if dsID != 0 {
		t.Errorf("expected datasource_id 0 for GET with no params, got %d", dsID)
	}
}

func TestGetDatasourceID_PutBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := map[string]interface{}{"datasource_id": 88, "name": "updated"}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/datasources", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	dsID := getDatasourceID(c)
	if dsID != 88 {
		t.Errorf("expected datasource_id 88, got %d", dsID)
	}
}

func TestGetDatasourceID_PostBodyReReadable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := map[string]interface{}{"datasource_id": 99, "name": "test"}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/datasources", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	dsID := getDatasourceID(c)
	if dsID != 99 {
		t.Errorf("expected datasource_id 99, got %d", dsID)
	}

	var reReadBody struct {
		DatasourceID int64  `json:"datasource_id"`
		Name         string `json:"name"`
	}
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(&reReadBody); err != nil {
		t.Fatalf("failed to re-read body: %v", err)
	}
	if reReadBody.DatasourceID != 99 {
		t.Errorf("expected re-read datasource_id 99, got %d", reReadBody.DatasourceID)
	}
	if reReadBody.Name != "test" {
		t.Errorf("expected re-read name 'test', got '%s'", reReadBody.Name)
	}
}

func TestDatasourcePermission_ResponseFormat_MissingID(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn:     func(dsID int64) (int64, error) { return 0, nil },
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) { return false, nil },
	}

	router := setupDatasourceRouter(dsSvc, "read", "user", 42, 5)
	router.GET("/datasources", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	var resp datasourceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != http.StatusForbidden {
		t.Errorf("expected response code 403, got %d", resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected response status 'error', got '%s'", resp.Status)
	}
	if resp.Message != "缺少数据源标识，无法进行权限校验" {
		t.Errorf("expected message '缺少数据源标识，无法进行权限校验', got '%s'", resp.Message)
	}
}

func TestDatasourcePermission_ResponseFormat_NotFound(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn:     func(dsID int64) (int64, error) { return 0, fmt.Errorf("not found") },
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) { return false, nil },
	}

	router := setupDatasourceRouter(dsSvc, "read", "user", 42, 5)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var resp datasourceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != http.StatusNotFound {
		t.Errorf("expected response code 404, got %d", resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected response status 'error', got '%s'", resp.Status)
	}
	if resp.Message != "数据源不存在" {
		t.Errorf("expected message '数据源不存在', got '%s'", resp.Message)
	}
}

func TestDatasourcePermission_ResponseFormat_InsufficientPermission(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn:     func(dsID int64) (int64, error) { return 5, nil },
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) { return false, nil },
	}

	router := setupDatasourceRouter(dsSvc, "write", "user", 42, 5)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	var resp datasourceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != http.StatusForbidden {
		t.Errorf("expected response code 403, got %d", resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected response status 'error', got '%s'", resp.Status)
	}
	if resp.Message != "您没有该数据源的写入权限，请联系管理员开通" {
		t.Errorf("expected message '您没有该数据源的写入权限，请联系管理员开通', got '%s'", resp.Message)
	}
}

func TestDatasourcePermission_ResponseFormat_CheckError(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn:     func(dsID int64) (int64, error) { return 5, nil },
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) { return false, fmt.Errorf("db error") },
	}

	router := setupDatasourceRouter(dsSvc, "read", "user", 42, 5)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var resp datasourceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != http.StatusInternalServerError {
		t.Errorf("expected response code 500, got %d", resp.Code)
	}
	if resp.Status != "error" {
		t.Errorf("expected response status 'error', got '%s'", resp.Status)
	}
	if resp.Message != "权限校验失败，请稍后重试" {
		t.Errorf("expected message '权限校验失败，请稍后重试', got '%s'", resp.Message)
	}
}

func TestDatasourcePermission_QueryParamDatasourceID(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			if dsID != 55 {
				t.Errorf("expected dsID 55, got %d", dsID)
			}
			return 5, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			return true, nil
		},
	}

	router := setupDatasourceRouter(dsSvc, "read", "user", 42, 5)
	router.GET("/datasources", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources?datasource_id=55", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDatasourcePermission_PostBodyDatasourceID(t *testing.T) {
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn: func(dsID int64) (int64, error) {
			if dsID != 77 {
				t.Errorf("expected dsID 77, got %d", dsID)
			}
			return 5, nil
		},
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) {
			return true, nil
		},
	}

	router := setupDatasourceRouter(dsSvc, "write", "user", 42, 5)
	router.POST("/datasources", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	body := map[string]interface{}{"datasource_id": 77, "name": "test"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/datasources", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDatasourcePermission_AbortPreventsNextHandler(t *testing.T) {
	nextCalled := false
	dsSvc := &mockDatasourceGetter{
		getDomainIDFn:     func(dsID int64) (int64, error) { return 0, nil },
		checkPermissionFn: func(userID int64, dsID int64, action string) (bool, error) { return false, nil },
	}

	router := setupDatasourceRouter(dsSvc, "read", "user", 42, 5)
	router.GET("/datasources", func(c *gin.Context) {
		nextCalled = true
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if nextCalled {
		t.Error("next handler should not be called after abort")
	}
}
