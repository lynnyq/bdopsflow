package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type mockInstancePermSvc struct {
	hasDatasourcePermissionFn func(ctx context.Context, userID int64, dsID int64, permissionType string) (bool, error)
}

func (m *mockInstancePermSvc) HasDatasourcePermission(ctx context.Context, userID int64, dsID int64, permissionType string) (bool, error) {
	return m.hasDatasourcePermissionFn(ctx, userID, dsID, permissionType)
}

func setupDatasourcePermRouter(permSvc DatasourcePermissionChecker, action string, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	router.Use(DatasourcePermissionMiddleware(permSvc, action))
	return router
}

func TestDatasourcePermission_Allowed(t *testing.T) {
	permSvc := &mockInstancePermSvc{
		hasDatasourcePermissionFn: func(ctx context.Context, userID int64, dsID int64, permissionType string) (bool, error) {
			return true, nil
		},
	}

	router := setupDatasourcePermRouter(permSvc, "read", 1)
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

func TestDatasourcePermission_Denied(t *testing.T) {
	permSvc := &mockInstancePermSvc{
		hasDatasourcePermissionFn: func(ctx context.Context, userID int64, dsID int64, permissionType string) (bool, error) {
			return false, nil
		},
	}

	router := setupDatasourcePermRouter(permSvc, "read", 1)
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

func TestDatasourcePermission_CheckError(t *testing.T) {
	permSvc := &mockInstancePermSvc{
		hasDatasourcePermissionFn: func(ctx context.Context, userID int64, dsID int64, permissionType string) (bool, error) {
			return false, service.ErrInstancePermissionDenied
		},
	}

	router := setupDatasourcePermRouter(permSvc, "read", 1)
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

func TestDatasourcePermission_NoUserID(t *testing.T) {
	permSvc := &mockInstancePermSvc{
		hasDatasourcePermissionFn: func(ctx context.Context, userID int64, dsID int64, permissionType string) (bool, error) {
			t.Error("HasDatasourcePermission should not be called without user_id")
			return false, nil
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(DatasourcePermissionMiddleware(permSvc, "read"))
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestDatasourcePermission_NoIDParam_PassesThrough(t *testing.T) {
	permSvc := &mockInstancePermSvc{
		hasDatasourcePermissionFn: func(ctx context.Context, userID int64, dsID int64, permissionType string) (bool, error) {
			t.Error("HasDatasourcePermission should not be called when no :id param")
			return false, nil
		},
	}

	router := setupDatasourcePermRouter(permSvc, "read", 1)
	router.GET("/datasources", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDatasourcePermission_InvalidID_PassesThrough(t *testing.T) {
	permSvc := &mockInstancePermSvc{
		hasDatasourcePermissionFn: func(ctx context.Context, userID int64, dsID int64, permissionType string) (bool, error) {
			t.Error("HasDatasourcePermission should not be called for invalid :id param")
			return false, nil
		},
	}

	router := setupDatasourcePermRouter(permSvc, "read", 1)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDatasourcePermission_AbortPreventsNextHandler(t *testing.T) {
	nextCalled := false
	permSvc := &mockInstancePermSvc{
		hasDatasourcePermissionFn: func(ctx context.Context, userID int64, dsID int64, permissionType string) (bool, error) {
			return false, nil
		},
	}

	router := setupDatasourcePermRouter(permSvc, "read", 1)
	router.GET("/datasources/:id", func(c *gin.Context) {
		nextCalled = true
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if nextCalled {
		t.Error("next handler should not be called after abort")
	}
}

func TestDatasourcePermission_CorrectParams(t *testing.T) {
	var capturedUserID int64
	var capturedDsID int64
	var capturedAction string

	permSvc := &mockInstancePermSvc{
		hasDatasourcePermissionFn: func(ctx context.Context, userID int64, dsID int64, permissionType string) (bool, error) {
			capturedUserID = userID
			capturedDsID = dsID
			capturedAction = permissionType
			return true, nil
		},
	}

	router := setupDatasourcePermRouter(permSvc, "update", 42)
	router.GET("/datasources/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/datasources/100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if capturedUserID != 42 {
		t.Errorf("expected userID 42, got %d", capturedUserID)
	}
	if capturedDsID != 100 {
		t.Errorf("expected dsID 100, got %d", capturedDsID)
	}
	if capturedAction != "update" {
		t.Errorf("expected action 'update', got '%s'", capturedAction)
	}
}
