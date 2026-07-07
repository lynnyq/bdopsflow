package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
)

func TestExecutorDomainAssignment(t *testing.T) {
	t.Run("创建执行器领域分配模型", func(t *testing.T) {
		de := &model.DomainExecutor{
			ID:         1,
			DomainID:   1,
			ExecutorID: 1,
			AssignedBy: ptrInt64(1),
		}

		if de.DomainID != 1 {
			t.Errorf("期望 DomainID 为 1，实际为 %d", de.DomainID)
		}
		if de.ExecutorID != 1 {
			t.Errorf("期望 ExecutorID 为 1，实际为 %d", de.ExecutorID)
		}
		if de.AssignedBy == nil || *de.AssignedBy != 1 {
			t.Error("期望 AssignedBy 为 1")
		}
	})

	t.Run("创建带有领域信息的执行器", func(t *testing.T) {
		domains := []*model.Domain{
			{ID: 1, Name: "default"},
			{ID: 2, Name: "production"},
		}

		exec := &model.ExecutorWithDomains{
			Executor: model.Executor{
				ID:          1,
				Name:        "测试执行器",
				Address:     "localhost:8080",
				Status:      "online",
				Capacity:    10,
				CurrentLoad: 0,
				IsGlobal:    false,
			},
			Domains:  domains,
			IsGlobal: false,
		}

		if len(exec.Domains) != 2 {
			t.Errorf("期望 2 个领域，实际为 %d", len(exec.Domains))
		}
		if exec.Domains[0].Name != "default" {
			t.Errorf("期望第一个领域名称为 'default'，实际为 '%s'", exec.Domains[0].Name)
		}
		if exec.Domains[1].Name != "production" {
			t.Errorf("期望第二个领域名称为 'production'，实际为 '%s'", exec.Domains[1].Name)
		}
	})

	t.Run("执行器是否为全局", func(t *testing.T) {
		globalExec := &model.Executor{
			ID:       1,
			IsGlobal: true,
		}

		if !globalExec.IsGlobal {
			t.Error("期望全局执行器 IsGlobal 为 true")
		}

		domainExec := &model.Executor{
			ID:       2,
			IsGlobal: false,
		}

		if domainExec.IsGlobal {
			t.Error("期望领域执行器 IsGlobal 为 false")
		}
	})
}

func TestDomainExecutorAssignment(t *testing.T) {
	t.Run("分配执行器到多个领域请求", func(t *testing.T) {
		req := &model.ExecutorDomainRequest{
			DomainIDs: []int64{1, 2, 3},
		}

		if len(req.DomainIDs) != 3 {
			t.Errorf("期望 3 个领域ID，实际为 %d", len(req.DomainIDs))
		}

		if req.DomainIDs[0] != 1 || req.DomainIDs[1] != 2 || req.DomainIDs[2] != 3 {
			t.Error("领域ID顺序不正确")
		}
	})

	t.Run("分配执行器到单个领域请求", func(t *testing.T) {
		req := &model.ExecutorDomainRequest{
			DomainIDs: []int64{1},
		}

		if len(req.DomainIDs) != 1 {
			t.Errorf("期望 1 个领域ID，实际为 %d", len(req.DomainIDs))
		}
	})

	t.Run("分配执行器到空领域请求（全局执行器）", func(t *testing.T) {
		req := &model.ExecutorDomainRequest{
			DomainIDs: []int64{},
		}

		if len(req.DomainIDs) != 0 {
			t.Errorf("期望 0 个领域ID，实际为 %d", len(req.DomainIDs))
		}
	})
}

func TestDomainWithStats(t *testing.T) {
	t.Run("创建带有统计信息的领域", func(t *testing.T) {
		domainStats := &model.DomainWithStats{
			Domain: model.Domain{
				ID:          1,
				Name:        "production",
				Description: "生产环境",
			},
			UserCount:     10,
			ExecutorCount: 5,
			TaskCount:     100,
		}

		if domainStats.UserCount != 10 {
			t.Errorf("期望 UserCount 为 10，实际为 %d", domainStats.UserCount)
		}
		if domainStats.ExecutorCount != 5 {
			t.Errorf("期望 ExecutorCount 为 5，实际为 %d", domainStats.ExecutorCount)
		}
		if domainStats.TaskCount != 100 {
			t.Errorf("期望 TaskCount 为 100，实际为 %d", domainStats.TaskCount)
		}
	})
}

func TestExecutorDeletePermission(t *testing.T) {
	t.Run("系统管理员可以删除任何执行器", func(t *testing.T) {
		userRole := "system_admin"
		isAdmin := userRole == "system_admin" || userRole == "admin"

		if !isAdmin {
			t.Error("系统管理员应该可以删除任何执行器")
		}
	})

	t.Run("普通管理员可以删除任何执行器", func(t *testing.T) {
		userRole := "admin"
		isAdmin := userRole == "system_admin" || userRole == "admin"

		if !isAdmin {
			t.Error("管理员应该可以删除任何执行器")
		}
	})

	t.Run("领域管理员只能删除只分配给其领域的执行器", func(t *testing.T) {
		userRole := "domain_admin"
		isAdmin := userRole == "system_admin" || userRole == "admin"

		if isAdmin {
			t.Error("领域管理员不应该是管理员")
		}

		executorDomainCount := 1
		canDelete := executorDomainCount <= 1

		if !canDelete {
			t.Error("领域管理员应该能够删除只分配给其领域的执行器")
		}
	})

	t.Run("领域管理员不能删除分配给多个领域的执行器", func(t *testing.T) {
		executorDomainCount := 2
		canDelete := executorDomainCount <= 1

		if canDelete {
			t.Error("领域管理员不应该能够删除分配给多个领域的执行器")
		}
	})
}

func TestTaskExecutorBinding(t *testing.T) {
	t.Run("检查执行器是否被任务绑定", func(t *testing.T) {
		task := &model.Task{
			ID:                 1,
			Name:               "测试任务",
			AssignedExecutorID: 1,
		}

		if task.AssignedExecutorID != 1 {
			t.Errorf("期望 AssignedExecutorID 为 1，实际为 %d", task.AssignedExecutorID)
		}

		isBound := task.AssignedExecutorID > 0
		if !isBound {
			t.Error("任务应该绑定到执行器")
		}
	})

	t.Run("未绑定执行器的任务", func(t *testing.T) {
		task := &model.Task{
			ID:                 2,
			Name:               "未绑定任务",
			AssignedExecutorID: 0,
		}

		isBound := task.AssignedExecutorID > 0
		if isBound {
			t.Error("任务不应该绑定到执行器")
		}
	})
}

func TestExecutorDomainFiltering(t *testing.T) {
	t.Run("系统管理员可以看到所有执行器", func(t *testing.T) {
		userRole := "system_admin"
		canSeeAll := userRole == "system_admin" || userRole == "admin"

		if !canSeeAll {
			t.Error("系统管理员应该可以看到所有执行器")
		}
	})

	t.Run("领域管理员只能看到分配给其领域的执行器", func(t *testing.T) {
		userRole := "domain_admin"
		canSeeAll := userRole == "system_admin" || userRole == "admin"

		if canSeeAll {
			t.Error("领域管理员不应该可以看到所有执行器")
		}

		userDomainID := int64(1)
		executors := []*model.ExecutorWithDomains{
			{Executor: model.Executor{ID: 1}, Domains: []*model.Domain{{ID: 1}}},
			{Executor: model.Executor{ID: 2}, Domains: []*model.Domain{{ID: 2}}},
		}

		var accessibleExecutors []*model.ExecutorWithDomains
		for _, exec := range executors {
			for _, domain := range exec.Domains {
				if domain.ID == userDomainID {
					accessibleExecutors = append(accessibleExecutors, exec)
					break
				}
			}
		}

		if len(accessibleExecutors) != 1 {
			t.Errorf("领域管理员应该只能看到 1 个执行器，实际为 %d", len(accessibleExecutors))
		}
	})

	t.Run("普通用户只能看到分配给其领域的执行器", func(t *testing.T) {
		userRole := "user"
		canSeeAll := userRole == "system_admin" || userRole == "admin"

		if canSeeAll {
			t.Error("普通用户不应该可以看到所有执行器")
		}
	})
}

func TestExecutorDomainMultiAssignment(t *testing.T) {
	t.Run("执行器可以被分配给多个领域", func(t *testing.T) {
		exec := &model.ExecutorWithDomains{
			Executor: model.Executor{
				ID:   1,
				Name: "多领域执行器",
			},
			Domains: []*model.Domain{
				{ID: 1, Name: "default"},
				{ID: 2, Name: "staging"},
				{ID: 3, Name: "production"},
			},
		}

		if len(exec.Domains) != 3 {
			t.Errorf("期望执行器分配到 3 个领域，实际为 %d", len(exec.Domains))
		}
	})

	t.Run("执行器所属领域ID列表", func(t *testing.T) {
		exec := &model.ExecutorWithDomains{
			Executor: model.Executor{
				ID: 1,
			},
			Domains: []*model.Domain{
				{ID: 1, Name: "default"},
				{ID: 2, Name: "production"},
			},
		}

		domainIDs := make([]int64, len(exec.Domains))
		for i, d := range exec.Domains {
			domainIDs[i] = d.ID
		}

		if len(domainIDs) != 2 {
			t.Errorf("期望 2 个领域ID，实际为 %d", len(domainIDs))
		}
		if domainIDs[0] != 1 || domainIDs[1] != 2 {
			t.Error("领域ID列表不正确")
		}
	})
}

func ptrInt64(v int64) *int64 {
	return &v
}

// ============ ExecutorDomainService 方法测试 ============

// domainRow 构造一行 bdopsflow_domains 表的数据（3 列：id, name, description）
func domainRow(id int64, name, desc string) []interface{} {
	return []interface{}{id, name, desc}
}

// executorWithGlobalRow 构造一行带 is_global 的执行器数据（10 列）
func executorWithGlobalRow(id int64, name, address, status string, isGlobal bool) []interface{} {
	return []interface{}{id, name, address, status, nil, int64(10), int64(0), isGlobal, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"}
}

// ============ NewExecutorDomainService ============

func TestNewExecutorDomainService(t *testing.T) {
	db := &MockDB{}
	svc := NewExecutorDomainService(db)
	if svc == nil {
		t.Fatal("期望非 nil 服务")
	}
	if svc.db == nil {
		t.Error("期望 db 非 nil")
	}
}

// ============ GetExecutorDomains ============

func TestExecutorDomainService_GetExecutorDomains(t *testing.T) {
	ctx := context.Background()

	t.Run("查询成功", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			domainRow(1, "default", "默认领域"),
			domainRow(2, "prod", "生产环境"),
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		domains, err := svc.GetExecutorDomains(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(domains) != 2 {
			t.Fatalf("期望 2 个领域，实际=%d", len(domains))
		}
		if domains[0].Name != "default" {
			t.Errorf("期望 Name=default，实际=%s", domains[0].Name)
		}
		if db.LastQueryStmt.Arguments[0].(int64) != 1 {
			t.Errorf("期望 executorID=1，实际=%v", db.LastQueryStmt.Arguments[0])
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewExecutorDomainService(db)

		domains, err := svc.GetExecutorDomains(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(domains) != 0 {
			t.Errorf("期望 0 个领域，实际=%d", len(domains))
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetExecutorDomains(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetExecutorDomains(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ AssignExecutorToDomains ============

func TestExecutorDomainService_AssignExecutorToDomains(t *testing.T) {
	ctx := context.Background()

	t.Run("分配到多个领域", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewExecutorDomainService(db)

		err := svc.AssignExecutorToDomains(ctx, 1, []int64{1, 2, 3}, 100)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 验证有写入调用（DELETE + batch INSERT + UPDATE is_global）
		if len(db.WriteStmts) < 2 {
			t.Errorf("期望至少 2 次写入调用，实际=%d", len(db.WriteStmts))
		}
	})

	t.Run("分配到空领域（标记为全局）", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewExecutorDomainService(db)

		err := svc.AssignExecutorToDomains(ctx, 1, []int64{}, 100)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 验证最后的 UPDATE 语句将 is_global 设为 true
		lastStmt := db.LastWriteStmt
		if lastStmt.Arguments[0] != true {
			t.Errorf("期望 isGlobal=true，实际=%v", lastStmt.Arguments[0])
		}
	})

	t.Run("DELETE 失败", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		err := svc.AssignExecutorToDomains(ctx, 1, []int64{1}, 100)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("批量 INSERT 失败", func(t *testing.T) {
		db := &MockDB{
			WriteResult:     database.NewWriteResult(0, 1),
			BatchWriteError: ErrMockDB,
		}
		svc := NewExecutorDomainService(db)

		err := svc.AssignExecutorToDomains(ctx, 1, []int64{1, 2}, 100)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ AssignExecutorToDefaultDomain ============

func TestExecutorDomainService_AssignExecutorToDefaultDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("成功分配到默认领域", func(t *testing.T) {
		// GetExecutorByName 查询返回执行器 + INSERT 写入
		qr := database.NewQueryResultWithRows([][]interface{}{
			executorWithGlobalRow(1, "exec-1", "host:8080", "online", false),
		})
		db := &MockDB{QueryResult: qr, WriteResult: database.NewWriteResult(1, 1)}
		svc := NewExecutorDomainService(db)

		err := svc.AssignExecutorToDefaultDomain(ctx, "exec-1", 100)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("执行器不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewExecutorDomainService(db)

		err := svc.AssignExecutorToDefaultDomain(ctx, "ghost", 100)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("写入失败", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			executorWithGlobalRow(1, "exec-1", "host:8080", "online", false),
		})
		db := &MockDB{QueryResult: qr, WriteError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		err := svc.AssignExecutorToDefaultDomain(ctx, "exec-1", 100)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ RemoveExecutorFromDomain ============

func TestExecutorDomainService_RemoveExecutorFromDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("移除成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewExecutorDomainService(db)

		err := svc.RemoveExecutorFromDomain(ctx, 1, 2)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		args := db.LastWriteStmt.Arguments
		if args[0].(int64) != 1 {
			t.Errorf("期望 executorID=1，实际=%v", args[0])
		}
		if args[1].(int64) != 2 {
			t.Errorf("期望 domainID=2，实际=%v", args[1])
		}
	})

	t.Run("写入失败", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		err := svc.RemoveExecutorFromDomain(ctx, 1, 2)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ IsExecutorInDomain ============

func TestExecutorDomainService_IsExecutorInDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("执行器在领域中（count > 0）", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		inDomain, err := svc.IsExecutorInDomain(ctx, 1, 2)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !inDomain {
			t.Error("期望 inDomain=true")
		}
	})

	t.Run("执行器不在领域中（count = 0）", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(0)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		inDomain, err := svc.IsExecutorInDomain(ctx, 1, 2)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if inDomain {
			t.Error("期望 inDomain=false")
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		_, err := svc.IsExecutorInDomain(ctx, 1, 2)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := NewExecutorDomainService(db)

		_, err := svc.IsExecutorInDomain(ctx, 1, 2)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ GetDomainExecutors ============

func TestExecutorDomainService_GetDomainExecutors(t *testing.T) {
	ctx := context.Background()

	t.Run("查询成功", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			executorRow(1, "exec-1", "h:8080", "online"),
			executorRow(2, "exec-2", "h:8081", "online"),
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		executors, err := svc.GetDomainExecutors(ctx, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(executors) != 2 {
			t.Fatalf("期望 2 个执行器，实际=%d", len(executors))
		}
		if db.LastQueryStmt.Arguments[0].(int64) != 5 {
			t.Errorf("期望 domainID=5，实际=%v", db.LastQueryStmt.Arguments[0])
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewExecutorDomainService(db)

		executors, err := svc.GetDomainExecutors(ctx, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(executors) != 0 {
			t.Errorf("期望 0 个执行器，实际=%d", len(executors))
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetDomainExecutors(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ GetExecutorsWithDomains ============

func TestExecutorDomainService_GetExecutorsWithDomains(t *testing.T) {
	ctx := context.Background()

	t.Run("查询成功", func(t *testing.T) {
		// GetExecutorsWithDomains 使用 QueryOne（非参数化），返回 10 列
		// 内部还会为每个执行器调用 GetExecutorDomains（参数化查询）
		// MockDB 对两种查询返回同一个 QueryResult
		// 但 GetExecutorsWithDomains 期望 10 列，GetExecutorDomains 期望 3 列
		// 由于两者共用同一 QueryResult，使用 10 列数据：
		// - GetExecutorsWithDomains 读取 row[0..9]
		// - GetExecutorDomains 也读取 row[0..2] 作为 domain id/name/desc
		qr := database.NewQueryResultWithRows([][]interface{}{
			executorWithGlobalRow(1, "exec-1", "h:8080", "online", false),
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		executors, err := svc.GetExecutorsWithDomains(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(executors) != 1 {
			t.Fatalf("期望 1 个执行器，实际=%d", len(executors))
		}
		if executors[0].Name != "exec-1" {
			t.Errorf("期望 Name=exec-1，实际=%s", executors[0].Name)
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewExecutorDomainService(db)

		executors, err := svc.GetExecutorsWithDomains(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(executors) != 0 {
			t.Errorf("期望 0 个执行器，实际=%d", len(executors))
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetExecutorsWithDomains(ctx)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ GetExecutorDomainCount ============

func TestExecutorDomainService_GetExecutorDomainCount(t *testing.T) {
	ctx := context.Background()

	t.Run("查询成功", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(3)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		count, err := svc.GetExecutorDomainCount(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if count != 3 {
			t.Errorf("期望 count=3，实际=%d", count)
		}
	})

	t.Run("无行返回 0", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewExecutorDomainService(db)

		count, err := svc.GetExecutorDomainCount(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if count != 0 {
			t.Errorf("期望 count=0，实际=%d", count)
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetExecutorDomainCount(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetExecutorDomainCount(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// ============ GetAssignedTasksForExecutor ============

func TestExecutorDomainService_GetAssignedTasksForExecutor(t *testing.T) {
	ctx := context.Background()

	t.Run("查询成功", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(5)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		count, err := svc.GetAssignedTasksForExecutor(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if count != 5 {
			t.Errorf("期望 count=5，实际=%d", count)
		}
	})

	t.Run("无绑定任务", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewExecutorDomainService(db)

		count, err := svc.GetAssignedTasksForExecutor(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if count != 0 {
			t.Errorf("期望 count=0，实际=%d", count)
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetAssignedTasksForExecutor(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ GetAssignedTaskNamesForExecutor ============

func TestExecutorDomainService_GetAssignedTaskNamesForExecutor(t *testing.T) {
	ctx := context.Background()

	t.Run("查询成功", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{"task-1"},
			{"task-2"},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		names, err := svc.GetAssignedTaskNamesForExecutor(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(names) != 2 {
			t.Fatalf("期望 2 个任务名，实际=%d", len(names))
		}
		if names[0] != "task-1" {
			t.Errorf("期望 names[0]=task-1，实际=%s", names[0])
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewExecutorDomainService(db)

		names, err := svc.GetAssignedTaskNamesForExecutor(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(names) != 0 {
			t.Errorf("期望 0 个任务名，实际=%d", len(names))
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetAssignedTaskNamesForExecutor(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ GetExecutorsByUserRole ============

func TestExecutorDomainService_GetExecutorsByUserRole(t *testing.T) {
	ctx := context.Background()

	t.Run("管理员查询", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			executorWithGlobalRow(1, "exec-1", "h:8080", "online", true),
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		executors, err := svc.GetExecutorsByUserRole(ctx, true, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(executors) != 1 {
			t.Fatalf("期望 1 个执行器，实际=%d", len(executors))
		}
		if !executors[0].IsGlobal {
			t.Error("期望 IsGlobal=true")
		}
	})

	t.Run("非管理员查询", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			executorWithGlobalRow(2, "exec-2", "h:8081", "online", false),
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		executors, err := svc.GetExecutorsByUserRole(ctx, false, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(executors) != 1 {
			t.Fatalf("期望 1 个执行器，实际=%d", len(executors))
		}
		// 验证首次查询参数包含 domainID（后续 GetExecutorDomains 调用会覆盖 LastQueryStmt）
		if len(db.QueryStmts) == 0 {
			t.Fatal("期望至少 1 次查询调用")
		}
		if db.QueryStmts[0].Arguments[0].(int64) != 5 {
			t.Errorf("期望 domainID=5，实际=%v", db.QueryStmts[0].Arguments[0])
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewExecutorDomainService(db)

		executors, err := svc.GetExecutorsByUserRole(ctx, true, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(executors) != 0 {
			t.Errorf("期望 0 个执行器，实际=%d", len(executors))
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetExecutorsByUserRole(ctx, true, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ CanDomainAdminDeleteExecutor ============

func TestExecutorDomainService_CanDomainAdminDeleteExecutor(t *testing.T) {
	ctx := context.Background()

	t.Run("只分配给一个领域，可删除", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		canDelete, err := svc.CanDomainAdminDeleteExecutor(ctx, 1, 2)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !canDelete {
			t.Error("期望 canDelete=true")
		}
	})

	t.Run("分配给多个领域，不可删除", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(3)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		canDelete, err := svc.CanDomainAdminDeleteExecutor(ctx, 1, 2)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if canDelete {
			t.Error("期望 canDelete=false")
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		_, err := svc.CanDomainAdminDeleteExecutor(ctx, 1, 2)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ GetExecutorByDBID ============

func TestExecutorDomainService_GetExecutorByDBID(t *testing.T) {
	ctx := context.Background()

	t.Run("找到执行器", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			executorWithGlobalRow(1, "exec-1", "h:8080", "online", false),
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		exec, err := svc.GetExecutorByDBID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if exec.ID != 1 || exec.Name != "exec-1" {
			t.Errorf("执行器字段不正确: %+v", exec)
		}
	})

	t.Run("执行器不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetExecutorByDBID(ctx, 999)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetExecutorByDBID(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ============ GetExecutorByName (ExecutorDomainService) ============

func TestExecutorDomainService_GetExecutorByName(t *testing.T) {
	ctx := context.Background()

	t.Run("找到执行器", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			executorWithGlobalRow(1, "exec-1", "h:8080", "online", false),
		})
		db := &MockDB{QueryResult: qr}
		svc := NewExecutorDomainService(db)

		exec, err := svc.GetExecutorByName(ctx, "exec-1")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if exec.Name != "exec-1" {
			t.Errorf("期望 Name=exec-1，实际=%s", exec.Name)
		}
		if db.LastQueryStmt.Arguments[0] != "exec-1" {
			t.Errorf("期望查询参数为 exec-1，实际=%v", db.LastQueryStmt.Arguments[0])
		}
	})

	t.Run("执行器不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetExecutorByName(ctx, "ghost")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("DB 错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewExecutorDomainService(db)

		_, err := svc.GetExecutorByName(ctx, "exec-1")
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}
