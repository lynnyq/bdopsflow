package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

type ProtoService struct {
	db database.DB
}

func NewProtoService(db database.DB) *ProtoService {
	return &ProtoService{db: db}
}

// Create inserts a new proto file record
func (s *ProtoService) Create(ctx context.Context, pf *model.ProtoFile) (*model.ProtoFile, error) {
	hash := computeFileHash(pf.Content)

	dependencies := pf.Dependencies
	if dependencies == "" {
		dependencies = "[]"
	}

	query := `
		INSERT INTO bdopsflow_proto_files (name, content, file_hash, parsed_result, dependencies, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			pf.Name,
			pf.Content,
			hash,
			pf.ParsedResult,
			dependencies,
			pf.CreatedBy,
			now,
			now,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to create proto file", "error", err, "name", pf.Name)
		return nil, err
	}
	if result.Err != nil {
		slog.Error("proto file create result error", "error", result.Err, "name", pf.Name)
		return nil, result.Err
	}

	pf.ID = result.LastInsertID
	pf.FileHash = hash
	pf.Dependencies = dependencies
	parsedTime, parseErr := time.Parse(DateTimeFormat, now)
	if parseErr != nil {
		slog.Warn("failed to parse proto file created_at time, using zero value", "error", parseErr)
		parsedTime = time.Now()
	}
	pf.CreatedAt = parsedTime
	pf.UpdatedAt = pf.CreatedAt
	return pf, nil
}

// Update modifies an existing proto file record
func (s *ProtoService) Update(ctx context.Context, id int64, pf *model.ProtoFile) error {
	hash := computeFileHash(pf.Content)

	query := `
		UPDATE bdopsflow_proto_files SET name = ?, content = ?, file_hash = ?, parsed_result = ?, dependencies = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			pf.Name,
			pf.Content,
			hash,
			pf.ParsedResult,
			pf.Dependencies,
			now,
			id,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to update proto file", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("proto file update result error", "error", result.Err, "id", id)
		return result.Err
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("proto file not found")
	}

	pf.FileHash = hash
	return nil
}

// Delete removes a proto file record by id
func (s *ProtoService) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM bdopsflow_proto_files WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to delete proto file", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("proto file delete result error", "error", result.Err, "id", id)
		return result.Err
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("proto file not found")
	}

	slog.Info("proto file deleted", "id", id)
	return nil
}

// GetByID retrieves a single proto file by ID
func (s *ProtoService) GetByID(ctx context.Context, id int64) (*model.ProtoFile, error) {
	query := `SELECT id, name, content, file_hash, parsed_result, dependencies, created_by, created_at, updated_at FROM bdopsflow_proto_files WHERE id = ?`
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
		return nil, fmt.Errorf("proto file not found")
	}

	row, err := qr.Slice()
	if err != nil {
		return nil, err
	}

	return scanProtoFile(row), nil
}

// ListByUser returns proto files with pagination.
// System admin can see all proto files; other users can only see their own.
// ListByUser 查询 Proto 文件列表，search 为可选参数，支持按名称模糊匹配
func (s *ProtoService) ListByUser(ctx context.Context, userID int64, isAdmin bool, page, pageSize int, search ...string) ([]*model.ProtoFile, int64, error) {
	var conditions []string
	var args []interface{}

	// 管理员可查看所有记录，普通用户和领域管理员只能查看自己创建的
	if !isAdmin {
		conditions = append(conditions, "p.created_by = ?")
		args = append(args, userID)
	}

	if len(search) > 0 && search[0] != "" {
		conditions = append(conditions, "p.name LIKE ?")
		args = append(args, "%"+search[0]+"%")
	}

	whereClause := "WHERE 1=1"
	if len(conditions) > 0 {
		whereClause += " AND " + strings.Join(conditions, " AND ")
	}

	// Count total records
	countQuery := "SELECT COUNT(*) FROM bdopsflow_proto_files p " + whereClause
	countStmt := rqlite.ParameterizedStatement{
		Query:     countQuery,
		Arguments: args,
	}
	countQR, err := s.db.QueryOneParameterized(countStmt)
	if err != nil {
		return nil, 0, err
	}
	if countQR.Err != nil {
		return nil, 0, countQR.Err
	}

	var total int64
	if countQR.Next() {
		row, _ := countQR.Slice()
		total = rowInt64(row[0])
	}

	// Fetch paginated data with creator name
	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(
		`SELECT p.id, p.name, p.content, p.file_hash, p.parsed_result, p.dependencies, p.created_by,
		        COALESCE(u.real_name, '') as created_by_name,
		        p.created_at, p.updated_at
		 FROM bdopsflow_proto_files p
		 LEFT JOIN bdopsflow_users u ON p.created_by = u.id
		 %s ORDER BY p.created_at DESC LIMIT %d OFFSET %d`,
		whereClause, pageSize, offset,
	)
	dataStmt := rqlite.ParameterizedStatement{
		Query:     dataQuery,
		Arguments: args,
	}
	dataQR, err := s.db.QueryOneParameterized(dataStmt)
	if err != nil {
		return nil, 0, err
	}
	if dataQR.Err != nil {
		return nil, 0, dataQR.Err
	}

	var protoFiles []*model.ProtoFile
	for dataQR.Next() {
		row, err := dataQR.Slice()
		if err != nil {
			slog.Warn("failed to slice proto file row", "error", err)
			continue
		}
		pf := scanProtoFile(row)
		// 第9列（索引8）是 created_by_name
		if len(row) > 8 {
			pf.CreatedByName = rowString(row[8])
		}
		protoFiles = append(protoFiles, pf)
	}

	return protoFiles, total, nil
}

// ParseProto parses proto file content to extract package name, services, and messages
func (s *ProtoService) ParseProto(ctx context.Context, content string, dependencies []string) (*model.ProtoParseResult, error) {
	result := &model.ProtoParseResult{}

	// Extract package name
	pkgRe := regexp.MustCompile(`package\s+([\w.]+)`)
	if matches := pkgRe.FindStringSubmatch(content); len(matches) > 1 {
		result.Package = matches[1]
	}

	// Extract message names
	msgRe := regexp.MustCompile(`message\s+(\w+)`)
	msgMatches := msgRe.FindAllStringSubmatch(content, -1)
	for _, m := range msgMatches {
		if len(m) > 1 {
			result.Messages = append(result.Messages, m[1])
		}
	}

	// Extract services and their methods
	svcRe := regexp.MustCompile(`service\s+(\w+)\s*\{`)
	svcMatches := svcRe.FindAllStringSubmatchIndex(content, -1)

	methodRe := regexp.MustCompile(`rpc\s+(\w+)\s*\(\s*([\w.]+)\s*\)\s*returns\s*\(\s*([\w.]+)\s*\)`)

	for _, svcIdx := range svcMatches {
		if len(svcIdx) < 4 {
			continue
		}
		svcName := content[svcIdx[2]:svcIdx[3]]

		// Find the service body (from opening brace to closing brace)
		svcBodyStart := svcIdx[1]
		svcBody := extractBlock(content, svcBodyStart)

		protoSvc := model.ProtoService{
			Name: svcName,
		}

		methodMatches := methodRe.FindAllStringSubmatch(svcBody, -1)
		for _, mm := range methodMatches {
			if len(mm) > 3 {
				protoSvc.Methods = append(protoSvc.Methods, model.ProtoMethod{
					Name:       mm[1],
					InputType:  mm[2],
					OutputType: mm[3],
				})
			}
		}

		result.Services = append(result.Services, protoSvc)
	}

	return result, nil
}

// ParseAndSave parses the proto file by ID and saves the parsed result as JSON
func (s *ProtoService) ParseAndSave(ctx context.Context, id int64) error {
	pf, err := s.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get proto file: %w", err)
	}

	var dependencies []string
	if pf.Dependencies != "" {
		if jsonErr := json.Unmarshal([]byte(pf.Dependencies), &dependencies); jsonErr != nil {
			slog.Warn("failed to parse dependencies, using empty list", "error", jsonErr, "id", id)
			dependencies = nil
		}
	}

	parseResult, err := s.ParseProto(ctx, pf.Content, dependencies)
	if err != nil {
		return fmt.Errorf("failed to parse proto file: %w", err)
	}

	resultJSON, err := json.Marshal(parseResult)
	if err != nil {
		return fmt.Errorf("failed to marshal parse result: %w", err)
	}

	query := `UPDATE bdopsflow_proto_files SET parsed_result = ?, updated_at = ? WHERE id = ?`
	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			string(resultJSON),
			now,
			id,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to save parsed result", "error", err, "id", id)
		return err
	}
	if result.Err != nil {
		slog.Error("save parsed result error", "error", result.Err, "id", id)
		return result.Err
	}

	return nil
}

// computeFileHash computes the SHA256 hash of the file content
func computeFileHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

// extractBlock extracts a brace-delimited block starting from the position after '{'
func extractBlock(content string, start int) string {
	depth := 0
	i := start
	for i < len(content) {
		ch := content[i]
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return content[start : i+1]
			}
		}
		i++
	}
	return content[start:]
}

// scanProtoFile maps a database row to a ProtoFile struct
func scanProtoFile(row []interface{}) *model.ProtoFile {
	pf := &model.ProtoFile{
		ID:           rowInt64(row[0]),
		Name:         rowString(row[1]),
		Content:      rowString(row[2]),
		FileHash:     rowString(row[3]),
		ParsedResult: rowString(row[4]),
		Dependencies: rowString(row[5]),
		CreatedBy:    rowInt64(row[6]),
	}
	if !isEmpty(row[7]) {
		pf.CreatedAt = parseDateTime(row[7])
	}
	if !isEmpty(row[8]) {
		pf.UpdatedAt = parseDateTime(row[8])
	}
	return pf
}
