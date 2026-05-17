package service

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
	"golang.org/x/crypto/bcrypt"
)

// UserAdminService 用户管理服务
type UserAdminService struct {
	db      rqlite.Connection
	permSvc *PermissionService
}

// NewUserAdminService 创建用户管理服务
func NewUserAdminService(db rqlite.Connection, permSvc *PermissionService) *UserAdminService {
	return &UserAdminService{
		db:      db,
		permSvc: permSvc,
	}
}

// ListUsers 获取用户列表
func (s *UserAdminService) ListUsers(ctx context.Context) ([]*model.User, error) {
	query := `
		SELECT id, username, email, domain_id, role, is_active
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
			Email:    rowString(row[2]),
			DomainID: rowInt64(row[3]),
			Role:     rowString(row[4]),
			IsActive: rowBool(row[5]),
		}

		bdopsflow_users = append(bdopsflow_users, user)
	}

	return bdopsflow_users, nil
}

// GetUserByID 根据ID获取用户
func (s *UserAdminService) GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	query := `
		SELECT id, username, email, domain_id, role, is_active
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
		Email:    rowString(row[2]),
		DomainID: rowInt64(row[3]),
		Role:     rowString(row[4]),
		IsActive: rowBool(row[5]),
	}

	return user, nil
}

// CreateUser 创建用户
func (s *UserAdminService) CreateUser(ctx context.Context, username, email, password string, createdBy int64) (*model.User, error) {
	// 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO bdopsflow_users (username, email, password, is_active, created_by, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?, ?)
	`

	now := time.Now()
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{username, email, string(hashedPassword), createdBy, now, now},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}

	userID := result.LastInsertID
	return s.GetUserByID(ctx, userID)
}

// UpdateUser 更新用户
func (s *UserAdminService) UpdateUser(ctx context.Context, userID int64, username, email, role string, isActive bool) (*model.User, error) {
	query := `
		UPDATE bdopsflow_users
		SET username = ?, email = ?, role = ?, is_active = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{username, email, role, isActive, now, userID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}

	// 清除权限缓存
	s.permSvc.InvalidateUserPermissionCache(ctx, userID)

	return s.GetUserByID(ctx, userID)
}

// DeleteUser 删除用户
func (s *UserAdminService) DeleteUser(ctx context.Context, userID int64) error {
	// 先删除用户角色映射
	deleteUserRolesQuery := `DELETE FROM bdopsflow_user_roles WHERE user_id = ?`
	deleteUserRolesStmt := rqlite.ParameterizedStatement{
		Query:     deleteUserRolesQuery,
		Arguments: []interface{}{userID},
	}
	_, err := s.db.WriteOneParameterized(deleteUserRolesStmt)
	if err != nil {
		return err
	}

	// 再删除用户
	query := `DELETE FROM bdopsflow_users WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	_, err = s.db.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	// 清除权限缓存
	s.permSvc.InvalidateUserPermissionCache(ctx, userID)

	return nil
}

// AssignUserRoles 分配用户角色
func (s *UserAdminService) AssignUserRoles(ctx context.Context, userID int64, roleIDs []int64, domainIDs []int64) error {
	// 删除旧的角色映射
	deleteQuery := `DELETE FROM bdopsflow_user_roles WHERE user_id = ?`
	deleteStmt := rqlite.ParameterizedStatement{
		Query:     deleteQuery,
		Arguments: []interface{}{userID},
	}
	_, err := s.db.WriteOneParameterized(deleteStmt)
	if err != nil {
		return err
	}

	// 批量插入新的角色映射
	if len(roleIDs) > 0 {
		var statements []rqlite.ParameterizedStatement
		now := time.Now()

		for i, roleID := range roleIDs {
			var domainID interface{}
			if len(domainIDs) > i && domainIDs[i] != 0 {
				domainID = domainIDs[i]
			} else {
				domainID = nil
			}

			query := `INSERT INTO bdopsflow_user_roles (user_id, role_id, domain_id, created_at) VALUES (?, ?, ?, ?)`
			stmt := rqlite.ParameterizedStatement{
				Query:     query,
				Arguments: []interface{}{userID, roleID, domainID, now},
			}
			statements = append(statements, stmt)
		}

		_, err := s.db.WriteParameterized(statements)
		if err != nil {
			return err
		}
	}

	// 清除权限缓存
	s.permSvc.InvalidateUserPermissionCache(ctx, userID)

	return nil
}

// GetUserRoles 获取用户角色
func (s *UserAdminService) GetUserRoles(ctx context.Context, userID int64) ([]*model.UserRoleDetail, error) {
	query := `
		SELECT ur.role_id, r.name, r.code, ur.domain_id, d.name
		FROM bdopsflow_user_roles ur
		LEFT JOIN bdopsflow_roles r ON ur.role_id = r.id
		LEFT JOIN bdopsflow_domains d ON ur.domain_id = d.id
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

	var bdopsflow_roles []*model.UserRoleDetail
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		role := &model.UserRoleDetail{
			RoleID:   rowInt64(row[0]),
			RoleName: rowString(row[1]),
			RoleCode: rowString(row[2]),
		}

		if row[3] != nil {
			domainID := rowInt64(row[3])
			role.DomainID = &domainID
		}

		if row[4] != nil {
			domainName := rowString(row[4])
			role.DomainName = domainName
		}

		bdopsflow_roles = append(bdopsflow_roles, role)
	}

	return bdopsflow_roles, nil
}

// AssignUserDomains 分配用户领域（用于兼容性）
func (s *UserAdminService) AssignUserDomains(ctx context.Context, userID int64, domainIDs []int64) error {
	query := `UPDATE bdopsflow_users SET domain_id = ? WHERE id = ?`

	var primaryDomainID interface{}
	if len(domainIDs) > 0 && domainIDs[0] != 0 {
		primaryDomainID = domainIDs[0]
	} else {
		primaryDomainID = nil
	}

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{primaryDomainID, userID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	// 清除权限缓存
	s.permSvc.InvalidateUserPermissionCache(ctx, userID)

	return nil
}

// GetUserPasswordHash 获取用户的密码哈希
func (s *UserAdminService) GetUserPasswordHash(ctx context.Context, userID int64) (string, error) {
	query := `SELECT password FROM bdopsflow_users WHERE id = ?`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return "", err
	}
	if qr.Err != nil {
		return "", qr.Err
	}

	if !qr.Next() {
		return "", ErrUserNotFound
	}

	row, err := qr.Slice()
	if err != nil {
		return "", err
	}

	return rowString(row[0]), nil
}

// UpdateCurrentUser 更新当前用户信息（只能修改邮箱）
func (s *UserAdminService) UpdateCurrentUser(ctx context.Context, userID int64, email string) (*model.User, error) {
	query := `
		UPDATE bdopsflow_users
		SET email = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{email, now, userID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}

	return s.GetUserByID(ctx, userID)
}

// ChangePassword 修改当前用户密码
func (s *UserAdminService) ChangePassword(ctx context.Context, userID int64, oldPasswordB64, newPasswordB64 string) error {
	// 解码 Base64 编码的密码
	oldPassword, err := base64.StdEncoding.DecodeString(oldPasswordB64)
	if err != nil {
		return ErrWrongPassword
	}

	newPassword, err := base64.StdEncoding.DecodeString(newPasswordB64)
	if err != nil {
		return ErrPasswordTooShort
	}

	// 验证新密码长度
	if len(newPassword) < 6 {
		return ErrPasswordTooShort
	}

	// 获取当前密码哈希
	currentHash, err := s.GetUserPasswordHash(ctx, userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), oldPassword); err != nil {
		return ErrWrongPassword
	}

	// 哈希新密码
	hashedPassword, err := bcrypt.GenerateFromPassword(newPassword, bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 更新密码
	query := `UPDATE bdopsflow_users SET password = ?, updated_at = ? WHERE id = ?`
	now := time.Now()
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{string(hashedPassword), now, userID},
	}
	_, err = s.db.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	return nil
}

// ResetUserPassword 重置用户密码（管理员用）
func (s *UserAdminService) ResetUserPassword(ctx context.Context, targetUserID int64, newPasswordB64 string) error {
	// 解码 Base64 编码的密码
	newPassword, err := base64.StdEncoding.DecodeString(newPasswordB64)
	if err != nil {
		return ErrPasswordTooShort
	}

	// 验证新密码长度
	if len(newPassword) < 6 {
		return ErrPasswordTooShort
	}

	// 哈希新密码
	hashedPassword, err := bcrypt.GenerateFromPassword(newPassword, bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 更新密码
	query := `UPDATE bdopsflow_users SET password = ?, updated_at = ? WHERE id = ?`
	now := time.Now()
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{string(hashedPassword), now, targetUserID},
	}
	_, err = s.db.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	// 清除权限缓存
	s.permSvc.InvalidateUserPermissionCache(ctx, targetUserID)

	return nil
}

// GetCurrentUserInfo 获取当前用户完整信息
func (s *UserAdminService) GetCurrentUserInfo(ctx context.Context, userID int64) (*model.User, error) {
	return s.GetUserByID(ctx, userID)
}

// GetUserDomainID 获取用户所属的领域ID
func (s *UserAdminService) GetUserDomainID(ctx context.Context, userID int64) (int64, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, ErrUserNotFound
	}
	return user.DomainID, nil
}

// IsUserInDomain 检查用户是否属于指定领域
func (s *UserAdminService) IsUserInDomain(ctx context.Context, userID, domainID int64) (bool, error) {
	query := `SELECT COUNT(*) FROM bdopsflow_users WHERE id = ? AND domain_id = ?`
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

	count := rowInt(row[0])
	return count > 0, nil
}

// CanDomainAdminManageUser 检查领域管理员是否可以管理目标用户
func (s *UserAdminService) CanDomainAdminManageUser(ctx context.Context, adminUserID, targetUserID int64) (bool, error) {
	adminUser, err := s.GetUserByID(ctx, adminUserID)
	if err != nil {
		return false, err
	}
	if adminUser == nil {
		return false, ErrUserNotFound
	}

	if adminUser.Role == "system_admin" {
		return true, nil
	}

	if adminUser.Role == "domain_admin" {
		return s.IsUserInDomain(ctx, targetUserID, adminUser.DomainID)
	}

	return false, nil
}

// UpdateUserWithDomainCheck 更新用户（带领域权限检查）
func (s *UserAdminService) UpdateUserWithDomainCheck(ctx context.Context, adminUserID, targetUserID int64, username, email, role string, isActive bool) (*model.User, error) {
	canManage, err := s.CanDomainAdminManageUser(ctx, adminUserID, targetUserID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, ErrPermissionDenied
	}

	return s.UpdateUser(ctx, targetUserID, username, email, role, isActive)
}

// ResetUserPasswordWithDomainCheck 重置用户密码（带领域权限检查）
func (s *UserAdminService) ResetUserPasswordWithDomainCheck(ctx context.Context, adminUserID, targetUserID int64, newPasswordB64 string) error {
	canManage, err := s.CanDomainAdminManageUser(ctx, adminUserID, targetUserID)
	if err != nil {
		return err
	}
	if !canManage {
		return ErrPermissionDenied
	}

	return s.ResetUserPassword(ctx, targetUserID, newPasswordB64)
}
