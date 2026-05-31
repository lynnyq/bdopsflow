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
	Permissions  []Permission `json:"permissions"`
}

var resourceNameMap = map[string]string{
	"dashboard":  "仪表盘",
	"user":       "用户管理",
	"role":       "角色管理",
	"permission": "权限管理",
	"domain":     "领域管理",
	"executor":   "执行器管理",
	"task":       "任务管理",
	"log":        "日志管理",
	"datasource": "数据源管理",
	"webhook":    "Webhook管理",
	"audit_log":  "审计日志",
	"config":     "系统配置",
}

var resourceOrder = []string{
	"dashboard", "user", "role", "permission", "domain", "executor",
	"task", "log", "datasource", "webhook", "audit_log", "config",
}

// BuildPermissionGroups 从数据库权限列表构建分组
func BuildPermissionGroups(permissions []*Permission) []PermissionGroup {
	groupMap := make(map[string][]Permission)
	for _, p := range permissions {
		groupMap[p.Resource] = append(groupMap[p.Resource], *p)
	}

	var groups []PermissionGroup
	for _, resource := range resourceOrder {
		perms, ok := groupMap[resource]
		if !ok {
			continue
		}
		name, exists := resourceNameMap[resource]
		if !exists {
			name = resource
		}
		groups = append(groups, PermissionGroup{
			Resource:     resource,
			ResourceName: name,
			Permissions:  perms,
		})
	}

	for resource, perms := range groupMap {
		found := false
		for _, r := range resourceOrder {
			if r == resource {
				found = true
				break
			}
		}
		if !found {
			name, exists := resourceNameMap[resource]
			if !exists {
				name = resource
			}
			groups = append(groups, PermissionGroup{
				Resource:     resource,
				ResourceName: name,
				Permissions:  perms,
			})
		}
	}

	return groups
}
