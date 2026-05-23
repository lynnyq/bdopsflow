package model

import "time"

// Role 角色模型
type Role struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`        // 角色名称
	Code        string     `json:"code"`        // 角色代码（唯一）
	Description string     `json:"description"` // 角色描述
	IsSystem    bool       `json:"is_system"`   // 是否系统预设角色
	DomainID    *int64     `json:"domain_id"`   // 领域专属角色，NULL表示全局角色
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// 关联数据
	Permissions []*Permission `json:"permissions,omitempty"` // 角色拥有的权限
}

// IsGlobal 检查是否为全局角色
func (r *Role) IsGlobal() bool {
	return r.DomainID == nil
}

// IsSystemAdmin 检查是否为系统管理员
func (r *Role) IsSystemAdmin() bool {
	return r.Code == "system_admin"
}

// GetCode 获取权限代码（用于缓存等）
func (r *Role) GetCode() string {
	return r.Code
}

// RoleRequest 创建/更新角色的请求
type RoleRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Code        string `json:"code" binding:"required,min=2,max=50,alphanum"`
	Description string `json:"description" binding:"max=500"`
	DomainID    *int64 `json:"domain_id"`
}

// RolePermissionRequest 配置角色权限的请求
type RolePermissionRequest struct {
	PermissionIDs []int64 `json:"permission_ids" binding:"required,min=1"`
}
