package driver

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	gohive "github.com/beltran/gohive"
)

// TestHiveDriverUseDatabaseNoLongerBlocks 测试 UseDatabase 不再阻塞
// 在连接池架构下，UseDatabase 只更新 defaultDB，不获取任何锁
func TestHiveDriverUseDatabaseNoLongerBlocks(t *testing.T) {
	d := &HiveDriver{
		config:    DatasourceConfig{Database: "default_db"},
		defaultDB: "default_db",
	}

	// UseDatabase 应立即返回，不阻塞
	done := make(chan error, 1)
	go func() {
		done <- d.UseDatabase(context.Background(), "new_db")
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("UseDatabase should not block, got error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("UseDatabase should not block in pool architecture")
	}

	if d.defaultDB != "new_db" {
		t.Errorf("defaultDB should be 'new_db', got '%s'", d.defaultDB)
	}
}

// TestHiveDriverUseDatabaseEmptyValue 测试 UseDatabase 空值处理
func TestHiveDriverUseDatabaseEmptyValue(t *testing.T) {
	d := &HiveDriver{
		defaultDB: "original_db",
	}

	err := d.UseDatabase(context.Background(), "")
	if err != nil {
		t.Errorf("UseDatabase with empty string should not error, got: %v", err)
	}
	if d.defaultDB != "original_db" {
		t.Errorf("defaultDB should not change with empty string, got '%s'", d.defaultDB)
	}
}

// TestHiveDriverQueryWithDBWithoutPool 测试无连接池时 QueryWithDB 报错
func TestHiveDriverQueryWithDBWithoutPool(t *testing.T) {
	d := &HiveDriver{
		config:    DatasourceConfig{Database: "test"},
		defaultDB: "test",
	}

	_, err := d.QueryWithDB(context.Background(), "SELECT 1", "test_db")
	if err == nil {
		t.Error("QueryWithDB without pool should return error")
	}
}

// TestHiveDriverQueryDelegatesToQueryWithDB 测试 Query 委托给 QueryWithDB
func TestHiveDriverQueryDelegatesToQueryWithDB(t *testing.T) {
	d := &HiveDriver{
		config:    DatasourceConfig{Database: "default_db"},
		defaultDB: "default_db",
	}

	// 没有连接池，Query 也应该报错（因为委托给 QueryWithDB）
	_, err := d.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Error("Query without pool should return error")
	}
}

// TestHiveDriverCloseWithPool 测试 Close 关闭连接池
func TestHiveDriverCloseWithPool(t *testing.T) {
	d := &HiveDriver{}
	err := d.Close()
	if err != nil {
		t.Errorf("Close with nil pool should not error, got: %v", err)
	}
}

// TestHiveDriverIsUnhealthy 测试 IsUnhealthy
func TestHiveDriverIsUnhealthy(t *testing.T) {
	d := &HiveDriver{}
	if d.IsUnhealthy() {
		t.Error("New HiveDriver should not be unhealthy")
	}
	d.unhealthy.Store(true)
	if !d.IsUnhealthy() {
		t.Error("HiveDriver should be unhealthy after Store(true)")
	}
}

// TestHiveDriverSupportsCancelPool 测试 SupportsCancel
func TestHiveDriverSupportsCancelPool(t *testing.T) {
	d := &HiveDriver{}
	if !d.SupportsCancel() {
		t.Error("HiveDriver should support cancel")
	}
}

// TestHiveDriverGetDatabasesWithoutConnection 测试无连接时 GetDatabases 报错
func TestHiveDriverGetDatabasesWithoutConnection(t *testing.T) {
	d := &HiveDriver{
		config: DatasourceConfig{Database: "test"},
	}
	_, err := d.GetDatabases(context.Background())
	if err == nil {
		t.Error("GetDatabases without connection should return error")
	}
}

// TestHiveDriverGetTablesWithoutConnection 测试无连接时 GetTables 报错
func TestHiveDriverGetTablesWithoutConnection(t *testing.T) {
	d := &HiveDriver{
		config: DatasourceConfig{Database: "test"},
	}
	_, err := d.GetTables(context.Background(), "test_db")
	if err == nil {
		t.Error("GetTables without connection should return error")
	}
}

// TestHiveDriverGetColumnsWithoutConnection 测试无连接时 GetColumns 报错
func TestHiveDriverGetColumnsWithoutConnection(t *testing.T) {
	d := &HiveDriver{
		config: DatasourceConfig{Database: "test"},
	}
	_, err := d.GetColumns(context.Background(), "test_db", "test_table")
	if err == nil {
		t.Error("GetColumns without connection should return error")
	}
}

// TestHiveConnPoolBasic 测试连接池基本操作
func TestHiveConnPoolBasic(t *testing.T) {
	pool := newHiveConnPool(PoolConfig{MaxOpen: 3, MinIdle: 1}, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})

	// 检查初始状态
	open, idle, maxOpen := pool.stats()
	if open != 0 || idle != 0 || maxOpen != 3 {
		t.Errorf("Initial pool should be empty, got open=%d idle=%d maxOpen=%d", open, idle, maxOpen)
	}
}

// TestHiveConnPoolStats 测试连接池统计
func TestHiveConnPoolStats(t *testing.T) {
	pool := newHiveConnPool(PoolConfig{MaxOpen: 5, MinIdle: 2}, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})

	open, idle, maxOpen := pool.stats()
	if open != 0 {
		t.Errorf("Initial open count should be 0, got %d", open)
	}
	if idle != 0 {
		t.Errorf("Initial idle count should be 0, got %d", idle)
	}
	if maxOpen != 5 {
		t.Errorf("MaxOpen should be 5, got %d", maxOpen)
	}
}

// TestHiveConnPoolClose 测试连接池关闭
func TestHiveConnPoolClose(t *testing.T) {
	pool := newHiveConnPool(PoolConfig{MaxOpen: 5, MinIdle: 2}, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	pool.close()

	// 关闭后不应 panic
	open, _, _ := pool.stats()
	if open != 0 {
		t.Errorf("After close, open count should be 0, got %d", open)
	}
}

// TestHiveDriverDefaultDB 测试 defaultDB 设置
func TestHiveDriverDefaultDB(t *testing.T) {
	d := &HiveDriver{
		config: DatasourceConfig{Database: "my_db"},
	}
	d.defaultDB = d.config.Database

	if d.defaultDB != "my_db" {
		t.Errorf("defaultDB should be 'my_db', got '%s'", d.defaultDB)
	}
}

// TestHiveDriverConcurrentUseDatabase 测试并发 UseDatabase 不会阻塞
func TestHiveDriverConcurrentUseDatabase(t *testing.T) {
	d := &HiveDriver{
		defaultDB: "default",
	}

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			dbName := fmt.Sprintf("db_%d", idx)
			if err := d.UseDatabase(context.Background(), dbName); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent UseDatabase should not error: %v", err)
	}
}

// TestHiveDriverQueryCancelledContext 测试 Query 在 context 取消时返回错误
func TestHiveDriverQueryCancelledContext(t *testing.T) {
	d := &HiveDriver{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := d.Query(ctx, "SELECT 1")
	if err == nil {
		t.Error("Query with cancelled context should return error")
	}
}

// TestHiveDriverPingWithoutPool 测试无连接池时 Ping
func TestHiveDriverPingWithoutPool(t *testing.T) {
	d := &HiveDriver{}
	err := d.Ping(context.Background())
	if err == nil {
		t.Error("Ping without pool should return error")
	}
}

// TestHiveDriverTestConnectionWithoutPool 测试无连接池时 TestConnection
func TestHiveDriverTestConnectionWithoutPool(t *testing.T) {
	d := &HiveDriver{}
	err := d.TestConnection(context.Background())
	if err == nil {
		t.Error("TestConnection without pool should return error")
	}
}

// TestHiveDriverGetTablesUsesConfigDatabase 测试 GetTables 使用默认 database
func TestHiveDriverGetTablesUsesConfigDatabase(t *testing.T) {
	d := &HiveDriver{
		config: DatasourceConfig{Database: "default_db"},
	}
	_, err := d.GetTables(context.Background(), "")
	if err == nil {
		t.Error("GetTables without connection should return error")
	}
}

// TestHiveDriverGetColumnsUsesConfigDatabase 测试 GetColumns 使用默认 database
func TestHiveDriverGetColumnsUsesConfigDatabase(t *testing.T) {
	d := &HiveDriver{
		config: DatasourceConfig{Database: "default_db"},
	}
	_, err := d.GetColumns(context.Background(), "", "test_table")
	if err == nil {
		t.Error("GetColumns without connection should return error")
	}
}
