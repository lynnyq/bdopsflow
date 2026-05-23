package service

import (
	"context"
	"fmt"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
	"github.com/redis/go-redis/v9"
)

// PermissionService 权限检查服务
type PermissionService struct {
	db    *rqlite.Connection
	cache *redis.Client
}

// NewPermissionService 创建权限服务
func NewPermissionService(db *rqlite.Connection, cache *redis.Client) *PermissionService {
	return &PermissionService{
		db:    db,
		cache: cache,
	}
}

// HasPermission 检查用户是否有指定权限
func (s *PermissionService) HasPermission(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error) {
	// 1. 检查是否为系统管理员
	isAdmin, err := s.IsSystemAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	if isAdmin {
		return true, nil
	}

	// 2. 获取用户角色
	bdopsflow_roles, err := s.GetUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}

	// 3. 检查是否有该资源权限
	for _, role := range bdopsflow_roles {
		hasPerm, err := s.checkRolePermission(ctx, role.ID, resource, action)
		if err != nil {
			return false, err
		}
		if hasPerm {
			// 4. 检查是否有该领域访问权限
			if s.canAccessDomain(role, domainID) {
				return true, nil
			}
		}
	}

	return false, nil
}

// IsSystemAdmin 检查用户是否为系统管理员
func (s *PermissionService) IsSystemAdmin(ctx context.Context, userID int64) (bool, error) {
	query := `
		SELECT COUNT(*) FROM bdopsflow_roles r
		WHERE r.code = 'system_admin'
		AND r.id IN (
			SELECT ur.role_id FROM bdopsflow_user_roles ur WHERE ur.user_id = ?
			UNION
			SELECT r2.id FROM bdopsflow_roles r2
			JOIN bdopsflow_users u ON u.role = r2.code
			WHERE u.id = ?
		)
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID, userID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return false, err
	}
	if qr.Err != nil {
		return false, qr.Err
	}

	if !qr.Next() {
		return false, nil
	}
	row, err := qr.Slice()
	if err != nil {
		return false, err
	}

	count := int(rowInt64(row[0]))
	return count > 0, nil
}

// GetUserRoles 获取用户的所有角色
func (s *PermissionService) GetUserRoles(ctx context.Context, userID int64) ([]*model.Role, error) {
	query := `
		SELECT DISTINCT r.id, r.name, r.code, r.description, r.is_system, r.domain_id
		FROM bdopsflow_roles r
		WHERE r.id IN (
			SELECT ur.role_id FROM bdopsflow_user_roles ur WHERE ur.user_id = ?
			UNION
			SELECT r2.id FROM bdopsflow_roles r2
			JOIN bdopsflow_users u ON u.role = r2.code
			WHERE u.id = ?
		)
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID, userID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_roles []*model.Role
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		role := &model.Role{
			ID:          rowInt64(row[0]),
			Name:        rowString(row[1]),
			Code:        rowString(row[2]),
			Description: rowString(row[3]),
			IsSystem:    rowBool(row[4]),
		}

		if row[5] != nil {
			domainID := rowInt64(row[5])
			role.DomainID = &domainID
		}

		bdopsflow_roles = append(bdopsflow_roles, role)
	}

	return bdopsflow_roles, nil
}

// GetUserPermissions 获取用户的所有权限
func (s *PermissionService) GetUserPermissions(ctx context.Context, userID int64) ([]*model.Permission, error) {
	query := `
		SELECT DISTINCT p.id, p.resource, p.action, p.description
		FROM bdopsflow_roles r
		JOIN bdopsflow_role_permissions rp ON r.id = rp.role_id
		JOIN bdopsflow_permissions p ON rp.permission_id = p.id
		WHERE r.id IN (
			SELECT ur.role_id FROM bdopsflow_user_roles ur WHERE ur.user_id = ?
			UNION
			SELECT r2.id FROM bdopsflow_roles r2
			JOIN bdopsflow_users u ON u.role = r2.code
			WHERE u.id = ?
		)
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID, userID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_permissions []*model.Permission
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		perm := &model.Permission{
			ID:          rowInt64(row[0]),
			Resource:    rowString(row[1]),
			Action:      rowString(row[2]),
			Description: rowString(row[3]),
		}
		bdopsflow_permissions = append(bdopsflow_permissions, perm)
	}

	return bdopsflow_permissions, nil
}

// GetUserDomains 获取用户可访问的所有领域
func (s *PermissionService) GetUserDomains(ctx context.Context, userID int64) ([]*model.Domain, error) {
	// 先检查是否为系统管理员
	isAdmin, err := s.IsSystemAdmin(ctx, userID)
	if err != nil {
		return nil, err
	}
	if isAdmin {
		// 系统管理员可访问所有领域
		query := `SELECT id, name, description FROM bdopsflow_domains`
		qr, err := s.db.QueryOne(query)
		if err != nil {
			return nil, err
		}
		if qr.Err != nil {
			return nil, qr.Err
		}

		var bdopsflow_domains []*model.Domain
		for qr.Next() {
			row, err := qr.Slice()
			if err != nil {
				continue
			}

			domain := &model.Domain{
				ID:          rowInt64(row[0]),
				Name:        rowString(row[1]),
				Description: rowString(row[2]),
			}
			bdopsflow_domains = append(bdopsflow_domains, domain)
		}

		return bdopsflow_domains, nil
	}

	// 普通用户只访问关联的领域
	query := `
		SELECT DISTINCT d.id, d.name, d.description
		FROM bdopsflow_user_roles ur
		JOIN bdopsflow_domains d ON ur.domain_id = d.id
		WHERE ur.user_id = ? AND ur.domain_id IS NOT NULL
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_domains []*model.Domain
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		domain := &model.Domain{
			ID:          rowInt64(row[0]),
			Name:        rowString(row[1]),
			Description: rowString(row[2]),
		}
		bdopsflow_domains = append(bdopsflow_domains, domain)
	}

	return bdopsflow_domains, nil
}

// checkRolePermission 检查角色是否有指定权限
func (s *PermissionService) checkRolePermission(ctx context.Context, roleID int64, resource, action string) (bool, error) {
	query := `
		SELECT COUNT(*) FROM bdopsflow_role_permissions rp
		JOIN bdopsflow_permissions p ON rp.permission_id = p.id
		WHERE rp.role_id = ? 
		AND (
			(p.resource = ? AND p.action = ?)
			OR (p.resource = ? AND p.action = 'manage')
		)
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{roleID, resource, action, resource},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return false, err
	}
	if qr.Err != nil {
		return false, qr.Err
	}

	if !qr.Next() {
		return false, nil
	}
	row, err := qr.Slice()
	if err != nil {
		return false, err
	}

	count := int(rowInt64(row[0]))
	return count > 0, nil
}

// canAccessDomain 检查角色是否可以访问指定领域
func (s *PermissionService) canAccessDomain(role *model.Role, domainID int64) bool {
	// 全局角色可访问所有领域
	if role.DomainID == nil {
		return true
	}
	// 领域角色只能访问所属领域
	return *role.DomainID == domainID
}

// InvalidateUserPermissionCache 清除用户权限缓存
func (s *PermissionService) InvalidateUserPermissionCache(ctx context.Context, userID int64) error {
	if s.cache == nil {
		return nil
	}

	pattern := fmt.Sprintf("perm:%d:*", userID)
	iter := s.cache.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := s.cache.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

// GetAllRoles 获取所有角色
func (s *PermissionService) GetAllRoles(ctx context.Context) ([]*model.Role, error) {
	query := `
		SELECT id, name, code, description, is_system, domain_id
		FROM bdopsflow_roles
		ORDER BY is_system DESC, id ASC
	`

	qr, err := s.db.QueryOne(query)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_roles []*model.Role
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		role := &model.Role{
			ID:          rowInt64(row[0]),
			Name:        rowString(row[1]),
			Code:        rowString(row[2]),
			Description: rowString(row[3]),
			IsSystem:    rowBool(row[4]),
		}

		if row[5] != nil {
			domainID := rowInt64(row[5])
			role.DomainID = &domainID
		}

		bdopsflow_roles = append(bdopsflow_roles, role)
	}

	return bdopsflow_roles, nil
}

// GetRoleByID 根据ID获取角色
func (s *PermissionService) GetRoleByID(ctx context.Context, roleID int64) (*model.Role, error) {
	query := `
		SELECT id, name, code, description, is_system, domain_id
		FROM bdopsflow_roles
		WHERE id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{roleID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, nil
	}

	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	role := &model.Role{
		ID:          rowInt64(row[0]),
		Name:        rowString(row[1]),
		Code:        rowString(row[2]),
		Description: rowString(row[3]),
		IsSystem:    rowBool(row[4]),
	}

	if row[5] != nil {
		domainID := rowInt64(row[5])
		role.DomainID = &domainID
	}

	return role, nil
}

// GetRoleByCode 根据代码获取角色
func (s *PermissionService) GetRoleByCode(ctx context.Context, code string) (*model.Role, error) {
	query := `
		SELECT id, name, code, description, is_system, domain_id
		FROM bdopsflow_roles
		WHERE code = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{code},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, nil
	}

	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	role := &model.Role{
		ID:          rowInt64(row[0]),
		Name:        rowString(row[1]),
		Code:        rowString(row[2]),
		Description: rowString(row[3]),
		IsSystem:    rowBool(row[4]),
	}

	if row[5] != nil {
		domainID := rowInt64(row[5])
		role.DomainID = &domainID
	}

	return role, nil
}

// GetAllPermissions 获取所有权限
func (s *PermissionService) GetAllPermissions(ctx context.Context) ([]*model.Permission, error) {
	query := `
		SELECT id, resource, action, description
		FROM bdopsflow_permissions
		ORDER BY resource, action
	`

	qr, err := s.db.QueryOne(query)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_permissions []*model.Permission
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		perm := &model.Permission{
			ID:          rowInt64(row[0]),
			Resource:    rowString(row[1]),
			Action:      rowString(row[2]),
			Description: rowString(row[3]),
		}
		bdopsflow_permissions = append(bdopsflow_permissions, perm)
	}

	return bdopsflow_permissions, nil
}

// GetRolePermissions 获取角色的权限
func (s *PermissionService) GetRolePermissions(ctx context.Context, roleID int64) ([]*model.Permission, error) {
	query := `
		SELECT p.id, p.resource, p.action, p.description
		FROM bdopsflow_role_permissions rp
		JOIN bdopsflow_permissions p ON rp.permission_id = p.id
		WHERE rp.role_id = ?
		ORDER BY p.resource, p.action
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{roleID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_permissions []*model.Permission
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		perm := &model.Permission{
			ID:          rowInt64(row[0]),
			Resource:    rowString(row[1]),
			Action:      rowString(row[2]),
			Description: rowString(row[3]),
		}
		bdopsflow_permissions = append(bdopsflow_permissions, perm)
	}

	return bdopsflow_permissions, nil
}

// AssignPermissionsToRole 为角色分配权限
func (s *PermissionService) AssignPermissionsToRole(ctx context.Context, roleID int64, permissionIDs []int64) error {
	// 删除旧权限
	deleteQuery := `DELETE FROM bdopsflow_role_permissions WHERE role_id = ?`
	deleteStmt := rqlite.ParameterizedStatement{
		Query:     deleteQuery,
		Arguments: []interface{}{roleID},
	}
	_, err := s.db.WriteOneParameterized(deleteStmt)
	if err != nil {
		return err
	}

	// 添加新权限
	if len(permissionIDs) > 0 {
		var statements []rqlite.ParameterizedStatement
		now := time.Now()

		for _, permID := range permissionIDs {
			insertQuery := `INSERT INTO bdopsflow_role_permissions (role_id, permission_id, created_at) VALUES (?, ?, ?)`
			stmt := rqlite.ParameterizedStatement{
				Query:     insertQuery,
				Arguments: []interface{}{roleID, permID, now},
			}
			statements = append(statements, stmt)
		}

		_, err := s.db.WriteParameterized(statements)
		if err != nil {
			return err
		}
	}

	return nil
}
