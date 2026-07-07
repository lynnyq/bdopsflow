package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// newDomainAdminServiceWithSeparateDBs 构造一个 DomainAdminService，其中 permSvc 和 DomainAdminService 使用不同的 MockDB。
// 这样可以分别控制 permSvc.IsSystemAdmin 等委托方法的返回值和直接 db 调用的返回值。
func newDomainAdminServiceWithSeparateDBs(permDB, domainDB *MockDB) *DomainAdminService {
	permSvc := NewPermissionService(permDB, nil)
	return NewDomainAdminService(domainDB, permSvc)
}

// TestNewDomainAdminService 测试构造函数
func TestNewDomainAdminService(t *testing.T) {
	t.Run("所有参数为 nil 时仍可创建实例", func(t *testing.T) {
		svc := NewDomainAdminService(nil, nil)
		if svc == nil {
			t.Fatal("期望返回非 nil 实例")
		}
	})

	t.Run("参数正确赋值", func(t *testing.T) {
		db := &MockDB{}
		permSvc := NewPermissionService(db, nil)
		svc := NewDomainAdminService(db, permSvc)
		if svc.db == nil || svc.permSvc == nil {
			t.Error("期望所有字段正确赋值")
		}
	})
}

// TestDomainAdminService_IsSystemAdmin 测试系统管理员检查（委托给 permSvc）
func TestDomainAdminService_IsSystemAdmin(t *testing.T) {
	ctx := context.Background()

	t.Run("是系统管理员", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1)},
		})
		db := &MockDB{QueryResult: qr}
		permSvc := NewPermissionService(db, nil)
		svc := NewDomainAdminService(db, permSvc)

		result, err := svc.IsSystemAdmin(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !result {
			t.Error("期望 true")
		}
	})

	t.Run("非系统管理员", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(0)},
		})
		db := &MockDB{QueryResult: qr}
		permSvc := NewPermissionService(db, nil)
		svc := NewDomainAdminService(db, permSvc)

		result, err := svc.IsSystemAdmin(ctx, 2)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if result {
			t.Error("期望 false")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		permSvc := NewPermissionService(db, nil)
		svc := NewDomainAdminService(db, permSvc)

		_, err := svc.IsSystemAdmin(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestDomainAdminService_ListDomainsByUser 测试列出用户可访问的域
func TestDomainAdminService_ListDomainsByUser(t *testing.T) {
	ctx := context.Background()

	t.Run("返回多个域", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "domain1", "描述1", "2024-01-01T00:00:00Z", int64(10), int64(2), int64(5)},
			{int64(2), "domain2", "描述2", "2024-01-02T00:00:00Z", int64(20), int64(3), int64(8)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		domains, err := svc.ListDomainsByUser(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(domains) != 2 {
			t.Fatalf("期望 2 个域，实际 %d", len(domains))
		}
		if domains[0].ID != 1 || domains[0].Name != "domain1" {
			t.Errorf("期望 ID=1 Name=domain1，实际 ID=%d Name=%s", domains[0].ID, domains[0].Name)
		}
		if domains[0].UserCount != 10 {
			t.Errorf("期望 UserCount=10，实际=%d", domains[0].UserCount)
		}
		if domains[0].ExecutorCount != 2 {
			t.Errorf("期望 ExecutorCount=2，实际=%d", domains[0].ExecutorCount)
		}
		if domains[0].TaskCount != 5 {
			t.Errorf("期望 TaskCount=5，实际=%d", domains[0].TaskCount)
		}
		if domains[1].ID != 2 || domains[1].Name != "domain2" {
			t.Errorf("期望 ID=2 Name=domain2，实际 ID=%d Name=%s", domains[1].ID, domains[1].Name)
		}
	})

	t.Run("空结果", func(t *testing.T) {
		qr := database.NewQueryResultWithRows(nil)
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		domains, err := svc.ListDomainsByUser(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(domains) != 0 {
			t.Errorf("期望 0 个域，实际 %d", len(domains))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewDomainAdminService(db, nil)

		_, err := svc.ListDomainsByUser(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}
		svc := NewDomainAdminService(db, nil)

		_, err := svc.ListDomainsByUser(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("断言查询参数", func(t *testing.T) {
		qr := database.NewQueryResultWithRows(nil)
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		_, _ = svc.ListDomainsByUser(ctx, 42)
		if db.LastQueryStmt.Arguments[0] != int64(42) {
			t.Errorf("期望参数 42，实际=%v", db.LastQueryStmt.Arguments[0])
		}
	})
}

// TestDomainAdminService_ListDomains 测试列出所有域
func TestDomainAdminService_ListDomains(t *testing.T) {
	ctx := context.Background()

	t.Run("返回多个域", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "domain1", "描述1", "2024-01-01T00:00:00Z", int64(10), int64(2), int64(5)},
			{int64(2), "domain2", "", "2024-01-02T00:00:00Z", int64(0), int64(0), int64(0)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		domains, err := svc.ListDomains(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(domains) != 2 {
			t.Fatalf("期望 2 个域，实际 %d", len(domains))
		}
		if domains[0].ID != 1 || domains[0].Name != "domain1" {
			t.Errorf("期望 ID=1 Name=domain1，实际 ID=%d Name=%s", domains[0].ID, domains[0].Name)
		}
		if domains[1].Description != "" {
			t.Errorf("期望空描述，实际=%s", domains[1].Description)
		}
	})

	t.Run("空结果", func(t *testing.T) {
		qr := database.NewQueryResultWithRows(nil)
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		domains, err := svc.ListDomains(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(domains) != 0 {
			t.Errorf("期望 0 个域，实际 %d", len(domains))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewDomainAdminService(db, nil)

		_, err := svc.ListDomains(ctx)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}
		svc := NewDomainAdminService(db, nil)

		_, err := svc.ListDomains(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestDomainAdminService_GetDomain 测试获取单个域详情
func TestDomainAdminService_GetDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("域存在", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "domain1", "描述1", "2024-01-01T00:00:00Z", int64(10), int64(2), int64(5)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		domain, err := svc.GetDomain(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if domain == nil {
			t.Fatal("期望返回非 nil")
		}
		if domain.ID != 1 || domain.Name != "domain1" {
			t.Errorf("期望 ID=1 Name=domain1，实际 ID=%d Name=%s", domain.ID, domain.Name)
		}
		if domain.UserCount != 10 || domain.ExecutorCount != 2 || domain.TaskCount != 5 {
			t.Errorf("期望 UserCount=10 ExecutorCount=2 TaskCount=5，实际 %d/%d/%d",
				domain.UserCount, domain.ExecutorCount, domain.TaskCount)
		}
	})

	t.Run("域不存在", func(t *testing.T) {
		qr := database.NewQueryResultWithRows(nil)
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		domain, err := svc.GetDomain(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if domain != nil {
			t.Error("期望 nil（未找到）")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewDomainAdminService(db, nil)

		_, err := svc.GetDomain(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}
		svc := NewDomainAdminService(db, nil)

		_, err := svc.GetDomain(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("断言查询参数", func(t *testing.T) {
		qr := database.NewQueryResultWithRows(nil)
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		_, _ = svc.GetDomain(ctx, 42)
		if db.LastQueryStmt.Arguments[0] != int64(42) {
			t.Errorf("期望参数 42，实际=%v", db.LastQueryStmt.Arguments[0])
		}
	})
}

// TestDomainAdminService_CreateDomain 测试创建域
func TestDomainAdminService_CreateDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("创建成功", func(t *testing.T) {
		// GetDomainByID 查询返回新建的域
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "newdomain", "新域描述", "2024-01-01T00:00:00Z"},
		})
		wr := database.NewWriteResult(1, 1) // LastInsertID=1
		db := &MockDB{QueryResult: qr, WriteResult: wr}
		svc := NewDomainAdminService(db, nil)

		domain, err := svc.CreateDomain(ctx, "newdomain", "新域描述")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if domain == nil {
			t.Fatal("期望返回非 nil")
		}
		if domain.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", domain.ID)
		}
		if domain.Name != "newdomain" {
			t.Errorf("期望 Name=newdomain，实际=%s", domain.Name)
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		wr := database.NewWriteResult(0, 0)
		db := &MockDB{WriteResult: wr, WriteError: ErrMockDB}
		svc := NewDomainAdminService(db, nil)

		_, err := svc.CreateDomain(ctx, "newdomain", "")
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误", func(t *testing.T) {
		wr := rqlite.WriteResult{Err: errors.New("write result error")}
		db := &MockDB{WriteResult: wr}
		svc := NewDomainAdminService(db, nil)

		_, err := svc.CreateDomain(ctx, "newdomain", "")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("断言写入参数", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "newdomain", "新域描述", "2024-01-01T00:00:00Z"},
		})
		wr := database.NewWriteResult(1, 1)
		db := &MockDB{QueryResult: qr, WriteResult: wr}
		svc := NewDomainAdminService(db, nil)

		_, _ = svc.CreateDomain(ctx, "newdomain", "新域描述")
		// 验证写入语句的参数（name, description, now）
		args := db.LastWriteStmt.Arguments
		if len(args) < 2 {
			t.Fatalf("期望至少 2 个参数，实际 %d", len(args))
		}
		if args[0] != "newdomain" {
			t.Errorf("期望第 1 个参数 newdomain，实际=%v", args[0])
		}
		if args[1] != "新域描述" {
			t.Errorf("期望第 2 个参数 新域描述，实际=%v", args[1])
		}
	})
}

// TestDomainAdminService_UpdateDomain 测试更新域
func TestDomainAdminService_UpdateDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("更新成功", func(t *testing.T) {
		// GetDomainByID 返回更新后的域
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "updated", "更新后描述", "2024-01-01T00:00:00Z"},
		})
		wr := database.NewWriteResult(0, 1)
		db := &MockDB{QueryResult: qr, WriteResult: wr}
		svc := NewDomainAdminService(db, nil)

		domain, err := svc.UpdateDomain(ctx, 1, "updated", "更新后描述")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if domain == nil {
			t.Fatal("期望返回非 nil")
		}
		if domain.Name != "updated" {
			t.Errorf("期望 Name=updated，实际=%s", domain.Name)
		}
		if domain.Description != "更新后描述" {
			t.Errorf("期望 Description=更新后描述，实际=%s", domain.Description)
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		wr := database.NewWriteResult(0, 0)
		db := &MockDB{WriteResult: wr, WriteError: ErrMockDB}
		svc := NewDomainAdminService(db, nil)

		_, err := svc.UpdateDomain(ctx, 1, "updated", "")
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("断言写入参数", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "updated", "更新后描述", "2024-01-01T00:00:00Z"},
		})
		wr := database.NewWriteResult(0, 1)
		db := &MockDB{QueryResult: qr, WriteResult: wr}
		svc := NewDomainAdminService(db, nil)

		_, _ = svc.UpdateDomain(ctx, 42, "updated", "更新后描述")
		args := db.LastWriteStmt.Arguments
		if len(args) < 3 {
			t.Fatalf("期望至少 3 个参数，实际 %d", len(args))
		}
		if args[0] != "updated" {
			t.Errorf("期望第 1 个参数 updated，实际=%v", args[0])
		}
		if args[1] != "更新后描述" {
			t.Errorf("期望第 2 个参数 更新后描述，实际=%v", args[1])
		}
		if args[2] != int64(42) {
			t.Errorf("期望第 3 个参数 42，实际=%v", args[2])
		}
	})
}

// TestDomainAdminService_DeleteDomain 测试删除域
func TestDomainAdminService_DeleteDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("删除成功", func(t *testing.T) {
		// GetDomain 返回一个无资源的域
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "domain1", "描述1", "2024-01-01T00:00:00Z", int64(0), int64(0), int64(0)},
		})
		wr := database.NewWriteResult(0, 1)
		db := &MockDB{QueryResult: qr, WriteResult: wr}
		svc := NewDomainAdminService(db, nil)

		err := svc.DeleteDomain(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("域不存在", func(t *testing.T) {
		// GetDomain 返回 nil（空结果）
		qr := database.NewQueryResultWithRows(nil)
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		err := svc.DeleteDomain(ctx, 999)
		if err != ErrDomainNotFound {
			t.Errorf("期望 ErrDomainNotFound，实际: %v", err)
		}
	})

	t.Run("域有用户资源", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "domain1", "描述1", "2024-01-01T00:00:00Z", int64(10), int64(0), int64(0)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		err := svc.DeleteDomain(ctx, 1)
		if err != ErrDomainHasResources {
			t.Errorf("期望 ErrDomainHasResources，实际: %v", err)
		}
	})

	t.Run("域有执行器资源", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "domain1", "描述1", "2024-01-01T00:00:00Z", int64(0), int64(2), int64(0)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		err := svc.DeleteDomain(ctx, 1)
		if err != ErrDomainHasResources {
			t.Errorf("期望 ErrDomainHasResources，实际: %v", err)
		}
	})

	t.Run("域有任务资源", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "domain1", "描述1", "2024-01-01T00:00:00Z", int64(0), int64(0), int64(5)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		err := svc.DeleteDomain(ctx, 1)
		if err != ErrDomainHasResources {
			t.Errorf("期望 ErrDomainHasResources，实际: %v", err)
		}
	})

	t.Run("查询域时错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewDomainAdminService(db, nil)

		err := svc.DeleteDomain(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询域时结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}
		svc := NewDomainAdminService(db, nil)

		err := svc.DeleteDomain(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("删除写入错误", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "domain1", "描述1", "2024-01-01T00:00:00Z", int64(0), int64(0), int64(0)},
		})
		wr := database.NewWriteResult(0, 0)
		db := &MockDB{QueryResult: qr, WriteResult: wr, WriteError: ErrMockDB}
		svc := NewDomainAdminService(db, nil)

		err := svc.DeleteDomain(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// TestDomainAdminService_GetDomainByID 测试根据 ID 获取域
func TestDomainAdminService_GetDomainByID(t *testing.T) {
	ctx := context.Background()

	t.Run("域存在", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "domain1", "描述1", "2024-01-01T00:00:00Z"},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		domain, err := svc.GetDomainByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if domain == nil {
			t.Fatal("期望返回非 nil")
		}
		if domain.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", domain.ID)
		}
		if domain.Name != "domain1" {
			t.Errorf("期望 Name=domain1，实际=%s", domain.Name)
		}
		if domain.Description != "描述1" {
			t.Errorf("期望 Description=描述1，实际=%s", domain.Description)
		}
	})

	t.Run("域不存在", func(t *testing.T) {
		qr := database.NewQueryResultWithRows(nil)
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		domain, err := svc.GetDomainByID(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if domain != nil {
			t.Error("期望 nil（未找到）")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewDomainAdminService(db, nil)

		_, err := svc.GetDomainByID(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Errorf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}
		svc := NewDomainAdminService(db, nil)

		_, err := svc.GetDomainByID(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("断言查询参数", func(t *testing.T) {
		qr := database.NewQueryResultWithRows(nil)
		db := &MockDB{QueryResult: qr}
		svc := NewDomainAdminService(db, nil)

		_, _ = svc.GetDomainByID(ctx, 42)
		if db.LastQueryStmt.Arguments[0] != int64(42) {
			t.Errorf("期望参数 42，实际=%v", db.LastQueryStmt.Arguments[0])
		}
	})
}

// TestDomainAdminService_NilService 测试 nil service 上的方法调用会 panic
func TestDomainAdminService_NilService(t *testing.T) {
	t.Run("nil service 的 ListDomains 应 panic", func(t *testing.T) {
		var svc *DomainAdminService
		defer func() {
			if r := recover(); r == nil {
				t.Error("期望在 nil service 上调用 ListDomains 时 panic")
			}
		}()
		svc.ListDomains(context.Background())
	})

	t.Run("nil service 的 GetDomain 应 panic", func(t *testing.T) {
		var svc *DomainAdminService
		defer func() {
			if r := recover(); r == nil {
				t.Error("期望在 nil service 上调用 GetDomain 时 panic")
			}
		}()
		svc.GetDomain(context.Background(), 1)
	})
}

// TestDomainAdminService_DomainWithStatsModel 验证 DomainWithStats 模型字段
func TestDomainAdminService_DomainWithStatsModel(t *testing.T) {
	t.Run("DomainWithStats 嵌入 Domain", func(t *testing.T) {
		d := &model.DomainWithStats{
			Domain: model.Domain{
				ID:          1,
				Name:        "test",
				Description: "desc",
			},
			UserCount:     5,
			ExecutorCount: 2,
			TaskCount:     3,
		}

		if d.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", d.ID)
		}
		if d.Name != "test" {
			t.Errorf("期望 Name=test，实际=%s", d.Name)
		}
		if d.UserCount != 5 {
			t.Errorf("期望 UserCount=5，实际=%d", d.UserCount)
		}
		if d.ExecutorCount != 2 {
			t.Errorf("期望 ExecutorCount=2，实际=%d", d.ExecutorCount)
		}
		if d.TaskCount != 3 {
			t.Errorf("期望 TaskCount=3，实际=%d", d.TaskCount)
		}
	})
}
