package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// apiTestRow 构造一行 api_test 查询结果（7 列）
// 列顺序：0=id 1=name 2=type 3=config 4=created_by 5=created_at 6=updated_at
func apiTestRow(id int64, name, tType, config string, createdBy int64) []interface{} {
	return []interface{}{id, name, tType, config, createdBy, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"}
}

// apiTestCountRow 构造一行 COUNT 查询结果（1 列）
func apiTestCountRow(count int64) []interface{} {
	return []interface{}{count}
}

// apiTestResultRow 构造一行 api_test_result 查询结果（11 列，GetResults/GetResultByID）
// 列顺序：0=id 1=test_id 2=type 3=status_code 4=latency_ms 5=headers
//
//	6=body 7=error 8=assertions_result 9=executed_by 10=created_at
func apiTestResultRow(id, testID int64, tType string, statusCode int, latencyMs int64, headers, body, errMsg, assertions string, executedBy int64) []interface{} {
	return []interface{}{id, testID, tType, statusCode, latencyMs, headers, body, errMsg, assertions, executedBy, "2026-01-01T00:00:00Z"}
}

// apiTestResultRowWithNames 构造一行带 test_name/executed_by_name 的查询结果（13 列，ListResultsByUser）
func apiTestResultRowWithNames(id, testID int64, tType string, statusCode int, latencyMs int64, headers, body, errMsg, assertions string, executedBy int64, testName, executedByName string) []interface{} {
	return []interface{}{id, testID, tType, statusCode, latencyMs, headers, body, errMsg, assertions, executedBy, "2026-01-01T00:00:00Z", testName, executedByName}
}

func TestNewApiTestService(t *testing.T) {
	t.Run("构造函数正常赋值", func(t *testing.T) {
		db := &MockDB{}
		svc := NewApiTestService(db)
		if svc == nil {
			t.Fatal("期望返回非 nil 实例")
		}
		if svc.db == nil {
			t.Error("期望 db 正确赋值")
		}
	})

	t.Run("nil db 也可构造", func(t *testing.T) {
		svc := NewApiTestService(nil)
		if svc == nil {
			t.Fatal("期望返回非 nil 实例")
		}
	})
}

func TestApiTestService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("创建成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(10, 1),
		}
		svc := NewApiTestService(db)
		test := &model.ApiTest{
			Name:      "test-1",
			Type:      "http",
			Config:    `{"method":"GET"}`,
			CreatedBy: 100,
		}
		created, err := svc.Create(ctx, test)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if created.ID != 10 {
			t.Errorf("期望 ID=10，实际=%d", created.ID)
		}
		if created.Name != "test-1" {
			t.Errorf("期望 Name=test-1，实际=%s", created.Name)
		}
		if created.CreatedBy != 100 {
			t.Errorf("期望 CreatedBy=100，实际=%d", created.CreatedBy)
		}
		if created.CreatedAt.IsZero() {
			t.Error("期望 CreatedAt 已设置")
		}
		if len(db.WriteStmts) != 1 {
			t.Errorf("期望 1 次写入，实际=%d", len(db.WriteStmts))
		}
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewApiTestService(db)
		_, err := svc.Create(ctx, &model.ApiTest{Name: "test", Type: "http"})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewApiTestService(db)
		_, err := svc.Create(ctx, &model.ApiTest{Name: "test", Type: "http"})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

func TestApiTestService_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("更新成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewApiTestService(db)
		err := svc.Update(ctx, 5, &model.ApiTest{Name: "updated", Type: "grpc", Config: "{}"})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.LastWriteStmt.Arguments[4] != int64(5) {
			t.Errorf("期望第5个参数为 id=5，实际=%v", db.LastWriteStmt.Arguments[4])
		}
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewApiTestService(db)
		err := svc.Update(ctx, 5, &model.ApiTest{Name: "updated"})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewApiTestService(db)
		err := svc.Update(ctx, 5, &model.ApiTest{Name: "updated"})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("RowsAffected为0返回not found", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 0),
		}
		svc := NewApiTestService(db)
		err := svc.Update(ctx, 999, &model.ApiTest{Name: "updated"})
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("期望 not found 错误，实际: %v", err)
		}
	})
}

func TestApiTestService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("删除成功", func(t *testing.T) {
		db := &MockDB{
			BatchWriteResult: []rqlite.WriteResult{
				{RowsAffected: 5},
				{RowsAffected: 1},
			},
		}
		svc := NewApiTestService(db)
		err := svc.Delete(ctx, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(db.WriteStmts) != 2 {
			t.Errorf("期望 2 次写入，实际=%d", len(db.WriteStmts))
		}
	})

	t.Run("批量写入失败返回错误", func(t *testing.T) {
		db := &MockDB{BatchWriteError: ErrMockDB}
		svc := NewApiTestService(db)
		err := svc.Delete(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("结果数量不足返回错误", func(t *testing.T) {
		db := &MockDB{
			BatchWriteResult: []rqlite.WriteResult{
				{RowsAffected: 1},
			},
		}
		svc := NewApiTestService(db)
		err := svc.Delete(ctx, 5)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("第二语句带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			BatchWriteResult: []rqlite.WriteResult{
				{RowsAffected: 1},
				{Err: ErrMockDB},
			},
		}
		svc := NewApiTestService(db)
		err := svc.Delete(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("第二语句RowsAffected为0返回not found", func(t *testing.T) {
		db := &MockDB{
			BatchWriteResult: []rqlite.WriteResult{
				{RowsAffected: 1},
				{RowsAffected: 0},
			},
		}
		svc := NewApiTestService(db)
		err := svc.Delete(ctx, 999)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("期望 not found 错误，实际: %v", err)
		}
	})
}

func TestApiTestService_GetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("找到记录", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				apiTestRow(1, "test-1", "http", `{"method":"GET"}`, 100),
			}),
		}
		svc := NewApiTestService(db)
		test, err := svc.GetByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if test.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", test.ID)
		}
		if test.Name != "test-1" {
			t.Errorf("期望 Name=test-1，实际=%s", test.Name)
		}
		if test.Type != "http" {
			t.Errorf("期望 Type=http，实际=%s", test.Type)
		}
		if test.CreatedBy != 100 {
			t.Errorf("期望 CreatedBy=100，实际=%d", test.CreatedBy)
		}
	})

	t.Run("记录不存在返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewApiTestService(db)
		_, err := svc.GetByID(ctx, 999)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("期望 not found 错误，实际: %v", err)
		}
	})

	t.Run("查询失败返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewApiTestService(db)
		_, err := svc.GetByID(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithErr(ErrMockDB),
		}
		svc := NewApiTestService(db)
		_, err := svc.GetByID(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

func TestApiTestService_ListByUser(t *testing.T) {
	ctx := context.Background()

	t.Run("普通用户查询成功", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(2)}),
				database.NewQueryResultWithRows([][]interface{}{
					apiTestRow(1, "test-1", "http", "{}", 100),
					apiTestRow(2, "test-2", "grpc", "{}", 100),
				}),
			},
		}
		svc := NewApiTestService(db)
		tests, total, err := svc.ListByUser(ctx, 100, false, "", 1, 20)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if total != 2 {
			t.Errorf("期望 total=2，实际=%d", total)
		}
		if len(tests) != 2 {
			t.Errorf("期望 2 条记录，实际=%d", len(tests))
		}
		// 普通用户应该带 created_by 条件
		if db.QueryStmts[0].Arguments[0] != int64(100) {
			t.Errorf("期望第1个参数为 userID=100，实际=%v", db.QueryStmts[0].Arguments[0])
		}
	})

	t.Run("管理员查询不带created_by条件", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewApiTestService(db)
		_, _, err := svc.ListByUser(ctx, 100, true, "", 1, 20)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 管理员无 WHERE 条件，Arguments 应为空
		if len(db.QueryStmts[0].Arguments) != 0 {
			t.Errorf("期望管理员查询无参数，实际有 %d 个参数", len(db.QueryStmts[0].Arguments))
		}
	})

	t.Run("带type过滤", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(1)}),
				database.NewQueryResultWithRows([][]interface{}{
					apiTestRow(1, "test-1", "http", "{}", 100),
				}),
			},
		}
		svc := NewApiTestService(db)
		tests, _, err := svc.ListByUser(ctx, 100, false, "http", 1, 20)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(tests) != 1 {
			t.Errorf("期望 1 条记录，实际=%d", len(tests))
		}
		// 普通用户 created_by + type 两个参数
		if len(db.QueryStmts[0].Arguments) != 2 {
			t.Errorf("期望 2 个参数，实际=%d", len(db.QueryStmts[0].Arguments))
		}
	})

	t.Run("默认分页参数生效", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewApiTestService(db)
		_, _, err := svc.ListByUser(ctx, 100, true, "", 0, 0)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 验证 LIMIT 20 OFFSET 0
		if !strings.Contains(db.QueryStmts[1].Query, "LIMIT 20 OFFSET 0") {
			t.Errorf("期望 LIMIT 20 OFFSET 0，实际=%s", db.QueryStmts[1].Query)
		}
	})

	t.Run("pageSize超过100被限制", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewApiTestService(db)
		_, _, err := svc.ListByUser(ctx, 100, true, "", 1, 200)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !strings.Contains(db.QueryStmts[1].Query, "LIMIT 100 OFFSET 0") {
			t.Errorf("期望 LIMIT 100 OFFSET 0，实际=%s", db.QueryStmts[1].Query)
		}
	})

	t.Run("count查询失败返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewApiTestService(db)
		_, _, err := svc.ListByUser(ctx, 100, true, "", 1, 20)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("count结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithErr(ErrMockDB),
			},
		}
		svc := NewApiTestService(db)
		_, _, err := svc.ListByUser(ctx, 100, true, "", 1, 20)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("数据查询失败返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(0)}),
			},
			QueryError: ErrMockDB, // 会在第一次查询就失败，需用不同策略
		}
		// 由于 QueryError 优先，第一次查询就会失败
		svc := NewApiTestService(db)
		_, _, err := svc.ListByUser(ctx, 100, true, "", 1, 20)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

func TestApiTestService_SaveResult(t *testing.T) {
	ctx := context.Background()

	t.Run("保存成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(20, 1),
		}
		svc := NewApiTestService(db)
		result := &model.ApiTestResult{
			TestID:          1,
			Type:            "http",
			StatusCode:      200,
			LatencyMs:       50,
			Headers:         `{"Content-Type":"application/json"}`,
			Body:            `{"ok":true}`,
			Error:           "",
			AssertionsResult: "[]",
			ExecutedBy:      100,
		}
		saved, err := svc.SaveResult(ctx, result)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if saved.ID != 20 {
			t.Errorf("期望 ID=20，实际=%d", saved.ID)
		}
		if saved.CreatedAt.IsZero() {
			t.Error("期望 CreatedAt 已设置")
		}
	})

	t.Run("超大body被截断", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(1, 1),
		}
		svc := NewApiTestService(db)
		// 构造超过 100KB 的 body
		bigBody := strings.Repeat("a", 100*1024+100)
		result := &model.ApiTestResult{
			TestID: 1,
			Type:   "http",
			Body:   bigBody,
		}
		_, err := svc.SaveResult(ctx, result)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 写入的 body 参数应该被截断
		writtenBody, ok := db.LastWriteStmt.Arguments[5].(string)
		if !ok {
			t.Fatal("期望第6个参数为 string")
		}
		if !strings.Contains(writtenBody, "[truncated]") {
			t.Error("期望 body 包含 [truncated] 标记")
		}
		if len(writtenBody) > 100*1024+50 {
			t.Errorf("body 应被截断，实际长度=%d", len(writtenBody))
		}
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewApiTestService(db)
		_, err := svc.SaveResult(ctx, &model.ApiTestResult{TestID: 1})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewApiTestService(db)
		_, err := svc.SaveResult(ctx, &model.ApiTestResult{TestID: 1})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

func TestApiTestService_GetResults(t *testing.T) {
	ctx := context.Background()

	t.Run("查询成功", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(2)}),
				database.NewQueryResultWithRows([][]interface{}{
					apiTestResultRow(1, 5, "http", 200, 50, "{}", "body1", "", "[]", 100),
					apiTestResultRow(2, 5, "http", 500, 100, "{}", "", "server error", "[]", 100),
				}),
			},
		}
		svc := NewApiTestService(db)
		results, total, err := svc.GetResults(ctx, 5, 1, 20)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if total != 2 {
			t.Errorf("期望 total=2，实际=%d", total)
		}
		if len(results) != 2 {
			t.Errorf("期望 2 条结果，实际=%d", len(results))
		}
		if results[0].StatusCode != 200 {
			t.Errorf("期望 StatusCode=200，实际=%d", results[0].StatusCode)
		}
		if results[0].LatencyMs != 50 {
			t.Errorf("期望 LatencyMs=50，实际=%d", results[0].LatencyMs)
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewApiTestService(db)
		results, total, err := svc.GetResults(ctx, 999, 1, 20)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if total != 0 {
			t.Errorf("期望 total=0，实际=%d", total)
		}
		if len(results) != 0 {
			t.Errorf("期望 0 条结果，实际=%d", len(results))
		}
	})

	t.Run("默认分页参数生效", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewApiTestService(db)
		_, _, err := svc.GetResults(ctx, 5, 0, 0)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !strings.Contains(db.QueryStmts[1].Query, "LIMIT 20 OFFSET 0") {
			t.Errorf("期望 LIMIT 20 OFFSET 0，实际=%s", db.QueryStmts[1].Query)
		}
	})

	t.Run("count查询失败返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewApiTestService(db)
		_, _, err := svc.GetResults(ctx, 5, 1, 20)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("count结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithErr(ErrMockDB),
			},
		}
		svc := NewApiTestService(db)
		_, _, err := svc.GetResults(ctx, 5, 1, 20)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

func TestApiTestService_ListResultsByUser(t *testing.T) {
	ctx := context.Background()

	t.Run("管理员查询成功带testName", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(1)}),
				database.NewQueryResultWithRows([][]interface{}{
					apiTestResultRowWithNames(1, 5, "http", 200, 50, "{}", "body", "", "[]", 100, "test-1", "Alice"),
				}),
			},
		}
		svc := NewApiTestService(db)
		results, total, err := svc.ListResultsByUser(ctx, 100, true, "", 1, 20)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if total != 1 {
			t.Errorf("期望 total=1，实际=%d", total)
		}
		if len(results) != 1 {
			t.Fatalf("期望 1 条结果，实际=%d", len(results))
		}
		if results[0].TestName != "test-1" {
			t.Errorf("期望 TestName=test-1，实际=%s", results[0].TestName)
		}
		if results[0].ExecutedByName != "Alice" {
			t.Errorf("期望 ExecutedByName=Alice，实际=%s", results[0].ExecutedByName)
		}
	})

	t.Run("普通用户带executed_by条件", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewApiTestService(db)
		_, _, err := svc.ListResultsByUser(ctx, 100, false, "", 1, 20)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(db.QueryStmts[0].Arguments) != 1 {
			t.Errorf("期望 1 个参数（userID），实际=%d", len(db.QueryStmts[0].Arguments))
		}
	})

	t.Run("带search参数按名称模糊匹配", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewApiTestService(db)
		_, _, err := svc.ListResultsByUser(ctx, 100, true, "", 1, 20, "keyword")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 第一个参数应为 %keyword%
		if db.QueryStmts[0].Arguments[0] != "%keyword%" {
			t.Errorf("期望 %%keyword%%，实际=%v", db.QueryStmts[0].Arguments[0])
		}
	})

	t.Run("带status=success过滤", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewApiTestService(db)
		_, _, err := svc.ListResultsByUser(ctx, 100, true, "", 1, 20, "", "success")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !strings.Contains(db.QueryStmts[0].Query, "r.error = ''") {
			t.Errorf("期望包含 success 条件，实际=%s", db.QueryStmts[0].Query)
		}
	})

	t.Run("带status=error过滤", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{apiTestCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewApiTestService(db)
		_, _, err := svc.ListResultsByUser(ctx, 100, true, "", 1, 20, "", "error")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !strings.Contains(db.QueryStmts[0].Query, "r.error != ''") {
			t.Errorf("期望包含 error 条件，实际=%s", db.QueryStmts[0].Query)
		}
	})

	t.Run("count查询失败返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewApiTestService(db)
		_, _, err := svc.ListResultsByUser(ctx, 100, true, "", 1, 20)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("count结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithErr(ErrMockDB),
			},
		}
		svc := NewApiTestService(db)
		_, _, err := svc.ListResultsByUser(ctx, 100, true, "", 1, 20)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

func TestApiTestService_DeleteResult(t *testing.T) {
	ctx := context.Background()

	t.Run("删除成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewApiTestService(db)
		err := svc.DeleteResult(ctx, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.LastWriteStmt.Arguments[0] != int64(5) {
			t.Errorf("期望第1个参数为 id=5，实际=%v", db.LastWriteStmt.Arguments[0])
		}
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewApiTestService(db)
		err := svc.DeleteResult(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewApiTestService(db)
		err := svc.DeleteResult(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("RowsAffected为0返回not found", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 0),
		}
		svc := NewApiTestService(db)
		err := svc.DeleteResult(ctx, 999)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("期望 not found 错误，实际: %v", err)
		}
	})
}

func TestApiTestService_GetResultByID(t *testing.T) {
	ctx := context.Background()

	t.Run("找到记录", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				apiTestResultRow(1, 5, "http", 200, 50, `{"H":"v"}`, "body", "", "[]", 100),
			}),
		}
		svc := NewApiTestService(db)
		result, err := svc.GetResultByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if result.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", result.ID)
		}
		if result.TestID != 5 {
			t.Errorf("期望 TestID=5，实际=%d", result.TestID)
		}
		if result.StatusCode != 200 {
			t.Errorf("期望 StatusCode=200，实际=%d", result.StatusCode)
		}
		if result.Headers != `{"H":"v"}` {
			t.Errorf("期望 Headers 正确，实际=%s", result.Headers)
		}
		if result.Body != "body" {
			t.Errorf("期望 Body=body，实际=%s", result.Body)
		}
	})

	t.Run("记录不存在返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewApiTestService(db)
		_, err := svc.GetResultByID(ctx, 999)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("期望 not found 错误，实际: %v", err)
		}
	})

	t.Run("查询失败返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewApiTestService(db)
		_, err := svc.GetResultByID(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithErr(ErrMockDB),
		}
		svc := NewApiTestService(db)
		_, err := svc.GetResultByID(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}
