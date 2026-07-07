package datasource

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// dsMockDB 是 datasource 包测试用的可配置 DB mock。
// 通过 QueryResults/WriteResult/WriteResults 控制返回值，支持多次调用按序返回。
type dsMockDB struct {
	// queryResults 按调用顺序依次返回，耗尽后返回 queryResult
	queryResults []rqlite.QueryResult
	queryResult  rqlite.QueryResult
	queryError   error

	// writeResults 按调用顺序依次返回，耗尽后返回 writeResult
	writeResults []rqlite.WriteResult
	writeResult  rqlite.WriteResult
	writeError   error

	// 记录所有调用
	queryStmts []rqlite.ParameterizedStatement
	writeStmts []rqlite.ParameterizedStatement
}

func (m *dsMockDB) QueryOne(query string) (rqlite.QueryResult, error) {
	m.queryStmts = append(m.queryStmts, rqlite.ParameterizedStatement{Query: query})
	if m.queryError != nil {
		return rqlite.QueryResult{}, m.queryError
	}
	return m.nextQuery(), nil
}

func (m *dsMockDB) QueryOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.QueryResult, error) {
	m.queryStmts = append(m.queryStmts, stmt)
	if m.queryError != nil {
		return rqlite.QueryResult{}, m.queryError
	}
	return m.nextQuery(), nil
}

func (m *dsMockDB) WriteOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.WriteResult, error) {
	m.writeStmts = append(m.writeStmts, stmt)
	if m.writeError != nil {
		return rqlite.WriteResult{}, m.writeError
	}
	return m.nextWrite(), nil
}

func (m *dsMockDB) WriteParameterized(stmts []rqlite.ParameterizedStatement) ([]rqlite.WriteResult, error) {
	for _, s := range stmts {
		m.writeStmts = append(m.writeStmts, s)
	}
	if m.writeError != nil {
		return nil, m.writeError
	}
	results := make([]rqlite.WriteResult, len(stmts))
	for i := range results {
		results[i] = m.nextWrite()
	}
	return results, nil
}

func (m *dsMockDB) nextQuery() rqlite.QueryResult {
	if len(m.queryResults) > 0 {
		qr := m.queryResults[0]
		m.queryResults = m.queryResults[1:]
		return qr
	}
	return m.queryResult
}

func (m *dsMockDB) nextWrite() rqlite.WriteResult {
	if len(m.writeResults) > 0 {
		wr := m.writeResults[0]
		m.writeResults = m.writeResults[1:]
		return wr
	}
	return m.writeResult
}

// makeDSRow 构造一行 datasource 数据（25 列），用于 scanDatasource 测试
func makeDSRow() []interface{} {
	return []interface{}{
		int64(1),                  // id
		"test-ds",                 // name
		"mysql",                   // type
		"localhost",               // host
		int64(3306),               // port
		"",                        // path
		"testdb",                  // database
		"root",                    // username
		"encrypted-pwd",           // password
		"password",                // auth_type
		"direct",                  // connection_mode
		"",                        // zk_hosts
		"",                        // zk_path
		"",                        // rqlite_hosts
		"",                        // config
		"test datasource",         // description
		int64(1),                  // domain_id
		int64(1),                  // is_enabled
		int64(0),                  // allow_write_sql
		"success",                 // test_status
		time.Now(),                // last_test_at (time.Time)
		int64(100),                // created_by
		int64(200),                // updated_by
		time.Now(),                // created_at (time.Time)
		time.Now(),                // updated_at (time.Time)
	}
}

// makeDSRowStrings 构造一行 datasource 数据，时间字段使用字符串
func makeDSRowStrings() []interface{} {
	return []interface{}{
		int64(2),                          // id
		"string-time-ds",                  // name
		"postgresql",                      // type
		"db.example.com",                  // host
		int64(5432),                       // port
		"",                                // path
		"mydb",                            // database
		"admin",                           // username
		"pwd",                             // password
		"password",                        // auth_type
		"direct",                          // connection_mode
		"",                                // zk_hosts
		"",                                // zk_path
		"",                                // rqlite_hosts
		`{"timeout":30}`,                  // config
		"string time ds",                  // description
		int64(2),                          // domain_id
		int64(0),                          // is_enabled
		int64(1),                          // allow_write_sql
		"untested",                        // test_status
		"2025-01-01T12:00:00Z",            // last_test_at (string RFC3339)
		nil,                               // created_by (nil)
		nil,                               // updated_by (nil)
		"2025-01-01T12:00:00Z",            // created_at (string RFC3339)
		"2025-01-02T08:30:00Z",            // updated_at (string RFC3339)
	}
}

// makeDSRowLegacyTime 构造一行 datasource 数据，时间字段使用旧格式字符串
func makeDSRowLegacyTime() []interface{} {
	return []interface{}{
		int64(3),                  // id
		"legacy-ds",               // name
		"rqlite",                  // type
		"localhost",               // host
		int64(4001),               // port
		"/data/db",                // path
		"",                        // database
		"",                        // username
		"",                        // password
		"none",                    // auth_type
		"",                        // connection_mode
		"",                        // zk_hosts
		"",                        // zk_path
		"localhost:4001",          // rqlite_hosts
		"",                        // config
		"",                        // description
		int64(1),                  // domain_id
		int64(1),                  // is_enabled
		int64(0),                  // allow_write_sql
		"",                        // test_status (empty)
		"2025-06-01 10:00:00",     // last_test_at (legacy format string)
		nil,                       // created_by
		nil,                       // updated_by
		"2025-06-01 10:00:00",     // created_at (legacy format string)
		"2025-06-01 10:00:00",     // updated_at (legacy format string)
	}
}

// ==================== Create ====================

func TestDatasourceService_Create_Success(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(0)}}), // name not exists
		},
		writeResult: database.NewWriteResult(1, 1),
	}
	svc := NewDatasourceService(db, nil, nil)

	createdBy := int64(100)
	ds := &model.Datasource{
		Name:      "new-ds",
		Type:      "mysql",
		Host:      "localhost",
		Port:      3306,
		Database:  "testdb",
		Username:  "root",
		Password:  "secret",
		DomainID:  1,
		IsEnabled: true,
		CreatedBy: &createdBy,
	}

	err := svc.Create(context.Background(), ds)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ds.ID != 1 {
		t.Errorf("expected ID=1, got %d", ds.ID)
	}
	if len(db.writeStmts) != 1 {
		t.Errorf("expected 1 write call, got %d", len(db.writeStmts))
	}
}

func TestDatasourceService_Create_NameExists(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{{int64(1)}}), // name exists
	}
	svc := NewDatasourceService(db, nil, nil)

	ds := &model.Datasource{
		Name:     "existing-ds",
		Type:     "mysql",
		DomainID: 1,
	}

	err := svc.Create(context.Background(), ds)
	if !errors.Is(err, ErrDatasourceNameExists) {
		t.Errorf("expected ErrDatasourceNameExists, got %v", err)
	}
}

func TestDatasourceService_Create_QueryError(t *testing.T) {
	db := &dsMockDB{
		queryError: errors.New("db connection error"),
	}
	svc := NewDatasourceService(db, nil, nil)

	ds := &model.Datasource{Name: "test", DomainID: 1}
	err := svc.Create(context.Background(), ds)
	if err == nil {
		t.Fatal("expected error on query failure")
	}
	if !strings.Contains(err.Error(), "failed to check datasource name") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDatasourceService_Create_WithCrypto(t *testing.T) {
	crypto, err := NewCrypto(strings.Repeat("a", 32))
	if err != nil {
		t.Fatalf("failed to create crypto: %v", err)
	}

	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(0)}}),
		},
		writeResult: database.NewWriteResult(5, 1),
	}
	svc := NewDatasourceService(db, crypto, nil)

	ds := &model.Datasource{
		Name:     "crypto-ds",
		Type:     "mysql",
		Password: "my-password",
		DomainID: 1,
	}

	err = svc.Create(context.Background(), ds)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ds.ID != 5 {
		t.Errorf("expected ID=5, got %d", ds.ID)
	}

	// 验证密码被加密（写入的参数中密码不等于明文）
	if len(db.writeStmts) != 1 {
		t.Fatalf("expected 1 write, got %d", len(db.writeStmts))
	}
	// password 是第 8 个参数（index 7）
	storedPwd, ok := db.writeStmts[0].Arguments[7].(string)
	if !ok {
		t.Fatalf("expected password to be string, got %T", db.writeStmts[0].Arguments[7])
	}
	if storedPwd == "my-password" {
		t.Error("expected password to be encrypted, but got plaintext")
	}

	// 验证加密后的密码可以解密
	decrypted, err := crypto.Decrypt(storedPwd)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}
	if decrypted != "my-password" {
		t.Errorf("expected decrypted password 'my-password', got %q", decrypted)
	}
}

func TestDatasourceService_Create_CryptoError(t *testing.T) {
	// 使用一个会加密失败的 crypto（通过构造一个有效的 crypto，但加密正常，
	// 这里测试 crypto 为 nil 但 password 不为空时不会加密）
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(0)}}),
		},
		writeResult: database.NewWriteResult(1, 1),
	}
	svc := NewDatasourceService(db, nil, nil)

	ds := &model.Datasource{
		Name:     "no-crypto-ds",
		Password: "plaintext",
		DomainID: 1,
	}

	err := svc.Create(context.Background(), ds)
	if err != nil {
		t.Fatalf("expected no error without crypto, got %v", err)
	}

	// 密码应为明文（无 crypto 时不加密）
	storedPwd := db.writeStmts[0].Arguments[7].(string)
	if storedPwd != "plaintext" {
		t.Errorf("expected plaintext password without crypto, got %q", storedPwd)
	}
}

func TestDatasourceService_Create_WriteError(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(0)}}),
		},
		writeError: errors.New("write failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	ds := &model.Datasource{Name: "test", DomainID: 1}
	err := svc.Create(context.Background(), ds)
	if err == nil {
		t.Fatal("expected error on write failure")
	}
	if !strings.Contains(err.Error(), "failed to create datasource") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== GetByID ====================

func TestDatasourceService_GetByID_Success(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{makeDSRow()}),
	}
	svc := NewDatasourceService(db, nil, nil)

	ds, err := svc.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ds == nil {
		t.Fatal("expected non-nil datasource")
	}
	if ds.ID != 1 {
		t.Errorf("expected ID=1, got %d", ds.ID)
	}
	if ds.Name != "test-ds" {
		t.Errorf("expected name 'test-ds', got %q", ds.Name)
	}
	if ds.Type != "mysql" {
		t.Errorf("expected type 'mysql', got %q", ds.Type)
	}
	if ds.Port != 3306 {
		t.Errorf("expected port 3306, got %d", ds.Port)
	}
	if !ds.IsEnabled {
		t.Error("expected IsEnabled=true")
	}
	if ds.LastTestAt == nil {
		t.Error("expected non-nil LastTestAt")
	}
}

func TestDatasourceService_GetByID_NotFound(t *testing.T) {
	// 空结果（无行）
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{}),
	}
	svc := NewDatasourceService(db, nil, nil)

	_, err := svc.GetByID(context.Background(), 999)
	if !errors.Is(err, ErrDatasourceNotFound) {
		t.Errorf("expected ErrDatasourceNotFound, got %v", err)
	}
}

func TestDatasourceService_GetByID_QueryError(t *testing.T) {
	db := &dsMockDB{
		queryError: errors.New("query failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	_, err := svc.GetByID(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error on query failure")
	}
	if !strings.Contains(err.Error(), "failed to get datasource") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDatasourceService_GetByID_StringTime(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{makeDSRowStrings()}),
	}
	svc := NewDatasourceService(db, nil, nil)

	ds, err := svc.GetByID(context.Background(), 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ds.ID != 2 {
		t.Errorf("expected ID=2, got %d", ds.ID)
	}
	if ds.LastTestAt == nil {
		t.Error("expected non-nil LastTestAt from string")
	}
	if ds.CreatedBy != nil {
		t.Errorf("expected nil CreatedBy, got %v", ds.CreatedBy)
	}
	if ds.UpdatedBy != nil {
		t.Errorf("expected nil UpdatedBy, got %v", ds.UpdatedBy)
	}
}

func TestDatasourceService_GetByID_LegacyTimeFormat(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{makeDSRowLegacyTime()}),
	}
	svc := NewDatasourceService(db, nil, nil)

	ds, err := svc.GetByID(context.Background(), 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ds.ID != 3 {
		t.Errorf("expected ID=3, got %d", ds.ID)
	}
	if ds.LastTestAt == nil {
		t.Error("expected non-nil LastTestAt from legacy format")
	}
	if ds.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

// ==================== Get ====================

func TestDatasourceService_Get_Success(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(1)}}),  // count
			database.NewQueryResultWithRows([][]interface{}{makeDSRow()}), // data
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	datasources, total, err := svc.Get(context.Background(), 1, "mysql", 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if len(datasources) != 1 {
		t.Fatalf("expected 1 datasource, got %d", len(datasources))
	}
	if datasources[0].Name != "test-ds" {
		t.Errorf("expected name 'test-ds', got %q", datasources[0].Name)
	}
}

func TestDatasourceService_Get_WithSearch(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(0)}}), // count=0
			database.NewQueryResultWithRows([][]interface{}{}),           // no data
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	datasources, total, err := svc.Get(context.Background(), 1, "", 1, 10, "keyword")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 0 {
		t.Errorf("expected total=0, got %d", total)
	}
	if len(datasources) != 0 {
		t.Errorf("expected 0 datasources, got %d", len(datasources))
	}
}

func TestDatasourceService_Get_NoArgs(t *testing.T) {
	// 没有 domainID、type、search 时走 QueryOne 路径
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(2)}}), // count
			database.NewQueryResultWithRows([][]interface{}{makeDSRow(), makeDSRowStrings()}), // data
		},
	}
	svc := NewDatasourceService(db, nil, nil)

	datasources, total, err := svc.Get(context.Background(), 0, "", 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if len(datasources) != 2 {
		t.Errorf("expected 2 datasources, got %d", len(datasources))
	}
}

func TestDatasourceService_Get_CountError(t *testing.T) {
	db := &dsMockDB{
		queryError: errors.New("count failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	_, _, err := svc.Get(context.Background(), 1, "mysql", 1, 10)
	if err == nil {
		t.Fatal("expected error on count failure")
	}
	if !strings.Contains(err.Error(), "failed to count") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDatasourceService_Get_DataError(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{{int64(1)}}), // count ok
		},
		queryError: errors.New("data query failed"), // 第二次查询报错
	}
	svc := NewDatasourceService(db, nil, nil)

	_, _, err := svc.Get(context.Background(), 1, "mysql", 1, 10)
	if err == nil {
		t.Fatal("expected error on data query failure")
	}
}

// ==================== Update ====================

func TestDatasourceService_Update_Success(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{makeDSRow()}), // GetByID
		},
		writeResult: database.NewWriteResult(0, 1),
	}
	mgr := NewManager(nil, nil)
	svc := NewDatasourceService(db, nil, mgr)

	updatedBy := int64(200)
	ds := &model.Datasource{
		ID:        1,
		Name:      "updated-ds",
		Type:      "mysql",
		Host:      "new-host",
		Port:      3306,
		UpdatedBy: &updatedBy,
	}

	err := svc.Update(context.Background(), ds)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDatasourceService_Update_NotFound(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{}), // GetByID returns no rows
	}
	svc := NewDatasourceService(db, nil, nil)

	ds := &model.Datasource{ID: 999, Name: "test"}
	err := svc.Update(context.Background(), ds)
	if !errors.Is(err, ErrDatasourceNotFound) {
		t.Errorf("expected ErrDatasourceNotFound, got %v", err)
	}
}

func TestDatasourceService_Update_WithCrypto(t *testing.T) {
	crypto, _ := NewCrypto(strings.Repeat("a", 32))
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{makeDSRow()}),
		},
		writeResult: database.NewWriteResult(0, 1),
	}
	mgr := NewManager(nil, nil)
	svc := NewDatasourceService(db, crypto, mgr)

	ds := &model.Datasource{
		ID:       1,
		Name:     "updated",
		Password: "new-password",
	}

	err := svc.Update(context.Background(), ds)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 验证密码被加密
	storedPwd := db.writeStmts[0].Arguments[7].(string)
	if storedPwd == "new-password" {
		t.Error("expected encrypted password")
	}
}

func TestDatasourceService_Update_EmptyPasswordKeepsExisting(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{makeDSRow()}),
		},
		writeResult: database.NewWriteResult(0, 1),
	}
	mgr := NewManager(nil, nil)
	svc := NewDatasourceService(db, nil, mgr)

	ds := &model.Datasource{
		ID:   1,
		Name: "updated",
		// Password 为空，应保留原密码
	}

	err := svc.Update(context.Background(), ds)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 验证使用了原密码
	storedPwd := db.writeStmts[0].Arguments[7].(string)
	if storedPwd != "encrypted-pwd" {
		t.Errorf("expected existing password 'encrypted-pwd', got %q", storedPwd)
	}
}

func TestDatasourceService_Update_WriteError(t *testing.T) {
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{makeDSRow()}),
		},
		writeError: errors.New("update failed"),
	}
	mgr := NewManager(nil, nil)
	svc := NewDatasourceService(db, nil, mgr)

	ds := &model.Datasource{ID: 1, Name: "test"}
	err := svc.Update(context.Background(), ds)
	if err == nil {
		t.Fatal("expected error on write failure")
	}
}

// ==================== Delete ====================

func TestDatasourceService_Delete_Success(t *testing.T) {
	db := &dsMockDB{
		writeResult: database.NewWriteResult(0, 1),
	}
	mgr := NewManager(nil, nil)
	svc := NewDatasourceService(db, nil, mgr)

	err := svc.Delete(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDatasourceService_Delete_WriteError(t *testing.T) {
	db := &dsMockDB{
		writeError: errors.New("delete failed"),
	}
	mgr := NewManager(nil, nil)
	svc := NewDatasourceService(db, nil, mgr)

	err := svc.Delete(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error on write failure")
	}
	if !strings.Contains(err.Error(), "failed to delete datasource") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ==================== GetDatasourceMaskedPassword ====================

func TestDatasourceService_GetDatasourceMaskedPassword_Success(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{makeDSRow()}),
	}
	svc := NewDatasourceService(db, nil, nil)

	pwd, err := svc.GetDatasourceMaskedPassword(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pwd != "******" {
		t.Errorf("expected '******', got %q", pwd)
	}
}

func TestDatasourceService_GetDatasourceMaskedPassword_NotFound(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{}),
	}
	svc := NewDatasourceService(db, nil, nil)

	_, err := svc.GetDatasourceMaskedPassword(context.Background(), 999)
	if !errors.Is(err, ErrDatasourceNotFound) {
		t.Errorf("expected ErrDatasourceNotFound, got %v", err)
	}
}

// ==================== GetDatasourceDomainID ====================

func TestDatasourceService_GetDatasourceDomainID_Success(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{{int64(5)}}),
	}
	svc := NewDatasourceService(db, nil, nil)

	domainID, err := svc.GetDatasourceDomainID(1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if domainID != 5 {
		t.Errorf("expected domainID=5, got %d", domainID)
	}
}

func TestDatasourceService_GetDatasourceDomainID_NotFound(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{}),
	}
	svc := NewDatasourceService(db, nil, nil)

	_, err := svc.GetDatasourceDomainID(999)
	if !errors.Is(err, ErrDatasourceNotFound) {
		t.Errorf("expected ErrDatasourceNotFound, got %v", err)
	}
}

func TestDatasourceService_GetDatasourceDomainID_QueryError(t *testing.T) {
	db := &dsMockDB{
		queryError: errors.New("query failed"),
	}
	svc := NewDatasourceService(db, nil, nil)

	_, err := svc.GetDatasourceDomainID(1)
	if !errors.Is(err, ErrDatasourceNotFound) {
		t.Errorf("expected ErrDatasourceNotFound on query error, got %v", err)
	}
}

// ==================== dsParseTimeInLocalTimezone ====================

func TestDsParseTimeInLocalTimezone_RFC3339Nano(t *testing.T) {
	input := "2025-06-15T10:30:00.123456789Z"
	parsed, err := dsParseTimeInLocalTimezone(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if parsed.Year() != 2025 || parsed.Month() != 6 || parsed.Day() != 15 {
		t.Errorf("unexpected parsed time: %v", parsed)
	}
}

func TestDsParseTimeInLocalTimezone_RFC3339(t *testing.T) {
	input := "2025-06-15T10:30:00Z"
	parsed, err := dsParseTimeInLocalTimezone(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if parsed.Year() != 2025 {
		t.Errorf("expected year 2025, got %d", parsed.Year())
	}
}

func TestDsParseTimeInLocalTimezone_LegacyFormat(t *testing.T) {
	input := "2025-06-15 10:30:00"
	parsed, err := dsParseTimeInLocalTimezone(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if parsed.Year() != 2025 {
		t.Errorf("expected year 2025, got %d", parsed.Year())
	}
}

func TestDsParseTimeInLocalTimezone_InvalidFormat(t *testing.T) {
	input := "not-a-date"
	_, err := dsParseTimeInLocalTimezone(input)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestDsParseTimeInLocalTimezone_EmptyString(t *testing.T) {
	_, err := dsParseTimeInLocalTimezone("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}
