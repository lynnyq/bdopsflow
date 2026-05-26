package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

type InstancePermissionService struct {
	db      database.DB
	permSvc *PermissionService
}

func NewInstancePermissionService(db database.DB, permSvc *PermissionService) *InstancePermissionService {
	return &InstancePermissionService{
		db:      db,
		permSvc: permSvc,
	}
}

var webhookPermissionIncludes = map[string][]string{
	"read":    {},
	"trigger": {"read"},
	"update":  {"read"},
	"delete":  {},
	"manage":  {"update", "read", "delete", "trigger"},
}

func getWebhookEffectivePermissions(permType string) []string {
	var result []string
	for pt, includes := range webhookPermissionIncludes {
		if pt == permType {
			result = append(result, pt)
			continue
		}
		for _, inc := range includes {
			if inc == permType {
				result = append(result, pt)
				break
			}
		}
	}
	if len(result) == 0 {
		return []string{permType}
	}
	return result
}

func (s *InstancePermissionService) HasDatasourcePermission(ctx context.Context, userID int64, datasourceID int64, permissionType string) (bool, error) {
	slog.Debug("HasDatasourcePermission: checking", "module", "instance_perm", "user_id", userID, "datasource_id", datasourceID, "permission_type", permissionType)
	isAdmin, err := s.permSvc.IsSystemAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	if isAdmin {
		slog.Debug("HasDatasourcePermission: system admin bypass", "module", "instance_perm", "user_id", userID)
		return true, nil
	}

	var domainID int64
	domainQuery := `SELECT domain_id FROM bdopsflow_datasources WHERE id = ?`
	domainStmt := rqlite.ParameterizedStatement{
		Query:     domainQuery,
		Arguments: []interface{}{datasourceID},
	}
	qr, err := s.db.QueryOneParameterized(domainStmt)
	if err != nil {
		return false, err
	}
	if qr.Err != nil {
		return false, qr.Err
	}
	if !qr.Next() {
		return false, ErrInstancePermissionDenied
	}
	row, err := qr.Slice()
	if err != nil {
		return false, err
	}
	domainID = rowInt64(row[0])

	isDomainAdmin, err := s.permSvc.IsDomainAdmin(ctx, userID, domainID)
	if err != nil {
		return false, err
	}
	if isDomainAdmin {
		if permissionType == "read" || permissionType == "query" {
			slog.Debug("HasDatasourcePermission: domain admin bypass for read/query", "module", "instance_perm", "user_id", userID, "permission_type", permissionType)
			return true, nil
		}
	}

	creatorQuery := `SELECT created_by FROM bdopsflow_datasources WHERE id = ?`
	creatorStmt := rqlite.ParameterizedStatement{
		Query:     creatorQuery,
		Arguments: []interface{}{datasourceID},
	}
	qr2, err := s.db.QueryOneParameterized(creatorStmt)
	if err != nil {
		return false, err
	}
	if qr2.Err != nil {
		return false, qr2.Err
	}
	if qr2.Next() {
		row2, err := qr2.Slice()
		if err == nil {
			createdBy := rowInt64(row2[0])
			if createdBy == userID {
				slog.Debug("HasDatasourcePermission: creator bypass", "module", "instance_perm", "user_id", userID)
				return true, nil
			}
		}
	}

	effectivePerms := datasource.GetEffectivePermissions(permissionType)
	permPlaceholders := ""
	permArgs := make([]interface{}, 0, len(effectivePerms))
	for i, pt := range effectivePerms {
		if i > 0 {
			permPlaceholders += ", "
		}
		permPlaceholders += "?"
		permArgs = append(permArgs, pt)
	}
	permQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM bdopsflow_datasource_permissions
		WHERE datasource_id = ?
		AND (
			(user_id = ?)
			OR (role_id IN (SELECT ur.role_id FROM bdopsflow_user_roles ur WHERE ur.user_id = ?))
		)
		AND permission_type IN (%s)
	`, permPlaceholders)
	args := []interface{}{datasourceID, userID, userID}
	args = append(args, permArgs...)
	permStmt := rqlite.ParameterizedStatement{
		Query:     permQuery,
		Arguments: args,
	}
	qr3, err := s.db.QueryOneParameterized(permStmt)
	if err != nil {
		return false, err
	}
	if qr3.Err != nil {
		return false, qr3.Err
	}
	if qr3.Next() {
		row3, err := qr3.Slice()
		if err == nil && rowInt64(row3[0]) > 0 {
			slog.Debug("HasDatasourcePermission: result", "module", "instance_perm", "user_id", userID, "datasource_id", datasourceID, "allowed", true)
			return true, nil
		}
	}

	slog.Debug("HasDatasourcePermission: result", "module", "instance_perm", "user_id", userID, "datasource_id", datasourceID, "allowed", false)
	return false, nil
}

func (s *InstancePermissionService) GetUserDatasourceIDs(ctx context.Context, userID int64, permissionType string) ([]int64, error) {
	effectivePerms := datasource.GetEffectivePermissions(permissionType)
	permPlaceholders := ""
	permArgs := make([]interface{}, 0, len(effectivePerms))
	for i, pt := range effectivePerms {
		if i > 0 {
			permPlaceholders += ", "
		}
		permPlaceholders += "?"
		permArgs = append(permArgs, pt)
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT datasource_id FROM bdopsflow_datasource_permissions
		WHERE (user_id = ? OR role_id IN (SELECT ur.role_id FROM bdopsflow_user_roles ur WHERE ur.user_id = ?))
		AND permission_type IN (%s)
	`, permPlaceholders)
	args := []interface{}{userID, userID}
	args = append(args, permArgs...)

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: args,
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var ids []int64
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		if id := rowInt64(row[0]); id > 0 {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

var permissionWeight = map[string]int{
	"manage":   100,
	"update":   50,
	"download": 40,
	"query":    30,
	"read":     20,
	"delete":   10,
}

func highestPermission(a, b string) string {
	wa, okA := permissionWeight[a]
	wb, okB := permissionWeight[b]
	if !okA {
		return b
	}
	if !okB {
		return a
	}
	if wa >= wb {
		return a
	}
	return b
}

func (s *InstancePermissionService) GetUserDatasourcePermissionLevels(ctx context.Context, userID int64, datasourceIDs []int64) (map[int64]string, error) {
	result := make(map[int64]string)
	if len(datasourceIDs) == 0 {
		return result, nil
	}

	dsPlaceholders := ""
	dsArgs := make([]interface{}, 0, len(datasourceIDs))
	for i, id := range datasourceIDs {
		if i > 0 {
			dsPlaceholders += ", "
		}
		dsPlaceholders += "?"
		dsArgs = append(dsArgs, id)
	}

	query := fmt.Sprintf(`
		SELECT datasource_id, permission_type FROM bdopsflow_datasource_permissions
		WHERE datasource_id IN (%s)
		AND (user_id = ? OR role_id IN (SELECT ur.role_id FROM bdopsflow_user_roles ur WHERE ur.user_id = ?))
	`, dsPlaceholders)
	args := append(dsArgs, userID, userID)

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: args,
	}
	qr, err := s.db.QueryOneParameterized(stmt)
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
		dsID := rowInt64(row[0])
		permType := rowString(row[1])
		if dsID > 0 && permType != "" {
			if existing, ok := result[dsID]; ok {
				result[dsID] = highestPermission(existing, permType)
			} else {
				result[dsID] = permType
			}
		}
	}
	return result, nil
}

func (s *InstancePermissionService) HasWebhookPermission(ctx context.Context, userID int64, webhookID int64, permissionType string) (bool, error) {
	slog.Debug("HasWebhookPermission: checking", "module", "instance_perm", "user_id", userID, "webhook_id", webhookID, "permission_type", permissionType)
	isAdmin, err := s.permSvc.IsSystemAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	if isAdmin {
		slog.Debug("HasWebhookPermission: system admin bypass", "module", "instance_perm", "user_id", userID)
		return true, nil
	}

	var domainID int64
	domainQuery := `SELECT domain_id FROM bdopsflow_webhooks WHERE id = ?`
	domainStmt := rqlite.ParameterizedStatement{
		Query:     domainQuery,
		Arguments: []interface{}{webhookID},
	}
	qr, err := s.db.QueryOneParameterized(domainStmt)
	if err != nil {
		return false, err
	}
	if qr.Err != nil {
		return false, qr.Err
	}
	if !qr.Next() {
		return false, ErrInstancePermissionDenied
	}
	row, err := qr.Slice()
	if err != nil {
		return false, err
	}
	domainID = rowInt64(row[0])

	isDomainAdmin, err := s.permSvc.IsDomainAdmin(ctx, userID, domainID)
	if err != nil {
		return false, err
	}
	if isDomainAdmin {
		if permissionType == "read" {
			slog.Debug("HasWebhookPermission: domain admin bypass for read", "module", "instance_perm", "user_id", userID, "permission_type", permissionType)
			return true, nil
		}
	}

	creatorQuery := `SELECT created_by FROM bdopsflow_webhooks WHERE id = ?`
	creatorStmt := rqlite.ParameterizedStatement{
		Query:     creatorQuery,
		Arguments: []interface{}{webhookID},
	}
	qr2, err := s.db.QueryOneParameterized(creatorStmt)
	if err != nil {
		return false, err
	}
	if qr2.Err != nil {
		return false, qr2.Err
	}
	if qr2.Next() {
		row2, err := qr2.Slice()
		if err == nil {
			createdBy := rowInt64(row2[0])
			if createdBy == userID {
				slog.Debug("HasWebhookPermission: creator bypass", "module", "instance_perm", "user_id", userID)
				return true, nil
			}
		}
	}

	effectivePerms := getWebhookEffectivePermissions(permissionType)
	permPlaceholders := ""
	permArgs := make([]interface{}, 0, len(effectivePerms))
	for i, pt := range effectivePerms {
		if i > 0 {
			permPlaceholders += ", "
		}
		permPlaceholders += "?"
		permArgs = append(permArgs, pt)
	}
	permQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM bdopsflow_webhook_permissions
		WHERE webhook_id = ?
		AND (
			(user_id = ?)
			OR (role_id IN (SELECT ur.role_id FROM bdopsflow_user_roles ur WHERE ur.user_id = ?))
		)
		AND permission_type IN (%s)
	`, permPlaceholders)
	args := []interface{}{webhookID, userID, userID}
	args = append(args, permArgs...)
	permStmt := rqlite.ParameterizedStatement{
		Query:     permQuery,
		Arguments: args,
	}
	qr3, err := s.db.QueryOneParameterized(permStmt)
	if err != nil {
		return false, err
	}
	if qr3.Err != nil {
		return false, qr3.Err
	}
	if qr3.Next() {
		row3, err := qr3.Slice()
		if err == nil && rowInt64(row3[0]) > 0 {
			slog.Debug("HasWebhookPermission: result", "module", "instance_perm", "user_id", userID, "webhook_id", webhookID, "allowed", true)
			return true, nil
		}
	}

	slog.Debug("HasWebhookPermission: result", "module", "instance_perm", "user_id", userID, "webhook_id", webhookID, "allowed", false)
	return false, nil
}

func (s *InstancePermissionService) GrantDatasourcePermission(ctx context.Context, datasourceID int64, roleID, userID *int64, permissionType string, grantedBy int64) error {
	slog.Info("GrantDatasourcePermission: granting", "module", "instance_perm", "datasource_id", datasourceID, "permission_type", permissionType, "granted_by", grantedBy)
	query := `
		INSERT INTO bdopsflow_datasource_permissions (datasource_id, role_id, user_id, permission_type, granted_by, granted_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{datasourceID, roleID, userID, permissionType, grantedBy, now},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}
	return nil
}

func (s *InstancePermissionService) RevokeDatasourcePermission(ctx context.Context, permID int64) error {
	slog.Info("RevokeDatasourcePermission: revoking", "module", "instance_perm", "perm_id", permID)
	query := `DELETE FROM bdopsflow_datasource_permissions WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{permID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	return err
}

func (s *InstancePermissionService) GetDatasourcePermissions(ctx context.Context, datasourceID int64) ([]*model.DatasourcePermission, error) {
	query := `
		SELECT id, datasource_id, role_id, user_id, permission_type, granted_by, granted_at
		FROM bdopsflow_datasource_permissions
		WHERE datasource_id = ?
		ORDER BY id ASC
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{datasourceID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var perms []*model.DatasourcePermission
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		perm := &model.DatasourcePermission{
			ID:             rowInt64(row[0]),
			DatasourceID:   rowInt64(row[1]),
			PermissionType: rowString(row[4]),
			GrantedAt:      rowString(row[6]),
		}

		if row[2] != nil {
			rid := rowInt64(row[2])
			if rid > 0 {
				perm.RoleID = &rid
			}
		}

		if row[3] != nil {
			uid := rowInt64(row[3])
			if uid > 0 {
				perm.UserID = &uid
			}
		}

		if row[5] != nil {
			gid := rowInt64(row[5])
			if gid > 0 {
				perm.GrantedBy = &gid
			}
		}

		perms = append(perms, perm)
	}

	return perms, nil
}

func (s *InstancePermissionService) GrantWebhookPermission(ctx context.Context, webhookID int64, roleID, userID *int64, permissionType string, grantedBy int64) error {
	slog.Info("GrantWebhookPermission: granting", "module", "instance_perm", "webhook_id", webhookID, "permission_type", permissionType, "granted_by", grantedBy)
	query := `
		INSERT INTO bdopsflow_webhook_permissions (webhook_id, role_id, user_id, permission_type, granted_by, granted_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{webhookID, roleID, userID, permissionType, grantedBy, now},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}
	return nil
}

func (s *InstancePermissionService) RevokeWebhookPermission(ctx context.Context, permID int64) error {
	slog.Info("RevokeWebhookPermission: revoking", "module", "instance_perm", "perm_id", permID)
	query := `DELETE FROM bdopsflow_webhook_permissions WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{permID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	return err
}

func (s *InstancePermissionService) GetWebhookPermissions(ctx context.Context, webhookID int64) ([]*model.WebhookPermission, error) {
	query := `
		SELECT id, webhook_id, role_id, user_id, permission_type, granted_by, granted_at
		FROM bdopsflow_webhook_permissions
		WHERE webhook_id = ?
		ORDER BY id ASC
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{webhookID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var perms []*model.WebhookPermission
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		perm := &model.WebhookPermission{
			ID:             rowInt64(row[0]),
			WebhookID:      rowInt64(row[1]),
			PermissionType: rowString(row[4]),
			GrantedAt:      rowString(row[6]),
		}

		if row[2] != nil {
			rid := rowInt64(row[2])
			if rid > 0 {
				perm.RoleID = &rid
			}
		}

		if row[3] != nil {
			uid := rowInt64(row[3])
			if uid > 0 {
				perm.UserID = &uid
			}
		}

		if row[5] != nil {
			gid := rowInt64(row[5])
			if gid > 0 {
				perm.GrantedBy = &gid
			}
		}

		perms = append(perms, perm)
	}

	return perms, nil
}
