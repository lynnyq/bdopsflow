package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// newRoleAdminServiceWithSeparateDBs 构造一个 RoleAdminService，其中 permSvc 和 RoleAdminService 使用不同的 MockDB。
// 这样可以分别控制 permSvc.GetRoleByID 等委托方法的返回值和直接 db 调用的返回值。
func newRoleAdminServiceWithSeparateDBs(permDB, roleDB *MockDB) *RoleAdminService {
	permSvc := NewPermissionService(permDB, nil)
	return NewRoleAdminService(roleDB, permSvc)
}

// TestNewRoleAdminService 测试构造函数
func TestNewRoleAdminService(t *testing.T) {
	t.Run("所有参数为 nil 时仍可创建实例", func(t *testing.T) {
		svc := NewRoleAdminService(nil, nil)
		if svc == nil {
			t.Fatal("期望返回非 nil 实例")
		}
	})

	t.Run("参数正确赋值", func(t *testing.T) {
		db := &MockDB{}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)
		if svc.db == nil || svc.permSvc == nil {
			t.Error("期望所有字段正确赋值")
		}
	})
}

// TestRoleAdminService_IsSystemAdmin 测试系统管理员检查（委托给 permSvc）
func TestRoleAdminService_IsSystemAdmin(t *testing.T) {
	ctx := context.Background()

	t.Run("是系统管理员", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1)},
		})
		db := &MockDB{QueryResult: qr}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

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
		svc := NewRoleAdminService(db, permSvc)

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
		svc := NewRoleAdminService(db, permSvc)

		_, err := svc.IsSystemAdmin(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestRoleAdminService_ListRoles 测试列出角色
func TestRoleAdminService_ListRoles(t *testing.T) {
	ctx := context.Background()

	t.Run("系统管理员-返回所有角色", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Admin", "admin", "管理员", true, nil, nil},
			{int64(2), "User", "user", "用户", false, nil, nil},
		})
		db := &MockDB{QueryResult: qr}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		roles, err := svc.ListRoles(ctx, 0, true)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roles) != 2 {
			t.Errorf("期望 2 个角色，实际=%d", len(roles))
		}
	})

	t.Run("非系统管理员-按域返回角色", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(2), "User", "user", "用户", false, nil, int64(10)},
		})
		db := &MockDB{QueryResult: qr}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		roles, err := svc.ListRoles(ctx, 10, false)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roles) != 1 {
			t.Errorf("期望 1 个角色，实际=%d", len(roles))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		_, err := svc.ListRoles(ctx, 0, true)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestRoleAdminService_GetRole 测试获取单个角色
func TestRoleAdminService_GetRole(t *testing.T) {
	ctx := context.Background()

	t.Run("角色存在", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Admin", "admin", "管理员", true, nil, nil},
		})
		db := &MockDB{QueryResult: qr}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		role, err := svc.GetRole(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if role == nil || role.ID != 1 || role.Code != "admin" {
			t.Errorf("角色字段不正确: %+v", role)
		}
	})

	t.Run("角色不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		role, err := svc.GetRole(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if role != nil {
			t.Error("期望 role 为 nil")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		_, err := svc.GetRole(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestRoleAdminService_CreateRole 测试创建角色
func TestRoleAdminService_CreateRole(t *testing.T) {
	ctx := context.Background()

	t.Run("创建成功-无父角色无域", func(t *testing.T) {
		// permSvc 返回新创建的角色，roleDB 处理写入
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Editor", "editor", "编辑者", false, nil, nil},
		})
		permDB := &MockDB{QueryResult: permQR}
		roleDB := &MockDB{WriteResult: database.NewWriteResult(1, 1)}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		role, err := svc.CreateRole(ctx, "Editor", "editor", "编辑者", nil, nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if role == nil || role.ID != 1 || role.Code != "editor" {
			t.Errorf("期望返回 ID=1 的角色，实际=%+v", role)
		}
		// 验证写入参数
		if roleDB.LastWriteStmt.Arguments[0] != "Editor" {
			t.Errorf("期望 name=Editor，实际=%v", roleDB.LastWriteStmt.Arguments[0])
		}
		if roleDB.LastWriteStmt.Arguments[1] != "editor" {
			t.Errorf("期望 code=editor，实际=%v", roleDB.LastWriteStmt.Arguments[1])
		}
	})

	t.Run("创建成功-带父角色和域", func(t *testing.T) {
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(2), "SubEditor", "sub_editor", "子编辑者", false, int64(1), int64(10)},
		})
		permDB := &MockDB{QueryResult: permQR}
		roleDB := &MockDB{WriteResult: database.NewWriteResult(2, 1)}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		parentID := int64(1)
		domainID := int64(10)
		role, err := svc.CreateRole(ctx, "SubEditor", "sub_editor", "子编辑者", &parentID, &domainID)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if role == nil || role.ID != 2 {
			t.Errorf("期望返回 ID=2 的角色，实际=%+v", role)
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		permDB := &MockDB{}
		roleDB := &MockDB{WriteError: ErrMockDB}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		_, err := svc.CreateRole(ctx, "Editor", "editor", "", nil, nil)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestRoleAdminService_UpdateRole 测试更新角色
func TestRoleAdminService_UpdateRole(t *testing.T) {
	ctx := context.Background()

	t.Run("更新成功", func(t *testing.T) {
		// permSvc 返回非系统角色
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Editor", "editor", "编辑者", false, nil, nil},
		})
		permDB := &MockDB{QueryResult: permQR}
		roleDB := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		role, err := svc.UpdateRole(ctx, 1, "NewEditor", "新编辑者")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if role == nil {
			t.Fatal("期望返回非 nil 角色")
		}
		// 验证写入参数
		if roleDB.LastWriteStmt.Arguments[0] != "NewEditor" {
			t.Errorf("期望 name=NewEditor，实际=%v", roleDB.LastWriteStmt.Arguments[0])
		}
	})

	t.Run("角色不存在", func(t *testing.T) {
		permDB := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		roleDB := &MockDB{}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		role, err := svc.UpdateRole(ctx, 999, "Name", "Desc")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if role != nil {
			t.Error("期望 role 为 nil")
		}
	})

	t.Run("系统角色不可修改", func(t *testing.T) {
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Admin", "admin", "管理员", true, nil, nil},
		})
		permDB := &MockDB{QueryResult: permQR}
		roleDB := &MockDB{}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		_, err := svc.UpdateRole(ctx, 1, "NewAdmin", "新管理员")
		if err != ErrSystemRoleCannotModify {
			t.Errorf("期望 ErrSystemRoleCannotModify，实际=%v", err)
		}
	})

	t.Run("查询角色错误", func(t *testing.T) {
		permDB := &MockDB{QueryError: ErrMockDB}
		roleDB := &MockDB{}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		_, err := svc.UpdateRole(ctx, 1, "Name", "Desc")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Editor", "editor", "编辑者", false, nil, nil},
		})
		permDB := &MockDB{QueryResult: permQR}
		roleDB := &MockDB{WriteError: ErrMockDB}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		_, err := svc.UpdateRole(ctx, 1, "NewEditor", "新编辑者")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestRoleAdminService_DeleteRole 测试删除角色
func TestRoleAdminService_DeleteRole(t *testing.T) {
	ctx := context.Background()

	t.Run("删除成功", func(t *testing.T) {
		// permSvc 返回非系统角色
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Editor", "editor", "编辑者", false, nil, nil},
		})
		permDB := &MockDB{QueryResult: permQR}
		// roleDB 子查询返回 0 个子角色
		roleQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(0)},
		})
		roleDB := &MockDB{
			QueryResult: roleQR,
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		err := svc.DeleteRole(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 3 次写入：删除角色权限、删除用户角色、删除角色
		if len(roleDB.WriteStmts) != 3 {
			t.Errorf("期望 3 次写入调用，实际=%d", len(roleDB.WriteStmts))
		}
	})

	t.Run("角色不存在", func(t *testing.T) {
		permDB := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		roleDB := &MockDB{}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		err := svc.DeleteRole(ctx, 999)
		if err != ErrRoleNotFound {
			t.Errorf("期望 ErrRoleNotFound，实际=%v", err)
		}
	})

	t.Run("系统角色不可删除", func(t *testing.T) {
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Admin", "admin", "管理员", true, nil, nil},
		})
		permDB := &MockDB{QueryResult: permQR}
		roleDB := &MockDB{}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		err := svc.DeleteRole(ctx, 1)
		if err != ErrSystemRoleCannotDelete {
			t.Errorf("期望 ErrSystemRoleCannotDelete，实际=%v", err)
		}
	})

	t.Run("有子角色不可删除", func(t *testing.T) {
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Parent", "parent", "父角色", false, nil, nil},
		})
		permDB := &MockDB{QueryResult: permQR}
		// 子角色数量 > 0
		roleQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(2)},
		})
		roleDB := &MockDB{QueryResult: roleQR}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		err := svc.DeleteRole(ctx, 1)
		if err != ErrCannotDeleteRoleWithChildren {
			t.Errorf("期望 ErrCannotDeleteRoleWithChildren，实际=%v", err)
		}
	})

	t.Run("查询角色错误", func(t *testing.T) {
		permDB := &MockDB{QueryError: ErrMockDB}
		roleDB := &MockDB{}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		err := svc.DeleteRole(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("子角色查询错误", func(t *testing.T) {
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Editor", "editor", "编辑者", false, nil, nil},
		})
		permDB := &MockDB{QueryResult: permQR}
		roleDB := &MockDB{QueryError: ErrMockDB}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		err := svc.DeleteRole(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("删除角色权限时写入错误", func(t *testing.T) {
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Editor", "editor", "编辑者", false, nil, nil},
		})
		permDB := &MockDB{QueryResult: permQR}
		roleQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(0)},
		})
		roleDB := &MockDB{
			QueryResult: roleQR,
			WriteError:  ErrMockDB,
		}
		svc := newRoleAdminServiceWithSeparateDBs(permDB, roleDB)

		err := svc.DeleteRole(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestRoleAdminService_GetRolePermissions 测试获取角色权限
func TestRoleAdminService_GetRolePermissions(t *testing.T) {
	ctx := context.Background()

	t.Run("返回多个权限", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "task", "read", "读取任务"},
			{int64(2), "task", "write", "写入任务"},
		})
		db := &MockDB{QueryResult: qr}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		perms, err := svc.GetRolePermissions(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 2 {
			t.Errorf("期望 2 个权限，实际=%d", len(perms))
		}
		if perms[0].Resource != "task" || perms[0].Action != "read" {
			t.Errorf("第一个权限字段不正确: %+v", perms[0])
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		perms, err := svc.GetRolePermissions(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 0 {
			t.Errorf("期望 0 个权限，实际=%d", len(perms))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		_, err := svc.GetRolePermissions(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestRoleAdminService_AssignPermissionsToRole 测试为角色分配权限
func TestRoleAdminService_AssignPermissionsToRole(t *testing.T) {
	ctx := context.Background()

	t.Run("分配成功", func(t *testing.T) {
		// permSvc 返回非系统角色，并处理 AssignPermissionsToRole 的写入
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Editor", "editor", "编辑者", false, nil, nil},
		})
		permDB := &MockDB{
			QueryResult:    permQR,
			WriteResult:    database.NewWriteResult(0, 1),
			BatchWriteResult: []rqlite.WriteResult{
				database.NewWriteResult(0, 1),
			},
		}
		svc := NewRoleAdminService(&MockDB{}, NewPermissionService(permDB, nil))

		err := svc.AssignPermissionsToRole(ctx, 1, []int64{10, 20})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("角色不存在", func(t *testing.T) {
		permDB := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewRoleAdminService(&MockDB{}, NewPermissionService(permDB, nil))

		err := svc.AssignPermissionsToRole(ctx, 999, []int64{1})
		if err != ErrRoleNotFound {
			t.Errorf("期望 ErrRoleNotFound，实际=%v", err)
		}
	})

	t.Run("系统角色不可修改", func(t *testing.T) {
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Admin", "admin", "管理员", true, nil, nil},
		})
		permDB := &MockDB{QueryResult: permQR}
		svc := NewRoleAdminService(&MockDB{}, NewPermissionService(permDB, nil))

		err := svc.AssignPermissionsToRole(ctx, 1, []int64{1})
		if err != ErrSystemRoleCannotModify {
			t.Errorf("期望 ErrSystemRoleCannotModify，实际=%v", err)
		}
	})

	t.Run("查询角色错误", func(t *testing.T) {
		permDB := &MockDB{QueryError: ErrMockDB}
		svc := NewRoleAdminService(&MockDB{}, NewPermissionService(permDB, nil))

		err := svc.AssignPermissionsToRole(ctx, 1, []int64{1})
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("分配权限写入错误", func(t *testing.T) {
		// permSvc 返回非系统角色，但写入时报错
		permQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Editor", "editor", "编辑者", false, nil, nil},
		})
		permDB := &MockDB{
			QueryResult: permQR,
			WriteError:  ErrMockDB,
		}
		svc := NewRoleAdminService(&MockDB{}, NewPermissionService(permDB, nil))

		err := svc.AssignPermissionsToRole(ctx, 1, []int64{10})
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestRoleAdminService_GetAllPermissions 测试获取所有权限
func TestRoleAdminService_GetAllPermissions(t *testing.T) {
	ctx := context.Background()

	t.Run("返回多个权限", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "task", "read", "读取任务"},
			{int64(2), "task", "write", "写入任务"},
			{int64(3), "user", "read", "读取用户"},
		})
		db := &MockDB{QueryResult: qr}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		perms, err := svc.GetAllPermissions(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 3 {
			t.Errorf("期望 3 个权限，实际=%d", len(perms))
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		perms, err := svc.GetAllPermissions(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(perms) != 0 {
			t.Errorf("期望 0 个权限，实际=%d", len(perms))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		_, err := svc.GetAllPermissions(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		permSvc := NewPermissionService(db, nil)
		svc := NewRoleAdminService(db, permSvc)

		_, err := svc.GetAllPermissions(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}
