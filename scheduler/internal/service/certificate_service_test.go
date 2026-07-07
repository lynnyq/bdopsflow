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

// certRow 构造一行 certificate 查询结果（8 列，GetByID）
// 列顺序：0=id 1=name 2=ca_cert 3=client_cert 4=client_key
//
//	5=created_by 6=created_at 7=updated_at
func certRow(id int64, name, caCert, clientCert, clientKey string, createdBy int64) []interface{} {
	return []interface{}{id, name, caCert, clientCert, clientKey, createdBy, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"}
}

// certSummaryRow 构造一行 certificate summary 查询结果（9 列，ListByUser）
// 列顺序：0=id 1=name 2=ca_cert 3=client_cert 4=client_key
//
//	5=created_by 6=created_by_name 7=created_at 8=updated_at
func certSummaryRow(id int64, name, caCert, clientCert, clientKey string, createdBy int64, createdByName string) []interface{} {
	return []interface{}{id, name, caCert, clientCert, clientKey, createdBy, createdByName, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"}
}

// certCountRow 构造一行 COUNT 查询结果（1 列）
func certCountRow(count int64) []interface{} {
	return []interface{}{count}
}

func TestNewCertificateService(t *testing.T) {
	t.Run("构造函数正常赋值", func(t *testing.T) {
		db := &MockDB{}
		rsaUtil := newTestRSAUtil(t)
		svc := NewCertificateService(db, rsaUtil)
		if svc == nil {
			t.Fatal("期望返回非 nil 实例")
		}
		if svc.db == nil {
			t.Error("期望 db 正确赋值")
		}
		if svc.rsaUtil == nil {
			t.Error("期望 rsaUtil 正确赋值")
		}
	})

	t.Run("nil 参数也可构造", func(t *testing.T) {
		svc := NewCertificateService(nil, nil)
		if svc == nil {
			t.Fatal("期望返回非 nil 实例")
		}
	})
}

func TestCertificateService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("创建成功带ClientKey加密", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			WriteResult: database.NewWriteResult(1, 1),
		}
		svc := NewCertificateService(db, rsaUtil)
		cert := &model.Certificate{
			Name:       "cert-1",
			CaCert:     "ca-content",
			ClientCert: "client-cert-content",
			ClientKey:  "secret-key",
			CreatedBy:  100,
		}
		created, err := svc.Create(ctx, cert)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if created.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", created.ID)
		}
		if created.CreatedAt.IsZero() {
			t.Error("期望 CreatedAt 已设置")
		}
		// 验证写入的 client_key 已被加密（第4个参数）
		writtenKey, ok := db.LastWriteStmt.Arguments[3].(string)
		if !ok {
			t.Fatal("期望第4个参数为 string")
		}
		if writtenKey == "secret-key" {
			t.Error("期望 client_key 已被加密，不应为明文")
		}
		if !strings.Contains(writtenKey, ".") {
			t.Error("期望加密后的 client_key 包含 '.' 分隔符（AES-GCM hybrid 格式）")
		}
	})

	t.Run("无ClientKey不加密", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			WriteResult: database.NewWriteResult(1, 1),
		}
		svc := NewCertificateService(db, rsaUtil)
		cert := &model.Certificate{
			Name:       "cert-2",
			ClientKey:  "",
			CreatedBy:  100,
		}
		_, err := svc.Create(ctx, cert)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 验证写入的 client_key 为空字符串
		writtenKey, ok := db.LastWriteStmt.Arguments[3].(string)
		if !ok {
			t.Fatal("期望第4个参数为 string")
		}
		if writtenKey != "" {
			t.Errorf("期望 client_key 为空，实际=%s", writtenKey)
		}
	})

	t.Run("rsaUtil为nil且有ClientKey时panic", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(1, 1)}
		svc := NewCertificateService(db, nil)
		cert := &model.Certificate{
			Name:      "cert-3",
			ClientKey: "secret-key",
		}
		defer func() {
			if r := recover(); r == nil {
				t.Error("期望 panic")
			}
		}()
		svc.Create(ctx, cert)
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewCertificateService(db, rsaUtil)
		_, err := svc.Create(ctx, &model.Certificate{Name: "cert"})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewCertificateService(db, rsaUtil)
		_, err := svc.Create(ctx, &model.Certificate{Name: "cert"})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}

func TestCertificateService_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("带ClientKey更新成功", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewCertificateService(db, rsaUtil)
		cert := &model.Certificate{
			Name:       "updated",
			CaCert:     "ca",
			ClientCert: "cc",
			ClientKey:  "new-key",
		}
		err := svc.Update(ctx, 5, cert)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 带 client_key 的更新有 6 个参数：name, ca_cert, client_cert, client_key, now, id
		if len(db.LastWriteStmt.Arguments) != 6 {
			t.Errorf("期望 6 个参数，实际=%d", len(db.LastWriteStmt.Arguments))
		}
		// 验证 client_key 已加密
		writtenKey, ok := db.LastWriteStmt.Arguments[3].(string)
		if !ok {
			t.Fatal("期望第4个参数为 string")
		}
		if writtenKey == "new-key" {
			t.Error("期望 client_key 已被加密")
		}
	})

	t.Run("无ClientKey更新成功", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewCertificateService(db, rsaUtil)
		cert := &model.Certificate{
			Name:       "updated",
			CaCert:     "ca",
			ClientCert: "cc",
			ClientKey:  "",
		}
		err := svc.Update(ctx, 5, cert)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 不带 client_key 的更新有 5 个参数：name, ca_cert, client_cert, now, id
		if len(db.LastWriteStmt.Arguments) != 5 {
			t.Errorf("期望 5 个参数，实际=%d", len(db.LastWriteStmt.Arguments))
		}
	})

	t.Run("rsaUtil为nil且有ClientKey时panic", func(t *testing.T) {
		db := &MockDB{WriteResult: database.NewWriteResult(0, 1)}
		svc := NewCertificateService(db, nil)
		defer func() {
			if r := recover(); r == nil {
				t.Error("期望 panic")
			}
		}()
		svc.Update(ctx, 5, &model.Certificate{Name: "x", ClientKey: "k"})
	})

	t.Run("写入失败返回错误", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{WriteError: ErrMockDB}
		svc := NewCertificateService(db, rsaUtil)
		err := svc.Update(ctx, 5, &model.Certificate{Name: "x"})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewCertificateService(db, rsaUtil)
		err := svc.Update(ctx, 5, &model.Certificate{Name: "x"})
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("RowsAffected为0返回not found", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 0),
		}
		svc := NewCertificateService(db, rsaUtil)
		err := svc.Update(ctx, 999, &model.Certificate{Name: "x"})
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("期望 not found 错误，实际: %v", err)
		}
	})
}

func TestCertificateService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("删除成功", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 1),
		}
		svc := NewCertificateService(db, newTestRSAUtil(t))
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
		svc := NewCertificateService(db, newTestRSAUtil(t))
		err := svc.Delete(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("写入结果带错误返回错误", func(t *testing.T) {
		db := &MockDB{
			WriteResult: rqlite.WriteResult{Err: ErrMockDB},
		}
		svc := NewCertificateService(db, newTestRSAUtil(t))
		err := svc.Delete(ctx, 5)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("RowsAffected为0返回not found", func(t *testing.T) {
		db := &MockDB{
			WriteResult: database.NewWriteResult(0, 0),
		}
		svc := NewCertificateService(db, newTestRSAUtil(t))
		err := svc.Delete(ctx, 999)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("期望 not found 错误，实际: %v", err)
		}
	})
}

func TestCertificateService_GetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("找到记录且无ClientKey", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				certRow(1, "cert-1", "ca-content", "client-cert", "", 100),
			}),
		}
		svc := NewCertificateService(db, rsaUtil)
		cert, err := svc.GetByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if cert.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", cert.ID)
		}
		if cert.Name != "cert-1" {
			t.Errorf("期望 Name=cert-1，实际=%s", cert.Name)
		}
		if cert.CaCert != "ca-content" {
			t.Errorf("期望 CaCert=ca-content，实际=%s", cert.CaCert)
		}
		if cert.ClientKey != "" {
			t.Errorf("期望 ClientKey 为空，实际=%s", cert.ClientKey)
		}
	})

	t.Run("找到记录并解密ClientKey新格式", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		// 先用 rsaUtil.EncryptLarge 加密一个 key
		encrypted, err := rsaUtil.EncryptLarge("secret-key")
		if err != nil {
			t.Fatalf("加密失败: %v", err)
		}
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				certRow(1, "cert-1", "ca", "cc", encrypted, 100),
			}),
		}
		svc := NewCertificateService(db, rsaUtil)
		cert, err := svc.GetByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if cert.ClientKey != "secret-key" {
			t.Errorf("期望 ClientKey=secret-key（已解密），实际=%s", cert.ClientKey)
		}
	})

	t.Run("找到记录并解密ClientKey旧格式", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		// 用 rsaUtil.Encrypt 加密（旧格式，不含 "."）
		encrypted, err := rsaUtil.Encrypt("secret-key")
		if err != nil {
			t.Fatalf("加密失败: %v", err)
		}
		if strings.Contains(encrypted, ".") {
			t.Skip("跳过：当前 Encrypt 也返回带点的格式")
		}
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				certRow(1, "cert-1", "ca", "cc", encrypted, 100),
			}),
		}
		svc := NewCertificateService(db, rsaUtil)
		cert, err := svc.GetByID(ctx, 1)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if cert.ClientKey != "secret-key" {
			t.Errorf("期望 ClientKey=secret-key（已解密），实际=%s", cert.ClientKey)
		}
	})

	t.Run("记录不存在返回错误", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows(nil),
		}
		svc := NewCertificateService(db, rsaUtil)
		_, err := svc.GetByID(ctx, 999)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("期望 not found 错误，实际: %v", err)
		}
	})

	t.Run("查询失败返回错误", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewCertificateService(db, rsaUtil)
		_, err := svc.GetByID(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("查询结果带错误返回错误", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			QueryResult: database.NewQueryResultWithErr(ErrMockDB),
		}
		svc := NewCertificateService(db, rsaUtil)
		_, err := svc.GetByID(ctx, 1)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("解密失败返回错误", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			QueryResult: database.NewQueryResultWithRows([][]interface{}{
				certRow(1, "cert-1", "ca", "cc", "invalid-encrypted-content", 100),
			}),
		}
		svc := NewCertificateService(db, rsaUtil)
		_, err := svc.GetByID(ctx, 1)
		if err == nil {
			t.Fatal("期望返回错误")
		}
		if !strings.Contains(err.Error(), "decrypt") {
			t.Errorf("期望错误包含 decrypt，实际: %v", err)
		}
	})
}

func TestCertificateService_ListByUser(t *testing.T) {
	ctx := context.Background()

	t.Run("管理员查询成功", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{certCountRow(2)}),
				database.NewQueryResultWithRows([][]interface{}{
					certSummaryRow(1, "cert-1", "ca", "cc", "ck", 100, "Alice"),
					certSummaryRow(2, "cert-2", "", "", "", 200, "Bob"),
				}),
			},
		}
		svc := NewCertificateService(db, rsaUtil)
		summaries, total, err := svc.ListByUser(ctx, 100, true, 1, 20)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if total != 2 {
			t.Errorf("期望 total=2，实际=%d", total)
		}
		if len(summaries) != 2 {
			t.Fatalf("期望 2 条记录，实际=%d", len(summaries))
		}
		if summaries[0].Name != "cert-1" {
			t.Errorf("期望 Name=cert-1，实际=%s", summaries[0].Name)
		}
		if !summaries[0].HasCACert {
			t.Error("期望 HasCACert=true")
		}
		if !summaries[0].HasClientCert {
			t.Error("期望 HasClientCert=true")
		}
		if !summaries[0].HasClientKey {
			t.Error("期望 HasClientKey=true")
		}
		if summaries[0].CreatedByName != "Alice" {
			t.Errorf("期望 CreatedByName=Alice，实际=%s", summaries[0].CreatedByName)
		}
		// cert-2 字段为空，对应的 Has* 应为 false
		if summaries[1].HasCACert {
			t.Error("期望 cert-2 HasCACert=false")
		}
		if summaries[1].HasClientCert {
			t.Error("期望 cert-2 HasClientCert=false")
		}
		if summaries[1].HasClientKey {
			t.Error("期望 cert-2 HasClientKey=false")
		}
	})

	t.Run("普通用户带created_by条件", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{certCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewCertificateService(db, rsaUtil)
		_, _, err := svc.ListByUser(ctx, 100, false, 1, 20)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if len(db.QueryStmts[0].Arguments) != 1 {
			t.Errorf("期望 1 个参数（userID），实际=%d", len(db.QueryStmts[0].Arguments))
		}
	})

	t.Run("带search按名称模糊匹配", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{certCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewCertificateService(db, rsaUtil)
		_, _, err := svc.ListByUser(ctx, 100, true, 1, 20, "keyword")
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.QueryStmts[0].Arguments[0] != "%keyword%" {
			t.Errorf("期望 %%keyword%%，实际=%v", db.QueryStmts[0].Arguments[0])
		}
	})

	t.Run("默认分页参数生效", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{certCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewCertificateService(db, rsaUtil)
		_, _, err := svc.ListByUser(ctx, 100, true, 0, 0)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		// 验证数据查询的参数（pageSize=20, offset=0）
		if db.QueryStmts[1].Arguments[0] != 20 {
			t.Errorf("期望 pageSize=20，实际=%v", db.QueryStmts[1].Arguments[0])
		}
	})

	t.Run("pageSize超过100被限制", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithRows([][]interface{}{certCountRow(0)}),
				database.NewQueryResultWithRows(nil),
			},
		}
		svc := NewCertificateService(db, rsaUtil)
		_, _, err := svc.ListByUser(ctx, 100, true, 1, 200)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if db.QueryStmts[1].Arguments[0] != 100 {
			t.Errorf("期望 pageSize=100，实际=%v", db.QueryStmts[1].Arguments[0])
		}
	})

	t.Run("count查询失败返回错误", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{QueryError: ErrMockDB}
		svc := NewCertificateService(db, rsaUtil)
		_, _, err := svc.ListByUser(ctx, 100, true, 1, 20)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})

	t.Run("count结果带错误返回错误", func(t *testing.T) {
		rsaUtil := newTestRSAUtil(t)
		db := &MockDB{
			QueryResults: []rqlite.QueryResult{
				database.NewQueryResultWithErr(ErrMockDB),
			},
		}
		svc := NewCertificateService(db, rsaUtil)
		_, _, err := svc.ListByUser(ctx, 100, true, 1, 20)
		if !errors.Is(err, ErrMockDB) {
			t.Fatalf("期望 ErrMockDB，实际: %v", err)
		}
	})
}
