package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

type AuditLogService struct {
	db           database.DB
	configService *sysconfig.Service
}

// NewAuditLogService 创建审计日志服务。
// configService 用于读取 audit_log.retention_days 配置（支持热更新），可为 nil（回退到默认 90 天）。
func NewAuditLogService(db database.DB, configService *sysconfig.Service) *AuditLogService {
	return &AuditLogService{db: db, configService: configService}
}

func (s *AuditLogService) Create(ctx context.Context, log *model.AuditLog) error {
	query := `
		INSERT INTO bdopsflow_audit_logs (user_id, username, real_name, role, domain_id, action, resource, resource_id, resource_name, status, response_code, ip_address, user_agent, request_method, request_path, detail, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			log.ResponseCode,
			log.IPAddress,
			log.UserAgent,
			log.RequestMethod,
			log.RequestPath,
			log.Detail,
			log.CreatedAt.Format(DateTimeFormat),
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
	if filter.DomainID > 0 {
		conditions = append(conditions, "domain_id = ?")
		args = append(args, filter.DomainID)
	}
	if filter.StartTime != "" {
		if t, err := parseTimeInLocalTimezone(filter.StartTime); err == nil {
			conditions = append(conditions, "created_at >= ?")
			args = append(args, t.Format(DateTimeFormat))
		}
	}
	if filter.EndTime != "" {
		if t, err := parseTimeInLocalTimezone(filter.EndTime); err == nil {
			conditions = append(conditions, "created_at <= ?")
			args = append(args, t.Format(DateTimeFormat))
		}
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
	// 注：rqlite 对 LIMIT/OFFSET 参数化支持有限，此处使用 %d 拼接。
	// PageSize 已在上方校验为 1-100 的整数，offset 为整数运算结果，无注入风险。
	dataQuery := fmt.Sprintf(
		"SELECT id, user_id, username, real_name, role, domain_id, action, resource, resource_id, resource_name, status, response_code, ip_address, user_agent, request_method, request_path, detail, created_at FROM bdopsflow_audit_logs %s ORDER BY created_at DESC LIMIT %d OFFSET %d",
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

		// 列顺序：0=id 1=user_id 2=username 3=real_name 4=role 5=domain_id
		//         6=action 7=resource 8=resource_id 9=resource_name 10=status
		//         11=response_code 12=ip_address 13=user_agent 14=request_method
		//         15=request_path 16=detail 17=created_at
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
			ResponseCode:  int(rowInt64(row[11])),
			IPAddress:     rowString(row[12]),
			UserAgent:     rowString(row[13]),
			RequestMethod: rowString(row[14]),
			RequestPath:   rowString(row[15]),
			Detail:        rowString(row[16]),
		}

		if !isEmpty(row[1]) {
			userID := rowInt64(row[1])
			auditLog.UserID = &userID
		}
		if !isEmpty(row[5]) {
			domainID := rowInt64(row[5])
			auditLog.DomainID = &domainID
		}
		if !isEmpty(row[17]) {
			auditLog.CreatedAt = parseDateTime(row[17])
		}

		logs = append(logs, auditLog)
	}

	return logs, total, nil
}

// CleanExpired 清理超过保留天数的审计日志。
// retentionDays 由 handler 层确保 > 0（默认从系统配置读取，回退到 90），
// service 层不再做默认值回退，避免与 handler 层重复。
// 分批删除（每批 1000 条），避免大表 DELETE 锁库时间过长。
//
// 实现说明：rqlite 基于 SQLite，默认不支持 `DELETE ... LIMIT` 语法
// （需要编译时启用 SQLITE_ENABLE_UPDATE_DELETE_LIMIT 选项），
// 因此采用子查询方式：DELETE ... WHERE id IN (SELECT id ... LIMIT N)。
// 这两种方式在功能上等价，且都能避免一次性删除大量数据导致锁库。
func (s *AuditLogService) CleanExpired(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 90
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays).Format(DateTimeFormat)

	const batchSize = 1000
	var totalDeleted int64

	for {
		// 检查 context 是否已取消（如 handler 超时）
		if ctx.Err() != nil {
			slog.Info("audit log clean interrupted by context", "deleted_count", totalDeleted, "retention_days", retentionDays)
			return totalDeleted, ctx.Err()
		}

		// 使用子查询方式分批删除，兼容标准 SQLite 语法（rqlite 不支持 DELETE ... LIMIT）
		stmt := rqlite.ParameterizedStatement{
			Query:     `DELETE FROM bdopsflow_audit_logs WHERE id IN (SELECT id FROM bdopsflow_audit_logs WHERE created_at < ? LIMIT 1000)`,
			Arguments: []interface{}{cutoffTime},
		}

		result, err := s.db.WriteOneParameterized(stmt)
		if err != nil {
			slog.Error("failed to clean expired audit logs", "error", err, "retention_days", retentionDays, "deleted_so_far", totalDeleted)
			return totalDeleted, err
		}
		if result.Err != nil {
			slog.Error("audit log clean result error", "error", result.Err, "retention_days", retentionDays, "deleted_so_far", totalDeleted)
			return totalDeleted, result.Err
		}

		deleted := result.RowsAffected
		totalDeleted += deleted

		// 本批无数据被删除，说明已清理完成
		if deleted == 0 {
			break
		}

		// 本批删除量小于 batchSize，说明剩余数据已全部清理
		if deleted < batchSize {
			break
		}
	}

	slog.Info("cleaned expired audit logs", "deleted_count", totalDeleted, "retention_days", retentionDays, "cutoff_time", cutoffTime)
	return totalDeleted, nil
}

// GetRetentionDays 读取审计日志保留天数。
// 从系统配置服务读取（支持热更新），配置缺失或异常时回退到默认值 90 天。
func (s *AuditLogService) GetRetentionDays() int {
	if s.configService != nil {
		days := s.configService.GetInt("audit_log.retention_days")
		if days > 0 {
			return days
		}
	}
	return 90
}

// Count 直接执行 COUNT(*) 查询审计日志总数。
// 与 List 的区别：不查询具体数据行，仅返回 total，性能更优。
// GetStats handler 应使用此方法而非 List(PageSize:1)。
func (s *AuditLogService) Count(ctx context.Context) (int64, error) {
	countStmt := rqlite.ParameterizedStatement{
		Query: "SELECT COUNT(*) FROM bdopsflow_audit_logs",
	}
	countQR, err := s.db.QueryOneParameterized(countStmt)
	if err != nil {
		return 0, err
	}
	if countQR.Err != nil {
		return 0, countQR.Err
	}

	var total int64
	if countQR.Next() {
		row, _ := countQR.Slice()
		total = rowInt64(row[0])
	}
	return total, nil
}
