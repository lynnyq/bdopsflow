package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	rqlite "github.com/rqlite/gorqlite"
)

// === TaskExecution.GetStartTime / GetEndTime 测试 ===

func TestTaskExecution_GetStartTime(t *testing.T) {
	tests := []struct {
		name      string
		startTime rqlite.NullTime
		wantNil   bool
	}{
		{
			name:      "invalid NullTime returns nil",
			startTime: rqlite.NullTime{Valid: false},
			wantNil:   true,
		},
		{
			name:      "zero value NullTime returns nil",
			startTime: rqlite.NullTime{},
			wantNil:   true,
		},
		{
			name: "valid NullTime returns pointer",
			startTime: rqlite.NullTime{
				Valid: true,
				Time:  time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := &TaskExecution{StartTime: tt.startTime}
			got := te.GetStartTime()
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
			} else {
				if got == nil {
					t.Fatal("expected non-nil pointer")
				}
				if !got.Equal(tt.startTime.Time) {
					t.Errorf("got %v, want %v", *got, tt.startTime.Time)
				}
			}
		})
	}
}

func TestTaskExecution_GetEndTime(t *testing.T) {
	tests := []struct {
		name    string
		endTime rqlite.NullTime
		wantNil bool
	}{
		{
			name:    "invalid NullTime returns nil",
			endTime: rqlite.NullTime{Valid: false},
			wantNil: true,
		},
		{
			name:    "zero value NullTime returns nil",
			endTime: rqlite.NullTime{},
			wantNil: true,
		},
		{
			name: "valid NullTime returns pointer",
			endTime: rqlite.NullTime{
				Valid: true,
				Time:  time.Date(2024, 6, 15, 11, 0, 0, 0, time.UTC),
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := &TaskExecution{EndTime: tt.endTime}
			got := te.GetEndTime()
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
			} else {
				if got == nil {
					t.Fatal("expected non-nil pointer")
				}
				if !got.Equal(tt.endTime.Time) {
					t.Errorf("got %v, want %v", *got, tt.endTime.Time)
				}
			}
		})
	}
}

func TestTaskExecution_GetStartTime_GetEndTime_Consistency(t *testing.T) {
	// 验证同一任务的开始时间和结束时间关系
	startTime := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	endTime := time.Date(2024, 6, 15, 10, 5, 30, 0, time.UTC)

	te := &TaskExecution{
		StartTime: rqlite.NullTime{Valid: true, Time: startTime},
		EndTime:   rqlite.NullTime{Valid: true, Time: endTime},
	}

	start := te.GetStartTime()
	end := te.GetEndTime()

	if start == nil || end == nil {
		t.Fatal("expected non-nil start and end times")
	}

	if !end.After(*start) {
		t.Errorf("end time %v should be after start time %v", *end, *start)
	}

	duration := end.Sub(*start)
	if duration != 5*time.Minute+30*time.Second {
		t.Errorf("duration = %v, want 5m30s", duration)
	}
}

// === Executor.GetLastHeartbeat 测试 ===

func TestExecutor_GetLastHeartbeat(t *testing.T) {
	tests := []struct {
		name           string
		lastHeartbeat  rqlite.NullTime
		wantNil        bool
	}{
		{
			name:          "invalid NullTime returns nil",
			lastHeartbeat: rqlite.NullTime{Valid: false},
			wantNil:       true,
		},
		{
			name:          "zero value NullTime returns nil",
			lastHeartbeat: rqlite.NullTime{},
			wantNil:       true,
		},
		{
			name: "valid NullTime returns pointer",
			lastHeartbeat: rqlite.NullTime{
				Valid: true,
				Time:  time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Executor{LastHeartbeat: tt.lastHeartbeat}
			got := e.GetLastHeartbeat()
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
			} else {
				if got == nil {
					t.Fatal("expected non-nil pointer")
				}
				if !got.Equal(tt.lastHeartbeat.Time) {
					t.Errorf("got %v, want %v", *got, tt.lastHeartbeat.Time)
				}
			}
		})
	}
}

// === BuildPermissionGroups 边界场景测试 ===

func TestBuildPermissionGroups_UnknownResource(t *testing.T) {
	// 测试未知资源类型应使用资源名作为 ResourceName
	permissions := []*Permission{
		{ID: 1, Resource: "unknown_resource", Action: "read", Description: "未知资源读权限"},
	}

	groups := BuildPermissionGroups(permissions)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	if groups[0].Resource != "unknown_resource" {
		t.Errorf("expected resource 'unknown_resource', got %q", groups[0].Resource)
	}

	// 未知资源应使用资源名作为 ResourceName（fallback 逻辑）
	if groups[0].ResourceName != "unknown_resource" {
		t.Errorf("expected ResourceName 'unknown_resource' for unknown resource, got %q", groups[0].ResourceName)
	}
}

func TestBuildPermissionGroups_OrderPreservation(t *testing.T) {
	// 验证已知资源按 resourceOrder 顺序排列
	permissions := []*Permission{
		// 故意乱序输入
		{ID: 1, Resource: "task", Action: "create"},
		{ID: 2, Resource: "user", Action: "create"},
		{ID: 3, Resource: "dashboard", Action: "read"},
	}

	groups := BuildPermissionGroups(permissions)

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// 验证顺序：dashboard, user, task（按 resourceOrder）
	expectedOrder := []string{"dashboard", "user", "task"}
	for i, expected := range expectedOrder {
		if groups[i].Resource != expected {
			t.Errorf("group[%d].Resource = %q, want %q", i, groups[i].Resource, expected)
		}
	}
}

func TestBuildPermissionGroups_MixedKnownAndUnknown(t *testing.T) {
	// 混合已知和未知资源，已知资源按顺序排列，未知资源追加到末尾
	permissions := []*Permission{
		{ID: 1, Resource: "custom_resource", Action: "read"}, // 未知
		{ID: 2, Resource: "task", Action: "create"},          // 已知
		{ID: 3, Resource: "user", Action: "read"},            // 已知
		{ID: 4, Resource: "another_custom", Action: "read"},  // 未知
	}

	groups := BuildPermissionGroups(permissions)

	if len(groups) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(groups))
	}

	// 已知资源应在前两个位置（按 resourceOrder: user, task）
	// 未知资源应在后两个位置（顺序不确定，因为来自 map 迭代）
	knownResources := []string{groups[0].Resource, groups[1].Resource}
	if knownResources[0] != "user" || knownResources[1] != "task" {
		t.Errorf("expected known resources [user, task] first, got %v", knownResources)
	}

	// 验证所有资源都被包含
	resourceSet := make(map[string]bool)
	for _, g := range groups {
		resourceSet[g.Resource] = true
	}
	for _, expected := range []string{"task", "user", "custom_resource", "another_custom"} {
		if !resourceSet[expected] {
			t.Errorf("expected resource %q to be present", expected)
		}
	}
}

func TestBuildPermissionGroups_SingleResourceMultiplePermissions(t *testing.T) {
	// 同一资源多个权限应合并到一个分组
	permissions := []*Permission{
		{ID: 1, Resource: "task", Action: "create", Description: "创建任务"},
		{ID: 2, Resource: "task", Action: "read", Description: "查看任务"},
		{ID: 3, Resource: "task", Action: "update", Description: "更新任务"},
		{ID: 4, Resource: "task", Action: "delete", Description: "删除任务"},
	}

	groups := BuildPermissionGroups(permissions)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	if len(groups[0].Permissions) != 4 {
		t.Errorf("expected 4 permissions in group, got %d", len(groups[0].Permissions))
	}

	// 验证 ResourceName 正确映射
	if groups[0].ResourceName != "任务管理" {
		t.Errorf("expected ResourceName '任务管理', got %q", groups[0].ResourceName)
	}
}

func TestBuildPermissionGroups_AllKnownResources(t *testing.T) {
	// 测试所有已知资源类型的映射
	tests := []struct {
		resource string
		wantName string
	}{
		{"dashboard", "仪表盘"},
		{"user", "用户管理"},
		{"role", "角色管理"},
		{"permission", "权限管理"},
		{"domain", "领域管理"},
		{"executor", "执行器管理"},
		{"task", "任务管理"},
		{"log", "日志管理"},
		{"datasource", "数据源管理"},
		{"webhook", "Webhook管理"},
		{"audit_log", "审计日志"},
		{"config", "系统配置"},
	}

	for _, tt := range tests {
		t.Run(tt.resource, func(t *testing.T) {
			permissions := []*Permission{
				{ID: 1, Resource: tt.resource, Action: "read"},
			}
			groups := BuildPermissionGroups(permissions)
			if len(groups) != 1 {
				t.Fatalf("expected 1 group, got %d", len(groups))
			}
			if groups[0].ResourceName != tt.wantName {
				t.Errorf("resource %q: expected ResourceName %q, got %q", tt.resource, tt.wantName, groups[0].ResourceName)
			}
		})
	}
}

func TestBuildPermissionGroups_NilInput(t *testing.T) {
	// nil 输入应返回空切片
	groups := BuildPermissionGroups(nil)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for nil input, got %d", len(groups))
	}
}

func TestBuildPermissionGroups_PermissionFieldsPreserved(t *testing.T) {
	// 验证权限字段在分组后完整保留
	original := &Permission{
		ID:          42,
		Resource:    "task",
		Action:      "create",
		Description: "创建任务权限",
		CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	groups := BuildPermissionGroups([]*Permission{original})

	if len(groups) != 1 || len(groups[0].Permissions) != 1 {
		t.Fatal("expected 1 group with 1 permission")
	}

	got := groups[0].Permissions[0]
	if got.ID != original.ID {
		t.Errorf("ID = %d, want %d", got.ID, original.ID)
	}
	if got.Resource != original.Resource {
		t.Errorf("Resource = %q, want %q", got.Resource, original.Resource)
	}
	if got.Action != original.Action {
		t.Errorf("Action = %q, want %q", got.Action, original.Action)
	}
	if got.Description != original.Description {
		t.Errorf("Description = %q, want %q", got.Description, original.Description)
	}
	if !got.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, original.CreatedAt)
	}
}

// === Role 方法边界场景测试 ===

func TestRole_IsSystemAdmin_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{"exact match", "system_admin", true},
		{"empty code", "", false},
		{"different case", "System_Admin", false},
		{"uppercase", "SYSTEM_ADMIN", false},
		{"with spaces", " system_admin ", false},
		{"prefix", "system_admin_extra", false},
		{"suffix", "prefix_system_admin", false},
		{"domain_admin", "domain_admin", false},
		{"user", "user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := &Role{Code: tt.code}
			if got := role.IsSystemAdmin(); got != tt.want {
				t.Errorf("IsSystemAdmin() with code %q = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestRole_IsDomainAdmin_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{"exact match", "domain_admin", true},
		{"empty code", "", false},
		{"different case", "Domain_Admin", false},
		{"system_admin", "system_admin", false},
		{"user", "user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := &Role{Code: tt.code}
			if got := role.IsDomainAdmin(); got != tt.want {
				t.Errorf("IsDomainAdmin() with code %q = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestRole_IsGlobal_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		domainID *int64
		want     bool
	}{
		{"nil domain ID", nil, true},
		{"zero domain ID", int64Ptr(0), false},
		{"positive domain ID", int64Ptr(1), false},
		{"negative domain ID", int64Ptr(-1), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := &Role{DomainID: tt.domainID}
			if got := role.IsGlobal(); got != tt.want {
				t.Errorf("IsGlobal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRole_GetCode_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{"normal code", "system_admin", "system_admin"},
		{"empty code", "", ""},
		{"code with spaces", "code with spaces", "code with spaces"},
		{"unicode code", "管理员", "管理员"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := &Role{Code: tt.code}
			if got := role.GetCode(); got != tt.want {
				t.Errorf("GetCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

// === Permission.GetCode 测试 ===

func TestPermission_GetCode_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		action   string
		want     string
	}{
		{"normal case", "task", "create", "task:create"},
		{"empty resource", "", "create", ":create"},
		{"empty action", "task", "", "task:"},
		{"both empty", "", "", ":"},
		{"with spaces", "task management", "create item", "task management:create item"},
		{"unicode", "任务", "创建", "任务:创建"},
		{"special chars", "task-1", "create_2", "task-1:create_2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Permission{Resource: tt.resource, Action: tt.action}
			if got := p.GetCode(); got != tt.want {
				t.Errorf("GetCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

// === TaskListFilter 零值语义测试 ===

func TestTaskListFilter_ZeroValueSemantics(t *testing.T) {
	// 零值字段表示不过滤
	filter := TaskListFilter{}

	if filter.DomainID != 0 {
		t.Errorf("zero DomainID should be 0, got %d", filter.DomainID)
	}
	if filter.CreatedBy != 0 {
		t.Errorf("zero CreatedBy should be 0, got %d", filter.CreatedBy)
	}
	if filter.Name != "" {
		t.Errorf("zero Name should be empty, got %q", filter.Name)
	}
	if filter.Type != "" {
		t.Errorf("zero Type should be empty, got %q", filter.Type)
	}
	if filter.IsEnabled != nil {
		t.Errorf("zero IsEnabled should be nil, got %v", filter.IsEnabled)
	}
	if filter.Page != 0 {
		t.Errorf("zero Page should be 0, got %d", filter.Page)
	}
	if filter.PageSize != 0 {
		t.Errorf("zero PageSize should be 0, got %d", filter.PageSize)
	}
}

func TestTaskListFilter_IsEnabledFilter(t *testing.T) {
	// 测试 IsEnabled 过滤器的三种状态
	enabled := true
	disabled := false

	tests := []struct {
		name      string
		isEnabled *bool
		wantNil   bool
		wantValue bool
	}{
		{"nil (no filter)", nil, true, false},
		{"true (enabled only)", &enabled, false, true},
		{"false (disabled only)", &disabled, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := TaskListFilter{IsEnabled: tt.isEnabled}
			if tt.wantNil {
				if filter.IsEnabled != nil {
					t.Errorf("expected nil IsEnabled, got %v", filter.IsEnabled)
				}
			} else {
				if filter.IsEnabled == nil {
					t.Fatal("expected non-nil IsEnabled")
				}
				if *filter.IsEnabled != tt.wantValue {
					t.Errorf("IsEnabled = %v, want %v", *filter.IsEnabled, tt.wantValue)
				}
			}
		})
	}
}

// === User 模型字段测试 ===

func TestUser_Model(t *testing.T) {
	t.Run("创建用户对象", func(t *testing.T) {
		user := &User{
			ID:        1,
			Username:  "testuser",
			RealName:  "测试用户",
			Email:     "test@example.com",
			IsActive:  true,
			RoleIDs:   []int64{1, 2},
			DomainIDs: []int64{1},
			RoleCodes: []string{"system_admin"},
		}

		if user.ID != 1 {
			t.Errorf("expected ID 1, got %d", user.ID)
		}
		if user.Username != "testuser" {
			t.Errorf("expected Username 'testuser', got %q", user.Username)
		}
		if !user.IsActive {
			t.Error("expected IsActive to be true")
		}
		if len(user.RoleIDs) != 2 {
			t.Errorf("expected 2 RoleIDs, got %d", len(user.RoleIDs))
		}
		if len(user.DomainIDs) != 1 {
			t.Errorf("expected 1 DomainID, got %d", len(user.DomainIDs))
		}
		if len(user.RoleCodes) != 1 {
			t.Errorf("expected 1 RoleCode, got %d", len(user.RoleCodes))
		}
	})

	t.Run("Password 字段 json tag 为 -", func(t *testing.T) {
		// 验证 Password 字段不会序列化到 JSON
		user := &User{
			ID:       1,
			Username: "test",
			Password: "secret",
		}

		data, err := json.Marshal(user)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		str := string(data)
		if strings.Contains(str, "secret") {
			t.Errorf("password should not be serialized, got: %s", str)
		}
		if strings.Contains(str, "\"password\"") {
			t.Errorf("password field should not appear in JSON, got: %s", str)
		}
	})
}

// === Domain 模型测试 ===

func TestDomain_Model(t *testing.T) {
	domain := &Domain{
		ID:          1,
		Name:        "production",
		Description: "生产环境",
	}

	if domain.ID != 1 {
		t.Errorf("expected ID 1, got %d", domain.ID)
	}
	if domain.Name != "production" {
		t.Errorf("expected Name 'production', got %q", domain.Name)
	}
	if domain.Description != "生产环境" {
		t.Errorf("expected Description '生产环境', got %q", domain.Description)
	}
}

// === Task 模型测试 ===

func TestTask_Model(t *testing.T) {
	t.Run("创建任务对象", func(t *testing.T) {
		task := &Task{
			ID:             1,
			Name:           "测试任务",
			Type:           "http",
			CronExpression: "*/5 * * * *",
			IsEnabled:      true,
			Status:         "pending",
			DomainID:       1,
			CreatedBy:      100,
		}

		if task.ID != 1 {
			t.Errorf("expected ID 1, got %d", task.ID)
		}
		if task.Type != "http" {
			t.Errorf("expected Type 'http', got %q", task.Type)
		}
		if !task.IsEnabled {
			t.Error("expected IsEnabled to be true")
		}
		if task.DomainID != 1 {
			t.Errorf("expected DomainID 1, got %d", task.DomainID)
		}
	})

	t.Run("任务带 Webhook", func(t *testing.T) {
		webhookID := int64(5)
		task := &Task{
			ID:              1,
			WebhookID:       &webhookID,
			WebhookEvents:   "success,failure",
		}

		if task.WebhookID == nil || *task.WebhookID != 5 {
			t.Error("expected WebhookID to be 5")
		}
		if task.WebhookEvents != "success,failure" {
			t.Errorf("expected WebhookEvents 'success,failure', got %q", task.WebhookEvents)
		}
	})

	t.Run("任务无 Webhook", func(t *testing.T) {
		task := &Task{
			ID:        1,
			WebhookID: nil,
		}

		if task.WebhookID != nil {
			t.Error("expected WebhookID to be nil")
		}
	})
}

// === Webhook 模型测试 ===

func TestWebhook_Model(t *testing.T) {
	t.Run("创建 Webhook 对象", func(t *testing.T) {
		webhook := &Webhook{
			ID:        1,
			Name:      "通知钩子",
			URL:       "https://example.com/webhook",
			Method:    "POST",
			IsEnabled: true,
			DomainID:  1,
		}

		if webhook.ID != 1 {
			t.Errorf("expected ID 1, got %d", webhook.ID)
		}
		if webhook.URL != "https://example.com/webhook" {
			t.Errorf("expected URL, got %q", webhook.URL)
		}
		if webhook.Method != "POST" {
			t.Errorf("expected Method 'POST', got %q", webhook.Method)
		}
		if !webhook.IsEnabled {
			t.Error("expected IsEnabled to be true")
		}
	})

	t.Run("Webhook 带 CreatedBy", func(t *testing.T) {
		createdBy := int64(10)
		webhook := &Webhook{
			ID:        1,
			CreatedBy: &createdBy,
		}

		if webhook.CreatedBy == nil || *webhook.CreatedBy != 10 {
			t.Error("expected CreatedBy to be 10")
		}
	})

	t.Run("Webhook Secret 字段 json omitempty", func(t *testing.T) {
		webhook := &Webhook{
			ID:     1,
			Name:   "test",
			Secret: "",
		}

		data, err := json.Marshal(webhook)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		str := string(data)
		if strings.Contains(str, "\"secret\"") {
			t.Errorf("empty secret should be omitted, got: %s", str)
		}
	})
}

// int64Ptr 辅助函数返回 int64 指针
func int64Ptr(v int64) *int64 {
	return &v
}
