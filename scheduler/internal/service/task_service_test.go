package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// taskRow18 构造一行 18 列的任务数据（含 created_by_name）
// 列顺序: id, name, type, config, cron_expression, timeout_seconds, retry_count,
//
//	retry_interval, is_enabled, status, domain_id, webhook_id, webhook_events,
//	assigned_executor_id, created_by, created_by_name, created_at, updated_at
func taskRow18(id int64, name string) []interface{} {
	return []interface{}{
		id, name, "http", `{"url":"http://example.com"}`, "*/5 * * * *", int64(300), int64(3), int64(60),
		true, "enabled", int64(1), nil, "", int64(0), int64(1), "admin", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z",
	}
}

// ============ GetTaskByID ============

func TestSchedulerService_GetTaskByID(t *testing.T) {
	ctx := context.Background()

	t.Run("找到任务", func(t *testing.T) {
		// GetTaskByID 查询 18 列 + getLastExecutionStatus 查询 1 列
		// MockDB 对两者返回同一 QueryResult
		qr := database.NewQueryResultWithRows([][]interface{}{
			taskRow18(1, "task-1"),
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		task, err := svc.GetTaskByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if task.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", task.ID)
		}
		if task.Name != "task-1" {
			t.Errorf("期望 Name=task-1，实际=%s", task.Name)
		}
		if task.Type != "http" {
			t.Errorf("期望 Type=http，实际=%s", task.Type)
		}
		if !task.IsEnabled {
			t.Error("期望 IsEnabled=true")
		}
		if task.CreatedByName != "admin" {
			t.Errorf("期望 CreatedByName=admin，实际=%s", task.CreatedByName)
		}
	})

	t.Run("任务不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)

		_, err := svc.GetTaskByID(ctx, 999)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		_, err := svc.GetTaskByID(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := newSchedulerWithDB(db)

		_, err := svc.GetTaskByID(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ GetTaskInfoByID ============

func TestSchedulerService_GetTaskInfoByID(t *testing.T) {
	ctx := context.Background()

	t.Run("委托 GetTaskByID", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			taskRow18(7, "task-7"),
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		task, err := svc.GetTaskInfoByID(ctx, 7)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if task.ID != 7 {
			t.Errorf("期望 ID=7，实际=%d", task.ID)
		}
	})
}

// ============ CreateTask ============

func TestSchedulerService_CreateTask(t *testing.T) {
	ctx := context.Background()

	t.Run("创建成功", func(t *testing.T) {
		// CreateTask: WriteOneParameterized (INSERT) + GetTaskByID (query 18列) + getLastExecutionStatus (query 1列)
		qr := database.NewQueryResultWithRows([][]interface{}{
			taskRow18(1, "new-task"),
		})
		db := &MockDB{
			QueryResult: qr,
			WriteResult: database.NewWriteResult(1, 1),
		}
		svc := newSchedulerWithDB(db)

		task, err := svc.CreateTask(ctx, "INSERT INTO bdopsflow_tasks (name) VALUES (?)", "new-task")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if task.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", task.ID)
		}
	})

	t.Run("写入失败", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		_, err := svc.CreateTask(ctx, "INSERT INTO bdopsflow_tasks (name) VALUES (?)", "new-task")
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: errors.New("write result err")},
		}
		svc := newSchedulerWithDB(db)

		_, err := svc.CreateTask(ctx, "INSERT INTO bdopsflow_tasks (name) VALUES (?)", "new-task")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ ListTasks ============

func TestSchedulerService_ListTasks(t *testing.T) {
	ctx := context.Background()

	t.Run("系统管理员查询成功", func(t *testing.T) {
		// ListTasks -> ListTasksWithFilter
		// 内部调用 count 查询 (1列) + data 查询 (18列) + 每个任务 getLastExecutionStatus (1列)
		// MockDB 返回同一 QueryResult，使用 18 列数据
		qr := database.NewQueryResultWithRows([][]interface{}{
			taskRow18(1, "task-1"),
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		tasks, total, err := svc.ListTasks(ctx, 1, "system_admin", 1, 10)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// total 取自 count 查询的 row[0]，即 task ID=1
		if total != 1 {
			t.Errorf("期望 total=1，实际=%d", total)
		}
		if len(tasks) != 1 {
			t.Fatalf("期望 1 个任务，实际=%d", len(tasks))
		}
		if tasks[0].Name != "task-1" {
			t.Errorf("期望 Name=task-1，实际=%s", tasks[0].Name)
		}
	})

	t.Run("普通用户按领域过滤", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			taskRow18(2, "task-2"),
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		tasks, _, err := svc.ListTasks(ctx, 5, "user", 1, 10)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("期望 1 个任务，实际=%d", len(tasks))
		}
		// 验证首次查询参数包含 domainID=5
		if len(db.QueryStmts) == 0 {
			t.Fatal("期望至少 1 次查询调用")
		}
		if db.QueryStmts[0].Arguments[0].(int64) != 5 {
			t.Errorf("期望 domainID=5，实际=%v", db.QueryStmts[0].Arguments[0])
		}
	})

	t.Run("默认分页参数", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)

		_, _, err := svc.ListTasks(ctx, 1, "system_admin", 0, 0)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		_, _, err := svc.ListTasks(ctx, 1, "system_admin", 1, 10)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ ListTasksWithFilter ============

func TestSchedulerService_ListTasksWithFilter(t *testing.T) {
	ctx := context.Background()

	t.Run("带名称过滤", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			taskRow18(1, "match-task"),
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		filter := model.TaskListFilter{
			DomainID: 1,
			Role:     "system_admin",
			Page:     1,
			PageSize: 10,
			Name:     "match",
		}

		tasks, _, err := svc.ListTasksWithFilter(ctx, filter)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("期望 1 个任务，实际=%d", len(tasks))
		}
	})

	t.Run("带类型和启用状态过滤", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			taskRow18(1, "task-1"),
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		enabled := true
		filter := model.TaskListFilter{
			DomainID: 1,
			Role:     "system_admin",
			Page:     1,
			PageSize: 10,
			Type:     "http",
			IsEnabled: &enabled,
		}

		tasks, _, err := svc.ListTasksWithFilter(ctx, filter)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("期望 1 个任务，实际=%d", len(tasks))
		}
	})

	t.Run("带 createdBy 过滤", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			taskRow18(1, "task-1"),
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		filter := model.TaskListFilter{
			DomainID:  1,
			Role:      "system_admin",
			Page:      1,
			PageSize:  10,
			CreatedBy: 100,
		}

		tasks, _, err := svc.ListTasksWithFilter(ctx, filter)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("期望 1 个任务，实际=%d", len(tasks))
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)

		filter := model.TaskListFilter{
			DomainID: 1,
			Role:     "system_admin",
			Page:     1,
			PageSize: 10,
		}

		tasks, total, err := svc.ListTasksWithFilter(ctx, filter)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(tasks) != 0 {
			t.Errorf("期望 0 个任务，实际=%d", len(tasks))
		}
		if total != 0 {
			t.Errorf("期望 total=0，实际=%d", total)
		}
	})
}

// ============ UpdateTask ============

func TestSchedulerService_UpdateTask(t *testing.T) {
	ctx := context.Background()

	t.Run("更新成功", func(t *testing.T) {
		// UpdateTask: WriteOneParameterized + GetTaskByID (query) + getLastExecutionStatus (query)
		qr := database.NewQueryResultWithRows([][]interface{}{
			taskRow18(1, "updated-task"),
		})
		db := &MockDB{
			QueryResult: qr,
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := newSchedulerWithDB(db)

		task := &model.Task{
			Name:           "updated-task",
			Type:           "http",
			TimeoutSeconds: 300,
			RetryCount:     3,
			RetryInterval:  60,
		}

		err := svc.UpdateTask(ctx, 1, task)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("写入失败", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		task := &model.Task{Name: "task"}
		err := svc.UpdateTask(ctx, 1, task)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: errors.New("write result err")},
		}
		svc := newSchedulerWithDB(db)

		task := &model.Task{Name: "task"}
		err := svc.UpdateTask(ctx, 1, task)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ DeleteTask ============

func TestSchedulerService_DeleteTask(t *testing.T) {
	ctx := context.Background()

	t.Run("删除成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := newSchedulerWithDB(db)

		err := svc.DeleteTask(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.LastWriteStmt.Arguments[0].(int64) != 1 {
			t.Errorf("期望 id=1，实际=%v", db.LastWriteStmt.Arguments[0])
		}
	})

	t.Run("写入失败", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		err := svc.DeleteTask(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: errors.New("write result err")},
		}
		svc := newSchedulerWithDB(db)

		err := svc.DeleteTask(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ UpdateTaskStatusByID ============

func TestSchedulerService_UpdateTaskStatusByID(t *testing.T) {
	ctx := context.Background()

	t.Run("无 Cron 表达式，更新状态", func(t *testing.T) {
		// UpdateTaskStatusByID: GetTaskByID (query) + WriteOneParameterized
		// taskRow18 中 cron_expression="*/5 * * * *"，需要构造无 cron 的行
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "task-1", "http", "{}", "", int64(300), int64(0), int64(0), true, "enabled", int64(1), nil, "", int64(0), int64(1), "admin", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"},
		})
		db := &MockDB{
			QueryResult: qr,
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := newSchedulerWithDB(db)

		err := svc.UpdateTaskStatusByID(ctx, 1, "failed")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("有 Cron 表达式，仅更新时间", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			taskRow18(1, "task-1"),
		})
		db := &MockDB{
			QueryResult: qr,
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := newSchedulerWithDB(db)

		err := svc.UpdateTaskStatusByID(ctx, 1, "failed")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("写入失败", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "task-1", "http", "{}", "", int64(300), int64(0), int64(0), true, "enabled", int64(1), nil, "", int64(0), int64(1), "admin", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"},
		})
		db := &MockDB{
			QueryResult: qr,
			WriteError:  ErrMockDB,
		}
		svc := newSchedulerWithDB(db)

		err := svc.UpdateTaskStatusByID(ctx, 1, "failed")
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ ScanPendingTasks ============

func TestSchedulerService_ScanPendingTasks(t *testing.T) {
	ctx := context.Background()

	t.Run("查询成功", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			taskRow18(1, "task-1"),
			taskRow18(2, "task-2"),
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		tasks, err := svc.ScanPendingTasks(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(tasks) != 2 {
			t.Fatalf("期望 2 个任务，实际=%d", len(tasks))
		}
		if tasks[0].Name != "task-1" {
			t.Errorf("期望 Name=task-1，实际=%s", tasks[0].Name)
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)

		tasks, err := svc.ScanPendingTasks(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(tasks) != 0 {
			t.Errorf("期望 0 个任务，实际=%d", len(tasks))
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		_, err := svc.ScanPendingTasks(ctx)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := newSchedulerWithDB(db)

		_, err := svc.ScanPendingTasks(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ getRetryTimesForExecution ============

func TestSchedulerService_getRetryTimesForExecution(t *testing.T) {
	ctx := context.Background()

	t.Run("查询成功", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(2)},
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		retryTimes, err := svc.getRetryTimesForExecution(ctx, 1, "exec-001")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if retryTimes != 2 {
			t.Errorf("期望 retryTimes=2，实际=%d", retryTimes)
		}
	})

	t.Run("执行记录不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)

		_, err := svc.getRetryTimesForExecution(ctx, 1, "ghost")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		_, err := svc.getRetryTimesForExecution(ctx, 1, "exec-001")
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := newSchedulerWithDB(db)

		_, err := svc.getRetryTimesForExecution(ctx, 1, "exec-001")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ TriggerTask ============

func TestSchedulerService_TriggerTask(t *testing.T) {
	ctx := context.Background()

	t.Run("非 leader 节点不能触发", func(t *testing.T) {
		svc := &SchedulerService{}
		_, err := svc.TriggerTask(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("任务不存在", func(t *testing.T) {
		// leader + 无运行中任务（查询返回空）+ GetTaskByID 返回空
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()
		svc.SetLeader(true)

		_, err := svc.TriggerTask(ctx, 999)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("DB 查询错误（检查运行中任务）", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()
		svc.SetLeader(true)

		// 检查运行中任务时 DB 出错 → 走 GetTaskByID 也出错
		_, err := svc.TriggerTask(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ RetryTask ============

func TestSchedulerService_RetryTask(t *testing.T) {
	ctx := context.Background()

	t.Run("任务不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		_, err := svc.RetryTask(ctx, 999, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		_, err := svc.RetryTask(ctx, 1, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ HandleTaskFailure ============

func TestSchedulerService_HandleTaskFailure(t *testing.T) {
	ctx := context.Background()

	t.Run("任务不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		err := svc.HandleTaskFailure(ctx, 999, "exec-001", "", "error")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("无重试配置（RetryCount=0）", func(t *testing.T) {
		// taskRow18 中 retry_count=3，需要构造 retry_count=0 的行
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "task-1", "http", "{}", "", int64(300), int64(0), int64(0), true, "enabled", int64(1), nil, "", int64(0), int64(1), "admin", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"},
		})
		db := &MockDB{
			QueryResult: qr,
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		err := svc.HandleTaskFailure(ctx, 1, "exec-001", "", "error")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})
}

// ============ dispatchTaskExecution ============

func TestSchedulerService_dispatchTaskExecution(t *testing.T) {
	ctx := context.Background()

	t.Run("锁冲突（任务正在执行）", func(t *testing.T) {
		// 需要先在 redis 中设置锁
		svc, mr, rdb := newSchedulerWithDBAndRedis(t, &MockDB{})
		defer mr.Close()
		svc.SetLeader(true)

		task := &model.Task{ID: 1, Name: "task-1", TimeoutSeconds: 300}
		executionID := "exec-conflict-001"
		lockKey := fmt.Sprintf("task:lock:%s", executionID)
		rdb.Set(ctx, lockKey, "existing-lock", 60*1000000000) // 60s

		_, err := svc.dispatchTaskExecution(ctx, task, executionID, 0, false)
		if err == nil {
			t.Fatal("期望返回错误（锁冲突）")
		}
	})

	t.Run("无可用执行器", func(t *testing.T) {
		// GetTaskByID 不需要，但需要 INSERT execution + SelectAvailableExecutor
		// MockDB 查询返回空（无可用执行器）
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
			WriteResult: database.NewWriteResult(1, 1),
		}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()
		svc.SetLeader(true)

		task := &model.Task{ID: 1, Name: "task-1", TimeoutSeconds: 300, DomainID: 1}

		_, err := svc.dispatchTaskExecution(ctx, task, "exec-no-executor-001", 0, false)
		if err == nil {
			t.Fatal("期望返回错误（无可用执行器）")
		}
	})
}
