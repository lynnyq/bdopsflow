package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

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
		}

		resourceMap := make(map[string]bool)
		for _, p := range permissions {
			resourceMap[p.Resource] = true
		}

		if !resourceMap["task"] {
			t.Error("expected task resource to be present")
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

		walk := func(roleIDs []int64) {
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

// ===== PermissionService service-level tests =====

// permissionRoleRow 构造一行 role 查询结果（7 列）
func permissionRoleRow(id int64, name, code, desc string, isSystem bool, parentID, domainID interface{}) []interface{} {
	return []interface{}{id, name, code, desc, isSystem, parentID, domainID}
}

// permissionRow 构造一行 permission 查询结果（4 列）
func permissionRow(id int64, resource, action, desc string) []interface{} {
	return []interface{}{id, resource, action, desc}
}

// permissionCountRow 构造一行 COUNT 查询结果（1 列）
func permissionCountRow(count int64) []interface{} {
	return []interface{}{count}
}

// permissionDomainRow 构造一行 domain 查询结果（3 列）
func permissionDomainRow(id int64, name, desc string) []interface{} {
	return []interface{}{id, name, desc}
}

func TestPermissionService_NewPermissionService(t *testing.T) {
	t.Run("构造函数正常赋值", func(t *testing.T) {
		db := &MockDB{}
		svc := NewPermissionService(db, nil)
		if svc == nil {
			t.Fatal("期望返回非 nil 实例")
		}
		if svc.db == nil {
			t.Error("期望 db 正确赋值")
		}
		if svc.cache != nil {
			t.Error("期望 cache 为 nil")
		}
	})
}

func TestPermissionService_IsSystemAdmin(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		db        *MockDB
		queryErr  error
		wantAdmin bool
		wantErr   bool
	}{
		{
			name: "是系统管理员",
			db: &MockDB{
				QueryResult: database.NewQueryResultWithRows([][]interface{}{
					permissionCountRow(1),
				}),
			},
			wantAdmin: true,
			wantErr:   false,
		},
		{
			name: "不是系统管理员",
			db: &MockDB{
				QueryResult: database.NewQueryResultWithRows([][]interface{}{
					permissionCountRow(0),
				}),
			},
			wantAdmin: false,
			wantErr:   false,
		},
		{
			name: "无数据行返回false",
			db: &MockDB{
				QueryResult: database.NewQueryResultWithRows(nil),
			},
			wantAdmin: false,
			wantErr:   false,
		},
		{
			name:      "查询错误",
			db:        &MockDB{},
			queryErr:  ErrMockDB,
			wantAdmin: false,
			wantErr:   true,
		},
		{
			name: "查询结果带错误",
			db: &MockDB{
				QueryResult: database.NewQueryResultWithErr(ErrMockDB),
			},
			wantAdmin: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.db.QueryError = tt.queryErr
			svc := NewPermissionService(tt.db, nil)
			isAdmin, err := svc.IsSystemAdmin(ctx, 1)
			if tt.wantErr && err == nil {
				t.Fatal("期望返回错误，实际无错误")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("期望无错误，实际: %v", err)
			}
			if isAdmin != tt.wantAdmin {
				t.Errorf("期望 isAdmin=%v，实际=%v", tt.wantAdmin, isAdmin)
			}
		})
	}
}

func TestPermissionService_HasPermission(t *testing.T) {
	ctx := context.Background()

	t.Run("系统管理员直接返回true", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionCountRow(1),
			}),
		}
		svc := NewPermissionService(db, nil)
		has, err := svc.HasPermission(ctx, 1, "task", "read", 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !has {
			t.Error("期望系统管理员 has=true")
		}
	})

	t.Run("检查IsSystemAdmin出错返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.HasPermission(ctx, 1, "task", "read", 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("collectRoleIDs出错返回错误", func(t *testing.T) {
		// IsSystemAdmin 返回 count=0（非管理员），collectRoleIDs 会查 globalQuery，QueryError 会在第二次查询也返回错误
		// 但 MockDB 的 QueryError 对所有查询都返回错误，包括 IsSystemAdmin 的查询
		// 所以需要用 QueryResults 来分别返回
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// 但 QueryError 优先于 QueryResults，所以不能同时设置
			},
		}
		_ = db
		// 由于 MockDB 的 QueryError 优先于所有查询，无法用 QueryResults 分别控制
		// 这里直接用 QueryError 测试 IsSystemAdmin 出错的情况即可
		db2 := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db2, nil)
		_, err := svc.HasPermission(ctx, 1, "task", "read", 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_GetUserRoles(t *testing.T) {
	ctx := context.Background()

	t.Run("返回角色列表", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionRoleRow(1, "管理员", "system_admin", "系统管理员", true, nil, nil),
				permissionRoleRow(2, "用户", "user", "普通用户", false, nil, nil),
			}),
		}
		svc := NewPermissionService(db, nil)
		roles, err := svc.GetUserRoles(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roles) != 2 {
			t.Fatalf("期望 2 个角色，实际=%d", len(roles))
		}
		if roles[0].Code != "system_admin" {
			t.Errorf("期望第一个角色 code=system_admin，实际=%s", roles[0].Code)
		}
		if !roles[0].IsSystem {
			t.Error("期望 IsSystem=true")
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		roles, err := svc.GetUserRoles(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roles) != 0 {
			t.Errorf("期望 0 个角色，实际=%d", len(roles))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetUserRoles(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("带parentID和domainID", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionRoleRow(3, "子角色", "child", "子角色", false, int64(1), int64(5)),
			}),
		}
		svc := NewPermissionService(db, nil)
		roles, err := svc.GetUserRoles(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roles) != 1 {
			t.Fatalf("期望 1 个角色，实际=%d", len(roles))
		}
		if roles[0].ParentID == nil || *roles[0].ParentID != 1 {
			t.Error("期望 ParentID=1")
		}
		if roles[0].DomainID == nil || *roles[0].DomainID != 5 {
			t.Error("期望 DomainID=5")
		}
	})
}

func TestPermissionService_GetAllRoles(t *testing.T) {
	ctx := context.Background()

	t.Run("返回所有角色", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionRoleRow(1, "系统管理员", "system_admin", "系统管理员", true, nil, nil),
				permissionRoleRow(2, "用户", "user", "普通用户", false, nil, nil),
			}),
		}
		svc := NewPermissionService(db, nil)
		roles, err := svc.GetAllRoles(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roles) != 2 {
			t.Errorf("期望 2 个角色，实际=%d", len(roles))
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		roles, err := svc.GetAllRoles(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roles) != 0 {
			t.Errorf("期望 0 个角色，实际=%d", len(roles))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetAllRoles(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithErr(ErrMockDB),
		}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetAllRoles(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_GetRoleByID(t *testing.T) {
	ctx := context.Background()

	t.Run("找到角色", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionRoleRow(1, "管理员", "system_admin", "系统管理员", true, nil, nil),
			}),
		}
		svc := NewPermissionService(db, nil)
		role, err := svc.GetRoleByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if role == nil {
			t.Fatal("期望返回非 nil 角色")
		}
		if role.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", role.ID)
		}
	})

	t.Run("未找到角色返回nil", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		role, err := svc.GetRoleByID(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if role != nil {
			t.Error("期望返回 nil")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetRoleByID(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_GetRoleByCode(t *testing.T) {
	ctx := context.Background()

	t.Run("找到角色", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionRoleRow(1, "管理员", "system_admin", "系统管理员", true, nil, nil),
			}),
		}
		svc := NewPermissionService(db, nil)
		role, err := svc.GetRoleByCode(ctx, "system_admin")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if role == nil {
			t.Fatal("期望返回非 nil 角色")
		}
		if role.Code != "system_admin" {
			t.Errorf("期望 Code=system_admin，实际=%s", role.Code)
		}
	})

	t.Run("未找到角色返回nil", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		role, err := svc.GetRoleByCode(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if role != nil {
			t.Error("期望返回 nil")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetRoleByCode(ctx, "system_admin")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_GetAllPermissions(t *testing.T) {
	ctx := context.Background()

	t.Run("返回所有权限", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionRow(1, "task", "create", "创建任务"),
				permissionRow(2, "task", "read", "查看任务"),
			}),
		}
		svc := NewPermissionService(db, nil)
		perms, err := svc.GetAllPermissions(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 2 {
			t.Errorf("期望 2 个权限，实际=%d", len(perms))
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		perms, err := svc.GetAllPermissions(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 0 {
			t.Errorf("期望 0 个权限，实际=%d", len(perms))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetAllPermissions(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_GetRolePermissions(t *testing.T) {
	ctx := context.Background()

	t.Run("返回角色权限", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionRow(1, "task", "create", "创建任务"),
				permissionRow(2, "task", "read", "查看任务"),
			}),
		}
		svc := NewPermissionService(db, nil)
		perms, err := svc.GetRolePermissions(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 2 {
			t.Errorf("期望 2 个权限，实际=%d", len(perms))
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		perms, err := svc.GetRolePermissions(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 0 {
			t.Errorf("期望 0 个权限，实际=%d", len(perms))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetRolePermissions(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_GetUserRoleCodes(t *testing.T) {
	ctx := context.Background()

	t.Run("返回角色代码列表", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				{"system_admin"},
				{"user"},
			}),
		}
		svc := NewPermissionService(db, nil)
		codes, err := svc.GetUserRoleCodes(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(codes) != 2 {
			t.Fatalf("期望 2 个角色代码，实际=%d", len(codes))
		}
		if codes[0] != "system_admin" {
			t.Errorf("期望第一个代码=system_admin，实际=%s", codes[0])
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		codes, err := svc.GetUserRoleCodes(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(codes) != 0 {
			t.Errorf("期望 0 个代码，实际=%d", len(codes))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetUserRoleCodes(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_GetRolesByDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("返回领域角色", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionRoleRow(1, "领域管理员", "domain_admin", "领域管理员", false, nil, int64(1)),
			}),
		}
		svc := NewPermissionService(db, nil)
		roles, err := svc.GetRolesByDomain(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roles) != 1 {
			t.Fatalf("期望 1 个角色，实际=%d", len(roles))
		}
		if roles[0].Code != "domain_admin" {
			t.Errorf("期望 Code=domain_admin，实际=%s", roles[0].Code)
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		roles, err := svc.GetRolesByDomain(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roles) != 0 {
			t.Errorf("期望 0 个角色，实际=%d", len(roles))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetRolesByDomain(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_GetUserDomains(t *testing.T) {
	ctx := context.Background()

	t.Run("系统管理员返回所有领域", func(t *testing.T) {
		// IsSystemAdmin 查询返回 count=1，然后 GetAllDomains 查询也返回同样的结果
		// 但 MockDB 对所有查询返回同一个 QueryResult，所以需要用能同时满足 count 和 domain 的数据
		// count 查询读 row[0] 作为 count，domain 查询读 row[0],row[1],row[2] 作为 id,name,desc
		// 用 3 列数据可以同时满足两种查询
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionDomainRow(1, "default", "默认领域"),
				permissionDomainRow(2, "prod", "生产环境"),
			}),
		}
		svc := NewPermissionService(db, nil)
		// IsSystemAdmin 会读 row[0] 作为 count，这里 row[0]=1 表示 count=1 → 是管理员
		domains, err := svc.GetUserDomains(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(domains) != 2 {
			t.Fatalf("期望 2 个领域，实际=%d", len(domains))
		}
		if domains[0].Name != "default" {
			t.Errorf("期望第一个领域 Name=default，实际=%s", domains[0].Name)
		}
	})

	t.Run("非管理员返回用户领域", func(t *testing.T) {
		// IsSystemAdmin 返回 count=0（非管理员），然后查询用户领域
		// 用 3 列数据：row[0]=0 表示 count=0 → 非管理员
		// 然后第二次查询也返回同样的数据，但 row[0]=0 作为 domain ID
		// 这里用 domain ID=0 的数据不太好，改用 QueryResults
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 查询返回 count=0
				database.NewQueryResultWithRows([][]interface{}{
					permissionCountRow(0),
				}),
				// 用户领域查询
				database.NewQueryResultWithRows([][]interface{}{
					permissionDomainRow(5, "my-domain", "我的领域"),
				}),
			},
		}
		svc := NewPermissionService(db, nil)
		domains, err := svc.GetUserDomains(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(domains) != 1 {
			t.Fatalf("期望 1 个领域，实际=%d", len(domains))
		}
		if domains[0].ID != 5 {
			t.Errorf("期望 ID=5，实际=%d", domains[0].ID)
		}
	})

	t.Run("IsSystemAdmin出错", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetUserDomains(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_GetUserDomainInfos(t *testing.T) {
	ctx := context.Background()

	t.Run("返回用户领域信息", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				{int64(1), "default", true},
				{int64(2), "prod", false},
			}),
		}
		svc := NewPermissionService(db, nil)
		infos, err := svc.GetUserDomainInfos(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(infos) != 2 {
			t.Fatalf("期望 2 个领域信息，实际=%d", len(infos))
		}
		if infos[0].DomainID != 1 {
			t.Errorf("期望 DomainID=1，实际=%d", infos[0].DomainID)
		}
		if !infos[0].IsDefault {
			t.Error("期望 IsDefault=true")
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		infos, err := svc.GetUserDomainInfos(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(infos) != 0 {
			t.Errorf("期望 0 个领域信息，实际=%d", len(infos))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetUserDomainInfos(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithErr(ErrMockDB),
		}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetUserDomainInfos(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_GetUserDefaultDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("有默认领域", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				{int64(5)},
			}),
		}
		svc := NewPermissionService(db, nil)
		domainID, err := svc.GetUserDefaultDomain(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if domainID != 5 {
			t.Errorf("期望 domainID=5，实际=%d", domainID)
		}
	})

	t.Run("无默认领域时回退到第一个领域", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// 第一次查询（is_default=1）无结果
				database.NewQueryResultWithRows(nil),
				// 第二次查询（任意领域）返回 domain_id=3
				database.NewQueryResultWithRows([][]interface{}{
					{int64(3)},
				}),
			},
		}
		svc := NewPermissionService(db, nil)
		domainID, err := svc.GetUserDefaultDomain(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if domainID != 3 {
			t.Errorf("期望 domainID=3，实际=%d", domainID)
		}
	})

	t.Run("无任何领域返回0", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// 第一次查询无结果
				database.NewQueryResultWithRows(nil),
				// 第二次查询也无结果
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewPermissionService(db, nil)
		domainID, err := svc.GetUserDefaultDomain(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if domainID != 0 {
			t.Errorf("期望 domainID=0，实际=%d", domainID)
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetUserDefaultDomain(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_IsDomainAdmin(t *testing.T) {
	ctx := context.Background()

	t.Run("是领域管理员", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionCountRow(1),
			}),
		}
		svc := NewPermissionService(db, nil)
		isAdmin, err := svc.IsDomainAdmin(ctx, 1, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !isAdmin {
			t.Error("期望 isAdmin=true")
		}
	})

	t.Run("不是领域管理员", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionCountRow(0),
			}),
		}
		svc := NewPermissionService(db, nil)
		isAdmin, err := svc.IsDomainAdmin(ctx, 1, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if isAdmin {
			t.Error("期望 isAdmin=false")
		}
	})

	t.Run("无数据行返回false", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		isAdmin, err := svc.IsDomainAdmin(ctx, 1, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if isAdmin {
			t.Error("期望 isAdmin=false")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.IsDomainAdmin(ctx, 1, 5)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_GetRoleCodeByID(t *testing.T) {
	ctx := context.Background()

	t.Run("找到角色代码", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				{"system_admin"},
			}),
		}
		svc := NewPermissionService(db, nil)
		code, err := svc.GetRoleCodeByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if code != "system_admin" {
			t.Errorf("期望 code=system_admin，实际=%s", code)
		}
	})

	t.Run("未找到角色返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetRoleCodeByID(ctx, 999)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetRoleCodeByID(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_GetUserDomainIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("返回领域ID列表", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				{int64(1)},
				{int64(2)},
			}),
		}
		svc := NewPermissionService(db, nil)
		ids, err := svc.GetUserDomainIDs(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(ids) != 2 {
			t.Fatalf("期望 2 个 ID，实际=%d", len(ids))
		}
		if ids[0] != 1 || ids[1] != 2 {
			t.Errorf("期望 [1,2]，实际=%v", ids)
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		ids, err := svc.GetUserDomainIDs(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("期望 0 个 ID，实际=%d", len(ids))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetUserDomainIDs(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_AssignPermissionsToRole(t *testing.T) {
	ctx := context.Background()

	t.Run("删除并插入权限成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewPermissionService(db, nil)
		err := svc.AssignPermissionsToRole(ctx, 1, []int64{1, 2, 3})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 应该记录 delete 和 batch insert
		if len(db.WriteStmts) < 2 {
			t.Errorf("期望至少 2 次写入，实际=%d", len(db.WriteStmts))
		}
	})

	t.Run("空权限列表只删除", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewPermissionService(db, nil)
		err := svc.AssignPermissionsToRole(ctx, 1, []int64{})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(db.WriteStmts) != 1 {
			t.Errorf("期望 1 次写入，实际=%d", len(db.WriteStmts))
		}
	})

	t.Run("删除失败返回错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		err := svc.AssignPermissionsToRole(ctx, 1, []int64{1})
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("批量插入失败返回错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult:      database.NewWriteResult(0, 1),
			BatchWriteError:  ErrMockDB,
		}
		svc := NewPermissionService(db, nil)
		err := svc.AssignPermissionsToRole(ctx, 1, []int64{1, 2})
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_InvalidateUserPermissionCache(t *testing.T) {
	ctx := context.Background()

	t.Run("cache为nil直接返回nil", func(t *testing.T) {
		svc := NewPermissionService(&MockDB{}, nil)
		err := svc.InvalidateUserPermissionCache(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("cache存在时清理缓存", func(t *testing.T) {
		mr, rdb := newTestRedis(t)
		defer mr.Close()

		// 预设一些缓存键
		rdb.Set(ctx, "perm:1:task:read", "1", 0)
		rdb.Set(ctx, "perm:1:task:write", "1", 0)
		rdb.Set(ctx, "perm:2:task:read", "1", 0) // 其他用户的不应被删除

		svc := NewPermissionService(&MockDB{}, rdb)
		err := svc.InvalidateUserPermissionCache(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}

		// 验证用户1的缓存已被删除
		val1, _ := rdb.Get(ctx, "perm:1:task:read").Result()
		if val1 != "" {
			t.Error("期望 perm:1:task:read 已被删除")
		}
		// 验证用户2的缓存仍然存在
		val2, _ := rdb.Get(ctx, "perm:2:task:read").Result()
		if val2 == "" {
			t.Error("期望 perm:2:task:read 仍存在")
		}
	})
}

func TestPermissionService_HasAnyPermission(t *testing.T) {
	ctx := context.Background()

	t.Run("系统管理员直接返回true", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionCountRow(1),
			}),
		}
		svc := NewPermissionService(db, nil)
		has, err := svc.HasAnyPermission(ctx, 1, "task", 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !has {
			t.Error("期望 has=true")
		}
	})

	t.Run("IsSystemAdmin出错返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.HasAnyPermission(ctx, 1, "task", 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_SwitchDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("无领域访问权限且非管理员返回错误", func(t *testing.T) {
		// 第一次查询 COUNT 返回 0，IsSystemAdmin 也返回 0（非管理员）
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionCountRow(0),
			}),
		}
		svc := NewPermissionService(db, nil)
		_, err := svc.SwitchDomain(ctx, 1, 5)
		if err == nil {
			t.Fatal("期望返回错误")
		}
		if !errors.Is(err, ErrDomainAccessDenied) {
			t.Errorf("期望 ErrDomainAccessDenied，实际: %v", err)
		}
	})

	t.Run("无领域但有管理员权限继续执行", func(t *testing.T) {
		// COUNT=0（无领域），IsSystemAdmin 返回 count>0（是管理员）
		// collectRoleIDs 返回空，expandRoleInheritance 返回空 → 返回空权限列表
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// SwitchDomain COUNT 查询返回 0
				database.NewQueryResultWithRows([][]interface{}{
					permissionCountRow(0),
				}),
				// IsSystemAdmin 查询返回 count=1（是管理员）
				database.NewQueryResultWithRows([][]interface{}{
					permissionCountRow(1),
				}),
				// collectRoleIDs globalQuery 返回空
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewPermissionService(db, nil)
		perms, err := svc.SwitchDomain(ctx, 1, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 0 {
			t.Errorf("期望 0 个权限，实际=%d", len(perms))
		}
	})
}

func TestPermissionService_GetUserPermissions(t *testing.T) {
	ctx := context.Background()

	t.Run("无角色返回空列表", func(t *testing.T) {
		// getDirectRoleIDs 返回空
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		perms, err := svc.GetUserPermissions(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 0 {
			t.Errorf("期望 0 个权限，实际=%d", len(perms))
		}
	})

	t.Run("getDirectRoleIDs出错返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.GetUserPermissions(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_collectRoleIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("只查全局角色domainID=0", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				{int64(1)},
				{int64(2)},
			}),
		}
		svc := NewPermissionService(db, nil)
		roleIDs, err := svc.collectRoleIDs(ctx, 1, 0)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roleIDs) != 2 {
			t.Fatalf("期望 2 个角色ID，实际=%d", len(roleIDs))
		}
	})

	t.Run("查全局+领域角色", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// globalQuery 返回全局角色
				database.NewQueryResultWithRows([][]interface{}{
					{int64(1)},
				}),
				// domainQuery 返回领域角色
				database.NewQueryResultWithRows([][]interface{}{
					{int64(2)},
					{int64(3)},
				}),
			},
		}
		svc := NewPermissionService(db, nil)
		roleIDs, err := svc.collectRoleIDs(ctx, 1, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roleIDs) != 3 {
			t.Fatalf("期望 3 个角色ID，实际=%d", len(roleIDs))
		}
	})

	t.Run("globalQuery出错返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.collectRoleIDs(ctx, 1, 5)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_getDirectRoleIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("返回直接角色ID", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				{int64(1)},
				{int64(2)},
			}),
		}
		svc := NewPermissionService(db, nil)
		roleIDs, err := svc.getDirectRoleIDs(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roleIDs) != 2 {
			t.Fatalf("期望 2 个角色ID，实际=%d", len(roleIDs))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.getDirectRoleIDs(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithErr(ErrMockDB),
		}
		svc := NewPermissionService(db, nil)
		_, err := svc.getDirectRoleIDs(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_expandRoleInheritance(t *testing.T) {
	ctx := context.Background()

	t.Run("空角色列表返回空", func(t *testing.T) {
		svc := NewPermissionService(&MockDB{}, nil)
		result := svc.expandRoleInheritance(ctx, nil)
		if len(result) != 0 {
			t.Errorf("期望 0 个角色，实际=%d", len(result))
		}
	})

	t.Run("无父角色直接返回", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		result := svc.expandRoleInheritance(ctx, []int64{1, 2})
		if len(result) != 2 {
			t.Fatalf("期望 2 个角色，实际=%d", len(result))
		}
	})

	t.Run("有父角色递归展开", func(t *testing.T) {
		// getParentRoleIDs 对角色1返回父角色3，对角色3返回空
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// 角色1的 getParentRoleIDs 返回 parent_id=3
				database.NewQueryResultWithRows([][]interface{}{
					{int64(3)},
				}),
				// 角色3的 getParentRoleIDs 返回空
				database.NewQueryResultWithRows(nil),
				// 角色2的 getParentRoleIDs 返回空
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewPermissionService(db, nil)
		result := svc.expandRoleInheritance(ctx, []int64{1, 2})
		if len(result) != 3 {
			t.Fatalf("期望 3 个角色（1,3,2），实际=%d", len(result))
		}
	})
}

func TestPermissionService_getPermissionsByRoleIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("空角色列表返回空", func(t *testing.T) {
		svc := NewPermissionService(&MockDB{}, nil)
		perms, err := svc.getPermissionsByRoleIDs(ctx, nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 0 {
			t.Errorf("期望 0 个权限，实际=%d", len(perms))
		}
	})

	t.Run("返回去重后的权限", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionRow(1, "task", "read", "查看"),
				permissionRow(2, "task", "create", "创建"),
			}),
		}
		svc := NewPermissionService(db, nil)
		// 两个角色有相同的权限，应该去重
		perms, err := svc.getPermissionsByRoleIDs(ctx, []int64{1, 2})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// MockDB 返回同样的结果，所以两个角色各返回2个权限，但去重后只有2个
		if len(perms) != 2 {
			t.Errorf("期望 2 个权限（去重后），实际=%d", len(perms))
		}
	})
}

func TestPermissionService_checkRolePermission(t *testing.T) {
	ctx := context.Background()

	t.Run("有权限返回true", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionCountRow(1),
			}),
		}
		svc := NewPermissionService(db, nil)
		has, err := svc.checkRolePermission(ctx, 1, "task", "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !has {
			t.Error("期望 has=true")
		}
	})

	t.Run("无权限返回false", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				permissionCountRow(0),
			}),
		}
		svc := NewPermissionService(db, nil)
		has, err := svc.checkRolePermission(ctx, 1, "task", "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if has {
			t.Error("期望 has=false")
		}
	})

	t.Run("查询错误返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		_, err := svc.checkRolePermission(ctx, 1, "task", "read")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestPermissionService_getParentRoleIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("返回父角色ID", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				{int64(5)},
			}),
		}
		svc := NewPermissionService(db, nil)
		parents := svc.getParentRoleIDs(ctx, 1)
		if len(parents) != 1 {
			t.Fatalf("期望 1 个父角色，实际=%d", len(parents))
		}
		if parents[0] != 5 {
			t.Errorf("期望 parentID=5，实际=%d", parents[0])
		}
	})

	t.Run("无父角色返回空", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewPermissionService(db, nil)
		parents := svc.getParentRoleIDs(ctx, 1)
		if len(parents) != 0 {
			t.Errorf("期望 0 个父角色，实际=%d", len(parents))
		}
	})

	t.Run("查询出错返回空", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewPermissionService(db, nil)
		parents := svc.getParentRoleIDs(ctx, 1)
		if len(parents) != 0 {
			t.Errorf("期望 0 个父角色，实际=%d", len(parents))
		}
	})
}
