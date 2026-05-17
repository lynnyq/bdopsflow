package model

import "time"

// UserRole 用户角色映射模型
type UserRole struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`    // 用户ID
	RoleID    int64     `json:"role_id"`    // 角色ID
	DomainID  *int64   `json:"domain_id"`  // 领域ID，NULL表示全局角色
	CreatedAt time.Time `json:"created_at"`

	// 关联数据
	Role *Role `json:"role,omitempty"`
}

// UserRoleRequest 分配用户角色的请求
type UserRoleRequest struct {
	RoleIDs   []int64 `json:"role_ids" binding:"required,min=1"`
	DomainIDs []int64 `json:"domain_ids"` // 可选，指定领域
}

// UserDomainRequest 分配用户领域的请求
type UserDomainRequest struct {
	DomainIDs []int64 `json:"domain_ids" binding:"required,min=1"`
}

// UserWithRoles 带有角色的用户信息
type UserWithRoles struct {
	User
	Roles []*UserRoleDetail `json:"bdopsflow_roles"`
}

// UserRoleDetail 用户角色详情
type UserRoleDetail struct {
	RoleID    int64   `json:"role_id"`
	RoleName  string  `json:"role_name"`
	RoleCode  string  `json:"role_code"`
	DomainID  *int64  `json:"domain_id"`
	DomainName string `json:"domain_name,omitempty"`
}
