package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/metrics"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

func (s *SchedulerService) SelectAvailableExecutor(ctx context.Context, domainID ...int64) (*model.Executor, error) {
	heartbeatCutoff := time.Now().Add(-45 * time.Second).Format(DateTimeFormat)

	query := `
		SELECT e.id, e.name, e.address, e.status, e.last_heartbeat, e.capacity, e.current_load, e.created_at, e.updated_at
		FROM bdopsflow_executors e
		WHERE e.status = 'online' AND e.current_load < e.capacity
		  AND e.last_heartbeat > ?
	`

	var args []interface{}
	args = []interface{}{heartbeatCutoff}

	if len(domainID) > 0 && domainID[0] > 0 {
		query = `
			SELECT e.id, e.name, e.address, e.status, e.last_heartbeat, e.capacity, e.current_load, e.created_at, e.updated_at
			FROM bdopsflow_executors e
			LEFT JOIN bdopsflow_domain_executors de ON e.id = de.executor_id
			WHERE e.status = 'online' AND e.current_load < e.capacity
			  AND e.last_heartbeat > ?
			  AND (de.domain_id = ? OR e.is_global = 1)
		`
		args = []interface{}{heartbeatCutoff, domainID[0]}
	}

	query += " ORDER BY e.current_load ASC, RANDOM() LIMIT 1"

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: args,
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}

	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("no available executor")
	}

	exec := &model.Executor{}
	if err := scanExecutorResult(&qr, exec); err != nil {
		return nil, err
	}

	return exec, nil
}

func (s *SchedulerService) RegisterExecutor(ctx context.Context, name, address string, capacity int32) (string, error) {
	now := time.Now().Format(DateTimeFormat)

	existingExecutor, err := s.GetExecutorByName(ctx, name)
	if err == nil && existingExecutor != nil {
		if existingExecutor.Status == "online" && existingExecutor.Address != address {
			existingHost := strings.SplitN(existingExecutor.Address, "#", 2)[0]
			newHost := strings.SplitN(address, "#", 2)[0]
			sameHost := existingHost == newHost

			if !sameHost && existingExecutor.LastHeartbeat.Valid {
				heartbeatCutoff := time.Now().Add(-45 * time.Second)
				if existingExecutor.LastHeartbeat.Time.After(heartbeatCutoff) {
					slog.Warn("RegisterExecutor: rejected duplicate executor from different host",
						"name", name,
						"existing_address", existingExecutor.Address,
						"new_address", address,
					)
					return "", fmt.Errorf("%w: executor %s is already online at %s, duplicate registration rejected", ErrExecutorDuplicate, name, existingExecutor.Address)
				}
			}

			if sameHost {
				slog.Info("RegisterExecutor: same host restart detected, allowing re-registration",
					"name", name,
					"old_address", existingExecutor.Address,
					"new_address", address,
				)
			}
		}

		// 当执行器重启时，清理该执行器上所有正在运行任务的 renew 记录
		if existingExecutor.ID > 0 {
			s.cleanupExecutorStaleTasks(ctx, existingExecutor.ID)
		}

		updateQuery := `
			UPDATE bdopsflow_executors
			SET address = ?, capacity = ?, status = 'online', last_heartbeat = ?, updated_at = ?
			WHERE name = ?
		`
		stmt := rqlite.ParameterizedStatement{
			Query:     updateQuery,
			Arguments: []interface{}{address, capacity, now, now, name},
		}
		result, err := s.DB.WriteOneParameterized(stmt)
		if err != nil {
			return "", err
		}
		if result.Err != nil {
			return "", result.Err
		}

		slog.Info("RegisterExecutor: updated existing executor",
			"name", name,
			"executor_id", existingExecutor.ID,
			"address", address,
			"capacity", capacity,
		)
		s.updateExecutorMetrics(ctx)
		return name, nil
	}

	insertQuery := `
		INSERT INTO bdopsflow_executors (name, address, status, capacity, current_load, is_global, last_heartbeat, created_at, updated_at)
		VALUES (?, ?, 'online', ?, 0, 0, ?, ?, ?)
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     insertQuery,
		Arguments: []interface{}{name, address, capacity, now, now, now},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return "", err
	}

	if result.Err != nil {
		return "", result.Err
	}

	executorDBID := result.LastInsertID

	if executorDBID > 0 && s.ExecutorDomainService != nil {
		if err := s.ExecutorDomainService.AssignExecutorToDefaultDomain(ctx, name, 1); err != nil {
			slog.Warn("failed to assign executor to default domain", "executor", name, "error", err)
		}
	}

	slog.Info("RegisterExecutor: created new executor",
		"name", name,
		"executor_id", executorDBID,
		"address", address,
		"capacity", capacity,
	)

	s.updateExecutorMetrics(ctx)

	return name, nil
}

func (s *SchedulerService) DeleteExecutor(ctx context.Context, id int64) error {
	query := `DELETE FROM bdopsflow_executors WHERE id = ?`
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	})
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	return nil
}

func (s *SchedulerService) DeleteExecutorByName(ctx context.Context, name string) error {
	query := `DELETE FROM bdopsflow_executors WHERE name = ?`
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{name},
	})
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	return nil
}

func (s *SchedulerService) SetExecutorStatusByName(ctx context.Context, name string, status string) error {
	query := `UPDATE bdopsflow_executors SET status = ?, updated_at = ? WHERE name = ?`
	now := time.Now().Format(DateTimeFormat)
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{status, now, name},
	})
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	s.updateExecutorMetrics(ctx)

	return nil
}

func (s *SchedulerService) UpdateExecutorCapacityByName(ctx context.Context, name string, capacity int64) error {
	if capacity <= 0 {
		return fmt.Errorf("capacity must be positive")
	}

	query := `UPDATE bdopsflow_executors SET capacity = ?, updated_at = ? WHERE name = ?`
	now := time.Now().Format(DateTimeFormat)
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{capacity, now, name},
	})
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}

	key := fmt.Sprintf("executor:target_capacity:%s", name)
	if err := s.redis.Set(ctx, key, capacity, 0).Err(); err != nil {
		slog.Warn("failed to store target capacity in redis", "error", err)
	}

	slog.Info("updated executor capacity",
		"executor_name", name,
		"new_capacity", capacity)
	return nil
}

func (s *SchedulerService) GetExecutorTargetCapacity(ctx context.Context, name string) (int32, error) {
	key := fmt.Sprintf("executor:target_capacity:%s", name)
	val, err := s.redis.Get(ctx, key).Int64()
	if err != nil {
		exec, err := s.GetExecutorByName(ctx, name)
		if err != nil {
			return 0, err
		}
		return int32(exec.Capacity), nil
	}
	return int32(val), nil
}

func (s *SchedulerService) UpdateExecutorHeartbeat(ctx context.Context, name string, currentLoad int32) error {
	return s.UpdateExecutorHeartbeatWithRunningTasks(ctx, name, currentLoad, nil)
}

func (s *SchedulerService) UpdateExecutorHeartbeatWithRunningTasks(ctx context.Context, name string, currentLoad int32, runningExecutionIds []string) error {
	query := `
		UPDATE bdopsflow_executors SET current_load = ?, last_heartbeat = ?, updated_at = ?
		WHERE name = ? AND status = 'online'
	`

	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{currentLoad, now, now, name},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	for _, execID := range runningExecutionIds {
		if err := s.renewTaskLock(ctx, execID); err != nil {
			slog.Warn("failed to renew task lock", "execution_id", execID, "error", err)
		}
	}

	return nil
}

func (s *SchedulerService) SetExecutorStatus(ctx context.Context, id int64, status string) error {
	query := `UPDATE bdopsflow_executors SET status = ?, updated_at = ? WHERE id = ?`
	now := time.Now().Format(DateTimeFormat)
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{status, now, id},
	})
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	return nil
}

func (s *SchedulerService) UpdateExecutorCapacity(ctx context.Context, id int64, capacity int64) error {
	if capacity <= 0 {
		return fmt.Errorf("capacity must be positive")
	}

	query := `UPDATE bdopsflow_executors SET capacity = ?, updated_at = ? WHERE id = ?`
	now := time.Now().Format(DateTimeFormat)
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{capacity, now, id},
	})
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}

	key := fmt.Sprintf("executor:target_capacity:%d", id)
	if err := s.redis.Set(ctx, key, capacity, 0).Err(); err != nil {
		slog.Warn("failed to store target capacity in redis", "error", err)
	}

	slog.Info("updated executor capacity",
		"executor_id", id,
		"new_capacity", capacity)
	return nil
}

func (s *SchedulerService) GetExecutorByID(ctx context.Context, id int64) (*model.Executor, error) {
	query := `
		SELECT id, name, address, status, last_heartbeat, capacity, current_load, is_global, created_at, updated_at
		FROM bdopsflow_executors WHERE id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("executor not found")
	}

	exec := &model.Executor{}
	if err := scanExecutorResult(&qr, exec); err != nil {
		return nil, err
	}

	return exec, nil
}

func (s *SchedulerService) GetExecutorByName(ctx context.Context, name string) (*model.Executor, error) {
	query := `
		SELECT id, name, address, status, last_heartbeat, capacity, current_load, is_global, created_at, updated_at
		FROM bdopsflow_executors WHERE name = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{name},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("executor not found")
	}

	exec := &model.Executor{}
	if err := scanExecutorResult(&qr, exec); err != nil {
		return nil, err
	}

	return exec, nil
}

func (s *SchedulerService) GetExecutorInfoByID(ctx context.Context, id int64) (*model.Executor, error) {
	return s.GetExecutorByID(ctx, id)
}

func (s *SchedulerService) ListExecutors(ctx context.Context, page, pageSize int) ([]*model.Executor, int, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if page <= 0 {
		page = 1
	}

	countQuery := "SELECT COUNT(*) FROM bdopsflow_executors"
	countQr, err := s.DB.QueryOne(countQuery)
	if err != nil {
		return nil, 0, err
	}
	if countQr.Err != nil {
		return nil, 0, countQr.Err
	}

	var total int
	if countQr.Next() {
		row, _ := countQr.Slice()
		total = int(rowInt64(row[0]))
	}

	offset := (page - 1) * pageSize
	dataQuery := `
		SELECT id, name, address, status, last_heartbeat, capacity, current_load, created_at, updated_at
		FROM bdopsflow_executors ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     dataQuery,
		Arguments: []interface{}{pageSize, offset},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, 0, err
	}
	if qr.Err != nil {
		return nil, 0, qr.Err
	}

	var bdopsflow_executors []*model.Executor
	for qr.Next() {
		exec := &model.Executor{}
		if err := scanExecutorResult(&qr, exec); err != nil {
			return nil, 0, err
		}
		bdopsflow_executors = append(bdopsflow_executors, exec)
	}

	return bdopsflow_executors, total, nil
}

func (s *SchedulerService) ListExecutorsByDomain(ctx context.Context, domainID int64) ([]*model.Executor, error) {
	query := `
		SELECT e.id, e.name, e.address, e.status, e.last_heartbeat, e.capacity, e.current_load, e.created_at, e.updated_at
		FROM bdopsflow_executors e
		LEFT JOIN bdopsflow_domain_executors de ON e.id = de.executor_id
		WHERE e.status = 'online' AND (de.domain_id = ? OR e.is_global = 1)
		ORDER BY e.current_load ASC
	`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{domainID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var executors []*model.Executor
	for qr.Next() {
		exec := &model.Executor{}
		if err := scanExecutorResult(&qr, exec); err != nil {
			continue
		}
		executors = append(executors, exec)
	}

	if executors == nil {
		executors = []*model.Executor{}
	}

	return executors, nil
}

// updateExecutorMetrics 更新执行器在线/离线 Prometheus 指标
func (s *SchedulerService) updateExecutorMetrics(ctx context.Context) {
	query := `SELECT status, COUNT(*) FROM bdopsflow_executors GROUP BY status`
	qr, err := s.DB.QueryOne(query)
	if err != nil || qr.Err != nil {
		slog.Warn("failed to query executor status counts for metrics", "error", err)
		return
	}

	var online, offline float64
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		status := rowString(row[0])
		count := rowInt64(row[1])
		switch status {
		case "online":
			online = float64(count)
		default:
			offline += float64(count)
		}
	}

	metrics.ExecutorsOnline.Set(online)
	metrics.ExecutorsOffline.Set(offline)
}
