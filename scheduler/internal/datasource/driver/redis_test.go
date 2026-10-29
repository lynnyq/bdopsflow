package driver

import (
	"context"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

// setupMiniredis 创建一个测试用的 miniredis 实例，返回 host 和 port（int）。
func setupMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(func() {
		mr.Close()
	})
	return mr
}

// miniredisPort 获取 miniredis 的端口号（int 类型）。
func miniredisPort(t *testing.T, mr *miniredis.Miniredis) int {
	t.Helper()
	port, err := strconv.Atoi(mr.Port())
	if err != nil {
		t.Fatalf("failed to parse miniredis port: %v", err)
	}
	return port
}

// TestRedisDriver_Connect_Single 测试单节点模式连接（无密码）。
func TestRedisDriver_Connect_Single(t *testing.T) {
	mr := setupMiniredis(t)

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	if err := drv.TestConnection(ctx); err != nil {
		t.Errorf("TestConnection failed: %v", err)
	}
	if err := drv.Ping(ctx); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

// TestRedisDriver_Connect_SingleWithPassword 测试单节点模式连接（带密码）。
func TestRedisDriver_Connect_SingleWithPassword(t *testing.T) {
	mr := setupMiniredis(t)
	mr.RequireUserAuth("default", "secret123")

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		Password:       "secret123",
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect with password failed: %v", err)
	}
	defer drv.Close()

	if err := drv.TestConnection(ctx); err != nil {
		t.Errorf("TestConnection failed: %v", err)
	}
}

// TestRedisDriver_Connect_SingleWrongPassword 测试单节点模式错误密码连接失败。
func TestRedisDriver_Connect_SingleWrongPassword(t *testing.T) {
	mr := setupMiniredis(t)
	mr.RequireUserAuth("default", "secret123")

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		Password:       "wrong-password",
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	err := drv.Connect(ctx, config)
	if err == nil {
		t.Fatal("expected connection to fail with wrong password, but succeeded")
	}
}

// TestRedisDriver_Connect_DefaultPort 测试默认端口处理。
func TestRedisDriver_Connect_DefaultPort(t *testing.T) {
	mr := setupMiniredis(t)

	drv := NewRedisDriver()
	// Port 设为 0，应使用默认端口 6379，但 miniredis 端口随机，
	// 这里测试 Port=0 时不崩溃（实际连接会失败但不 panic）
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           0,
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	// 预期连接失败（因为端口是 6379 而非 miniredis 的随机端口），但不应 panic
	_ = drv.Connect(ctx, config)
}

// TestRedisDriver_Query_GET 测试 GET 命令查询（单值，单列 "value"）。
func TestRedisDriver_Query_GET(t *testing.T) {
	mr := setupMiniredis(t)
	mr.Set("foo", "bar")

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	result, err := drv.Query(ctx, "GET foo")
	if err != nil {
		t.Fatalf("Query GET failed: %v", err)
	}
	if result.RowCount != 1 {
		t.Errorf("expected 1 row, got %d", result.RowCount)
	}
	if len(result.Columns) != 1 || result.Columns[0] != "value" {
		t.Errorf("unexpected columns: %v", result.Columns)
	}
	if len(result.Rows) != 1 || len(result.Rows[0]) != 1 {
		t.Fatalf("unexpected rows: %v", result.Rows)
	}
	if result.Rows[0][0] != "bar" {
		t.Errorf("expected 'bar', got %v", result.Rows[0][0])
	}
}

// TestRedisDriver_Query_SET 测试 SET 命令（写操作）。
func TestRedisDriver_Query_SET(t *testing.T) {
	mr := setupMiniredis(t)

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	_, err := drv.Query(ctx, "SET mykey myvalue")
	if err != nil {
		t.Fatalf("Query SET failed: %v", err)
	}

	// 验证值已写入
	val, err := mr.Get("mykey")
	if err != nil {
		t.Fatalf("miniredis Get failed: %v", err)
	}
	if val != "myvalue" {
		t.Errorf("expected 'myvalue', got %s", val)
	}
}

// TestRedisDriver_Query_QuotedArgs 测试带引号参数的命令解析。
func TestRedisDriver_Query_QuotedArgs(t *testing.T) {
	mr := setupMiniredis(t)

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	// 带双引号的参数
	_, err := drv.Query(ctx, `SET greeting "hello world"`)
	if err != nil {
		t.Fatalf("Query SET with quoted args failed: %v", err)
	}

	val, err := mr.Get("greeting")
	if err != nil {
		t.Fatalf("miniredis Get failed: %v", err)
	}
	if val != "hello world" {
		t.Errorf("expected 'hello world', got %s", val)
	}
}

// TestRedisDriver_Query_KEYS 测试 KEYS 命令（展开为多行单列 "key"）。
func TestRedisDriver_Query_KEYS(t *testing.T) {
	mr := setupMiniredis(t)
	mr.Set("key1", "val1")
	mr.Set("key2", "val2")
	mr.Set("other", "val3")

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	result, err := drv.Query(ctx, "KEYS key*")
	if err != nil {
		t.Fatalf("Query KEYS failed: %v", err)
	}
	// 应该展开为多行：key1, key2
	if result.RowCount != 2 {
		t.Errorf("expected 2 rows (key1, key2), got %d", result.RowCount)
	}
	if len(result.Columns) != 1 || result.Columns[0] != "key" {
		t.Errorf("unexpected columns: %v", result.Columns)
	}
	// 验证值包含 key1 和 key2
	found := make(map[string]bool)
	for _, row := range result.Rows {
		found[row[0].(string)] = true
	}
	if !found["key1"] || !found["key2"] {
		t.Errorf("expected key1 and key2, got %v", result.Rows)
	}
}

// TestRedisDriver_Query_HGETALL 测试 HGETALL 哈希命令（两列 field/value）。
func TestRedisDriver_Query_HGETALL(t *testing.T) {
	mr := setupMiniredis(t)
	mr.HSet("myhash", "name", "alice")
	mr.HSet("myhash", "age", "30")

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	result, err := drv.Query(ctx, "HGETALL myhash")
	if err != nil {
		t.Fatalf("Query HGETALL failed: %v", err)
	}
	if result.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", result.RowCount)
	}
	if len(result.Columns) != 2 {
		t.Fatalf("expected 2 columns (field, value), got %v", result.Columns)
	}
	if result.Columns[0] != "field" || result.Columns[1] != "value" {
		t.Errorf("unexpected columns: %v", result.Columns)
	}
	// 哈希遍历无顺序，转成 map 验证
	got := make(map[string]string)
	for _, row := range result.Rows {
		got[row[0].(string)] = row[1].(string)
	}
	if got["name"] != "alice" || got["age"] != "30" {
		t.Errorf("expected name=alice, age=30, got %v", got)
	}
}

// TestRedisDriver_Query_LRANGE 测试列表命令（单列 "value"，多行）。
func TestRedisDriver_Query_LRANGE(t *testing.T) {
	mr := setupMiniredis(t)
	mr.RPush("mylist", "a", "b", "c")

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	result, err := drv.Query(ctx, "LRANGE mylist 0 -1")
	if err != nil {
		t.Fatalf("Query LRANGE failed: %v", err)
	}
	if result.RowCount != 3 {
		t.Errorf("expected 3 rows, got %d", result.RowCount)
	}
	if len(result.Columns) != 1 || result.Columns[0] != "value" {
		t.Errorf("unexpected columns: %v", result.Columns)
	}
	if result.Rows[0][0] != "a" || result.Rows[1][0] != "b" || result.Rows[2][0] != "c" {
		t.Errorf("expected [a,b,c], got %v", result.Rows)
	}
}

// TestRedisDriver_Query_SMEMBERS 测试集合命令（单列 "member"，多行）。
func TestRedisDriver_Query_SMEMBERS(t *testing.T) {
	mr := setupMiniredis(t)
	mr.SAdd("myset", "x", "y", "z")

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	result, err := drv.Query(ctx, "SMEMBERS myset")
	if err != nil {
		t.Fatalf("Query SMEMBERS failed: %v", err)
	}
	if result.RowCount != 3 {
		t.Errorf("expected 3 rows, got %d", result.RowCount)
	}
	if len(result.Columns) != 1 || result.Columns[0] != "member" {
		t.Errorf("unexpected columns: %v", result.Columns)
	}
}

// TestRedisDriver_Query_ZRANGE_WithScores 测试 ZRANGE ... WITHSCORES（两列 member/score）。
func TestRedisDriver_Query_ZRANGE_WithScores(t *testing.T) {
	mr := setupMiniredis(t)
	mr.ZAdd("myzset", 1, "one")
	mr.ZAdd("myzset", 2, "two")
	mr.ZAdd("myzset", 3, "three")

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	result, err := drv.Query(ctx, "ZRANGE myzset 0 -1 WITHSCORES")
	if err != nil {
		t.Fatalf("Query ZRANGE WITHSCORES failed: %v", err)
	}
	if result.RowCount != 3 {
		t.Errorf("expected 3 rows, got %d", result.RowCount)
	}
	if len(result.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %v", result.Columns)
	}
	if result.Columns[0] != "member" || result.Columns[1] != "score" {
		t.Errorf("unexpected columns: %v", result.Columns)
	}
}

// TestRedisDriver_Query_ZRANGE_NoWithScores 测试不带 WITHSCORES 的 ZRANGE（单列 "member"）。
func TestRedisDriver_Query_ZRANGE_NoWithScores(t *testing.T) {
	mr := setupMiniredis(t)
	mr.ZAdd("myzset2", 1, "one")
	mr.ZAdd("myzset2", 2, "two")

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	result, err := drv.Query(ctx, "ZRANGE myzset2 0 -1")
	if err != nil {
		t.Fatalf("Query ZRANGE failed: %v", err)
	}
	if len(result.Columns) != 1 || result.Columns[0] != "member" {
		t.Errorf("unexpected columns: %v", result.Columns)
	}
	if result.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", result.RowCount)
	}
}

// TestRedisDriver_Query_MGET 测试 MGET 批量查询（单列 "value"，多行）。
func TestRedisDriver_Query_MGET(t *testing.T) {
	mr := setupMiniredis(t)
	mr.Set("a", "1")
	mr.Set("b", "2")

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	result, err := drv.Query(ctx, "MGET a b")
	if err != nil {
		t.Fatalf("Query MGET failed: %v", err)
	}
	if len(result.Columns) != 1 || result.Columns[0] != "value" {
		t.Errorf("unexpected columns: %v", result.Columns)
	}
	if result.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", result.RowCount)
	}
}

// TestRedisDriver_Query_RAWINFO 测试 INFO/Time 等原始文本命令（单列 "result"）。
func TestRedisDriver_Query_RAWINFO(t *testing.T) {
	mr := setupMiniredis(t)

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	// PING 是原始文本类
	result, err := drv.Query(ctx, "PING")
	if err != nil {
		t.Fatalf("Query PING failed: %v", err)
	}
	if len(result.Columns) != 1 || result.Columns[0] != "result" {
		t.Errorf("unexpected columns for PING: %v", result.Columns)
	}
}

// TestRedisDriver_Query_EmptyCommand 测试空命令返回错误。
func TestRedisDriver_Query_EmptyCommand(t *testing.T) {
	mr := setupMiniredis(t)

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	_, err := drv.Query(ctx, "")
	if err == nil {
		t.Error("expected error for empty command, got nil")
	}

	_, err = drv.Query(ctx, "   ")
	if err == nil {
		t.Error("expected error for whitespace-only command, got nil")
	}
}

// TestRedisDriver_Query_NotConnected 测试未连接时查询返回错误。
func TestRedisDriver_Query_NotConnected(t *testing.T) {
	drv := NewRedisDriver()
	ctx := context.Background()

	_, err := drv.Query(ctx, "GET key")
	if err == nil {
		t.Error("expected error when not connected, got nil")
	}
}

// TestRedisDriver_UseDatabase 测试切换数据库。
func TestRedisDriver_UseDatabase(t *testing.T) {
	mr := setupMiniredis(t)

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
		Database:       "0",
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	// 切换到 db 1
	if err := drv.UseDatabase(ctx, "1"); err != nil {
		t.Errorf("UseDatabase failed: %v", err)
	}

	// 无效的数据库编号
	if err := drv.UseDatabase(ctx, "invalid"); err == nil {
		t.Error("expected error for invalid database number")
	}
}

// TestRedisDriver_GetDatabases 测试获取数据库列表。
func TestRedisDriver_GetDatabases(t *testing.T) {
	mr := setupMiniredis(t)

	// 单节点模式
	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	dbs, err := drv.GetDatabases(ctx)
	if err != nil {
		t.Fatalf("GetDatabases failed: %v", err)
	}
	if len(dbs) != 16 {
		t.Errorf("expected 16 databases, got %d", len(dbs))
	}
	if dbs[0] != "0" || dbs[15] != "15" {
		t.Errorf("unexpected database list: %v", dbs)
	}
}

// TestRedisDriver_GetDatabases_Cluster 测试集群模式数据库列表。
func TestRedisDriver_GetDatabases_Cluster(t *testing.T) {
	drv := &RedisDriver{
		config: DatasourceConfig{ConnectionMode: redisModeCluster},
	}

	ctx := context.Background()
	dbs, err := drv.GetDatabases(ctx)
	if err != nil {
		t.Fatalf("GetDatabases failed: %v", err)
	}
	if len(dbs) != 1 || dbs[0] != "0" {
		t.Errorf("expected only db 0 for cluster, got %v", dbs)
	}
}

// TestRedisDriver_GetColumns 测试获取列信息（Redis 无列概念，返回空）。
func TestRedisDriver_GetColumns(t *testing.T) {
	drv := NewRedisDriver()
	ctx := context.Background()

	cols, err := drv.GetColumns(ctx, "0", "somekey")
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}
	if len(cols) != 0 {
		t.Errorf("expected empty columns, got %v", cols)
	}
}

// TestRedisDriver_SupportsCancel 测试是否支持取消。
func TestRedisDriver_SupportsCancel(t *testing.T) {
	drv := NewRedisDriver()
	if drv.SupportsCancel() {
		t.Error("Redis driver should not support cancel")
	}
}

// TestRedisDriver_Close_NilClient 测试未连接时 Close 不报错。
func TestRedisDriver_Close_NilClient(t *testing.T) {
	drv := NewRedisDriver()
	if err := drv.Close(); err != nil {
		t.Errorf("Close on nil client should not error, got %v", err)
	}
}

// TestParseRedisCommand 测试 Redis 命令解析。
func TestParseRedisCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"simple command", "GET key", []string{"GET", "key"}, false},
		{"single arg", "PING", []string{"PING"}, false},
		{"multiple args", "SET key value", []string{"SET", "key", "value"}, false},
		{"double quoted", `SET key "hello world"`, []string{"SET", "key", "hello world"}, false},
		{"single quoted", "SET key 'hello world'", []string{"SET", "key", "hello world"}, false},
		{"escaped quote", `SET key "hello \"world\""`, []string{"SET", "key", `hello "world"`}, false},
		{"leading spaces", "   GET key", []string{"GET", "key"}, false},
		{"trailing spaces", "GET key   ", []string{"GET", "key"}, false},
		{"tabs", "GET\tkey", []string{"GET", "key"}, false},
		{"empty", "", nil, false},
		{"unterminated quote", `SET key "unclosed`, nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRedisCommand(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("arg[%d]: expected %q, got %q", i, tc.want[i], got[i])
				}
			}
		})
	}
}

// TestFormatRedisResult 测试 Redis 结果格式化。
func TestFormatRedisResult(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"nil", nil, "(nil)"},
		{"string", "hello", "hello"},
		{"bytes", []byte("world"), "world"},
		{"int64", int64(42), "42"},
		{"float64", float64(3.14), "3.14"},
		{"bool true", true, "1"},
		{"bool false", false, "0"},
		{"int64 zero", int64(0), "0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatRedisResult(tc.input)
			if got != tc.want {
				t.Errorf("formatRedisResult(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestFormatRedisResult_Array 测试数组结果格式化。
func TestFormatRedisResult_Array(t *testing.T) {
	input := []interface{}{"a", "b", "c"}
	got := formatRedisResult(input)
	if got != `["a","b","c"]` {
		t.Errorf("expected [\"a\",\"b\",\"c\"], got %s", got)
	}
}

// TestIsRedisReadOnlyCommand 测试只读命令判断。
func TestIsRedisReadOnlyCommand(t *testing.T) {
	readOnly := []string{"GET", "MGET", "KEYS", "SCAN", "TTL", "EXISTS", "TYPE", "LRANGE", "HGET", "HGETALL", "ZRANGE", "INFO", "PING"}
	for _, cmd := range readOnly {
		if !IsRedisReadOnlyCommand(cmd) {
			t.Errorf("expected %q to be read-only", cmd)
		}
	}

	writeCmds := []string{"SET", "DEL", "INCR", "DECR", "LPUSH", "RPUSH", "SADD", "HSET", "ZADD", "EXPIRE", "FLUSHDB"}
	for _, cmd := range writeCmds {
		if IsRedisReadOnlyCommand(cmd) {
			t.Errorf("expected %q to NOT be read-only", cmd)
		}
	}
}

// TestParseRedisDB 测试 Redis 数据库编号解析。
func TestParseRedisDB(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"0", 0},
		{"5", 5},
		{"15", 15},
		{"invalid", 0},
		{"-1", 0},
	}

	for _, tc := range tests {
		got := parseRedisDB(tc.input)
		if got != tc.want {
			t.Errorf("parseRedisDB(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// TestGetConfigStringSlice 测试从 Config map 解析字符串切片。
func TestGetConfigStringSlice(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
		key    string
		want   []string
	}{
		{
			name:   "comma separated string",
			config: map[string]interface{}{"addrs": "host1:6379,host2:6379,host3:6379"},
			key:    "addrs",
			want:   []string{"host1:6379", "host2:6379", "host3:6379"},
		},
		{
			name:   "string slice",
			config: map[string]interface{}{"addrs": []string{"a:1", "b:2"}},
			key:    "addrs",
			want:   []string{"a:1", "b:2"},
		},
		{
			name:   "interface slice",
			config: map[string]interface{}{"addrs": []interface{}{"a:1", "b:2"}},
			key:    "addrs",
			want:   []string{"a:1", "b:2"},
		},
		{
			name:   "missing key",
			config: map[string]interface{}{},
			key:    "addrs",
			want:   nil,
		},
		{
			name:   "nil config",
			config: nil,
			key:    "addrs",
			want:   nil,
		},
		{
			name:   "with spaces",
			config: map[string]interface{}{"addrs": "host1:6379, host2:6379 , host3:6379"},
			key:    "addrs",
			want:   []string{"host1:6379", "host2:6379", "host3:6379"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := getConfigStringSlice(tc.config, tc.key)
			if len(got) != len(tc.want) {
				t.Errorf("expected %v, got %v", tc.want, got)
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("item[%d]: expected %q, got %q", i, tc.want[i], got[i])
				}
			}
		})
	}
}

// TestRedisDriver_Connect_ClusterMissingAddrs 测试集群模式缺少地址时报错。
func TestRedisDriver_Connect_ClusterMissingAddrs(t *testing.T) {
	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		ConnectionMode: redisModeCluster,
	}

	ctx := context.Background()
	err := drv.Connect(ctx, config)
	if err == nil {
		t.Fatal("expected error for cluster mode without cluster_addrs")
	}
}

// TestRedisDriver_Connect_SentinelMissingConfig 测试哨兵模式缺少配置时报错。
func TestRedisDriver_Connect_SentinelMissingConfig(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
	}{
		{"missing sentinel_addrs", map[string]interface{}{"master_name": "mymaster"}},
		{"missing master_name", map[string]interface{}{"sentinel_addrs": "host1:26379"}},
		{"missing both", map[string]interface{}{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			drv := NewRedisDriver()
			config := DatasourceConfig{
				Type:           "redis",
				ConnectionMode: redisModeSentinel,
				Config:         tc.config,
			}

			ctx := context.Background()
			err := drv.Connect(ctx, config)
			if err == nil {
				t.Fatal("expected error for sentinel mode with missing config")
			}
		})
	}
}

// TestRedisDriver_QueryWithDB 测试 QueryWithDB 切换数据库。
func TestRedisDriver_QueryWithDB(t *testing.T) {
	mr := setupMiniredis(t)

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
		Database:       "0",
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	// 在 db 0 设置值
	_, err := drv.Query(ctx, "SET key0 val0")
	if err != nil {
		t.Fatalf("SET failed: %v", err)
	}

	// 在 db 1 设置值
	if err := drv.UseDatabase(ctx, "1"); err != nil {
		t.Fatalf("UseDatabase failed: %v", err)
	}
	_, err = drv.Query(ctx, "SET key1 val1")
	if err != nil {
		t.Fatalf("SET on db1 failed: %v", err)
	}

	// 通过 QueryWithDB 查询 db 0 的值
	result, err := drv.QueryWithDB(ctx, "GET key0", "0")
	if err != nil {
		t.Fatalf("QueryWithDB failed: %v", err)
	}
	if result.Rows[0][0] != "val0" {
		t.Errorf("expected 'val0', got %v", result.Rows[0][0])
	}

	// 通过 QueryWithDB 查询 db 1 的值
	result, err = drv.QueryWithDB(ctx, "GET key1", "1")
	if err != nil {
		t.Fatalf("QueryWithDB db1 failed: %v", err)
	}
	if result.Rows[0][0] != "val1" {
		t.Errorf("expected 'val1', got %v", result.Rows[0][0])
	}
}

// TestRedisDriver_GetTables 测试获取 key 列表（SCAN）。
func TestRedisDriver_GetTables(t *testing.T) {
	mr := setupMiniredis(t)
	mr.Set("key1", "val1")
	mr.Set("key2", "val2")

	drv := NewRedisDriver()
	config := DatasourceConfig{
		Type:           "redis",
		Host:           mr.Host(),
		Port:           miniredisPort(t, mr),
		ConnectionMode: redisModeSingle,
	}

	ctx := context.Background()
	if err := drv.Connect(ctx, config); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer drv.Close()

	tables, err := drv.GetTables(ctx, "")
	if err != nil {
		t.Fatalf("GetTables failed: %v", err)
	}
	if len(tables) != 2 {
		t.Errorf("expected 2 keys, got %d", len(tables))
	}

	keySet := map[string]bool{}
	for _, tbl := range tables {
		keySet[tbl.Name] = true
	}
	if !keySet["key1"] || !keySet["key2"] {
		t.Errorf("expected keys key1 and key2, got %v", tables)
	}
}
