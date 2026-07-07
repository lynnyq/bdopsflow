package driver

import (
	"context"
	"fmt"
	"testing"
	"time"

	gohive "github.com/beltran/gohive"
)

// === 纯函数测试 ===

// TestConvertMySQLValue 测试 MySQL 值转换函数
// 覆盖 mysql.go:236 的 convertMySQLValue（当前 0%）
func TestConvertMySQLValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"nil 值", nil, nil},
		{"数字字节切片", []byte("123"), "123"},
		{"非数字字节切片", []byte("hello"), "hello"},
		{"空字节切片", []byte(""), ""},
		{"字符串", "test", "test"},
		{"整数", 42, 42},
		{"int64", int64(100), int64(100)},
		{"float64", 3.14, 3.14},
		{"bool", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertMySQLValue(tt.input)
			if got != tt.expected {
				t.Errorf("convertMySQLValue(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestBuildTrinoQualifiedName 测试 Trino 限定名构建
// 覆盖 trino.go:312 的 buildTrinoQualifiedName（当前 0%）
func TestBuildTrinoQualifiedName(t *testing.T) {
	tests := []struct {
		name     string
		database string
		table    string
		expected string
	}{
		{"catalog.schema.table", "hive.default", "users", `"hive"."default"."users"`},
		{"catalog.schema 无表", "hive.default", "", `"hive"."default"`},
		{"schema.table 无 catalog", "default", "users", `"default"."users"`},
		{"仅 schema", "default", "", `"default"`},
		{"空 database 空 table", "", "", `""`},
		{"含双引号的标识符", `ca"t.sch"ema`, "tab", `"ca""t"."sch""ema"."tab"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTrinoQualifiedName(tt.database, tt.table)
			if got != tt.expected {
				t.Errorf("buildTrinoQualifiedName(%q, %q) = %q, want %q", tt.database, tt.table, got, tt.expected)
			}
		})
	}
}

// TestNormalizeTrinoSQL 测试 Trino SQL 规范化（反引号转双引号）
// 覆盖 trino.go:326 的 normalizeTrinoSQL（当前 0%）
func TestNormalizeTrinoSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"无反引号", "SELECT * FROM t", "SELECT * FROM t"},
		{"反引号转双引号", "SELECT `col` FROM `t`", `SELECT "col" FROM "t"`},
		{"字符串字面量中的反引号保留", "SELECT 'a`b' FROM t", "SELECT 'a`b' FROM t"},
		{"混合场景", "SELECT `col`, 'str`str' FROM `t`", "SELECT \"col\", 'str`str' FROM \"t\""},
		{"空字符串", "", ""},
		{"仅反引号", "``", `""`},
		{"多个反引号", "SELECT `a`, `b` FROM `c`", `SELECT "a", "b" FROM "c"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTrinoSQL(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeTrinoSQL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestEscapeSQLiteIdentifier 测试 SQLite 标识符转义
// 覆盖 sqlite.go:160 的 escapeSQLiteIdentifier（当前 0%）
func TestEscapeSQLiteIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"普通名称", "normal", "normal"},
		{"含双引号", `with"quote`, `with""quote`},
		{"空字符串", "", ""},
		{"多个双引号", `a"b"c`, `a""b""c`},
		{"仅双引号", `"`, `""`},
		{"Unicode", "用户表", "用户表"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeSQLiteIdentifier(tt.input)
			if got != tt.expected {
				t.Errorf("escapeSQLiteIdentifier(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestExtractQueryTimeout 测试从 context 提取超时
// 覆盖 hive_pool.go:513 的 extractQueryTimeout（当前 0%）
func TestExtractQueryTimeout(t *testing.T) {
	t.Run("有 deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		sec, sql := extractQueryTimeout(ctx, "SET timeout=", 5*time.Second)
		if sec <= 0 {
			t.Errorf("应返回正数秒数, got %d", sec)
		}
		if sql == "" {
			t.Error("应返回非空 SQL")
		}
		expectedPrefix := "SET timeout="
		if len(sql) < len(expectedPrefix) || sql[:len(expectedPrefix)] != expectedPrefix {
			t.Errorf("SQL 应以 %q 开头, got %q", expectedPrefix, sql)
		}
	})

	t.Run("无 deadline", func(t *testing.T) {
		sec, sql := extractQueryTimeout(context.Background(), "SET timeout=", 5*time.Second)
		if sec != 0 {
			t.Errorf("无 deadline 应返回 0 秒, got %d", sec)
		}
		if sql != "" {
			t.Errorf("无 deadline 应返回空 SQL, got %q", sql)
		}
	})

	t.Run("已过期 deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), -1*time.Second)
		defer cancel()

		sec, sql := extractQueryTimeout(ctx, "SET timeout=", 5*time.Second)
		if sec != 0 {
			t.Errorf("已过期 deadline 应返回 0 秒, got %d", sec)
		}
		if sql != "" {
			t.Errorf("已过期 deadline 应返回空 SQL, got %q", sql)
		}
	})

	t.Run("不同 SET 前缀", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, sql := extractQueryTimeout(ctx, "SET hive.query.timeout=", 5*time.Second)
		expectedPrefix := "SET hive.query.timeout="
		if len(sql) < len(expectedPrefix) || sql[:len(expectedPrefix)] != expectedPrefix {
			t.Errorf("SQL 应以 %q 开头, got %q", expectedPrefix, sql)
		}
	})
}

// === nil db 路径测试 ===

// TestNilDB_GetPoolConfig 测试各驱动在 db 为 nil 时 GetPoolConfig 返回默认配置
// 覆盖 doris/mysql/sqlite/starrocks/trino 的 GetPoolConfig nil 路径
func TestNilDB_GetPoolConfig(t *testing.T) {
	drivers := []struct {
		name   string
		driver PoolConfigUpdater
	}{
		{"mysql", &MySQLDriver{}},
		{"doris", &DorisDriver{}},
		{"sqlite", &SQLiteDriver{}},
		{"starrocks", &StarRocksDriver{}},
		{"trino", &TrinoDriver{}},
		{"hive", &HiveDriver{}},
		{"kyuubi", &KyuubiDriver{}},
		{"spark", &SparkDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			cfg := d.driver.GetPoolConfig()
			// nil db 应返回 DefaultPoolConfig
			if cfg.MaxOpen != 5 {
				t.Errorf("GetPoolConfig().MaxOpen = %d, want 5 (default)", cfg.MaxOpen)
			}
			if cfg.MinIdle != 2 {
				t.Errorf("GetPoolConfig().MinIdle = %d, want 2 (default)", cfg.MinIdle)
			}
		})
	}
}

// TestNilDB_GetPoolStats 测试各驱动在 db 为 nil 时 GetPoolStats 返回零值
func TestNilDB_GetPoolStats(t *testing.T) {
	drivers := []struct {
		name   string
		driver PoolConfigUpdater
	}{
		{"mysql", &MySQLDriver{}},
		{"doris", &DorisDriver{}},
		{"sqlite", &SQLiteDriver{}},
		{"starrocks", &StarRocksDriver{}},
		{"trino", &TrinoDriver{}},
		{"hive", &HiveDriver{}},
		{"kyuubi", &KyuubiDriver{}},
		{"spark", &SparkDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			oc, ic, iu, mo := d.driver.GetPoolStats()
			if oc != 0 || ic != 0 || iu != 0 || mo != 0 {
				t.Errorf("GetPoolStats() = (%d,%d,%d,%d), want (0,0,0,0)", oc, ic, iu, mo)
			}
		})
	}
}

// TestNilDB_UpdatePoolConfig 测试各驱动在 db 为 nil 时 UpdatePoolConfig 不 panic
func TestNilDB_UpdatePoolConfig(t *testing.T) {
	drivers := []struct {
		name   string
		driver PoolConfigUpdater
	}{
		{"mysql", &MySQLDriver{}},
		{"doris", &DorisDriver{}},
		{"sqlite", &SQLiteDriver{}},
		{"starrocks", &StarRocksDriver{}},
		{"trino", &TrinoDriver{}},
		{"hive", &HiveDriver{}},
		{"kyuubi", &KyuubiDriver{}},
		{"spark", &SparkDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			// 不应 panic
			d.driver.UpdatePoolConfig(PoolConfig{
				MaxOpen:     10,
				MinIdle:     5,
				MaxLifetime: 1 * time.Hour,
			})
		})
	}
}

// TestNilDB_Ping 测试各驱动在未连接时 Ping 返回错误
func TestNilDB_Ping(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"mysql", &MySQLDriver{}},
		{"doris", &DorisDriver{}},
		{"sqlite", &SQLiteDriver{}},
		{"starrocks", &StarRocksDriver{}},
		{"trino", &TrinoDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			err := d.driver.Ping(context.Background())
			if err == nil {
				t.Error("Ping 应在未连接时返回错误")
			}
		})
	}
}

// TestNilDB_TestConnection 测试 MySQL/SQLite 未连接时 TestConnection 返回错误
func TestNilDB_TestConnection(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"mysql", &MySQLDriver{}},
		{"sqlite", &SQLiteDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			err := d.driver.TestConnection(context.Background())
			if err == nil {
				t.Error("TestConnection 应在未连接时返回错误")
			}
		})
	}
}

// TestNilDB_Close 测试 MySQL/SQLite 未连接时 Close 不报错
func TestNilDB_Close(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"mysql", &MySQLDriver{}},
		{"sqlite", &SQLiteDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			if err := d.driver.Close(); err != nil {
				t.Errorf("Close 未连接时不应报错, got: %v", err)
			}
		})
	}
}

// TestNilDB_SupportsCancel 测试各驱动的 SupportsCancel
func TestNilDB_SupportsCancel(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
		want   bool
	}{
		{"mysql", &MySQLDriver{}, true},
		{"sqlite", &SQLiteDriver{}, true},
		{"doris", &DorisDriver{}, true},
		{"starrocks", &StarRocksDriver{}, true},
		{"trino", &TrinoDriver{}, true},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			if got := d.driver.SupportsCancel(); got != d.want {
				t.Errorf("SupportsCancel() = %v, want %v", got, d.want)
			}
		})
	}
}

// TestNilDB_UseDatabaseEmpty 测试空 database 参数返回 nil
func TestNilDB_UseDatabaseEmpty(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"doris", &DorisDriver{}},
		{"starrocks", &StarRocksDriver{}},
		{"trino", &TrinoDriver{}},
		{"rqlite", &RqliteDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			err := d.driver.UseDatabase(context.Background(), "")
			if err != nil {
				t.Errorf("UseDatabase 空参数应返回 nil, got: %v", err)
			}
		})
	}
}

// TestNilDB_UseDatabaseNonEmpty 测试未连接时非空 database 返回错误
func TestNilDB_UseDatabaseNonEmpty(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"doris", &DorisDriver{}},
		{"starrocks", &StarRocksDriver{}},
		{"trino", &TrinoDriver{}},
		{"mysql", &MySQLDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			err := d.driver.UseDatabase(context.Background(), "test_db")
			if err == nil {
				t.Error("UseDatabase 非空参数在未连接时应返回错误")
			}
		})
	}
}

// TestNilDB_GetDatabases 测试未连接时 GetDatabases 返回错误
// 注意：SQLite 和 Rqlite 的 GetDatabases 不依赖连接，始终返回固定值，因此不在此测试中
func TestNilDB_GetDatabases(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"mysql", &MySQLDriver{}},
		{"doris", &DorisDriver{}},
		{"starrocks", &StarRocksDriver{}},
		{"trino", &TrinoDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			_, err := d.driver.GetDatabases(context.Background())
			if err == nil {
				t.Error("GetDatabases 应在未连接时返回错误")
			}
		})
	}
}

// TestNilDB_GetTables 测试未连接时 GetTables 返回错误
func TestNilDB_GetTables(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"mysql", &MySQLDriver{}},
		{"doris", &DorisDriver{}},
		{"sqlite", &SQLiteDriver{}},
		{"starrocks", &StarRocksDriver{}},
		{"trino", &TrinoDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			_, err := d.driver.GetTables(context.Background(), "test_db")
			if err == nil {
				t.Error("GetTables 应在未连接时返回错误")
			}
		})
	}
}

// TestNilDB_GetColumns 测试未连接时 GetColumns 返回错误
func TestNilDB_GetColumns(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"mysql", &MySQLDriver{}},
		{"doris", &DorisDriver{}},
		{"sqlite", &SQLiteDriver{}},
		{"starrocks", &StarRocksDriver{}},
		{"trino", &TrinoDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			_, err := d.driver.GetColumns(context.Background(), "test_db", "test_table")
			if err == nil {
				t.Error("GetColumns 应在未连接时返回错误")
			}
		})
	}
}

// TestNilDB_QueryWithDB 测试未连接时 QueryWithDB 返回错误
func TestNilDB_QueryWithDB(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"mysql", &MySQLDriver{}},
		{"doris", &DorisDriver{}},
		{"sqlite", &SQLiteDriver{}},
		{"starrocks", &StarRocksDriver{}},
		{"trino", &TrinoDriver{}},
		{"rqlite", &RqliteDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			_, err := d.driver.QueryWithDB(context.Background(), "SELECT 1", "")
			if err == nil {
				t.Error("QueryWithDB 应在未连接时返回错误")
			}
		})
	}
}

// TestNilDB_TryQueryWithDB 测试未连接时 TryQueryWithDB 返回错误
func TestNilDB_TryQueryWithDB(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"mysql", &MySQLDriver{}},
		{"doris", &DorisDriver{}},
		{"sqlite", &SQLiteDriver{}},
		{"starrocks", &StarRocksDriver{}},
		{"trino", &TrinoDriver{}},
		{"rqlite", &RqliteDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			_, err := d.driver.TryQueryWithDB(context.Background(), "SELECT 1", "")
			if err == nil {
				t.Error("TryQueryWithDB 应在未连接时返回错误")
			}
		})
	}
}

// === Rqlite 特有方法测试 ===

// TestRqliteDriver_GetDatabasesAlwaysReturnsMain 测试 rqlite GetDatabases 固定返回 main
// 覆盖 rqlite_driver.go:182 的 GetDatabases（当前 0%）
func TestRqliteDriver_GetDatabasesAlwaysReturnsMain(t *testing.T) {
	d := &RqliteDriver{}
	dbs, err := d.GetDatabases(context.Background())
	if err != nil {
		t.Fatalf("GetDatabases 不应返回错误: %v", err)
	}
	if len(dbs) != 1 || dbs[0] != "main" {
		t.Errorf("GetDatabases() = %v, want [main]", dbs)
	}
}

// TestSQLiteDriver_GetDatabases 测试 SQLite GetDatabases 返回 path
// 覆盖 sqlite.go:106 的 GetDatabases（当前 0%）
func TestSQLiteDriver_GetDatabases(t *testing.T) {
	d := &SQLiteDriver{config: DatasourceConfig{Path: "/tmp/test.db"}}
	dbs, err := d.GetDatabases(context.Background())
	if err != nil {
		t.Fatalf("GetDatabases 不应返回错误: %v", err)
	}
	if len(dbs) != 1 || dbs[0] != "/tmp/test.db" {
		t.Errorf("GetDatabases() = %v, want [/tmp/test.db]", dbs)
	}
}

// === MarkUnhealthy / IsUnhealthy 测试 ===

// TestMarkUnhealthyAndIsUnhealthy 测试 Hive/Kyuubi/Spark 的健康状态管理
// 覆盖 hive.go:196 MarkUnhealthy、kyuubi.go:177-184、spark.go:177-184
func TestMarkUnhealthyAndIsUnhealthy(t *testing.T) {
	drivers := []struct {
		name   string
		driver UnhealthyChecker
	}{
		{"hive", &HiveDriver{}},
		{"kyuubi", &KyuubiDriver{}},
		{"spark", &SparkDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			// 初始状态应为健康
			if d.driver.IsUnhealthy() {
				t.Error("初始状态不应为不健康")
			}

			// 标记为不健康
			d.driver.MarkUnhealthy()
			if !d.driver.IsUnhealthy() {
				t.Error("MarkUnhealthy 后应为不健康状态")
			}
		})
	}
}

// TestHiveDriver_PingUnhealthy 测试 Hive/Kyuubi/Spark 在不健康状态时 Ping 返回错误
func TestPoolDriver_PingUnhealthy(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"hive", &HiveDriver{}},
		{"kyuubi", &KyuubiDriver{}},
		{"spark", &SparkDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			// 使用类型断言标记不健康
			if uc, ok := d.driver.(UnhealthyChecker); ok {
				uc.MarkUnhealthy()
			}
			err := d.driver.Ping(context.Background())
			if err == nil {
				t.Error("不健康状态时 Ping 应返回错误")
			}
		})
	}
}

// TestPoolDriver_PingWithoutConnect 测试 Hive/Kyuubi/Spark 未连接时 Ping 返回错误
func TestPoolDriver_PingWithoutConnect(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"hive", &HiveDriver{}},
		{"kyuubi", &KyuubiDriver{}},
		{"spark", &SparkDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			err := d.driver.Ping(context.Background())
			if err == nil {
				t.Error("未连接时 Ping 应返回错误")
			}
		})
	}
}

// === Hive 连接池辅助函数测试 ===

// TestHiveConnPool_TryAcquireFromEmptyPool 测试空池 tryAcquire 创建新连接
// 覆盖 hive_pool.go:113 的 tryAcquire（当前 0%）
func TestHiveConnPool_TryAcquireFromEmptyPool(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        2,
		MinIdle:        0,
		MaxLifetime:    0,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	pc, err := pool.tryAcquire(context.Background())
	if err != nil {
		t.Fatalf("tryAcquire 失败: %v", err)
	}
	if pc == nil {
		t.Fatal("tryAcquire 返回 nil")
	}

	oc, _, iu, _ := pool.stats()
	if oc != 1 {
		t.Errorf("openCount = %d, want 1", oc)
	}
	if iu != 1 {
		t.Errorf("inUseCount = %d, want 1", iu)
	}
}

// TestHiveConnPool_TryAcquireFromPooledConn 测试从池中获取已有连接
func TestHiveConnPool_TryAcquireFromPooledConn(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        0,
		MaxLifetime:    0,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 预热一个连接
	pool.put(nil, "db1")

	// tryAcquire 应从池中获取
	pc, err := pool.tryAcquire(context.Background())
	if err != nil {
		t.Fatalf("tryAcquire 失败: %v", err)
	}
	if pc == nil {
		t.Fatal("tryAcquire 返回 nil")
	}
	if pc.database != "db1" {
		t.Errorf("database = %q, want db1", pc.database)
	}

	oc, ic, iu, _ := pool.stats()
	if oc != 1 {
		t.Errorf("openCount = %d, want 1", oc)
	}
	if ic != 0 {
		t.Errorf("idleCount = %d, want 0 (已被取出)", ic)
	}
	if iu != 1 {
		t.Errorf("inUseCount = %d, want 1", iu)
	}
}

// TestHiveConnPool_TryAcquireFullPool 测试池满时 tryAcquire 返回错误
func TestHiveConnPool_TryAcquireFullPool(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        1,
		MinIdle:        0,
		MaxLifetime:    0,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 获取唯一连接
	pc1, err := pool.tryAcquire(context.Background())
	if err != nil {
		t.Fatalf("第一次 tryAcquire 失败: %v", err)
	}

	// 池已满，第二次应失败
	_, err = pool.tryAcquire(context.Background())
	if err == nil {
		t.Error("池满时 tryAcquire 应返回错误")
	}

	// 释放第一个连接
	pool.release(pc1)

	// 现在应成功（从池中获取）
	pc2, err := pool.tryAcquire(context.Background())
	if err != nil {
		t.Fatalf("释放后 tryAcquire 失败: %v", err)
	}
	pool.release(pc2)
}

// TestHiveConnPool_TryAcquireCancelledContext 测试已取消 context
func TestHiveConnPool_TryAcquireCancelledContext(t *testing.T) {
	cfg := DefaultPoolConfig()
	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pool.tryAcquire(ctx)
	if err == nil {
		t.Error("已取消 context 的 tryAcquire 应返回错误")
	}
}

// TestHiveConnPool_TryCreateConnError 测试创建连接失败
// 覆盖 hive_pool.go:144 的 tryCreateConn（当前 0%）
func TestHiveConnPool_TryCreateConnError(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        0,
		MaxLifetime:    0,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, fmt.Errorf("connection failed")
	})
	defer pool.close()

	_, err := pool.tryCreateConn(context.Background(), cfg)
	if err == nil {
		t.Error("tryCreateConn 应在 createConn 失败时返回错误")
	}

	// openCount 不应增加
	oc, _, _, _ := pool.stats()
	if oc != 0 {
		t.Errorf("失败后 openCount = %d, want 0", oc)
	}
}

// TestHiveConnPool_TryCreateConnAtCapacity 测试达到上限时 tryCreateConn 返回错误
func TestHiveConnPool_TryCreateConnAtCapacity(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        1,
		MinIdle:        0,
		MaxLifetime:    0,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 预热达到上限
	pool.put(nil, "db1")

	// 尝试创建应失败（已达上限）
	_, err := pool.tryCreateConn(context.Background(), cfg)
	if err == nil {
		t.Error("tryCreateConf 达到上限应返回错误")
	}
}

// TestHiveConnPool_AcquireWithCancelledContext 测试 acquire 已取消 context
func TestHiveConnPool_AcquireWithCancelledContext(t *testing.T) {
	cfg := DefaultPoolConfig()
	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pool.acquire(ctx)
	if err == nil {
		t.Error("已取消 context 的 acquire 应返回错误")
	}
}

// TestHiveConnPool_AcquireTimeoutWhenFull 测试池满时 acquire 超时
func TestHiveConnPool_AcquireTimeoutWhenFull(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        1,
		MinIdle:        0,
		MaxLifetime:    0,
		AcquireTimeout: 100 * time.Millisecond,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 获取唯一连接
	pc1, err := pool.acquire(context.Background())
	if err != nil {
		t.Fatalf("第一次 acquire 失败: %v", err)
	}

	// 第二次应超时
	start := time.Now()
	_, err = pool.acquire(context.Background())
	elapsed := time.Since(start)
	if err == nil {
		t.Error("池满时 acquire 应超时返回错误")
	}
	if elapsed < 90*time.Millisecond {
		t.Errorf("应等待至少 AcquireTimeout, elapsed=%v", elapsed)
	}

	pool.release(pc1)
}

// TestHiveConnPool_DiscardNil 测试 discard nil 不 panic
func TestHiveConnPool_DiscardNil(t *testing.T) {
	cfg := DefaultPoolConfig()
	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// discard nil 不应 panic
	pool.discard(nil)
}

// TestHiveConnPool_ReleaseNil 测试 release nil 不 panic
func TestHiveConnPool_ReleaseNil(t *testing.T) {
	cfg := DefaultPoolConfig()
	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// release nil 不应 panic
	pool.release(nil)
}

// TestHiveConnPool_ReleaseToClosedPool 测试向已关闭池 release 不 panic
func TestHiveConnPool_ReleaseToClosedPool(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        2,
		MinIdle:        0,
		MaxLifetime:    0,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})

	// 获取连接
	pc, err := pool.acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire 失败: %v", err)
	}

	// 关闭池
	pool.close()

	// 向已关闭池 release 不应 panic
	pool.release(pc)
}

// TestHiveConnPool_UpdateConfigEdgeCases 测试 UpdateConfig 边界场景
func TestHiveConnPool_UpdateConfigEdgeCases(t *testing.T) {
	cfg := DefaultPoolConfig()
	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	tests := []struct {
		name    string
		newCfg  PoolConfig
		wantMax int
		wantMin int
	}{
		{
			name:    "zero MaxOpen defaults to 5",
			newCfg:  PoolConfig{MaxOpen: 0, MinIdle: 1, AcquireTimeout: 5 * time.Second},
			wantMax: 5,
			wantMin: 1,
		},
		{
			name:    "negative MinIdle defaults to 0",
			newCfg:  PoolConfig{MaxOpen: 3, MinIdle: -1, AcquireTimeout: 5 * time.Second},
			wantMax: 3,
			wantMin: 0,
		},
		{
			name:    "zero AcquireTimeout defaults to 30s",
			newCfg:  PoolConfig{MaxOpen: 5, MinIdle: 2, AcquireTimeout: 0},
			wantMax: 5,
			wantMin: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool.UpdateConfig(tt.newCfg)
			gotCfg := pool.GetConfig()
			if gotCfg.MaxOpen != tt.wantMax {
				t.Errorf("MaxOpen = %d, want %d", gotCfg.MaxOpen, tt.wantMax)
			}
			if gotCfg.MinIdle != tt.wantMin {
				t.Errorf("MinIdle = %d, want %d", gotCfg.MinIdle, tt.wantMin)
			}
			if gotCfg.AcquireTimeout <= 0 {
				t.Errorf("AcquireTimeout = %v, want positive", gotCfg.AcquireTimeout)
			}
		})
	}
}

// TestHiveConnPool_StatsDefenseClamping 测试 stats 防御性校验
func TestHiveConnPool_StatsDefenseClamping(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        0,
		MaxLifetime:    0,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 通过 put 增加 openCount
	pool.put(nil, "db1")
	pool.put(nil, "db2")

	// 手动设置 inUseCount 为负数（模拟异常状态）
	pool.inUseCount.Store(-5)

	oc, ic, iu, _ := pool.stats()
	// 防御性校验：iu 不应小于 0
	if iu < 0 {
		t.Errorf("inUse 不应为负数, got %d", iu)
	}
	// iu 被 clamp 到 [0, oc]
	if iu > oc {
		t.Errorf("inUse 不应大于 openCount, iu=%d oc=%d", iu, oc)
	}
	// ic = oc - iu, 不应小于 0
	if ic < 0 {
		t.Errorf("idleCount 不应为负数, got %d", ic)
	}
}

// === ApplyLimitToSQL 补充场景 ===

// TestApplyLimitToSQL_AdditionalScenarios 补充 ApplyLimitToSQL 更多场景
func TestApplyLimitToSQL_AdditionalScenarios(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		limit    int
		dsType   string
		expected string
	}{
		// 小写 select 也能识别
		{"小写 select", "select * from t", 100, "mysql", "select * from t LIMIT 100"},
		// WITH 后跟换行
		{"WITH 多行", "WITH cte AS (\nSELECT 1\n) SELECT * FROM cte", 50, "mysql", "WITH cte AS (\nSELECT 1\n) SELECT * FROM cte LIMIT 50"},
		// 已有 LIMIT 但无法提取值（如 LIMIT ?）
		// extractUserLimit 对非数字返回 0，会追加 LIMIT
		// 用户 LIMIT 小于系统限制，保持原样
		{"用户LIMIT小于系统", "SELECT * FROM t LIMIT 10", 1000, "hive", "SELECT * FROM t LIMIT 10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyLimitToSQL(tt.sql, tt.limit, tt.dsType)
			if got != tt.expected {
				t.Errorf("ApplyLimitToSQL(%q, %d, %q) = %q, want %q", tt.sql, tt.limit, tt.dsType, got, tt.expected)
			}
		})
	}
}

// TestReplaceLimitValue_NoMatch 测试 replaceLimitValue 无匹配时的行为
func TestReplaceLimitValue_NoMatch(t *testing.T) {
	// 当 oldLimit 不匹配时，SQL 应保持不变
	got := replaceLimitValue("SELECT * FROM t LIMIT 100", 2000, 500)
	if got != "SELECT * FROM t LIMIT 100" {
		t.Errorf("replaceLimitValue 无匹配时应保持原样, got %q", got)
	}
}

// TestReplaceLimitValue_ExactMatch 测试精确匹配替换
func TestReplaceLimitValue_ExactMatch(t *testing.T) {
	got := replaceLimitValue("SELECT * FROM t LIMIT 2000", 2000, 500)
	if got != "SELECT * FROM t LIMIT 500" {
		t.Errorf("replaceLimitValue 精确匹配 = %q, want %q", got, "SELECT * FROM t LIMIT 500")
	}
}

// === Hive/Kyuubi/Spark QueryWithDB 无连接池测试 ===

// TestPoolDriver_QueryWithDBWithoutConnect 测试 Hive/Kyuubi/Spark 未连接时 QueryWithDB 返回错误
func TestPoolDriver_QueryWithDBWithoutConnect(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"hive", &HiveDriver{}},
		{"kyuubi", &KyuubiDriver{}},
		{"spark", &SparkDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			_, err := d.driver.QueryWithDB(context.Background(), "SELECT 1", "")
			if err == nil {
				t.Error("QueryWithDB 应在未连接时返回错误")
			}
		})
	}
}

// TestPoolDriver_TryQueryWithDBWithoutConnect 测试未连接时 TryQueryWithDB 返回错误
func TestPoolDriver_TryQueryWithDBWithoutConnect(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"hive", &HiveDriver{}},
		{"kyuubi", &KyuubiDriver{}},
		{"spark", &SparkDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			_, err := d.driver.TryQueryWithDB(context.Background(), "SELECT 1", "")
			if err == nil {
				t.Error("TryQueryWithDB 应在未连接时返回错误")
			}
		})
	}
}

// TestPoolDriver_TestConnectionWithoutConnect 测试未连接时 TestConnection 返回错误
func TestPoolDriver_TestConnectionWithoutConnect(t *testing.T) {
	drivers := []struct {
		name   string
		driver Driver
	}{
		{"hive", &HiveDriver{}},
		{"kyuubi", &KyuubiDriver{}},
		{"spark", &SparkDriver{}},
	}

	for _, d := range drivers {
		t.Run(d.name, func(t *testing.T) {
			err := d.driver.TestConnection(context.Background())
			if err == nil {
				t.Error("TestConnection 应在未连接时返回错误")
			}
		})
	}
}

// === Trino buildDSN 补充场景 ===

// TestTrinoDriver_buildDSN_NoAuth 测试 Trino DSN 无认证场景
func TestTrinoDriver_buildDSN_NoAuth(t *testing.T) {
	d := &TrinoDriver{config: DatasourceConfig{
		Host:     "localhost",
		Port:     8080,
		Database: "catalog1",
	}}

	dsn := d.buildDSN(8080)
	if dsn == "" {
		t.Error("buildDSN 不应返回空字符串")
	}
}

// TestTrinoDriver_buildDSN_WithPassword 测试 Trino DSN 带密码
func TestTrinoDriver_buildDSN_WithPassword(t *testing.T) {
	d := &TrinoDriver{config: DatasourceConfig{
		Host:     "localhost",
		Port:     8080,
		Username: "user",
		Password: "pass",
		Database: "catalog1.schema1",
	}}

	dsn := d.buildDSN(8080)
	if dsn == "" {
		t.Error("buildDSN 不应返回空字符串")
	}
}

// TestTrinoDriver_buildDSN_NoDatabase 测试 Trino DSN 无 database
func TestTrinoDriver_buildDSN_NoDatabase(t *testing.T) {
	d := &TrinoDriver{config: DatasourceConfig{
		Host:     "localhost",
		Port:     8080,
		Username: "user",
	}}

	dsn := d.buildDSN(8080)
	if dsn == "" {
		t.Error("buildDSN 不应返回空字符串")
	}
}

// === StarRocks buildDSN 补充场景 ===

// TestStarRocksDriver_buildDSN_WithSSL 测试 StarRocks DSN 带 SSL
func TestStarRocksDriver_buildDSN_WithSSL(t *testing.T) {
	d := &StarRocksDriver{config: DatasourceConfig{
		Host:     "secure-host",
		Port:     9030,
		Username: "root",
		Password: "pass",
		Database: "test_db",
		Config:   map[string]interface{}{"ssl": true},
	}}

	dsn := d.buildDSN()
	if dsn == "" {
		t.Error("buildDSN 不应返回空字符串")
	}
}

// === MySQL buildDSN 补充场景 ===

// TestMySQLDriver_buildDSN_NoSSL 测试 MySQL DSN 不带 SSL
func TestMySQLDriver_buildDSN_NoSSL(t *testing.T) {
	d := &MySQLDriver{config: DatasourceConfig{
		Host:     "localhost",
		Port:     3306,
		Username: "root",
		Password: "pass",
		Database: "testdb",
		Config:   map[string]interface{}{"ssl": false},
	}}

	dsn := d.buildDSN()
	if dsn == "" {
		t.Error("buildDSN 不应返回空字符串")
	}
}
