package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/rsautil"
	rqlite "github.com/rqlite/gorqlite"
)

type CertificateService struct {
	db      database.DB
	rsaUtil *rsautil.RSAUtil
}

func NewCertificateService(db database.DB, rsaUtil *rsautil.RSAUtil) *CertificateService {
	return &CertificateService{db: db, rsaUtil: rsaUtil}
}

// Create inserts a new certificate. ClientKey is encrypted at rest if provided.
func (s *CertificateService) Create(ctx context.Context, cert *model.Certificate) (*model.Certificate, error) {
	clientKey := cert.ClientKey
	if clientKey != "" {
		encrypted, err := s.rsaUtil.EncryptLarge(clientKey)
		if err != nil {
			slog.Error("failed to encrypt client key", "error", err)
			return nil, fmt.Errorf("failed to encrypt client key: %w", err)
		}
		clientKey = encrypted
	}

	query := `
		INSERT INTO bdopsflow_certificates (name, ca_cert, client_cert, client_key, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			cert.Name,
			cert.CaCert,
			cert.ClientCert,
			clientKey,
			cert.CreatedBy,
			now,
			now,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to create certificate", "error", err, "name", cert.Name)
		return nil, err
	}
	if result.Err != nil {
		slog.Error("certificate create result error", "error", result.Err, "name", cert.Name)
		return nil, result.Err
	}

	cert.ID = result.LastInsertID
	parsedTime, parseErr := time.Parse(DateTimeFormat, now)
	if parseErr != nil {
		slog.Warn("failed to parse certificate created_at time, using zero value", "error", parseErr)
		parsedTime = time.Now()
	}
	cert.CreatedAt = parsedTime
	cert.UpdatedAt = cert.CreatedAt
	return cert, nil
}

// Update modifies an existing certificate. ClientKey is re-encrypted if provided.
// If ClientKey is empty, the existing client_key is preserved in the database.
func (s *CertificateService) Update(ctx context.Context, id int64, cert *model.Certificate) error {
	now := time.Now().Format(DateTimeFormat)

	var stmt rqlite.ParameterizedStatement

	if cert.ClientKey != "" {
		encrypted, err := s.rsaUtil.EncryptLarge(cert.ClientKey)
		if err != nil {
			slog.Error("failed to encrypt client key on update", "error", err, "id", id)
			return fmt.Errorf("failed to encrypt client key: %w", err)
		}
		query := `
			UPDATE bdopsflow_certificates SET name = ?, ca_cert = ?, client_cert = ?, client_key = ?, updated_at = ?
			WHERE id = ?
		`
		stmt = rqlite.ParameterizedStatement{
			Query: query,
			Arguments: []interface{}{
				cert.Name,
				cert.CaCert,
				cert.ClientCert,
				encrypted,
				now,
				id,
			},
		}
	} else {
		query := `
			UPDATE bdopsflow_certificates SET name = ?, ca_cert = ?, client_cert = ?, updated_at = ?
			WHERE id = ?
		`
		stmt = rqlite.ParameterizedStatement{
			Query: query,
			Arguments: []interface{}{
				cert.Name,
				cert.CaCert,
				cert.ClientCert,
				now,
				id,
			},
		}
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to update certificate", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("certificate update result error", "error", result.Err, "id", id)
		return result.Err
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("certificate not found")
	}

	return nil
}

// Delete removes a certificate by id.
func (s *CertificateService) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM bdopsflow_certificates WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to delete certificate", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("certificate delete result error", "error", result.Err, "id", id)
		return result.Err
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("certificate not found")
	}

	slog.Info("certificate deleted", "id", id)
	return nil
}

// GetByID retrieves a certificate by ID. ClientKey is decrypted before returning.
func (s *CertificateService) GetByID(ctx context.Context, id int64) (*model.Certificate, error) {
	query := `SELECT id, name, ca_cert, client_cert, client_key, created_by, created_at, updated_at FROM bdopsflow_certificates WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}

	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("certificate not found")
	}

	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	cert := &model.Certificate{
		ID:         rowInt64(row[0]),
		Name:       rowString(row[1]),
		CaCert:     rowString(row[2]),
		ClientCert: rowString(row[3]),
		ClientKey:  rowString(row[4]),
		CreatedBy:  rowInt64(row[5]),
		CreatedAt:  parseDateTime(row[6]),
		UpdatedAt:  parseDateTime(row[7]),
	}

	// Decrypt ClientKey so the executor can use it
	// Support both old format (hex-only RSA ciphertext) and new format (hex.base64 AES-GCM ciphertext)
	if cert.ClientKey != "" {
		var decrypted string
		var err error
		if strings.Contains(cert.ClientKey, ".") {
			// New format: AES-GCM hybrid encryption
			decrypted, err = s.rsaUtil.DecryptLarge(cert.ClientKey)
		} else {
			// Old format: RSA-only encryption
			decrypted, err = s.rsaUtil.Decrypt(cert.ClientKey)
		}
		if err != nil {
			slog.Error("failed to decrypt client key", "error", err, "id", id)
			return nil, fmt.Errorf("failed to decrypt client key: %w", err)
		}
		cert.ClientKey = decrypted
	}

	return cert, nil
}

// ListByUser returns certificates with pagination.
// System admin can see all certificates; other users can only see their own.
// Only metadata is returned (no sensitive certificate content).
// ListByUser 查询证书列表，search 为可选参数，支持按名称模糊匹配
func (s *CertificateService) ListByUser(ctx context.Context, userID int64, isAdmin bool, page, pageSize int, search ...string) ([]*model.CertificateSummary, int64, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if page <= 0 {
		page = 1
	}

	var conditions []string
	var args []interface{}

	// 管理员可查看所有记录，普通用户和领域管理员只能查看自己创建的
	if !isAdmin {
		conditions = append(conditions, "c.created_by = ?")
		args = append(args, userID)
	}

	if len(search) > 0 && search[0] != "" {
		conditions = append(conditions, "c.name LIKE ?")
		args = append(args, "%"+search[0]+"%")
	}

	var whereClause string
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM bdopsflow_certificates c %s", whereClause)
	countStmt := rqlite.ParameterizedStatement{
		Query:     countQuery,
		Arguments: args,
	}
	countQr, err := s.db.QueryOneParameterized(countStmt)
	if err != nil {
		slog.Error("failed to count certificates", "error", err, "user_id", userID)
		return nil, 0, err
	}
	if countQr.Err != nil {
		slog.Error("certificate count query error", "error", countQr.Err, "user_id", userID)
		return nil, 0, countQr.Err
	}

	var total int64
	if countQr.Next() {
		row, sliceErr := countQr.Slice()
		if sliceErr != nil {
			slog.Error("failed to read certificate count row", "error", sliceErr)
			return nil, 0, sliceErr
		}
		total = rowInt64(row[0])
	}

	// Query data with pagination, join users table for creator name
	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(`
		SELECT c.id, c.name, c.ca_cert, c.client_cert, c.client_key, c.created_by,
		       COALESCE(u.real_name, '') as created_by_name,
		       c.created_at, c.updated_at
		FROM bdopsflow_certificates c
		LEFT JOIN bdopsflow_users u ON c.created_by = u.id
		%s ORDER BY c.created_at DESC LIMIT ? OFFSET ?
	`, whereClause)
	dataStmt := rqlite.ParameterizedStatement{
		Query:     dataQuery,
		Arguments: append(args, pageSize, offset),
	}

	qr, err := s.db.QueryOneParameterized(dataStmt)
	if err != nil {
		slog.Error("failed to list certificates", "error", err, "user_id", userID)
		return nil, 0, err
	}
	if qr.Err != nil {
		slog.Error("certificate list query error", "error", qr.Err, "user_id", userID)
		return nil, 0, qr.Err
	}

	var summaries []*model.CertificateSummary
	for qr.Next() {
		row, sliceErr := qr.Slice()
		if sliceErr != nil {
			slog.Warn("failed to read certificate row", "error", sliceErr)
			continue
		}

		summary := &model.CertificateSummary{
			ID:            rowInt64(row[0]),
			Name:          rowString(row[1]),
			HasCACert:     rowString(row[2]) != "",
			HasClientCert: rowString(row[3]) != "",
			HasClientKey:  rowString(row[4]) != "",
			CreatedBy:     rowInt64(row[5]),
			CreatedByName: rowString(row[6]),
			CreatedAt:     parseDateTime(row[7]),
			UpdatedAt:     parseDateTime(row[8]),
		}

		summaries = append(summaries, summary)
	}

	return summaries, total, nil
}
