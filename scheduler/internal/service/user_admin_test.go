package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/rsautil"
	rqlite "github.com/rqlite/gorqlite"
)

// newTestRSAUtil 构造一个用于测试的 RSAUtil，包含公钥和私钥。
func newTestRSAUtil(t *testing.T) *rsautil.RSAUtil {
	t.Helper()
	pubB64, privB64, err := rsautil.GenerateKeyPair()
	if err != nil {
		t.Fatalf("生成 RSA 密钥对失败: %v", err)
	}
	u, err := rsautil.NewFromConfig(pubB64, privB64)
	if err != nil {
		t.Fatalf("构造 RSAUtil 失败: %v", err)
	}
	return u
}

// encryptTestPassword 使用测试 RSAUtil 加密明文密码，便于测试 CreateUser/ChangePassword/ResetPassword。
func encryptTestPassword(t *testing.T, u *rsautil.RSAUtil, plaintext string) string {
	t.Helper()
	enc, err := u.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("加密测试密码失败: %v", err)
	}
	return enc
}

// newUserAdminServiceWithQueryResult 构造一个 UserAdminService，其 MockDB 查询返回指定结果。
func newUserAdminServiceWithQueryResult(qr rqlite.QueryResult) *UserAdminService {
	permSvc := NewPermissionService(&MockDB{QueryResult: qr}, nil)
	return NewUserAdminService(&MockDB{QueryResult: qr}, permSvc, nil)
}

// newUserAdminServiceWithWriteResult 构造一个 UserAdminService，其 MockDB 写入返回指定结果。
func newUserAdminServiceWithWriteResult(wr rqlite.WriteResult) *UserAdminService {
	permSvc := NewPermissionService(&MockDB{WriteResult: wr}, nil)
	return NewUserAdminService(&MockDB{WriteResult: wr}, permSvc, nil)
}

// TestNewUserAdminService 测试构造函数
func TestNewUserAdminService(t *testing.T) {
	t.Run("所有参数为 nil 时仍可创建实例", func(t *testing.T) {
		svc := NewUserAdminService(nil, nil, nil)
		if svc == nil {
			t.Fatal("期望返回非 nil 实例")
		}
	})

	t.Run("参数正确赋值", func(t *testing.T) {
		db := &MockDB{}
		permSvc := NewPermissionService(db, nil)
		rsaUtil := newTestRSAUtil(t)
		svc := NewUserAdminService(db, permSvc, rsaUtil)
		if svc.db == nil || svc.permSvc == nil || svc.rsaUtil == nil {
			t.Error("期望所有字段正确赋值")
		}
	})
}

// TestValidatePlaintextPassword 测试密码校验逻辑
func TestValidatePlaintextPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"正常密码", "abc123", nil},
		{"过短", "ab1", ErrPasswordTooShort},
		{"过长", string(make([]byte, 31)), ErrPasswordTooLong}, // 全零字节长度 31 > 30
		{"无数字", "abcdef", ErrPasswordWeak},
		{"无字母", "123456", ErrPasswordWeak},
		{"边界-最小长度", "abc123", nil},
		{"边界-最大长度", "abcdefghij1234567890", nil}, // 20 字符
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePlaintextPassword(tt.password)
			if tt.wantErr == nil && err != nil {
				t.Errorf("期望无错误，实际: %v", err)
			}
			if tt.wantErr != nil && err == nil {
				t.Errorf("期望错误 %v，实际无错误", tt.wantErr)
			}
		})
	}
}

// TestUserAdminService_ListUsers 测试列出所有用户
func TestUserAdminService_ListUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("正常返回多个用户", func(t *testing.T) {
		now := time.Now()
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "Alice", "138", "a@b.com", true, now, now, now},
			{int64(2), "bob", "Bob", "139", "b@b.com", false, nil, now, now},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		users, err := svc.ListUsers(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(users) != 2 {
			t.Fatalf("期望 2 个用户，实际=%d", len(users))
		}
		if users[0].ID != 1 || users[0].Username != "alice" {
			t.Errorf("第一个用户字段不正确: %+v", users[0])
		}
		if !users[0].IsActive {
			t.Error("期望 alice IsActive=true")
		}
		if users[1].IsActive {
			t.Error("期望 bob IsActive=false")
		}
		if users[0].LastLoginAt == nil {
			t.Error("期望 alice LastLoginAt 非 nil")
		}
		if users[1].LastLoginAt != nil {
			t.Error("期望 bob LastLoginAt 为 nil")
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewUserAdminService(db, nil, nil)

		users, err := svc.ListUsers(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(users) != 0 {
			t.Errorf("期望 0 个用户，实际=%d", len(users))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.ListUsers(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.ListUsers(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_GetUserByID 测试根据 ID 获取用户
func TestUserAdminService_GetUserByID(t *testing.T) {
	ctx := context.Background()

	t.Run("用户存在", func(t *testing.T) {
		now := time.Now()
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "Alice", "138", "a@b.com", true, "hashedpass", now, now, now},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		user, err := svc.GetUserByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if user == nil {
			t.Fatal("期望返回非 nil 用户")
		}
		if user.ID != 1 || user.Username != "alice" || user.Password != "hashedpass" {
			t.Errorf("用户字段不正确: %+v", user)
		}
	})

	t.Run("用户不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewUserAdminService(db, nil, nil)

		user, err := svc.GetUserByID(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if user != nil {
			t.Error("期望 user 为 nil")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.GetUserByID(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.GetUserByID(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("断言查询参数", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(42), "u", "U", "", "", true, "", nil, nil, nil},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		_, _ = svc.GetUserByID(ctx, 42)
		if db.LastQueryStmt.Arguments[0].(int64) != 42 {
			t.Errorf("期望查询参数 userID=42，实际=%v", db.LastQueryStmt.Arguments[0])
		}
	})
}

// TestUserAdminService_GetUserByUsername 测试根据用户名获取用户
func TestUserAdminService_GetUserByUsername(t *testing.T) {
	ctx := context.Background()

	t.Run("用户存在", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "Alice", "138", "a@b.com", true, nil, nil, nil},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		user, err := svc.GetUserByUsername(ctx, "alice")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if user == nil || user.Username != "alice" {
			t.Errorf("用户字段不正确: %+v", user)
		}
	})

	t.Run("用户不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewUserAdminService(db, nil, nil)

		user, err := svc.GetUserByUsername(ctx, "ghost")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if user != nil {
			t.Error("期望 user 为 nil")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.GetUserByUsername(ctx, "alice")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("断言查询参数", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "", "", "", true, nil, nil, nil},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		_, _ = svc.GetUserByUsername(ctx, "alice")
		if db.LastQueryStmt.Arguments[0] != "alice" {
			t.Errorf("期望查询参数 username=alice，实际=%v", db.LastQueryStmt.Arguments[0])
		}
	})
}

// TestUserAdminService_GetUsersByDomain 测试根据域获取用户列表
func TestUserAdminService_GetUsersByDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("返回多个用户", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "Alice", "138", "a@b.com", true, nil, nil, nil},
			{int64(2), "bob", "Bob", "139", "b@b.com", false, nil, nil, nil},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		users, err := svc.GetUsersByDomain(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(users) != 2 {
			t.Errorf("期望 2 个用户，实际=%d", len(users))
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewUserAdminService(db, nil, nil)

		users, err := svc.GetUsersByDomain(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(users) != 0 {
			t.Errorf("期望 0 个用户，实际=%d", len(users))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.GetUsersByDomain(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("断言查询参数", func(t *testing.T) {
		qr := database.NewQueryResultWithRows(nil)
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		_, _ = svc.GetUsersByDomain(ctx, 7)
		if db.LastQueryStmt.Arguments[0].(int64) != 7 {
			t.Errorf("期望查询参数 domainID=7，实际=%v", db.LastQueryStmt.Arguments[0])
		}
	})
}

// TestUserAdminService_GetUserRoles 测试获取用户角色
func TestUserAdminService_GetUserRoles(t *testing.T) {
	ctx := context.Background()

	t.Run("返回多个角色", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Admin", "admin", "管理员", true, nil, nil, nil, nil},
			{int64(2), "User", "user", "普通用户", false, int64(1), int64(10), nil, nil},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		roles, err := svc.GetUserRoles(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roles) != 2 {
			t.Fatalf("期望 2 个角色，实际=%d", len(roles))
		}
		if roles[0].ID != 1 || roles[0].Code != "admin" || !roles[0].IsSystem {
			t.Errorf("第一个角色字段不正确: %+v", roles[0])
		}
		if roles[1].ParentID == nil || *roles[1].ParentID != 1 {
			t.Errorf("期望 ParentID=1，实际=%v", roles[1].ParentID)
		}
		if roles[1].DomainID == nil || *roles[1].DomainID != 10 {
			t.Errorf("期望 DomainID=10，实际=%v", roles[1].DomainID)
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewUserAdminService(db, nil, nil)

		roles, err := svc.GetUserRoles(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(roles) != 0 {
			t.Errorf("期望 0 个角色，实际=%d", len(roles))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.GetUserRoles(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("ParentID 为 0 不设置", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "Admin", "admin", "", true, int64(0), int64(0), nil, nil},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		roles, _ := svc.GetUserRoles(ctx, 1)
		if roles[0].ParentID != nil {
			t.Errorf("期望 ParentID 为 nil（值为 0），实际=%v", roles[0].ParentID)
		}
		if roles[0].DomainID != nil {
			t.Errorf("期望 DomainID 为 nil（值为 0），实际=%v", roles[0].DomainID)
		}
	})
}

// TestUserAdminService_CreateUser 测试创建用户
func TestUserAdminService_CreateUser(t *testing.T) {
	ctx := context.Background()
	rsaUtil := newTestRSAUtil(t)

	t.Run("创建成功-无角色无域", func(t *testing.T) {
		// 写入返回新 ID=42，查询返回新用户
		queryQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(42), "newuser", "New", "138", "new@b.com", true, "hashedpass", nil, nil, nil},
		})
		db := &MockDB{
			WriteResult: database.NewWriteResult(42, 1),
			QueryResult: queryQR,
		}
		svc := NewUserAdminService(db, nil, rsaUtil)

		encPass := encryptTestPassword(t, rsaUtil, "abc123")
		user, err := svc.CreateUser(ctx, "newuser", "New", "138", "new@b.com", encPass, nil, nil, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if user == nil || user.ID != 42 {
			t.Errorf("期望返回 ID=42 的用户，实际=%+v", user)
		}
		// 验证写入参数包含正确的用户名
		if db.LastWriteStmt.Arguments[0] != "newuser" {
			t.Errorf("期望 username=newuser，实际=%v", db.LastWriteStmt.Arguments[0])
		}
	})

	t.Run("创建成功-带角色和域", func(t *testing.T) {
		queryQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(100), "newuser", "New", "138", "new@b.com", true, "hashedpass", nil, nil, nil},
		})
		db := &MockDB{
			WriteResult: database.NewWriteResult(100, 1),
			QueryResult: queryQR,
		}
		svc := NewUserAdminService(db, nil, rsaUtil)

		encPass := encryptTestPassword(t, rsaUtil, "abc123")
		user, err := svc.CreateUser(ctx, "newuser", "New", "138", "new@b.com", encPass, []int64{1, 2}, []int64{10, 20}, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if user == nil || user.ID != 100 {
			t.Errorf("期望返回 ID=100 的用户，实际=%+v", user)
		}
		// 1 次插入用户 + 2 次插入角色 + 2 次插入域 = 5 次写入
		if len(db.WriteStmts) != 5 {
			t.Errorf("期望 5 次写入调用，实际=%d", len(db.WriteStmts))
		}
	})

	t.Run("密码过短", func(t *testing.T) {
		svc := NewUserAdminService(&MockDB{}, nil, rsaUtil)

		encPass := encryptTestPassword(t, rsaUtil, "ab1")
		_, err := svc.CreateUser(ctx, "u", "", "", "", encPass, nil, nil, 1)
		if err != ErrPasswordTooShort {
			t.Errorf("期望 ErrPasswordTooShort，实际=%v", err)
		}
	})

	t.Run("密码无字母", func(t *testing.T) {
		svc := NewUserAdminService(&MockDB{}, nil, rsaUtil)

		encPass := encryptTestPassword(t, rsaUtil, "123456")
		_, err := svc.CreateUser(ctx, "u", "", "", "", encPass, nil, nil, 1)
		if err != ErrPasswordWeak {
			t.Errorf("期望 ErrPasswordWeak，实际=%v", err)
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		svc := NewUserAdminService(&MockDB{WriteError: ErrMockDB}, nil, rsaUtil)

		encPass := encryptTestPassword(t, rsaUtil, "abc123")
		_, err := svc.CreateUser(ctx, "u", "", "", "", encPass, nil, nil, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_UpdateUser 测试更新用户
func TestUserAdminService_UpdateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("更新成功", func(t *testing.T) {
		queryQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice_updated", "Alice", "138", "a@b.com", true, "hashedpass", nil, nil, nil},
		})
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 1),
			QueryResult: queryQR,
		}
		svc := NewUserAdminService(db, nil, nil)

		user, err := svc.UpdateUser(ctx, 1, "alice_updated", "Alice", "138", "a@b.com", true)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if user == nil || user.Username != "alice_updated" {
			t.Errorf("期望返回更新后的用户，实际=%+v", user)
		}
		// 验证写入参数
		if db.LastWriteStmt.Arguments[0] != "alice_updated" {
			t.Errorf("期望 username=alice_updated，实际=%v", db.LastWriteStmt.Arguments[0])
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.UpdateUser(ctx, 1, "u", "", "", "", true)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_DeleteUser 测试删除用户
func TestUserAdminService_DeleteUser(t *testing.T) {
	ctx := context.Background()

	t.Run("删除成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewUserAdminService(db, nil, nil)

		err := svc.DeleteUser(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 删除 user_roles、user_domains、users 共 3 次写入
		if len(db.WriteStmts) != 3 {
			t.Errorf("期望 3 次写入调用，实际=%d", len(db.WriteStmts))
		}
	})

	t.Run("最后一次写入错误", func(t *testing.T) {
		// 使用一个会在第 3 次写入时返回错误的 mock
		// 由于 MockDB 的 WriteError 是全局的，我们用一个自定义包装
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewUserAdminService(db, nil, nil)

		err := svc.DeleteUser(ctx, 1)
		// 删除 user_roles 时忽略错误，删除 user_domains 时忽略错误，删除 users 时返回错误
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_UpdateLastLogin 测试更新最后登录时间
func TestUserAdminService_UpdateLastLogin(t *testing.T) {
	ctx := context.Background()

	t.Run("更新成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewUserAdminService(db, nil, nil)

		err := svc.UpdateLastLogin(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.LastWriteStmt.Arguments[1].(int64) != 1 {
			t.Errorf("期望 userID=1，实际=%v", db.LastWriteStmt.Arguments[1])
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		svc := NewUserAdminService(&MockDB{WriteError: ErrMockDB}, nil, nil)

		err := svc.UpdateLastLogin(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_AssignUserRoles 测试分配用户角色
func TestUserAdminService_AssignUserRoles(t *testing.T) {
	ctx := context.Background()

	t.Run("分配成功-多个角色", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewUserAdminService(db, nil, nil)

		err := svc.AssignUserRoles(ctx, 10, []int64{1, 2, 3}, nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 1 次删除 + 3 次插入 = 4 次写入
		if len(db.WriteStmts) != 4 {
			t.Errorf("期望 4 次写入调用，实际=%d", len(db.WriteStmts))
		}
	})

	t.Run("分配成功-空角色列表", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewUserAdminService(db, nil, nil)

		err := svc.AssignUserRoles(ctx, 10, nil, nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 仅 1 次删除
		if len(db.WriteStmts) != 1 {
			t.Errorf("期望 1 次写入调用，实际=%d", len(db.WriteStmts))
		}
	})

	t.Run("删除时写入错误", func(t *testing.T) {
		svc := NewUserAdminService(&MockDB{WriteError: ErrMockDB}, nil, nil)

		err := svc.AssignUserRoles(ctx, 10, []int64{1}, nil)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_AssignUserRolesWithDomain 测试带域分配用户角色
func TestUserAdminService_AssignUserRolesWithDomain(t *testing.T) {
	ctx := context.Background()

	t.Run("分配成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewUserAdminService(db, nil, nil)
		domainID := int64(5)

		err := svc.AssignUserRolesWithDomain(ctx, 10, []int64{1, 2}, &domainID)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 1 次删除 + 2 次插入 = 3 次写入
		if len(db.WriteStmts) != 3 {
			t.Errorf("期望 3 次写入调用，实际=%d", len(db.WriteStmts))
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		svc := NewUserAdminService(&MockDB{WriteError: ErrMockDB}, nil, nil)
		domainID := int64(5)

		err := svc.AssignUserRolesWithDomain(ctx, 10, []int64{1}, &domainID)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_AssignUserDomains 测试分配用户域
func TestUserAdminService_AssignUserDomains(t *testing.T) {
	ctx := context.Background()

	t.Run("分配成功-多个域", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewUserAdminService(db, nil, nil)

		err := svc.AssignUserDomains(ctx, 10, []int64{1, 2, 3})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 1 次删除 + 3 次插入 = 4 次写入
		if len(db.WriteStmts) != 4 {
			t.Errorf("期望 4 次写入调用，实际=%d", len(db.WriteStmts))
		}
	})

	t.Run("分配成功-空域列表（仅删除）", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewUserAdminService(db, nil, nil)

		err := svc.AssignUserDomains(ctx, 10, nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(db.WriteStmts) != 1 {
			t.Errorf("期望 1 次写入调用，实际=%d", len(db.WriteStmts))
		}
	})

	t.Run("删除时写入错误", func(t *testing.T) {
		svc := NewUserAdminService(&MockDB{WriteError: ErrMockDB}, nil, nil)

		err := svc.AssignUserDomains(ctx, 10, []int64{1})
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_UpdateUserWithDomainCheck 测试带域检查的更新用户
func TestUserAdminService_UpdateUserWithDomainCheck(t *testing.T) {
	ctx := context.Background()

	t.Run("用户不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.UpdateUserWithDomainCheck(ctx, 1, 999, "u", "", "", "", true)
		if err != ErrUserNotFound {
			t.Errorf("期望 ErrUserNotFound，实际=%v", err)
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.UpdateUserWithDomainCheck(ctx, 1, 1, "u", "", "", "", true)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("用户存在-更新成功", func(t *testing.T) {
		// GetUserByID 返回用户，UpdateUser 写入后再次 GetUserByID 返回更新后的用户
		// 由于 MockDB 每次查询返回相同结果，这里返回存在的用户
		queryQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "Alice", "138", "a@b.com", true, "hashedpass", nil, nil, nil},
		})
		db := &MockDB{
			QueryResult: queryQR,
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewUserAdminService(db, nil, nil)

		user, err := svc.UpdateUserWithDomainCheck(ctx, 1, 1, "alice", "Alice", "138", "a@b.com", true)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if user == nil {
			t.Fatal("期望返回非 nil 用户")
		}
	})
}

// TestUserAdminService_GetCurrentUserInfo 测试获取当前用户信息
func TestUserAdminService_GetCurrentUserInfo(t *testing.T) {
	ctx := context.Background()

	t.Run("用户存在", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "Alice", "138", "a@b.com", true, "hashedpass", nil, nil, nil},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		user, err := svc.GetCurrentUserInfo(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if user == nil || user.ID != 1 {
			t.Errorf("期望返回 ID=1 的用户，实际=%+v", user)
		}
	})

	t.Run("用户不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewUserAdminService(db, nil, nil)

		user, err := svc.GetCurrentUserInfo(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if user != nil {
			t.Error("期望 user 为 nil")
		}
	})
}

// TestUserAdminService_UpdateCurrentUser 测试当前用户更新自己的资料
func TestUserAdminService_UpdateCurrentUser(t *testing.T) {
	ctx := context.Background()

	t.Run("用户不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.UpdateCurrentUser(ctx, 999, "New", "138", "new@b.com")
		if err != ErrUserNotFound {
			t.Errorf("期望 ErrUserNotFound，实际=%v", err)
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.UpdateCurrentUser(ctx, 1, "New", "138", "new@b.com")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("更新成功", func(t *testing.T) {
		queryQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "Alice", "138", "a@b.com", true, "hashedpass", nil, nil, nil},
		})
		db := &MockDB{
			QueryResult: queryQR,
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewUserAdminService(db, nil, nil)

		user, err := svc.UpdateCurrentUser(ctx, 1, "NewName", "139", "new@b.com")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if user == nil {
			t.Fatal("期望返回非 nil 用户")
		}
	})
}

// TestUserAdminService_ResetPassword 测试重置密码
func TestUserAdminService_ResetPassword(t *testing.T) {
	ctx := context.Background()
	rsaUtil := newTestRSAUtil(t)

	t.Run("重置成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewUserAdminService(db, nil, rsaUtil)

		encPass := encryptTestPassword(t, rsaUtil, "newpass123")
		err := svc.ResetPassword(ctx, 1, encPass)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})

	t.Run("密码过短", func(t *testing.T) {
		svc := NewUserAdminService(&MockDB{}, nil, rsaUtil)

		encPass := encryptTestPassword(t, rsaUtil, "ab1")
		err := svc.ResetPassword(ctx, 1, encPass)
		if err != ErrPasswordTooShort {
			t.Errorf("期望 ErrPasswordTooShort，实际=%v", err)
		}
	})

	t.Run("密码无字母", func(t *testing.T) {
		svc := NewUserAdminService(&MockDB{}, nil, rsaUtil)

		encPass := encryptTestPassword(t, rsaUtil, "123456")
		err := svc.ResetPassword(ctx, 1, encPass)
		if err != ErrPasswordWeak {
			t.Errorf("期望 ErrPasswordWeak，实际=%v", err)
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		svc := NewUserAdminService(&MockDB{WriteError: ErrMockDB}, nil, rsaUtil)

		encPass := encryptTestPassword(t, rsaUtil, "newpass123")
		err := svc.ResetPassword(ctx, 1, encPass)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_ChangePassword 测试修改密码
func TestUserAdminService_ChangePassword(t *testing.T) {
	ctx := context.Background()
	rsaUtil := newTestRSAUtil(t)

	t.Run("用户不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewUserAdminService(db, nil, rsaUtil)

		encOld := encryptTestPassword(t, rsaUtil, "oldpass123")
		encNew := encryptTestPassword(t, rsaUtil, "newpass123")
		err := svc.ChangePassword(ctx, 999, encOld, encNew)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewUserAdminService(db, nil, rsaUtil)

		encOld := encryptTestPassword(t, rsaUtil, "oldpass123")
		encNew := encryptTestPassword(t, rsaUtil, "newpass123")
		err := svc.ChangePassword(ctx, 1, encOld, encNew)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("旧密码不匹配", func(t *testing.T) {
		// 用户已存在，密码哈希是 "realhashedpass"，旧密码解密后是 "wrongpass123"
		queryQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "Alice", "138", "a@b.com", true, "$2a$10$somehash", nil, nil, nil},
		})
		db := &MockDB{QueryResult: queryQR}
		svc := NewUserAdminService(db, nil, rsaUtil)

		encOld := encryptTestPassword(t, rsaUtil, "wrongpass123")
		encNew := encryptTestPassword(t, rsaUtil, "newpass123")
		err := svc.ChangePassword(ctx, 1, encOld, encNew)
		if err == nil {
			t.Fatal("期望返回错误（旧密码不匹配）")
		}
	})
}

// TestUserAdminService_ResetUserPasswordWithDomainCheck 测试带域检查的重置密码
func TestUserAdminService_ResetUserPasswordWithDomainCheck(t *testing.T) {
	ctx := context.Background()
	rsaUtil := newTestRSAUtil(t)

	t.Run("用户不存在", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewUserAdminService(db, nil, rsaUtil)

		encPass := encryptTestPassword(t, rsaUtil, "newpass123")
		err := svc.ResetUserPasswordWithDomainCheck(ctx, 1, 999, encPass)
		if err != ErrUserNotFound {
			t.Errorf("期望 ErrUserNotFound，实际=%v", err)
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewUserAdminService(db, nil, rsaUtil)

		encPass := encryptTestPassword(t, rsaUtil, "newpass123")
		err := svc.ResetUserPasswordWithDomainCheck(ctx, 1, 1, encPass)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("重置成功", func(t *testing.T) {
		queryQR := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "Alice", "138", "a@b.com", true, "hashedpass", nil, nil, nil},
		})
		db := &MockDB{
			QueryResult: queryQR,
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewUserAdminService(db, nil, rsaUtil)

		encPass := encryptTestPassword(t, rsaUtil, "newpass123")
		err := svc.ResetUserPasswordWithDomainCheck(ctx, 1, 1, encPass)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})
}

// TestUserAdminService_BatchGetUserRoleIDs 测试批量获取用户角色 ID
func TestUserAdminService_BatchGetUserRoleIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("返回多用户多角色", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), int64(10)},
			{int64(1), int64(20)},
			{int64(2), int64(10)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		result, err := svc.BatchGetUserRoleIDs(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(result[1]) != 2 {
			t.Errorf("期望用户 1 有 2 个角色，实际=%d", len(result[1]))
		}
		if len(result[2]) != 1 {
			t.Errorf("期望用户 2 有 1 个角色，实际=%d", len(result[2]))
		}
	})

	t.Run("空结果", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := NewUserAdminService(db, nil, nil)

		result, err := svc.BatchGetUserRoleIDs(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("期望空 map，实际=%d", len(result))
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.BatchGetUserRoleIDs(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_BatchGetUserDomainIDs 测试批量获取用户域 ID
func TestUserAdminService_BatchGetUserDomainIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("返回多用户多域", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), int64(100)},
			{int64(2), int64(200)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		result, err := svc.BatchGetUserDomainIDs(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("期望 2 个用户，实际=%d", len(result))
		}
		if result[1][0] != 100 {
			t.Errorf("期望用户 1 的域 ID=100，实际=%d", result[1][0])
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.BatchGetUserDomainIDs(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_BatchGetUserRoleCodes 测试批量获取用户角色代码
func TestUserAdminService_BatchGetUserRoleCodes(t *testing.T) {
	ctx := context.Background()

	t.Run("返回多用户多角色代码", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "admin"},
			{int64(1), "user"},
			{int64(2), "user"},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		result, err := svc.BatchGetUserRoleCodes(ctx)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(result[1]) != 2 {
			t.Errorf("期望用户 1 有 2 个角色代码，实际=%d", len(result[1]))
		}
		if result[2][0] != "user" {
			t.Errorf("期望用户 2 的角色代码=user，实际=%s", result[2][0])
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewUserAdminService(db, nil, nil)

		_, err := svc.BatchGetUserRoleCodes(ctx)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_AreRolesSystemOnly 测试检查角色是否为系统角色
func TestUserAdminService_AreRolesSystemOnly(t *testing.T) {
	ctx := context.Background()

	t.Run("空列表返回 false", func(t *testing.T) {
		svc := NewUserAdminService(&MockDB{}, nil, nil)

		result, err := svc.AreRolesSystemOnly(ctx, nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if result {
			t.Error("期望 false")
		}
	})

	t.Run("包含 system_admin 角色", func(t *testing.T) {
		// GetRoleCodeByID 返回 "system_admin"
		qr := database.NewQueryResultWithRows([][]interface{}{
			{"system_admin"},
		})
		db := &MockDB{QueryResult: qr}
		permSvc := NewPermissionService(db, nil)
		svc := NewUserAdminService(db, permSvc, nil)

		result, err := svc.AreRolesSystemOnly(ctx, []int64{1})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !result {
			t.Error("期望 true")
		}
	})

	t.Run("不包含 system_admin 角色", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{"user"},
		})
		db := &MockDB{QueryResult: qr}
		permSvc := NewPermissionService(db, nil)
		svc := NewUserAdminService(db, permSvc, nil)

		result, err := svc.AreRolesSystemOnly(ctx, []int64{1})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if result {
			t.Error("期望 false")
		}
	})
}

// TestUserAdminService_AreDomainsAccessibleByUser 测试检查用户是否可访问指定域
func TestUserAdminService_AreDomainsAccessibleByUser(t *testing.T) {
	ctx := context.Background()

	t.Run("空域列表返回 true", func(t *testing.T) {
		svc := NewUserAdminService(&MockDB{}, nil, nil)

		result, err := svc.AreDomainsAccessibleByUser(ctx, 1, nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !result {
			t.Error("期望 true")
		}
	})

	t.Run("用户可访问所有域", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1)},
			{int64(2)},
		})
		db := &MockDB{QueryResult: qr}
		permSvc := NewPermissionService(db, nil)
		svc := NewUserAdminService(db, permSvc, nil)

		result, err := svc.AreDomainsAccessibleByUser(ctx, 1, []int64{1, 2})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !result {
			t.Error("期望 true")
		}
	})

	t.Run("用户无法访问某些域", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1)},
		})
		db := &MockDB{QueryResult: qr}
		permSvc := NewPermissionService(db, nil)
		svc := NewUserAdminService(db, permSvc, nil)

		result, err := svc.AreDomainsAccessibleByUser(ctx, 1, []int64{1, 2})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if result {
			t.Error("期望 false（用户无法访问域 2）")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		permSvc := NewPermissionService(db, nil)
		svc := NewUserAdminService(db, permSvc, nil)

		_, err := svc.AreDomainsAccessibleByUser(ctx, 1, []int64{1})
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestUserAdminService_EnrichUsersWithRolesAndDomains 测试为用户列表补充角色和域信息
func TestUserAdminService_EnrichUsersWithRolesAndDomains(t *testing.T) {
	ctx := context.Background()

	t.Run("空用户列表直接返回", func(t *testing.T) {
		db := &MockDB{}
		svc := NewUserAdminService(db, nil, nil)

		// 不应 panic
		svc.EnrichUsersWithRolesAndDomains(ctx, nil)
		if len(db.QueryStmts) != 0 {
			t.Errorf("期望无查询调用，实际=%d", len(db.QueryStmts))
		}
	})

	t.Run("为用户补充角色和域信息", func(t *testing.T) {
		// 由于 MockDB 每次查询返回相同结果，BatchGetUserRoleIDs/BatchGetUserDomainIDs/BatchGetUserRoleCodes/getDomainNameMap
		// 都会返回相同结构的数据。这里用包含两列的数据模拟。
		// BatchGetUserRoleIDs 期望 (user_id, role_id)
		// BatchGetUserDomainIDs 期望 (user_id, domain_id)
		// BatchGetUserRoleCodes 期望 (user_id, role_code)
		// getDomainNameMap 期望 (id, name)
		// 由于所有查询共享一个 QueryResult，我们用一个兼容所有场景的两列结构。
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), int64(10)},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewUserAdminService(db, nil, nil)

		users := []*model.User{
			{ID: 1, Username: "alice"},
		}
		svc.EnrichUsersWithRolesAndDomains(ctx, users)

		// 应该执行了 4 次查询
		if len(db.QueryStmts) != 4 {
			t.Errorf("期望 4 次查询调用，实际=%d", len(db.QueryStmts))
		}
	})
}
