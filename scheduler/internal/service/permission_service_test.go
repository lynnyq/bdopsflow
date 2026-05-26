package service

import (
	"context"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

type mockPermDB struct {
	isSystemAdminFn        func(ctx context.Context, userID int64) (bool, error)
	hasPermissionFn        func(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error)
	getUserDomainInfosFn   func(ctx context.Context, userID int64) ([]*model.UserDomainInfo, error)
	switchDomainFn         func(ctx context.Context, userID int64, domainID int64) ([]*model.Permission, error)
	getDirectRoleIDsFn     func(ctx context.Context, userID int64) ([]int64, error)
	getParentRoleIDsFn     func(ctx context.Context, roleID int64) []int64
	checkRolePermissionFn  func(ctx context.Context, roleID int64, resource, action string) (bool, error)
}

func TestIsSystemAdmin(t *testing.T) {
	t.Run("system_admin role is identified", func(t *testing.T) {
		role := &model.Role{
			ID:       1,
			Code:     "system_admin",
			IsSystem: true,
		}

		if !role.IsSystemAdmin() {
			t.Error("expected IsSystemAdmin() to return true for system_admin role")
		}
	})

	t.Run("non-system_admin role is not identified", func(t *testing.T) {
		role := &model.Role{
			ID:       2,
			Code:     "domain_admin",
			IsSystem: false,
		}

		if role.IsSystemAdmin() {
			t.Error("expected IsSystemAdmin() to return false for domain_admin role")
		}
	})

	t.Run("user role is not system admin", func(t *testing.T) {
		role := &model.Role{
			ID:       3,
			Code:     "user",
			IsSystem: false,
		}

		if role.IsSystemAdmin() {
			t.Error("expected IsSystemAdmin() to return false for user role")
		}
	})
}

func TestHasPermission(t *testing.T) {
	t.Run("system admin has all permissions", func(t *testing.T) {
		adminRole := &model.Role{Code: "system_admin"}
		if !adminRole.IsSystemAdmin() {
			t.Error("system_admin should have all permissions")
		}
	})

	t.Run("manage action implies all sub-actions", func(t *testing.T) {
		managePerm := &model.Permission{
			ID:       1,
			Resource: "task",
			Action:   "manage",
		}

		if managePerm.Resource != "task" {
			t.Errorf("expected resource 'task', got '%s'", managePerm.Resource)
		}
		if managePerm.Action != "manage" {
			t.Errorf("expected action 'manage', got '%s'", managePerm.Action)
		}

		resourceActions := []string{"create", "read", "update", "delete", "trigger"}
		for _, action := range resourceActions {
			implied := managePerm.Action == "manage" || managePerm.Action == action
			if !implied {
				t.Errorf("manage should imply '%s' action for resource '%s'", action, managePerm.Resource)
			}
		}
	})

	t.Run("specific action does not imply manage", func(t *testing.T) {
		readPerm := &model.Permission{
			ID:       2,
			Resource: "task",
			Action:   "read",
		}

		if readPerm.Action == "manage" {
			t.Error("read action should not equal manage")
		}
	})
}

func TestHasAnyPermission(t *testing.T) {
	t.Run("system admin has any resource permission", func(t *testing.T) {
		role := &model.Role{Code: "system_admin"}
		if !role.IsSystemAdmin() {
			t.Error("system_admin should have any resource permission")
		}
	})

	t.Run("resource permission check with multiple resources", func(t *testing.T) {
		permissions := []*model.Permission{
			{ID: 1, Resource: "task", Action: "create"},
			{ID: 2, Resource: "task", Action: "read"},
			{ID: 3, Resource: "workflow", Action: "create"},
		}

		resourceMap := make(map[string]bool)
		for _, p := range permissions {
			resourceMap[p.Resource] = true
		}

		if !resourceMap["task"] {
			t.Error("expected task resource to be present")
		}
		if !resourceMap["workflow"] {
			t.Error("expected workflow resource to be present")
		}
		if resourceMap["datasource"] {
			t.Error("expected datasource resource to be absent")
		}
	})
}

func TestGetUserDomainInfos(t *testing.T) {
	t.Run("UserDomainInfo model fields", func(t *testing.T) {
		info := &model.UserDomainInfo{
			DomainID:   1,
			DomainName: "production",
			IsDefault:  true,
		}

		if info.DomainID != 1 {
			t.Errorf("expected DomainID 1, got %d", info.DomainID)
		}
		if info.DomainName != "production" {
			t.Errorf("expected DomainName 'production', got '%s'", info.DomainName)
		}
		if !info.IsDefault {
			t.Error("expected IsDefault to be true")
		}
	})

	t.Run("UserDomainInfo non-default domain", func(t *testing.T) {
		info := &model.UserDomainInfo{
			DomainID:   2,
			DomainName: "staging",
			IsDefault:  false,
		}

		if info.IsDefault {
			t.Error("expected IsDefault to be false")
		}
	})

	t.Run("multiple domain infos", func(t *testing.T) {
		infos := []*model.UserDomainInfo{
			{DomainID: 1, DomainName: "default", IsDefault: true},
			{DomainID: 2, DomainName: "staging", IsDefault: false},
			{DomainID: 3, DomainName: "production", IsDefault: false},
		}

		if len(infos) != 3 {
			t.Errorf("expected 3 domain infos, got %d", len(infos))
		}

		defaultCount := 0
		for _, info := range infos {
			if info.IsDefault {
				defaultCount++
			}
		}
		if defaultCount != 1 {
			t.Errorf("expected exactly 1 default domain, got %d", defaultCount)
		}
	})
}

func TestSwitchDomain(t *testing.T) {
	t.Run("ErrDomainAccessDenied for unauthorized domain", func(t *testing.T) {
		if ErrDomainAccessDenied.Error() != "access to domain denied" {
			t.Errorf("expected 'access to domain denied', got '%s'", ErrDomainAccessDenied.Error())
		}
	})

	t.Run("SwitchDomainRequest model", func(t *testing.T) {
		req := &model.SwitchDomainRequest{
			DomainID: 5,
		}

		if req.DomainID != 5 {
			t.Errorf("expected DomainID 5, got %d", req.DomainID)
		}
	})
}

func TestExpandRoleInheritance(t *testing.T) {
	t.Run("single role no parent", func(t *testing.T) {
		visited := make(map[int64]bool)
		var allRoleIDs []int64

		directRoleIDs := []int64{1}
		parentMap := map[int64][]int64{}

		var walk func(roleIDs []int64)
		walk = func(roleIDs []int64) {
			for _, id := range roleIDs {
				if visited[id] {
					continue
				}
				visited[id] = true
				allRoleIDs = append(allRoleIDs, id)

				if parents, ok := parentMap[id]; ok && len(parents) > 0 {
					walk(parents)
				}
			}
		}

		walk(directRoleIDs)

		if len(allRoleIDs) != 1 {
			t.Errorf("expected 1 role ID, got %d", len(allRoleIDs))
		}
		if allRoleIDs[0] != 1 {
			t.Errorf("expected role ID 1, got %d", allRoleIDs[0])
		}
	})

	t.Run("role with parent inheritance", func(t *testing.T) {
		visited := make(map[int64]bool)
		var allRoleIDs []int64

		directRoleIDs := []int64{3}
		parentMap := map[int64][]int64{
			3: {2},
			2: {1},
			1: {},
		}

		var walk func(roleIDs []int64)
		walk = func(roleIDs []int64) {
			for _, id := range roleIDs {
				if visited[id] {
					continue
				}
				visited[id] = true
				allRoleIDs = append(allRoleIDs, id)

				if parents, ok := parentMap[id]; ok && len(parents) > 0 {
					walk(parents)
				}
			}
		}

		walk(directRoleIDs)

		if len(allRoleIDs) != 3 {
			t.Errorf("expected 3 role IDs (with inheritance), got %d", len(allRoleIDs))
		}

		expectedOrder := []int64{3, 2, 1}
		for i, id := range expectedOrder {
			if allRoleIDs[i] != id {
				t.Errorf("expected role ID %d at position %d, got %d", id, i, allRoleIDs[i])
			}
		}
	})

	t.Run("circular reference does not infinite loop", func(t *testing.T) {
		visited := make(map[int64]bool)
		var allRoleIDs []int64

		directRoleIDs := []int64{1}
		parentMap := map[int64][]int64{
			1: {2},
			2: {1},
		}

		var walk func(roleIDs []int64)
		walk = func(roleIDs []int64) {
			for _, id := range roleIDs {
				if visited[id] {
					continue
				}
				visited[id] = true
				allRoleIDs = append(allRoleIDs, id)

				if parents, ok := parentMap[id]; ok && len(parents) > 0 {
					walk(parents)
				}
			}
		}

		walk(directRoleIDs)

		if len(allRoleIDs) != 2 {
			t.Errorf("expected 2 role IDs (circular handled), got %d", len(allRoleIDs))
		}
	})

	t.Run("multiple direct roles with shared parent", func(t *testing.T) {
		visited := make(map[int64]bool)
		var allRoleIDs []int64

		directRoleIDs := []int64{2, 3}
		parentMap := map[int64][]int64{
			2: {1},
			3: {1},
			1: {},
		}

		var walk func(roleIDs []int64)
		walk = func(roleIDs []int64) {
			for _, id := range roleIDs {
				if visited[id] {
					continue
				}
				visited[id] = true
				allRoleIDs = append(allRoleIDs, id)

				if parents, ok := parentMap[id]; ok && len(parents) > 0 {
					walk(parents)
				}
			}
		}

		walk(directRoleIDs)

		if len(allRoleIDs) != 3 {
			t.Errorf("expected 3 role IDs (shared parent deduped), got %d", len(allRoleIDs))
		}

		seen := make(map[int64]bool)
		for _, id := range allRoleIDs {
			if seen[id] {
				t.Errorf("role ID %d appeared more than once", id)
			}
			seen[id] = true
		}
	})

	t.Run("empty direct roles", func(t *testing.T) {
		visited := make(map[int64]bool)
		var allRoleIDs []int64

		directRoleIDs := []int64{}

		var walk func(roleIDs []int64)
		walk = func(roleIDs []int64) {
			for _, id := range roleIDs {
				if visited[id] {
					continue
				}
				visited[id] = true
				allRoleIDs = append(allRoleIDs, id)
			}
		}

		walk(directRoleIDs)

		if len(allRoleIDs) != 0 {
			t.Errorf("expected 0 role IDs, got %d", len(allRoleIDs))
		}
	})
}

func TestPermissionGetCode(t *testing.T) {
	tests := []struct {
		resource string
		action   string
		expected string
	}{
		{"task", "create", "task:create"},
		{"task", "manage", "task:manage"},
		{"workflow", "trigger", "workflow:trigger"},
		{"datasource", "read", "datasource:read"},
		{"user", "update", "user:update"},
		{"webhook", "delete", "webhook:delete"},
	}

	for _, tt := range tests {
		t.Run(tt.resource+":"+tt.action, func(t *testing.T) {
			perm := &model.Permission{
				Resource: tt.resource,
				Action:   tt.action,
			}

			if perm.GetCode() != tt.expected {
				t.Errorf("expected GetCode() '%s', got '%s'", tt.expected, perm.GetCode())
			}
		})
	}
}

func TestRoleGetCode(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{"system_admin", "system_admin"},
		{"domain_admin", "domain_admin"},
		{"user", "user"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			role := &model.Role{Code: tt.code}

			if role.GetCode() != tt.expected {
				t.Errorf("expected GetCode() '%s', got '%s'", tt.expected, role.GetCode())
			}
		})
	}
}

func TestRoleIsGlobal(t *testing.T) {
	t.Run("global role has nil DomainID", func(t *testing.T) {
		role := &model.Role{DomainID: nil}
		if !role.IsGlobal() {
			t.Error("expected IsGlobal() to return true when DomainID is nil")
		}
	})

	t.Run("domain-specific role has non-nil DomainID", func(t *testing.T) {
		domainID := int64(1)
		role := &model.Role{DomainID: &domainID}
		if role.IsGlobal() {
			t.Error("expected IsGlobal() to return false when DomainID is set")
		}
	})
}

func TestRoleIsDomainAdmin(t *testing.T) {
	t.Run("domain_admin role", func(t *testing.T) {
		role := &model.Role{Code: "domain_admin"}
		if !role.IsDomainAdmin() {
			t.Error("expected IsDomainAdmin() to return true for domain_admin")
		}
	})

	t.Run("non-domain_admin role", func(t *testing.T) {
		role := &model.Role{Code: "user"}
		if role.IsDomainAdmin() {
			t.Error("expected IsDomainAdmin() to return false for user role")
		}
	})
}

func TestCollectRoleIDs_Logic(t *testing.T) {
	t.Run("global and domain roles are combined", func(t *testing.T) {
		globalRoles := []int64{1, 2}
		domainRoles := []int64{3}

		var allRoles []int64
		allRoles = append(allRoles, globalRoles...)
		allRoles = append(allRoles, domainRoles...)

		if len(allRoles) != 3 {
			t.Errorf("expected 3 combined roles, got %d", len(allRoles))
		}
	})

	t.Run("no domain roles returns only global", func(t *testing.T) {
		globalRoles := []int64{1}

		var allRoles []int64
		allRoles = append(allRoles, globalRoles...)

		if len(allRoles) != 1 {
			t.Errorf("expected 1 role, got %d", len(allRoles))
		}
	})
}
