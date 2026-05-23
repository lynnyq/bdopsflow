package model

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50,regexp=^[a-zA-Z0-9_ ]+$"`
	RealName string `json:"real_name" binding:"max=50"`
	Phone    string `json:"phone" binding:"max=20"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=1,max=512"`
	Role     string `json:"role" binding:"required,oneof=system_admin domain_admin user"`
}

type UpdateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50,regexp=^[a-zA-Z0-9_ ]+$"`
	RealName string `json:"real_name" binding:"max=50"`
	Phone    string `json:"phone" binding:"max=20"`
	Email    string `json:"email" binding:"required,email"`
	Role     string `json:"role" binding:"required,oneof=system_admin domain_admin user"`
	IsActive bool   `json:"is_active"`
}

// UpdateCurrentUserRequest 更新当前用户信息请求（不包含密码）
type UpdateCurrentUserRequest struct {
	RealName string `json:"real_name" binding:"max=50"`
	Phone    string `json:"phone" binding:"max=20"`
	Email    string `json:"email" binding:"required,email"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=1,max=512"`
	NewPassword string `json:"new_password" binding:"required,min=1,max=512"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=1,max=512"`
}

// AdminUpdateUserRequest 管理员更新用户信息请求
type AdminUpdateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50,regexp=^[a-zA-Z0-9_ ]+$"`
	RealName string `json:"real_name" binding:"max=50"`
	Phone    string `json:"phone" binding:"max=20"`
	Email    string `json:"email" binding:"required,email"`
	Role     string `json:"role" binding:"required,oneof=system_admin domain_admin user"`
	IsActive bool   `json:"is_active"`
}

// CreateRoleRequest 创建角色请求
type CreateRoleRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Code        string `json:"code" binding:"required,min=2,max=50,alphanum"`
	Description string `json:"description" binding:"max=500"`
}

// UpdateRoleRequest 更新角色请求
type UpdateRoleRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Description string `json:"description" binding:"max=500"`
}

// AssignRolePermissionsRequest 分配角色权限请求
type AssignRolePermissionsRequest struct {
	Permissions []string `json:"bdopsflow_permissions" binding:"required,min=1"`
}

// AssignUserRolesRequest 分配用户角色请求
type AssignUserRolesRequest struct {
	RoleIDs []int64 `json:"role_ids" binding:"required,min=1"`
}

// CreateDomainRequest 创建领域请求
type CreateDomainRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Description string `json:"description" binding:"max=500"`
}

// UpdateDomainRequest 更新领域请求
type UpdateDomainRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Description string `json:"description" binding:"max=500"`
}
