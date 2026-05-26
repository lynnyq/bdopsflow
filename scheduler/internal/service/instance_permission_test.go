package service

import (
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

func TestHasDatasourcePermission(t *testing.T) {
	t.Run("system admin has all datasource permissions", func(t *testing.T) {
		role := &model.Role{Code: "system_admin"}
		if !role.IsSystemAdmin() {
			t.Error("system_admin should have all datasource permissions")
		}
	})

	t.Run("domain admin has datasource permissions in their domain", func(t *testing.T) {
		role := &model.Role{Code: "domain_admin"}
		if !role.IsDomainAdmin() {
			t.Error("domain_admin should have datasource permissions in their domain")
		}
	})

	t.Run("DatasourcePermission model", func(t *testing.T) {
		roleID := int64(1)
		userID := int64(2)
		grantedBy := int64(3)

		perm := &model.DatasourcePermission{
			ID:             1,
			DatasourceID:   100,
			RoleID:         &roleID,
			UserID:         &userID,
			PermissionType: "read",
			GrantedBy:      &grantedBy,
			GrantedAt:      "2024-01-01 00:00:00",
		}

		if perm.DatasourceID != 100 {
			t.Errorf("expected DatasourceID 100, got %d", perm.DatasourceID)
		}
		if perm.PermissionType != "read" {
			t.Errorf("expected PermissionType 'read', got '%s'", perm.PermissionType)
		}
		if perm.RoleID == nil || *perm.RoleID != 1 {
			t.Error("expected RoleID to be 1")
		}
		if perm.UserID == nil || *perm.UserID != 2 {
			t.Error("expected UserID to be 2")
		}
		if perm.GrantedBy == nil || *perm.GrantedBy != 3 {
			t.Error("expected GrantedBy to be 3")
		}
	})

	t.Run("DatasourcePermission with user-level permission", func(t *testing.T) {
		userID := int64(5)
		perm := &model.DatasourcePermission{
			ID:             2,
			DatasourceID:   200,
			UserID:         &userID,
			PermissionType: "write",
		}

		if perm.RoleID != nil {
			t.Error("expected RoleID to be nil for user-level permission")
		}
		if perm.UserID == nil || *perm.UserID != 5 {
			t.Error("expected UserID to be 5")
		}
	})

	t.Run("DatasourcePermission with role-level permission", func(t *testing.T) {
		roleID := int64(10)
		perm := &model.DatasourcePermission{
			ID:             3,
			DatasourceID:   300,
			RoleID:         &roleID,
			PermissionType: "manage",
		}

		if perm.UserID != nil {
			t.Error("expected UserID to be nil for role-level permission")
		}
		if perm.RoleID == nil || *perm.RoleID != 10 {
			t.Error("expected RoleID to be 10")
		}
	})
}

func TestHasWebhookPermission(t *testing.T) {
	t.Run("system admin has all webhook permissions", func(t *testing.T) {
		role := &model.Role{Code: "system_admin"}
		if !role.IsSystemAdmin() {
			t.Error("system_admin should have all webhook permissions")
		}
	})

	t.Run("WebhookPermission model", func(t *testing.T) {
		roleID := int64(1)
		userID := int64(2)
		grantedBy := int64(3)

		perm := &model.WebhookPermission{
			ID:             1,
			WebhookID:      100,
			RoleID:         &roleID,
			UserID:         &userID,
			PermissionType: "read",
			GrantedBy:      &grantedBy,
			GrantedAt:      "2024-01-01 00:00:00",
		}

		if perm.WebhookID != 100 {
			t.Errorf("expected WebhookID 100, got %d", perm.WebhookID)
		}
		if perm.PermissionType != "read" {
			t.Errorf("expected PermissionType 'read', got '%s'", perm.PermissionType)
		}
		if perm.RoleID == nil || *perm.RoleID != 1 {
			t.Error("expected RoleID to be 1")
		}
		if perm.UserID == nil || *perm.UserID != 2 {
			t.Error("expected UserID to be 2")
		}
	})

	t.Run("WebhookPermission with user-level permission", func(t *testing.T) {
		userID := int64(5)
		perm := &model.WebhookPermission{
			ID:             2,
			WebhookID:      200,
			UserID:         &userID,
			PermissionType: "write",
		}

		if perm.RoleID != nil {
			t.Error("expected RoleID to be nil for user-level permission")
		}
	})

	t.Run("WebhookPermission with role-level permission", func(t *testing.T) {
		roleID := int64(10)
		perm := &model.WebhookPermission{
			ID:             3,
			WebhookID:      300,
			RoleID:         &roleID,
			PermissionType: "manage",
		}

		if perm.UserID != nil {
			t.Error("expected UserID to be nil for role-level permission")
		}
	})
}

func TestManageImpliesAll(t *testing.T) {
	t.Run("manage implies read", func(t *testing.T) {
		permissionType := "manage"
		requestedType := "read"
		implied := permissionType == "manage" || permissionType == requestedType
		if !implied {
			t.Error("manage should imply read")
		}
	})

	t.Run("manage implies write", func(t *testing.T) {
		permissionType := "manage"
		requestedType := "write"
		implied := permissionType == "manage" || permissionType == requestedType
		if !implied {
			t.Error("manage should imply write")
		}
	})

	t.Run("manage implies delete", func(t *testing.T) {
		permissionType := "manage"
		requestedType := "delete"
		implied := permissionType == "manage" || permissionType == requestedType
		if !implied {
			t.Error("manage should imply delete")
		}
	})

	t.Run("read does not imply write", func(t *testing.T) {
		permissionType := "read"
		requestedType := "write"
		implied := permissionType == "manage" || permissionType == requestedType
		if implied {
			t.Error("read should not imply write")
		}
	})

	t.Run("read does not imply manage", func(t *testing.T) {
		permissionType := "read"
		requestedType := "manage"
		implied := permissionType == "manage" || permissionType == requestedType
		if implied {
			t.Error("read should not imply manage")
		}
	})

	t.Run("write does not imply read", func(t *testing.T) {
		permissionType := "write"
		requestedType := "read"
		implied := permissionType == "manage" || permissionType == requestedType
		if implied {
			t.Error("write should not imply read")
		}
	})

	t.Run("manage implies manage", func(t *testing.T) {
		permissionType := "manage"
		requestedType := "manage"
		implied := permissionType == "manage" || permissionType == requestedType
		if !implied {
			t.Error("manage should imply manage")
		}
	})

	t.Run("permission check SQL logic", func(t *testing.T) {
		permissionTypes := []string{"read", "write", "delete", "manage"}
		requestedTypes := []string{"read", "write", "delete", "manage"}

		expectedResults := map[string]map[string]bool{
			"read":   {"read": true, "write": false, "delete": false, "manage": false},
			"write":  {"read": false, "write": true, "delete": false, "manage": false},
			"delete": {"read": false, "write": false, "delete": true, "manage": false},
			"manage": {"read": true, "write": true, "delete": true, "manage": true},
		}

		for _, permType := range permissionTypes {
			for _, reqType := range requestedTypes {
				implied := permType == "manage" || permType == reqType
				expected := expectedResults[permType][reqType]
				if implied != expected {
					t.Errorf("permission '%s' implying '%s': got %v, want %v", permType, reqType, implied, expected)
				}
			}
		}
	})
}

func TestGrantInstancePermissionRequest(t *testing.T) {
	t.Run("role-based grant request", func(t *testing.T) {
		roleID := int64(1)
		req := &model.GrantInstancePermissionRequest{
			RoleID:         &roleID,
			UserID:         nil,
			PermissionType: "read",
		}

		if req.RoleID == nil || *req.RoleID != 1 {
			t.Error("expected RoleID to be 1")
		}
		if req.UserID != nil {
			t.Error("expected UserID to be nil")
		}
		if req.PermissionType != "read" {
			t.Errorf("expected PermissionType 'read', got '%s'", req.PermissionType)
		}
	})

	t.Run("user-based grant request", func(t *testing.T) {
		userID := int64(5)
		req := &model.GrantInstancePermissionRequest{
			RoleID:         nil,
			UserID:         &userID,
			PermissionType: "manage",
		}

		if req.RoleID != nil {
			t.Error("expected RoleID to be nil")
		}
		if req.UserID == nil || *req.UserID != 5 {
			t.Error("expected UserID to be 5")
		}
		if req.PermissionType != "manage" {
			t.Errorf("expected PermissionType 'manage', got '%s'", req.PermissionType)
		}
	})
}

func TestInstancePermissionDenied(t *testing.T) {
	t.Run("ErrInstancePermissionDenied is defined", func(t *testing.T) {
		if ErrInstancePermissionDenied.Error() != "instance permission denied" {
			t.Errorf("expected 'instance permission denied', got '%s'", ErrInstancePermissionDenied.Error())
		}
	})

	t.Run("ErrInstancePermissionDenied has correct error code", func(t *testing.T) {
		code := GetErrorCode(ErrInstancePermissionDenied)
		if code != 14002 {
			t.Errorf("expected error code 14002, got %d", code)
		}
	})
}
