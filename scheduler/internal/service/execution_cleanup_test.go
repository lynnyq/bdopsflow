package service

import (
	"context"
	"testing"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// ============ CleanExpiredExecutions ============

func TestCleanExpiredExecutions_RetentionDaysZero_Skip(t *testing.T) {
	db := &MockDB{}
	svc := newSchedulerWithDB(db)

	deleted, err := svc.CleanExpiredExecutions(context.Background(), ExecutionCleanupFilter{RetentionDays: 0})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("期望删除 0 条，实际: %d", deleted)
	}
	if len(db.QueryStmts) > 0 || len(db.WriteStmts) > 0 {
		t.Fatalf("retention_days=0 时不应执行任何 DB 操作")
	}
}

func TestCleanExpiredExecutions_NoExpiredRecords_ReturnZero(t *testing.T) {
	db := &MockDB{
		QueryResult: database.NewQueryResultWithRows([][]interface{}{}),
	}
	svc := newSchedulerWithDB(db)

	deleted, err := svc.CleanExpiredExecutions(context.Background(), ExecutionCleanupFilter{RetentionDays: 30})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("期望删除 0 条，实际: %d", deleted)
	}
}

func TestCleanExpiredExecutions_ExpiredRecordsDeletedWithCascade(t *testing.T) {
	db := &MockDB{}
	svc := newSchedulerWithDB(db)

	// 让第一次 Query 有 2 条，第二次（循环检查）空
	db.QueryResults = []rqlite.QueryResult{
		database.NewQueryResultWithRows([][]interface{}{
			{"exec-001"},
			{"exec-002"},
		}),
		database.NewQueryResultWithRows([][]interface{}{}), // 第二次循环没有过期数据了
	}
	// 设置 BatchWriteResult 让第二次 RowsAffected 有值
	db.BatchWriteResult = []rqlite.WriteResult{
		database.NewWriteResult(0, 2), // 删过程日志
		database.NewWriteResult(0, 2), // 删执行记录
	}

	deleted, err := svc.CleanExpiredExecutions(context.Background(), ExecutionCleanupFilter{RetentionDays: 30})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("期望删除 2 条执行记录，实际: %d", deleted)
	}

	// 验证 WriteParameterized 被调用且包含两条 DELETE
	if len(db.WriteStmts) != 2 {
		t.Fatalf("期望 2 次写入（删日志 + 删执行记录），实际: %d", len(db.WriteStmts))
	}
}

func TestCleanExpiredExecutions_WithContextTimeout_ReturnsDeletedCount(t *testing.T) {
	db := &MockDB{
		QueryResult: database.NewQueryResultWithRows([][]interface{}{
			{"exec-001"},
		}),
		WriteResult: database.NewWriteResult(0, 1),
	}
	svc := newSchedulerWithDB(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	deleted, err := svc.CleanExpiredExecutions(ctx, ExecutionCleanupFilter{RetentionDays: 30})
	if err == nil {
		t.Fatalf("期望 context 取消错误")
	}
	// deleted 可能是 0（取消太快），也可能是 1（已完成）
	if deleted < 0 {
		t.Fatalf("deleted count 不应为负")
	}
}

func TestCleanExpiredExecutions_WithTaskNameFilter(t *testing.T) {
	db := &MockDB{
		QueryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{
				{"exec-001"},
			}),
			database.NewQueryResultWithRows([][]interface{}{}),
		},
		BatchWriteResult: []rqlite.WriteResult{
			database.NewWriteResult(0, 1),
			database.NewWriteResult(0, 1),
		},
	}
	svc := newSchedulerWithDB(db)

	deleted, err := svc.CleanExpiredExecutions(context.Background(), ExecutionCleanupFilter{
		RetentionDays: 30,
		TaskName:      "backup",
	})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("期望删除 1 条，实际: %d", deleted)
	}

	// 验证 SQL 中包含 JOIN bdopsflow_tasks
	if len(db.QueryStmts) > 0 {
		querySQL := db.QueryStmts[0].Query
		if !containsString(querySQL, "bdopsflow_tasks") {
			t.Fatalf("TaskName 筛选应 JOIN bdopsflow_tasks，实际 SQL: %s", querySQL)
		}
		if !containsString(querySQL, "LIKE") {
			t.Fatalf("TaskName 筛选应使用 LIKE，实际 SQL: %s", querySQL)
		}
	}
}

// ============ CleanExpiredTaskLogs ============

func TestCleanExpiredTaskLogs_RetentionDaysZero_Skip(t *testing.T) {
	db := &MockDB{}
	svc := newSchedulerWithDB(db)

	deleted, err := svc.CleanExpiredTaskLogs(context.Background(), TaskLogCleanupFilter{RetentionDays: 0})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("期望删除 0 条，实际: %d", deleted)
	}
}

func TestCleanExpiredTaskLogs_ExpiredLogsDeleted(t *testing.T) {
	db := &MockDB{
		WriteResult: database.NewWriteResult(0, 5), // 一次删 5 条，< batchSize 会 break
	}
	svc := newSchedulerWithDB(db)

	deleted, err := svc.CleanExpiredTaskLogs(context.Background(), TaskLogCleanupFilter{RetentionDays: 30})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if deleted != 5 {
		t.Fatalf("期望删除 5 条，实际: %d", deleted)
	}

	// 验证只有 WriteStmts（子查询在 WRITE SQL 内部，不需要额外 query）
	if len(db.WriteStmts) != 1 {
		t.Fatalf("期望 1 次写入，实际: %d", len(db.WriteStmts))
	}
}

func TestCleanExpiredTaskLogs_NoExpiredLogs_ReturnZero(t *testing.T) {
	db := &MockDB{
		WriteResult: database.NewWriteResult(0, 0),
	}
	svc := newSchedulerWithDB(db)

	deleted, err := svc.CleanExpiredTaskLogs(context.Background(), TaskLogCleanupFilter{RetentionDays: 30})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("期望删除 0 条，实际: %d", deleted)
	}
}

func TestCleanExpiredTaskLogs_WithExecutorNameFilter(t *testing.T) {
	db := &MockDB{
		WriteResult: database.NewWriteResult(0, 3),
	}
	svc := newSchedulerWithDB(db)

	deleted, err := svc.CleanExpiredTaskLogs(context.Background(), TaskLogCleanupFilter{
		RetentionDays: 30,
		ExecutorName:  "exec-01",
	})
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("期望删除 3 条，实际: %d", deleted)
	}

	if len(db.WriteStmts) > 0 {
		writeSQL := db.WriteStmts[0].Query
		if !containsString(writeSQL, "bdopsflow_executors") {
			t.Fatalf("ExecutorName 筛选应 JOIN bdopsflow_executors，实际 SQL: %s", writeSQL)
		}
	}
}

// ============ 配置读取方法 ============

func TestGetExecutionRetentionDays_FallbackDefault(t *testing.T) {
	db := &MockDB{}
	svc := newSchedulerWithDB(db)

	days := svc.GetExecutionRetentionDays()
	if days != 30 {
		t.Fatalf("默认值应为 30，实际: %d", days)
	}
}

func TestGetTaskLogRetentionDays_FallbackDefault(t *testing.T) {
	db := &MockDB{}
	svc := newSchedulerWithDB(db)

	days := svc.GetTaskLogRetentionDays()
	if days != 30 {
		t.Fatalf("默认值应为 30，实际: %d", days)
	}
}

// ============ 辅助函数 ============

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// 确保 time 包在测试中被引用（避免 import 警告）
var _ = time.Now
