package model

import "testing"

func TestRoleModel(t *testing.T) {
	t.Run("创建角色对象", func(t *testing.T) {
		role := &Role{
			ID:          1,
			Name:        "测试角色",
			Code:        "test_role",
			Description: "测试用角色",
			IsSystem:    false,
		}

		if role.Name != "测试角色" {
			t.Errorf("期望 Name 为 '测试角色'，实际为 '%s'", role.Name)
		}
		if role.Code != "test_role" {
			t.Errorf("期望 Code 为 'test_role'，实际为 '%s'", role.Code)
		}
		if role.IsSystem {
			t.Error("期望 IsSystem 为 false")
		}
	})

	t.Run("IsGlobal 全局角色", func(t *testing.T) {
		role := &Role{
			DomainID: nil,
		}

		if !role.IsGlobal() {
			t.Error("期望 DomainID 为 nil 时 IsGlobal() 返回 true")
		}
	})

	t.Run("IsGlobal 领域角色", func(t *testing.T) {
		domainID := int64(1)
		role := &Role{
			DomainID: &domainID,
		}

		if role.IsGlobal() {
			t.Error("期望 DomainID 不为 nil 时 IsGlobal() 返回 false")
		}
	})

	t.Run("IsSystemAdmin 系统管理员", func(t *testing.T) {
		role := &Role{
			Code: "system_admin",
		}

		if !role.IsSystemAdmin() {
			t.Error("期望 Code 为 'system_admin' 时 IsSystemAdmin() 返回 true")
		}
	})

	t.Run("IsSystemAdmin 非系统管理员", func(t *testing.T) {
		role := &Role{
			Code: "domain_admin",
		}

		if role.IsSystemAdmin() {
			t.Error("期望 Code 不为 'system_admin' 时 IsSystemAdmin() 返回 false")
		}
	})

	t.Run("IsDomainAdmin 领域管理员", func(t *testing.T) {
		role := &Role{
			Code: "domain_admin",
		}

		if !role.IsDomainAdmin() {
			t.Error("期望 Code 为 'domain_admin' 时 IsDomainAdmin() 返回 true")
		}
	})

	t.Run("IsDomainAdmin 非领域管理员", func(t *testing.T) {
		role := &Role{
			Code: "user",
		}

		if role.IsDomainAdmin() {
			t.Error("期望 Code 不为 'domain_admin' 时 IsDomainAdmin() 返回 false")
		}
	})

	t.Run("GetCode 获取权限代码", func(t *testing.T) {
		role := &Role{
			Code: "test_role",
		}

		if role.GetCode() != "test_role" {
			t.Errorf("期望 GetCode() 返回 'test_role'，实际为 '%s'", role.GetCode())
		}
	})

	t.Run("角色带父角色", func(t *testing.T) {
		parentID := int64(1)
		role := &Role{
			ID:       2,
			Code:     "child_role",
			ParentID: &parentID,
		}

		if role.ParentID == nil || *role.ParentID != 1 {
			t.Error("期望 ParentID 为 1")
		}
	})

	t.Run("角色带领域", func(t *testing.T) {
		domainID := int64(5)
		role := &Role{
			ID:       3,
			Code:     "domain_role",
			DomainID: &domainID,
		}

		if role.DomainID == nil || *role.DomainID != 5 {
			t.Error("期望 DomainID 为 5")
		}
	})

	t.Run("角色带权限列表", func(t *testing.T) {
		role := &Role{
			ID:   1,
			Code: "admin",
			Permissions: []*Permission{
				{ID: 1, Resource: "task", Action: "create"},
				{ID: 2, Resource: "task", Action: "read"},
			},
		}

		if len(role.Permissions) != 2 {
			t.Errorf("期望 2 个权限，实际为 %d", len(role.Permissions))
		}
	})
}

func TestPermissionModel(t *testing.T) {
	t.Run("创建权限对象", func(t *testing.T) {
		perm := &Permission{
			ID:          1,
			Resource:    "task",
			Action:      "create",
			Description: "创建任务",
		}

		if perm.Resource != "task" {
			t.Errorf("期望 Resource 为 'task'，实际为 '%s'", perm.Resource)
		}
		if perm.Action != "create" {
			t.Errorf("期望 Action 为 'create'，实际为 '%s'", perm.Action)
		}
	})

	t.Run("GetCode 权限代码格式", func(t *testing.T) {
		perm := &Permission{
			Resource: "task",
			Action:   "create",
		}

		expected := "task:create"
		if perm.GetCode() != expected {
			t.Errorf("期望 GetCode() 返回 '%s'，实际为 '%s'", expected, perm.GetCode())
		}
	})

	t.Run("BuildPermissionGroups 从数据库权限构建分组", func(t *testing.T) {
		permissions := []*Permission{
			{ID: 1, Resource: "user", Action: "create", Description: "创建用户"},
			{ID: 2, Resource: "user", Action: "read", Description: "查看用户"},
			{ID: 3, Resource: "task", Action: "create", Description: "创建任务"},
			{ID: 4, Resource: "task", Action: "read", Description: "查看任务"},
			{ID: 5, Resource: "audit_log", Action: "read", Description: "查看审计日志"},
		}

		groups := BuildPermissionGroups(permissions)

		if len(groups) != 3 {
			t.Errorf("期望 3 个权限分组，实际为 %d", len(groups))
		}

		resources := make(map[string]bool)
		for _, group := range groups {
			resources[group.Resource] = true
		}

		expectedResources := []string{"user", "task", "audit_log"}
		for _, res := range expectedResources {
			if !resources[res] {
				t.Errorf("期望包含资源 '%s'", res)
			}
		}
	})

	t.Run("BuildPermissionGroups 权限ID正确传递", func(t *testing.T) {
		permissions := []*Permission{
			{ID: 10, Resource: "task", Action: "create", Description: "创建任务"},
			{ID: 20, Resource: "task", Action: "read", Description: "查看任务"},
		}

		groups := BuildPermissionGroups(permissions)

		for _, group := range groups {
			if group.Resource == "task" {
				for _, perm := range group.Permissions {
					if perm.ID == 0 {
						t.Error("权限ID不应为0，应从数据库权限中获取")
					}
				}
			}
		}
	})

	t.Run("BuildPermissionGroups 资源名称映射", func(t *testing.T) {
		permissions := []*Permission{
			{ID: 1, Resource: "audit_log", Action: "read", Description: "查看审计日志"},
		}

		groups := BuildPermissionGroups(permissions)

		if len(groups) == 0 {
			t.Fatal("期望至少有一个权限分组")
		}

		if groups[0].ResourceName != "审计日志" {
			t.Errorf("期望 audit_log 资源名称为 '审计日志'，实际为 '%s'", groups[0].ResourceName)
		}
	})

	t.Run("BuildPermissionGroups 空权限列表", func(t *testing.T) {
		groups := BuildPermissionGroups([]*Permission{})

		if len(groups) != 0 {
			t.Errorf("期望 0 个权限分组，实际为 %d", len(groups))
		}
	})

	t.Run("BuildPermissionGroups 按 resourceOrder 排序", func(t *testing.T) {
		permissions := []*Permission{
			{ID: 1, Resource: "workflow", Action: "create", Description: "创建工作流"},
			{ID: 2, Resource: "user", Action: "create", Description: "创建用户"},
			{ID: 3, Resource: "task", Action: "create", Description: "创建任务"},
		}

		groups := BuildPermissionGroups(permissions)

		if len(groups) != 3 {
			t.Fatalf("期望 3 个权限分组，实际为 %d", len(groups))
		}

		if groups[0].Resource != "user" {
			t.Errorf("期望第一个分组为 'user'，实际为 '%s'", groups[0].Resource)
		}
		if groups[1].Resource != "task" {
			t.Errorf("期望第二个分组为 'task'，实际为 '%s'", groups[1].Resource)
		}
		if groups[2].Resource != "workflow" {
			t.Errorf("期望第三个分组为 'workflow'，实际为 '%s'", groups[2].Resource)
		}
	})

	t.Run("BuildPermissionGroups 不包含 menu 资源", func(t *testing.T) {
		permissions := []*Permission{
			{ID: 1, Resource: "task", Action: "create", Description: "创建任务"},
			{ID: 2, Resource: "user", Action: "read", Description: "查看用户"},
		}

		groups := BuildPermissionGroups(permissions)

		for _, group := range groups {
			if group.Resource == "menu" {
				t.Error("权限分组不应包含 menu 资源")
			}
		}
	})
}

func TestUserRoleModel(t *testing.T) {
	t.Run("创建用户角色映射", func(t *testing.T) {
		userRole := &UserRole{
			ID:       1,
			UserID:   1,
			RoleID:   1,
			DomainID: nil,
		}

		if userRole.UserID != 1 {
			t.Errorf("期望 UserID 为 1，实际为 %d", userRole.UserID)
		}
		if userRole.RoleID != 1 {
			t.Errorf("期望 RoleID 为 1，实际为 %d", userRole.RoleID)
		}
		if userRole.DomainID != nil {
			t.Error("期望 DomainID 为 nil")
		}
	})

	t.Run("用户角色映射带领域", func(t *testing.T) {
		domainID := int64(2)
		userRole := &UserRole{
			ID:       2,
			UserID:   1,
			RoleID:   2,
			DomainID: &domainID,
		}

		if userRole.DomainID == nil {
			t.Error("期望 DomainID 不为 nil")
		}
		if *userRole.DomainID != 2 {
			t.Errorf("期望 DomainID 为 2，实际为 %d", *userRole.DomainID)
		}
	})

	t.Run("用户角色带关联角色信息", func(t *testing.T) {
		userRole := &UserRole{
			ID:     1,
			UserID: 1,
			RoleID: 1,
			Role: &Role{
				ID:   1,
				Code: "system_admin",
			},
		}

		if userRole.Role == nil {
			t.Error("期望 Role 不为 nil")
		}
		if userRole.Role.Code != "system_admin" {
			t.Errorf("期望 Role.Code 为 'system_admin'，实际为 '%s'", userRole.Role.Code)
		}
	})
}

func TestUserDomainInfoModel(t *testing.T) {
	t.Run("创建用户领域信息", func(t *testing.T) {
		info := &UserDomainInfo{
			DomainID:   1,
			DomainName: "production",
			IsDefault:  true,
		}

		if info.DomainID != 1 {
			t.Errorf("期望 DomainID 为 1，实际为 %d", info.DomainID)
		}
		if info.DomainName != "production" {
			t.Errorf("期望 DomainName 为 'production'，实际为 '%s'", info.DomainName)
		}
		if !info.IsDefault {
			t.Error("期望 IsDefault 为 true")
		}
	})

	t.Run("非默认领域", func(t *testing.T) {
		info := &UserDomainInfo{
			DomainID:   2,
			DomainName: "staging",
			IsDefault:  false,
		}

		if info.IsDefault {
			t.Error("期望 IsDefault 为 false")
		}
	})
}

func TestUserDomainModel(t *testing.T) {
	t.Run("创建用户领域映射", func(t *testing.T) {
		ud := &UserDomain{
			ID:        1,
			UserID:    1,
			DomainID:  1,
			IsDefault: true,
		}

		if ud.UserID != 1 {
			t.Errorf("期望 UserID 为 1，实际为 %d", ud.UserID)
		}
		if ud.DomainID != 1 {
			t.Errorf("期望 DomainID 为 1，实际为 %d", ud.DomainID)
		}
		if !ud.IsDefault {
			t.Error("期望 IsDefault 为 true")
		}
	})
}

func TestDatasourcePermissionModel(t *testing.T) {
	t.Run("创建数据源权限-角色级别", func(t *testing.T) {
		roleID := int64(1)
		grantedBy := int64(10)
		perm := &DatasourcePermission{
			ID:             1,
			DatasourceID:   100,
			RoleID:         &roleID,
			PermissionType: "read",
			GrantedBy:      &grantedBy,
			GrantedAt:      "2024-01-01 00:00:00",
		}

		if perm.DatasourceID != 100 {
			t.Errorf("期望 DatasourceID 为 100，实际为 %d", perm.DatasourceID)
		}
		if perm.RoleID == nil || *perm.RoleID != 1 {
			t.Error("期望 RoleID 为 1")
		}
		if perm.UserID != nil {
			t.Error("期望 UserID 为 nil")
		}
		if perm.PermissionType != "read" {
			t.Errorf("期望 PermissionType 为 'read'，实际为 '%s'", perm.PermissionType)
		}
	})

	t.Run("创建数据源权限-用户级别", func(t *testing.T) {
		userID := int64(5)
		perm := &DatasourcePermission{
			ID:             2,
			DatasourceID:   200,
			UserID:         &userID,
			PermissionType: "manage",
		}

		if perm.UserID == nil || *perm.UserID != 5 {
			t.Error("期望 UserID 为 5")
		}
		if perm.RoleID != nil {
			t.Error("期望 RoleID 为 nil")
		}
		if perm.PermissionType != "manage" {
			t.Errorf("期望 PermissionType 为 'manage'，实际为 '%s'", perm.PermissionType)
		}
	})
}

func TestWebhookPermissionModel(t *testing.T) {
	t.Run("创建Webhook权限-角色级别", func(t *testing.T) {
		roleID := int64(1)
		grantedBy := int64(10)
		perm := &WebhookPermission{
			ID:             1,
			WebhookID:      100,
			RoleID:         &roleID,
			PermissionType: "read",
			GrantedBy:      &grantedBy,
			GrantedAt:      "2024-01-01 00:00:00",
		}

		if perm.WebhookID != 100 {
			t.Errorf("期望 WebhookID 为 100，实际为 %d", perm.WebhookID)
		}
		if perm.RoleID == nil || *perm.RoleID != 1 {
			t.Error("期望 RoleID 为 1")
		}
		if perm.UserID != nil {
			t.Error("期望 UserID 为 nil")
		}
		if perm.PermissionType != "read" {
			t.Errorf("期望 PermissionType 为 'read'，实际为 '%s'", perm.PermissionType)
		}
	})

	t.Run("创建Webhook权限-用户级别", func(t *testing.T) {
		userID := int64(5)
		perm := &WebhookPermission{
			ID:             2,
			WebhookID:      200,
			UserID:         &userID,
			PermissionType: "manage",
		}

		if perm.UserID == nil || *perm.UserID != 5 {
			t.Error("期望 UserID 为 5")
		}
		if perm.RoleID != nil {
			t.Error("期望 RoleID 为 nil")
		}
	})
}

func TestDomainExecutorModel(t *testing.T) {
	t.Run("创建执行器领域分配", func(t *testing.T) {
		de := &DomainExecutor{
			ID:         1,
			DomainID:   1,
			ExecutorID: 1,
		}

		if de.DomainID != 1 {
			t.Errorf("期望 DomainID 为 1，实际为 %d", de.DomainID)
		}
		if de.ExecutorID != 1 {
			t.Errorf("期望 ExecutorID 为 1，实际为 %d", de.ExecutorID)
		}
	})

	t.Run("执行器领域分配带分配者", func(t *testing.T) {
		assignedBy := int64(1)
		de := &DomainExecutor{
			ID:         2,
			DomainID:   2,
			ExecutorID: 1,
			AssignedBy: &assignedBy,
		}

		if de.AssignedBy == nil {
			t.Error("期望 AssignedBy 不为 nil")
		}
		if *de.AssignedBy != 1 {
			t.Errorf("期望 AssignedBy 为 1，实际为 %d", *de.AssignedBy)
		}
	})
}

func TestExecutorWithDomainsModel(t *testing.T) {
	t.Run("创建带领域的执行器", func(t *testing.T) {
		executor := &ExecutorWithDomains{
			Executor: Executor{
				ID:    1,
				Name:  "test-executor",
				Status: "online",
			},
			IsGlobal: false,
			Domains: []*Domain{
				{ID: 1, Name: "Domain 1"},
				{ID: 2, Name: "Domain 2"},
			},
		}

		if len(executor.Domains) != 2 {
			t.Errorf("期望 2 个领域，实际为 %d", len(executor.Domains))
		}
		if executor.IsGlobal {
			t.Error("期望 IsGlobal 为 false")
		}
	})

	t.Run("全局执行器", func(t *testing.T) {
		executor := &ExecutorWithDomains{
			Executor: Executor{
				ID:    2,
				Name:  "global-executor",
				Status: "online",
			},
			IsGlobal: true,
		}

		if !executor.IsGlobal {
			t.Error("期望 IsGlobal 为 true")
		}
	})
}

func TestDomainWithStatsModel(t *testing.T) {
	t.Run("创建带统计的领域", func(t *testing.T) {
		domain := &DomainWithStats{
			Domain: Domain{
				ID:   1,
				Name: "Test Domain",
			},
			UserCount:     10,
			ExecutorCount: 5,
			TaskCount:     100,
		}

		if domain.UserCount != 10 {
			t.Errorf("期望 UserCount 为 10，实际为 %d", domain.UserCount)
		}
		if domain.ExecutorCount != 5 {
			t.Errorf("期望 ExecutorCount 为 5，实际为 %d", domain.ExecutorCount)
		}
		if domain.TaskCount != 100 {
			t.Errorf("期望 TaskCount 为 100，实际为 %d", domain.TaskCount)
		}
	})
}

func TestSwitchDomainRequestModel(t *testing.T) {
	t.Run("创建领域切换请求", func(t *testing.T) {
		req := &SwitchDomainRequest{
			DomainID: 5,
		}

		if req.DomainID != 5 {
			t.Errorf("期望 DomainID 为 5，实际为 %d", req.DomainID)
		}
	})
}

func TestRoleRequestModel(t *testing.T) {
	t.Run("创建角色请求", func(t *testing.T) {
		parentID := int64(1)
		domainID := int64(2)
		req := &RoleRequest{
			Name:        "测试角色",
			Code:        "test_role",
			Description: "测试用角色",
			ParentID:    &parentID,
			DomainID:    &domainID,
		}

		if req.Name != "测试角色" {
			t.Errorf("期望 Name 为 '测试角色'，实际为 '%s'", req.Name)
		}
		if req.Code != "test_role" {
			t.Errorf("期望 Code 为 'test_role'，实际为 '%s'", req.Code)
		}
		if req.ParentID == nil || *req.ParentID != 1 {
			t.Error("期望 ParentID 为 1")
		}
		if req.DomainID == nil || *req.DomainID != 2 {
			t.Error("期望 DomainID 为 2")
		}
	})

	t.Run("全局角色请求无领域", func(t *testing.T) {
		req := &RoleRequest{
			Name:        "全局角色",
			Code:        "global_role",
			Description: "全局角色",
			ParentID:    nil,
			DomainID:    nil,
		}

		if req.ParentID != nil {
			t.Error("期望 ParentID 为 nil")
		}
		if req.DomainID != nil {
			t.Error("期望 DomainID 为 nil")
		}
	})
}

func TestUserRoleDetailModel(t *testing.T) {
	t.Run("创建用户角色详情", func(t *testing.T) {
		domainID := int64(1)
		detail := &UserRoleDetail{
			RoleID:     1,
			RoleName:   "系统管理员",
			RoleCode:   "system_admin",
			DomainID:   &domainID,
			DomainName: "默认领域",
		}

		if detail.RoleID != 1 {
			t.Errorf("期望 RoleID 为 1，实际为 %d", detail.RoleID)
		}
		if detail.RoleCode != "system_admin" {
			t.Errorf("期望 RoleCode 为 'system_admin'，实际为 '%s'", detail.RoleCode)
		}
		if detail.DomainID == nil || *detail.DomainID != 1 {
			t.Error("期望 DomainID 为 1")
		}
		if detail.DomainName != "默认领域" {
			t.Errorf("期望 DomainName 为 '默认领域'，实际为 '%s'", detail.DomainName)
		}
	})

	t.Run("全局角色详情无领域", func(t *testing.T) {
		detail := &UserRoleDetail{
			RoleID:   1,
			RoleName: "系统管理员",
			RoleCode: "system_admin",
			DomainID: nil,
		}

		if detail.DomainID != nil {
			t.Error("期望 DomainID 为 nil")
		}
	})
}
