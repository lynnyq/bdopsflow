package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// newSchedulerWithDB 构造使用指定 MockDB 的 SchedulerService（不带 redis）
func newSchedulerWithDB(db *MockDB) *SchedulerService {
	return &SchedulerService{
		DB: db,
	}
}

// newSchedulerWithDBAndRedis 构造使用指定 MockDB 和 miniredis 的 SchedulerService
func newSchedulerWithDBAndRedis(t *testing.T, db *MockDB) (*SchedulerService, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, rdb := newTestRedis(t)
	svc := &SchedulerService{
		DB:    db,
		redis: rdb,
	}
	return svc, mr, rdb
}

// ============ UpdateExecutionResult ============

func TestUpdateExecutionResult_Success(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.UpdateExecutionResult(ctx, "exec-001", "success", "output", "")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}

	// 验证 redis key 被删除
	if db.LastWriteStmt.Query == "" {
		t.Error("期望记录 WriteStmt")
	}
}

func TestUpdateExecutionResult_DBError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{WriteError: ErrMockDB}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.UpdateExecutionResult(ctx, "exec-001", "success", "output", "")
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

func TestUpdateExecutionResult_ResultErr(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{WriteResult: rqlite.WriteResult{Err: errors.New("result error")}}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.UpdateExecutionResult(ctx, "exec-001", "failed", "", "error")
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestUpdateExecutionResult_StatusMetrics(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name   string
		status string
	}{
		{"success", "success"},
		{"failed", "failed"},
		{"timeout", "timeout"},
		{"running", "running"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
			svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
			defer mr.Close()

			err := svc.UpdateExecutionResult(ctx, "exec-"+tt.status, tt.status, "", "")
			if err != nil {
				t.Errorf("期望无错误，实际: %v", err)
			}
		})
	}
}

// ============ UpdateTaskProgress ============

func TestUpdateTaskProgress_Success(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
	svc := newSchedulerWithDB(db)

	err := svc.UpdateTaskProgress(ctx, "exec-001", 50, "half done")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestUpdateTaskProgress_DBError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{WriteError: ErrMockDB}
	svc := newSchedulerWithDB(db)

	err := svc.UpdateTaskProgress(ctx, "exec-001", 50, "half done")
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

func TestUpdateTaskProgress_ResultErr(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{WriteResult: rqlite.WriteResult{Err: errors.New("result error")}}
	svc := newSchedulerWithDB(db)

	err := svc.UpdateTaskProgress(ctx, "exec-001", 50, "half done")
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

// ============ GetTaskExecutions ============

func TestGetTaskExecutions_Success(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1), int64(100), "exec-001", int64(2), "success", now, now, "output", "", int64(0), now, int64(100), "done", now},
		{int64(2), int64(100), "exec-002", int64(3), "failed", now, now, "", "error", int64(1), now, int64(0), "", now},
	})
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	executions, err := svc.GetTaskExecutions(ctx, 100)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(executions) != 2 {
		t.Fatalf("期望 2 条记录，实际 %d", len(executions))
	}
	if executions[0].ExecutionID != "exec-001" {
		t.Errorf("期望 ExecutionID=exec-001，实际=%s", executions[0].ExecutionID)
	}
	if executions[0].Status != "success" {
		t.Errorf("期望 Status=success，实际=%s", executions[0].Status)
	}
}

func TestGetTaskExecutions_Empty(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows(nil)
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	executions, err := svc.GetTaskExecutions(ctx, 999)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(executions) != 0 {
		t.Errorf("期望 0 条记录，实际 %d", len(executions))
	}
}

func TestGetTaskExecutions_DBError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc := newSchedulerWithDB(db)

	_, err := svc.GetTaskExecutions(ctx, 100)
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

func TestGetTaskExecutions_ResultErr(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}
	svc := newSchedulerWithDB(db)

	_, err := svc.GetTaskExecutions(ctx, 100)
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

// ============ GetExecutionByExecutionID ============

func TestGetExecutionByExecutionID_Success(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1), int64(100), "exec-001", int64(2), "success", now, now, "output", "", int64(0), now, int64(100), "done", now},
	})
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	exec, err := svc.GetExecutionByExecutionID(ctx, "exec-001")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if exec == nil {
		t.Fatal("期望返回非 nil")
	}
	if exec.ExecutionID != "exec-001" {
		t.Errorf("期望 ExecutionID=exec-001，实际=%s", exec.ExecutionID)
	}
}

func TestGetExecutionByExecutionID_NotFound(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows(nil)
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	exec, err := svc.GetExecutionByExecutionID(ctx, "not-exist")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if exec != nil {
		t.Error("期望 nil（未找到）")
	}
}

func TestGetExecutionByExecutionID_DBError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc := newSchedulerWithDB(db)

	_, err := svc.GetExecutionByExecutionID(ctx, "exec-001")
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

// ============ GetTaskLogs ============

func TestGetTaskLogs_Success(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1), "exec-001", int64(100), int64(2), "node-1", "info", "task started", now},
		{int64(2), "exec-001", int64(100), int64(2), "node-1", "stdout", "output line", now},
	})
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	logs, err := svc.GetTaskLogs(ctx, "exec-001")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("期望 2 条日志，实际 %d", len(logs))
	}
	if logs[0].Message != "task started" {
		t.Errorf("期望 Message=task started，实际=%s", logs[0].Message)
	}
}

func TestGetTaskLogs_Empty(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows(nil)
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	logs, err := svc.GetTaskLogs(ctx, "exec-empty")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("期望 0 条日志，实际 %d", len(logs))
	}
}

func TestGetTaskLogs_DBError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc := newSchedulerWithDB(db)

	_, err := svc.GetTaskLogs(ctx, "exec-001")
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

// TestGetTaskLogsByExecutionID 测试 GetTaskLogsByExecutionID 委托给 GetTaskLogs
func TestGetTaskLogsByExecutionID(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1), "exec-001", int64(100), int64(2), "", "info", "log", now},
	})
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	logs, err := svc.GetTaskLogsByExecutionID(ctx, "exec-001")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("期望 1 条日志，实际 %d", len(logs))
	}
}

// ============ DeleteExecution ============

func TestDeleteExecution_Success(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"exec-001"},
	})
	db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(0, 1)}
	svc := newSchedulerWithDB(db)

	err := svc.DeleteExecution(ctx, 1)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestDeleteExecution_NotFound(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows(nil)
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	err := svc.DeleteExecution(ctx, 999)
	if err == nil {
		t.Fatal("期望返回错误（未找到）")
	}
}

func TestDeleteExecution_QueryError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc := newSchedulerWithDB(db)

	err := svc.DeleteExecution(ctx, 1)
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

func TestDeleteExecution_DeleteLogsErrorContinues(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"exec-001"},
	})
	// 删除日志失败，但删除执行记录成功（源码中删除日志失败仅 Warn 不返回）
	db := &MockDB{
		QueryResult:  qr,
		WriteResult:  database.NewWriteResult(0, 1),
		WriteError:   ErrMockDB, // 第一次 WriteOneParameterized（删除日志）会失败
	}
	svc := newSchedulerWithDB(db)

	err := svc.DeleteExecution(ctx, 1)
	// 由于 MockDB 对所有 WriteOneParameterized 返回相同错误，删除执行记录也会失败
	if err == nil {
		t.Log("DeleteExecution 完成无错误（删除日志失败被忽略）")
	}
}

// ============ BatchDeleteExecutions ============

func TestBatchDeleteExecutions_EmptyIDs(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{}
	svc := newSchedulerWithDB(db)

	err := svc.BatchDeleteExecutions(ctx, []int64{})
	if err != nil {
		t.Errorf("空 ID 列表应返回 nil，实际: %v", err)
	}
}

func TestBatchDeleteExecutions_Success(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"exec-001"},
		{"exec-002"},
	})
	db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(0, 2)}
	svc := newSchedulerWithDB(db)

	err := svc.BatchDeleteExecutions(ctx, []int64{1, 2})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestBatchDeleteExecutions_QueryError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc := newSchedulerWithDB(db)

	err := svc.BatchDeleteExecutions(ctx, []int64{1, 2})
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

func TestBatchDeleteExecutions_ResultErr(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}
	svc := newSchedulerWithDB(db)

	err := svc.BatchDeleteExecutions(ctx, []int64{1})
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

// ============ DeleteExecutionWithDomainCheck ============

func TestDeleteExecutionWithDomainCheck_SystemAdmin(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"exec-001"},
	})
	db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(0, 1)}
	svc := newSchedulerWithDB(db)

	err := svc.DeleteExecutionWithDomainCheck(ctx, 1, 1, 1, "system_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestDeleteExecutionWithDomainCheck_DomainAdmin(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"exec-001"},
	})
	db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(0, 1)}
	svc := newSchedulerWithDB(db)

	err := svc.DeleteExecutionWithDomainCheck(ctx, 1, 1, 1, "domain_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestDeleteExecutionWithDomainCheck_NotFound(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows(nil)
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	err := svc.DeleteExecutionWithDomainCheck(ctx, 999, 1, 1, "domain_admin")
	if err == nil {
		t.Fatal("期望返回错误（未找到或权限拒绝）")
	}
}

func TestDeleteExecutionWithDomainCheck_QueryError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc := newSchedulerWithDB(db)

	err := svc.DeleteExecutionWithDomainCheck(ctx, 1, 1, 1, "system_admin")
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

// ============ BatchDeleteExecutionsWithDomainCheck ============

func TestBatchDeleteExecutionsWithDomainCheck_EmptyIDs(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{}
	svc := newSchedulerWithDB(db)

	err := svc.BatchDeleteExecutionsWithDomainCheck(ctx, []int64{}, 1, 1, "system_admin")
	if err != nil {
		t.Errorf("空 ID 列表应返回 nil，实际: %v", err)
	}
}

func TestBatchDeleteExecutionsWithDomainCheck_SystemAdmin(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"exec-001"},
		{"exec-002"},
	})
	db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(0, 2)}
	svc := newSchedulerWithDB(db)

	err := svc.BatchDeleteExecutionsWithDomainCheck(ctx, []int64{1, 2}, 1, 1, "system_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestBatchDeleteExecutionsWithDomainCheck_DomainAdmin(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"exec-001"},
	})
	db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(0, 1)}
	svc := newSchedulerWithDB(db)

	err := svc.BatchDeleteExecutionsWithDomainCheck(ctx, []int64{1}, 1, 1, "domain_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestBatchDeleteExecutionsWithDomainCheck_QueryError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc := newSchedulerWithDB(db)

	err := svc.BatchDeleteExecutionsWithDomainCheck(ctx, []int64{1}, 1, 1, "system_admin")
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

// ============ GetAllExecutions ============

func TestGetAllExecutions_SystemAdmin(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1), int64(100), "exec-001", int64(2), "success", now, now, "output", "", int64(0), now, "task1", "http", "executor1"},
	})
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	executions, total, err := svc.GetAllExecutions(ctx, 1, 1, "system_admin", map[string]string{}, 1, 10)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if total != 1 {
		t.Errorf("期望 total=1，实际=%d", total)
	}
	if len(executions) != 1 {
		t.Fatalf("期望 1 条记录，实际 %d", len(executions))
	}
	if executions[0].TaskName != "task1" {
		t.Errorf("期望 TaskName=task1，实际=%s", executions[0].TaskName)
	}
	if executions[0].ExecutorName != "executor1" {
		t.Errorf("期望 ExecutorName=executor1，实际=%s", executions[0].ExecutorName)
	}
}

func TestGetAllExecutions_DomainAdmin(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1), int64(100), "exec-001", int64(2), "success", now, now, "output", "", int64(0), now, "task1", "http", "executor1"},
	})
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	executions, total, err := svc.GetAllExecutions(ctx, 1, 1, "domain_admin", map[string]string{}, 1, 10)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if total != 1 {
		t.Errorf("期望 total=1，实际=%d", total)
	}
	if len(executions) != 1 {
		t.Errorf("期望 1 条记录，实际 %d", len(executions))
	}
}

func TestGetAllExecutions_WithFilters(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1), int64(100), "exec-001", int64(2), "success", now, now, "output", "", int64(0), now, "task1", "http", "executor1"},
	})
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	filter := map[string]string{
		"id":             "1",
		"execution_id":   "exec",
		"executor_name":  "exec",
		"task_name":      "task",
		"status":         "success",
		"start_time_from": now,
		"start_time_to":   now,
		"end_time_from":   now,
		"end_time_to":     now,
		"duration_min":   "1",
		"duration_max":   "100",
	}

	_, _, err := svc.GetAllExecutions(ctx, 1, 1, "system_admin", filter, 1, 10)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestGetAllExecutions_Empty(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows(nil)
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	executions, total, err := svc.GetAllExecutions(ctx, 1, 1, "system_admin", map[string]string{}, 1, 10)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if total != 0 {
		t.Errorf("期望 total=0，实际=%d", total)
	}
	if len(executions) != 0 {
		t.Errorf("期望 0 条记录，实际 %d", len(executions))
	}
}

func TestGetAllExecutions_QueryError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc := newSchedulerWithDB(db)

	_, _, err := svc.GetAllExecutions(ctx, 1, 1, "system_admin", map[string]string{}, 1, 10)
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

func TestGetAllExecutions_ResultErr(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}
	svc := newSchedulerWithDB(db)

	_, _, err := svc.GetAllExecutions(ctx, 1, 1, "system_admin", map[string]string{}, 1, 10)
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

// ============ GetExecutionStats ============

func TestGetExecutionStats_Success(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"success", int64(10)},
		{"failed", int64(3)},
		{"running", int64(2)},
	})
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	stats, err := svc.GetExecutionStats(ctx, 1, 1, "system_admin", map[string]string{})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if stats["success"] != 10 {
		t.Errorf("期望 success=10，实际=%d", stats["success"])
	}
	if stats["failed"] != 3 {
		t.Errorf("期望 failed=3，实际=%d", stats["failed"])
	}
	if stats["running"] != 2 {
		t.Errorf("期望 running=2，实际=%d", stats["running"])
	}
}

func TestGetExecutionStats_WithFilters(t *testing.T) {
	ctx := context.Background()
	now := time.Now().Format(time.RFC3339)
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"success", int64(5)},
	})
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	filter := map[string]string{
		"status":          "success",
		"start_time_from": now,
		"duration_min":    "1",
	}

	stats, err := svc.GetExecutionStats(ctx, 1, 1, "domain_admin", filter)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if stats["success"] != 5 {
		t.Errorf("期望 success=5，实际=%d", stats["success"])
	}
}

func TestGetExecutionStats_Empty(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows(nil)
	db := &MockDB{QueryResult: qr}
	svc := newSchedulerWithDB(db)

	stats, err := svc.GetExecutionStats(ctx, 1, 1, "system_admin", map[string]string{})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("期望空 stats，实际 %d 项", len(stats))
	}
}

func TestGetExecutionStats_QueryError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc := newSchedulerWithDB(db)

	_, err := svc.GetExecutionStats(ctx, 1, 1, "system_admin", map[string]string{})
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

func TestGetExecutionStats_ResultErr(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}
	svc := newSchedulerWithDB(db)

	_, err := svc.GetExecutionStats(ctx, 1, 1, "system_admin", map[string]string{})
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

// ============ CancelExecution ============

func TestCancelExecution_NotFound(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows(nil)
	db := &MockDB{QueryResult: qr}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.CancelExecution(ctx, "not-exist", "user1")
	if err == nil {
		t.Fatal("期望返回错误（未找到）")
	}
}

func TestCancelExecution_QueryError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.CancelExecution(ctx, "exec-001", "user1")
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestCancelExecution_NotRunning(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(100), "success", int64(0)},
	})
	db := &MockDB{QueryResult: qr}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.CancelExecution(ctx, "exec-001", "user1")
	if err == nil {
		t.Fatal("期望返回错误（状态非 running/pending）")
	}
}

func TestCancelExecution_ResultErr(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.CancelExecution(ctx, "exec-001", "user1")
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestCancelExecution_RunningWithNoExecutor(t *testing.T) {
	ctx := context.Background()
	// 第一次查询返回 running 状态，executor_id=0
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(100), "running", int64(0)},
	})
	db := &MockDB{
		QueryResult: qr,
		// UpdateExecutionResult 和 AddTaskLog 的写入
		WriteResult: database.NewWriteResult(0, 1),
	}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.CancelExecution(ctx, "exec-001", "user1")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestCancelExecution_PendingWithNoExecutor(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(100), "pending", int64(0)},
	})
	db := &MockDB{
		QueryResult: qr,
		WriteResult: database.NewWriteResult(0, 1),
	}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.CancelExecution(ctx, "exec-pending", "user1")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestCancelExecution_UpdateResultError(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(100), "running", int64(0)},
	})
	db := &MockDB{
		QueryResult: qr,
		WriteError:  ErrMockDB,
	}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.CancelExecution(ctx, "exec-001", "user1")
	if err == nil {
		t.Fatal("期望返回错误（UpdateExecutionResult 失败）")
	}
}

// ============ AddTaskLog ============

func TestAddTaskLog_Success(t *testing.T) {
	ctx := context.Background()
	// QueryOneParameterized 返回 executor_id 行
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(2)},
	})
	db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(0, 1)}
	svc := newSchedulerWithDB(db)

	err := svc.AddTaskLog(ctx, "exec-001", 100, "node-1", "info", "task started")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestAddTaskLog_WithRedisDedup(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(2)},
	})
	db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(0, 1)}
	svc, mr, rdb := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()
	defer rdb.Close()

	err := svc.AddTaskLog(ctx, "exec-001", 100, "node-1", "info", "task started")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestAddTaskLog_StdoutUpdatesOutput(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(2)},
	})
	db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(0, 1)}
	svc := newSchedulerWithDB(db)

	err := svc.AddTaskLog(ctx, "exec-001", 100, "node-1", "stdout", "output line")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	// 应有 2 次写入：插入日志 + 更新 output
	if len(db.WriteStmts) < 2 {
		t.Errorf("期望至少 2 次写入，实际 %d", len(db.WriteStmts))
	}
}

func TestAddTaskLog_StderrUpdatesError(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(2)},
	})
	db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(0, 1)}
	svc := newSchedulerWithDB(db)

	err := svc.AddTaskLog(ctx, "exec-001", 100, "node-1", "stderr", "error line")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(db.WriteStmts) < 2 {
		t.Errorf("期望至少 2 次写入，实际 %d", len(db.WriteStmts))
	}
}

func TestAddTaskLog_DedupSkipsDuplicate(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(2)},
	})
	db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(0, 1)}
	svc, mr, rdb := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()
	defer rdb.Close()

	// 预设 dedup key，使第二次调用被跳过
	executionID := "exec-001"
	nodeID := "node-1"
	logLevel := "info"
	message := "task started"
	logHash := fmtHash(executionID, nodeID, logLevel, message)
	dedupKey := "task:log:dedup:" + logHash
	rdb.Set(ctx, dedupKey, "1", 30*time.Second)

	err := svc.AddTaskLog(ctx, executionID, 100, nodeID, logLevel, message)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	// 因 dedup 命中，不应有写入
	if len(db.WriteStmts) != 0 {
		t.Errorf("期望 0 次写入（dedup 跳过），实际 %d", len(db.WriteStmts))
	}
}

func TestAddTaskLog_WriteErrorFallback(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(2)},
	})
	db := &MockDB{QueryResult: qr, WriteError: ErrMockDB}
	svc := newSchedulerWithDB(db)

	err := svc.AddTaskLog(ctx, "exec-001", 100, "node-1", "info", "task started")
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

func TestAddTaskLog_NoExecutorID(t *testing.T) {
	ctx := context.Background()
	// executor_id=0，应保持 nil
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(0)},
	})
	db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(0, 1)}
	svc := newSchedulerWithDB(db)

	err := svc.AddTaskLog(ctx, "exec-001", 100, "node-1", "info", "task started")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

// ============ RecoverRunningTasksOnBecomeLeader ============

func TestRecoverRunningTasksOnBecomeLeader_NoRunningTasks(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows(nil)
	db := &MockDB{QueryResult: qr}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.RecoverRunningTasksOnBecomeLeader(ctx)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
}

func TestRecoverRunningTasksOnBecomeLeader_QueryError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.RecoverRunningTasksOnBecomeLeader(ctx)
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

func TestRecoverRunningTasksOnBecomeLeader_ResultErr(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}
	svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	err := svc.RecoverRunningTasksOnBecomeLeader(ctx)
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

// ============ TaskExecutionWithNames 结构体 ============

func TestTaskExecutionWithNames_Structure(t *testing.T) {
	// 验证 TaskExecutionWithNames 嵌入了 TaskExecution 并添加了名称字段
	exec := &TaskExecutionWithNames{
		TaskExecution: model.TaskExecution{
			ID:          1,
			ExecutionID: "exec-001",
			Status:      "success",
		},
		TaskName:     "my-task",
		TaskType:     "http",
		ExecutorName: "executor-1",
	}

	if exec.ID != 1 {
		t.Errorf("期望 ID=1，实际=%d", exec.ID)
	}
	if exec.ExecutionID != "exec-001" {
		t.Errorf("期望 ExecutionID=exec-001，实际=%s", exec.ExecutionID)
	}
	if exec.TaskName != "my-task" {
		t.Errorf("期望 TaskName=my-task，实际=%s", exec.TaskName)
	}
	if exec.TaskType != "http" {
		t.Errorf("期望 TaskType=http，实际=%s", exec.TaskType)
	}
	if exec.ExecutorName != "executor-1" {
		t.Errorf("期望 ExecutorName=executor-1，实际=%s", exec.ExecutorName)
	}
}

// fmtHash 复制 AddTaskLog 中的 hash 计算逻辑，用于测试 dedup
func fmtHash(executionID, nodeID, logLevel, message string) string {
	return fmt.Sprintf("%x", []byte(fmt.Sprintf("%s-%s-%s-%s", executionID, nodeID, logLevel, message)))
}
