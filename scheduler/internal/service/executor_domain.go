package service

import (
	"context"
	"fmt"
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

	var domains []*model.Domain
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
		domains = append(domains, domain)
	}

	return domains, nil
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

	// 如果分配到了领域，将执行器标记为非全局，否则标记为全局
	isGlobal := len(domainIDs) == 0
	updateQuery := `UPDATE bdopsflow_executors SET is_global = ? WHERE id = ?`
	updateStmt := rqlite.ParameterizedStatement{
		Query:     updateQuery,
		Arguments: []interface{}{isGlobal, executorID},
	}
	_, err = s.db.WriteOneParameterized(updateStmt)
	if err != nil {
		return err
	}

	return nil
}

// AssignExecutorToDefaultDomain 将新注册的执行器分配到默认领域（admin）
func (s *ExecutorDomainService) AssignExecutorToDefaultDomain(ctx context.Context, executorName string, assignedBy int64) error {
	executor, err := s.GetExecutorByName(ctx, executorName)
	if err != nil {
		return fmt.Errorf("获取执行器失败: %w", err)
	}
	
	query := `INSERT INTO bdopsflow_domain_executors (domain_id, executor_id, assigned_by, created_at) VALUES (1, ?, ?, ?)`
	now := time.Now()
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executor.ID, assignedBy, now},
	}
	_, err = s.db.WriteOneParameterized(stmt)
	return err
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
		SELECT e.id, e.name, e.address, e.status, e.last_heartbeat, e.capacity, e.current_load, e.created_at, e.updated_at
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

	var executors []*model.Executor
	for qr.Next() {
		executor := &model.Executor{}
		if err := scanExecutorResult(&qr, executor); err != nil {
			return nil, err
		}
		executors = append(executors, executor)
	}

	return executors, nil
}

// GetExecutorsWithDomains 获取所有执行器及其所属领域
func (s *ExecutorDomainService) GetExecutorsWithDomains(ctx context.Context) ([]*model.ExecutorWithDomains, error) {
	query := `
		SELECT e.id, e.name, e.address, e.status, e.last_heartbeat, e.capacity, e.current_load, e.is_global, e.created_at, e.updated_at
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

	var executors []*model.ExecutorWithDomains
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		executor := &model.ExecutorWithDomains{
			Executor: model.Executor{
				ID:          rowInt64(row[0]),
				Name:        rowString(row[1]),
				Address:     rowString(row[2]),
				Status:      rowString(row[3]),
				Capacity:    rowInt64(row[5]),
				CurrentLoad: rowInt64(row[6]),
			},
			IsGlobal: rowBool(row[7]),
		}
		
		if t, ok := row[4].(time.Time); ok {
			executor.LastHeartbeat = rqlite.NullTime{Time: t, Valid: true}
		}
		if t, ok := row[8].(time.Time); ok {
			executor.CreatedAt = t
		}
		if t, ok := row[9].(time.Time); ok {
			executor.UpdatedAt = t
		}

		domains, err := s.GetExecutorDomains(ctx, executor.ID)
		if err == nil {
			executor.Domains = domains
		}

		executors = append(executors, executor)
	}

	return executors, nil
}

// GetExecutorDomainCount 获取执行器所属的领域数量
func (s *ExecutorDomainService) GetExecutorDomainCount(ctx context.Context, executorID int64) (int, error) {
	query := `SELECT COUNT(*) FROM bdopsflow_domain_executors WHERE executor_id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executorID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return 0, err
	}
	if qr.Err != nil {
		return 0, qr.Err
	}

	if !qr.Next() {
		return 0, nil
	}
	row, err := qr.Slice()
	if err != nil {
		return 0, err
	}

	return int(rowInt64(row[0])), nil
}

// GetAssignedTasksForExecutor 获取绑定到指定执行器的任务数量
func (s *ExecutorDomainService) GetAssignedTasksForExecutor(ctx context.Context, executorDBID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM bdopsflow_tasks WHERE assigned_executor_id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executorDBID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return 0, err
	}
	if qr.Err != nil {
		return 0, qr.Err
	}

	if !qr.Next() {
		return 0, nil
	}
	row, err := qr.Slice()
	if err != nil {
		return 0, err
	}

	return rowInt64(row[0]), nil
}

// GetAssignedTaskNamesForExecutor 获取绑定到指定执行器的任务名称列表
func (s *ExecutorDomainService) GetAssignedTaskNamesForExecutor(ctx context.Context, executorDBID int64) ([]string, error) {
	query := `SELECT name FROM bdopsflow_tasks WHERE assigned_executor_id = ? ORDER BY name`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executorDBID},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var taskNames []string
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		taskNames = append(taskNames, rowString(row[0]))
	}

	return taskNames, nil
}

// GetExecutorsByUserRole 根据用户角色和领域获取执行器列表
func (s *ExecutorDomainService) GetExecutorsByUserRole(ctx context.Context, userRole string, userDomainID int64) ([]*model.ExecutorWithDomains, error) {
	var query string
	var args []interface{}

	if userRole == "system_admin" || userRole == "admin" {
		query = `
			SELECT e.id, e.name, e.address, e.status, e.last_heartbeat, e.capacity, e.current_load, e.is_global, e.created_at, e.updated_at
			FROM bdopsflow_executors e
			ORDER BY e.name ASC
		`
		args = []interface{}{}
	} else {
		query = `
			SELECT e.id, e.name, e.address, e.status, e.last_heartbeat, e.capacity, e.current_load, e.is_global, e.created_at, e.updated_at
			FROM bdopsflow_executors e
			JOIN bdopsflow_domain_executors de ON e.id = de.executor_id
			WHERE de.domain_id = ?
			ORDER BY e.name ASC
		`
		args = []interface{}{userDomainID}
	}

	qr, err := s.db.QueryOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var executors []*model.ExecutorWithDomains
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		executor := &model.ExecutorWithDomains{
			Executor: model.Executor{
				ID:          rowInt64(row[0]),
				Name:        rowString(row[1]),
				Address:     rowString(row[2]),
				Status:      rowString(row[3]),
				Capacity:    rowInt64(row[5]),
				CurrentLoad: rowInt64(row[6]),
			},
			IsGlobal: rowBool(row[7]),
		}
		
		if t, ok := row[4].(time.Time); ok {
			executor.LastHeartbeat = rqlite.NullTime{Time: t, Valid: true}
		}
		if t, ok := row[8].(time.Time); ok {
			executor.CreatedAt = t
		}
		if t, ok := row[9].(time.Time); ok {
			executor.UpdatedAt = t
		}

		domains, err := s.GetExecutorDomains(ctx, executor.ID)
		if err == nil {
			executor.Domains = domains
		}

		executors = append(executors, executor)
	}

	return executors, nil
}

// CanDomainAdminDeleteExecutor 检查领域管理员是否可以删除执行器
func (s *ExecutorDomainService) CanDomainAdminDeleteExecutor(ctx context.Context, executorID int64, domainID int64) (bool, error) {
	domainCount, err := s.GetExecutorDomainCount(ctx, executorID)
	if err != nil {
		return false, err
	}

	// 如果执行器只分配给一个领域，则可以删除
	return domainCount <= 1, nil
}

// GetExecutorByDBID 根据数据库ID获取执行器
func (s *ExecutorDomainService) GetExecutorByDBID(ctx context.Context, id int64) (*model.Executor, error) {
	query := `
		SELECT id, name, address, status, last_heartbeat, capacity, current_load, is_global, created_at, updated_at
		FROM bdopsflow_executors
		WHERE id = ?
	`
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
		return nil, fmt.Errorf("executor not found")
	}

	executor := &model.Executor{}
	if err := scanExecutorResult(&qr, executor); err != nil {
		return nil, err
	}

	return executor, nil
}

// GetExecutorByName 根据执行器名称获取执行器
func (s *ExecutorDomainService) GetExecutorByName(ctx context.Context, name string) (*model.Executor, error) {
	query := `
		SELECT id, name, address, status, last_heartbeat, capacity, current_load, is_global, created_at, updated_at
		FROM bdopsflow_executors
		WHERE name = ?
	`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{name},
	}
	qr, err := s.db.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("executor not found")
	}

	executor := &model.Executor{}
	if err := scanExecutorResult(&qr, executor); err != nil {
		return nil, err
	}

	return executor, nil
}
