package service

import (
	"context"
	"errors"
	"testing"

	pb "github.com/lynnyq/bdopsflow/proto"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
)

// executorRow 构造一行 bdopsflow_executors 表的数据（9 列：id, name, address, status, last_heartbeat, capacity, current_load, created_at, updated_at）
func executorRow(id int64, name, address, status string) []interface{} {
	return []interface{}{id, name, address, status, nil, int64(10), int64(0), "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"}
}

// ============ SelectAvailableExecutor ============

func TestSchedulerService_SelectAvailableExecutor(t *testing.T) {
	ctx := context.Background()

	t.Run("找到可用执行器（无 domainID）", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			executorRow(1, "exec-1", "localhost:8080", "online"),
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		exec, err := svc.SelectAvailableExecutor(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if exec == nil {
			t.Fatal("期望非 nil 执行器")
		}
		if exec.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", exec.ID)
		}
		if exec.Name != "exec-1" {
			t.Errorf("期望 Name=exec-1，实际=%s", exec.Name)
		}
	})

	t.Run("找到可用执行器（带 domainID）", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			executorRow(2, "exec-2", "host:8080", "online"),
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		exec, err := svc.SelectAvailableExecutor(ctx, int64(5))
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if exec.ID != 2 {
			t.Errorf("期望 ID=2，实际=%d", exec.ID)
		}
		// 验证查询参数包含 domainID
		args := db.LastQueryStmt.Arguments
		if len(args) < 2 {
			t.Fatalf("期望至少 2 个参数，实际=%d", len(args))
		}
		if args[1].(int64) != 5 {
			t.Errorf("期望 domainID=5，实际=%v", args[1])
		}
	})

	t.Run("无可用执行器（空结果）", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)

		_, err := svc.SelectAvailableExecutor(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		_, err := svc.SelectAvailableExecutor(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := newSchedulerWithDB(db)

		_, err := svc.SelectAvailableExecutor(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ GetExecutorByID / GetExecutorInfoByID ============

func TestSchedulerService_GetExecutorByID(t *testing.T) {
	ctx := context.Background()

	t.Run("找到执行器", func(t *testing.T) {
		// GetExecutorByID 查询 10 列 (含 is_global)，但 scanExecutorResult 只读取 9 列
		// 因此 is_global 字段不会被扫描到 Executor 结构体
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "exec-1", "host:8080", "online", nil, int64(10), int64(2), true, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"},
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		exec, err := svc.GetExecutorByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if exec.ID != 1 || exec.Name != "exec-1" || exec.Status != "online" {
			t.Errorf("执行器字段不正确: %+v", exec)
		}
	})

	t.Run("执行器不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)

		_, err := svc.GetExecutorByID(ctx, 999)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		_, err := svc.GetExecutorByID(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := newSchedulerWithDB(db)

		_, err := svc.GetExecutorByID(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("GetExecutorInfoByID 委托 GetExecutorByID", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(7), "exec-7", "h:8080", "online", nil, int64(5), int64(0), false, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"},
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		exec, err := svc.GetExecutorInfoByID(ctx, 7)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if exec.ID != 7 {
			t.Errorf("期望 ID=7，实际=%d", exec.ID)
		}
	})
}

// ============ GetExecutorByName ============

func TestSchedulerService_GetExecutorByName(t *testing.T) {
	ctx := context.Background()

	t.Run("找到执行器", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "exec-1", "host:8080", "online", nil, int64(10), int64(2), false, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"},
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		exec, err := svc.GetExecutorByName(ctx, "exec-1")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if exec.Name != "exec-1" {
			t.Errorf("期望 Name=exec-1，实际=%s", exec.Name)
		}
		// 验证查询参数
		if db.LastQueryStmt.Arguments[0] != "exec-1" {
			t.Errorf("期望查询参数为 exec-1，实际=%v", db.LastQueryStmt.Arguments[0])
		}
	})

	t.Run("执行器不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)

		_, err := svc.GetExecutorByName(ctx, "ghost")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		_, err := svc.GetExecutorByName(ctx, "exec-1")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := newSchedulerWithDB(db)

		_, err := svc.GetExecutorByName(ctx, "exec-1")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ DeleteExecutor ============

func TestSchedulerService_DeleteExecutor(t *testing.T) {
	ctx := context.Background()

	t.Run("删除成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := newSchedulerWithDB(db)

		err := svc.DeleteExecutor(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.LastWriteStmt.Arguments[0].(int64) != 1 {
			t.Errorf("期望参数 id=1，实际=%v", db.LastWriteStmt.Arguments[0])
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		err := svc.DeleteExecutor(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ DeleteExecutorByName ============

func TestSchedulerService_DeleteExecutorByName(t *testing.T) {
	ctx := context.Background()

	t.Run("按名称删除成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := newSchedulerWithDB(db)

		err := svc.DeleteExecutorByName(ctx, "exec-1")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.LastWriteStmt.Arguments[0] != "exec-1" {
			t.Errorf("期望参数 name=exec-1，实际=%v", db.LastWriteStmt.Arguments[0])
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		err := svc.DeleteExecutorByName(ctx, "exec-1")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ SetExecutorStatus / SetExecutorStatusByName ============

func TestSchedulerService_SetExecutorStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("按 ID 设置状态成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := newSchedulerWithDB(db)

		err := svc.SetExecutorStatus(ctx, 1, "offline")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		args := db.LastWriteStmt.Arguments
		if args[0] != "offline" {
			t.Errorf("期望 status=offline，实际=%v", args[0])
		}
		if args[2].(int64) != 1 {
			t.Errorf("期望 id=1，实际=%v", args[2])
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		err := svc.SetExecutorStatus(ctx, 1, "offline")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestSchedulerService_SetExecutorStatusByName(t *testing.T) {
	ctx := context.Background()

	t.Run("按名称设置状态成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := newSchedulerWithDB(db)

		err := svc.SetExecutorStatusByName(ctx, "exec-1", "offline")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		args := db.LastWriteStmt.Arguments
		if args[0] != "offline" {
			t.Errorf("期望 status=offline，实际=%v", args[0])
		}
		if args[2] != "exec-1" {
			t.Errorf("期望 name=exec-1，实际=%v", args[2])
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		err := svc.SetExecutorStatusByName(ctx, "exec-1", "offline")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ UpdateExecutorCapacity / UpdateExecutorCapacityByName ============

func TestSchedulerService_UpdateExecutorCapacity(t *testing.T) {
	ctx := context.Background()

	t.Run("按 ID 更新容量成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		err := svc.UpdateExecutorCapacity(ctx, 1, 20)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		args := db.LastWriteStmt.Arguments
		if args[0].(int64) != 20 {
			t.Errorf("期望 capacity=20，实际=%v", args[0])
		}
		if args[2].(int64) != 1 {
			t.Errorf("期望 id=1，实际=%v", args[2])
		}
	})

	t.Run("容量为 0 时返回错误", func(t *testing.T) {
		svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
		defer mr.Close()

		err := svc.UpdateExecutorCapacity(ctx, 1, 0)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("负容量返回错误", func(t *testing.T) {
		svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
		defer mr.Close()

		err := svc.UpdateExecutorCapacity(ctx, 1, -5)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		err := svc.UpdateExecutorCapacity(ctx, 1, 20)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestSchedulerService_UpdateExecutorCapacityByName(t *testing.T) {
	ctx := context.Background()

	t.Run("按名称更新容量成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		err := svc.UpdateExecutorCapacityByName(ctx, "exec-1", 30)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		args := db.LastWriteStmt.Arguments
		if args[0].(int64) != 30 {
			t.Errorf("期望 capacity=30，实际=%v", args[0])
		}
		if args[2] != "exec-1" {
			t.Errorf("期望 name=exec-1，实际=%v", args[2])
		}
	})

	t.Run("容量为 0 时返回错误", func(t *testing.T) {
		svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
		defer mr.Close()

		err := svc.UpdateExecutorCapacityByName(ctx, "exec-1", 0)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		err := svc.UpdateExecutorCapacityByName(ctx, "exec-1", 30)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ GetExecutorTargetCapacity ============

func TestSchedulerService_GetExecutorTargetCapacity(t *testing.T) {
	ctx := context.Background()

	t.Run("Redis 中存在目标容量", func(t *testing.T) {
		db := &MockDB{}
		svc, mr, rdb := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()
		rdb.Set(ctx, "executor:target_capacity:exec-1", 42, 0)

		cap, err := svc.GetExecutorTargetCapacity(ctx, "exec-1")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if cap != 42 {
			t.Errorf("期望容量=42，实际=%d", cap)
		}
	})

	t.Run("Redis 中无值，回退到 DB 查询", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "exec-1", "host:8080", "online", nil, int64(15), int64(0), true, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"},
		})
		db := &MockDB{QueryResult: qr}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		cap, err := svc.GetExecutorTargetCapacity(ctx, "exec-1")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if cap != 15 {
			t.Errorf("期望容量=15，实际=%d", cap)
		}
	})

	t.Run("Redis 无值且 DB 查询失败", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		_, err := svc.GetExecutorTargetCapacity(ctx, "exec-1")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ UpdateExecutorHeartbeat / UpdateExecutorHeartbeatWithRunningTasks ============

func TestSchedulerService_UpdateExecutorHeartbeat(t *testing.T) {
	ctx := context.Background()

	t.Run("心跳更新成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := newSchedulerWithDB(db)

		err := svc.UpdateExecutorHeartbeat(ctx, "exec-1", 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		args := db.LastWriteStmt.Arguments
		if args[0].(int32) != 5 {
			t.Errorf("期望 currentLoad=5，实际=%v", args[0])
		}
		if args[3] != "exec-1" {
			t.Errorf("期望 name=exec-1，实际=%v", args[3])
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		err := svc.UpdateExecutorHeartbeat(ctx, "exec-1", 5)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

func TestSchedulerService_UpdateExecutorHeartbeatWithRunningTasks(t *testing.T) {
	ctx := context.Background()

	t.Run("带运行任务的心跳更新成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		err := svc.UpdateExecutorHeartbeatWithRunningTasks(ctx, "exec-1", 5, []string{"exec-1", "exec-2"})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("空运行任务列表", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		err := svc.UpdateExecutorHeartbeatWithRunningTasks(ctx, "exec-1", 5, nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		err := svc.UpdateExecutorHeartbeatWithRunningTasks(ctx, "exec-1", 5, nil)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ RegisterExecutor ============

func TestSchedulerService_RegisterExecutor(t *testing.T) {
	ctx := context.Background()

	t.Run("新建执行器（不存在同名）", func(t *testing.T) {
		// GetExecutorByName 返回空结果 → 走 INSERT 路径
		// RegisterExecutor 内部会调用 GetExecutorByName（查询1）+ WriteOneParameterized（写入1）+ updateExecutorMetrics（查询2）
		db := &MockDB{
			QueryResult:  database.NewQueryResultWithRows(nil),
			WriteResult:  database.NewWriteResult(1, 1),
		}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		name, err := svc.RegisterExecutor(ctx, "new-exec", "localhost:8080", 10)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if name != "new-exec" {
			t.Errorf("期望 name=new-exec，实际=%s", name)
		}
	})

	t.Run("DB 查询错误", func(t *testing.T) {
		// GetExecutorByName 返回错误时，由于 err != nil，会进入 INSERT 路径
		db := &MockDB{
			QueryError:   ErrMockDB,
			WriteResult:  database.NewWriteResult(1, 1),
		}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		_, err := svc.RegisterExecutor(ctx, "new-exec", "localhost:8080", 10)
		if err != nil {
			t.Fatalf("GetExecutorByName 失败时应走 INSERT 路径，期望无错误，实际: %v", err)
		}
	})

	t.Run("写入失败", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
			WriteError:  ErrMockDB,
		}
		svc, mr, _ := newSchedulerWithDBAndRedis(t, db)
		defer mr.Close()

		_, err := svc.RegisterExecutor(ctx, "new-exec", "localhost:8080", 10)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ ListExecutors ============

func TestSchedulerService_ListExecutors(t *testing.T) {
	ctx := context.Background()

	t.Run("列表查询成功", func(t *testing.T) {
		// ListExecutors 内部调用 2 次查询：count (1 列) 和 data (9 列)
		// MockDB 对所有查询返回同一个 QueryResult，因此使用 9 列数据：
		// - count 查询读取 row[0] 作为 total
		// - data 查询通过 scanExecutorResult 读取全部 9 列
		qr := database.NewQueryResultWithRows([][]interface{}{
			executorRow(1, "exec-1", "host:8080", "online"),
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		executors, total, err := svc.ListExecutors(ctx, 1, 10)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// total 取自 count 查询的 row[0]，即 executorRow 的 ID=1
		if total != 1 {
			t.Errorf("期望 total=1（取自首列），实际=%d", total)
		}
		if len(executors) != 1 {
			t.Fatalf("期望 1 个执行器，实际=%d", len(executors))
		}
		if executors[0].Name != "exec-1" {
			t.Errorf("期望 Name=exec-1，实际=%s", executors[0].Name)
		}
	})

	t.Run("默认分页参数", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)

		_, _, err := svc.ListExecutors(ctx, 0, 0)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 验证分页参数被规范化（page=1, pageSize=20）
		// 由于 QueryStmts[1] 是数据查询语句
		if len(db.QueryStmts) < 2 {
			t.Fatalf("期望至少 2 次查询调用，实际=%d", len(db.QueryStmts))
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		_, _, err := svc.ListExecutors(ctx, 1, 10)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ ListExecutorsByDomain ============

func TestSchedulerService_ListExecutorsByDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("按领域查询成功", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			executorRow(1, "exec-1", "h:8080", "online"),
			executorRow(2, "exec-2", "h:8081", "online"),
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		executors, err := svc.ListExecutorsByDomain(ctx, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(executors) != 2 {
			t.Errorf("期望 2 个执行器，实际=%d", len(executors))
		}
		if db.LastQueryStmt.Arguments[0].(int64) != 5 {
			t.Errorf("期望 domainID=5，实际=%v", db.LastQueryStmt.Arguments[0])
		}
	})

	t.Run("空结果返回空切片", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)

		executors, err := svc.ListExecutorsByDomain(ctx, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(executors) != 0 {
			t.Errorf("期望 0 个执行器，实际=%d", len(executors))
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		_, err := svc.ListExecutorsByDomain(ctx, 5)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := newSchedulerWithDB(db)

		_, err := svc.ListExecutorsByDomain(ctx, 5)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ NewSchedulerService ============

func TestNewSchedulerService(t *testing.T) {
	t.Run("构造函数返回非 nil", func(t *testing.T) {
		db := &MockDB{}
		svc := NewSchedulerService(db, nil)
		if svc == nil {
			t.Fatal("期望非 nil 服务")
		}
		if svc.DB != db {
			t.Error("期望 DB 正确赋值")
		}
		if svc.redis != nil {
			t.Error("期望 redis 为 nil")
		}
		if svc.httpClient == nil {
			t.Error("期望 httpClient 默认非 nil")
		}
		if svc.stopCleanupCh == nil {
			t.Error("期望 stopCleanupCh 非 nil")
		}
	})
}

// ============ SetLeader / IsLeader ============

func TestSchedulerService_SetLeader_IsLeader(t *testing.T) {
	svc := &SchedulerService{}

	if svc.IsLeader() {
		t.Error("初始状态应为非 leader")
	}

	svc.SetLeader(true)
	if !svc.IsLeader() {
		t.Error("SetLeader(true) 后应为 leader")
	}

	svc.SetLeader(false)
	if svc.IsLeader() {
		t.Error("SetLeader(false) 后应为非 leader")
	}
}

// ============ Setter 方法 ============

func TestSchedulerService_Setters(t *testing.T) {
	svc := &SchedulerService{}

	t.Run("SetCronScheduler", func(t *testing.T) {
		cs := &mockCronScheduler{}
		svc.SetCronScheduler(cs)
		if svc.cronScheduler == nil {
			t.Error("期望 cronScheduler 非 nil")
		}
	})

	t.Run("SetTaskDispatcher", func(t *testing.T) {
		dispatcher := func(executorName string, task *pb.Task) error { return nil }
		svc.SetTaskDispatcher(dispatcher)
		if svc.dispatcher == nil {
			t.Error("期望 dispatcher 非 nil")
		}
	})

	t.Run("SetConnectivityChecker", func(t *testing.T) {
		checker := &mockConnectivityChecker{connected: map[string]bool{"exec-1": true}}
		svc.SetConnectivityChecker(checker)
		if svc.connectivityChecker == nil {
			t.Error("期望 connectivityChecker 非 nil")
		}
	})

	t.Run("SetLeaderAddrResolver", func(t *testing.T) {
		resolver := &mockLeaderAddrResolver{leaderAddr: "localhost:8080"}
		svc.SetLeaderAddrResolver(resolver)
		if svc.leaderAddrResolver == nil {
			t.Error("期望 leaderAddrResolver 非 nil")
		}
	})

	t.Run("SetCancelNotifier", func(t *testing.T) {
		notifier := &mockCancelNotifier{}
		svc.SetCancelNotifier(notifier)
		if svc.cancelNotifier == nil {
			t.Error("期望 cancelNotifier 非 nil")
		}
	})
}

// mockCancelNotifier 用于测试
type mockCancelNotifier struct{}

func (m *mockCancelNotifier) AddCancelExecutionId(executorName, executionId string) {}
