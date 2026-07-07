package driver

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	gohive "github.com/beltran/gohive"
)

// === Connect 错误路径测试 ===
// 以下测试覆盖各驱动的 Connect 函数（当前 0%）
// 使用不可达地址触发连接失败

// TestSQLiteDriver_Connect_EmptyPath 测试 SQLite Connect 空路径错误
func TestSQLiteDriver_Connect_EmptyPath(t *testing.T) {
	d := &SQLiteDriver{}
	err := d.Connect(context.Background(), DatasourceConfig{Path: ""})
	if err == nil {
		t.Error("空路径应返回错误")
	}
}

// TestSQLiteDriver_Connect_TempFile 测试 SQLite Connect 使用临时文件成功连接
func TestSQLiteDriver_Connect_TempFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d := &SQLiteDriver{}
	err := d.Connect(context.Background(), DatasourceConfig{Path: dbPath})
	if err != nil {
		t.Fatalf("Connect 应成功: %v", err)
	}
	defer d.Close()

	// 验证连接可用
	if err := d.Ping(context.Background()); err != nil {
		t.Errorf("Ping 应成功: %v", err)
	}
}

// TestMySQLDriver_Connect_UnreachableHost 测试 MySQL Connect 不可达主机
func TestMySQLDriver_Connect_UnreachableHost(t *testing.T) {
	d := &MySQLDriver{}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := d.Connect(ctx, DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "root",
		Password: "pass",
		Database: "testdb",
	})
	if err == nil {
		t.Error("不可达主机应返回错误")
	}
}

// TestDorisDriver_Connect_UnreachableHost 测试 Doris Connect 不可达主机
func TestDorisDriver_Connect_UnreachableHost(t *testing.T) {
	d := &DorisDriver{}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := d.Connect(ctx, DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "root",
		Password: "pass",
		Database: "testdb",
	})
	if err == nil {
		t.Error("不可达主机应返回错误")
	}
}

// TestStarRocksDriver_Connect_UnreachableHost 测试 StarRocks Connect 不可达主机
func TestStarRocksDriver_Connect_UnreachableHost(t *testing.T) {
	d := &StarRocksDriver{}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := d.Connect(ctx, DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "root",
		Password: "pass",
		Database: "testdb",
	})
	if err == nil {
		t.Error("不可达主机应返回错误")
	}
}

// TestTrinoDriver_Connect_UnreachableHost 测试 Trino Connect
// 注意：trino-go-client 使用惰性连接，sql.Open + PingContext 不实际连接，
// 因此这里仅验证 Connect 不 panic 且不返回 sql.Open 错误
func TestTrinoDriver_Connect_UnreachableHost(t *testing.T) {
	d := &TrinoDriver{}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Trino client 使用惰性连接，Connect 可能成功（不实际连接）
	// 这里仅验证不 panic
	_ = d.Connect(ctx, DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "test",
		Database: "default",
	})
	defer d.Close()
}

// === RqliteDriver nil-conn 方法测试 ===

// TestRqliteDriver_NilConn_Ping 测试 RqliteDriver Ping 未连接时返回错误
// 覆盖 rqlite_driver.go:116 Ping（当前 0%）
func TestRqliteDriver_NilConn_Ping(t *testing.T) {
	d := &RqliteDriver{}
	err := d.Ping(context.Background())
	if err == nil {
		t.Error("Ping 应在未连接时返回错误")
	}
}

// TestRqliteDriver_NilConn_GetTables 测试 RqliteDriver GetTables 未连接时返回错误
// 覆盖 rqlite_driver.go:186 GetTables（当前 0%）
func TestRqliteDriver_NilConn_GetTables(t *testing.T) {
	d := &RqliteDriver{}
	_, err := d.GetTables(context.Background(), "")
	if err == nil {
		t.Error("GetTables 应在未连接时返回错误")
	}
}

// TestRqliteDriver_NilConn_GetColumns 测试 RqliteDriver GetColumns 未连接时返回错误
// 覆盖 rqlite_driver.go:200 GetColumns（当前 0%）
func TestRqliteDriver_NilConn_GetColumns(t *testing.T) {
	d := &RqliteDriver{}
	_, err := d.GetColumns(context.Background(), "", "test_table")
	if err == nil {
		t.Error("GetColumns 应在未连接时返回错误")
	}
}

// === Hive/Kyuubi/Spark nil-pool Ping 测试 ===

// TestPoolDriver_PingNilPool 测试 Hive/Kyuubi/Spark 在 pool 为 nil 但未标记 unhealthy 时 Ping 返回错误
// 覆盖 hive.go:175、kyuubi.go:163、spark.go:163 的 nil pool 分支
func TestPoolDriver_PingNilPool(t *testing.T) {
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
				t.Error("Ping 应在 pool 为 nil 时返回错误")
			}
		})
	}
}

// === HiveConnPool doCleanup 测试 ===

// TestHiveConnPool_DoCleanup_EmptyPool 测试 doCleanup 在空池上的行为
// 覆盖 hive_pool.go:388 doCleanup（当前 0%）
func TestHiveConnPool_DoCleanup_EmptyPool(t *testing.T) {
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

	// 空池上调用 doCleanup 不应 panic
	pool.doCleanup()
}

// TestHiveConnPool_DoCleanup_WithConnections 测试 doCleanup 处理未过期连接
func TestHiveConnPool_DoCleanup_WithConnections(t *testing.T) {
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

	pool.put(nil, "db1")
	pool.put(nil, "db2")

	oc, ic, _, _ := pool.stats()
	if oc != 2 {
		t.Fatalf("初始 openCount = %d, want 2", oc)
	}
	if ic != 2 {
		t.Fatalf("初始 idleCount = %d, want 2", ic)
	}

	// 连接未过期应保留
	pool.doCleanup()

	oc, ic, _, _ = pool.stats()
	if oc != 2 {
		t.Errorf("doCleanup 后 openCount = %d, want 2", oc)
	}
	if ic != 2 {
		t.Errorf("doCleanup 后 idleCount = %d, want 2", ic)
	}
}

// TestHiveConnPool_DoCleanup_MinIdlePreWarm 测试 doCleanup 预热 MinIdle 连接
func TestHiveConnPool_DoCleanup_MinIdlePreWarm(t *testing.T) {
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

	// 空池上调用 doCleanup，应触发 MinIdle 预热
	pool.doCleanup()

	oc, ic, _, _ := pool.stats()
	if ic < 1 {
		t.Errorf("doCleanup 后 idleCount = %d, want >= 1", ic)
	}
	if oc < 1 {
		t.Errorf("doCleanup 后 openCount = %d, want >= 1", oc)
	}
}

// === SQLite Connect 成功后的方法测试 ===

// TestSQLiteDriver_ConnectThenQuery 测试 SQLite 连接后执行查询
// 覆盖 SQLite 的 Query、GetTables、GetColumns 方法（需要实际连接）
func TestSQLiteDriver_ConnectThenQuery(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d := &SQLiteDriver{}
	if err := d.Connect(context.Background(), DatasourceConfig{Path: dbPath}); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer d.Close()

	// 创建一张表
	_, err := d.Query(context.Background(), "CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("创建表失败: %v", err)
	}

	// 插入数据
	_, err = d.Query(context.Background(), "INSERT INTO test_table (id, name) VALUES (1, 'hello')")
	if err != nil {
		t.Fatalf("插入数据失败: %v", err)
	}

	// 查询数据
	result, err := d.Query(context.Background(), "SELECT id, name FROM test_table")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if result.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", result.RowCount)
	}

	// 获取表列表
	tables, err := d.GetTables(context.Background(), "")
	if err != nil {
		t.Fatalf("GetTables 失败: %v", err)
	}
	if len(tables) == 0 {
		t.Error("GetTables 应返回至少一张表")
	}

	// 获取列信息
	columns, err := d.GetColumns(context.Background(), "", "test_table")
	if err != nil {
		t.Fatalf("GetColumns 失败: %v", err)
	}
	if len(columns) != 2 {
		t.Errorf("GetColumns 返回 %d 列, want 2", len(columns))
	}

	// 测试 QueryWithDB
	_, err = d.QueryWithDB(context.Background(), "SELECT * FROM test_table", "")
	if err != nil {
		t.Errorf("QueryWithDB 失败: %v", err)
	}

	// 测试 TryQueryWithDB
	_, err = d.TryQueryWithDB(context.Background(), "SELECT * FROM test_table", "")
	if err != nil {
		t.Errorf("TryQueryWithDB 失败: %v", err)
	}

	// 测试 GetPoolConfig 和 GetPoolStats（需要连接）
	// SQLite 默认 MaxOpen 为 0（无限），先 UpdatePoolConfig 设置后再检查
	d.UpdatePoolConfig(PoolConfig{MaxOpen: 10, MinIdle: 5, MaxLifetime: 30 * time.Minute})

	pc := d.GetPoolConfig()
	if pc.MaxOpen != 10 {
		t.Errorf("GetPoolConfig MaxOpen = %d, want 10", pc.MaxOpen)
	}

	_, _, _, maxOpen := d.GetPoolStats()
	if maxOpen != 10 {
		t.Errorf("GetPoolStats maxOpen = %d, want 10", maxOpen)
	}
}

// TestSQLiteDriver_ConnectThenTestConnection 测试 SQLite 连接后 TestConnection
func TestSQLiteDriver_ConnectThenTestConnection(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d := &SQLiteDriver{}
	if err := d.Connect(context.Background(), DatasourceConfig{Path: dbPath}); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer d.Close()

	if err := d.TestConnection(context.Background()); err != nil {
		t.Errorf("TestConnection 应成功: %v", err)
	}
}

// TestSQLiteDriver_CloseWithoutConnect 测试 SQLite 未连接时 Close 不 panic
func TestSQLiteDriver_CloseWithoutConnect(t *testing.T) {
	d := &SQLiteDriver{}
	if err := d.Close(); err != nil {
		t.Errorf("Close 不应返回错误: %v", err)
	}
}

// TestSQLiteDriver_SupportsCancel 测试 SQLite SupportsCancel
func TestSQLiteDriver_SupportsCancel(t *testing.T) {
	d := &SQLiteDriver{}
	if !d.SupportsCancel() {
		t.Error("SQLite 应支持取消")
	}
}

// TestSQLiteDriver_UseDatabase 测试 SQLite UseDatabase 始终返回 nil
func TestSQLiteDriver_UseDatabase(t *testing.T) {
	d := &SQLiteDriver{}
	if err := d.UseDatabase(context.Background(), "test"); err != nil {
		t.Errorf("UseDatabase 应返回 nil: %v", err)
	}
	if err := d.UseDatabase(context.Background(), ""); err != nil {
		t.Errorf("UseDatabase 空参数应返回 nil: %v", err)
	}
}

// === Hive/Kyuubi/Spark 带 pool 的测试 ===
// 以下测试通过手动设置 pool 字段覆盖更多代码路径

// newFailingPool 创建一个 createConn 总是失败的连接池
func newFailingPool() *hiveConnPool {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        0,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 100 * time.Millisecond,
	}
	return newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, fmt.Errorf("simulated connection error")
	})
}

// TestPoolDriver_TestConnectionWithFailingPool 测试 Hive/Kyuubi/Spark 在 pool 存在但 acquire 失败时 TestConnection 返回错误
// 覆盖 hive.go:154-156、kyuubi.go:142-144、spark.go:142-144
func TestPoolDriver_TestConnectionWithFailingPool(t *testing.T) {
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
			// 使用类型断言设置 pool 字段
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

			err := d.driver.TestConnection(ctx)
			if err == nil {
				t.Error("TestConnection 应在 acquire 失败时返回错误")
			}
		})
	}
}

// TestPoolDriver_CloseWithPool 测试 Hive/Kyuubi/Spark 在有 pool 时 Close 关闭 pool
// 覆盖 hive.go:225、kyuubi.go:211、spark.go:211
func TestPoolDriver_CloseWithPool(t *testing.T) {
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
			case *KyuubiDriver:
				dd.pool = newFailingPool()
			case *SparkDriver:
				dd.pool = newFailingPool()
			}

			// Close 应关闭 pool 而不 panic
			if err := d.driver.Close(); err != nil {
				t.Errorf("Close 不应返回错误: %v", err)
			}
		})
	}
}

// TestPoolDriver_GetPoolConfigWithPool 测试 Hive/Kyuubi/Spark 在有 pool 时 GetPoolConfig 返回配置
// 覆盖 hive.go:208-214、kyuubi.go:194-200、spark.go:194-200
func TestPoolDriver_GetPoolConfigWithPool(t *testing.T) {
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

			// 通过接口断言获取 PoolConfigUpdater
			pcu, ok := d.driver.(PoolConfigUpdater)
			if !ok {
				t.Fatalf("%s 不实现 PoolConfigUpdater", d.name)
			}

			cfg := pcu.GetPoolConfig()
			if cfg.MaxOpen != 5 {
				t.Errorf("GetPoolConfig MaxOpen = %d, want 5", cfg.MaxOpen)
			}

			oc, ic, inUse, mo := pcu.GetPoolStats()
			if mo != 5 {
				t.Errorf("GetPoolStats maxOpen = %d, want 5", mo)
			}
			if oc != 0 {
				t.Errorf("GetPoolStats openCount = %d, want 0", oc)
			}
			if ic != 0 {
				t.Errorf("GetPoolStats idleCount = %d, want 0", ic)
			}
			if inUse != 0 {
				t.Errorf("GetPoolStats inUse = %d, want 0", inUse)
			}
		})
	}
}

// TestPoolDriver_UpdatePoolConfigWithPool 测试 Hive/Kyuubi/Spark 在有 pool 时 UpdatePoolConfig 更新配置
// 覆盖 hive.go:201-204、kyuubi.go:187-190、spark.go:187-190
func TestPoolDriver_UpdatePoolConfigWithPool(t *testing.T) {
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

			pcu, ok := d.driver.(PoolConfigUpdater)
			if !ok {
				t.Fatalf("%s 不实现 PoolConfigUpdater", d.name)
			}

			newCfg := PoolConfig{MaxOpen: 10, MinIdle: 3, MaxLifetime: 60 * time.Minute}
			pcu.UpdatePoolConfig(newCfg)

			got := pcu.GetPoolConfig()
			if got.MaxOpen != 10 {
				t.Errorf("UpdatePoolConfig 后 MaxOpen = %d, want 10", got.MaxOpen)
			}
		})
	}
}

// TestPoolDriver_PingWithPool 测试 Hive/Kyuubi/Spark 在有 pool 且未占满时 Ping 返回 nil
// 覆盖 hive.go:182-188、kyuubi.go:170-176、spark.go:170-176
func TestPoolDriver_PingWithPool(t *testing.T) {
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

			// pool 未占满，Ping 应返回 nil
			err := d.driver.Ping(context.Background())
			if err != nil {
				t.Errorf("Ping 应在 pool 未占满时返回 nil, got: %v", err)
			}
		})
	}
}

// === RqliteDriver.Connect 测试 ===

// TestRqliteDriver_Connect_SingleNode_Unreachable 测试 RqliteDriver Connect 单节点模式不可达主机
// 覆盖 rqlite_driver.go:22 Connect（当前 0%）
func TestRqliteDriver_Connect_SingleNode_Unreachable(t *testing.T) {
	d := &RqliteDriver{}

	err := d.Connect(context.Background(), DatasourceConfig{
		Host: "127.0.0.1",
		Port: 1,
	})
	if err == nil {
		defer d.Close()
		t.Error("不可达主机应返回错误")
	}
}

// TestRqliteDriver_Connect_MultiNode_Unreachable 测试 RqliteDriver Connect 多节点模式不可达主机
// 覆盖 rqlite_driver.go:37-75 的多节点路径
func TestRqliteDriver_Connect_MultiNode_Unreachable(t *testing.T) {
	d := &RqliteDriver{}

	err := d.Connect(context.Background(), DatasourceConfig{
		ConnectionMode: "multi",
		RqliteHosts:    "127.0.0.1:1, 127.0.0.1:2",
	})
	if err == nil {
		defer d.Close()
		t.Error("所有节点不可达应返回错误")
	}
}

// TestRqliteDriver_Connect_SingleNode_WithAuth 测试 RqliteDriver Connect 带认证
func TestRqliteDriver_Connect_SingleNode_WithAuth(t *testing.T) {
	d := &RqliteDriver{}

	err := d.Connect(context.Background(), DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "admin",
		Password: "secret",
	})
	if err == nil {
		defer d.Close()
		t.Error("不可达主机应返回错误")
	}
}

// TestRqliteDriver_Connect_SingleNode_WithSSL 测试 RqliteDriver Connect 带 SSL
func TestRqliteDriver_Connect_SingleNode_WithSSL(t *testing.T) {
	d := &RqliteDriver{}

	err := d.Connect(context.Background(), DatasourceConfig{
		Host: "127.0.0.1",
		Port: 1,
		Config: map[string]interface{}{
			"ssl": true,
		},
	})
	if err == nil {
		defer d.Close()
		t.Error("不可达主机应返回错误")
	}
}
