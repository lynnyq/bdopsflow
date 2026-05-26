package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

type RoleAdminService struct {
	db      database.DB
	permSvc *PermissionService
}

func NewRoleAdminService(db database.DB, permSvc *PermissionService) *RoleAdminService {
	return &RoleAdminService{
		db:      db,
		permSvc: permSvc,
	}
}

func (s *RoleAdminService) IsSystemAdmin(ctx context.Context, userID int64) (bool, error) {
	return s.permSvc.IsSystemAdmin(ctx, userID)
}

func (s *RoleAdminService) ListRoles(ctx context.Context, domainID int64, isSystemAdmin bool) ([]*model.Role, error) {
	if isSystemAdmin {
		return s.permSvc.GetAllRoles(ctx)
	}
	return s.permSvc.GetRolesByDomain(ctx, domainID)
}

func (s *RoleAdminService) GetRole(ctx context.Context, roleID int64) (*model.Role, error) {
	return s.permSvc.GetRoleByID(ctx, roleID)
}

func (s *RoleAdminService) CreateRole(ctx context.Context, name, code, description string, parentID *int64, domainID *int64) (*model.Role, error) {
	slog.Info("CreateRole: creating", "module", "role_admin", "name", name, "code", code)
	query := `
		INSERT INTO bdopsflow_roles (name, code, description, is_system, parent_id, domain_id, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, ?, ?, ?)
	`

	now := time.Now()
	var parentIDValue interface{}
	if parentID != nil {
		parentIDValue = *parentID
	} else {
		parentIDValue = nil
	}

	var domainIDValue interface{}
	if domainID != nil {
		domainIDValue = *domainID
	} else {
		domainIDValue = nil
	}

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{name, code, description, parentIDValue, domainIDValue, now, now},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}

	roleID := result.LastInsertID
	slog.Info("CreateRole: success", "module", "role_admin", "role_id", roleID, "code", code)
	return s.permSvc.GetRoleByID(ctx, roleID)
}

func (s *RoleAdminService) UpdateRole(ctx context.Context, roleID int64, name, description string) (*model.Role, error) {
	slog.Info("UpdateRole: updating", "module", "role_admin", "role_id", roleID)
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

func (s *RoleAdminService) DeleteRole(ctx context.Context, roleID int64) error {
	slog.Info("DeleteRole: deleting", "module", "role_admin", "role_id", roleID)
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

	childCountQuery := `SELECT COUNT(*) FROM bdopsflow_roles WHERE parent_id = ?`
	childCountStmt := rqlite.ParameterizedStatement{
		Query:     childCountQuery,
		Arguments: []interface{}{roleID},
	}
	qr, err := s.db.QueryOneParameterized(childCountStmt)
	if err != nil {
		return err
	}
	if qr.Err != nil {
		return qr.Err
	}
	if qr.Next() {
		row, err := qr.Slice()
		if err == nil && rowInt64(row[0]) > 0 {
			slog.Warn("DeleteRole: has child roles", "module", "role_admin", "role_id", roleID)
			return ErrCannotDeleteRoleWithChildren
		}
	}

	deletePermQuery := `DELETE FROM bdopsflow_role_permissions WHERE role_id = ?`
	deletePermStmt := rqlite.ParameterizedStatement{
		Query:     deletePermQuery,
		Arguments: []interface{}{roleID},
	}
	_, err = s.db.WriteOneParameterized(deletePermStmt)
	if err != nil {
		return err
	}

	deleteUserQuery := `DELETE FROM bdopsflow_user_roles WHERE role_id = ?`
	deleteUserStmt := rqlite.ParameterizedStatement{
		Query:     deleteUserQuery,
		Arguments: []interface{}{roleID},
	}
	_, err = s.db.WriteOneParameterized(deleteUserStmt)
	if err != nil {
		return err
	}

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

func (s *RoleAdminService) GetRolePermissions(ctx context.Context, roleID int64) ([]*model.Permission, error) {
	return s.permSvc.GetRolePermissions(ctx, roleID)
}

func (s *RoleAdminService) AssignPermissionsToRole(ctx context.Context, roleID int64, permissionIDs []int64) error {
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

func (s *RoleAdminService) GetAllPermissions(ctx context.Context) ([]*model.Permission, error) {
	return s.permSvc.GetAllPermissions(ctx)
}
