package datasource

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

// mockDriver 是 driver.Driver 接口的测试 mock
type mockDriver struct {
	connectErr      error
	testConnErr     error
	pingErr         error
	closeErr        error
	unhealthy       bool
	poolOpenCount   int
	poolIdleCount   int
	poolInUse       int
	poolMaxOpen     int
	poolConfig      driver.PoolConfig
	queryResult     *driver.QueryResult
	queryErr        error
	databases       []string
	databasesErr    error
	tables          []driver.TableInfo
	tablesErr       error
	columns         []driver.ColumnInfo
	columnsErr      error
	supportsCancel  bool
	useDatabaseErr  error
}

func (m *mockDriver) Connect(ctx context.Context, config driver.DatasourceConfig) error {
	return m.connectErr
}
func (m *mockDriver) TestConnection(ctx context.Context) error {
	return m.testConnErr
}
func (m *mockDriver) Ping(ctx context.Context) error {
	return m.pingErr
}
func (m *mockDriver) Close() error {
	return m.closeErr
}
func (m *mockDriver) Query(ctx context.Context, sql string, args ...interface{}) (*driver.QueryResult, error) {
	return m.queryResult, m.queryErr
}
func (m *mockDriver) QueryWithDB(ctx context.Context, sql string, database string) (*driver.QueryResult, error) {
	return m.queryResult, m.queryErr
}
func (m *mockDriver) TryQueryWithDB(ctx context.Context, sql string, database string) (*driver.QueryResult, error) {
	return m.queryResult, m.queryErr
}
func (m *mockDriver) GetDatabases(ctx context.Context) ([]string, error) {
	return m.databases, m.databasesErr
}
func (m *mockDriver) GetTables(ctx context.Context, database string) ([]driver.TableInfo, error) {
	return m.tables, m.tablesErr
}
func (m *mockDriver) GetColumns(ctx context.Context, database, table string) ([]driver.ColumnInfo, error) {
	return m.columns, m.columnsErr
}
func (m *mockDriver) SupportsCancel() bool {
	return m.supportsCancel
}
func (m *mockDriver) UseDatabase(ctx context.Context, database string) error {
	return m.useDatabaseErr
}

// 实现 UnhealthyChecker 接口
func (m *mockDriver) IsUnhealthy() bool {
	return m.unhealthy
}
func (m *mockDriver) MarkUnhealthy() {
	m.unhealthy = true
}

// 实现 PoolConfigUpdater 接口
func (m *mockDriver) UpdatePoolConfig(cfg driver.PoolConfig) {
	m.poolConfig = cfg
}
func (m *mockDriver) GetPoolConfig() driver.PoolConfig {
	return m.poolConfig
}
func (m *mockDriver) GetPoolStats() (int, int, int, int) {
	return m.poolOpenCount, m.poolIdleCount, m.poolInUse, m.poolMaxOpen
}

// ==================== TestDatasource ====================

func TestDatasourceService_TestDatasource_NotFound(t *testing.T) {
	db := &dsMockDB{
		queryResult: database.NewQueryResultWithRows([][]interface{}{}),
	}
	svc := NewDatasourceService(db, nil, NewManager(nil, nil))

	err := svc.TestDatasource(context.Background(), 999)
	if !errors.Is(err, ErrDatasourceNotFound) {
		t.Errorf("expected ErrDatasourceNotFound, got %v", err)
	}
}

func TestDatasourceService_TestDatasource_ConnectionFailed(t *testing.T) {
	// GetByID 返回一个不支持类型的数据源，TestConnection 会失败
	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{makeDSRow()}), // GetByID
		},
		writeResult: database.NewWriteResult(0, 1), // 更新 test_status
	}
	mgr := NewManager(nil, nil)
	svc := NewDatasourceService(db, nil, mgr)

	err := svc.TestDatasource(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error on test connection failure")
	}
	// 由于 type=mysql 但没有真实的 MySQL 连接，TestConnection 会失败
	// 验证写入了更新 test_status 的语句
	if len(db.writeStmts) < 1 {
		t.Errorf("expected at least 1 write for test status update, got %d", len(db.writeStmts))
	}
}

func TestDatasourceService_TestDatasource_UnsupportedType(t *testing.T) {
	// 使用不支持的数据源类型
	row := makeDSRow()
	row[2] = "unsupported-type" // type

	db := &dsMockDB{
		queryResults: []rqlite.QueryResult{
			database.NewQueryResultWithRows([][]interface{}{row}),
		},
		writeResult: database.NewWriteResult(0, 1),
	}
	mgr := NewManager(nil, nil)
	svc := NewDatasourceService(db, nil, mgr)

	err := svc.TestDatasource(context.Background(), 1)
	if !errors.Is(err, ErrDatasourceTypeNotSupport) {
		t.Errorf("expected ErrDatasourceTypeNotSupport, got %v", err)
	}

	// 验证写入了 "failed" 状态
	if len(db.writeStmts) != 1 {
		t.Fatalf("expected 1 write, got %d", len(db.writeStmts))
	}
	statusArg := db.writeStmts[0].Arguments[0]
	if statusArg != "failed" {
		t.Errorf("expected test_status='failed', got %v", statusArg)
	}
}

func TestDatasourceService_TestDatasource_GetByIDError(t *testing.T) {
	db := &dsMockDB{
		queryError: errors.New("query failed"),
	}
	svc := NewDatasourceService(db, nil, NewManager(nil, nil))

	err := svc.TestDatasource(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error on GetByID failure")
	}
}

// ==================== checkAllConnections ====================

func TestManager_CheckAllConnections_Empty(t *testing.T) {
	mgr := NewManager(nil, nil)
	// 没有任何连接，不应 panic
	mgr.checkAllConnections()
}

func TestManager_CheckAllConnections_WithMockDriver(t *testing.T) {
	mgr := NewManager(nil, nil)

	// 添加一个模拟驱动到 pools 中
	md := &mockDriver{
		testConnErr:  nil, // TestConnection 成功
		poolOpenCount: 1,
		poolIdleCount: 1,
		poolInUse:     0,
		poolMaxOpen:   5,
	}
	mgr.poolMu.Lock()
	mgr.pools[1] = md
	mgr.poolMu.Unlock()

	// 不应 panic，且应记录成功
	mgr.checkAllConnections()

	// 验证熔断器状态
	state, failures := mgr.GetCircuitBreakerState(1)
	if state != CircuitStateClosed {
		t.Errorf("expected Closed state after successful check, got %v", state)
	}
	if failures != 0 {
		t.Errorf("expected 0 failures, got %d", failures)
	}
}

func TestManager_CheckAllConnections_DriverFailure(t *testing.T) {
	mgr := NewManager(nil, nil)

	md := &mockDriver{
		testConnErr:  errors.New("connection lost"),
		poolOpenCount: 1,
		poolIdleCount: 0,
		poolInUse:     0,
		poolMaxOpen:   5,
	}
	mgr.poolMu.Lock()
	mgr.pools[1] = md
	mgr.poolMu.Unlock()

	mgr.checkAllConnections()

	// 验证熔断器记录了失败
	state, failures := mgr.GetCircuitBreakerState(1)
	if failures == 0 {
		t.Error("expected failures > 0 after failed check")
	}
	_ = state // 状态取决于失败次数是否达到阈值
}

func TestManager_CheckAllConnections_PoolFull(t *testing.T) {
	mgr := NewManager(nil, nil)

	// 连接池满（inUse >= maxOpen），应跳过检查
	md := &mockDriver{
		testConnErr:   errors.New("should not be called"),
		poolOpenCount: 5,
		poolIdleCount: 0,
		poolInUse:     5,
		poolMaxOpen:   5,
	}
	mgr.poolMu.Lock()
	mgr.pools[1] = md
	mgr.poolMu.Unlock()

	mgr.checkAllConnections()

	// 连接池满时应跳过，不调用 TestConnection
	// 熔断器应记录成功（cb.RecordSuccess）
	state, failures := mgr.GetCircuitBreakerState(1)
	if state != CircuitStateClosed {
		t.Errorf("expected Closed state when pool full, got %v", state)
	}
	if failures != 0 {
		t.Errorf("expected 0 failures when pool full, got %d", failures)
	}
}

func TestManager_CheckAllConnections_MarksUnhealthy(t *testing.T) {
	mgr := NewManager(nil, nil)

	md := &mockDriver{
		testConnErr: errors.New("connection lost"),
		unhealthy:   false,
		poolOpenCount: 1,
		poolIdleCount: 1,
		poolInUse:     0,
		poolMaxOpen:   5,
	}
	mgr.poolMu.Lock()
	mgr.pools[1] = md
	mgr.poolMu.Unlock()

	mgr.checkAllConnections()

	// 验证驱动被标记为不健康
	if !md.unhealthy {
		t.Error("expected driver to be marked unhealthy after failed check")
	}
}

// ==================== GetDriver (additional paths) ====================

func TestManager_GetDriver_CircuitBreakerOpen(t *testing.T) {
	mgr := NewManager(nil, nil)

	// 手动设置熔断器为 Open 状态
	cb := mgr.getCircuitBreaker(1)
	cb.failureThreshold = 1
	cb.RecordFailure() // 触发熔断

	ds := &model.Datasource{
		ID:        1,
		Type:      "mysql",
		IsEnabled: true,
	}

	_, err := mgr.GetDriver(context.Background(), ds)
	if !errors.Is(err, ErrDatasourceCircuitOpen) {
		t.Errorf("expected ErrDatasourceCircuitOpen, got %v", err)
	}
}

func TestManager_GetDriver_AlreadyConnecting(t *testing.T) {
	mgr := NewManager(nil, nil)

	ds := &model.Datasource{
		ID:        1,
		Type:      "mysql",
		IsEnabled: true,
		Host:      "localhost",
		Port:      3306,
	}

	// 模拟正在连接中
	mgr.connectMu.Lock()
	mgr.connecting[1] = struct{}{}
	mgr.connectMu.Unlock()

	_, err := mgr.GetDriver(context.Background(), ds)
	if err == nil {
		t.Fatal("expected error when datasource is already connecting")
	}
}

func TestManager_GetDriver_ExistingHealthyDriver(t *testing.T) {
	mgr := NewManager(nil, nil)

	// 添加一个健康的 mock 驱动
	md := &mockDriver{
		pingErr: nil,
	}
	mgr.poolMu.Lock()
	mgr.pools[1] = md
	mgr.poolMu.Unlock()

	// 设置 lastCheck 为当前时间，使健康检查间隔内直接返回
	mgr.checkMu.Lock()
	mgr.lastCheck[1] = time.Now()
	mgr.checkMu.Unlock()

	ds := &model.Datasource{
		ID:        1,
		Type:      "mysql",
		IsEnabled: true,
	}

	drv, err := mgr.GetDriver(context.Background(), ds)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if drv == nil {
		t.Fatal("expected non-nil driver")
	}
}

func TestManager_GetDriver_UnhealthyDriver(t *testing.T) {
	mgr := NewManager(nil, nil)

	// 添加一个不健康的 mock 驱动
	md := &mockDriver{
		unhealthy: true,
	}
	mgr.poolMu.Lock()
	mgr.pools[1] = md
	mgr.poolMu.Unlock()

	ds := &model.Datasource{
		ID:        1,
		Type:      "mysql",
		IsEnabled: true,
		Host:      "localhost",
		Port:      3306,
	}

	// 不健康的驱动会被关闭并删除，然后尝试重新连接
	// 由于 type=mysql 但没有真实连接，connect 会失败
	_, err := mgr.GetDriver(context.Background(), ds)
	if err == nil {
		t.Fatal("expected error when reconnecting with unsupported/invalid connection")
	}
}

func TestManager_GetDriver_PingFailure(t *testing.T) {
	mgr := NewManager(nil, nil)

	// 添加一个 Ping 失败的 mock 驱动
	md := &mockDriver{
		pingErr: errors.New("ping failed"),
	}
	mgr.poolMu.Lock()
	mgr.pools[1] = md
	mgr.poolMu.Unlock()

	// lastCheck 设为很久以前，触发 Ping
	mgr.checkMu.Lock()
	mgr.lastCheck[1] = time.Now().Add(-1 * time.Hour)
	mgr.checkMu.Unlock()

	ds := &model.Datasource{
		ID:        1,
		Type:      "mysql",
		IsEnabled: true,
		Host:      "localhost",
		Port:      3306,
	}

	// Ping 失败后会尝试重新连接，由于没有真实连接会失败
	_, err := mgr.GetDriver(context.Background(), ds)
	if err == nil {
		t.Fatal("expected error after ping failure and reconnection attempt")
	}
}

func TestManager_GetDriver_PoolBusy(t *testing.T) {
	mgr := NewManager(nil, nil)

	// 添加一个 Ping 返回 "pool fully occupied" 的 mock 驱动
	md := &mockDriver{
		pingErr: errors.New("pool fully occupied"),
	}
	mgr.poolMu.Lock()
	mgr.pools[1] = md
	mgr.poolMu.Unlock()

	// lastCheck 设为很久以前，触发 Ping
	mgr.checkMu.Lock()
	mgr.lastCheck[1] = time.Now().Add(-1 * time.Hour)
	mgr.checkMu.Unlock()

	ds := &model.Datasource{
		ID:        1,
		Type:      "mysql",
		IsEnabled: true,
	}

	drv, err := mgr.GetDriver(context.Background(), ds)
	if err != nil {
		t.Fatalf("expected no error for pool busy, got %v", err)
	}
	if drv == nil {
		t.Fatal("expected non-nil driver (existing driver returned for pool busy)")
	}
}
