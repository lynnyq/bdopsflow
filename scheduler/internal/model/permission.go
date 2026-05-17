package model

import "time"

// Permission 权限模型
type Permission struct {
	ID          int64     `json:"id"`
	Resource    string    `json:"resource"`    // 资源类型
	Action      string    `json:"action"`      // 操作类型
	Description string    `json:"description"` // 权限描述
	CreatedAt   time.Time `json:"created_at"`
}

// GetCode 获取权限代码
func (p *Permission) GetCode() string {
	return p.Resource + ":" + p.Action
}

// PermissionGroup 权限分组（用于前端展示）
type PermissionGroup struct {
	Resource     string       `json:"resource"`
	ResourceName string       `json:"resource_name"`
	Permissions  []Permission `json:"bdopsflow_permissions"`
}

// GetAllPermissionGroups 获取所有权限分组
func GetAllPermissionGroups() []PermissionGroup {
	return []PermissionGroup{
		{
			Resource:     "user",
			ResourceName: "用户管理",
			Permissions: []Permission{
				{Resource: "user", Action: "create", Description: "创建用户"},
				{Resource: "user", Action: "read", Description: "查看用户"},
				{Resource: "user", Action: "update", Description: "更新用户"},
				{Resource: "user", Action: "delete", Description: "删除用户"},
				{Resource: "user", Action: "manage", Description: "完整管理用户"},
			},
		},
		{
			Resource:     "role",
			ResourceName: "角色管理",
			Permissions: []Permission{
				{Resource: "role", Action: "create", Description: "创建角色"},
				{Resource: "role", Action: "read", Description: "查看角色"},
				{Resource: "role", Action: "update", Description: "更新角色"},
				{Resource: "role", Action: "delete", Description: "删除角色"},
				{Resource: "role", Action: "manage", Description: "完整管理角色"},
			},
		},
		{
			Resource:     "permission",
			ResourceName: "权限管理",
			Permissions: []Permission{
				{Resource: "permission", Action: "read", Description: "查看权限列表"},
			},
		},
		{
			Resource:     "domain",
			ResourceName: "领域管理",
			Permissions: []Permission{
				{Resource: "domain", Action: "create", Description: "创建领域"},
				{Resource: "domain", Action: "read", Description: "查看领域"},
				{Resource: "domain", Action: "update", Description: "更新领域"},
				{Resource: "domain", Action: "delete", Description: "删除领域"},
				{Resource: "domain", Action: "manage", Description: "完整管理领域"},
			},
		},
		{
			Resource:     "executor",
			ResourceName: "执行器管理",
			Permissions: []Permission{
				{Resource: "executor", Action: "read", Description: "查看执行器"},
				{Resource: "executor", Action: "assign", Description: "分配执行器"},
				{Resource: "executor", Action: "manage", Description: "完整管理执行器"},
			},
		},
		{
			Resource:     "task",
			ResourceName: "任务管理",
			Permissions: []Permission{
				{Resource: "task", Action: "create", Description: "创建任务"},
				{Resource: "task", Action: "read", Description: "查看任务"},
				{Resource: "task", Action: "update", Description: "更新任务"},
				{Resource: "task", Action: "delete", Description: "删除任务"},
				{Resource: "task", Action: "trigger", Description: "手动触发任务"},
				{Resource: "task", Action: "manage", Description: "完整管理任务"},
			},
		},
		{
			Resource:     "log",
			ResourceName: "日志管理",
			Permissions: []Permission{
				{Resource: "log", Action: "read", Description: "查看日志"},
				{Resource: "log", Action: "delete", Description: "删除日志"},
				{Resource: "log", Action: "manage", Description: "完整管理日志"},
			},
		},
		{
			Resource:     "workflow",
			ResourceName: "工作流管理",
			Permissions: []Permission{
				{Resource: "workflow", Action: "create", Description: "创建工作流"},
				{Resource: "workflow", Action: "read", Description: "查看工作流"},
				{Resource: "workflow", Action: "update", Description: "更新工作流"},
				{Resource: "workflow", Action: "delete", Description: "删除工作流"},
				{Resource: "workflow", Action: "manage", Description: "完整管理工作流"},
			},
		},
	}
}
