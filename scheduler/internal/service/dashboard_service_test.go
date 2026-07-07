package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	rqlite "github.com/rqlite/gorqlite"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
)

// mockCronScheduler 用于测试的 cronScheduler mock
type mockCronScheduler struct {
	paused bool
	uptime time.Duration
}

func (m *mockCronScheduler) RegisterTask(taskID int64, cronExpr string) {}
func (m *mockCronScheduler) UnregisterTask(taskID int64)                {}
func (m *mockCronScheduler) Pause()                                     { m.paused = true }
func (m *mockCronScheduler) Resume()                                    { m.paused = false }
func (m *mockCronScheduler) IsPaused() bool                             { return m.paused }
func (m *mockCronScheduler) GetUptime() time.Duration                   { return m.uptime }
func (m *mockCronScheduler) LoadAndRegisterTasks()                      {}

// mockLeaderAddrResolver 用于测试的 LeaderAddrResolver mock
type mockLeaderAddrResolver struct {
	nodeID    string
	leaderID  string
	leaderAddr string
	err       error
}

func (m *mockLeaderAddrResolver) GetLeaderHTTPAddr(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.leaderAddr, nil
}

func (m *mockLeaderAddrResolver) GetLeaderInfo(ctx context.Context) (string, string, error) {
	if m.err != nil {
		return "", "", m.err
	}
	return m.leaderID, m.leaderAddr, nil
}

func (m *mockLeaderAddrResolver) GetNodeID() string {
	return m.nodeID
}

// newDashboardSchedulerWithDB 构造用于 dashboard 测试的 SchedulerService
func newDashboardSchedulerWithDB(db *MockDB) *SchedulerService {
	return &SchedulerService{
		DB: db,
	}
}

// newDashboardSchedulerWithDBAndRedis 构造带 redis 的 SchedulerService
func newDashboardSchedulerWithDBAndRedis(t *testing.T, db *MockDB) (*SchedulerService, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, rdb := newTestRedis(t)
	svc := &SchedulerService{
		DB:    db,
		redis: rdb,
	}
	return svc, mr, rdb
}

// ============ GetDashboardStats ============

func TestGetDashboardStats_SystemAdmin(t *testing.T) {
	ctx := context.Background()
	// 4 段查询依次返回不同结果：
	// 1. taskQuery   (total, enabled, cron)         = (10, 8, 5)
	// 2. runningQuery (COUNT)                       = (3)
	// 3. recentExecQuery (success, failed, avg)     = (20, 12, 2.5)
	// 4. execQuery   (total, online)                = (4, 3)
	db := &MockDB{QueryResults: []rqlite.QueryResult{
		database.NewQueryResultWithRows([][]interface{}{
			{int64(10), int64(8), int64(5)},
		}),
		database.NewQueryResultWithRows([][]interface{}{
			{int64(3)},
		}),
		database.NewQueryResultWithRows([][]interface{}{
			{int64(20), int64(12), 2.5},
		}),
		database.NewQueryResultWithRows([][]interface{}{
			{int64(4), int64(3)},
		}),
	}}
	svc := newDashboardSchedulerWithDB(db)

	stats, err := svc.GetDashboardStats(ctx, 1, "system_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if stats == nil {
		t.Fatal("期望返回非 nil")
	}
	if stats.Tasks.Total != 10 {
		t.Errorf("期望 Tasks.Total=10，实际=%d", stats.Tasks.Total)
	}
	if stats.Tasks.Enabled != 8 {
		t.Errorf("期望 Tasks.Enabled=8，实际=%d", stats.Tasks.Enabled)
	}
	if stats.Tasks.Cron != 5 {
		t.Errorf("期望 Tasks.Cron=5，实际=%d", stats.Tasks.Cron)
	}
	if stats.Tasks.Running != 3 {
		t.Errorf("期望 Tasks.Running=3，实际=%d", stats.Tasks.Running)
	}
	if stats.Tasks.Success != 20 {
		t.Errorf("期望 Tasks.Success=20，实际=%d", stats.Tasks.Success)
	}
	if stats.Tasks.Failed != 12 {
		t.Errorf("期望 Tasks.Failed=12，实际=%d", stats.Tasks.Failed)
	}
	if stats.Tasks.AvgDuration != 2.5 {
		t.Errorf("期望 Tasks.AvgDuration=2.5，实际=%f", stats.Tasks.AvgDuration)
	}
	if stats.Executors.Total != 4 {
		t.Errorf("期望 Executors.Total=4，实际=%d", stats.Executors.Total)
	}
	if stats.Executors.Active != 3 {
		t.Errorf("期望 Executors.Active=3，实际=%d", stats.Executors.Active)
	}
	// 验证调用了 4 次查询
	if len(db.QueryStmts) != 4 {
		t.Errorf("期望 4 次查询调用，实际=%d", len(db.QueryStmts))
	}
}

// TestGetDashboardStats_TimeoutCountedAsFailed 验证 dashboard stats SQL
// 将 timeout 状态计入失败统计。
// 修复前 SQL 仅统计 status='failed'，导致 success+failed ≠ total。
func TestGetDashboardStats_TimeoutCountedAsFailed(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryResults: []rqlite.QueryResult{
		database.NewQueryResultWithRows([][]interface{}{
			{int64(0), int64(0), int64(0)},
		}),
		database.NewQueryResultWithRows([][]interface{}{
			{int64(0)},
		}),
		// success=10, failed=5 (含 timeout), avg_duration=nil
		database.NewQueryResultWithRows([][]interface{}{
			{int64(10), int64(5), nil},
		}),
		database.NewQueryResultWithRows([][]interface{}{
			{int64(0), int64(0)},
		}),
	}}
	svc := newDashboardSchedulerWithDB(db)

	stats, err := svc.GetDashboardStats(ctx, 1, "system_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}

	// 验证 recentExecQuery（第 3 段）的 SQL 包含 'timeout'
	if len(db.QueryStmts) < 3 {
		t.Fatalf("期望至少 3 次查询调用，实际=%d", len(db.QueryStmts))
	}
	recentExecSQL := db.QueryStmts[2].Query
	if !strings.Contains(recentExecSQL, "timeout") {
		t.Errorf("期望 recentExecQuery SQL 包含 'timeout'，实际: %s", recentExecSQL)
	}
	if !strings.Contains(recentExecSQL, "IN ('failed', 'timeout')") {
		t.Errorf("期望 SQL 包含 \"IN ('failed', 'timeout')\"，实际: %s", recentExecSQL)
	}
	// 验证 AVG 不再使用 ELSE 0（避免拉低均值）
	if strings.Contains(recentExecSQL, "ELSE 0 END) * 86400") {
		t.Errorf("AVG 不应使用 ELSE 0，实际: %s", recentExecSQL)
	}

	if stats.Tasks.Success != 10 {
		t.Errorf("期望 Success=10，实际=%d", stats.Tasks.Success)
	}
	if stats.Tasks.Failed != 5 {
		t.Errorf("期望 Failed=5（含 timeout），实际=%d", stats.Tasks.Failed)
	}
	// avg_duration 为 nil 时应回退为 0
	if stats.Tasks.AvgDuration != 0 {
		t.Errorf("期望 AvgDuration=0（nil 回退），实际=%f", stats.Tasks.AvgDuration)
	}
}

// TestGetTrendData_TimeoutCountedAsFailed 验证 trend SQL 将 timeout 计入失败。
func TestGetTrendData_TimeoutCountedAsFailed(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"2024-01-01", int64(10), int64(8), int64(2)},
	})
	db := &MockDB{QueryResult: qr}
	svc := newDashboardSchedulerWithDB(db)

	trends, err := svc.GetTrendData(ctx, 1, "system_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(trends) != 1 {
		t.Fatalf("期望 1 条趋势，实际 %d", len(trends))
	}
	// 验证 SQL 包含 timeout
	if len(db.QueryStmts) != 1 {
		t.Fatalf("期望 1 次查询调用，实际=%d", len(db.QueryStmts))
	}
	trendSQL := db.QueryStmts[0].Query
	if !strings.Contains(trendSQL, "IN ('failed', 'timeout')") {
		t.Errorf("期望 trend SQL 包含 \"IN ('failed', 'timeout')\"，实际: %s", trendSQL)
	}
}

func TestGetDashboardStats_AdminRole(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(50), int64(40), int64(10)},
	})
	db := &MockDB{QueryResult: qr}
	svc := newDashboardSchedulerWithDB(db)

	stats, err := svc.GetDashboardStats(ctx, 1, "admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if stats.Tasks.Total != 50 {
		t.Errorf("期望 Tasks.Total=50，实际=%d", stats.Tasks.Total)
	}
}

func TestGetDashboardStats_DomainUser(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(30), int64(20), int64(5)},
	})
	db := &MockDB{QueryResult: qr}
	svc := newDashboardSchedulerWithDB(db)

	stats, err := svc.GetDashboardStats(ctx, 1, "domain_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if stats.Tasks.Total != 30 {
		t.Errorf("期望 Tasks.Total=30，实际=%d", stats.Tasks.Total)
	}
	// 验证带参数查询被调用
	if len(db.QueryStmts) == 0 {
		t.Error("期望有查询调用记录")
	}
}

func TestGetDashboardStats_FirstQueryError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc := newDashboardSchedulerWithDB(db)

	stats, err := svc.GetDashboardStats(ctx, 1, "system_admin")
	if err == nil {
		t.Fatal("期望返回错误")
	}
	if stats != nil {
		t.Error("期望 stats 为 nil")
	}
}

func TestGetDashboardStats_WithCronScheduler(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(10), int64(5), int64(2)},
	})
	db := &MockDB{QueryResult: qr}
	cs := &mockCronScheduler{paused: true, uptime: 120 * time.Second}
	svc := &SchedulerService{
		DB:            db,
		cronScheduler: cs,
	}

	stats, err := svc.GetDashboardStats(ctx, 1, "system_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if !stats.Scheduler.Paused {
		t.Error("期望 Scheduler.Paused=true")
	}
	if stats.Scheduler.Uptime != 120 {
		t.Errorf("期望 Scheduler.Uptime=120，实际=%d", stats.Scheduler.Uptime)
	}
}

func TestGetDashboardStats_EmptyResult(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows(nil)
	db := &MockDB{QueryResult: qr}
	svc := newDashboardSchedulerWithDB(db)

	stats, err := svc.GetDashboardStats(ctx, 1, "system_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	// 空结果时 Next() 返回 false，stats 保持零值
	if stats.Tasks.Total != 0 {
		t.Errorf("期望 Tasks.Total=0，实际=%d", stats.Tasks.Total)
	}
	if stats.Executors.Total != 0 {
		t.Errorf("期望 Executors.Total=0，实际=%d", stats.Executors.Total)
	}
}

// ============ GetTrendData ============

func TestGetTrendData_Success(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"2024-01-01", int64(10), int64(8), int64(2)},
		{"2024-01-02", int64(20), int64(15), int64(5)},
	})
	db := &MockDB{QueryResult: qr}
	svc := newDashboardSchedulerWithDB(db)

	trends, err := svc.GetTrendData(ctx, 1, "system_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(trends) != 2 {
		t.Fatalf("期望 2 条趋势，实际 %d", len(trends))
	}
	if trends[0].Date != "2024-01-01" {
		t.Errorf("期望 Date=2024-01-01，实际=%s", trends[0].Date)
	}
	if trends[0].Total != 10 {
		t.Errorf("期望 Total=10，实际=%d", trends[0].Total)
	}
	if trends[0].Success != 8 {
		t.Errorf("期望 Success=8，实际=%d", trends[0].Success)
	}
	if trends[0].Failed != 2 {
		t.Errorf("期望 Failed=2，实际=%d", trends[0].Failed)
	}
}

func TestGetTrendData_Empty(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows(nil)
	db := &MockDB{QueryResult: qr}
	svc := newDashboardSchedulerWithDB(db)

	trends, err := svc.GetTrendData(ctx, 1, "system_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(trends) != 0 {
		t.Errorf("期望 0 条趋势，实际 %d", len(trends))
	}
}

func TestGetTrendData_QueryError(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryError: ErrMockDB}
	svc := newDashboardSchedulerWithDB(db)

	_, err := svc.GetTrendData(ctx, 1, "system_admin")
	if !errors.Is(err, ErrMockDB) {
		t.Errorf("期望 ErrMockDB，实际: %v", err)
	}
}

func TestGetTrendData_ResultErr(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}
	svc := newDashboardSchedulerWithDB(db)

	_, err := svc.GetTrendData(ctx, 1, "system_admin")
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestGetTrendData_DomainUser(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"2024-01-01", int64(5), int64(4), int64(1)},
	})
	db := &MockDB{QueryResult: qr}
	svc := newDashboardSchedulerWithDB(db)

	trends, err := svc.GetTrendData(ctx, 42, "domain_admin")
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if len(trends) != 1 {
		t.Fatalf("期望 1 条趋势，实际 %d", len(trends))
	}
	if trends[0].Total != 5 {
		t.Errorf("期望 Total=5，实际=%d", trends[0].Total)
	}
}

// ============ PauseScheduler / ResumeScheduler / IsSchedulerPaused ============

func TestPauseScheduler_WithCronScheduler(t *testing.T) {
	cs := &mockCronScheduler{paused: false}
	svc := &SchedulerService{cronScheduler: cs}

	svc.PauseScheduler()
	if !cs.paused {
		t.Error("期望 cronScheduler 被暂停")
	}
}

func TestPauseScheduler_NilCronScheduler(t *testing.T) {
	svc := &SchedulerService{}
	// 不应 panic
	svc.PauseScheduler()
}

func TestResumeScheduler_WithCronScheduler(t *testing.T) {
	cs := &mockCronScheduler{paused: true}
	svc := &SchedulerService{cronScheduler: cs}

	svc.ResumeScheduler()
	if cs.paused {
		t.Error("期望 cronScheduler 被恢复")
	}
}

func TestResumeScheduler_NilCronScheduler(t *testing.T) {
	svc := &SchedulerService{}
	// 不应 panic
	svc.ResumeScheduler()
}

func TestIsSchedulerPaused_Paused(t *testing.T) {
	cs := &mockCronScheduler{paused: true}
	svc := &SchedulerService{cronScheduler: cs}

	if !svc.IsSchedulerPaused() {
		t.Error("期望 true")
	}
}

func TestIsSchedulerPaused_NotPaused(t *testing.T) {
	cs := &mockCronScheduler{paused: false}
	svc := &SchedulerService{cronScheduler: cs}

	if svc.IsSchedulerPaused() {
		t.Error("期望 false")
	}
}

func TestIsSchedulerPaused_NilCronScheduler(t *testing.T) {
	svc := &SchedulerService{}
	if svc.IsSchedulerPaused() {
		t.Error("nil cronScheduler 期望 false")
	}
}

// ============ HealthCheck ============

func TestHealthCheck_AllHealthy(t *testing.T) {
	ctx := context.Background()
	// checkRQLite 两次调用 QueryOne：
	//   1. SELECT 1 —— 只检查 err/qr.Err，不读取行数据
	//   2. SELECT name FROM sqlite_master —— 迭代每行读取表名
	// MockDB 对所有 QueryOne 返回同一 QueryResult（结构体拷贝，rowNumber 重置）
	// 因此提供包含所有 requiredTables 的多行，第二查询能找到全部表
	rows := make([][]interface{}, 0, len(requiredTables))
	for _, tbl := range requiredTables {
		rows = append(rows, []interface{}{tbl})
	}
	qr := database.NewQueryResultWithRows(rows)

	svc, mr, _ := newDashboardSchedulerWithDBAndRedis(t, &MockDB{QueryResult: qr})
	defer mr.Close()
	cs := &mockCronScheduler{paused: false, uptime: 60 * time.Second}
	svc.cronScheduler = cs
	svc.leaderAddrResolver = &mockLeaderAddrResolver{
		nodeID:     "node-1",
		leaderID:   "node-1",
		leaderAddr: "localhost:8080",
	}

	result := svc.HealthCheck(ctx)
	if result == nil {
		t.Fatal("期望返回非 nil")
	}
	if result.Status != "healthy" {
		t.Errorf("期望 Status=healthy，实际=%s", result.Status)
	}
	if result.Components["rqlite"].Status != "healthy" {
		t.Errorf("期望 rqlite healthy，实际=%s", result.Components["rqlite"].Status)
	}
	if result.Components["redis"].Status != "healthy" {
		t.Errorf("期望 redis healthy，实际=%s", result.Components["redis"].Status)
	}
	if result.Components["scheduler"].Status != "healthy" {
		t.Errorf("期望 scheduler healthy，实际=%s", result.Components["scheduler"].Status)
	}
	if result.Components["leader"].Status != "healthy" {
		t.Errorf("期望 leader healthy，实际=%s", result.Components["leader"].Status)
	}
}

func TestHealthCheck_RQLiteError(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newDashboardSchedulerWithDBAndRedis(t, &MockDB{QueryError: ErrMockDB})
	defer mr.Close()

	result := svc.HealthCheck(ctx)
	if result.Status != "unhealthy" {
		t.Errorf("期望 unhealthy，实际=%s", result.Status)
	}
	if result.Components["rqlite"].Status != "unhealthy" {
		t.Errorf("期望 rqlite unhealthy，实际=%s", result.Components["rqlite"].Status)
	}
}

func TestHealthCheck_RQLiteResultErr(t *testing.T) {
	ctx := context.Background()
	db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("rqlite error"))}
	svc, mr, _ := newDashboardSchedulerWithDBAndRedis(t, db)
	defer mr.Close()

	result := svc.HealthCheck(ctx)
	if result.Status != "unhealthy" {
		t.Errorf("期望 unhealthy，实际=%s", result.Status)
	}
}

func TestHealthCheck_RedisError(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1)},
	})
	db := &MockDB{QueryResult: qr}
	svc := newDashboardSchedulerWithDB(db)
	// 使用一个不可达的 redis
	svc.redis = redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})

	result := svc.HealthCheck(ctx)
	if result.Status != "unhealthy" {
		t.Errorf("期望 unhealthy，实际=%s", result.Status)
	}
	if result.Components["redis"].Status != "unhealthy" {
		t.Errorf("期望 redis unhealthy，实际=%s", result.Components["redis"].Status)
	}
}

func TestHealthCheck_SchedulerNil(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1)},
	})
	svc, mr, _ := newDashboardSchedulerWithDBAndRedis(t, &MockDB{QueryResult: qr})
	defer mr.Close()
	// cronScheduler 为 nil

	result := svc.HealthCheck(ctx)
	if result.Components["scheduler"].Status != "unhealthy" {
		t.Errorf("期望 scheduler unhealthy，实际=%s", result.Components["scheduler"].Status)
	}
}

func TestHealthCheck_SchedulerPaused(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1)},
	})
	svc, mr, _ := newDashboardSchedulerWithDBAndRedis(t, &MockDB{QueryResult: qr})
	defer mr.Close()
	svc.cronScheduler = &mockCronScheduler{paused: true, uptime: 30 * time.Second}

	result := svc.HealthCheck(ctx)
	if result.Components["scheduler"].Status != "unhealthy" {
		t.Errorf("期望 scheduler unhealthy（已暂停），实际=%s", result.Components["scheduler"].Status)
	}
}

func TestHealthCheck_LeaderNotConfigured(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1)},
	})
	svc, mr, _ := newDashboardSchedulerWithDBAndRedis(t, &MockDB{QueryResult: qr})
	defer mr.Close()
	svc.cronScheduler = &mockCronScheduler{paused: false, uptime: 30 * time.Second}
	// leaderAddrResolver 为 nil

	result := svc.HealthCheck(ctx)
	if result.Components["leader"].Status != "unhealthy" {
		t.Errorf("期望 leader unhealthy，实际=%s", result.Components["leader"].Status)
	}
}

func TestHealthCheck_LeaderError(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1)},
	})
	svc, mr, _ := newDashboardSchedulerWithDBAndRedis(t, &MockDB{QueryResult: qr})
	defer mr.Close()
	svc.cronScheduler = &mockCronScheduler{paused: false, uptime: 30 * time.Second}
	svc.leaderAddrResolver = &mockLeaderAddrResolver{err: ErrMockDB}

	result := svc.HealthCheck(ctx)
	if result.Components["leader"].Status != "unhealthy" {
		t.Errorf("期望 leader unhealthy，实际=%s", result.Components["leader"].Status)
	}
}

func TestHealthCheck_NoLeader(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1)},
	})
	svc, mr, _ := newDashboardSchedulerWithDBAndRedis(t, &MockDB{QueryResult: qr})
	defer mr.Close()
	svc.cronScheduler = &mockCronScheduler{paused: false, uptime: 30 * time.Second}
	svc.leaderAddrResolver = &mockLeaderAddrResolver{
		nodeID:   "node-1",
		leaderID: "", // 无 leader
	}

	result := svc.HealthCheck(ctx)
	if result.Components["leader"].Status != "unhealthy" {
		t.Errorf("期望 leader unhealthy（无 leader），实际=%s", result.Components["leader"].Status)
	}
}

func TestHealthCheck_CurrentNodeIsLeader(t *testing.T) {
	ctx := context.Background()
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(1)},
	})
	svc, mr, _ := newDashboardSchedulerWithDBAndRedis(t, &MockDB{QueryResult: qr})
	defer mr.Close()
	svc.cronScheduler = &mockCronScheduler{paused: false, uptime: 30 * time.Second}
	svc.leaderAddrResolver = &mockLeaderAddrResolver{
		nodeID:    "node-1",
		leaderID:  "node-1",
		leaderAddr: "localhost:8080",
	}

	result := svc.HealthCheck(ctx)
	if result.Components["leader"].Status != "healthy" {
		t.Errorf("期望 leader healthy，实际=%s", result.Components["leader"].Status)
	}
}

// ============ formatLatency ============

func TestFormatLatency(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"nanoseconds", 500 * time.Nanosecond, "500.00ns"},
		{"microseconds", 500 * time.Microsecond, "500.00μs"},
		{"milliseconds", 500 * time.Millisecond, "500.00ms"},
		{"seconds", 5 * time.Second, "5.00s"},
		{"sub-microsecond", 100 * time.Nanosecond, "100.00ns"},
		{"exactly 1 microsecond", 1 * time.Microsecond, "1.00μs"},
		{"exactly 1 millisecond", 1 * time.Millisecond, "1.00ms"},
		{"exactly 1 second", 1 * time.Second, "1.00s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLatency(tt.duration)
			if result != tt.expected {
				t.Errorf("期望 %q，实际 %q", tt.expected, result)
			}
		})
	}
}

// ============ formatUptime ============

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"minutes only", 5 * time.Minute, "5m"},
		{"hours and minutes", 90 * time.Minute, "1h30m"},
		{"days hours minutes", 25*time.Hour + 30*time.Minute, "1d1h30m"},
		{"zero", 0, "0m"},
		{"less than a minute", 30 * time.Second, "0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatUptime(tt.duration)
			if result != tt.expected {
				t.Errorf("期望 %q，实际 %q", tt.expected, result)
			}
		})
	}
}

// ============ DashboardStats / TrendData 结构体 ============

func TestDashboardStats_Struct(t *testing.T) {
	stats := &DashboardStats{}
	stats.Tasks.Total = 100
	stats.Tasks.Enabled = 80
	stats.Tasks.Cron = 20
	stats.Tasks.Running = 5
	stats.Tasks.Success = 200
	stats.Tasks.Failed = 10
	stats.Tasks.AvgDuration = 3.14
	stats.Executors.Total = 10
	stats.Executors.Active = 8
	stats.Scheduler.Paused = false
	stats.Scheduler.Uptime = 3600

	if stats.Tasks.Total != 100 {
		t.Errorf("期望 Tasks.Total=100，实际=%d", stats.Tasks.Total)
	}
	if stats.Tasks.AvgDuration != 3.14 {
		t.Errorf("期望 Tasks.AvgDuration=3.14，实际=%f", stats.Tasks.AvgDuration)
	}
	if stats.Scheduler.Uptime != 3600 {
		t.Errorf("期望 Scheduler.Uptime=3600，实际=%d", stats.Scheduler.Uptime)
	}
}

func TestTrendData_Struct(t *testing.T) {
	td := &TrendData{
		Date:    "2024-01-01",
		Total:   10,
		Success: 8,
		Failed:  2,
	}
	if td.Date != "2024-01-01" {
		t.Errorf("期望 Date=2024-01-01，实际=%s", td.Date)
	}
	if td.Total != 10 || td.Success != 8 || td.Failed != 2 {
		t.Errorf("字段值不正确: Total=%d Success=%d Failed=%d", td.Total, td.Success, td.Failed)
	}
}

func TestHealthCheckResult_Struct(t *testing.T) {
	result := &HealthCheckResult{
		Status:    "healthy",
		Timestamp: "2024-01-01T00:00:00Z",
		Components: map[string]ComponentCheck{
			"rqlite": {Status: "healthy", Message: "OK", Latency: "1.00ms"},
		},
	}
	if result.Status != "healthy" {
		t.Errorf("期望 Status=healthy，实际=%s", result.Status)
	}
	if result.Components["rqlite"].Status != "healthy" {
		t.Error("期望 rqlite healthy")
	}
}
