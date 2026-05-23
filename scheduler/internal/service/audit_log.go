package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

type AuditLogService struct {
	db *rqlite.Connection
}

func NewAuditLogService(db *rqlite.Connection) *AuditLogService {
	return &AuditLogService{db: db}
}

func (s *AuditLogService) Create(ctx context.Context, log *model.AuditLog) error {
	query := `
		INSERT INTO bdopsflow_audit_logs (user_id, username, real_name, role, domain_id, action, resource, resource_id, resource_name, status, ip_address, user_agent, request_method, request_path, detail, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			log.UserID,
			log.Username,
			log.RealName,
			log.Role,
			log.DomainID,
			log.Action,
			log.Resource,
			log.ResourceID,
			log.ResourceName,
			log.Status,
			log.IPAddress,
			log.UserAgent,
			log.RequestMethod,
			log.RequestPath,
			log.Detail,
			log.CreatedAt,
		},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to write audit log", "error", err, "action", log.Action, "resource", log.Resource)
		return err
	}
	if result.Err != nil {
		slog.Error("audit log write result error", "error", result.Err, "action", log.Action, "resource", log.Resource)
		return result.Err
	}

	return nil
}

func (s *AuditLogService) List(ctx context.Context, filter model.AuditLogFilter) ([]model.AuditLog, int64, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	var conditions []string
	var args []interface{}

	if filter.Username != "" {
		conditions = append(conditions, "username LIKE ?")
		args = append(args, "%"+filter.Username+"%")
	}
	if filter.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, filter.Action)
	}
	if filter.Resource != "" {
		conditions = append(conditions, "resource = ?")
		args = append(args, filter.Resource)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.StartTime != "" {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, filter.StartTime)
	}
	if filter.EndTime != "" {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, filter.EndTime)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM bdopsflow_audit_logs %s", whereClause)
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

	offset := (filter.Page - 1) * filter.PageSize
	dataQuery := fmt.Sprintf(
		"SELECT id, user_id, username, real_name, role, domain_id, action, resource, resource_id, resource_name, status, ip_address, user_agent, request_method, request_path, detail, created_at FROM bdopsflow_audit_logs %s ORDER BY created_at DESC LIMIT %d OFFSET %d",
		whereClause, filter.PageSize, offset,
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

	var logs []model.AuditLog
	for dataQR.Next() {
		row, err := dataQR.Slice()
		if err != nil {
			continue
		}

		auditLog := model.AuditLog{
			ID:            rowInt64(row[0]),
			Username:      rowString(row[2]),
			RealName:      rowString(row[3]),
			Role:          rowString(row[4]),
			Action:        rowString(row[6]),
			Resource:      rowString(row[7]),
			ResourceID:    rowString(row[8]),
			ResourceName:  rowString(row[9]),
			Status:        rowString(row[10]),
			IPAddress:     rowString(row[11]),
			UserAgent:     rowString(row[12]),
			RequestMethod: rowString(row[13]),
			RequestPath:   rowString(row[14]),
			Detail:        rowString(row[15]),
		}

		if !isEmpty(row[1]) {
			userID := rowInt64(row[1])
			auditLog.UserID = &userID
		}
		if !isEmpty(row[5]) {
			domainID := rowInt64(row[5])
			auditLog.DomainID = &domainID
		}
		if !isEmpty(row[16]) {
			auditLog.CreatedAt = parseDateTime(row[16])
		}

		logs = append(logs, auditLog)
	}

	return logs, total, nil
}

func (s *AuditLogService) CleanExpired(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 90
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays).Format(DateTimeFormat)

	query := `DELETE FROM bdopsflow_audit_logs WHERE created_at < ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{cutoffTime},
	}

	result, err := s.db.WriteOneParameterized(stmt)
	if err != nil {
		slog.Error("failed to clean expired audit logs", "error", err, "retention_days", retentionDays)
		return 0, err
	}
	if result.Err != nil {
		slog.Error("audit log clean result error", "error", result.Err, "retention_days", retentionDays)
		return 0, result.Err
	}

	deleted := result.RowsAffected
	slog.Info("cleaned expired audit logs", "deleted_count", deleted, "retention_days", retentionDays, "cutoff_time", cutoffTime)
	return deleted, nil
}

func (s *AuditLogService) GetRetentionDays() int {
	query := `SELECT config_value FROM bdopsflow_system_config WHERE config_key = 'audit_log.retention_days'`
	qr, err := s.db.QueryOne(query)
	if err != nil || qr.Err != nil {
		return 90
	}

	if qr.Next() {
		row, _ := qr.Slice()
		val := rowString(row[0])
		if days, err := strconv.Atoi(val); err == nil && days > 0 {
			return days
		}
	}

	return 90
}
