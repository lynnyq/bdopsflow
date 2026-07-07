package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// dsPermRow 构造一行 datasource_permission 查询结果（7 列）
// 列顺序：0=id 1=datasource_id 2=role_id 3=user_id 4=permission_type 5=granted_by 6=granted_at
// 注意：rqlite 返回的是 int64 而非 *int64，所以这里将指针解引用为 int64；
// nil 指针转为 nil interface 表示 NULL。
func dsPermRow(id, dsID int64, roleID, userID *int64, permType string, grantedBy *int64) []interface{} {
	var roleVal, userVal, grantedVal interface{}
	if roleID != nil {
		roleVal = *roleID
	}
	if userID != nil {
		userVal = *userID
	}
	if grantedBy != nil {
		grantedVal = *grantedBy
	}
	return []interface{}{id, dsID, roleVal, userVal, permType, grantedVal, "2026-01-01T00:00:00Z"}
}

// webhookPermRow 构造一行 webhook_permission 查询结果（7 列）
// 列顺序：0=id 1=webhook_id 2=role_id 3=user_id 4=permission_type 5=granted_by 6=granted_at
func webhookPermRow(id, webhookID int64, roleID, userID *int64, permType string, grantedBy *int64) []interface{} {
	var roleVal, userVal, grantedVal interface{}
	if roleID != nil {
		roleVal = *roleID
	}
	if userID != nil {
		userVal = *userID
	}
	if grantedBy != nil {
		grantedVal = *grantedBy
	}
	return []interface{}{id, webhookID, roleVal, userVal, permType, grantedVal, "2026-01-01T00:00:00Z"}
}

// instanceCountRow 构造一行 COUNT 查询结果（1 列）
func instanceCountRow(count int64) []interface{} {
	return []interface{}{count}
}

// int64Ptr 返回 int64 指针
func int64Ptr(v int64) *int64 {
	return &v
}

func TestNewInstancePermissionService(t *testing.T) {
	t.Run("构造函数正常赋值", func(t *testing.T) {
		db := &MockDB{}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		if svc == nil {
			t.Fatal("期望返回非 nil 实例")
		}
		if svc.db == nil {
			t.Error("期望 db 正确赋值")
		}
		if svc.permSvc == nil {
			t.Error("期望 permSvc 正确赋值")
		}
	})
}

// ===== HasDatasourcePermission =====

func TestInstancePermissionService_HasDatasourcePermission(t *testing.T) {
	ctx := context.Background()

	t.Run("系统管理员直接通过", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 返回 count=1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
			},
		}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		allowed, err := svc.HasDatasourcePermission(ctx, 100, 5, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !allowed {
			t.Error("期望系统管理员允许访问")
		}
	})

	t.Run("IsSystemAdmin出错返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		_, err := svc.HasDatasourcePermission(ctx, 100, 5, "read")
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("数据源不存在返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 返回 count=0（非管理员）
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// domain_id 查询返回空（数据源不存在）
				database.NewQueryResultWithRows(nil),
			},
		}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		_, err := svc.HasDatasourcePermission(ctx, 100, 999, "read")
		if !errors.Is(err, ErrInstancePermissionDenied) {
			t.Fatalf("期望 ErrInstancePermissionDenied，实际: %v", err)
		}
	})

	t.Run("领域管理员read权限通过", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// domain_id 查询返回 domain_id=1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
				// IsDomainAdmin 返回 count=1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
			},
		}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		allowed, err := svc.HasDatasourcePermission(ctx, 100, 5, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !allowed {
			t.Error("期望领域管理员 read 权限通过")
		}
	})

	t.Run("创建者直接通过", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// domain_id 查询返回 domain_id=1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
				// IsDomainAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// created_by 查询返回 100（等于 userID）
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(100)}),
			},
		}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		allowed, err := svc.HasDatasourcePermission(ctx, 100, 5, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !allowed {
			t.Error("期望创建者允许访问")
		}
	})

	t.Run("有权限记录允许访问", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// domain_id 查询返回 domain_id=1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
				// IsDomainAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// created_by 查询返回 200（不等于 userID=100）
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(200)}),
				// 权限 COUNT 查询返回 1（有权限）
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
			},
		}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		allowed, err := svc.HasDatasourcePermission(ctx, 100, 5, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !allowed {
			t.Error("期望有权限记录时允许访问")
		}
	})

	t.Run("无权限记录拒绝访问", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// domain_id 查询返回 domain_id=1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
				// IsDomainAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// created_by 查询返回 200（不等于 userID=100）
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(200)}),
				// 权限 COUNT 查询返回 0（无权限）
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
			},
		}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		allowed, err := svc.HasDatasourcePermission(ctx, 100, 5, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if allowed {
			t.Error("期望无权限记录时拒绝访问")
		}
	})
}

// ===== GetUserDatasourceIDs =====

func TestInstancePermissionService_GetUserDatasourceIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("返回数据源ID列表", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				{int64(1)},
				{int64(3)},
				{int64(5)},
			}),
		}
		svc := NewInstancePermissionService(db, nil)
		ids, err := svc.GetUserDatasourceIDs(ctx, 100, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(ids) != 3 {
			t.Fatalf("期望 3 个 ID，实际=%d", len(ids))
		}
		if ids[0] != 1 || ids[1] != 3 || ids[2] != 5 {
			t.Errorf("期望 [1,3,5]，实际=%v", ids)
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewInstancePermissionService(db, nil)
		ids, err := svc.GetUserDatasourceIDs(ctx, 100, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("期望 0 个 ID，实际=%d", len(ids))
		}
	})

	t.Run("查询失败返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewInstancePermissionService(db, nil)
		_, err := svc.GetUserDatasourceIDs(ctx, 100, "read")
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithErr(ErrMockDB),
		}
		svc := NewInstancePermissionService(db, nil)
		_, err := svc.GetUserDatasourceIDs(ctx, 100, "read")
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("过滤掉ID为0的记录", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				{int64(0)}, // 应被过滤
				{int64(1)},
				{int64(0)}, // 应被过滤
			}),
		}
		svc := NewInstancePermissionService(db, nil)
		ids, err := svc.GetUserDatasourceIDs(ctx, 100, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(ids) != 1 {
			t.Errorf("期望 1 个 ID（过滤掉0），实际=%d", len(ids))
		}
	})
}

// ===== GetUserDatasourcePermissionLevels =====

func TestInstancePermissionService_GetUserDatasourcePermissionLevels(t *testing.T) {
	ctx := context.Background()

	t.Run("返回权限级别映射", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				{int64(1), "read"},
				{int64(2), "manage"},
				{int64(1), "query"}, // 同一数据源，应取最高权限
			}),
		}
		svc := NewInstancePermissionService(db, nil)
		result, err := svc.GetUserDatasourcePermissionLevels(ctx, 100, []int64{1, 2})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("期望 2 个条目，实际=%d", len(result))
		}
		// 数据源1: read vs query → query (weight 30 > 20)
		if result[1] != "query" {
			t.Errorf("期望数据源1权限=query，实际=%s", result[1])
		}
		// 数据源2: manage
		if result[2] != "manage" {
			t.Errorf("期望数据源2权限=manage，实际=%s", result[2])
		}
	})

	t.Run("空datasourceIDs返回空map", func(t *testing.T) {
		svc := NewInstancePermissionService(&MockDB{}, nil)
		result, err := svc.GetUserDatasourcePermissionLevels(ctx, 100, []int64{})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("期望 0 个条目，实际=%d", len(result))
		}
	})

	t.Run("查询失败返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewInstancePermissionService(db, nil)
		_, err := svc.GetUserDatasourcePermissionLevels(ctx, 100, []int64{1})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithErr(ErrMockDB),
		}
		svc := NewInstancePermissionService(db, nil)
		_, err := svc.GetUserDatasourcePermissionLevels(ctx, 100, []int64{1})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ===== HasWebhookPermission =====

func TestInstancePermissionService_HasWebhookPermission(t *testing.T) {
	ctx := context.Background()

	t.Run("系统管理员直接通过", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 返回 count=1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
			},
		}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		allowed, err := svc.HasWebhookPermission(ctx, 100, 5, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !allowed {
			t.Error("期望系统管理员允许访问")
		}
	})

	t.Run("IsSystemAdmin出错返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		_, err := svc.HasWebhookPermission(ctx, 100, 5, "read")
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("webhook不存在返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// domain_id 查询返回空
				database.NewQueryResultWithRows(nil),
			},
		}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		_, err := svc.HasWebhookPermission(ctx, 100, 999, "read")
		if !errors.Is(err, ErrInstancePermissionDenied) {
			t.Fatalf("期望 ErrInstancePermissionDenied，实际: %v", err)
		}
	})

	t.Run("领域管理员read权限通过", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// domain_id 查询返回 domain_id=1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
				// IsDomainAdmin 返回 count=1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
			},
		}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		allowed, err := svc.HasWebhookPermission(ctx, 100, 5, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !allowed {
			t.Error("期望领域管理员 read 权限通过")
		}
	})

	t.Run("创建者直接通过", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// domain_id 查询返回 domain_id=1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
				// IsDomainAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// created_by 查询返回 100（等于 userID）
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(100)}),
			},
		}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		allowed, err := svc.HasWebhookPermission(ctx, 100, 5, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !allowed {
			t.Error("期望创建者允许访问")
		}
	})

	t.Run("有权限记录允许访问", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// domain_id 查询返回 domain_id=1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
				// IsDomainAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// created_by 查询返回 200（不等于 userID=100）
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(200)}),
				// 权限 COUNT 查询返回 1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
			},
		}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		allowed, err := svc.HasWebhookPermission(ctx, 100, 5, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !allowed {
			t.Error("期望有权限记录时允许访问")
		}
	})

	t.Run("无权限记录拒绝访问", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				// IsSystemAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// domain_id 查询返回 domain_id=1
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(1)}),
				// IsDomainAdmin 返回 count=0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
				// created_by 查询返回 200
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(200)}),
				// 权限 COUNT 查询返回 0
				database.NewQueryResultWithRows([][]interface{}{instanceCountRow(0)}),
			},
		}
		permSvc := NewPermissionService(db, nil)
		svc := NewInstancePermissionService(db, permSvc)
		allowed, err := svc.HasWebhookPermission(ctx, 100, 5, "read")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if allowed {
			t.Error("期望无权限记录时拒绝访问")
		}
	})
}

// ===== GrantDatasourcePermission =====

func TestInstancePermissionService_GrantDatasourcePermission(t *testing.T) {
	ctx := context.Background()

	t.Run("授权用户成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(1, 1),
		}
		svc := NewInstancePermissionService(db, nil)
		userID := int64(100)
		err := svc.GrantDatasourcePermission(ctx, 5, nil, &userID, "read", 200)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 验证参数：datasource_id, role_id, user_id, permission_type, granted_by, granted_at
		if db.LastWriteStmt.Arguments[0] != int64(5) {
			t.Errorf("期望 datasource_id=5，实际=%v", db.LastWriteStmt.Arguments[0])
		}
		// role_id 是 *int64 类型的 nil，在 interface 中不为 nil（typed nil）
		// 需要类型断言检查
		if v, ok := db.LastWriteStmt.Arguments[1].(*int64); ok && v != nil {
			t.Errorf("期望 role_id=nil，实际=%v", db.LastWriteStmt.Arguments[1])
		}
		if db.LastWriteStmt.Arguments[3] != "read" {
			t.Errorf("期望 permission_type=read，实际=%v", db.LastWriteStmt.Arguments[3])
		}
	})

	t.Run("授权角色成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(1, 1),
		}
		svc := NewInstancePermissionService(db, nil)
		roleID := int64(1)
		err := svc.GrantDatasourcePermission(ctx, 5, &roleID, nil, "manage", 200)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewInstancePermissionService(db, nil)
		err := svc.GrantDatasourcePermission(ctx, 5, nil, nil, "read", 200)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewInstancePermissionService(db, nil)
		err := svc.GrantDatasourcePermission(ctx, 5, nil, nil, "read", 200)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ===== RevokeDatasourcePermission =====

func TestInstancePermissionService_RevokeDatasourcePermission(t *testing.T) {
	ctx := context.Background()

	t.Run("撤销成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewInstancePermissionService(db, nil)
		err := svc.RevokeDatasourcePermission(ctx, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.LastWriteStmt.Arguments[0] != int64(5) {
			t.Errorf("期望 perm_id=5，实际=%v", db.LastWriteStmt.Arguments[0])
		}
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewInstancePermissionService(db, nil)
		err := svc.RevokeDatasourcePermission(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ===== GetDatasourcePermissions =====

func TestInstancePermissionService_GetDatasourcePermissions(t *testing.T) {
	ctx := context.Background()

	t.Run("返回权限列表", func(t *testing.T) {
		roleID := int64(1)
		userID := int64(100)
		grantedBy := int64(200)
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				dsPermRow(1, 5, &roleID, nil, "read", &grantedBy),
				dsPermRow(2, 5, nil, &userID, "manage", &grantedBy),
			}),
		}
		svc := NewInstancePermissionService(db, nil)
		perms, err := svc.GetDatasourcePermissions(ctx, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 2 {
			t.Fatalf("期望 2 条权限，实际=%d", len(perms))
		}
		if perms[0].ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", perms[0].ID)
		}
		if perms[0].DatasourceID != 5 {
			t.Errorf("期望 DatasourceID=5，实际=%d", perms[0].DatasourceID)
		}
		if perms[0].PermissionType != "read" {
			t.Errorf("期望 PermissionType=read，实际=%s", perms[0].PermissionType)
		}
		if perms[0].RoleID == nil || *perms[0].RoleID != 1 {
			t.Errorf("期望 RoleID=1，实际=%v", perms[0].RoleID)
		}
		if perms[0].UserID != nil {
			t.Errorf("期望 UserID=nil，实际=%v", perms[0].UserID)
		}
		if perms[0].GrantedBy == nil || *perms[0].GrantedBy != 200 {
			t.Errorf("期望 GrantedBy=200，实际=%v", perms[0].GrantedBy)
		}
		// 第二条：用户权限
		if perms[1].RoleID != nil {
			t.Errorf("期望第二条 RoleID=nil，实际=%v", perms[1].RoleID)
		}
		if perms[1].UserID == nil || *perms[1].UserID != 100 {
			t.Errorf("期望第二条 UserID=100，实际=%v", perms[1].UserID)
		}
		if perms[1].PermissionType != "manage" {
			t.Errorf("期望第二条 PermissionType=manage，实际=%s", perms[1].PermissionType)
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewInstancePermissionService(db, nil)
		perms, err := svc.GetDatasourcePermissions(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 0 {
			t.Errorf("期望 0 条权限，实际=%d", len(perms))
		}
	})

	t.Run("查询失败返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewInstancePermissionService(db, nil)
		_, err := svc.GetDatasourcePermissions(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithErr(ErrMockDB),
		}
		svc := NewInstancePermissionService(db, nil)
		_, err := svc.GetDatasourcePermissions(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ===== GrantWebhookPermission =====

func TestInstancePermissionService_GrantWebhookPermission(t *testing.T) {
	ctx := context.Background()

	t.Run("授权用户成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(1, 1),
		}
		svc := NewInstancePermissionService(db, nil)
		userID := int64(100)
		err := svc.GrantWebhookPermission(ctx, 5, nil, &userID, "read", 200)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.LastWriteStmt.Arguments[0] != int64(5) {
			t.Errorf("期望 webhook_id=5，实际=%v", db.LastWriteStmt.Arguments[0])
		}
		if db.LastWriteStmt.Arguments[3] != "read" {
			t.Errorf("期望 permission_type=read，实际=%v", db.LastWriteStmt.Arguments[3])
		}
	})

	t.Run("授权角色成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(1, 1),
		}
		svc := NewInstancePermissionService(db, nil)
		roleID := int64(1)
		err := svc.GrantWebhookPermission(ctx, 5, &roleID, nil, "trigger", 200)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewInstancePermissionService(db, nil)
		err := svc.GrantWebhookPermission(ctx, 5, nil, nil, "read", 200)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewInstancePermissionService(db, nil)
		err := svc.GrantWebhookPermission(ctx, 5, nil, nil, "read", 200)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ===== RevokeWebhookPermission =====

func TestInstancePermissionService_RevokeWebhookPermission(t *testing.T) {
	ctx := context.Background()

	t.Run("撤销成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewInstancePermissionService(db, nil)
		err := svc.RevokeWebhookPermission(ctx, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.LastWriteStmt.Arguments[0] != int64(5) {
			t.Errorf("期望 perm_id=5，实际=%v", db.LastWriteStmt.Arguments[0])
		}
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewInstancePermissionService(db, nil)
		err := svc.RevokeWebhookPermission(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ===== GetWebhookPermissions =====

func TestInstancePermissionService_GetWebhookPermissions(t *testing.T) {
	ctx := context.Background()

	t.Run("返回权限列表", func(t *testing.T) {
		roleID := int64(1)
		userID := int64(100)
		grantedBy := int64(200)
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				webhookPermRow(1, 5, &roleID, nil, "read", &grantedBy),
				webhookPermRow(2, 5, nil, &userID, "trigger", &grantedBy),
			}),
		}
		svc := NewInstancePermissionService(db, nil)
		perms, err := svc.GetWebhookPermissions(ctx, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 2 {
			t.Fatalf("期望 2 条权限，实际=%d", len(perms))
		}
		if perms[0].ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", perms[0].ID)
		}
		if perms[0].WebhookID != 5 {
			t.Errorf("期望 WebhookID=5，实际=%d", perms[0].WebhookID)
		}
		if perms[0].PermissionType != "read" {
			t.Errorf("期望 PermissionType=read，实际=%s", perms[0].PermissionType)
		}
		if perms[0].RoleID == nil || *perms[0].RoleID != 1 {
			t.Errorf("期望 RoleID=1，实际=%v", perms[0].RoleID)
		}
		if perms[0].UserID != nil {
			t.Errorf("期望 UserID=nil，实际=%v", perms[0].UserID)
		}
		if perms[0].GrantedBy == nil || *perms[0].GrantedBy != 200 {
			t.Errorf("期望 GrantedBy=200，实际=%v", perms[0].GrantedBy)
		}
		// 第二条
		if perms[1].PermissionType != "trigger" {
			t.Errorf("期望第二条 PermissionType=trigger，实际=%s", perms[1].PermissionType)
		}
		if perms[1].UserID == nil || *perms[1].UserID != 100 {
			t.Errorf("期望第二条 UserID=100，实际=%v", perms[1].UserID)
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewInstancePermissionService(db, nil)
		perms, err := svc.GetWebhookPermissions(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 0 {
			t.Errorf("期望 0 条权限，实际=%d", len(perms))
		}
	})

	t.Run("查询失败返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewInstancePermissionService(db, nil)
		_, err := svc.GetWebhookPermissions(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithErr(ErrMockDB),
		}
		svc := NewInstancePermissionService(db, nil)
		_, err := svc.GetWebhookPermissions(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

// ===== 辅助函数测试 =====

func TestGetWebhookEffectivePermissions(t *testing.T) {
	t.Run("read权限包含read本身", func(t *testing.T) {
		perms := getWebhookEffectivePermissions("read")
		found := false
		for _, p := range perms {
			if p == "read" {
				found = true
				break
			}
		}
		if !found {
			t.Error("期望 read 在有效权限列表中")
		}
	})

	t.Run("trigger权限包含trigger和read", func(t *testing.T) {
		// trigger 包含 read，所以 trigger 的有效权限应包含 trigger 本身
		// 以及包含 read 的权限（trigger 和 read 本身）
		perms := getWebhookEffectivePermissions("read")
		foundTrigger := false
		for _, p := range perms {
			if p == "trigger" {
				foundTrigger = true
				break
			}
		}
		if !foundTrigger {
			t.Error("期望 trigger 在 read 的有效权限列表中（因为 trigger 包含 read）")
		}
	})

	t.Run("manage权限包含所有", func(t *testing.T) {
		// manage 包含 update, read, delete, trigger
		// 所以对于 read，manage 应该在有效列表中
		perms := getWebhookEffectivePermissions("read")
		foundManage := false
		for _, p := range perms {
			if p == "manage" {
				foundManage = true
				break
			}
		}
		if !foundManage {
			t.Error("期望 manage 在 read 的有效权限列表中")
		}
	})

	t.Run("未知权限返回自身", func(t *testing.T) {
		perms := getWebhookEffectivePermissions("unknown")
		if len(perms) != 1 {
			t.Fatalf("期望 1 个权限，实际=%d", len(perms))
		}
		if perms[0] != "unknown" {
			t.Errorf("期望 unknown，实际=%s", perms[0])
		}
	})
}

func TestHighestPermission(t *testing.T) {
	t.Run("manage高于read", func(t *testing.T) {
		if highestPermission("read", "manage") != "manage" {
			t.Error("期望 manage 高于 read")
		}
		if highestPermission("manage", "read") != "manage" {
			t.Error("期望 manage 高于 read")
		}
	})

	t.Run("query高于read", func(t *testing.T) {
		if highestPermission("read", "query") != "query" {
			t.Error("期望 query 高于 read")
		}
	})

	t.Run("manage高于所有", func(t *testing.T) {
		for _, perm := range []string{"read", "query", "update", "download", "delete"} {
			if highestPermission(perm, "manage") != "manage" {
				t.Errorf("期望 manage 高于 %s", perm)
			}
		}
	})

	t.Run("相同权限返回自身", func(t *testing.T) {
		if highestPermission("read", "read") != "read" {
			t.Error("期望相同权限返回自身")
		}
	})

	t.Run("未知权限a返回b", func(t *testing.T) {
		if highestPermission("unknown", "read") != "read" {
			t.Error("期望未知权限时返回已知的 b")
		}
	})

	t.Run("未知权限b返回a", func(t *testing.T) {
		if highestPermission("read", "unknown") != "read" {
			t.Error("期望未知权限时返回已知的 a")
		}
	})
}

// ===== 编译时验证 model 类型 =====

func TestInstancePermission_ModelTypes(t *testing.T) {
	t.Run("DatasourcePermission字段", func(t *testing.T) {
		roleID := int64(1)
		userID := int64(100)
		grantedBy := int64(200)
		perm := &model.DatasourcePermission{
			ID:             1,
			DatasourceID:   5,
			RoleID:         &roleID,
			UserID:         &userID,
			PermissionType: "read",
			GrantedBy:      &grantedBy,
			GrantedAt:      "2026-01-01T00:00:00Z",
		}
		if perm.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", perm.ID)
		}
		if perm.DatasourceID != 5 {
			t.Errorf("期望 DatasourceID=5，实际=%d", perm.DatasourceID)
		}
		if perm.RoleID == nil || *perm.RoleID != 1 {
			t.Errorf("期望 RoleID=1，实际=%v", perm.RoleID)
		}
	})

	t.Run("WebhookPermission字段", func(t *testing.T) {
		userID := int64(100)
		perm := &model.WebhookPermission{
			ID:             2,
			WebhookID:      5,
			UserID:         &userID,
			PermissionType: "trigger",
			GrantedAt:      "2026-01-01T00:00:00Z",
		}
		if perm.WebhookID != 5 {
			t.Errorf("期望 WebhookID=5，实际=%d", perm.WebhookID)
		}
		if perm.PermissionType != "trigger" {
			t.Errorf("期望 PermissionType=trigger，实际=%s", perm.PermissionType)
		}
	})
}
