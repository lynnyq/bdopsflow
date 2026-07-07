package system_config

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// mockDB 是 database.DB 的可配置 mock，用于测试 Service 的 DB 交互。
type mockDB struct {
	queryErr     error
	queryResult  rqlite.QueryResult
	writeErr     error
	writeResults []rqlite.WriteResult
	// 调用追踪
	mu             sync.Mutex
	queryCalls     int
	writeCalls     int
	lastWriteStmts []rqlite.ParameterizedStatement
}

func (m *mockDB) QueryOne(query string) (rqlite.QueryResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryCalls++
	return m.queryResult, m.queryErr
}

func (m *mockDB) QueryOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.QueryResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryCalls++
	return m.queryResult, m.queryErr
}

func (m *mockDB) WriteOneParameterized(stmt rqlite.ParameterizedStatement) (rqlite.WriteResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalls++
	m.lastWriteStmts = []rqlite.ParameterizedStatement{stmt}
	if m.writeErr != nil {
		return rqlite.WriteResult{}, m.writeErr
	}
	if len(m.writeResults) > 0 {
		return m.writeResults[0], nil
	}
	return rqlite.WriteResult{}, nil
}

func (m *mockDB) WriteParameterized(stmts []rqlite.ParameterizedStatement) ([]rqlite.WriteResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalls++
	m.lastWriteStmts = stmts
	if m.writeErr != nil {
		return nil, m.writeErr
	}
	if len(m.writeResults) > 0 {
		return m.writeResults, nil
	}
	// 默认返回与 stmts 等长的空结果
	results := make([]rqlite.WriteResult, len(stmts))
	return results, nil
}

// chanObserver 是一个使用 channel 记录通知的观察者，便于在测试中同步等待。
type chanObserver struct {
	notifications chan ConfigChangeEvent
}

func newChanObserver(buffer int) *chanObserver {
	return &chanObserver{
		notifications: make(chan ConfigChangeEvent, buffer),
	}
}

func (c *chanObserver) OnConfigChanged(key, value string) {
	select {
	case c.notifications <- ConfigChangeEvent{Key: key, Value: value}:
	default:
		// 缓冲已满，丢弃
	}
}

// panicObserver 模拟观察者 panic，用于测试 recover 逻辑。
type panicObserver struct{}

func (p *panicObserver) OnConfigChanged(key, value string) {
	panic("test panic from observer")
}

// newTestService 创建一个用于测试的 Service 实例，使用 mockDB。
// Reload 会因 queryResult 为零值（无行）而成功，cache 使用 defaultConfigValues。
// 调用者应在测试结束后调用 svc.Close() 释放后台 goroutine。
func newTestService(t *testing.T) (*Service, *mockDB) {
	t.Helper()
	db := &mockDB{}
	svc := NewService(db)
	t.Cleanup(svc.Close)
	return svc, db
}

// ------------------------------------------------------------
// NewService / Reload
// ------------------------------------------------------------

func TestNewService_FallsBackToDefaultsOnQueryError(t *testing.T) {
	db := &mockDB{queryErr: errors.New("db connection failed")}
	svc := NewService(db)
	defer svc.Close()

	// Reload 失败时应回退到默认值
	got := svc.Get("datasource.query_timeout")
	if got != "60" {
		t.Errorf("expected default '60', got %q", got)
	}
}

func TestNewService_StartsWithDefaultsWhenNoRows(t *testing.T) {
	db := &mockDB{} // queryResult 为零值，Next() 返回 false
	svc := NewService(db)
	defer svc.Close()

	// 没有数据库行，应使用默认值
	if got := svc.Get("datasource.cache_ttl"); got != "300" {
		t.Errorf("expected default '300', got %q", got)
	}
	if got := svc.GetInt("datasource.cache_max_size"); got != 100 {
		t.Errorf("expected 100, got %d", got)
	}
}

func TestReload_QueryErrReturnsError(t *testing.T) {
	db := &mockDB{queryErr: errors.New("network error")}
	svc := NewService(db) // 初始化时 Reload 失败，回退到默认
	defer svc.Close()

	// 再次调用 Reload，应返回错误
	err := svc.Reload(context.Background())
	if err == nil {
		t.Fatal("expected error from Reload, got nil")
	}
}

func TestReload_QueryResultErrReturnsError(t *testing.T) {
	db := &mockDB{queryResult: rqlite.QueryResult{Err: errors.New("result error")}}
	svc := NewService(db)
	defer svc.Close()

	err := svc.Reload(context.Background())
	if err == nil {
		t.Fatal("expected error from Reload when query result has Err, got nil")
	}
}

// ------------------------------------------------------------
// Get / GetInt / GetBool
// ------------------------------------------------------------

func TestGet_KnownKeyReturnsValue(t *testing.T) {
	svc, _ := newTestService(t)

	// 默认值存在
	got := svc.Get("datasource.default_limit")
	if got != "1000" {
		t.Errorf("expected '1000', got %q", got)
	}
}

func TestGet_UnknownKeyReturnsEmpty(t *testing.T) {
	svc, _ := newTestService(t)

	got := svc.Get("nonexistent.key")
	if got != "" {
		t.Errorf("expected empty string for unknown key, got %q", got)
	}
}

func TestGetInt_ValidNumber(t *testing.T) {
	svc, _ := newTestService(t)

	tests := []struct {
		key      string
		expected int
	}{
		{"datasource.default_limit", 1000},
		{"datasource.cache_ttl", 300},
		{"datasource.query_timeout", 60},
		{"datasource.connection_max_idle", 2},
		{"audit_log.retention_days", 90},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := svc.GetInt(tt.key); got != tt.expected {
				t.Errorf("GetInt(%q) = %d, want %d", tt.key, got, tt.expected)
			}
		})
	}
}

func TestGetInt_NonNumericReturnsZero(t *testing.T) {
	svc, _ := newTestService(t)

	// wecom.robot_url 是 text 类型，无法转为 int
	got := svc.GetInt("wecom.robot_url")
	if got != 0 {
		t.Errorf("expected 0 for non-numeric value, got %d", got)
	}
}

func TestGetInt_UnknownKeyReturnsZero(t *testing.T) {
	svc, _ := newTestService(t)

	if got := svc.GetInt("unknown.key"); got != 0 {
		t.Errorf("expected 0 for unknown key, got %d", got)
	}
}

func TestGetBool(t *testing.T) {
	svc, _ := newTestService(t)

	tests := []struct {
		key      string
		expected bool
	}{
		{"datasource.allow_write_sql", false},
		{"web.enabled", false},
		{"api_test.allow_private_network", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := svc.GetBool(tt.key); got != tt.expected {
				t.Errorf("GetBool(%q) = %v, want %v", tt.key, got, tt.expected)
			}
		})
	}
}

func TestGetBool_TrueValues(t *testing.T) {
	svc, _ := newTestService(t)

	// 手动设置 cache 为 "true" 和 "1"
	svc.mu.Lock()
	svc.cache["test.bool.true"] = "true"
	svc.cache["test.bool.one"] = "1"
	svc.cache["test.bool.other"] = "yes"
	svc.mu.Unlock()

	if !svc.GetBool("test.bool.true") {
		t.Error("GetBool should return true for 'true'")
	}
	if !svc.GetBool("test.bool.one") {
		t.Error("GetBool should return true for '1'")
	}
	if svc.GetBool("test.bool.other") {
		t.Error("GetBool should return false for 'yes'")
	}
}

// ------------------------------------------------------------
// GetAll / GetAllWithMeta
// ------------------------------------------------------------

func TestGetAll_ReturnsCopy(t *testing.T) {
	svc, _ := newTestService(t)

	all := svc.GetAll()
	if len(all) == 0 {
		t.Fatal("expected non-empty config map")
	}

	// 修改返回的 map 不应影响内部 cache
	all["injected.key"] = "injected"
	if got := svc.Get("injected.key"); got != "" {
		t.Error("modifying GetAll result should not affect internal cache")
	}
}

func TestGetAllWithMeta_ContainsAllConfigItems(t *testing.T) {
	svc, _ := newTestService(t)

	metas := svc.GetAllWithMeta()
	if len(metas) != len(configMetaList) {
		t.Fatalf("expected %d meta items, got %d", len(configMetaList), len(metas))
	}

	// 每个元数据项应有 key、value、default_value
	for _, m := range metas {
		if m.Key == "" {
			t.Error("found meta item with empty key")
		}
		if m.Value == "" && m.DefaultValue == "" {
			t.Errorf("meta item %s has empty value and default_value", m.Key)
		}
	}
}

func TestGetAllWithMeta_FillsValueFromCache(t *testing.T) {
	svc, _ := newTestService(t)

	// 修改 cache 中的值
	svc.mu.Lock()
	svc.cache["datasource.cache_ttl"] = "999"
	svc.mu.Unlock()

	metas := svc.GetAllWithMeta()
	for _, m := range metas {
		if m.Key == "datasource.cache_ttl" {
			if m.Value != "999" {
				t.Errorf("expected value '999' from cache, got %q", m.Value)
			}
			if m.DefaultValue != "300" {
				t.Errorf("expected default '300', got %q", m.DefaultValue)
			}
			return
		}
	}
	t.Fatal("datasource.cache_ttl not found in meta list")
}

func TestGetAllWithMeta_UnknownKeyUsesDefault(t *testing.T) {
	svc, _ := newTestService(t)

	// 清空 cache，GetAllWithMeta 应使用 default_value
	svc.mu.Lock()
	svc.cache = make(map[string]string)
	svc.mu.Unlock()

	metas := svc.GetAllWithMeta()
	for _, m := range metas {
		if m.Key == "datasource.query_timeout" {
			if m.Value != "60" {
				t.Errorf("expected default '60' when cache is empty, got %q", m.Value)
			}
			return
		}
	}
	t.Fatal("datasource.query_timeout not found in meta list")
}

// ------------------------------------------------------------
// validateConfigValue
// ------------------------------------------------------------

func TestValidateConfigValue_UnknownKey(t *testing.T) {
	svc, _ := newTestService(t)

	err := svc.validateConfigValue("nonexistent.key", "value")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}

	// 校验错误应为 *InvalidConfigValueError 类型，便于 handler 通过 errors.As 识别
	var invalidErr *InvalidConfigValueError
	if !errors.As(err, &invalidErr) {
		t.Errorf("expected *InvalidConfigValueError, got %T: %v", err, err)
	}
	if invalidErr.Key != "nonexistent.key" {
		t.Errorf("expected Key='nonexistent.key', got %q", invalidErr.Key)
	}
}

func TestValidateConfigValue_Boolean(t *testing.T) {
	svc, _ := newTestService(t)

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"true is valid", "true", false},
		{"false is valid", "false", false},
		{"True is invalid", "True", true},
		{"1 is invalid", "1", true},
		{"0 is invalid", "0", true},
		{"empty is invalid", "", true},
		{"yes is invalid", "yes", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.validateConfigValue("web.enabled", tt.value)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for value %q, got nil", tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error for value %q, got %v", tt.value, err)
			}
		})
	}
}

func TestValidateConfigValue_Number(t *testing.T) {
	svc, _ := newTestService(t)

	// datasource.query_timeout: min=1, max=3600
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{"valid in range", "datasource.query_timeout", "60", false},
		{"min boundary", "datasource.query_timeout", "1", false},
		{"max boundary", "datasource.query_timeout", "3600", false},
		{"below min", "datasource.query_timeout", "0", true},
		{"above max", "datasource.query_timeout", "3601", true},
		{"non-integer", "datasource.query_timeout", "abc", true},
		{"float", "datasource.query_timeout", "3.14", true},
		{"empty", "datasource.query_timeout", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.validateConfigValue(tt.key, tt.value)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for value %q, got nil", tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error for value %q, got %v", tt.value, err)
			}
		})
	}
}

func TestValidateConfigValue_Text(t *testing.T) {
	svc, _ := newTestService(t)

	// wecom.robot_url 是 text 类型
	err := svc.validateConfigValue("wecom.robot_url", "https://example.com/webhook")
	if err != nil {
		t.Errorf("expected no error for valid text, got %v", err)
	}

	// text 类型不允许空值
	err = svc.validateConfigValue("wecom.robot_url", "")
	if err == nil {
		t.Error("expected error for empty text value")
	}
}

// ------------------------------------------------------------
// Set
// ------------------------------------------------------------

func TestSet_ValidationErrorReturnsBeforeDB(t *testing.T) {
	svc, db := newTestService(t)

	// 传入非法值（超出范围）
	err := svc.Set(context.Background(), "datasource.query_timeout", "999999", 1)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	// Set 透传 *InvalidConfigValueError，handler 可通过 errors.As 识别为 400 而非 500
	var invalidErr *InvalidConfigValueError
	if !errors.As(err, &invalidErr) {
		t.Errorf("expected *InvalidConfigValueError from Set, got %T: %v", err, err)
	}

	// DB 不应被调用
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.writeCalls != 0 {
		t.Errorf("expected 0 DB write calls, got %d", db.writeCalls)
	}
}

func TestSet_UnknownKeyReturnsError(t *testing.T) {
	svc, db := newTestService(t)

	err := svc.Set(context.Background(), "unknown.key", "value", 1)
	if err == nil {
		t.Fatal("expected error for unknown key")
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	if db.writeCalls != 0 {
		t.Errorf("expected 0 DB write calls, got %d", db.writeCalls)
	}
}

func TestSet_SuccessUpdatesCacheAndWritesDB(t *testing.T) {
	svc, db := newTestService(t)

	err := svc.Set(context.Background(), "datasource.cache_ttl", "600", 42)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// cache 应更新
	if got := svc.Get("datasource.cache_ttl"); got != "600" {
		t.Errorf("expected cache updated to '600', got %q", got)
	}

	// DB 应收到 WriteParameterized 调用，包含 2 条语句（upsert + history）
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.writeCalls != 1 {
		t.Fatalf("expected 1 write call, got %d", db.writeCalls)
	}
	if len(db.lastWriteStmts) != 2 {
		t.Fatalf("expected 2 statements (upsert+history), got %d", len(db.lastWriteStmts))
	}
	// upsert 语句的参数应为 [key, value, now]
	upsertArgs := db.lastWriteStmts[0].Arguments
	if len(upsertArgs) < 2 {
		t.Fatalf("expected at least 2 upsert args, got %d", len(upsertArgs))
	}
	if upsertArgs[0] != "datasource.cache_ttl" {
		t.Errorf("expected key 'datasource.cache_ttl', got %v", upsertArgs[0])
	}
	if upsertArgs[1] != "600" {
		t.Errorf("expected value '600', got %v", upsertArgs[1])
	}
	// history 语句的参数应为 [key, oldValue, newValue, changedBy, now]
	historyArgs := db.lastWriteStmts[1].Arguments
	if len(historyArgs) < 4 {
		t.Fatalf("expected at least 4 history args, got %d", len(historyArgs))
	}
	if historyArgs[0] != "datasource.cache_ttl" {
		t.Errorf("expected history key, got %v", historyArgs[0])
	}
	// oldValue 应为默认值 "300"
	if historyArgs[1] != "300" {
		t.Errorf("expected old value '300', got %v", historyArgs[1])
	}
	if historyArgs[2] != "600" {
		t.Errorf("expected new value '600', got %v", historyArgs[2])
	}
	if historyArgs[3] != int64(42) {
		t.Errorf("expected changedBy=42, got %v", historyArgs[3])
	}
}

func TestSet_DBErrorDoesNotUpdateCache(t *testing.T) {
	db := &mockDB{writeErr: errors.New("db write failed")}
	svc := NewService(db) // 初始化时 Reload 成功（空 queryResult）
	defer svc.Close()

	originalValue := svc.Get("datasource.cache_ttl")

	err := svc.Set(context.Background(), "datasource.cache_ttl", "600", 1)
	if err == nil {
		t.Fatal("expected error from DB write failure, got nil")
	}

	// cache 不应更新
	if got := svc.Get("datasource.cache_ttl"); got != originalValue {
		t.Errorf("cache should not be updated on DB error; expected %q, got %q", originalValue, got)
	}
}

func TestSet_WriteResultErrDoesNotUpdateCache(t *testing.T) {
	db := &mockDB{
		writeResults: []rqlite.WriteResult{
			{Err: errors.New("upsert constraint violation")},
		},
	}
	svc := NewService(db)
	defer svc.Close()

	originalValue := svc.Get("datasource.cache_ttl")

	err := svc.Set(context.Background(), "datasource.cache_ttl", "600", 1)
	if err == nil {
		t.Fatal("expected error from WriteResult.Err, got nil")
	}

	if got := svc.Get("datasource.cache_ttl"); got != originalValue {
		t.Errorf("cache should not be updated on write result error; expected %q, got %q", originalValue, got)
	}
}

func TestSet_HistoryErrDoesNotBlockUpdate(t *testing.T) {
	// 第二条语句（history）返回 Err，但主 upsert 成功。
	// cache 仍应更新，且 Set 不应返回错误。
	db := &mockDB{
		writeResults: []rqlite.WriteResult{
			{Err: nil},                       // upsert 成功
			{Err: errors.New("history fail")}, // history 失败
		},
	}
	svc := NewService(db)
	defer svc.Close()

	err := svc.Set(context.Background(), "datasource.cache_ttl", "600", 1)
	if err != nil {
		t.Fatalf("expected no error (history failure is non-blocking), got %v", err)
	}

	// cache 应已更新
	if got := svc.Get("datasource.cache_ttl"); got != "600" {
		t.Errorf("expected cache updated to '600', got %q", got)
	}
}

func TestSet_BooleanValue(t *testing.T) {
	svc, _ := newTestService(t)

	// web.enabled 默认 false，设为 true
	err := svc.Set(context.Background(), "web.enabled", "true", 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !svc.GetBool("web.enabled") {
		t.Error("expected web.enabled to be true after Set")
	}

	// 设回 false
	err = svc.Set(context.Background(), "web.enabled", "false", 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if svc.GetBool("web.enabled") {
		t.Error("expected web.enabled to be false after Set")
	}
}

// ------------------------------------------------------------
// Observer 通知
// ------------------------------------------------------------

func TestRegisterObserver_AndSetNotifies(t *testing.T) {
	svc, _ := newTestService(t)

	obs := newChanObserver(10)
	svc.RegisterObserver(obs)

	err := svc.Set(context.Background(), "datasource.cache_ttl", "777", 1)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// 等待观察者被通知（processChanges 是异步的）
	select {
	case event := <-obs.notifications:
		if event.Key != "datasource.cache_ttl" {
			t.Errorf("expected key 'datasource.cache_ttl', got %q", event.Key)
		}
		if event.Value != "777" {
			t.Errorf("expected value '777', got %q", event.Value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("observer was not notified within 2 seconds")
	}
}

func TestUnregisterObserver_StopsNotification(t *testing.T) {
	svc, _ := newTestService(t)

	obs := newChanObserver(10)
	svc.RegisterObserver(obs)
	svc.UnregisterObserver(obs)

	err := svc.Set(context.Background(), "datasource.cache_ttl", "888", 1)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// 给足时间让可能的异步通知到达
	select {
	case <-obs.notifications:
		t.Fatal("unregistered observer should not receive notifications")
	case <-time.After(500 * time.Millisecond):
		// 预期：没有通知
	}
}

func TestObserverPanic_DoesNotCrashService(t *testing.T) {
	svc, _ := newTestService(t)

	panicObs := &panicObserver{}
	svc.RegisterObserver(panicObs)

	// Set 不应因观察者 panic 而失败
	err := svc.Set(context.Background(), "datasource.cache_ttl", "555", 1)
	if err != nil {
		t.Fatalf("Set failed due to observer panic: %v", err)
	}

	// Service 仍可正常工作
	if got := svc.Get("datasource.cache_ttl"); got != "555" {
		t.Errorf("expected '555' after Set, got %q", got)
	}

	// 再设置一次，确认 processChanges goroutine 未崩溃
	err = svc.Set(context.Background(), "datasource.cache_ttl", "666", 1)
	if err != nil {
		t.Fatalf("second Set failed: %v", err)
	}
	if got := svc.Get("datasource.cache_ttl"); got != "666" {
		t.Errorf("expected '666' after second Set, got %q", got)
	}
}

// ------------------------------------------------------------
// Close
// ------------------------------------------------------------

func TestClose_Idempotent(t *testing.T) {
	svc, _ := newTestService(t)

	// 多次 Close 不应 panic
	svc.Close()
	svc.Close()
	svc.Close()
}

func TestClose_StopsBackgroundGoroutine(t *testing.T) {
	db := &mockDB{}
	svc := NewService(db)
	svc.Close()

	// Close 后，stopCh 已关闭。Set 发送通知应走 stopCh 分支（不阻塞）
	// 这里主要验证不会死锁或 panic
	_ = svc.Set(context.Background(), "datasource.cache_ttl", "123", 1)
	// cache 仍应更新（DB 写入成功，仅通知被跳过）
	if got := svc.Get("datasource.cache_ttl"); got != "123" {
		t.Errorf("expected cache updated to '123' after Close, got %q", got)
	}
}

// ------------------------------------------------------------
// defaultConfigValues 与 configMetaList 一致性
// ------------------------------------------------------------

func TestDefaultConfigValues_AllMetaKeysHaveDefaults(t *testing.T) {
	// 每个元数据项都应有对应的默认值
	for _, meta := range configMetaList {
		if _, ok := defaultConfigValues[meta.Key]; !ok {
			t.Errorf("config key %q in configMetaList has no default value in defaultConfigValues", meta.Key)
		}
	}
}

func TestDefaultConfigValues_MatchMetaDefaults(t *testing.T) {
	// 元数据中的 DefaultValue 应与 defaultConfigValues 一致
	for _, meta := range configMetaList {
		def, ok := defaultConfigValues[meta.Key]
		if !ok {
			continue
		}
		if def != meta.DefaultValue {
			t.Errorf("key %q: defaultConfigValues=%q but configMetaList.DefaultValue=%q", meta.Key, def, meta.DefaultValue)
		}
	}
}

// ------------------------------------------------------------
// 编译期接口检查
// ------------------------------------------------------------

var _ database.DB = (*mockDB)(nil)
var _ ConfigObserver = (*chanObserver)(nil)
var _ ConfigObserver = (*panicObserver)(nil)
