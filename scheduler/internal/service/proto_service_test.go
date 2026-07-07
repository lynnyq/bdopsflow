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

// protoFileRow 构造一行 proto_file 查询结果（9 列，GetByID/ListByUser 数据查询）
// 列顺序：0=id 1=name 2=content 3=file_hash 4=parsed_result 5=dependencies
//
//	6=created_by 7=created_at 8=updated_at
func protoFileRow(id int64, name, content, hash, parsed, deps string, createdBy int64) []interface{} {
	return []interface{}{id, name, content, hash, parsed, deps, createdBy, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"}
}

// protoFileRowWithName 构造一行带 created_by_name 的查询结果（10 列，ListByUser）
// 列顺序与 ListByUser SQL 一致：0=id 1=name 2=content 3=file_hash 4=parsed_result
// 5=dependencies 6=created_by 7=created_by_name 8=created_at 9=updated_at
func protoFileRowWithName(id int64, name, content, hash, parsed, deps string, createdBy int64, createdByName string) []interface{} {
	return []interface{}{id, name, content, hash, parsed, deps, createdBy, createdByName, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"}
}

// protoCountRow 构造一行 COUNT 查询结果（1 列）
func protoCountRow(count int64) []interface{} {
	return []interface{}{count}
}

func TestNewProtoService(t *testing.T) {
	t.Run("构造函数正常赋值", func(t *testing.T) {
		db := &MockDB{}
		svc := NewProtoService(db)
		if svc == nil {
			t.Fatal("期望返回非 nil 实例")
		}
		if svc.db == nil {
			t.Error("期望 db 正确赋值")
		}
	})

	t.Run("nil db 也可构造", func(t *testing.T) {
		svc := NewProtoService(nil)
		if svc == nil {
			t.Fatal("期望返回非 nil 实例")
		}
	})
}

func TestProtoService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("创建成功带默认dependencies", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(1, 1),
		}
		svc := NewProtoService(db)
		pf := &model.ProtoFile{
			Name:      "test.proto",
			Content:   "syntax = \"proto3\";",
			CreatedBy: 100,
		}
		created, err := svc.Create(ctx, pf)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if created.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", created.ID)
		}
		if created.FileHash == "" {
			t.Error("期望 FileHash 已设置")
		}
		if created.Dependencies != "[]" {
			t.Errorf("期望 Dependencies 默认为 []，实际=%s", created.Dependencies)
		}
		if created.CreatedAt.IsZero() {
			t.Error("期望 CreatedAt 已设置")
		}
		// 验证写入参数中 file_hash 已计算
		writtenHash, ok := db.LastWriteStmt.Arguments[2].(string)
		if !ok {
			t.Fatal("期望第3个参数为 string")
		}
		if writtenHash == "" {
			t.Error("期望写入的 file_hash 非空")
		}
	})

	t.Run("创建成功带自定义dependencies", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(1, 1),
		}
		svc := NewProtoService(db)
		pf := &model.ProtoFile{
			Name:         "test.proto",
			Content:      "syntax = \"proto3\";",
			Dependencies: `["dep.proto"]`,
			CreatedBy:    100,
		}
		created, err := svc.Create(ctx, pf)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if created.Dependencies != `["dep.proto"]` {
			t.Errorf("期望 Dependencies 保留自定义值，实际=%s", created.Dependencies)
		}
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewProtoService(db)
		_, err := svc.Create(ctx, &model.ProtoFile{Name: "test.proto"})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewProtoService(db)
		_, err := svc.Create(ctx, &model.ProtoFile{Name: "test.proto"})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

func TestProtoService_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("更新成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewProtoService(db)
		pf := &model.ProtoFile{
			Name:         "updated.proto",
			Content:      "syntax = \"proto3\";",
			ParsedResult: "{}",
			Dependencies: "[]",
		}
		err := svc.Update(ctx, 5, pf)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if pf.FileHash == "" {
			t.Error("期望 FileHash 已更新")
		}
		// 验证 id 是最后一个参数
		if db.LastWriteStmt.Arguments[6] != int64(5) {
			t.Errorf("期望第7个参数为 id=5，实际=%v", db.LastWriteStmt.Arguments[6])
		}
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewProtoService(db)
		err := svc.Update(ctx, 5, &model.ProtoFile{Name: "x"})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewProtoService(db)
		err := svc.Update(ctx, 5, &model.ProtoFile{Name: "x"})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("RowsAffected为0返回not found", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 0),
		}
		svc := NewProtoService(db)
		err := svc.Update(ctx, 999, &model.ProtoFile{Name: "x"})
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("期望 not found 错误，实际: %v", err)
		}
	})
}

func TestProtoService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("删除成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewProtoService(db)
		err := svc.Delete(ctx, 5)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.LastWriteStmt.Arguments[0] != int64(5) {
			t.Errorf("期望第1个参数为 id=5，实际=%v", db.LastWriteStmt.Arguments[0])
		}
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewProtoService(db)
		err := svc.Delete(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewProtoService(db)
		err := svc.Delete(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("RowsAffected为0返回not found", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 0),
		}
		svc := NewProtoService(db)
		err := svc.Delete(ctx, 999)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("期望 not found 错误，实际: %v", err)
		}
	})
}

func TestProtoService_GetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("找到记录", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				protoFileRow(1, "test.proto", "content", "hash123", "{}", "[]", 100),
			}),
		}
		svc := NewProtoService(db)
		pf, err := svc.GetByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if pf.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", pf.ID)
		}
		if pf.Name != "test.proto" {
			t.Errorf("期望 Name=test.proto，实际=%s", pf.Name)
		}
		if pf.Content != "content" {
			t.Errorf("期望 Content=content，实际=%s", pf.Content)
		}
		if pf.FileHash != "hash123" {
			t.Errorf("期望 FileHash=hash123，实际=%s", pf.FileHash)
		}
		if pf.Dependencies != "[]" {
			t.Errorf("期望 Dependencies=[]，实际=%s", pf.Dependencies)
		}
		if pf.CreatedBy != 100 {
			t.Errorf("期望 CreatedBy=100，实际=%d", pf.CreatedBy)
		}
	})

	t.Run("记录不存在返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewProtoService(db)
		_, err := svc.GetByID(ctx, 999)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("期望 not found 错误，实际: %v", err)
		}
	})

	t.Run("查询失败返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewProtoService(db)
		_, err := svc.GetByID(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithErr(ErrMockDB),
		}
		svc := NewProtoService(db)
		_, err := svc.GetByID(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

func TestProtoService_ListByUser(t *testing.T) {
	ctx := context.Background()

	t.Run("管理员查询成功", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{protoCountRow(2)}),
				database.NewQueryResultWithRows([][]interface{}{
					protoFileRowWithName(1, "a.proto", "content1", "h1", "", "[]", 100, "Alice"),
					protoFileRowWithName(2, "b.proto", "content2", "h2", "", "[]", 200, "Bob"),
				}),
			},
		}
		svc := NewProtoService(db)
		files, total, err := svc.ListByUser(ctx, 100, true, 1, 20)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if total != 2 {
			t.Errorf("期望 total=2，实际=%d", total)
		}
		if len(files) != 2 {
			t.Fatalf("期望 2 条记录，实际=%d", len(files))
		}
		if files[0].Name != "a.proto" {
			t.Errorf("期望 Name=a.proto，实际=%s", files[0].Name)
		}
		if files[0].FileHash != "h1" {
			t.Errorf("期望 FileHash=h1，实际=%s", files[0].FileHash)
		}
		if files[0].CreatedBy != 100 {
			t.Errorf("期望 CreatedBy=100，实际=%d", files[0].CreatedBy)
		}
	})

	t.Run("普通用户带created_by条件", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{protoCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewProtoService(db)
		_, _, err := svc.ListByUser(ctx, 100, false, 1, 20)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(db.QueryStmts[0].Arguments) != 1 {
			t.Errorf("期望 1 个参数（userID），实际=%d", len(db.QueryStmts[0].Arguments))
		}
	})

	t.Run("带search按名称模糊匹配", func(t *testing.T) {
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{protoCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewProtoService(db)
		_, _, err := svc.ListByUser(ctx, 100, true, 1, 20, "keyword")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.QueryStmts[0].Arguments[0] != "%keyword%" {
			t.Errorf("期望 %%keyword%%，实际=%v", db.QueryStmts[0].Arguments[0])
		}
	})

	t.Run("count查询失败返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewProtoService(db)
		_, _, err := svc.ListByUser(ctx, 100, true, 1, 20)
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
		svc := NewProtoService(db)
		_, _, err := svc.ListByUser(ctx, 100, true, 1, 20)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

func TestProtoService_ParseProto(t *testing.T) {
	ctx := context.Background()

	t.Run("解析完整proto文件", func(t *testing.T) {
		svc := NewProtoService(nil)
		content := `
syntax = "proto3";
package com.example.api;

message UserRequest {
  string name = 1;
  int32 age = 2;
}

message UserResponse {
  string id = 1;
  string name = 2;
}

service UserService {
  rpc GetUser(UserRequest) returns (UserResponse);
  rpc ListUsers(UserRequest) returns (UserResponse);
}
`
		result, err := svc.ParseProto(ctx, content, nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if result.Package != "com.example.api" {
			t.Errorf("期望 Package=com.example.api，实际=%s", result.Package)
		}
		if len(result.Messages) != 2 {
			t.Fatalf("期望 2 个 message，实际=%d", len(result.Messages))
		}
		if result.Messages[0] != "UserRequest" {
			t.Errorf("期望第1个 message=UserRequest，实际=%s", result.Messages[0])
		}
		if result.Messages[1] != "UserResponse" {
			t.Errorf("期望第2个 message=UserResponse，实际=%s", result.Messages[1])
		}
		if len(result.Services) != 1 {
			t.Fatalf("期望 1 个 service，实际=%d", len(result.Services))
		}
		if result.Services[0].Name != "UserService" {
			t.Errorf("期望 Service=UserService，实际=%s", result.Services[0].Name)
		}
		if len(result.Services[0].Methods) != 2 {
			t.Fatalf("期望 2 个 method，实际=%d", len(result.Services[0].Methods))
		}
		if result.Services[0].Methods[0].Name != "GetUser" {
			t.Errorf("期望第1个 method=GetUser，实际=%s", result.Services[0].Methods[0].Name)
		}
		if result.Services[0].Methods[0].InputType != "UserRequest" {
			t.Errorf("期望 InputType=UserRequest，实际=%s", result.Services[0].Methods[0].InputType)
		}
		if result.Services[0].Methods[0].OutputType != "UserResponse" {
			t.Errorf("期望 OutputType=UserResponse，实际=%s", result.Services[0].Methods[0].OutputType)
		}
	})

	t.Run("空内容返回空结果", func(t *testing.T) {
		svc := NewProtoService(nil)
		result, err := svc.ParseProto(ctx, "", nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if result.Package != "" {
			t.Errorf("期望 Package 为空，实际=%s", result.Package)
		}
		if len(result.Messages) != 0 {
			t.Errorf("期望 0 个 message，实际=%d", len(result.Messages))
		}
		if len(result.Services) != 0 {
			t.Errorf("期望 0 个 service，实际=%d", len(result.Services))
		}
	})

	t.Run("只有package声明", func(t *testing.T) {
		svc := NewProtoService(nil)
		content := `syntax = "proto3"; package my.package;`
		result, err := svc.ParseProto(ctx, content, nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if result.Package != "my.package" {
			t.Errorf("期望 Package=my.package，实际=%s", result.Package)
		}
		if len(result.Messages) != 0 {
			t.Errorf("期望 0 个 message，实际=%d", len(result.Messages))
		}
		if len(result.Services) != 0 {
			t.Errorf("期望 0 个 service，实际=%d", len(result.Services))
		}
	})

	t.Run("service无方法", func(t *testing.T) {
		svc := NewProtoService(nil)
		content := `
syntax = "proto3";
package empty;
service EmptyService {}
`
		result, err := svc.ParseProto(ctx, content, nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(result.Services) != 1 {
			t.Fatalf("期望 1 个 service，实际=%d", len(result.Services))
		}
		if result.Services[0].Name != "EmptyService" {
			t.Errorf("期望 Service=EmptyService，实际=%s", result.Services[0].Name)
		}
		if len(result.Services[0].Methods) != 0 {
			t.Errorf("期望 0 个 method，实际=%d", len(result.Services[0].Methods))
		}
	})
}

func TestProtoService_ParseAndSave(t *testing.T) {
	ctx := context.Background()

	t.Run("解析并保存成功", func(t *testing.T) {
		// GetByID 返回 proto 文件，然后 ParseProto 解析，最后 WriteOneParameterized 保存
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				protoFileRow(1, "test.proto", `syntax="proto3"; package my.api; message Req{} service S{rpc M(Req) returns(Req);}`, "hash", "", "[]", 100),
			}),
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewProtoService(db)
		err := svc.ParseAndSave(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 验证保存的 parsed_result 包含 package 信息
		writtenParsed, ok := db.LastWriteStmt.Arguments[0].(string)
		if !ok {
			t.Fatal("期望第1个参数为 string")
		}
		if !strings.Contains(writtenParsed, "my.api") {
			t.Errorf("期望 parsed_result 包含 'my.api'，实际=%s", writtenParsed)
		}
	})

	t.Run("GetByID失败返回错误", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewProtoService(db)
		err := svc.ParseAndSave(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("GetByID记录不存在返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewProtoService(db)
		err := svc.ParseAndSave(ctx, 999)
		if err == nil {
			t.Fatal("期望返回错误")
		}
		if !strings.Contains(err.Error(), "failed to get proto file") {
			t.Errorf("期望错误包含 'failed to get proto file'，实际: %v", err)
		}
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				protoFileRow(1, "test.proto", `package my.api;`, "hash", "", "[]", 100),
			}),
			WriteError: ErrMockDB,
		}
		svc := NewProtoService(db)
		err := svc.ParseAndSave(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				protoFileRow(1, "test.proto", `package my.api;`, "hash", "", "[]", 100),
			}),
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewProtoService(db)
		err := svc.ParseAndSave(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

func TestComputeFileHash(t *testing.T) {
	t.Run("相同内容返回相同hash", func(t *testing.T) {
		h1 := computeFileHash("hello")
		h2 := computeFileHash("hello")
		if h1 != h2 {
			t.Errorf("相同内容应返回相同 hash，h1=%s, h2=%s", h1, h2)
		}
	})

	t.Run("不同内容返回不同hash", func(t *testing.T) {
		h1 := computeFileHash("hello")
		h2 := computeFileHash("world")
		if h1 == h2 {
			t.Error("不同内容应返回不同 hash")
		}
	})

	t.Run("空内容返回固定hash", func(t *testing.T) {
		// SHA256 of empty string
		expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		h := computeFileHash("")
		if h != expected {
			t.Errorf("空内容 hash 应为 %s，实际=%s", expected, h)
		}
	})

	t.Run("hash长度为64（SHA256十六进制）", func(t *testing.T) {
		h := computeFileHash("test")
		if len(h) != 64 {
			t.Errorf("期望 hash 长度 64，实际=%d", len(h))
		}
	})
}

func TestExtractBlock(t *testing.T) {
	t.Run("正常提取块", func(t *testing.T) {
		content := "service Foo {\n  rpc M() returns (R);\n}"
		// 从第一个 '{' 之后开始
		start := strings.Index(content, "{") + 1
		block := extractBlock(content, start)
		if !strings.Contains(block, "rpc M") {
			t.Errorf("期望 block 包含 'rpc M'，实际=%s", block)
		}
		if !strings.Contains(block, "}") {
			t.Errorf("期望 block 以 } 结尾，实际=%s", block)
		}
	})

	t.Run("嵌套块正确提取", func(t *testing.T) {
		content := "service Foo {\n  rpc M() returns (R) {\n    option x = 1;\n  };\n}"
		start := strings.Index(content, "{") + 1
		block := extractBlock(content, start)
		// 应该提取到最外层的闭合括号
		if !strings.Contains(block, "option x = 1") {
			t.Errorf("期望 block 包含嵌套内容，实际=%s", block)
		}
	})

	t.Run("未闭合块返回剩余内容", func(t *testing.T) {
		content := "service Foo {\n  rpc M() returns (R);"
		start := strings.Index(content, "{") + 1
		block := extractBlock(content, start)
		// 未闭合，应返回从 start 到结尾
		if block != "\n  rpc M() returns (R);" {
			t.Errorf("期望返回剩余内容，实际=%s", block)
		}
	})
}
