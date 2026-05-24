package datasource

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

type DatasourceService struct {
	db      *rqlite.Connection
	crypto  *Crypto
	config  *ConfigService
	manager *Manager
}

func NewDatasourceService(db *rqlite.Connection, crypto *Crypto, config *ConfigService, manager *Manager) *DatasourceService {
	return &DatasourceService{
		db:      db,
		crypto:  crypto,
		config:  config,
		manager: manager,
	}
}

func (s *DatasourceService) Create(ctx context.Context, ds *model.Datasource) error {
	checkStmt := rqlite.ParameterizedStatement{
		Query:     "SELECT COUNT(*) FROM bdopsflow_datasources WHERE name = ? AND domain_id = ?",
		Arguments: []interface{}{ds.Name, ds.DomainID},
	}
	qr, err := s.db.QueryOneParameterized(checkStmt)
	if err != nil {
		return fmt.Errorf("failed to check datasource name: %w", err)
	}
	if qr.Err != nil {
		return fmt.Errorf("failed to check datasource name: %w", qr.Err)
	}
	if qr.Next() {
		row, _ := qr.Slice()
		if dsRowInt64(row[0]) > 0 {
			return ErrDatasourceNameExists
		}
	}

	password := ds.Password
	if s.crypto != nil && password != "" {
		encrypted, err := s.crypto.Encrypt(password)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrCryptoFailed, err)
		}
		password = encrypted
	}

	isEnabled := 0
	if ds.IsEnabled {
		isEnabled = 1
	}

	allowWriteSQL := 0
	if ds.AllowWriteSQL {
		allowWriteSQL = 1
	}

	var createdBy interface{}
	if ds.CreatedBy != nil {
		createdBy = *ds.CreatedBy
	}

	now := time.Now().Format(dsDateTimeFormat)

	testStatus := ds.TestStatus
	if testStatus == "" {
		testStatus = "untested"
	}

	var lastTestAt interface{}
	if ds.LastTestAt != nil {
		lastTestAt = ds.LastTestAt.Format(dsDateTimeFormat)
	}

	insertStmt := rqlite.ParameterizedStatement{
		Query: `INSERT INTO bdopsflow_datasources (name, type, host, port, path, database, username, password, auth_type, connection_mode, zk_hosts, zk_path, rqlite_hosts, config, description, domain_id, is_enabled, allow_write_sql, test_status, last_test_at, created_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		Arguments: []interface{}{
			ds.Name, ds.Type, ds.Host, ds.Port, ds.Path, ds.Database,
			ds.Username, password, ds.AuthType, ds.ConnectionMode, ds.ZkHosts, ds.ZkPath, ds.RqliteHosts,
			ds.Config, ds.Description,
			ds.DomainID, isEnabled, allowWriteSQL, testStatus, lastTestAt, createdBy, now, now,
		},
	}
	result, err := s.db.WriteOneParameterized(insertStmt)
	if err != nil {
		return fmt.Errorf("failed to create datasource: %w", err)
	}
	if result.Err != nil {
		return fmt.Errorf("failed to create datasource: %w", result.Err)
	}

	ds.ID = result.LastInsertID
	return nil
}

func (s *DatasourceService) GetByID(ctx context.Context, id int64) (*model.Datasource, error) {
	stmt := rqlite.ParameterizedStatement{
		Query: `SELECT id, name, type, host, port, path, database, username, password, auth_type, connection_mode, zk_hosts, zk_path, rqlite_hosts, config, description, domain_id, is_enabled, allow_write_sql, test_status, last_test_at, created_by, updated_by, created_at, updated_at
			FROM bdopsflow_datasources WHERE id = ?`,
		Arguments: []interface{}{id},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, fmt.Errorf("failed to get datasource: %w", err)
	}
	if qr.Err != nil {
		return nil, fmt.Errorf("failed to get datasource: %w", qr.Err)
	}

	if !qr.Next() {
		return nil, ErrDatasourceNotFound
	}

	return scanDatasource(&qr)
}

func (s *DatasourceService) Get(ctx context.Context, domainID int64, dsType string, page, pageSize int) ([]*model.Datasource, int64, error) {
	whereClause := " WHERE 1=1"
	var args []interface{}

	if domainID > 0 {
		whereClause += " AND domain_id = ?"
		args = append(args, domainID)
	}
	if dsType != "" {
		whereClause += " AND type = ?"
		args = append(args, dsType)
	}

	countQuery := "SELECT COUNT(*) FROM bdopsflow_datasources" + whereClause
	var countQr rqlite.QueryResult
	var err error
	if len(args) > 0 {
		countStmt := rqlite.ParameterizedStatement{Query: countQuery, Arguments: args}
		countQr, err = s.db.QueryOneParameterized(countStmt)
	} else {
		countQr, err = s.db.QueryOne(countQuery)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count datasources: %w", err)
	}
	if countQr.Err != nil {
		return nil, 0, fmt.Errorf("failed to count datasources: %w", countQr.Err)
	}

	var total int64
	if countQr.Next() {
		row, _ := countQr.Slice()
		total = dsRowInt64(row[0])
	}

	offset := (page - 1) * pageSize
	dataQuery := `SELECT id, name, type, host, port, path, database, username, password, auth_type, connection_mode, zk_hosts, zk_path, rqlite_hosts, config, description, domain_id, is_enabled, allow_write_sql, test_status, last_test_at, created_by, updated_by, created_at, updated_at
		FROM bdopsflow_datasources` + whereClause + " ORDER BY created_at DESC LIMIT ? OFFSET ?"

	dataArgs := make([]interface{}, len(args))
	copy(dataArgs, args)
	dataArgs = append(dataArgs, pageSize, offset)

	var dataQr rqlite.QueryResult
	dataStmt := rqlite.ParameterizedStatement{Query: dataQuery, Arguments: dataArgs}
	dataQr, err = s.db.QueryOneParameterized(dataStmt)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query datasources: %w", err)
	}
	if dataQr.Err != nil {
		return nil, 0, fmt.Errorf("failed to query datasources: %w", dataQr.Err)
	}

	var datasources []*model.Datasource
	for dataQr.Next() {
		ds, scanErr := scanDatasource(&dataQr)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		datasources = append(datasources, ds)
	}

	return datasources, total, nil
}

func (s *DatasourceService) Update(ctx context.Context, ds *model.Datasource) error {
	existing, err := s.GetByID(ctx, ds.ID)
	if err != nil {
		return err
	}

	password := ds.Password
	if password != "" {
		if s.crypto != nil {
			encrypted, encErr := s.crypto.Encrypt(password)
			if encErr != nil {
				return fmt.Errorf("%w: %v", ErrCryptoFailed, encErr)
			}
			password = encrypted
		}
	} else {
		password = existing.Password
	}

	isEnabled := 0
	if ds.IsEnabled {
		isEnabled = 1
	}

	allowWriteSQL := 0
	if ds.AllowWriteSQL {
		allowWriteSQL = 1
	}

	var updatedBy interface{}
	if ds.UpdatedBy != nil {
		updatedBy = *ds.UpdatedBy
	}

	now := time.Now().Format(dsDateTimeFormat)
	updateStmt := rqlite.ParameterizedStatement{
		Query: `UPDATE bdopsflow_datasources SET name = ?, type = ?, host = ?, port = ?, path = ?, database = ?, username = ?, password = ?, auth_type = ?, connection_mode = ?, zk_hosts = ?, zk_path = ?, rqlite_hosts = ?, config = ?, description = ?, domain_id = ?, is_enabled = ?, allow_write_sql = ?, updated_by = ?, updated_at = ? WHERE id = ?`,
		Arguments: []interface{}{
			ds.Name, ds.Type, ds.Host, ds.Port, ds.Path, ds.Database,
			ds.Username, password, ds.AuthType, ds.ConnectionMode, ds.ZkHosts, ds.ZkPath, ds.RqliteHosts,
			ds.Config, ds.Description,
			ds.DomainID, isEnabled, allowWriteSQL, updatedBy, now, ds.ID,
		},
	}
	result, err := s.db.WriteOneParameterized(updateStmt)
	if err != nil {
		return fmt.Errorf("failed to update datasource: %w", err)
	}
	if result.Err != nil {
		return fmt.Errorf("failed to update datasource: %w", result.Err)
	}

	s.manager.RemoveDatasource(ds.ID)
	return nil
}

func (s *DatasourceService) Delete(ctx context.Context, id int64) error {
	s.manager.RemoveDatasource(id)

	deleteStmt := rqlite.ParameterizedStatement{
		Query:     "DELETE FROM bdopsflow_datasources WHERE id = ?",
		Arguments: []interface{}{id},
	}
	result, err := s.db.WriteOneParameterized(deleteStmt)
	if err != nil {
		return fmt.Errorf("failed to delete datasource: %w", err)
	}
	if result.Err != nil {
		return fmt.Errorf("failed to delete datasource: %w", result.Err)
	}

	return nil
}

func (s *DatasourceService) TestDatasource(ctx context.Context, id int64) error {
	ds, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	testErr := s.manager.TestConnection(ctx, ds)

	testStatus := "success"
	if testErr != nil {
		testStatus = "failed"
		slog.Error("datasource test connection failed", "datasource_id", id, "name", ds.Name, "type", ds.Type, "error", testErr)
	} else {
		slog.Info("datasource test connection succeeded", "datasource_id", id, "name", ds.Name, "type", ds.Type)
	}

	now := time.Now().Format(dsDateTimeFormat)
	updateStmt := rqlite.ParameterizedStatement{
		Query:     "UPDATE bdopsflow_datasources SET test_status = ?, last_test_at = ?, updated_at = ? WHERE id = ?",
		Arguments: []interface{}{testStatus, now, now, id},
	}
	if _, dbErr := s.db.WriteOneParameterized(updateStmt); dbErr != nil {
		slog.Warn("failed to update test status", "id", id, "error", dbErr)
	}

	return testErr
}

func (s *DatasourceService) GrantPermission(ctx context.Context, perm *model.DatasourcePermission) error {
	if perm.RoleID == nil && perm.UserID == nil {
		return fmt.Errorf("role_id or user_id is required")
	}

	if !IsValidPermissionType(perm.PermissionType) {
		return ErrInvalidPermissionType
	}

	includedPerms := GetIncludedPermissions(perm.PermissionType)
	if len(includedPerms) > 0 {
		placeholders := make([]string, len(includedPerms))
		for i := range includedPerms {
			placeholders[i] = "?"
		}
		inClause := strings.Join(placeholders, ",")

		var deleteQuery string
		var deleteArgs []interface{}
		if perm.UserID != nil {
			deleteQuery = fmt.Sprintf(
				"DELETE FROM bdopsflow_datasource_permissions WHERE datasource_id = ? AND user_id = ? AND permission_type IN (%s)",
				inClause,
			)
			deleteArgs = []interface{}{perm.DatasourceID, *perm.UserID}
		} else {
			deleteQuery = fmt.Sprintf(
				"DELETE FROM bdopsflow_datasource_permissions WHERE datasource_id = ? AND role_id = ? AND permission_type IN (%s)",
				inClause,
			)
			deleteArgs = []interface{}{perm.DatasourceID, *perm.RoleID}
		}
		for _, p := range includedPerms {
			deleteArgs = append(deleteArgs, p)
		}

		deleteStmt := rqlite.ParameterizedStatement{Query: deleteQuery, Arguments: deleteArgs}
		if _, err := s.db.WriteOneParameterized(deleteStmt); err != nil {
			slog.Warn("failed to clean up lower-level permissions before granting", "error", err)
		}
	}

	var grantedBy interface{}
	if perm.GrantedBy != nil {
		grantedBy = *perm.GrantedBy
	}

	var roleIDVal interface{}
	if perm.RoleID != nil {
		roleIDVal = *perm.RoleID
	}
	var userIDVal interface{}
	if perm.UserID != nil {
		userIDVal = *perm.UserID
	}

	now := time.Now().Format(dsDateTimeFormat)
	insertStmt := rqlite.ParameterizedStatement{
		Query:     "INSERT INTO bdopsflow_datasource_permissions (datasource_id, role_id, user_id, permission_type, granted_by, granted_at) VALUES (?, ?, ?, ?, ?, ?)",
		Arguments: []interface{}{perm.DatasourceID, roleIDVal, userIDVal, perm.PermissionType, grantedBy, now},
	}
	result, err := s.db.WriteOneParameterized(insertStmt)
	if err != nil {
		return fmt.Errorf("failed to grant permission: %w", err)
	}
	if result.Err != nil {
		return fmt.Errorf("failed to grant permission: %w", result.Err)
	}

	perm.ID = result.LastInsertID
	return nil
}

func (s *DatasourceService) UpdatePermission(ctx context.Context, id int64, permissionType string) error {
	if !IsValidPermissionType(permissionType) {
		return ErrInvalidPermissionType
	}

	checkStmt := rqlite.ParameterizedStatement{
		Query:     "SELECT COUNT(*) FROM bdopsflow_datasource_permissions WHERE id = ?",
		Arguments: []interface{}{id},
	}
	qr, err := s.db.QueryOneParameterized(checkStmt)
	if err != nil {
		return fmt.Errorf("failed to check permission: %w", err)
	}
	if qr.Err != nil {
		return fmt.Errorf("failed to check permission: %w", qr.Err)
	}
	if qr.Next() {
		row, _ := qr.Slice()
		if dsRowInt64(row[0]) == 0 {
			return ErrPermissionNotFound
		}
	}

	now := time.Now().Format(dsDateTimeFormat)
	updateStmt := rqlite.ParameterizedStatement{
		Query:     "UPDATE bdopsflow_datasource_permissions SET permission_type = ?, granted_at = ? WHERE id = ?",
		Arguments: []interface{}{permissionType, now, id},
	}
	result, err := s.db.WriteOneParameterized(updateStmt)
	if err != nil {
		return fmt.Errorf("failed to update permission: %w", err)
	}
	if result.Err != nil {
		return fmt.Errorf("failed to update permission: %w", result.Err)
	}
	return nil
}

func (s *DatasourceService) RevokePermission(ctx context.Context, id int64) error {
	deleteStmt := rqlite.ParameterizedStatement{
		Query:     "DELETE FROM bdopsflow_datasource_permissions WHERE id = ?",
		Arguments: []interface{}{id},
	}
	result, err := s.db.WriteOneParameterized(deleteStmt)
	if err != nil {
		return fmt.Errorf("failed to revoke permission: %w", err)
	}
	if result.Err != nil {
		return fmt.Errorf("failed to revoke permission: %w", result.Err)
	}
	return nil
}

func (s *DatasourceService) CheckPermission(ctx context.Context, userID int64, datasourceID int64, permType string) (bool, error) {
	effectivePerms := GetEffectivePermissions(permType)
	if len(effectivePerms) == 0 {
		effectivePerms = []string{permType}
	}

	placeholders := make([]string, len(effectivePerms))
	userPermArgs := make([]interface{}, 0, len(effectivePerms)+2)
	userPermArgs = append(userPermArgs, userID, datasourceID)
	for i, p := range effectivePerms {
		placeholders[i] = "?"
		userPermArgs = append(userPermArgs, p)
	}

	userPermQuery := fmt.Sprintf(
		"SELECT COUNT(*) FROM bdopsflow_datasource_permissions WHERE user_id = ? AND datasource_id = ? AND permission_type IN (%s)",
		strings.Join(placeholders, ","),
	)
	stmt := rqlite.ParameterizedStatement{
		Query:     userPermQuery,
		Arguments: userPermArgs,
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrPermissionServiceError, err)
	}
	if qr.Err != nil {
		return false, fmt.Errorf("%w: %v", ErrPermissionServiceError, qr.Err)
	}
	if qr.Next() {
		row, _ := qr.Slice()
		if dsRowInt64(row[0]) > 0 {
			return true, nil
		}
	}

	rolePermArgs := make([]interface{}, 0, len(effectivePerms)+2)
	rolePermArgs = append(rolePermArgs, userID, datasourceID)
	for _, p := range effectivePerms {
		rolePermArgs = append(rolePermArgs, p)
	}

	rolePermQuery := fmt.Sprintf(
		"SELECT COUNT(*) FROM bdopsflow_datasource_permissions dp JOIN bdopsflow_user_roles ur ON dp.role_id = ur.role_id WHERE ur.user_id = ? AND dp.datasource_id = ? AND dp.permission_type IN (%s)",
		strings.Join(placeholders, ","),
	)
	stmt = rqlite.ParameterizedStatement{
		Query:     rolePermQuery,
		Arguments: rolePermArgs,
	}
	qr, err = s.db.QueryOneParameterized(stmt)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrPermissionServiceError, err)
	}
	if qr.Err != nil {
		return false, fmt.Errorf("%w: %v", ErrPermissionServiceError, qr.Err)
	}
	if qr.Next() {
		row, _ := qr.Slice()
		return dsRowInt64(row[0]) > 0, nil
	}

	return false, nil
}

func (s *DatasourceService) GetDatasourceDomainID(dsID int64) (int64, error) {
	stmt := rqlite.ParameterizedStatement{
		Query:     "SELECT domain_id FROM bdopsflow_datasources WHERE id = ?",
		Arguments: []interface{}{dsID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return 0, ErrDatasourceNotFound
	}
	if qr.Err != nil {
		return 0, ErrDatasourceNotFound
	}
	if qr.Next() {
		row, _ := qr.Slice()
		return dsRowInt64(row[0]), nil
	}
	return 0, ErrDatasourceNotFound
}

func (s *DatasourceService) CheckDatasourcePermission(userID int64, dsID int64, action string) (bool, error) {
	return s.CheckPermission(context.Background(), userID, dsID, action)
}

func (s *DatasourceService) GetPermissions(ctx context.Context, datasourceID int64) ([]*model.DatasourcePermission, error) {
	stmt := rqlite.ParameterizedStatement{
		Query:     "SELECT id, datasource_id, role_id, user_id, permission_type, granted_by, granted_at FROM bdopsflow_datasource_permissions WHERE datasource_id = ?",
		Arguments: []interface{}{datasourceID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}
	if qr.Err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", qr.Err)
	}

	var permissions []*model.DatasourcePermission
	for qr.Next() {
		row, sliceErr := qr.Slice()
		if sliceErr != nil {
			continue
		}

		perm := &model.DatasourcePermission{
			ID:             dsRowInt64(row[0]),
			DatasourceID:   dsRowInt64(row[1]),
			PermissionType: dsRowString(row[4]),
			GrantedAt:      dsRowString(row[6]),
		}

		if row[2] != nil {
			roleID := dsRowInt64(row[2])
			perm.RoleID = &roleID
		}
		if row[3] != nil {
			userID := dsRowInt64(row[3])
			perm.UserID = &userID
		}
		if row[5] != nil {
			grantedBy := dsRowInt64(row[5])
			perm.GrantedBy = &grantedBy
		}

		permissions = append(permissions, perm)
	}

	return permissions, nil
}

func (s *DatasourceService) GetDatasourceMaskedPassword(ctx context.Context, id int64) (string, error) {
	_, err := s.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return "******", nil
}

func scanDatasource(qr *rqlite.QueryResult) (*model.Datasource, error) {
	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	ds := &model.Datasource{
		ID:             dsRowInt64(row[0]),
		Name:           dsRowString(row[1]),
		Type:           dsRowString(row[2]),
		Host:           dsRowString(row[3]),
		Port:           int(dsRowInt64(row[4])),
		Path:           dsRowString(row[5]),
		Database:       dsRowString(row[6]),
		Username:       dsRowString(row[7]),
		Password:       dsRowString(row[8]),
		AuthType:       dsRowString(row[9]),
		ConnectionMode: dsRowString(row[10]),
		ZkHosts:        dsRowString(row[11]),
		ZkPath:         dsRowString(row[12]),
		RqliteHosts:    dsRowString(row[13]),
		Config:         dsRowString(row[14]),
		Description:    dsRowString(row[15]),
		DomainID:       dsRowInt64(row[16]),
		IsEnabled:      dsRowBool(row[17]),
		AllowWriteSQL:  dsRowBool(row[18]),
		TestStatus:     dsRowString(row[19]),
	}

	if t, ok := row[20].(time.Time); ok {
		ds.LastTestAt = &t
	} else if row[20] != nil {
		lastTestAt := dsRowString(row[20])
		if lastTestAt != "" {
			if t, parseErr := dsParseTimeInLocalTimezone(lastTestAt); parseErr == nil {
				ds.LastTestAt = &t
			}
		}
	}

	if row[21] != nil {
		createdBy := dsRowInt64(row[21])
		if createdBy > 0 {
			ds.CreatedBy = &createdBy
		}
	}

	if row[22] != nil {
		updatedBy := dsRowInt64(row[22])
		if updatedBy > 0 {
			ds.UpdatedBy = &updatedBy
		}
	}

	if t, ok := row[23].(time.Time); ok {
		ds.CreatedAt = t
	} else if s, ok := row[23].(string); ok && s != "" {
		if parsed, parseErr := dsParseTimeInLocalTimezone(s); parseErr == nil {
			ds.CreatedAt = parsed
		}
	}

	if t, ok := row[24].(time.Time); ok {
		ds.UpdatedAt = t
	} else if s, ok := row[24].(string); ok && s != "" {
		if parsed, parseErr := dsParseTimeInLocalTimezone(s); parseErr == nil {
			ds.UpdatedAt = parsed
		}
	}

	return ds, nil
}

const (
	dsDateTimeFormat       = time.RFC3339Nano
	dsLegacyDateTimeFormat = "2006-01-02 15:04:05"
)

func dsParseTimeInLocalTimezone(timeStr string) (time.Time, error) {
	if parsed, err := time.Parse(dsDateTimeFormat, timeStr); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return parsed, nil
	}
	return time.Parse(dsLegacyDateTimeFormat, timeStr)
}

func dsRowInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		var n int64
		fmt.Sscanf(val, "%d", &n)
		return n
	}
	return 0
}

func dsRowString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func (s *DatasourceService) RecordQueryHistory(ctx context.Context, history *model.QueryHistory) error {
	var datasourceID interface{}
	if history.DatasourceID != nil {
		datasourceID = *history.DatasourceID
	}

	var executedBy interface{}
	if history.ExecutedBy != nil {
		executedBy = *history.ExecutedBy
	}

	now := time.Now().Format(dsDateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query: `INSERT INTO bdopsflow_query_history (query_id, datasource_id, datasource_name, sql_text, database, execution_time, row_count, status, error_message, executed_by, domain_id, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		Arguments: []interface{}{
			history.QueryID, datasourceID, history.DatasourceName,
			history.SQLText, history.Database, history.ExecutionTime,
			history.RowCount, history.Status, history.ErrorMessage,
			executedBy, history.DomainID, now,
		},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return fmt.Errorf("failed to record query history: %w", err)
	}
	if result.Err != nil {
		return fmt.Errorf("failed to record query history: %w", result.Err)
	}

	history.ID = result.LastInsertID
	return nil
}

func (s *DatasourceService) GetQueryHistory(ctx context.Context, domainID int64, page, pageSize int) ([]*model.QueryHistory, int64, error) {
	whereClause := " WHERE 1=1"
	var args []interface{}

	if domainID > 0 {
		whereClause += " AND domain_id = ?"
		args = append(args, domainID)
	}

	countQuery := "SELECT COUNT(*) FROM bdopsflow_query_history" + whereClause
	var countQr rqlite.QueryResult
	var err error
	if len(args) > 0 {
		countStmt := rqlite.ParameterizedStatement{Query: countQuery, Arguments: args}
		countQr, err = s.db.QueryOneParameterized(countStmt)
	} else {
		countQr, err = s.db.QueryOne(countQuery)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count query history: %w", err)
	}
	if countQr.Err != nil {
		return nil, 0, fmt.Errorf("failed to count query history: %w", countQr.Err)
	}

	var total int64
	if countQr.Next() {
		row, _ := countQr.Slice()
		total = dsRowInt64(row[0])
	}

	offset := (page - 1) * pageSize
	dataQuery := `SELECT id, query_id, datasource_id, datasource_name, sql_text, database, execution_time, row_count, status, error_message, executed_by, domain_id, created_at
		FROM bdopsflow_query_history` + whereClause + " ORDER BY created_at DESC LIMIT ? OFFSET ?"

	dataArgs := make([]interface{}, len(args))
	copy(dataArgs, args)
	dataArgs = append(dataArgs, pageSize, offset)

	dataStmt := rqlite.ParameterizedStatement{Query: dataQuery, Arguments: dataArgs}
	dataQr, err := s.db.QueryOneParameterized(dataStmt)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query history: %w", err)
	}
	if dataQr.Err != nil {
		return nil, 0, fmt.Errorf("failed to query history: %w", dataQr.Err)
	}

	var histories []*model.QueryHistory
	for dataQr.Next() {
		row, sliceErr := dataQr.Slice()
		if sliceErr != nil {
			continue
		}

		h := &model.QueryHistory{
			ID:             dsRowInt64(row[0]),
			QueryID:        dsRowString(row[1]),
			DatasourceName: dsRowString(row[3]),
			SQLText:        dsRowString(row[4]),
			Database:       dsRowString(row[5]),
			ExecutionTime:  dsRowFloat64(row[6]),
			RowCount:       int(dsRowInt64(row[7])),
			Status:         dsRowString(row[8]),
			ErrorMessage:   dsRowString(row[9]),
			DomainID:       dsRowInt64(row[11]),
		}

		if row[2] != nil {
			dsID := dsRowInt64(row[2])
			h.DatasourceID = &dsID
		}

		if row[10] != nil {
			execBy := dsRowInt64(row[10])
			h.ExecutedBy = &execBy
		}

		if t, ok := row[12].(time.Time); ok {
			h.CreatedAt = t
		} else if s, ok := row[12].(string); ok && s != "" {
			if parsed, parseErr := dsParseTimeInLocalTimezone(s); parseErr == nil {
				h.CreatedAt = parsed
			}
		}

		histories = append(histories, h)
	}

	return histories, total, nil
}

// DeleteQueryHistory deletes a single query history record
func (s *DatasourceService) DeleteQueryHistory(ctx context.Context, id int64) error {
	stmt := rqlite.ParameterizedStatement{
		Query:     "DELETE FROM bdopsflow_query_history WHERE id = ?",
		Arguments: []interface{}{id},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return fmt.Errorf("failed to delete query history: %w", err)
	}
	return nil
}

// BatchDeleteQueryHistory batch deletes query history records
func (s *DatasourceService) BatchDeleteQueryHistory(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("DELETE FROM bdopsflow_query_history WHERE id IN (%s)", strings.Join(placeholders, ","))
	stmt := rqlite.ParameterizedStatement{Query: query, Arguments: args}
	_, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return fmt.Errorf("failed to batch delete query history: %w", err)
	}
	return nil
}

func (s *DatasourceService) CreateSavedSQL(ctx context.Context, saved *model.SavedSQL) error {
	var createdBy interface{}
	if saved.CreatedBy != nil {
		createdBy = *saved.CreatedBy
	}

	var updatedBy interface{}
	if saved.UpdatedBy != nil {
		updatedBy = *saved.UpdatedBy
	}

	isPublic := 0
	if saved.IsPublic {
		isPublic = 1
	}

	now := time.Now().Format(dsDateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query: `INSERT INTO bdopsflow_saved_sql (name, datasource_id, sql_text, description, created_by, updated_by, domain_id, is_public, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		Arguments: []interface{}{
			saved.Name, saved.DatasourceID, saved.SQLText, saved.Description,
			createdBy, updatedBy, saved.DomainID, isPublic, now, now,
		},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return fmt.Errorf("failed to create saved SQL: %w", err)
	}
	if result.Err != nil {
		return fmt.Errorf("failed to create saved SQL: %w", result.Err)
	}

	saved.ID = result.LastInsertID
	return nil
}

func (s *DatasourceService) GetSavedSQL(ctx context.Context, domainID int64, userID int64, page, pageSize int) ([]*model.SavedSQL, int64, error) {
	whereClause := " WHERE 1=1"
	var args []interface{}

	if domainID > 0 {
		whereClause += " AND domain_id = ?"
		args = append(args, domainID)
	}

	if userID > 0 {
		whereClause += " AND (created_by = ? OR is_public = 1)"
		args = append(args, userID)
	}

	countQuery := "SELECT COUNT(*) FROM bdopsflow_saved_sql" + whereClause
	var countQr rqlite.QueryResult
	var err error
	if len(args) > 0 {
		countStmt := rqlite.ParameterizedStatement{Query: countQuery, Arguments: args}
		countQr, err = s.db.QueryOneParameterized(countStmt)
	} else {
		countQr, err = s.db.QueryOne(countQuery)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count saved SQL: %w", err)
	}
	if countQr.Err != nil {
		return nil, 0, fmt.Errorf("failed to count saved SQL: %w", countQr.Err)
	}

	var total int64
	if countQr.Next() {
		row, _ := countQr.Slice()
		total = dsRowInt64(row[0])
	}

	offset := (page - 1) * pageSize
	dataQuery := `SELECT id, name, datasource_id, sql_text, description, created_by, updated_by, domain_id, is_public, created_at, updated_at
		FROM bdopsflow_saved_sql` + whereClause + " ORDER BY created_at DESC LIMIT ? OFFSET ?"

	dataArgs := make([]interface{}, len(args))
	copy(dataArgs, args)
	dataArgs = append(dataArgs, pageSize, offset)

	dataStmt := rqlite.ParameterizedStatement{Query: dataQuery, Arguments: dataArgs}
	dataQr, err := s.db.QueryOneParameterized(dataStmt)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query saved SQL: %w", err)
	}
	if dataQr.Err != nil {
		return nil, 0, fmt.Errorf("failed to query saved SQL: %w", dataQr.Err)
	}

	var savedList []*model.SavedSQL
	for dataQr.Next() {
		row, sliceErr := dataQr.Slice()
		if sliceErr != nil {
			continue
		}

		saved := &model.SavedSQL{
			ID:           dsRowInt64(row[0]),
			Name:         dsRowString(row[1]),
			DatasourceID: dsRowInt64(row[2]),
			SQLText:      dsRowString(row[3]),
			Description:  dsRowString(row[4]),
			DomainID:     dsRowInt64(row[7]),
			IsPublic:     dsRowBool(row[8]),
		}

		if row[5] != nil {
			createdBy := dsRowInt64(row[5])
			saved.CreatedBy = &createdBy
		}

		if row[6] != nil {
			updatedBy := dsRowInt64(row[6])
			saved.UpdatedBy = &updatedBy
		}

		if t, ok := row[9].(time.Time); ok {
			saved.CreatedAt = t
		} else if s, ok := row[9].(string); ok && s != "" {
			if parsed, parseErr := dsParseTimeInLocalTimezone(s); parseErr == nil {
				saved.CreatedAt = parsed
			}
		}

		if t, ok := row[10].(time.Time); ok {
			saved.UpdatedAt = t
		} else if s, ok := row[10].(string); ok && s != "" {
			if parsed, parseErr := dsParseTimeInLocalTimezone(s); parseErr == nil {
				saved.UpdatedAt = parsed
			}
		}

		savedList = append(savedList, saved)
	}

	return savedList, total, nil
}

func (s *DatasourceService) DeleteSavedSQL(ctx context.Context, id int64) error {
	stmt := rqlite.ParameterizedStatement{
		Query:     "DELETE FROM bdopsflow_saved_sql WHERE id = ?",
		Arguments: []interface{}{id},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return fmt.Errorf("failed to delete saved SQL: %w", err)
	}
	if result.Err != nil {
		return fmt.Errorf("failed to delete saved SQL: %w", result.Err)
	}
	return nil
}

func dsRowBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case int64:
		return val != 0
	case float64:
		return val != 0
	}
	return false
}

func dsRowFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	}
	return 0
}
