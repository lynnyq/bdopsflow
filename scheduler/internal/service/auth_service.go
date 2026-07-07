package service

import (
	"context"
	"fmt"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// LoginUser 包含登录所需的用户完整信息（含密码哈希）
type LoginUser struct {
	ID             int64
	Username       string
	RealName       string
	Phone          string
	HashedPassword string
	Email          string
	IsActive       bool
}

// UserInfo 包含用户基本信息（不含密码）
type UserInfo struct {
	ID          int64
	Username    string
	RealName    string
	Phone       string
	Email       string
	IsActive    bool
	LastLoginAt *time.Time
}

// AuthService 封装认证相关的数据库操作，避免 handler 层直接操作 DB
type AuthService struct {
	db database.DB
}

// NewAuthService 创建认证服务
func NewAuthService(db database.DB) *AuthService {
	return &AuthService{db: db}
}

// GetUserByUsername 根据用户名查询用户完整信息（含密码哈希），用于登录验证
// 返回 (user, found, error)：found=false 表示用户不存在
func (s *AuthService) GetUserByUsername(ctx context.Context, username string) (*LoginUser, bool, error) {
	query := "SELECT id, username, real_name, phone, password, email, is_active FROM bdopsflow_users WHERE username = ?"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{username},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, false, fmt.Errorf("query user by username failed: %w", err)
	}
	if qr.Err != nil {
		return nil, false, fmt.Errorf("query result error: %w", qr.Err)
	}
	if !qr.Next() {
		return nil, false, nil
	}
	row, err := qr.Slice()
	if err != nil {
		return nil, false, fmt.Errorf("slice row failed: %w", err)
	}
	return &LoginUser{
		ID:             RowInt64(row[0]),
		Username:       RowString(row[1]),
		RealName:       RowString(row[2]),
		Phone:          RowString(row[3]),
		HashedPassword: RowString(row[4]),
		Email:          RowString(row[5]),
		IsActive:       RowBool(row[6]),
	}, true, nil
}

// GetSSOUserByUsername 根据用户名查询 SSO 用户信息（不含密码），用于 SSO 登录
// 返回 (user, found, error)：found=false 表示用户不存在
func (s *AuthService) GetSSOUserByUsername(ctx context.Context, username string) (*UserInfo, bool, error) {
	query := "SELECT id, username, real_name, phone, email, is_active FROM bdopsflow_users WHERE username = ?"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{username},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, false, fmt.Errorf("query sso user by username failed: %w", err)
	}
	if qr.Err != nil {
		return nil, false, fmt.Errorf("query result error: %w", qr.Err)
	}
	if !qr.Next() {
		return nil, false, nil
	}
	row, err := qr.Slice()
	if err != nil {
		return nil, false, fmt.Errorf("slice row failed: %w", err)
	}
	return &UserInfo{
		ID:       RowInt64(row[0]),
		Username: RowString(row[1]),
		RealName: RowString(row[2]),
		Phone:    RowString(row[3]),
		Email:    RowString(row[4]),
		IsActive: RowBool(row[5]),
	}, true, nil
}

// GetUserByID 根据用户 ID 查询用户基本信息（不含密码），用于获取当前登录用户
// 返回 (user, found, error)：found=false 表示用户不存在
func (s *AuthService) GetUserByID(ctx context.Context, userID int64) (*UserInfo, bool, error) {
	query := "SELECT username, real_name, phone, email, is_active, last_login_at FROM bdopsflow_users WHERE id = ?"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, false, fmt.Errorf("query user by id failed: %w", err)
	}
	if qr.Err != nil {
		return nil, false, fmt.Errorf("query result error: %w", qr.Err)
	}
	if !qr.Next() {
		return nil, false, nil
	}
	row, err := qr.Slice()
	if err != nil {
		return nil, false, fmt.Errorf("slice row failed: %w", err)
	}
	info := &UserInfo{
		Username: RowString(row[0]),
		RealName: RowString(row[1]),
		Phone:    RowString(row[2]),
		Email:    RowString(row[3]),
		IsActive: RowBool(row[4]),
	}
	info.LastLoginAt = scanNullTimePtr(row, 5)
	return info, true, nil
}

// GetUserActiveStatus 查询用户激活状态，用于 refresh token 校验
// 返回 (isActive, found, error)：found=false 表示用户不存在
func (s *AuthService) GetUserActiveStatus(ctx context.Context, userID int64) (bool, bool, error) {
	query := "SELECT is_active FROM bdopsflow_users WHERE id = ?"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return false, false, fmt.Errorf("query user active status failed: %w", err)
	}
	if qr.Err != nil {
		return false, false, fmt.Errorf("query result error: %w", qr.Err)
	}
	if !qr.Next() {
		return false, false, nil
	}
	row, err := qr.Slice()
	if err != nil {
		return false, false, fmt.Errorf("slice row failed: %w", err)
	}
	return RowBool(row[0]), true, nil
}

// UpdateLastLogin 更新用户最后登录时间
func (s *AuthService) UpdateLastLogin(ctx context.Context, userID int64) error {
	stmt := rqlite.ParameterizedStatement{
		Query:     "UPDATE bdopsflow_users SET last_login_at = ? WHERE id = ?",
		Arguments: []interface{}{time.Now(), userID},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return fmt.Errorf("update last login failed: %w", err)
	}
	if result.Err != nil {
		return fmt.Errorf("update last login result error: %w", result.Err)
	}
	return nil
}

// CreateUser 创建新用户，返回新用户 ID
func (s *AuthService) CreateUser(ctx context.Context, username, realName, phone, hashedPassword, email string) (int64, error) {
	stmt := rqlite.ParameterizedStatement{
		Query:     "INSERT INTO bdopsflow_users (username, real_name, phone, password, email, is_active, created_at) VALUES (?, ?, ?, ?, ?, 1, ?)",
		Arguments: []interface{}{username, realName, phone, hashedPassword, email, time.Now()},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return 0, fmt.Errorf("create user failed: %w", err)
	}
	if result.Err != nil {
		return 0, fmt.Errorf("create user result error: %w", result.Err)
	}
	return result.LastInsertID, nil
}

// GetRoleIDByCode 根据角色 code 查询角色 ID，未找到返回 0
func (s *AuthService) GetRoleIDByCode(ctx context.Context, code string) (int64, error) {
	stmt := rqlite.ParameterizedStatement{
		Query: "SELECT id FROM bdopsflow_roles WHERE code = ? LIMIT 1",
		Arguments: []interface{}{code},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return 0, fmt.Errorf("query role by code failed: %w", err)
	}
	if qr.Err != nil {
		return 0, fmt.Errorf("query role result error: %w", qr.Err)
	}
	if !qr.Next() {
		return 0, nil
	}
	row, err := qr.Slice()
	if err != nil {
		return 0, fmt.Errorf("slice role row failed: %w", err)
	}
	if len(row) == 0 {
		return 0, nil
	}
	roleID := RowInt64(row[0])
	return roleID, nil
}

// AssignUserRole 为用户分配角色
func (s *AuthService) AssignUserRole(ctx context.Context, userID, roleID int64) error {
	stmt := rqlite.ParameterizedStatement{
		Query:     "INSERT INTO bdopsflow_user_roles (user_id, role_id, created_at) VALUES (?, ?, ?)",
		Arguments: []interface{}{userID, roleID, time.Now()},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		return fmt.Errorf("assign user role failed: %w", err)
	}
	if result.Err != nil {
		return fmt.Errorf("assign user role result error: %w", result.Err)
	}
	return nil
}

// scanNullTimePtr 从行数据中扫描可能为 NULL 的时间字段，返回 *time.Time
func scanNullTimePtr(row []interface{}, idx int) *time.Time {
	if idx < 0 || idx >= len(row) {
		return nil
	}
	t := scanTime(row, idx)
	if t.IsZero() {
		return nil
	}
	return &t
}
