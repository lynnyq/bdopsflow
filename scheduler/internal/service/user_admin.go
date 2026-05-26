package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/rsautil"
	rqlite "github.com/rqlite/gorqlite"
	"golang.org/x/crypto/bcrypt"
)

type UserAdminService struct {
	db      database.DB
	permSvc *PermissionService
	rsaUtil *rsautil.RSAUtil
}

func NewUserAdminService(db database.DB, permSvc *PermissionService, rsaUtil *rsautil.RSAUtil) *UserAdminService {
	return &UserAdminService{
		db:      db,
		permSvc: permSvc,
		rsaUtil: rsaUtil,
	}
}

func (s *UserAdminService) IsSystemAdminCheck(ctx context.Context, userID int64) (bool, error) {
	return s.permSvc.IsSystemAdmin(ctx, userID)
}

func (s *UserAdminService) ListUsers(ctx context.Context) ([]*model.User, error) {
	slog.Debug("ListUsers: fetching", "module", "user_admin")
	query := `
		SELECT id, username, real_name, phone, email, is_active, last_login_at, created_at, updated_at
		FROM bdopsflow_users
		ORDER BY id DESC
	`

	qr, err := s.db.QueryOne(query)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_users []*model.User
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		user := &model.User{
			ID:       rowInt64(row[0]),
			Username: rowString(row[1]),
			RealName: rowString(row[2]),
			Phone:    rowString(row[3]),
			Email:    rowString(row[4]),
			IsActive: rowBool(row[5]),
		}

		if t, ok := row[6].(time.Time); ok {
			user.LastLoginAt = &t
		}

		if t, ok := row[7].(time.Time); ok {
			user.CreatedAt = t
		}

		if t, ok := row[8].(time.Time); ok {
			user.UpdatedAt = t
		}

		bdopsflow_users = append(bdopsflow_users, user)
	}

	return bdopsflow_users, nil
}

func (s *UserAdminService) GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	slog.Debug("GetUserByID: fetching", "module", "user_admin", "user_id", userID)
	query := `
		SELECT id, username, real_name, phone, email, is_active, password, last_login_at, created_at, updated_at
		FROM bdopsflow_users
		WHERE id = ?
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

	if !qr.Next() {
		return nil, nil
	}

	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	user := &model.User{
		ID:       rowInt64(row[0]),
		Username: rowString(row[1]),
		RealName: rowString(row[2]),
		Phone:    rowString(row[3]),
		Email:    rowString(row[4]),
		IsActive: rowBool(row[5]),
		Password: rowString(row[6]),
	}

	if t, ok := row[7].(time.Time); ok {
		user.LastLoginAt = &t
	}

	if t, ok := row[8].(time.Time); ok {
		user.CreatedAt = t
	}

	if t, ok := row[9].(time.Time); ok {
		user.UpdatedAt = t
	}

	return user, nil
}

func (s *UserAdminService) CreateUser(ctx context.Context, username, realName, phone, email, password string, roleIDs, domainIDs []int64, createdBy int64) (*model.User, error) {
	slog.Info("CreateUser: creating", "module", "user_admin", "username", username)
	decodedPassword, err := s.decryptPassword(password)
	if err != nil {
		return nil, err
	}
	if err := validatePlaintextPassword(decodedPassword); err != nil {
		return nil, err
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(decodedPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO bdopsflow_users (username, real_name, phone, email, password, is_active, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?)
	`

	now := time.Now()
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{username, realName, phone, email, string(hashedPassword), createdBy, now, now},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}

	userID := result.LastInsertID
	slog.Info("CreateUser: success", "module", "user_admin", "user_id", userID, "username", username)

	if len(roleIDs) > 0 {
		for _, roleID := range roleIDs {
			insertQuery := `INSERT INTO bdopsflow_user_roles (user_id, role_id, created_at) VALUES (?, ?, ?)`
			insertStmt := rqlite.ParameterizedStatement{
				Query:     insertQuery,
				Arguments: []interface{}{userID, roleID, now},
			}
			_, err := s.db.WriteOneParameterized(insertStmt)
			if err != nil {
				slog.Error("CreateUser: failed to insert user role", "user_id", userID, "role_id", roleID, "error", err)
			}
		}
	}

	if len(domainIDs) > 0 {
		for i, domainID := range domainIDs {
			isDefault := 0
			if i == 0 {
				isDefault = 1
			}
			insertQuery := `INSERT INTO bdopsflow_user_domains (user_id, domain_id, is_default, created_at) VALUES (?, ?, ?, ?)`
			insertStmt := rqlite.ParameterizedStatement{
				Query:     insertQuery,
				Arguments: []interface{}{userID, domainID, isDefault, now},
			}
			_, err := s.db.WriteOneParameterized(insertStmt)
			if err != nil {
				slog.Error("CreateUser: failed to insert user domain", "user_id", userID, "domain_id", domainID, "error", err)
			}
		}
	}

	return s.GetUserByID(ctx, userID)
}

func (s *UserAdminService) UpdateUser(ctx context.Context, userID int64, username, realName, phone, email string, isActive bool) (*model.User, error) {
	slog.Info("UpdateUser: updating", "module", "user_admin", "user_id", userID)
	query := `
		UPDATE bdopsflow_users
		SET username = ?, real_name = ?, phone = ?, email = ?, is_active = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{username, realName, phone, email, isActive, now, userID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}

	slog.Info("UpdateUser: success", "module", "user_admin", "user_id", userID)
	return s.GetUserByID(ctx, userID)
}

func (s *UserAdminService) DeleteUser(ctx context.Context, userID int64) error {
	slog.Info("DeleteUser: deleting", "module", "user_admin", "user_id", userID)
	deleteRolesQuery := `DELETE FROM bdopsflow_user_roles WHERE user_id = ?`
	deleteRolesStmt := rqlite.ParameterizedStatement{
		Query:     deleteRolesQuery,
		Arguments: []interface{}{userID},
	}
	_, err := s.db.WriteOneParameterized(deleteRolesStmt)
	if err != nil {
		slog.Error("DeleteUser: failed to delete user roles", "user_id", userID, "error", err)
	}

	deleteDomainsQuery := `DELETE FROM bdopsflow_user_domains WHERE user_id = ?`
	deleteDomainsStmt := rqlite.ParameterizedStatement{
		Query:     deleteDomainsQuery,
		Arguments: []interface{}{userID},
	}
	_, err = s.db.WriteOneParameterized(deleteDomainsStmt)
	if err != nil {
		slog.Error("DeleteUser: failed to delete user domains", "user_id", userID, "error", err)
	}

	query := `DELETE FROM bdopsflow_users WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	_, err = s.db.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	slog.Info("DeleteUser: success", "module", "user_admin", "user_id", userID)
	return nil
}

func (s *UserAdminService) UpdateLastLogin(ctx context.Context, userID int64) error {
	query := `UPDATE bdopsflow_users SET last_login_at = ? WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{time.Now().Format(DateTimeFormat), userID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	return err
}

func (s *UserAdminService) decryptPassword(encrypted string) (string, error) {
	return s.rsaUtil.Decrypt(encrypted)
}

const (
	passwordMinLength = 6
	passwordMaxLength = 30
)

func validatePlaintextPassword(password string) error {
	if len(password) < passwordMinLength {
		return ErrPasswordTooShort
	}
	if len(password) > passwordMaxLength {
		return ErrPasswordTooLong
	}
	hasLetter := false
	hasDigit := false
	for _, c := range password {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasLetter = true
		}
		if c >= '0' && c <= '9' {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrPasswordWeak
	}
	return nil
}

func (s *UserAdminService) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}

	decodedOldPassword, err := s.decryptPassword(oldPassword)
	if err != nil {
		slog.Error("ChangePassword: failed to decrypt old password", "error", err)
		return fmt.Errorf("invalid old password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(decodedOldPassword)); err != nil {
		slog.Error("ChangePassword: bcrypt compare failed", "error", err)
		return fmt.Errorf("invalid old password")
	}

	decodedNewPassword, err := s.decryptPassword(newPassword)
	if err != nil {
		return err
	}

	if err := validatePlaintextPassword(decodedNewPassword); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(decodedNewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := `UPDATE bdopsflow_users SET password = ?, updated_at = ? WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{string(hashedPassword), time.Now().Format(DateTimeFormat), userID},
	}
	_, err = s.db.WriteOneParameterized(stmt)
	return err
}

func (s *UserAdminService) ResetPassword(ctx context.Context, userID int64, newPassword string) error {
	decodedNewPassword, err := s.decryptPassword(newPassword)
	if err != nil {
		return err
	}

	if err := validatePlaintextPassword(decodedNewPassword); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(decodedNewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := `UPDATE bdopsflow_users SET password = ?, updated_at = ? WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{string(hashedPassword), time.Now().Format(DateTimeFormat), userID},
	}
	_, err = s.db.WriteOneParameterized(stmt)
	return err
}

func (s *UserAdminService) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	slog.Debug("GetUserByUsername: fetching", "module", "user_admin", "username", username)
	query := `
		SELECT id, username, real_name, phone, email, is_active, last_login_at, created_at, updated_at
		FROM bdopsflow_users
		WHERE username = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{username},
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

	user := &model.User{
		ID:       rowInt64(row[0]),
		Username: rowString(row[1]),
		RealName: rowString(row[2]),
		Phone:    rowString(row[3]),
		Email:    rowString(row[4]),
		IsActive: rowBool(row[5]),
	}

	if t, ok := row[6].(time.Time); ok {
		user.LastLoginAt = &t
	}

	if t, ok := row[7].(time.Time); ok {
		user.CreatedAt = t
	}

	if t, ok := row[8].(time.Time); ok {
		user.UpdatedAt = t
	}

	return user, nil
}

func (s *UserAdminService) GetUsersByDomain(ctx context.Context, domainID int64) ([]*model.User, error) {
	query := `
		SELECT u.id, u.username, u.real_name, u.phone, u.email, u.is_active, u.last_login_at, u.created_at, u.updated_at
		FROM bdopsflow_users u
		JOIN bdopsflow_user_domains ud ON u.id = ud.user_id
		WHERE ud.domain_id = ?
		ORDER BY u.id DESC
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

	var users []*model.User
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		user := &model.User{
			ID:       rowInt64(row[0]),
			Username: rowString(row[1]),
			RealName: rowString(row[2]),
			Phone:    rowString(row[3]),
			Email:    rowString(row[4]),
			IsActive: rowBool(row[5]),
		}

		if t, ok := row[6].(time.Time); ok {
			user.LastLoginAt = &t
		}

		if t, ok := row[7].(time.Time); ok {
			user.CreatedAt = t
		}

		if t, ok := row[8].(time.Time); ok {
			user.UpdatedAt = t
		}

		users = append(users, user)
	}

	return users, nil
}

func (s *UserAdminService) GetUserRoles(ctx context.Context, userID int64) ([]*model.Role, error) {
	query := `
		SELECT r.id, r.name, r.code, r.description, r.is_system, r.parent_id, r.domain_id, r.created_at, r.updated_at
		FROM bdopsflow_roles r
		JOIN bdopsflow_user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = ?
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

	var roles []*model.Role
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
			val := rowInt64(row[5])
			if val > 0 {
				role.ParentID = &val
			}
		}

		if row[6] != nil {
			val := rowInt64(row[6])
			if val > 0 {
				role.DomainID = &val
			}
		}

		if t, ok := row[7].(time.Time); ok {
			role.CreatedAt = t
		}

		if t, ok := row[8].(time.Time); ok {
			role.UpdatedAt = t
		}

		roles = append(roles, role)
	}

	return roles, nil
}

func (s *UserAdminService) UpdateUserWithDomainCheck(ctx context.Context, adminID, userID int64, username, realName, phone, email string, isActive bool) (*model.User, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return s.UpdateUser(ctx, userID, username, realName, phone, email, isActive)
}

func (s *UserAdminService) AssignUserRoles(ctx context.Context, userID int64, roleIDs, domainIDs []int64) error {
	slog.Info("AssignUserRoles: assigning", "module", "user_admin", "user_id", userID, "role_ids", roleIDs)
	query := `DELETE FROM bdopsflow_user_roles WHERE user_id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, roleID := range roleIDs {
		query := `INSERT INTO bdopsflow_user_roles (user_id, role_id, created_at) VALUES (?, ?, ?)`
		stmt := rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: []interface{}{userID, roleID, now},
		}
		_, err := s.db.WriteOneParameterized(stmt)
		if err != nil {
			return err
		}
	}

	slog.Info("AssignUserRoles: success", "module", "user_admin", "user_id", userID)
	return nil
}

func (s *UserAdminService) AssignUserRolesWithDomain(ctx context.Context, userID int64, roleIDs []int64, domainID *int64) error {
	query := `DELETE FROM bdopsflow_user_roles WHERE user_id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, roleID := range roleIDs {
		query := `INSERT INTO bdopsflow_user_roles (user_id, role_id, domain_id, created_at) VALUES (?, ?, ?, ?)`
		stmt := rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: []interface{}{userID, roleID, domainID, now},
		}
		_, err := s.db.WriteOneParameterized(stmt)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *UserAdminService) AssignUserDomains(ctx context.Context, userID int64, domainIDs []int64) error {
	slog.Info("AssignUserDomains: assigning", "module", "user_admin", "user_id", userID, "domain_ids", domainIDs)
	deleteQuery := `DELETE FROM bdopsflow_user_domains WHERE user_id = ?`
	deleteStmt := rqlite.ParameterizedStatement{
		Query:     deleteQuery,
		Arguments: []interface{}{userID},
	}
	_, err := s.db.WriteOneParameterized(deleteStmt)
	if err != nil {
		return err
	}

	if len(domainIDs) == 0 {
		return nil
	}

	now := time.Now()
	for i, domainID := range domainIDs {
		isDefault := 0
		if i == 0 {
			isDefault = 1
		}
		insertQuery := `INSERT INTO bdopsflow_user_domains (user_id, domain_id, is_default, created_at) VALUES (?, ?, ?, ?)`
		insertStmt := rqlite.ParameterizedStatement{
			Query:     insertQuery,
			Arguments: []interface{}{userID, domainID, isDefault, now},
		}
		_, err := s.db.WriteOneParameterized(insertStmt)
		if err != nil {
			return err
		}
	}

	slog.Info("AssignUserDomains: success", "module", "user_admin", "user_id", userID)
	return nil
}

func (s *UserAdminService) GetCurrentUserInfo(ctx context.Context, userID int64) (*model.User, error) {
	return s.GetUserByID(ctx, userID)
}

func (s *UserAdminService) UpdateCurrentUser(ctx context.Context, userID int64, realName, phone, email string) (*model.User, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return s.UpdateUser(ctx, userID, user.Username, realName, phone, email, user.IsActive)
}

func (s *UserAdminService) ResetUserPasswordWithDomainCheck(ctx context.Context, adminID, userID int64, newPassword string) error {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	return s.ResetPassword(ctx, userID, newPassword)
}

func (s *UserAdminService) GetUserDomainInfos(ctx context.Context, userID int64) ([]*model.UserDomainInfo, error) {
	return s.permSvc.GetUserDomainInfos(ctx, userID)
}

func (s *UserAdminService) GetAllRoles(ctx context.Context) ([]*model.Role, error) {
	return s.permSvc.GetAllRoles(ctx)
}

func (s *UserAdminService) AreRolesSystemOnly(ctx context.Context, roleIDs []int64) (bool, error) {
	if len(roleIDs) == 0 {
		return false, nil
	}

	for _, roleID := range roleIDs {
		roleCode, err := s.permSvc.GetRoleCodeByID(ctx, roleID)
		if err != nil {
			slog.Error("AreRolesSystemOnly: failed to get role code", "role_id", roleID, "error", err)
			continue
		}
		if roleCode == "system_admin" {
			return true, nil
		}
	}
	return false, nil
}

func (s *UserAdminService) AreDomainsAccessibleByUser(ctx context.Context, userID int64, domainIDs []int64) (bool, error) {
	if len(domainIDs) == 0 {
		return true, nil
	}

	userDomainIDs, err := s.permSvc.GetUserDomainIDs(ctx, userID)
	if err != nil {
		return false, err
	}

	userDomainMap := make(map[int64]bool)
	for _, dID := range userDomainIDs {
		userDomainMap[dID] = true
	}

	for _, dID := range domainIDs {
		if !userDomainMap[dID] {
			return false, nil
		}
	}
	return true, nil
}

func (s *UserAdminService) BatchGetUserRoleIDs(ctx context.Context) (map[int64][]int64, error) {
	query := `SELECT user_id, role_id FROM bdopsflow_user_roles`
	qr, err := s.db.QueryOne(query)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	result := make(map[int64][]int64)
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		userID := rowInt64(row[0])
		roleID := rowInt64(row[1])
		result[userID] = append(result[userID], roleID)
	}
	return result, nil
}

func (s *UserAdminService) BatchGetUserDomainIDs(ctx context.Context) (map[int64][]int64, error) {
	query := `SELECT user_id, domain_id FROM bdopsflow_user_domains`
	qr, err := s.db.QueryOne(query)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	result := make(map[int64][]int64)
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		userID := rowInt64(row[0])
		domainID := rowInt64(row[1])
		result[userID] = append(result[userID], domainID)
	}
	return result, nil
}

func (s *UserAdminService) BatchGetUserRoleCodes(ctx context.Context) (map[int64][]string, error) {
	query := `SELECT ur.user_id, r.code FROM bdopsflow_user_roles ur JOIN bdopsflow_roles r ON ur.role_id = r.id`
	qr, err := s.db.QueryOne(query)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	result := make(map[int64][]string)
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		userID := rowInt64(row[0])
		roleCode := rowString(row[1])
		result[userID] = append(result[userID], roleCode)
	}
	return result, nil
}

func (s *UserAdminService) EnrichUsersWithRolesAndDomains(ctx context.Context, users []*model.User) {
	if len(users) == 0 {
		return
	}

	roleIDsMap, roleErr := s.BatchGetUserRoleIDs(ctx)
	if roleErr != nil {
		slog.Error("EnrichUsersWithRolesAndDomains: failed to batch get role IDs", "error", roleErr)
	}

	domainIDsMap, domainErr := s.BatchGetUserDomainIDs(ctx)
	if domainErr != nil {
		slog.Error("EnrichUsersWithRolesAndDomains: failed to batch get domain IDs", "error", domainErr)
	}

	roleCodesMap, roleCodesErr := s.BatchGetUserRoleCodes(ctx)
	if roleCodesErr != nil {
		slog.Error("EnrichUsersWithRolesAndDomains: failed to batch get role codes", "error", roleCodesErr)
	}

	domainNameMap, domainNameErr := s.getDomainNameMap(ctx)
	if domainNameErr != nil {
		slog.Error("EnrichUsersWithRolesAndDomains: failed to get domain names", "error", domainNameErr)
	}

	for _, u := range users {
		if u == nil {
			continue
		}
		if ids, ok := roleIDsMap[u.ID]; ok {
			u.RoleIDs = ids
		}
		if ids, ok := domainIDsMap[u.ID]; ok {
			u.DomainIDs = ids
			if domainNameMap != nil {
				for _, dID := range ids {
					if name, found := domainNameMap[dID]; found {
						u.DomainNames = append(u.DomainNames, name)
					}
				}
			}
		}
		if codes, ok := roleCodesMap[u.ID]; ok {
			u.RoleCodes = codes
		}
	}
}

func (s *UserAdminService) getDomainNameMap(ctx context.Context) (map[int64]string, error) {
	query := `SELECT id, name FROM bdopsflow_domains`
	qr, err := s.db.QueryOne(query)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	result := make(map[int64]string)
	for qr.Next() {
		row, sliceErr := qr.Slice()
		if sliceErr != nil {
			continue
		}
		id := rowInt64(row[0])
		name := rowString(row[1])
		if id > 0 && name != "" {
			result[id] = name
		}
	}
	return result, nil
}
