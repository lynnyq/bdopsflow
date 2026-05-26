package service

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

type TaskExecutionWithNames struct {
	model.TaskExecution
	TaskName     string
	TaskType     string
	ExecutorName string
}

func (s *SchedulerService) UpdateExecutionResult(ctx context.Context, executionID, status, output, errorMsg string) error {
	now := time.Now().Format(DateTimeFormat)
	query := `
		UPDATE bdopsflow_task_executions
		SET status = ?, output = ?, error = ?,
		    end_time = CASE WHEN ? IN ('success', 'failed', 'timeout') THEN ? ELSE end_time END,
		    start_time = CASE WHEN start_time IS NULL OR start_time = '' THEN ? ELSE start_time END
		WHERE execution_id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{status, output, errorMsg, status, now, now, executionID},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	renewKey := fmt.Sprintf("task:renew:%s", executionID)
	failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)
	s.redis.Del(ctx, lockKey, renewKey, failCountKey)

	slog.Info("task execution finished",
		"execution_id", executionID,
		"status", status,
	)

	return nil
}

func (s *SchedulerService) UpdateTaskProgress(ctx context.Context, executionID string, progress int32, progressMsg string) error {
	query := `
		UPDATE bdopsflow_task_executions
		SET progress = ?, progress_msg = ?, updated_at = ?
		WHERE execution_id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{progress, progressMsg, time.Now().Format(DateTimeFormat), executionID},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}

	slog.Debug("task progress updated",
		"execution_id", executionID,
		"progress", progress,
		"message", progressMsg,
	)

	return nil
}

func (s *SchedulerService) GetTaskExecutions(ctx context.Context, taskID int64) ([]*model.TaskExecution, error) {
	query := `
		SELECT id, task_id, execution_id, executor_id, status, start_time, end_time,
		       output, error, retry_times, created_at, progress, progress_msg, updated_at
		FROM bdopsflow_task_executions
		WHERE task_id = ?
		ORDER BY created_at DESC
		LIMIT 100
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{taskID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var executions []*model.TaskExecution
	for qr.Next() {
		exec := &model.TaskExecution{}
		if err := scanExecutionResult(&qr, exec); err != nil {
			return nil, err
		}
		executions = append(executions, exec)
	}

	return executions, nil
}

func (s *SchedulerService) GetAllExecutions(ctx context.Context, domainID int64, role string, filter map[string]string, page int, pageSize int) ([]*TaskExecutionWithNames, int, error) {
	whereClause := " WHERE 1=1"
	var args []interface{}

	isSystemAdmin := role == "system_admin" || role == "admin"
	if !isSystemAdmin {
		whereClause += " AND t.domain_id = ?"
		args = append(args, domainID)
	}

	if filter["id"] != "" {
		if id, err := strconv.ParseInt(filter["id"], 10, 64); err == nil {
			whereClause += " AND te.id = ?"
			args = append(args, id)
		}
	}
	if filter["execution_id"] != "" {
		whereClause += " AND te.execution_id LIKE ?"
		args = append(args, "%"+filter["execution_id"]+"%")
	}
	if filter["executor_name"] != "" {
		whereClause += " AND e.name LIKE ?"
		args = append(args, "%"+filter["executor_name"]+"%")
	}
	if filter["task_name"] != "" {
		whereClause += " AND t.name LIKE ?"
		args = append(args, "%"+filter["task_name"]+"%")
	}
	if filter["status"] != "" {
		whereClause += " AND te.status = ?"
		args = append(args, filter["status"])
	}
	if filter["start_time_from"] != "" {
		if t, err := parseTimeInLocalTimezone(filter["start_time_from"]); err == nil {
			whereClause += " AND te.start_time >= ?"
			args = append(args, t.Format(DateTimeFormat))
		}
	}
	if filter["start_time_to"] != "" {
		if t, err := parseTimeInLocalTimezone(filter["start_time_to"]); err == nil {
			whereClause += " AND te.start_time <= ?"
			args = append(args, t.Format(DateTimeFormat))
		}
	}
	if filter["end_time_from"] != "" {
		if t, err := parseTimeInLocalTimezone(filter["end_time_from"]); err == nil {
			whereClause += " AND te.end_time >= ?"
			args = append(args, t.Format(DateTimeFormat))
		}
	}
	if filter["end_time_to"] != "" {
		if t, err := parseTimeInLocalTimezone(filter["end_time_to"]); err == nil {
			whereClause += " AND te.end_time <= ?"
			args = append(args, t.Format(DateTimeFormat))
		}
	}
	if filter["duration_min"] != "" || filter["duration_max"] != "" {
		whereClause += " AND te.end_time IS NOT NULL"
		if filter["duration_min"] != "" {
			if duration, err := strconv.ParseFloat(filter["duration_min"], 64); err == nil {
				durationSecs := int64(duration)
				whereClause += " AND (STRFTIME('%s', te.end_time) - STRFTIME('%s', te.start_time)) >= ?"
				args = append(args, durationSecs)
			}
		}
		if filter["duration_max"] != "" {
			if duration, err := strconv.ParseFloat(filter["duration_max"], 64); err == nil {
				durationSecs := int64(duration)
				whereClause += " AND (STRFTIME('%s', te.end_time) - STRFTIME('%s', te.start_time)) <= ?"
				args = append(args, durationSecs)
			}
		}
	}

	joinClause := `
		FROM bdopsflow_task_executions te
		LEFT JOIN bdopsflow_tasks t ON te.task_id = t.id
		LEFT JOIN bdopsflow_executors e ON te.executor_id = e.id
	`

	countQuery := "SELECT COUNT(*) " + joinClause + whereClause
	var countQr rqlite.QueryResult
	var err error

	if len(args) > 0 {
		countStmt := rqlite.ParameterizedStatement{
			Query:     countQuery,
			Arguments: args,
		}
		countQr, err = s.DB.QueryOneParameterized(countStmt)
	} else {
		countQr, err = s.DB.QueryOne(countQuery)
	}

	if err != nil {
		slog.Error("GetAllExecutions: count query failed", "error", err)
		return nil, 0, err
	}
	if countQr.Err != nil {
		slog.Error("GetAllExecutions: count query returned error", "error", countQr.Err)
		return nil, 0, countQr.Err
	}

	var total int
	if countQr.Next() {
		row, err := countQr.Slice()
		if err == nil {
			total = int(rowInt64(row[0]))
		}
	}

	offset := (page - 1) * pageSize
	dataQuery := `
		SELECT te.id, te.task_id, te.execution_id, te.executor_id, te.status, te.start_time, te.end_time,
		       te.output, te.error, te.retry_times, te.created_at,
		       t.name, t.type, e.name
	` + joinClause + whereClause + " ORDER BY te.created_at DESC LIMIT ? OFFSET ?"

	dataArgs := make([]interface{}, len(args))
	copy(dataArgs, args)
	dataArgs = append(dataArgs, pageSize, offset)

	slog.Debug("GetAllExecutions: fetching records", "page", page, "pageSize", pageSize)

	var dataQr rqlite.QueryResult
	if len(dataArgs) > 0 {
		dataStmt := rqlite.ParameterizedStatement{
			Query:     dataQuery,
			Arguments: dataArgs,
		}
		dataQr, err = s.DB.QueryOneParameterized(dataStmt)
	} else {
		dataQr, err = s.DB.QueryOne(dataQuery)
	}

	if err != nil {
		slog.Error("GetAllExecutions: data query failed", "error", err)
		return nil, 0, err
	}
	if dataQr.Err != nil {
		slog.Error("GetAllExecutions: data query returned error", "error", dataQr.Err)
		return nil, 0, dataQr.Err
	}

	var executions []*TaskExecutionWithNames
	for dataQr.Next() {
		exec := &TaskExecutionWithNames{}
		row, err := dataQr.Slice()
		if err != nil {
			slog.Error("GetAllExecutions: slice failed", "error", err)
			return nil, 0, err
		}

		exec.ID = rowInt64(row[0])
		exec.TaskID = rowInt64(row[1])
		exec.ExecutionID = rowString(row[2])
		exec.ExecutorID = rowInt64(row[3])
		exec.Status = rowString(row[4])

		if t, ok := row[5].(time.Time); ok {
			exec.StartTime = rqlite.NullTime{Time: t, Valid: true}
		} else if s, ok := row[5].(string); ok && s != "" {
			parsed, err := parseTimeInLocalTimezone(s)
			if err == nil {
				exec.StartTime = rqlite.NullTime{Time: parsed, Valid: true}
			}
		}

		if t, ok := row[6].(time.Time); ok {
			exec.EndTime = rqlite.NullTime{Time: t, Valid: true}
		} else if s, ok := row[6].(string); ok && s != "" {
			parsed, err := parseTimeInLocalTimezone(s)
			if err == nil {
				exec.EndTime = rqlite.NullTime{Time: parsed, Valid: true}
			}
		}

		exec.Output = rowString(row[7])
		exec.Error = rowString(row[8])
		exec.RetryTimes = int32(rowInt64(row[9]))

		if t, ok := row[10].(time.Time); ok {
			exec.CreatedAt = t
		} else if s, ok := row[10].(string); ok && s != "" {
			parsed, err := parseTimeInLocalTimezone(s)
			if err == nil {
				exec.CreatedAt = parsed
			}
		}

		exec.TaskName = rowString(row[11])
		exec.TaskType = rowString(row[12])
		exec.ExecutorName = rowString(row[13])

		executions = append(executions, exec)
	}

	slog.Debug("GetAllExecutions: completed", "total", total, "returned", len(executions))
	return executions, total, nil
}

func (s *SchedulerService) DeleteExecution(ctx context.Context, id int64) error {
	query := "SELECT execution_id FROM bdopsflow_task_executions WHERE id = ?"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return err
	}
	if qr.Err != nil {
		return qr.Err
	}

	var executionID string
	if qr.Next() {
		row, err := qr.Slice()
		if err == nil {
			executionID = rowString(row[0])
		}
	} else {
		return fmt.Errorf("execution not found")
	}

	deleteLogsQuery := "DELETE FROM bdopsflow_task_logs WHERE execution_id = ?"
	deleteLogsStmt := rqlite.ParameterizedStatement{
		Query:     deleteLogsQuery,
		Arguments: []interface{}{executionID},
	}
	_, err = s.DB.WriteOneParameterized(deleteLogsStmt)
	if err != nil {
		slog.Warn("failed to delete related logs", "error", err)
	}

	deleteExecQuery := "DELETE FROM bdopsflow_task_executions WHERE id = ?"
	deleteExecStmt := rqlite.ParameterizedStatement{
		Query:     deleteExecQuery,
		Arguments: []interface{}{id},
	}
	_, err = s.DB.WriteOneParameterized(deleteExecStmt)
	return err
}

func (s *SchedulerService) BatchDeleteExecutions(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "SELECT execution_id FROM bdopsflow_task_executions WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: args,
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return err
	}
	if qr.Err != nil {
		return qr.Err
	}

	var executionIDs []string
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		eid := rowString(row[0])
		executionIDs = append(executionIDs, eid)
	}

	if len(executionIDs) > 0 {
		logPlaceholders := make([]string, len(executionIDs))
		logArgs := make([]interface{}, len(executionIDs))
		for i, eid := range executionIDs {
			logPlaceholders[i] = "?"
			logArgs[i] = eid
		}

		deleteLogsQuery := "DELETE FROM bdopsflow_task_logs WHERE execution_id IN (" + strings.Join(logPlaceholders, ",") + ")"
		deleteLogsStmt := rqlite.ParameterizedStatement{
			Query:     deleteLogsQuery,
			Arguments: logArgs,
		}
		_, err = s.DB.WriteOneParameterized(deleteLogsStmt)
		if err != nil {
			slog.Warn("failed to delete related logs", "error", err)
		}
	}

	deleteExecQuery := "DELETE FROM bdopsflow_task_executions WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	deleteExecStmt := rqlite.ParameterizedStatement{
		Query:     deleteExecQuery,
		Arguments: args,
	}
	_, err = s.DB.WriteOneParameterized(deleteExecStmt)
	return err
}

func (s *SchedulerService) DeleteExecutionWithDomainCheck(ctx context.Context, id int64, domainID int64, role string) error {
	isSystemAdmin := role == "system_admin" || role == "admin"

	checkQuery := `
		SELECT te.execution_id
		FROM bdopsflow_task_executions te
		LEFT JOIN bdopsflow_tasks t ON te.task_id = t.id
		WHERE te.id = ?
	`
	checkArgs := []interface{}{id}

	if !isSystemAdmin {
		checkQuery += " AND t.domain_id = ?"
		checkArgs = append(checkArgs, domainID)
	}

	checkStmt := rqlite.ParameterizedStatement{
		Query:     checkQuery,
		Arguments: checkArgs,
	}

	qr, err := s.DB.QueryOneParameterized(checkStmt)
	if err != nil {
		return err
	}
	if qr.Err != nil {
		return qr.Err
	}

	var executionID string
	if qr.Next() {
		row, err := qr.Slice()
		if err == nil {
			executionID = rowString(row[0])
		}
	} else {
		return fmt.Errorf("execution not found or permission denied")
	}

	if executionID != "" {
		deleteLogsQuery := "DELETE FROM bdopsflow_task_logs WHERE execution_id = ?"
		deleteLogsStmt := rqlite.ParameterizedStatement{
			Query:     deleteLogsQuery,
			Arguments: []interface{}{executionID},
		}
		_, _ = s.DB.WriteOneParameterized(deleteLogsStmt)
	}

	deleteExecQuery := "DELETE FROM bdopsflow_task_executions WHERE id = ?"
	deleteExecStmt := rqlite.ParameterizedStatement{
		Query:     deleteExecQuery,
		Arguments: []interface{}{id},
	}
	_, err = s.DB.WriteOneParameterized(deleteExecStmt)
	return err
}

func (s *SchedulerService) BatchDeleteExecutionsWithDomainCheck(ctx context.Context, ids []int64, domainID int64, role string) error {
	if len(ids) == 0 {
		return nil
	}

	isSystemAdmin := role == "system_admin" || role == "admin"

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	checkQuery := `
		SELECT te.execution_id
		FROM bdopsflow_task_executions te
		LEFT JOIN bdopsflow_tasks t ON te.task_id = t.id
		WHERE te.id IN (` + strings.Join(placeholders, ",") + `)
	`

	if !isSystemAdmin {
		checkQuery += " AND t.domain_id = ?"
		args = append(args, domainID)
	}

	checkStmt := rqlite.ParameterizedStatement{
		Query:     checkQuery,
		Arguments: args,
	}

	qr, err := s.DB.QueryOneParameterized(checkStmt)
	if err != nil {
		return err
	}
	if qr.Err != nil {
		return qr.Err
	}

	var executionIDs []string
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		eid := rowString(row[0])
		executionIDs = append(executionIDs, eid)
	}

	if len(executionIDs) > 0 {
		logPlaceholders := make([]string, len(executionIDs))
		logArgs := make([]interface{}, len(executionIDs))
		for i, eid := range executionIDs {
			logPlaceholders[i] = "?"
			logArgs[i] = eid
		}

		deleteLogsQuery := "DELETE FROM bdopsflow_task_logs WHERE execution_id IN (" + strings.Join(logPlaceholders, ",") + ")"
		deleteLogsStmt := rqlite.ParameterizedStatement{
			Query:     deleteLogsQuery,
			Arguments: logArgs,
		}
		_, _ = s.DB.WriteOneParameterized(deleteLogsStmt)
	}

	deleteArgs := make([]interface{}, len(ids))
	for i, id := range ids {
		deleteArgs[i] = id
	}

	deleteExecQuery := "DELETE FROM bdopsflow_task_executions WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	deleteExecStmt := rqlite.ParameterizedStatement{
		Query:     deleteExecQuery,
		Arguments: deleteArgs,
	}

	if !isSystemAdmin {
		deleteExecQuery = `
			DELETE FROM bdopsflow_task_executions
			WHERE id IN (` + strings.Join(placeholders, ",") + `)
			AND task_id IN (SELECT id FROM bdopsflow_tasks WHERE domain_id = ?)
		`
		deleteArgs = append(deleteArgs, domainID)
	}

	deleteExecStmt = rqlite.ParameterizedStatement{
		Query:     deleteExecQuery,
		Arguments: deleteArgs,
	}

	_, err = s.DB.WriteOneParameterized(deleteExecStmt)
	return err
}

func (s *SchedulerService) GetTaskLogs(ctx context.Context, executionID string) ([]*model.TaskLog, error) {
	query := `
		SELECT id, execution_id, task_id, executor_id, node_id, log_level, message, log_time
		FROM bdopsflow_task_logs WHERE execution_id = ?
		ORDER BY log_time ASC
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executionID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var logs []*model.TaskLog
	for qr.Next() {
		tl := &model.TaskLog{}
		if err := scanTaskLogResult(&qr, tl); err != nil {
			return nil, err
		}
		logs = append(logs, tl)
	}

	return logs, nil
}

func (s *SchedulerService) GetTaskLogsByExecutionID(ctx context.Context, executionID string) ([]*model.TaskLog, error) {
	return s.GetTaskLogs(ctx, executionID)
}

func (s *SchedulerService) AddTaskLog(ctx context.Context, executionID string, taskID int64, nodeID string, logLevel string, message string) error {
	var executorID interface{} = nil
	execQuery := `SELECT executor_id FROM bdopsflow_task_executions WHERE execution_id = ? LIMIT 1`
	execStmt := rqlite.ParameterizedStatement{
		Query:     execQuery,
		Arguments: []interface{}{executionID},
	}
	execQr, err := s.DB.QueryOneParameterized(execStmt)
	if err == nil && execQr.Err == nil && execQr.Next() {
		row, _ := execQr.Slice()
		rawID := rowInt64(row[0])
		if rawID > 0 {
			executorID = rawID
		}
	}

	dedupEnabled := true
	if dedupEnabled && s.redis != nil {
		logHash := fmt.Sprintf("%x", []byte(fmt.Sprintf("%s-%s-%s-%s", executionID, nodeID, logLevel, message)))
		dedupKey := fmt.Sprintf("task:log:dedup:%s", logHash)

		exists, _ := s.redis.Exists(ctx, dedupKey).Result()
		if exists > 0 {
			slog.Debug("Skipping duplicate task log",
				"execution_id", executionID,
				"log_level", logLevel)
			return nil
		}

		s.redis.Set(ctx, dedupKey, "1", 30*time.Second)
	}

	query := `
		INSERT INTO bdopsflow_task_logs (execution_id, task_id, executor_id, node_id, log_level, message, log_time)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().Format(DateTimeFormat)
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executionID, taskID, executorID, nodeID, logLevel, message, now},
	}
	result, err := s.DB.WriteOneParameterized(stmt)

	if err != nil || result.Err != nil {
		slog.Debug("Falling back to old insert format for bdopsflow_task_logs")
		fallbackQuery := `
			INSERT INTO bdopsflow_task_logs (execution_id, task_id, node_id, log_level, message, log_time)
			VALUES (?, ?, ?, ?, ?, ?)
		`
		fallbackStmt := rqlite.ParameterizedStatement{
			Query:     fallbackQuery,
			Arguments: []interface{}{executionID, taskID, nodeID, logLevel, message, now},
		}
		result, err = s.DB.WriteOneParameterized(fallbackStmt)
		if err != nil {
			return err
		}
		if result.Err != nil {
			return result.Err
		}
	}

	if logLevel == "stdout" || logLevel == "stderr" {
		updateQuery := `
			UPDATE bdopsflow_task_executions
			SET `
		if logLevel == "stdout" {
			updateQuery += `output = COALESCE(output, '') || ?`
		} else {
			updateQuery += `error = COALESCE(error, '') || ?`
		}
		updateQuery += ` WHERE execution_id = ?`

		updateStmt := rqlite.ParameterizedStatement{
			Query:     updateQuery,
			Arguments: []interface{}{message, executionID},
		}
		result, err := s.DB.WriteOneParameterized(updateStmt)
		if err != nil {
			slog.Warn("failed to update execution output/error", "error", err, "execution_id", executionID)
		} else if result.Err != nil {
			slog.Warn("failed to update execution output/error", "error", result.Err, "execution_id", executionID)
		}
	}

	return nil
}

func (s *SchedulerService) addRecoveryLogSafe(ctx context.Context, executionID string, taskID int64, logLevel string, message string) error {
	dedupKey := fmt.Sprintf("task:log:dedup:%s:recovery:%d", executionID, time.Now().Unix()/300)
	exists, err := s.redis.Exists(ctx, dedupKey).Result()
	if err == nil && exists > 0 {
		slog.Debug("Skipping duplicate recovery log", "execution_id", executionID)
		return nil
	}

	s.redis.Set(ctx, dedupKey, "1", 10*time.Minute)

	return s.AddTaskLog(ctx, executionID, taskID, "", logLevel, message)
}

func (s *SchedulerService) GetExecutionStats(ctx context.Context, domainID int64, role string, filter map[string]string) (map[string]int, error) {
	whereClause := " WHERE 1=1"
	var args []interface{}

	isSystemAdmin := role == "system_admin" || role == "admin"
	if !isSystemAdmin {
		whereClause += " AND t.domain_id = ?"
		args = append(args, domainID)
	}

	if filter["id"] != "" {
		if id, err := strconv.ParseInt(filter["id"], 10, 64); err == nil {
			whereClause += " AND te.id = ?"
			args = append(args, id)
		}
	}
	if filter["execution_id"] != "" {
		whereClause += " AND te.execution_id LIKE ?"
		args = append(args, "%"+filter["execution_id"]+"%")
	}
	if filter["executor_name"] != "" {
		whereClause += " AND e.name LIKE ?"
		args = append(args, "%"+filter["executor_name"]+"%")
	}
	if filter["task_name"] != "" {
		whereClause += " AND t.name LIKE ?"
		args = append(args, "%"+filter["task_name"]+"%")
	}
	if filter["status"] != "" {
		whereClause += " AND te.status = ?"
		args = append(args, filter["status"])
	}
	if filter["start_time_from"] != "" {
		if t, err := parseTimeInLocalTimezone(filter["start_time_from"]); err == nil {
			whereClause += " AND te.start_time >= ?"
			args = append(args, t.Format(DateTimeFormat))
		}
	}
	if filter["start_time_to"] != "" {
		if t, err := parseTimeInLocalTimezone(filter["start_time_to"]); err == nil {
			whereClause += " AND te.start_time <= ?"
			args = append(args, t.Format(DateTimeFormat))
		}
	}
	if filter["end_time_from"] != "" {
		if t, err := parseTimeInLocalTimezone(filter["end_time_from"]); err == nil {
			whereClause += " AND te.end_time >= ?"
			args = append(args, t.Format(DateTimeFormat))
		}
	}
	if filter["end_time_to"] != "" {
		if t, err := parseTimeInLocalTimezone(filter["end_time_to"]); err == nil {
			whereClause += " AND te.end_time <= ?"
			args = append(args, t.Format(DateTimeFormat))
		}
	}
	if filter["duration_min"] != "" || filter["duration_max"] != "" {
		whereClause += " AND te.end_time IS NOT NULL"
		if filter["duration_min"] != "" {
			if duration, err := strconv.ParseFloat(filter["duration_min"], 64); err == nil {
				durationSecs := int64(duration)
				whereClause += " AND (STRFTIME('%s', te.end_time) - STRFTIME('%s', te.start_time)) >= ?"
				args = append(args, durationSecs)
			}
		}
		if filter["duration_max"] != "" {
			if duration, err := strconv.ParseFloat(filter["duration_max"], 64); err == nil {
				durationSecs := int64(duration)
				whereClause += " AND (STRFTIME('%s', te.end_time) - STRFTIME('%s', te.start_time)) <= ?"
				args = append(args, durationSecs)
			}
		}
	}

	joinClause := `
		FROM bdopsflow_task_executions te
		LEFT JOIN bdopsflow_tasks t ON te.task_id = t.id
		LEFT JOIN bdopsflow_executors e ON te.executor_id = e.id
	`

	query := `
		SELECT te.status, COUNT(*) as count
	` + joinClause + whereClause + " GROUP BY te.status"

	var qr rqlite.QueryResult
	var err error

	if len(args) > 0 {
		stmt := rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: args,
		}
		qr, err = s.DB.QueryOneParameterized(stmt)
	} else {
		qr, err = s.DB.QueryOne(query)
	}

	if err != nil {
		slog.Error("GetExecutionStats: query failed", "error", err)
		return nil, err
	}
	if qr.Err != nil {
		slog.Error("GetExecutionStats: query returned error", "error", qr.Err)
		return nil, qr.Err
	}

	stats := make(map[string]int)
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		status := rowString(row[0])
		count := int(rowInt64(row[1]))
		stats[status] = count
	}

	return stats, nil
}

func (s *SchedulerService) RecoverRunningTasksOnBecomeLeader(ctx context.Context) error {
	slog.Info("recovering running tasks on becoming leader")

	cutoffTime := time.Now().Add(-24 * time.Hour).Format(DateTimeFormat)
	query := `
		SELECT execution_id, task_id, executor_id, status, start_time, progress, progress_msg
		FROM bdopsflow_task_executions
		WHERE status = 'running'
		AND created_at > ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{cutoffTime},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return err
	}
	if qr.Err != nil {
		return qr.Err
	}

	type recoveryDetail struct {
		ExecutionID string
		TaskID      int64
		ExecutorID  int64
		Action      string
		Reason      string
	}
	var recoveryDetails []recoveryDetail
	recoveredCount := 0
	failedCount := 0
	errorCount := 0

	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		executionID := rowString(row[0])
		taskID := rowInt64(row[1])
		executorID := rowInt64(row[2])
		startTimeStr := rowString(row[4])
		progress := int32(rowInt(row[5]))
		progressMsg := rowString(row[6])

		slog.Debug("recovering running task",
			"execution_id", executionID,
			"task_id", taskID,
			"executor_id", executorID,
			"progress", progress,
		)

		task, taskErr := s.GetTaskByID(ctx, taskID)
		timeoutSeconds := int64(300)
		noTimeout := false
		if taskErr == nil && task.TimeoutSeconds > 0 {
			timeoutSeconds = int64(task.TimeoutSeconds)
		} else if taskErr == nil && task.TimeoutSeconds <= 0 {
			noTimeout = true
		}

		taskTimeout := false
		if !noTimeout && startTimeStr != "" {
			if startTime, parseErr := parseTimeInLocalTimezone(startTimeStr); parseErr == nil {
				if time.Since(startTime) > time.Duration(timeoutSeconds)*time.Second {
					taskTimeout = true
				}
			}
		}

		executor, execErr := s.GetExecutorByID(ctx, executorID)
		executorOnline := execErr == nil && executor.Status == "online"
		executorRecentlyActive := false
		if execErr == nil && executor.LastHeartbeat.Valid {
			executorRecentlyActive = time.Since(executor.LastHeartbeat.Time) < 90*time.Second
		}

		executorReachable := false
		if execErr == nil && executorOnline {
			executorReachable = s.pingExecutor(ctx, executor)
			if !executorReachable {
				slog.Warn("task recovery: executor is online in DB but unreachable via ping",
					"execution_id", executionID,
					"task_id", taskID,
					"executor_id", executorID,
					"executor_name", executor.Name,
					"executor_address", executor.Address,
				)
			}
		}

		if execErr != nil && executorID > 0 {
			slog.Warn("task recovery: failed to get executor info, treating as offline",
				"execution_id", executionID,
				"task_id", taskID,
				"executor_id", executorID,
				"error", execErr,
			)
		}

		renewKey := fmt.Sprintf("task:renew:%s", executionID)
		lastRenewStr, renewErr := s.redis.Get(ctx, renewKey).Result()
		noRenewal := renewErr != nil || lastRenewStr == ""

		var lastRenewSecondsAgo int64 = 9999
		if !noRenewal {
			var lastRenew int64
			fmt.Sscanf(lastRenewStr, "%d", &lastRenew)
			lastRenewSecondsAgo = time.Now().Unix() - lastRenew
		}

		lockKey := fmt.Sprintf("task:lock:%s", executionID)
		lockExists, _ := s.redis.Exists(ctx, lockKey).Result()

		renewalExpired := !noTimeout && lastRenewSecondsAgo > timeoutSeconds
		shouldFail := !executorOnline || !executorRecentlyActive || !executorReachable || noRenewal || renewalExpired || taskTimeout

		if shouldFail {
			slog.Warn("task recovery: marking task as failed",
				"execution_id", executionID,
				"task_id", taskID,
				"executor_id", executorID,
				"executor_online", executorOnline,
				"executor_recently_active", executorRecentlyActive,
				"executor_reachable", executorReachable,
				"lock_exists", lockExists,
				"task_timeout", taskTimeout,
				"no_renewal", noRenewal,
				"last_renew_seconds_ago", lastRenewSecondsAgo,
			)

			var reason string
			if !executorOnline {
				reason = "scheduler failover: executor is offline"
			} else if !executorRecentlyActive {
				reason = "scheduler failover: executor heartbeat expired"
			} else if !executorReachable {
				reason = "scheduler failover: executor is unreachable (ping failed)"
			} else if noRenewal {
				reason = "scheduler failover: no renewal record found"
			} else if renewalExpired {
				reason = fmt.Sprintf("scheduler failover: renewal expired (%d seconds ago, timeout %d)", lastRenewSecondsAgo, timeoutSeconds)
			} else {
				reason = "scheduler failover: task execution timeout"
			}

			s.forceFailTask(ctx, executionID, taskID, reason)
			failedCount++
			recoveryDetails = append(recoveryDetails, recoveryDetail{
				ExecutionID: executionID,
				TaskID:      taskID,
				ExecutorID:  executorID,
				Action:      "failed",
				Reason:      reason,
			})
			continue
		}

		lockTTL := s.calculateLockTTL(taskErr, task)
		if err := s.redis.Set(ctx, lockKey, "leader_recovered", time.Duration(lockTTL)*time.Second).Err(); err != nil {
			slog.Warn("failed to set task lock during recovery", "execution_id", executionID, "error", err)
		}

		if err := s.redis.Set(ctx, renewKey, time.Now().Unix(), time.Duration(lockTTL)*time.Second).Err(); err != nil {
			slog.Warn("failed to set task renew timestamp during recovery", "execution_id", executionID, "error", err)
		}

		failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)
		s.redis.Del(ctx, failCountKey)

		s.addRecoveryLogSafe(ctx, executionID, taskID, "info",
			fmt.Sprintf("Task recovered by new leader, progress: %d%%, message: %s", progress, progressMsg))

		recoveredCount++
		recoveryDetails = append(recoveryDetails, recoveryDetail{
			ExecutionID: executionID,
			TaskID:      taskID,
			ExecutorID:  executorID,
			Action:      "recovered",
			Reason:      fmt.Sprintf("progress: %d%%, message: %s", progress, progressMsg),
		})
	}

	for _, detail := range recoveryDetails {
		slog.Info("task recovery detail",
			"execution_id", detail.ExecutionID,
			"task_id", detail.TaskID,
			"executor_id", detail.ExecutorID,
			"action", detail.Action,
			"reason", detail.Reason,
		)
	}

	slog.Info("finished recovering running tasks",
		"recovered_count", recoveredCount,
		"failed_count", failedCount,
		"error_count", errorCount,
		"total", recoveredCount+failedCount+errorCount,
	)
	return nil
}

func (s *SchedulerService) calculateLockTTL(taskErr error, task *model.Task) int64 {
	lockTTL := int64(300)
	if taskErr == nil && task.TimeoutSeconds > 0 {
		lockTTL = int64(task.TimeoutSeconds) * 2
	} else if taskErr == nil && task.TimeoutSeconds <= 0 {
		lockTTL = 3600
	}
	if lockTTL < 60 {
		lockTTL = 60
	}
	if lockTTL > 7200 {
		lockTTL = 7200
	}
	return lockTTL
}

func (s *SchedulerService) pingExecutor(ctx context.Context, executor *model.Executor) bool {
	if executor == nil {
		return false
	}

	if s.connectivityChecker != nil && s.connectivityChecker.IsExecutorConnected(executor.Name) {
		return true
	}

	if executor.Address == "" {
		return false
	}

	addr := executor.Address
	if !strings.Contains(addr, ":") {
		addr = addr + ":50051"
	}

	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		slog.Debug("ping executor: TCP dial failed",
			"executor_name", executor.Name,
			"address", addr,
			"error", err,
		)
		return false
	}
	conn.Close()
	return true
}
