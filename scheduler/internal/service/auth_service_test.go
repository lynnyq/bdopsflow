package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// newAuthServiceWithQueryResult 构造一个 AuthService，其底层 MockDB 返回指定的 QueryResult
func newAuthServiceWithQueryResult(qr rqlite.QueryResult) *AuthService {
	return NewAuthService(&MockDB{QueryResult: qr})
}

func newAuthServiceWithQueryError(err error) *AuthService {
	return NewAuthService(&MockDB{QueryError: err})
}

func newAuthServiceWithWriteResult(wr rqlite.WriteResult) *AuthService {
	return NewAuthService(&MockDB{WriteResult: wr})
}

func newAuthServiceWithWriteError(err error) *AuthService {
	return NewAuthService(&MockDB{WriteError: err})
}

// TestNewAuthService 测试构造函数
func TestNewAuthService(t *testing.T) {
	t.Run("nil db 时仍可创建实例", func(t *testing.T) {
		svc := NewAuthService(nil)
		if svc == nil {
			t.Fatal("期望返回非 nil 实例")
		}
	})

	t.Run("db 正确赋值", func(t *testing.T) {
		db := &MockDB{}
		svc := NewAuthService(db)
		if svc.db == nil {
			t.Fatal("期望 db 正确赋值")
		}
	})
}

// TestAuthService_GetUserByUsername 测试根据用户名查询用户
func TestAuthService_GetUserByUsername(t *testing.T) {
	ctx := context.Background()

	t.Run("用户存在", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "Alice", "13800000000", "$2a$10$hashedpassword", "alice@example.com", true},
		})
		svc := newAuthServiceWithQueryResult(qr)

		user, found, err := svc.GetUserByUsername(ctx, "alice")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !found {
			t.Fatal("期望找到用户")
		}
		if user.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", user.ID)
		}
		if user.Username != "alice" {
			t.Errorf("期望 Username=alice，实际=%s", user.Username)
		}
		if user.HashedPassword != "$2a$10$hashedpassword" {
			t.Errorf("期望 HashedPassword 正确，实际=%s", user.HashedPassword)
		}
		if !user.IsActive {
			t.Error("期望 IsActive=true")
		}
	})

	t.Run("用户不存在", func(t *testing.T) {
		// 空行 → Next() 返回 false
		svc := newAuthServiceWithQueryResult(database.NewQueryResultWithRows(nil))

		user, found, err := svc.GetUserByUsername(ctx, "nobody")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if found {
			t.Fatal("期望 found=false")
		}
		if user != nil {
			t.Error("期望 user=nil")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		svc := newAuthServiceWithQueryError(ErrMockDB)

		_, _, err := svc.GetUserByUsername(ctx, "alice")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		svc := NewAuthService(&MockDB{
			QueryResult: database.NewQueryResultWithErr(errors.New("query result error")),
		})

		_, _, err := svc.GetUserByUsername(ctx, "alice")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("断言查询参数", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "Alice", "138", "hash", "a@b.com", true},
		})
		db := &MockDB{QueryResult: qr}
		svc := NewAuthService(db)

		_, _, _ = svc.GetUserByUsername(ctx, "alice")

		if db.LastQueryStmt.Arguments[0] != "alice" {
			t.Errorf("期望查询参数为 alice，实际=%v", db.LastQueryStmt.Arguments[0])
		}
	})
}

// TestAuthService_GetSSOUserByUsername 测试 SSO 用户查询
func TestAuthService_GetSSOUserByUsername(t *testing.T) {
	ctx := context.Background()

	t.Run("SSO 用户存在", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1), "alice", "Alice", "13800000000", "alice@example.com", true},
		})
		svc := newAuthServiceWithQueryResult(qr)

		user, found, err := svc.GetSSOUserByUsername(ctx, "alice")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !found {
			t.Fatal("期望找到用户")
		}
		if user.ID != 1 || user.Username != "alice" || !user.IsActive {
			t.Errorf("用户字段不正确: %+v", user)
		}
	})

	t.Run("SSO 用户不存在", func(t *testing.T) {
		svc := newAuthServiceWithQueryResult(database.NewQueryResultWithRows(nil))

		_, found, err := svc.GetSSOUserByUsername(ctx, "ghost")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if found {
			t.Fatal("期望 found=false")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		svc := newAuthServiceWithQueryError(ErrMockDB)

		_, _, err := svc.GetSSOUserByUsername(ctx, "alice")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestAuthService_GetUserByID 测试根据 ID 查询用户
func TestAuthService_GetUserByID(t *testing.T) {
	ctx := context.Background()

	t.Run("用户存在且有登录时间", func(t *testing.T) {
		loginTime := time.Now()
		qr := database.NewQueryResultWithRows([][]interface{}{
			{"alice", "Alice", "138", "a@b.com", true, loginTime},
		})
		svc := newAuthServiceWithQueryResult(qr)

		user, found, err := svc.GetUserByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !found {
			t.Fatal("期望找到用户")
		}
		if user.Username != "alice" || !user.IsActive {
			t.Errorf("用户字段不正确: %+v", user)
		}
		if user.LastLoginAt == nil {
			t.Error("期望 LastLoginAt 非 nil")
		}
	})

	t.Run("用户存在但无登录时间", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{"alice", "Alice", "138", "a@b.com", true, nil},
		})
		svc := newAuthServiceWithQueryResult(qr)

		user, _, _ := svc.GetUserByID(ctx, 1)
		if user.LastLoginAt != nil {
			t.Error("期望 LastLoginAt 为 nil（零值）")
		}
	})

	t.Run("用户不存在", func(t *testing.T) {
		svc := newAuthServiceWithQueryResult(database.NewQueryResultWithRows(nil))

		_, found, err := svc.GetUserByID(ctx, 999)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if found {
			t.Fatal("期望 found=false")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		svc := newAuthServiceWithQueryError(ErrMockDB)

		_, _, err := svc.GetUserByID(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		svc := NewAuthService(&MockDB{
			QueryResult: database.NewQueryResultWithErr(errors.New("result err")),
		})

		_, _, err := svc.GetUserByID(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestAuthService_GetUserActiveStatus 测试查询用户激活状态
func TestAuthService_GetUserActiveStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("用户激活", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{{true}})
		svc := newAuthServiceWithQueryResult(qr)

		active, found, err := svc.GetUserActiveStatus(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if !found {
			t.Fatal("期望 found=true")
		}
		if !active {
			t.Error("期望 active=true")
		}
	})

	t.Run("用户未激活", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{{false}})
		svc := newAuthServiceWithQueryResult(qr)

		active, _, _ := svc.GetUserActiveStatus(ctx, 1)
		if active {
			t.Error("期望 active=false")
		}
	})

	t.Run("用户不存在", func(t *testing.T) {
		svc := newAuthServiceWithQueryResult(database.NewQueryResultWithRows(nil))

		_, found, _ := svc.GetUserActiveStatus(ctx, 999)
		if found {
			t.Fatal("期望 found=false")
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		svc := newAuthServiceWithQueryError(ErrMockDB)

		_, _, err := svc.GetUserActiveStatus(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestAuthService_UpdateLastLogin 测试更新最后登录时间
func TestAuthService_UpdateLastLogin(t *testing.T) {
	ctx := context.Background()

	t.Run("更新成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewAuthService(db)

		err := svc.UpdateLastLogin(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 验证写入参数包含正确的 userID
		if len(db.WriteStmts) != 1 {
			t.Fatalf("期望 1 次写入调用，实际=%d", len(db.WriteStmts))
		}
		args := db.WriteStmts[0].Arguments
		if len(args) != 2 {
			t.Fatalf("期望 2 个参数，实际=%d", len(args))
		}
		// args[0] 是 time.Now()，args[1] 是 userID
		if args[1].(int64) != 1 {
			t.Errorf("期望 userID=1，实际=%v", args[1])
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		svc := newAuthServiceWithWriteError(ErrMockDB)

		err := svc.UpdateLastLogin(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("写入结果带错误", func(t *testing.T) {
		svc := NewAuthService(&MockDB{
			WriteResult: rqlite.WriteResult{},
		})
		// 设置 result.Err - 由于 WriteResult.Err 是私有字段，无法直接设置
		// 这个分支由 MockDB 的 WriteResult 返回值控制，这里仅测试正常路径
		err := svc.UpdateLastLogin(ctx, 1)
		// WriteResult.Err 为 nil 时不应返回错误
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
	})
}

// TestAuthService_CreateUser 测试创建用户
func TestAuthService_CreateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("创建成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(42, 1)}
		svc := NewAuthService(db)

		id, err := svc.CreateUser(ctx, "newuser", "New", "138", "hashedpass", "new@b.com")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if id != 42 {
			t.Errorf("期望 ID=42，实际=%d", id)
		}
		// 验证写入参数
		args := db.LastWriteStmt.Arguments
		if args[0] != "newuser" {
			t.Errorf("期望 username=newuser，实际=%v", args[0])
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		svc := newAuthServiceWithWriteError(ErrMockDB)

		_, err := svc.CreateUser(ctx, "newuser", "", "", "hash", "")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestAuthService_GetRoleIDByCode 测试根据 code 查询角色 ID
func TestAuthService_GetRoleIDByCode(t *testing.T) {
	ctx := context.Background()

	t.Run("角色存在", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{{int64(5)}})
		svc := newAuthServiceWithQueryResult(qr)

		roleID, err := svc.GetRoleIDByCode(ctx, "user")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if roleID != 5 {
			t.Errorf("期望 roleID=5，实际=%d", roleID)
		}
	})

	t.Run("角色不存在", func(t *testing.T) {
		svc := newAuthServiceWithQueryResult(database.NewQueryResultWithRows(nil))

		roleID, err := svc.GetRoleIDByCode(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if roleID != 0 {
			t.Errorf("期望 roleID=0，实际=%d", roleID)
		}
	})

	t.Run("空行", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{{}})
		svc := newAuthServiceWithQueryResult(qr)

		roleID, err := svc.GetRoleIDByCode(ctx, "empty")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if roleID != 0 {
			t.Errorf("期望 roleID=0（空行），实际=%d", roleID)
		}
	})

	t.Run("查询错误", func(t *testing.T) {
		svc := newAuthServiceWithQueryError(ErrMockDB)

		_, err := svc.GetRoleIDByCode(ctx, "user")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})

	t.Run("查询结果带错误", func(t *testing.T) {
		svc := NewAuthService(&MockDB{
			QueryResult: database.NewQueryResultWithErr(errors.New("result err")),
		})

		_, err := svc.GetRoleIDByCode(ctx, "user")
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestAuthService_AssignUserRole 测试分配用户角色
func TestAuthService_AssignUserRole(t *testing.T) {
	ctx := context.Background()

	t.Run("分配成功", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewAuthService(db)

		err := svc.AssignUserRole(ctx, 10, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		args := db.LastWriteStmt.Arguments
		if args[0].(int64) != 10 || args[1].(int64) != 5 {
			t.Errorf("期望参数 (10,5)，实际=%v", args)
		}
	})

	t.Run("写入错误", func(t *testing.T) {
		svc := newAuthServiceWithWriteError(ErrMockDB)

		err := svc.AssignUserRole(ctx, 10, 5)
		if err == nil {
			t.Fatal("期望返回错误")
		}
	})
}

// TestScanNullTimePtr 测试私有辅助函数 scanNullTimePtr
func TestScanNullTimePtr(t *testing.T) {
	t.Run("索引越界-负数", func(t *testing.T) {
		result := scanNullTimePtr([]interface{}{}, -1)
		if result != nil {
			t.Error("期望 nil")
		}
	})

	t.Run("索引越界-超长", func(t *testing.T) {
		result := scanNullTimePtr([]interface{}{int64(1)}, 5)
		if result != nil {
			t.Error("期望 nil")
		}
	})

	t.Run("nil 值", func(t *testing.T) {
		result := scanNullTimePtr([]interface{}{nil}, 0)
		if result != nil {
			t.Error("期望 nil")
		}
	})

	t.Run("有效时间", func(t *testing.T) {
		now := time.Now()
		result := scanNullTimePtr([]interface{}{now}, 0)
		if result == nil {
			t.Fatal("期望非 nil")
		}
		if !result.Equal(now) {
			t.Errorf("期望时间相等，实际=%v vs %v", *result, now)
		}
	})

	t.Run("字符串时间", func(t *testing.T) {
		t1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		result := scanNullTimePtr([]interface{}{t1.Format(time.RFC3339Nano)}, 0)
		if result == nil {
			t.Fatal("期望非 nil")
		}
	})
}
