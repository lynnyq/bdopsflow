package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/rsautil"
	rqlite "github.com/rqlite/gorqlite"
)

// APITokenService API Token服务
type APITokenService struct {
	db      database.DB
	rsaUtil *rsautil.RSAUtil
	permSvc *PermissionService
}

// NewAPITokenService 创建API Token服务实例
func NewAPITokenService(db database.DB, rsaUtil *rsautil.RSAUtil, permSvc *PermissionService) *APITokenService {
	return &APITokenService{
		db:      db,
		rsaUtil: rsaUtil,
		permSvc: permSvc,
	}
}

// GenerateToken 为用户生成API Token，每个用户只保留一个有效Token
func (s *APITokenService) GenerateToken(ctx context.Context, userID int64) (string, *model.APIToken, error) {
	// 生成随机Token: bdf_ + 32字节随机hex字符串
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		slog.Error("GenerateToken: failed to generate random bytes", "error", err, "user_id", userID)
		return "", nil, fmt.Errorf("生成随机Token失败: %w", err)
	}
	tokenString := "bdf_" + hex.EncodeToString(tokenBytes)

	// 计算token_prefix（前8个字符，用于快速查找）
	tokenPrefix := tokenString[:8]

	// 加密Token
	tokenEncrypted, err := s.rsaUtil.EncryptLarge(tokenString)
	if err != nil {
		slog.Error("GenerateToken: failed to encrypt token", "error", err, "user_id", userID)
		return "", nil, fmt.Errorf("加密Token失败: %w", err)
	}

	// 删除用户旧的Token
	deleteStmt := rqlite.ParameterizedStatement{
		Query:     `DELETE FROM bdopsflow_api_tokens WHERE user_id = ?`,
		Arguments: []interface{}{userID},
	}
	_, err = s.db.WriteOneParameterized(deleteStmt)
	if err != nil {
		slog.Error("GenerateToken: failed to delete old token", "error", err, "user_id", userID)
		return "", nil, fmt.Errorf("删除旧Token失败: %w", err)
	}

	// 插入新Token
	now := time.Now()
	insertStmt := rqlite.ParameterizedStatement{
		Query: `INSERT INTO bdopsflow_api_tokens (user_id, token_encrypted, token_prefix, created_at) VALUES (?, ?, ?, ?)`,
		Arguments: []interface{}{
			userID,
			tokenEncrypted,
			tokenPrefix,
			now.Format(DateTimeFormat),
		},
	}
	result, err := s.db.WriteOneParameterized(insertStmt)
	if err != nil {
		slog.Error("GenerateToken: failed to insert token", "error", err, "user_id", userID)
		return "", nil, fmt.Errorf("插入Token失败: %w", err)
	}
	if result.Err != nil {
		slog.Error("GenerateToken: insert result error", "error", result.Err, "user_id", userID)
		return "", nil, fmt.Errorf("插入Token失败: %w", result.Err)
	}

	apiToken := &model.APIToken{
		ID:             result.LastInsertID,
		UserID:         userID,
		TokenEncrypted: tokenEncrypted,
		TokenPrefix:    tokenPrefix,
		CreatedAt:      now,
	}

	slog.Info("GenerateToken: success", "user_id", userID, "token_id", apiToken.ID)
	return tokenString, apiToken, nil
}

// GetTokenInfo 获取用户的Token信息（不解密）
func (s *APITokenService) GetTokenInfo(ctx context.Context, userID int64) (*model.APIToken, error) {
	query := `SELECT id, user_id, token_encrypted, token_prefix, last_used_at, created_at FROM bdopsflow_api_tokens WHERE user_id = ?`
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
		return nil, ErrAPITokenNotFound
	}

	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	apiToken := &model.APIToken{
		ID:             rowInt64(row[0]),
		UserID:         rowInt64(row[1]),
		TokenEncrypted: rowString(row[2]),
		TokenPrefix:    rowString(row[3]),
		CreatedAt:      parseDateTime(row[5]),
	}

	if !isEmpty(row[4]) {
		t := parseDateTime(row[4])
		if !t.IsZero() {
			apiToken.LastUsedAt = &t
		}
	}

	return apiToken, nil
}

// RevealToken 解密并返回用户的明文Token
func (s *APITokenService) RevealToken(ctx context.Context, userID int64) (string, error) {
	tokenInfo, err := s.GetTokenInfo(ctx, userID)
	if err != nil {
		return "", err
	}
	if tokenInfo == nil {
		return "", ErrAPITokenNotFound
	}

	plaintext, err := s.rsaUtil.DecryptLarge(tokenInfo.TokenEncrypted)
	if err != nil {
		slog.Error("RevealToken: failed to decrypt token", "error", err, "user_id", userID)
		return "", fmt.Errorf("解密Token失败: %w", err)
	}

	return plaintext, nil
}

// RevokeToken 撤销用户的API Token
func (s *APITokenService) RevokeToken(ctx context.Context, userID int64) error {
	// 先检查Token是否存在
	tokenInfo, err := s.GetTokenInfo(ctx, userID)
	if err != nil {
		return err
	}
	if tokenInfo == nil {
		return ErrAPITokenNotFound
	}

	stmt := rqlite.ParameterizedStatement{
		Query:     `DELETE FROM bdopsflow_api_tokens WHERE user_id = ?`,
		Arguments: []interface{}{userID},
	}
	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("RevokeToken: failed to delete token", "error", err, "user_id", userID)
		return fmt.Errorf("撤销Token失败: %w", err)
	}
	if result.Err != nil {
		slog.Error("RevokeToken: delete result error", "error", result.Err, "user_id", userID)
		return fmt.Errorf("撤销Token失败: %w", result.Err)
	}

	slog.Info("RevokeToken: success", "user_id", userID)
	return nil
}

// ValidateToken 验证API Token，返回对应的用户ID
func (s *APITokenService) ValidateToken(ctx context.Context, tokenString string) (int64, error) {
	// 检查Token前缀
	if len(tokenString) < 8 || tokenString[:4] != "bdf_" {
		return 0, ErrAPITokenInvalid
	}

	// 查询所有Token记录，逐一解密比对
	// 由于每用户只有一个Token，记录总数有限，且RSA解密比对保证安全性
	query := `SELECT id, user_id, token_encrypted, token_prefix, last_used_at, created_at FROM bdopsflow_api_tokens`
	qr, err := s.db.QueryOneParameterized(rqlite.ParameterizedStatement{Query: query})
	if err != nil {
		return 0, err
	}
	if qr.Err != nil {
		return 0, qr.Err
	}

	var matchedTokenID int64
	var matchedUserID int64

	for qr.Next() {
		row, rowErr := qr.Slice()
		if rowErr != nil {
			continue
		}

		tokenID := rowInt64(row[0])
		userID := rowInt64(row[1])
		tokenEncrypted := rowString(row[2])

		// 解密Token并比对
		plaintext, decryptErr := s.rsaUtil.DecryptLarge(tokenEncrypted)
		if decryptErr != nil {
			slog.Error("ValidateToken: failed to decrypt token", "error", decryptErr, "token_id", tokenID)
			continue
		}

		if plaintext == tokenString {
			matchedTokenID = tokenID
			matchedUserID = userID
			break
		}
	}

	if matchedUserID == 0 {
		return 0, ErrAPITokenInvalid
	}

	// 检查用户是否激活
	userQuery := `SELECT is_active FROM bdopsflow_users WHERE id = ?`
	userStmt := rqlite.ParameterizedStatement{
		Query:     userQuery,
		Arguments: []interface{}{matchedUserID},
	}
	userQR, err := s.db.QueryOneParameterized(userStmt)
	if err != nil {
		return 0, err
	}
	if userQR.Err != nil {
		return 0, userQR.Err
	}

	if !userQR.Next() {
		return 0, ErrAPITokenInvalid
	}

	userRow, err := userQR.Slice()
	if err != nil {
		return 0, err
	}

	if !rowBool(userRow[0]) {
		return 0, ErrUserInactive
	}

	// 异步更新last_used_at
	go func() {
		updateStmt := rqlite.ParameterizedStatement{
			Query: `UPDATE bdopsflow_api_tokens SET last_used_at = ? WHERE id = ?`,
			Arguments: []interface{}{
				time.Now().Format(DateTimeFormat),
				matchedTokenID,
			},
		}
		if _, err := s.db.WriteOneParameterized(updateStmt); err != nil {
			slog.Error("ValidateToken: failed to update last_used_at", "error", err, "token_id", matchedTokenID)
		}
	}()

	return matchedUserID, nil
}

// GetTokenUserInfo 获取用户信息（供API Token认证中间件使用）
func (s *APITokenService) GetTokenUserInfo(ctx context.Context, userID int64) (username string, realName string, domainID int64, err error) {
	query := `SELECT username, real_name FROM bdopsflow_users WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{userID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return "", "", 0, err
	}
	if qr.Err != nil {
		return "", "", 0, qr.Err
	}

	if !qr.Next() {
		return "", "", 0, ErrUserNotFound
	}

	row, err := qr.Slice()
	if err != nil {
		return "", "", 0, err
	}

	username = rowString(row[0])
	realName = rowString(row[1])

	// 获取默认领域
	if s.permSvc != nil {
		defaultDomainID, defaultErr := s.permSvc.GetUserDefaultDomain(ctx, userID)
		if defaultErr != nil {
			slog.Warn("GetTokenUserInfo: failed to get default domain", "error", defaultErr, "user_id", userID)
		}
		if defaultDomainID > 0 {
			domainID = defaultDomainID
		}
	}

	return username, realName, domainID, nil
}
