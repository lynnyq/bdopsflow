package driver

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	gohive "github.com/beltran/gohive"
)

// === Hive/Kyuubi/Spark queryWithDB acquire 错误路径测试 ===
// 以下测试覆盖 hive.go:253、kyuubi.go:236、spark.go:236 的 queryWithDB 函数
// 当 pool 存在但 acquire 失败时的错误处理路径（当前仅覆盖 nil pool 路径）

// TestPoolDriver_QueryWithDB_FailingPool 测试 Hive/Kyuubi/Spark 在 pool 存在但 acquire 失败时 QueryWithDB 返回错误
// 覆盖 queryWithDB 的 acquire 错误分支（line 267-270）
func TestPoolDriver_QueryWithDB_FailingPool(t *testing.T) {
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
			switch dd := d.driver.(type) {
			case *HiveDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *KyuubiDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *SparkDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := d.driver.QueryWithDB(ctx, "SELECT 1", "test_db")
			if err == nil {
				t.Error("QueryWithDB 应在 acquire 失败时返回错误")
			}
		})
	}
}

// TestPoolDriver_TryQueryWithDB_FailingPool 测试 Hive/Kyuubi/Spark 在 pool 存在但 tryAcquire 失败时 TryQueryWithDB 返回错误
// 覆盖 queryWithDB 的 acquire 错误分支（通过 tryAcquire）
func TestPoolDriver_TryQueryWithDB_FailingPool(t *testing.T) {
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
			switch dd := d.driver.(type) {
			case *HiveDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *KyuubiDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *SparkDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := d.driver.TryQueryWithDB(ctx, "SELECT 1", "test_db")
			if err == nil {
				t.Error("TryQueryWithDB 应在 tryAcquire 失败时返回错误")
			}
		})
	}
}

// TestPoolDriver_Query_FailingPool 测试 Hive/Kyuubi/Spark 在 pool 存在但 acquire 失败时 Query 返回错误
// Query 委托给 QueryWithDB，覆盖 queryWithDB 的 acquire 错误分支
func TestPoolDriver_Query_FailingPool(t *testing.T) {
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
			switch dd := d.driver.(type) {
			case *HiveDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *KyuubiDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *SparkDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := d.driver.Query(ctx, "SELECT 1")
			if err == nil {
				t.Error("Query 应在 acquire 失败时返回错误")
			}
		})
	}
}

// TestPoolDriver_GetDatabases_FailingPool 测试 Hive/Kyuubi/Spark 在 pool 存在但 acquire 失败时 GetDatabases 返回错误
// GetDatabases 委托给 TryQueryWithDB，覆盖 queryWithDB 的 acquire 错误分支
func TestPoolDriver_GetDatabases_FailingPool(t *testing.T) {
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
			switch dd := d.driver.(type) {
			case *HiveDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *KyuubiDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *SparkDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := d.driver.GetDatabases(ctx)
			if err == nil {
				t.Error("GetDatabases 应在 acquire 失败时返回错误")
			}
		})
	}
}

// TestPoolDriver_GetTables_FailingPool 测试 Hive/Kyuubi/Spark 在 pool 存在但 acquire 失败时 GetTables 返回错误
func TestPoolDriver_GetTables_FailingPool(t *testing.T) {
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
			switch dd := d.driver.(type) {
			case *HiveDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *KyuubiDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *SparkDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := d.driver.GetTables(ctx, "test_db")
			if err == nil {
				t.Error("GetTables 应在 acquire 失败时返回错误")
			}
		})
	}
}

// TestPoolDriver_GetColumns_FailingPool 测试 Hive/Kyuubi/Spark 在 pool 存在但 acquire 失败时 GetColumns 返回错误
func TestPoolDriver_GetColumns_FailingPool(t *testing.T) {
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
			switch dd := d.driver.(type) {
			case *HiveDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *KyuubiDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *SparkDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := d.driver.GetColumns(ctx, "test_db", "test_table")
			if err == nil {
				t.Error("GetColumns 应在 acquire 失败时返回错误")
			}
		})
	}
}

// TestPoolDriver_QueryWithDB_CancelledContext 测试 Hive/Kyuubi/Spark 在 context 已取消时 QueryWithDB 返回错误
// 覆盖 queryWithDB 中 acquireFn 传入已取消 context 的路径
func TestPoolDriver_QueryWithDB_CancelledContext(t *testing.T) {
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
			switch dd := d.driver.(type) {
			case *HiveDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *KyuubiDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *SparkDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err := d.driver.QueryWithDB(ctx, "SELECT 1", "")
			if err == nil {
				t.Error("QueryWithDB 应在 context 取消时返回错误")
			}
		})
	}
}

// TestPoolDriver_TryQueryWithDB_CancelledContext 测试 Hive/Kyuubi/Spark 在 context 已取消时 TryQueryWithDB 返回错误
func TestPoolDriver_TryQueryWithDB_CancelledContext(t *testing.T) {
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
			switch dd := d.driver.(type) {
			case *HiveDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *KyuubiDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			case *SparkDriver:
				dd.pool = newFailingPool()
				defer dd.Close()
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err := d.driver.TryQueryWithDB(ctx, "SELECT 1", "")
			if err == nil {
				t.Error("TryQueryWithDB 应在 context 取消时返回错误")
			}
		})
	}
}

// === Hive pool 满时 tryAcquire 错误路径测试 ===

// TestHiveConnPool_TryAcquire_PoolFull 测试 tryAcquire 在连接池满时返回错误
// 覆盖 hive_pool.go:144 tryCreateConn 的 "已达上限" 分支
func TestHiveConnPool_TryAcquire_PoolFull(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        1,
		MinIdle:        0,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 先放入一个连接占满池
	pool.put(nil, "db1")

	// tryAcquire 应该成功获取一个连接
	pc, err := pool.tryAcquire(context.Background())
	if err != nil {
		t.Fatalf("首次 tryAcquire 应成功: %v", err)
	}
	if pc == nil {
		t.Fatal("tryAcquire 返回 nil 连接")
	}

	// 此时 openCount=1, inUse=1, 池已满
	// 再次 tryAcquire 应失败（已达上限）
	_, err = pool.tryAcquire(context.Background())
	if err == nil {
		t.Error("池满时 tryAcquire 应返回错误")
	}

	// 归还连接
	pool.release(pc)
}

// TestHiveConnPool_Acquire_Timeout 测试 acquire 在连接池满且等待超时时返回错误
// 覆盖 hive_pool.go:228 acquireOrCreate 的超时分支
func TestHiveConnPool_Acquire_Timeout(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        1,
		MinIdle:        0,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 100 * time.Millisecond,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 放入一个连接占满池
	pool.put(nil, "db1")

	// 获取连接（成功）
	pc, err := pool.acquire(context.Background())
	if err != nil {
		t.Fatalf("首次 acquire 应成功: %v", err)
	}

	// 再次 acquire 应超时失败
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err = pool.acquire(ctx)
	if err == nil {
		t.Error("池满时 acquire 应超时返回错误")
	}

	// 归还连接
	pool.release(pc)
}

// TestHiveConnPool_TryAcquire_CancelledContext 测试 tryAcquire 在 context 已取消时返回错误
// 覆盖 hive_pool.go:114 的 ctx.Err() 检查
func TestHiveConnPool_TryAcquire_CancelledContext(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        0,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pool.tryAcquire(ctx)
	if err == nil {
		t.Error("tryAcquire 应在 context 取消时返回错误")
	}
}

// TestHiveConnPool_Acquire_CancelledContext 测试 acquire 在 context 已取消时返回错误
// 覆盖 hive_pool.go:166 的 ctx.Err() 检查
func TestHiveConnPool_Acquire_CancelledContext(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        0,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pool.acquire(ctx)
	if err == nil {
		t.Error("acquire 应在 context 取消时返回错误")
	}
}

// TestHiveConnPool_Release_NilConn 测试 release nil 连接不 panic
// 覆盖 hive_pool.go:235 的 nil 检查
func TestHiveConnPool_Release_NilConn(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        0,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// release nil 不应 panic
	pool.release(nil)
}

// TestHiveConnPool_Discard_NilConn 测试 discard nil 连接不 panic
// 覆盖 hive_pool.go:316 的 nil 检查
func TestHiveConnPool_Discard_NilConn(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        0,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// discard nil 不应 panic
	pool.discard(nil)
}

// TestHiveConnPool_UpdateConfig_NewValues 测试 UpdateConfig 动态更新配置为新值
// 覆盖 hive_pool.go:84 UpdateConfig
func TestHiveConnPool_UpdateConfig_NewValues(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        2,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 更新配置
	newCfg := PoolConfig{
		MaxOpen:        10,
		MinIdle:        3,
		MaxLifetime:    60 * time.Minute,
		AcquireTimeout: 10 * time.Second,
	}
	pool.UpdateConfig(newCfg)

	got := pool.GetConfig()
	if got.MaxOpen != 10 {
		t.Errorf("UpdateConfig 后 MaxOpen = %d, want 10", got.MaxOpen)
	}
	if got.MinIdle != 3 {
		t.Errorf("UpdateConfig 后 MinIdle = %d, want 3", got.MinIdle)
	}
}

// TestHiveConnPool_UpdateConfig_Defaults 测试 UpdateConfig 对非法值的修正
// 覆盖 hive_pool.go:85-93 的默认值修正逻辑
func TestHiveConnPool_UpdateConfig_Defaults(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        2,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 传入非法值
	pool.UpdateConfig(PoolConfig{
		MaxOpen:        0,
		MinIdle:        -1,
		AcquireTimeout: 0,
	})

	got := pool.GetConfig()
	if got.MaxOpen != 5 {
		t.Errorf("MaxOpen 应被修正为 5, got %d", got.MaxOpen)
	}
	if got.MinIdle != 0 {
		t.Errorf("MinIdle 应被修正为 0, got %d", got.MinIdle)
	}
	if got.AcquireTimeout != 30*time.Second {
		t.Errorf("AcquireTimeout 应被修正为 30s, got %v", got.AcquireTimeout)
	}
}

// === SQLite 完整工作流补充测试 ===

// TestSQLiteDriver_ConnectThenGetDatabases 测试 SQLite 连接后 GetDatabases
// 覆盖 sqlite.go:106 GetDatabases（需要实际连接）
func TestSQLiteDriver_ConnectThenGetDatabases(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d := &SQLiteDriver{}
	if err := d.Connect(context.Background(), DatasourceConfig{Path: dbPath}); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer d.Close()

	dbs, err := d.GetDatabases(context.Background())
	if err != nil {
		t.Fatalf("GetDatabases 失败: %v", err)
	}
	if len(dbs) == 0 {
		t.Error("GetDatabases 应返回至少一个数据库")
	}
}

// TestSQLiteDriver_QueryError 测试 SQLite 查询语法错误时返回错误
// 覆盖 sqlite.go:64 Query 的错误处理路径
func TestSQLiteDriver_QueryError(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d := &SQLiteDriver{}
	if err := d.Connect(context.Background(), DatasourceConfig{Path: dbPath}); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer d.Close()

	// 执行语法错误的 SQL
	_, err := d.Query(context.Background(), "INVALID SQL STATEMENT")
	if err == nil {
		t.Error("语法错误的 SQL 应返回错误")
	}
}

// TestSQLiteDriver_GetTablesEmpty 测试 SQLite 空数据库 GetTables 返回空
func TestSQLiteDriver_GetTablesEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d := &SQLiteDriver{}
	if err := d.Connect(context.Background(), DatasourceConfig{Path: dbPath}); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer d.Close()

	// 空数据库应返回空表列表
	tables, err := d.GetTables(context.Background(), "")
	if err != nil {
		t.Fatalf("GetTables 不应返回错误: %v", err)
	}
	if len(tables) != 0 {
		t.Errorf("空数据库 GetTables 应返回 0 张表, got %d", len(tables))
	}
}

// TestSQLiteDriver_GetColumnsEmpty 测试 SQLite GetColumns 不存在表时返回空
func TestSQLiteDriver_GetColumnsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d := &SQLiteDriver{}
	if err := d.Connect(context.Background(), DatasourceConfig{Path: dbPath}); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer d.Close()

	// 不存在的表应返回空列列表
	columns, err := d.GetColumns(context.Background(), "", "nonexistent_table")
	if err != nil {
		t.Fatalf("GetColumns 不应返回错误: %v", err)
	}
	if len(columns) != 0 {
		t.Errorf("不存在的表 GetColumns 应返回 0 列, got %d", len(columns))
	}
}

// TestSQLiteDriver_UpdatePoolConfig_AfterConnect 测试 SQLite 连接后 UpdatePoolConfig
// 覆盖 sqlite.go:165 UpdatePoolConfig（有 db 时）
func TestSQLiteDriver_UpdatePoolConfig_AfterConnect(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d := &SQLiteDriver{}
	if err := d.Connect(context.Background(), DatasourceConfig{Path: dbPath}); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer d.Close()

	// 更新连接池配置
	d.UpdatePoolConfig(PoolConfig{
		MaxOpen:     20,
		MinIdle:     5,
		MaxLifetime: 60 * time.Minute,
	})

	pc := d.GetPoolConfig()
	if pc.MaxOpen != 20 {
		t.Errorf("GetPoolConfig MaxOpen = %d, want 20", pc.MaxOpen)
	}
}

// TestSQLiteDriver_CloseAfterConnect 测试 SQLite 连接后 Close
func TestSQLiteDriver_CloseAfterConnect(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d := &SQLiteDriver{}
	if err := d.Connect(context.Background(), DatasourceConfig{Path: dbPath}); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}

	if err := d.Close(); err != nil {
		t.Errorf("Close 不应返回错误: %v", err)
	}

	// 再次 Close 不应 panic
	if err := d.Close(); err != nil {
		t.Errorf("重复 Close 不应返回错误: %v", err)
	}
}

// TestSQLiteDriver_PingAfterClose 测试 SQLite Close 后 Ping 返回错误
func TestSQLiteDriver_PingAfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d := &SQLiteDriver{}
	if err := d.Connect(context.Background(), DatasourceConfig{Path: dbPath}); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}

	_ = d.Close()

	// Close 后 Ping 应返回错误
	err := d.Ping(context.Background())
	if err == nil {
		t.Error("Close 后 Ping 应返回错误")
	}
}

// TestSQLiteDriver_TestConnectionAfterClose 测试 SQLite Close 后 TestConnection 返回错误
func TestSQLiteDriver_TestConnectionAfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d := &SQLiteDriver{}
	if err := d.Connect(context.Background(), DatasourceConfig{Path: dbPath}); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}

	_ = d.Close()

	// Close 后 TestConnection 应返回错误
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("Close 后 TestConnection 应返回错误")
	}
}

// === MySQL nil-db 方法测试 ===

// TestMySQLDriver_NilConn_TestConnection 测试 MySQL TestConnection 未连接时返回错误
func TestMySQLDriver_NilConn_TestConnection(t *testing.T) {
	d := &MySQLDriver{}
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("TestConnection 应在未连接时返回错误")
	}
}

// TestMySQLDriver_NilConn_Ping 测试 MySQL Ping 未连接时返回错误
func TestMySQLDriver_NilConn_Ping(t *testing.T) {
	d := &MySQLDriver{}
	err := d.Ping(context.Background())
	if err == nil {
		t.Error("Ping 应在未连接时返回错误")
	}
}

// TestMySQLDriver_NilConn_Close 测试 MySQL Close 未连接时不报错
func TestMySQLDriver_NilConn_Close(t *testing.T) {
	d := &MySQLDriver{}
	if err := d.Close(); err != nil {
		t.Errorf("Close 不应返回错误: %v", err)
	}
}

// TestMySQLDriver_NilConn_Query 测试 MySQL Query 未连接时返回错误
func TestMySQLDriver_NilConn_Query(t *testing.T) {
	d := &MySQLDriver{}
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("Query 应在未连接时返回错误")
	}
}

// TestMySQLDriver_NilConn_UseDatabase 测试 MySQL UseDatabase 未连接时返回错误
func TestMySQLDriver_NilConn_UseDatabase(t *testing.T) {
	d := &MySQLDriver{}
	err := d.UseDatabase(context.Background(), "test")
	if err == nil {
		t.Error("UseDatabase 应在未连接时返回错误")
	}
}

// TestMySQLDriver_UseDatabase_Empty 测试 MySQL UseDatabase 空参数返回 nil
func TestMySQLDriver_UseDatabase_Empty(t *testing.T) {
	d := &MySQLDriver{}
	if err := d.UseDatabase(context.Background(), ""); err != nil {
		t.Errorf("UseDatabase 空参数应返回 nil: %v", err)
	}
}

// TestMySQLDriver_UpdatePoolConfig_NilDB 测试 MySQL UpdatePoolConfig 未连接时不 panic
func TestMySQLDriver_UpdatePoolConfig_NilDB(t *testing.T) {
	d := &MySQLDriver{}
	d.UpdatePoolConfig(PoolConfig{MaxOpen: 10})
	// 不应 panic
}

// TestMySQLDriver_GetPoolConfig_NilDB 测试 MySQL GetPoolConfig 未连接时返回默认值
func TestMySQLDriver_GetPoolConfig_NilDB(t *testing.T) {
	d := &MySQLDriver{}
	cfg := d.GetPoolConfig()
	if cfg.MaxOpen != 5 {
		t.Errorf("GetPoolConfig MaxOpen = %d, want 5 (default)", cfg.MaxOpen)
	}
}

// TestMySQLDriver_GetPoolStats_NilDB 测试 MySQL GetPoolStats 未连接时返回 0
func TestMySQLDriver_GetPoolStats_NilDB(t *testing.T) {
	d := &MySQLDriver{}
	oc, ic, iu, mo := d.GetPoolStats()
	if oc != 0 || ic != 0 || iu != 0 || mo != 0 {
		t.Errorf("GetPoolStats 未连接时应全为 0, got oc=%d ic=%d iu=%d mo=%d", oc, ic, iu, mo)
	}
}

// TestMySQLDriver_QueryWithDB_EmptyDatabase 测试 MySQL QueryWithDB 空数据库委托给 Query
func TestMySQLDriver_QueryWithDB_EmptyDatabase(t *testing.T) {
	d := &MySQLDriver{}
	_, err := d.QueryWithDB(context.Background(), "SELECT 1", "")
	if err == nil {
		t.Error("QueryWithDB 空数据库应委托给 Query 并返回错误")
	}
}

// TestMySQLDriver_TryQueryWithDB 测试 MySQL TryQueryWithDB 委托给 QueryWithDB
func TestMySQLDriver_TryQueryWithDB(t *testing.T) {
	d := &MySQLDriver{}
	_, err := d.TryQueryWithDB(context.Background(), "SELECT 1", "test")
	if err == nil {
		t.Error("TryQueryWithDB 应返回错误")
	}
}

// TestMySQLDriver_SupportsCancel 测试 MySQL SupportsCancel
func TestMySQLDriver_SupportsCancel(t *testing.T) {
	d := &MySQLDriver{}
	if !d.SupportsCancel() {
		t.Error("MySQL 应支持取消")
	}
}

// === Doris nil-db 方法测试 ===

// TestDorisDriver_NilConn_TestConnection 测试 Doris TestConnection 未连接时返回错误
func TestDorisDriver_NilConn_TestConnection(t *testing.T) {
	d := &DorisDriver{}
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("TestConnection 应在未连接时返回错误")
	}
}

// TestDorisDriver_NilConn_Ping 测试 Doris Ping 未连接时返回错误
func TestDorisDriver_NilConn_Ping(t *testing.T) {
	d := &DorisDriver{}
	err := d.Ping(context.Background())
	if err == nil {
		t.Error("Ping 应在未连接时返回错误")
	}
}

// TestDorisDriver_NilConn_Close 测试 Doris Close 未连接时不报错
func TestDorisDriver_NilConn_Close(t *testing.T) {
	d := &DorisDriver{}
	if err := d.Close(); err != nil {
		t.Errorf("Close 不应返回错误: %v", err)
	}
}

// TestDorisDriver_NilConn_Query 测试 Doris Query 未连接时返回错误
func TestDorisDriver_NilConn_Query(t *testing.T) {
	d := &DorisDriver{}
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("Query 应在未连接时返回错误")
	}
}

// TestDorisDriver_NilConn_UseDatabase 测试 Doris UseDatabase 未连接时返回错误
func TestDorisDriver_NilConn_UseDatabase(t *testing.T) {
	d := &DorisDriver{}
	err := d.UseDatabase(context.Background(), "test")
	if err == nil {
		t.Error("UseDatabase 应在未连接时返回错误")
	}
}

// TestDorisDriver_UseDatabase_Empty 测试 Doris UseDatabase 空参数返回 nil
func TestDorisDriver_UseDatabase_Empty(t *testing.T) {
	d := &DorisDriver{}
	if err := d.UseDatabase(context.Background(), ""); err != nil {
		t.Errorf("UseDatabase 空参数应返回 nil: %v", err)
	}
}

// TestDorisDriver_NilConn_GetDatabases 测试 Doris GetDatabases 未连接时返回错误
func TestDorisDriver_NilConn_GetDatabases(t *testing.T) {
	d := &DorisDriver{}
	_, err := d.GetDatabases(context.Background())
	if err == nil {
		t.Error("GetDatabases 应在未连接时返回错误")
	}
}

// TestDorisDriver_NilConn_GetTables 测试 Doris GetTables 未连接时返回错误
func TestDorisDriver_NilConn_GetTables(t *testing.T) {
	d := &DorisDriver{}
	_, err := d.GetTables(context.Background(), "test")
	if err == nil {
		t.Error("GetTables 应在未连接时返回错误")
	}
}

// TestDorisDriver_NilConn_GetColumns 测试 Doris GetColumns 未连接时返回错误
func TestDorisDriver_NilConn_GetColumns(t *testing.T) {
	d := &DorisDriver{}
	_, err := d.GetColumns(context.Background(), "test", "table")
	if err == nil {
		t.Error("GetColumns 应在未连接时返回错误")
	}
}

// TestDorisDriver_UpdatePoolConfig_NilDB 测试 Doris UpdatePoolConfig 未连接时不 panic
func TestDorisDriver_UpdatePoolConfig_NilDB(t *testing.T) {
	d := &DorisDriver{}
	d.UpdatePoolConfig(PoolConfig{MaxOpen: 10})
}

// TestDorisDriver_GetPoolConfig_NilDB 测试 Doris GetPoolConfig 未连接时返回默认值
func TestDorisDriver_GetPoolConfig_NilDB(t *testing.T) {
	d := &DorisDriver{}
	cfg := d.GetPoolConfig()
	if cfg.MaxOpen != 5 {
		t.Errorf("GetPoolConfig MaxOpen = %d, want 5", cfg.MaxOpen)
	}
}

// TestDorisDriver_GetPoolStats_NilDB 测试 Doris GetPoolStats 未连接时返回 0
func TestDorisDriver_GetPoolStats_NilDB(t *testing.T) {
	d := &DorisDriver{}
	oc, ic, iu, mo := d.GetPoolStats()
	if oc != 0 || ic != 0 || iu != 0 || mo != 0 {
		t.Errorf("GetPoolStats 未连接时应全为 0, got oc=%d ic=%d iu=%d mo=%d", oc, ic, iu, mo)
	}
}

// TestDorisDriver_SupportsCancel 测试 Doris SupportsCancel
func TestDorisDriver_SupportsCancel(t *testing.T) {
	d := &DorisDriver{}
	if !d.SupportsCancel() {
		t.Error("Doris 应支持取消")
	}
}

// === StarRocks nil-db 方法测试 ===

// TestStarRocksDriver_NilConn_TestConnection 测试 StarRocks TestConnection 未连接时返回错误
func TestStarRocksDriver_NilConn_TestConnection(t *testing.T) {
	d := &StarRocksDriver{}
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("TestConnection 应在未连接时返回错误")
	}
}

// TestStarRocksDriver_NilConn_Ping 测试 StarRocks Ping 未连接时返回错误
func TestStarRocksDriver_NilConn_Ping(t *testing.T) {
	d := &StarRocksDriver{}
	err := d.Ping(context.Background())
	if err == nil {
		t.Error("Ping 应在未连接时返回错误")
	}
}

// TestStarRocksDriver_NilConn_Close 测试 StarRocks Close 未连接时不报错
func TestStarRocksDriver_NilConn_Close(t *testing.T) {
	d := &StarRocksDriver{}
	if err := d.Close(); err != nil {
		t.Errorf("Close 不应返回错误: %v", err)
	}
}

// TestStarRocksDriver_NilConn_Query 测试 StarRocks Query 未连接时返回错误
func TestStarRocksDriver_NilConn_Query(t *testing.T) {
	d := &StarRocksDriver{}
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("Query 应在未连接时返回错误")
	}
}

// TestStarRocksDriver_NilConn_UseDatabase 测试 StarRocks UseDatabase 未连接时返回错误
func TestStarRocksDriver_NilConn_UseDatabase(t *testing.T) {
	d := &StarRocksDriver{}
	err := d.UseDatabase(context.Background(), "test")
	if err == nil {
		t.Error("UseDatabase 应在未连接时返回错误")
	}
}

// TestStarRocksDriver_UseDatabase_Empty 测试 StarRocks UseDatabase 空参数返回 nil
func TestStarRocksDriver_UseDatabase_Empty(t *testing.T) {
	d := &StarRocksDriver{}
	if err := d.UseDatabase(context.Background(), ""); err != nil {
		t.Errorf("UseDatabase 空参数应返回 nil: %v", err)
	}
}

// TestStarRocksDriver_NilConn_GetDatabases 测试 StarRocks GetDatabases 未连接时返回错误
func TestStarRocksDriver_NilConn_GetDatabases(t *testing.T) {
	d := &StarRocksDriver{}
	_, err := d.GetDatabases(context.Background())
	if err == nil {
		t.Error("GetDatabases 应在未连接时返回错误")
	}
}

// TestStarRocksDriver_NilConn_GetTables 测试 StarRocks GetTables 未连接时返回错误
func TestStarRocksDriver_NilConn_GetTables(t *testing.T) {
	d := &StarRocksDriver{}
	_, err := d.GetTables(context.Background(), "test")
	if err == nil {
		t.Error("GetTables 应在未连接时返回错误")
	}
}

// TestStarRocksDriver_NilConn_GetColumns 测试 StarRocks GetColumns 未连接时返回错误
func TestStarRocksDriver_NilConn_GetColumns(t *testing.T) {
	d := &StarRocksDriver{}
	_, err := d.GetColumns(context.Background(), "test", "table")
	if err == nil {
		t.Error("GetColumns 应在未连接时返回错误")
	}
}

// TestStarRocksDriver_UpdatePoolConfig_NilDB 测试 StarRocks UpdatePoolConfig 未连接时不 panic
func TestStarRocksDriver_UpdatePoolConfig_NilDB(t *testing.T) {
	d := &StarRocksDriver{}
	d.UpdatePoolConfig(PoolConfig{MaxOpen: 10})
}

// TestStarRocksDriver_GetPoolConfig_NilDB 测试 StarRocks GetPoolConfig 未连接时返回默认值
func TestStarRocksDriver_GetPoolConfig_NilDB(t *testing.T) {
	d := &StarRocksDriver{}
	cfg := d.GetPoolConfig()
	if cfg.MaxOpen != 5 {
		t.Errorf("GetPoolConfig MaxOpen = %d, want 5", cfg.MaxOpen)
	}
}

// TestStarRocksDriver_GetPoolStats_NilDB 测试 StarRocks GetPoolStats 未连接时返回 0
func TestStarRocksDriver_GetPoolStats_NilDB(t *testing.T) {
	d := &StarRocksDriver{}
	oc, ic, iu, mo := d.GetPoolStats()
	if oc != 0 || ic != 0 || iu != 0 || mo != 0 {
		t.Errorf("GetPoolStats 未连接时应全为 0, got oc=%d ic=%d iu=%d mo=%d", oc, ic, iu, mo)
	}
}

// TestStarRocksDriver_SupportsCancel 测试 StarRocks SupportsCancel
func TestStarRocksDriver_SupportsCancel(t *testing.T) {
	d := &StarRocksDriver{}
	if !d.SupportsCancel() {
		t.Error("StarRocks 应支持取消")
	}
}

// === Trino nil-db 方法测试 ===

// TestTrinoDriver_NilConn_TestConnection 测试 Trino TestConnection 未连接时返回错误
func TestTrinoDriver_NilConn_TestConnection(t *testing.T) {
	d := &TrinoDriver{}
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("TestConnection 应在未连接时返回错误")
	}
}

// TestTrinoDriver_NilConn_Ping 测试 Trino Ping 未连接时返回错误
func TestTrinoDriver_NilConn_Ping(t *testing.T) {
	d := &TrinoDriver{}
	err := d.Ping(context.Background())
	if err == nil {
		t.Error("Ping 应在未连接时返回错误")
	}
}

// TestTrinoDriver_NilConn_Close 测试 Trino Close 未连接时不报错
func TestTrinoDriver_NilConn_Close(t *testing.T) {
	d := &TrinoDriver{}
	if err := d.Close(); err != nil {
		t.Errorf("Close 不应返回错误: %v", err)
	}
}

// TestTrinoDriver_NilConn_Query 测试 Trino Query 未连接时返回错误
func TestTrinoDriver_NilConn_Query(t *testing.T) {
	d := &TrinoDriver{}
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("Query 应在未连接时返回错误")
	}
}

// TestTrinoDriver_NilConn_UseDatabase 测试 Trino UseDatabase 未连接时返回错误
func TestTrinoDriver_NilConn_UseDatabase(t *testing.T) {
	d := &TrinoDriver{}
	err := d.UseDatabase(context.Background(), "test")
	if err == nil {
		t.Error("UseDatabase 应在未连接时返回错误")
	}
}

// TestTrinoDriver_UseDatabase_Empty 测试 Trino UseDatabase 空参数返回 nil
func TestTrinoDriver_UseDatabase_Empty(t *testing.T) {
	d := &TrinoDriver{}
	if err := d.UseDatabase(context.Background(), ""); err != nil {
		t.Errorf("UseDatabase 空参数应返回 nil: %v", err)
	}
}

// TestTrinoDriver_NilConn_GetTables 测试 Trino GetTables 未连接时返回错误
func TestTrinoDriver_NilConn_GetTables(t *testing.T) {
	d := &TrinoDriver{}
	_, err := d.GetTables(context.Background(), "test")
	if err == nil {
		t.Error("GetTables 应在未连接时返回错误")
	}
}

// TestTrinoDriver_NilConn_GetColumns 测试 Trino GetColumns 未连接时返回错误
func TestTrinoDriver_NilConn_GetColumns(t *testing.T) {
	d := &TrinoDriver{}
	_, err := d.GetColumns(context.Background(), "test", "table")
	if err == nil {
		t.Error("GetColumns 应在未连接时返回错误")
	}
}

// TestTrinoDriver_GetTables_EmptyDatabase 测试 Trino GetTables 空数据库且无配置时返回错误
func TestTrinoDriver_GetTables_EmptyDatabase(t *testing.T) {
	d := &TrinoDriver{}
	_, err := d.GetTables(context.Background(), "")
	if err == nil {
		t.Error("GetTables 空数据库应返回错误")
	}
}

// TestTrinoDriver_GetColumns_EmptyDatabase 测试 Trino GetColumns 空数据库且无配置时返回错误
func TestTrinoDriver_GetColumns_EmptyDatabase(t *testing.T) {
	d := &TrinoDriver{}
	_, err := d.GetColumns(context.Background(), "", "table")
	if err == nil {
		t.Error("GetColumns 空数据库应返回错误")
	}
}

// TestTrinoDriver_UpdatePoolConfig_NilDB 测试 Trino UpdatePoolConfig 未连接时不 panic
func TestTrinoDriver_UpdatePoolConfig_NilDB(t *testing.T) {
	d := &TrinoDriver{}
	d.UpdatePoolConfig(PoolConfig{MaxOpen: 10})
}

// TestTrinoDriver_GetPoolConfig_NilDB 测试 Trino GetPoolConfig 未连接时返回默认值
func TestTrinoDriver_GetPoolConfig_NilDB(t *testing.T) {
	d := &TrinoDriver{}
	cfg := d.GetPoolConfig()
	if cfg.MaxOpen != 5 {
		t.Errorf("GetPoolConfig MaxOpen = %d, want 5", cfg.MaxOpen)
	}
}

// TestTrinoDriver_GetPoolStats_NilDB 测试 Trino GetPoolStats 未连接时返回 0
func TestTrinoDriver_GetPoolStats_NilDB(t *testing.T) {
	d := &TrinoDriver{}
	oc, ic, iu, mo := d.GetPoolStats()
	if oc != 0 || ic != 0 || iu != 0 || mo != 0 {
		t.Errorf("GetPoolStats 未连接时应全为 0, got oc=%d ic=%d iu=%d mo=%d", oc, ic, iu, mo)
	}
}

// TestTrinoDriver_SupportsCancel 测试 Trino SupportsCancel
func TestTrinoDriver_SupportsCancel(t *testing.T) {
	d := &TrinoDriver{}
	if !d.SupportsCancel() {
		t.Error("Trino 应支持取消")
	}
}

// TestTrinoDriver_QueryWithDB_EmptyDatabase 测试 Trino QueryWithDB 空数据库委托给 Query
func TestTrinoDriver_QueryWithDB_EmptyDatabase(t *testing.T) {
	d := &TrinoDriver{}
	_, err := d.QueryWithDB(context.Background(), "SELECT 1", "")
	if err == nil {
		t.Error("QueryWithDB 空数据库应委托给 Query 并返回错误")
	}
}

// TestTrinoDriver_TryQueryWithDB 测试 Trino TryQueryWithDB 委托给 QueryWithDB
func TestTrinoDriver_TryQueryWithDB(t *testing.T) {
	d := &TrinoDriver{}
	_, err := d.TryQueryWithDB(context.Background(), "SELECT 1", "test")
	if err == nil {
		t.Error("TryQueryWithDB 应返回错误")
	}
}

// === Rqlite nil-conn 方法补充测试 ===

// TestRqliteDriver_NilConn_TestConnection 测试 Rqlite TestConnection 未连接时返回错误
func TestRqliteDriver_NilConn_TestConnection(t *testing.T) {
	d := &RqliteDriver{}
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("TestConnection 应在未连接时返回错误")
	}
}

// TestRqliteDriver_NilConn_Close 测试 Rqlite Close 未连接时不报错
func TestRqliteDriver_NilConn_Close(t *testing.T) {
	d := &RqliteDriver{}
	if err := d.Close(); err != nil {
		t.Errorf("Close 不应返回错误: %v", err)
	}
}

// TestRqliteDriver_NilConn_Query 测试 Rqlite Query 未连接时返回错误
func TestRqliteDriver_NilConn_Query(t *testing.T) {
	d := &RqliteDriver{}
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("Query 应在未连接时返回错误")
	}
}

// TestRqliteDriver_NilConn_QueryWithDB 测试 Rqlite QueryWithDB 未连接时返回错误
func TestRqliteDriver_NilConn_QueryWithDB(t *testing.T) {
	d := &RqliteDriver{}
	_, err := d.QueryWithDB(context.Background(), "SELECT 1", "")
	if err == nil {
		t.Error("QueryWithDB 应在未连接时返回错误")
	}
}

// TestRqliteDriver_NilConn_TryQueryWithDB 测试 Rqlite TryQueryWithDB 未连接时返回错误
func TestRqliteDriver_NilConn_TryQueryWithDB(t *testing.T) {
	d := &RqliteDriver{}
	_, err := d.TryQueryWithDB(context.Background(), "SELECT 1", "")
	if err == nil {
		t.Error("TryQueryWithDB 应在未连接时返回错误")
	}
}

// TestRqliteDriver_SupportsCancel 测试 Rqlite SupportsCancel 返回 false
func TestRqliteDriver_SupportsCancel(t *testing.T) {
	d := &RqliteDriver{}
	if d.SupportsCancel() {
		t.Error("Rqlite 不应支持取消")
	}
}

// TestRqliteDriver_UseDatabase 测试 Rqlite UseDatabase 始终返回 nil
func TestRqliteDriver_UseDatabase(t *testing.T) {
	d := &RqliteDriver{}
	if err := d.UseDatabase(context.Background(), "test"); err != nil {
		t.Errorf("UseDatabase 应返回 nil: %v", err)
	}
}

// TestRqliteDriver_GetDatabases 测试 Rqlite GetDatabases 始终返回 main
func TestRqliteDriver_GetDatabases(t *testing.T) {
	d := &RqliteDriver{}
	dbs, err := d.GetDatabases(context.Background())
	if err != nil {
		t.Fatalf("GetDatabases 不应返回错误: %v", err)
	}
	if len(dbs) != 1 || dbs[0] != "main" {
		t.Errorf("GetDatabases 应返回 [main], got %v", dbs)
	}
}

// TestRqliteDriver_Ping_NilConn 测试 Rqlite Ping 未连接时返回错误
func TestRqliteDriver_Ping_NilConn(t *testing.T) {
	d := &RqliteDriver{}
	err := d.Ping(context.Background())
	if err == nil {
		t.Error("Ping 应在未连接时返回错误")
	}
}
