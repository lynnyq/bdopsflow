package service

import (
	"context"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

// RoleAdminService 角色管理服务
type RoleAdminService struct {
	db      *rqlite.Connection
	permSvc *PermissionService
}

// NewRoleAdminService 创建角色管理服务
func NewRoleAdminService(db *rqlite.Connection, permSvc *PermissionService) *RoleAdminService {
	return &RoleAdminService{
		db:      db,
		permSvc: permSvc,
	}
}

// ListRoles 获取角色列表
func (s *RoleAdminService) ListRoles(ctx context.Context) ([]*model.Role, error) {
	return s.permSvc.GetAllRoles(ctx)
}

// GetRole 获取角色详情
func (s *RoleAdminService) GetRole(ctx context.Context, roleID int64) (*model.Role, error) {
	return s.permSvc.GetRoleByID(ctx, roleID)
}

// CreateRole 创建角色
func (s *RoleAdminService) CreateRole(ctx context.Context, name, code, description string, domainID *int64) (*model.Role, error) {
	query := `
		INSERT INTO bdopsflow_roles (name, code, description, is_system, domain_id, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, ?, ?)
	`

	now := time.Now()
	var domainIDValue interface{}
	if domainID != nil {
		domainIDValue = *domainID
	} else {
		domainIDValue = nil
	}

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{name, code, description, domainIDValue, now, now},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}

	roleID := result.LastInsertID
	return s.permSvc.GetRoleByID(ctx, roleID)
}

// UpdateRole 更新角色
func (s *RoleAdminService) UpdateRole(ctx context.Context, roleID int64, name, description string) (*model.Role, error) {
	// 检查是否为系统角色
	role, err := s.permSvc.GetRoleByID(ctx, roleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	if role.IsSystem {
		return nil, ErrSystemRoleCannotModify
	}

	query := `
		UPDATE bdopsflow_roles
		SET name = ?, description = ?, updated_at = ?
		WHERE id = ? AND is_system = 0
	`

	now := time.Now()
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{name, description, now, roleID},
	}
	_, err = s.db.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}

	return s.permSvc.GetRoleByID(ctx, roleID)
}

// DeleteRole 删除角色
func (s *RoleAdminService) DeleteRole(ctx context.Context, roleID int64) error {
	// 检查是否为系统角色
	role, err := s.permSvc.GetRoleByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role == nil {
		return ErrRoleNotFound
	}
	if role.IsSystem {
		return ErrSystemRoleCannotDelete
	}

	// 删除角色权限关联
	deletePermQuery := `DELETE FROM bdopsflow_role_permissions WHERE role_id = ?`
	deletePermStmt := rqlite.ParameterizedStatement{
		Query:     deletePermQuery,
		Arguments: []interface{}{roleID},
	}
	_, err = s.db.WriteOneParameterized(deletePermStmt)
	if err != nil {
		return err
	}

	// 删除用户角色关联
	deleteUserQuery := `DELETE FROM bdopsflow_user_roles WHERE role_id = ?`
	deleteUserStmt := rqlite.ParameterizedStatement{
		Query:     deleteUserQuery,
		Arguments: []interface{}{roleID},
	}
	_, err = s.db.WriteOneParameterized(deleteUserStmt)
	if err != nil {
		return err
	}

	// 删除角色
	deleteRoleQuery := `DELETE FROM bdopsflow_roles WHERE id = ? AND is_system = 0`
	deleteRoleStmt := rqlite.ParameterizedStatement{
		Query:     deleteRoleQuery,
		Arguments: []interface{}{roleID},
	}
	_, err = s.db.WriteOneParameterized(deleteRoleStmt)
	if err != nil {
		return err
	}

	return nil
}

// GetRolePermissions 获取角色权限
func (s *RoleAdminService) GetRolePermissions(ctx context.Context, roleID int64) ([]*model.Permission, error) {
	return s.permSvc.GetRolePermissions(ctx, roleID)
}

// AssignPermissionsToRole 分配权限给角色
func (s *RoleAdminService) AssignPermissionsToRole(ctx context.Context, roleID int64, permissionIDs []int64) error {
	// 检查是否为系统角色
	role, err := s.permSvc.GetRoleByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role == nil {
		return ErrRoleNotFound
	}
	if role.IsSystem {
		return ErrSystemRoleCannotModify
	}

	return s.permSvc.AssignPermissionsToRole(ctx, roleID, permissionIDs)
}

// GetAllPermissions 获取所有权限
func (s *RoleAdminService) GetAllPermissions(ctx context.Context) ([]*model.Permission, error) {
	return s.permSvc.GetAllPermissions(ctx)
}
