package service

import (
	"context"
	"encoding/base64"
	"fmt"
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
		SELECT id, username, email, domain_id, role, is_active, last_login_at, created_at, updated_at
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

		// 解析 last_login_at
		if t, ok := row[6].(time.Time); ok {
			user.LastLoginAt = &t
		}

		// 解析 created_at
		if t, ok := row[7].(time.Time); ok {
			user.CreatedAt = t
		}

		// 解析 updated_at
		if t, ok := row[8].(time.Time); ok {
			user.UpdatedAt = t
		}

		bdopsflow_users = append(bdopsflow_users, user)
	}

	return bdopsflow_users, nil
}

// GetUserByID 根据ID获取用户
func (s *UserAdminService) GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	query := `
		SELECT id, username, email, domain_id, role, is_active, last_login_at, created_at, updated_at
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

	// 解析 last_login_at
	if t, ok := row[6].(time.Time); ok {
		user.LastLoginAt = &t
	}

	// 解析 created_at
	if t, ok := row[7].(time.Time); ok {
		user.CreatedAt = t
	}

	// 解析 updated_at
	if t, ok := row[8].(time.Time); ok {
		user.UpdatedAt = t
	}

	return user, nil
}

// CreateUser 创建用户
func (s *UserAdminService) CreateUser(ctx context.Context, username, email, password, role string, domainID *int64, createdBy int64) (*model.User, error) {
	// 解码密码
	decodedPassword, err := decodePassword(password)
	if err != nil {
		return nil, err
	}
	// 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(decodedPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO bdopsflow_users (username, email, password, role, domain_id, is_active, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?)
	`

	now := time.Now()
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{username, email, string(hashedPassword), role, domainID, createdBy, now, now},
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

	return s.GetUserByID(ctx, userID)
}

// DeleteUser 删除用户
func (s *UserAdminService) DeleteUser(ctx context.Context, userID int64) error {
	query := `DELETE FROM bdopsflow_users WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	return err
}

// UpdateLastLogin 更新用户最后登录时间
func (s *UserAdminService) UpdateLastLogin(ctx context.Context, userID int64) error {
	query := `UPDATE bdopsflow_users SET last_login_at = ? WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{time.Now(), userID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	return err
}

// decodePassword 解码 Base64 编码的密码
func decodePassword(encodedPassword string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encodedPassword)
	if err != nil {
		return "", fmt.Errorf("invalid password encoding: %w", err)
	}
	return string(decoded), nil
}

// ChangePassword 修改密码
func (s *UserAdminService) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}

	// 解码旧密码
	decodedOldPassword, err := decodePassword(oldPassword)
	if err != nil {
		// 如果解码失败，尝试直接使用原始密码（兼容旧数据）
		decodedOldPassword = oldPassword
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(decodedOldPassword)); err != nil {
		return fmt.Errorf("invalid old password")
	}

	// 解码新密码
	decodedNewPassword, err := decodePassword(newPassword)
	if err != nil {
		return err
	}

	// 哈希新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(decodedNewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := `UPDATE bdopsflow_users SET password = ?, updated_at = ? WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{string(hashedPassword), time.Now(), userID},
	}
	_, err = s.db.WriteOneParameterized(stmt)
	return err
}

// ResetPassword 重置密码（管理员操作）
func (s *UserAdminService) ResetPassword(ctx context.Context, userID int64, newPassword string) error {
	// 解码新密码
	decodedNewPassword, err := decodePassword(newPassword)
	if err != nil {
		return err
	}

	// 哈希新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(decodedNewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := `UPDATE bdopsflow_users SET password = ?, updated_at = ? WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{string(hashedPassword), time.Now(), userID},
	}
	_, err = s.db.WriteOneParameterized(stmt)
	return err
}

// GetUserByUsername 根据用户名获取用户
func (s *UserAdminService) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	query := `
		SELECT id, username, email, domain_id, role, is_active, last_login_at, created_at, updated_at
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
		Email:    rowString(row[2]),
		DomainID: rowInt64(row[3]),
		Role:     rowString(row[4]),
		IsActive: rowBool(row[5]),
	}

	// 解析 last_login_at
	if t, ok := row[6].(time.Time); ok {
		user.LastLoginAt = &t
	}

	// 解析 created_at
	if t, ok := row[7].(time.Time); ok {
		user.CreatedAt = t
	}

	// 解析 updated_at
	if t, ok := row[8].(time.Time); ok {
		user.UpdatedAt = t
	}

	return user, nil
}

// GetUsersByDomain 获取指定领域的所有用户
func (s *UserAdminService) GetUsersByDomain(ctx context.Context, domainID int64) ([]*model.User, error) {
	query := `
		SELECT id, username, email, domain_id, role, is_active, last_login_at, created_at, updated_at
		FROM bdopsflow_users
		WHERE domain_id = ?
		ORDER BY id DESC
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
			Email:    rowString(row[2]),
			DomainID: rowInt64(row[3]),
			Role:     rowString(row[4]),
			IsActive: rowBool(row[5]),
		}

		// 解析 last_login_at
		if t, ok := row[6].(time.Time); ok {
			user.LastLoginAt = &t
		}

		// 解析 created_at
		if t, ok := row[7].(time.Time); ok {
			user.CreatedAt = t
		}

		// 解析 updated_at
		if t, ok := row[8].(time.Time); ok {
			user.UpdatedAt = t
		}

		users = append(users, user)
	}

	return users, nil
}

// GetUserRoles 获取用户角色
func (s *UserAdminService) GetUserRoles(ctx context.Context, userID int64) ([]*model.Role, error) {
	query := `
		SELECT r.id, r.name, r.code, r.description, r.is_system, r.domain_id, r.created_at, r.updated_at
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

		// 解析 domain_id
		if row[5] != nil {
			val := rowInt64(row[5])
			role.DomainID = &val
		}

		// 解析 created_at
		if t, ok := row[6].(time.Time); ok {
			role.CreatedAt = t
		}

		// 解析 updated_at
		if t, ok := row[7].(time.Time); ok {
			role.UpdatedAt = t
		}

		roles = append(roles, role)
	}

	return roles, nil
}

// UpdateUserWithDomainCheck 更新用户（带领域权限检查）
func (s *UserAdminService) UpdateUserWithDomainCheck(ctx context.Context, adminID, userID int64, username, email, role string, isActive bool) (*model.User, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return s.UpdateUser(ctx, userID, username, email, role, isActive)
}

// AssignUserRoles 分配用户角色
func (s *UserAdminService) AssignUserRoles(ctx context.Context, userID int64, roleIDs, domainIDs []int64) error {
	query := `DELETE FROM bdopsflow_user_roles WHERE user_id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	for _, roleID := range roleIDs {
		query := `INSERT INTO bdopsflow_user_roles (user_id, role_id, created_at) VALUES (?, ?, ?)`
		stmt := rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: []interface{}{userID, roleID, time.Now()},
		}
		_, err := s.db.WriteOneParameterized(stmt)
		if err != nil {
			return err
		}
	}

	return nil
}

// AssignUserDomains 分配用户领域
func (s *UserAdminService) AssignUserDomains(ctx context.Context, userID int64, domainIDs []int64) error {
	query := `UPDATE bdopsflow_users SET domain_id = ? WHERE id = ?`
	var domainID *int64
	if len(domainIDs) > 0 {
		domainID = &domainIDs[0]
	}
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{domainID, userID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	return err
}

// GetCurrentUserInfo 获取当前用户信息
func (s *UserAdminService) GetCurrentUserInfo(ctx context.Context, userID int64) (*model.User, error) {
	return s.GetUserByID(ctx, userID)
}

// UpdateCurrentUser 更新当前用户信息
func (s *UserAdminService) UpdateCurrentUser(ctx context.Context, userID int64, email string) (*model.User, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	return s.UpdateUser(ctx, userID, user.Username, email, user.Role, user.IsActive)
}

// ResetUserPasswordWithDomainCheck 重置用户密码（带领域权限检查）
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
