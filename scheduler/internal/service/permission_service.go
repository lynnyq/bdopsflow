package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	"github.com/redis/go-redis/v9"
	rqlite "github.com/rqlite/gorqlite"
)

type PermissionService struct {
	db    database.DB
	cache *redis.Client
}

func NewPermissionService(db database.DB, cache *redis.Client) *PermissionService {
	return &PermissionService{
		db:    db,
		cache: cache,
	}
}

func (s *PermissionService) HasPermission(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error) {
	slog.Debug("HasPermission: checking", "module", "permission", "user_id", userID, "resource", resource, "action", action, "domain_id", domainID)
	isAdmin, err := s.IsSystemAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	if isAdmin {
		slog.Debug("HasPermission: system admin bypass", "module", "permission", "user_id", userID)
		return true, nil
	}

	roleIDs, err := s.collectRoleIDs(ctx, userID, domainID)
	if err != nil {
		return false, err
	}

	for _, roleID := range roleIDs {
		hasPerm, err := s.checkRolePermission(ctx, roleID, resource, action)
		if err != nil {
			return false, err
		}
		if hasPerm {
			slog.Debug("HasPermission: result", "module", "permission", "user_id", userID, "resource", resource, "action", action, "allowed", true)
			return true, nil
		}
	}

	slog.Debug("HasPermission: result", "module", "permission", "user_id", userID, "resource", resource, "action", action, "allowed", false)
	return false, nil
}

func (s *PermissionService) IsSystemAdmin(ctx context.Context, userID int64) (bool, error) {
	slog.Debug("IsSystemAdmin: checking", "module", "permission", "user_id", userID)
	query := `
		SELECT COUNT(*) FROM bdopsflow_roles r
		WHERE r.code = 'system_admin'
		AND r.id IN (
			SELECT ur.role_id FROM bdopsflow_user_roles ur WHERE ur.user_id = ?
		)
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
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
	slog.Debug("IsSystemAdmin: result", "module", "permission", "user_id", userID, "is_admin", count > 0)
	return count > 0, nil
}

func (s *PermissionService) GetUserRoles(ctx context.Context, userID int64) ([]*model.Role, error) {
	slog.Debug("GetUserRoles: fetching", "module", "permission", "user_id", userID)
	query := `
		SELECT DISTINCT r.id, r.name, r.code, r.description, r.is_system, r.parent_id, r.domain_id
		FROM bdopsflow_roles r
		WHERE r.id IN (
			SELECT ur.role_id FROM bdopsflow_user_roles ur WHERE ur.user_id = ?
		)
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
			parentID := rowInt64(row[5])
			if parentID > 0 {
				role.ParentID = &parentID
			}
		}

		if row[6] != nil {
			domainID := rowInt64(row[6])
			if domainID > 0 {
				role.DomainID = &domainID
			}
		}

		bdopsflow_roles = append(bdopsflow_roles, role)
	}

	return bdopsflow_roles, nil
}

func (s *PermissionService) GetUserPermissions(ctx context.Context, userID int64) ([]*model.Permission, error) {
	slog.Debug("GetUserPermissions: fetching", "module", "permission", "user_id", userID)
	roleIDs, err := s.getDirectRoleIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	allRoleIDs := s.expandRoleInheritance(ctx, roleIDs)
	slog.Debug("GetUserPermissions: expanded roles", "module", "permission", "user_id", userID, "direct_count", len(roleIDs), "total_count", len(allRoleIDs))

	if len(allRoleIDs) == 0 {
		return []*model.Permission{}, nil
	}

	permissions, err := s.getPermissionsByRoleIDs(ctx, allRoleIDs)
	if err != nil {
		return nil, err
	}
	slog.Debug("GetUserPermissions: result", "module", "permission", "user_id", userID, "permissions_count", len(permissions))
	return permissions, nil
}

func (s *PermissionService) GetUserDomains(ctx context.Context, userID int64) ([]*model.Domain, error) {
	isAdmin, err := s.IsSystemAdmin(ctx, userID)
	if err != nil {
		return nil, err
	}
	if isAdmin {
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

	query := `
		SELECT DISTINCT d.id, d.name, d.description
		FROM bdopsflow_user_domains ud
		JOIN bdopsflow_domains d ON ud.domain_id = d.id
		WHERE ud.user_id = ?
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

func (s *PermissionService) GetUserDomainInfos(ctx context.Context, userID int64) ([]*model.UserDomainInfo, error) {
	slog.Debug("GetUserDomainInfos: fetching", "module", "permission", "user_id", userID)
	query := `
		SELECT ud.domain_id, d.name, ud.is_default
		FROM bdopsflow_user_domains ud
		JOIN bdopsflow_domains d ON ud.domain_id = d.id
		WHERE ud.user_id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		slog.Error("GetUserDomainInfos: query failed", "error", err, "user_id", userID)
		return nil, err
	}
	if qr.Err != nil {
		slog.Error("GetUserDomainInfos: query result error", "error", qr.Err, "user_id", userID)
		return nil, qr.Err
	}

	var infos []*model.UserDomainInfo
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		info := &model.UserDomainInfo{
			DomainID:   rowInt64(row[0]),
			DomainName: rowString(row[1]),
			IsDefault:  rowBool(row[2]),
		}
		infos = append(infos, info)
	}

	slog.Debug("GetUserDomainInfos: result", "module", "permission", "user_id", userID, "domains_count", len(infos))
	return infos, nil
}

func (s *PermissionService) GetUserDefaultDomain(ctx context.Context, userID int64) (int64, error) {
	slog.Debug("GetUserDefaultDomain: fetching", "module", "permission", "user_id", userID)
	query := `
		SELECT ud.domain_id
		FROM bdopsflow_user_domains ud
		WHERE ud.user_id = ? AND ud.is_default = 1
		LIMIT 1
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		slog.Error("GetUserDefaultDomain: query failed", "error", err, "user_id", userID)
		return 0, err
	}
	if qr.Err != nil {
		slog.Error("GetUserDefaultDomain: query result error", "error", qr.Err, "user_id", userID)
		return 0, qr.Err
	}

	if !qr.Next() {
		query2 := `
			SELECT ud.domain_id
			FROM bdopsflow_user_domains ud
			WHERE ud.user_id = ?
			LIMIT 1
		`
		stmt2 := rqlite.ParameterizedStatement{
			Query:     query2,
			Arguments: []interface{}{userID},
		}
		qr2, err := s.db.QueryOneParameterized(stmt2)
		if err != nil {
			return 0, err
		}
		if qr2.Err != nil {
			return 0, qr2.Err
		}
		if !qr2.Next() {
			return 0, nil
		}
		row2, err := qr2.Slice()
		if err != nil {
			return 0, err
		}
		return rowInt64(row2[0]), nil
	}

	row, err := qr.Slice()
	if err != nil {
		return 0, err
	}
	domainID := rowInt64(row[0])
	slog.Debug("GetUserDefaultDomain: result", "module", "permission", "user_id", userID, "domain_id", domainID)
	return domainID, nil
}

func (s *PermissionService) SwitchDomain(ctx context.Context, userID int64, domainID int64) ([]*model.Permission, error) {
	slog.Info("SwitchDomain: switching", "module", "permission", "user_id", userID, "target_domain_id", domainID)
	query := `SELECT COUNT(*) FROM bdopsflow_user_domains WHERE user_id = ? AND domain_id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID, domainID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, ErrDomainAccessDenied
	}
	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}
	count := int(rowInt64(row[0]))
	if count == 0 {
		isAdmin, err := s.IsSystemAdmin(ctx, userID)
		if err != nil {
			return nil, err
		}
		if !isAdmin {
			slog.Warn("SwitchDomain: access denied", "module", "permission", "user_id", userID, "domain_id", domainID)
			return nil, ErrDomainAccessDenied
		}
	}

	roleIDs, err := s.collectRoleIDs(ctx, userID, domainID)
	if err != nil {
		return nil, err
	}

	allRoleIDs := s.expandRoleInheritance(ctx, roleIDs)

	if len(allRoleIDs) == 0 {
		slog.Info("SwitchDomain: success", "module", "permission", "user_id", userID, "domain_id", domainID, "permissions_count", 0)
		return []*model.Permission{}, nil
	}

	permissions, err := s.getPermissionsByRoleIDs(ctx, allRoleIDs)
	if err != nil {
		return nil, err
	}
	slog.Info("SwitchDomain: success", "module", "permission", "user_id", userID, "domain_id", domainID, "permissions_count", len(permissions))
	return permissions, nil
}

func (s *PermissionService) HasAnyPermission(ctx context.Context, userID int64, resource string, domainID int64) (bool, error) {
	slog.Debug("HasAnyPermission: checking", "module", "permission", "user_id", userID, "resource", resource, "domain_id", domainID)
	isAdmin, err := s.IsSystemAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	if isAdmin {
		return true, nil
	}

	roleIDs, err := s.collectRoleIDs(ctx, userID, domainID)
	if err != nil {
		return false, err
	}

	allRoleIDs := s.expandRoleInheritance(ctx, roleIDs)

	for _, roleID := range allRoleIDs {
		query := `
			SELECT COUNT(*) FROM bdopsflow_role_permissions rp
			JOIN bdopsflow_permissions p ON rp.permission_id = p.id
			WHERE rp.role_id = ? AND p.resource = ?
		`
		stmt := rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: []interface{}{roleID, resource},
		}
		qr, err := s.db.QueryOneParameterized(stmt)
		if err != nil {
			return false, err
		}
		if qr.Err != nil {
			return false, qr.Err
		}
		if qr.Next() {
			row, err := qr.Slice()
			if err != nil {
				continue
			}
			if rowInt64(row[0]) > 0 {
				slog.Debug("HasAnyPermission: result", "module", "permission", "user_id", userID, "resource", resource, "has", true)
				return true, nil
			}
		}
	}

	slog.Debug("HasAnyPermission: result", "module", "permission", "user_id", userID, "resource", resource, "has", false)
	return false, nil
}

func (s *PermissionService) IsDomainAdmin(ctx context.Context, userID int64, domainID int64) (bool, error) {
	slog.Debug("IsDomainAdmin: checking", "module", "permission", "user_id", userID, "domain_id", domainID)
	query := `
		SELECT COUNT(*) FROM bdopsflow_user_roles ur
		JOIN bdopsflow_roles r ON ur.role_id = r.id
		WHERE ur.user_id = ? AND r.code = 'domain_admin' AND (ur.domain_id = ? OR ur.domain_id IS NULL)
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID, domainID},
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

	return rowInt64(row[0]) > 0, nil
}

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

func (s *PermissionService) GetAllRoles(ctx context.Context) ([]*model.Role, error) {
	query := `
		SELECT id, name, code, description, is_system, parent_id, domain_id
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
			parentID := rowInt64(row[5])
			if parentID > 0 {
				role.ParentID = &parentID
			}
		}

		if row[6] != nil {
			domainID := rowInt64(row[6])
			if domainID > 0 {
				role.DomainID = &domainID
			}
		}

		bdopsflow_roles = append(bdopsflow_roles, role)
	}

	return bdopsflow_roles, nil
}

func (s *PermissionService) GetRoleByID(ctx context.Context, roleID int64) (*model.Role, error) {
	query := `
		SELECT id, name, code, description, is_system, parent_id, domain_id
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
		parentID := rowInt64(row[5])
		if parentID > 0 {
			role.ParentID = &parentID
		}
	}

	if row[6] != nil {
		domainID := rowInt64(row[6])
		if domainID > 0 {
			role.DomainID = &domainID
		}
	}

	return role, nil
}

func (s *PermissionService) GetRoleByCode(ctx context.Context, code string) (*model.Role, error) {
	query := `
		SELECT id, name, code, description, is_system, parent_id, domain_id
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
		parentID := rowInt64(row[5])
		if parentID > 0 {
			role.ParentID = &parentID
		}
	}

	if row[6] != nil {
		domainID := rowInt64(row[6])
		if domainID > 0 {
			role.DomainID = &domainID
		}
	}

	return role, nil
}

func (s *PermissionService) GetAllPermissions(ctx context.Context) ([]*model.Permission, error) {
	slog.Debug("GetAllPermissions: fetching", "module", "permission")
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

func (s *PermissionService) AssignPermissionsToRole(ctx context.Context, roleID int64, permissionIDs []int64) error {
	deleteQuery := `DELETE FROM bdopsflow_role_permissions WHERE role_id = ?`
	deleteStmt := rqlite.ParameterizedStatement{
		Query:     deleteQuery,
		Arguments: []interface{}{roleID},
	}
	_, err := s.db.WriteOneParameterized(deleteStmt)
	if err != nil {
		return err
	}

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

	slog.Info("AssignPermissionsToRole: permissions assigned", "module", "permission", "role_id", roleID, "permission_count", len(permissionIDs))
	return nil
}

func (s *PermissionService) getDirectRoleIDs(ctx context.Context, userID int64) ([]int64, error) {
	query := `SELECT role_id FROM bdopsflow_user_roles WHERE user_id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		slog.Error("getDirectRoleIDs: query failed", "error", err, "user_id", userID)
		return nil, err
	}
	if qr.Err != nil {
		slog.Error("getDirectRoleIDs: query result error", "error", qr.Err, "user_id", userID)
		return nil, qr.Err
	}

	var roleIDs []int64
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		roleIDs = append(roleIDs, rowInt64(row[0]))
	}

	slog.Debug("collectRoleIDs: result", "module", "permission", "user_id", userID, "role_count", len(roleIDs))
	return roleIDs, nil
}

func (s *PermissionService) GetRoleCodeByID(ctx context.Context, roleID int64) (string, error) {
	query := `SELECT code FROM bdopsflow_roles WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{roleID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return "", err
	}
	if qr.Err != nil {
		return "", qr.Err
	}
	if !qr.Next() {
		return "", fmt.Errorf("role not found")
	}
	row, err := qr.Slice()
	if err != nil {
		return "", err
	}
	return rowString(row[0]), nil
}

func (s *PermissionService) GetUserDomainIDs(ctx context.Context, userID int64) ([]int64, error) {
	query := `SELECT domain_id FROM bdopsflow_user_domains WHERE user_id = ?`
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

	var domainIDs []int64
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		domainIDs = append(domainIDs, rowInt64(row[0]))
	}
	return domainIDs, nil
}

func (s *PermissionService) expandRoleInheritance(ctx context.Context, directRoleIDs []int64) []int64 {
	visited := make(map[int64]bool)
	var allRoleIDs []int64

	var walk func(roleIDs []int64)
	walk = func(roleIDs []int64) {
		for _, id := range roleIDs {
			if visited[id] {
				slog.Warn("expandRoleInheritance: cycle detected", "module", "permission", "role_id", id)
				continue
			}
			visited[id] = true
			allRoleIDs = append(allRoleIDs, id)

			parentIDs := s.getParentRoleIDs(ctx, id)
			if len(parentIDs) > 0 {
				walk(parentIDs)
			}
		}
	}

	walk(directRoleIDs)
	slog.Debug("expandRoleInheritance: expanded", "module", "permission", "direct_count", len(directRoleIDs), "total_count", len(allRoleIDs))
	return allRoleIDs
}

func (s *PermissionService) getParentRoleIDs(ctx context.Context, roleID int64) []int64 {
	query := `SELECT parent_id FROM bdopsflow_roles WHERE id = ? AND parent_id IS NOT NULL AND parent_id > 0`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{roleID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil || qr.Err != nil {
		return nil
	}

	var parentIDs []int64
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		pid := rowInt64(row[0])
		if pid > 0 {
			parentIDs = append(parentIDs, pid)
		}
	}

	return parentIDs
}

func (s *PermissionService) getPermissionsByRoleIDs(ctx context.Context, roleIDs []int64) ([]*model.Permission, error) {
	if len(roleIDs) == 0 {
		return []*model.Permission{}, nil
	}

	var bdopsflow_permissions []*model.Permission
	seen := make(map[string]bool)

	for _, roleID := range roleIDs {
		perms, err := s.GetRolePermissions(ctx, roleID)
		if err != nil {
			continue
		}
		for _, p := range perms {
			key := p.GetCode()
			if !seen[key] {
				seen[key] = true
				bdopsflow_permissions = append(bdopsflow_permissions, p)
			}
		}
	}

	return bdopsflow_permissions, nil
}

func (s *PermissionService) GetUserRoleCodes(ctx context.Context, userID int64) ([]string, error) {
	slog.Debug("GetUserRoleCodes: fetching", "module", "permission", "user_id", userID)
	query := `
		SELECT DISTINCT r.code
		FROM bdopsflow_roles r
		WHERE r.id IN (
			SELECT ur.role_id FROM bdopsflow_user_roles ur WHERE ur.user_id = ?
		)
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

	var codes []string
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		codes = append(codes, rowString(row[0]))
	}

	slog.Debug("GetUserRoleCodes: result", "module", "permission", "user_id", userID, "role_codes", codes)
	return codes, nil
}

func (s *PermissionService) GetRolesByDomain(ctx context.Context, domainID int64) ([]*model.Role, error) {
	slog.Debug("GetRolesByDomain: fetching", "module", "permission", "domain_id", domainID)
	query := `
		SELECT id, name, code, description, is_system, parent_id, domain_id
		FROM bdopsflow_roles
		WHERE domain_id = ?
		ORDER BY id ASC
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{domainID},
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
			parentID := rowInt64(row[5])
			if parentID > 0 {
				role.ParentID = &parentID
			}
		}

		if row[6] != nil {
			did := rowInt64(row[6])
			if did > 0 {
				role.DomainID = &did
			}
		}

		bdopsflow_roles = append(bdopsflow_roles, role)
	}

	return bdopsflow_roles, nil
}

func (s *PermissionService) collectRoleIDs(ctx context.Context, userID int64, domainID int64) ([]int64, error) {
	slog.Debug("collectRoleIDs: collecting", "module", "permission", "user_id", userID, "domain_id", domainID)
	var roleIDs []int64

	globalQuery := `SELECT role_id FROM bdopsflow_user_roles WHERE user_id = ? AND domain_id IS NULL`
	globalStmt := rqlite.ParameterizedStatement{
		Query:     globalQuery,
		Arguments: []interface{}{userID},
	}
	qr, err := s.db.QueryOneParameterized(globalStmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		roleIDs = append(roleIDs, rowInt64(row[0]))
	}

	if domainID > 0 {
		domainQuery := `SELECT role_id FROM bdopsflow_user_roles WHERE user_id = ? AND domain_id = ?`
		domainStmt := rqlite.ParameterizedStatement{
			Query:     domainQuery,
			Arguments: []interface{}{userID, domainID},
		}
		qr2, err := s.db.QueryOneParameterized(domainStmt)
		if err != nil {
			return nil, err
		}
		if qr2.Err != nil {
			return nil, qr2.Err
		}
		for qr2.Next() {
			row, err := qr2.Slice()
			if err != nil {
				continue
			}
			roleIDs = append(roleIDs, rowInt64(row[0]))
		}
	}

	return roleIDs, nil
}
