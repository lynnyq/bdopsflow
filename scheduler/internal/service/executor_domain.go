package service

import (
	"context"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

// ExecutorDomainService 执行器领域分配服务
type ExecutorDomainService struct {
	db rqlite.Connection
}

// NewExecutorDomainService 创建执行器领域分配服务
func NewExecutorDomainService(db rqlite.Connection) *ExecutorDomainService {
	return &ExecutorDomainService{db: db}
}

// GetExecutorDomains 获取执行器所属的所有领域
func (s *ExecutorDomainService) GetExecutorDomains(ctx context.Context, executorID int64) ([]*model.Domain, error) {
	query := `
		SELECT d.id, d.name, d.description
		FROM bdopsflow_domain_executors de
		JOIN bdopsflow_domains d ON de.domain_id = d.id
		WHERE de.executor_id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executorID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_domains []*model.Domain
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		domain := &model.Domain{
			ID:          rowInt64(row[0]),
			Name:        rowString(row[1]),
			Description: rowString(row[2]),
		}
		bdopsflow_domains = append(bdopsflow_domains, domain)
	}

	return bdopsflow_domains, nil
}

// AssignExecutorToDomains 分配执行器到多个领域
func (s *ExecutorDomainService) AssignExecutorToDomains(ctx context.Context, executorID int64, domainIDs []int64, assignedBy int64) error {
	// 先删除旧的关联
	deleteQuery := `DELETE FROM bdopsflow_domain_executors WHERE executor_id = ?`
	deleteStmt := rqlite.ParameterizedStatement{
		Query:     deleteQuery,
		Arguments: []interface{}{executorID},
	}
	_, err := s.db.WriteOneParameterized(deleteStmt)
	if err != nil {
		return err
	}

	// 插入新的关联
	if len(domainIDs) > 0 {
		var statements []rqlite.ParameterizedStatement
		now := time.Now()

		for _, domainID := range domainIDs {
			query := `INSERT INTO bdopsflow_domain_executors (domain_id, executor_id, assigned_by, created_at) VALUES (?, ?, ?, ?)`
			stmt := rqlite.ParameterizedStatement{
				Query:     query,
				Arguments: []interface{}{domainID, executorID, assignedBy, now},
			}
			statements = append(statements, stmt)
		}

		_, err := s.db.WriteParameterized(statements)
		if err != nil {
			return err
		}
	}

	// 如果分配到了领域，将执行器标记为非全局
	if len(domainIDs) > 0 {
		updateQuery := `UPDATE bdopsflow_executors SET is_global = 0 WHERE id = ?`
		updateStmt := rqlite.ParameterizedStatement{
			Query:     updateQuery,
			Arguments: []interface{}{executorID},
		}
		_, err := s.db.WriteOneParameterized(updateStmt)
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoveExecutorFromDomain 从指定领域移除执行器
func (s *ExecutorDomainService) RemoveExecutorFromDomain(ctx context.Context, executorID int64, domainID int64) error {
	query := `DELETE FROM bdopsflow_domain_executors WHERE executor_id = ? AND domain_id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executorID, domainID},
	}
	_, err := s.db.WriteOneParameterized(stmt)
	return err
}

// IsExecutorInDomain 检查执行器是否在指定领域
func (s *ExecutorDomainService) IsExecutorInDomain(ctx context.Context, executorID int64, domainID int64) (bool, error) {
	query := `SELECT COUNT(*) FROM bdopsflow_domain_executors WHERE executor_id = ? AND domain_id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executorID, domainID},
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

	count := int(rowInt64(row[0]))
	return count > 0, nil
}

// GetDomainExecutors 获取指定领域的所有执行器
func (s *ExecutorDomainService) GetDomainExecutors(ctx context.Context, domainID int64) ([]*model.Executor, error) {
	query := `
		SELECT e.id, e.executor_id, e.name, e.address, e.status, e.last_heartbeat, e.capacity, e.current_load, e.created_at, e.updated_at
		FROM bdopsflow_executors e
		JOIN bdopsflow_domain_executors de ON e.id = de.executor_id
		WHERE de.domain_id = ?
		ORDER BY e.name ASC
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

	var bdopsflow_executors []*model.Executor
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		executor := &model.Executor{
			ID:          rowInt64(row[0]),
			ExecutorID:  rowString(row[1]),
			Name:        rowString(row[2]),
			Address:     rowString(row[3]),
			Status:      rowString(row[4]),
			Capacity:    rowInt64(row[6]),
			CurrentLoad: rowInt64(row[7]),
		}
		bdopsflow_executors = append(bdopsflow_executors, executor)
	}

	return bdopsflow_executors, nil
}

// GetExecutorsWithDomains 获取所有执行器及其所属领域
func (s *ExecutorDomainService) GetExecutorsWithDomains(ctx context.Context) ([]*model.ExecutorWithDomains, error) {
	query := `
		SELECT e.id, e.executor_id, e.name, e.address, e.status, e.is_global
		FROM bdopsflow_executors e
		ORDER BY e.name ASC
	`

	qr, err := s.db.QueryOne(query)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_executors []*model.ExecutorWithDomains
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		executor := &model.ExecutorWithDomains{
			Executor: model.Executor{
				ID:        rowInt64(row[0]),
				ExecutorID: rowString(row[1]),
				Name:      rowString(row[2]),
				Address:   rowString(row[3]),
				Status:    rowString(row[4]),
			},
			IsGlobal: rowBool(row[5]),
		}

		// 获取执行器所属的领域
		bdopsflow_domains, err := s.GetExecutorDomains(ctx, executor.ID)
		if err == nil {
			executor.Domains = bdopsflow_domains
		}

		bdopsflow_executors = append(bdopsflow_executors, executor)
	}

	return bdopsflow_executors, nil
}
