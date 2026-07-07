package datasource

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	rqlite "github.com/rqlite/gorqlite"

	"github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
)

// mockConfigDB 是 database.DB 的简单 mock，用于创建 system_config.Service。
// 返回空查询结果，使 Service 回退到默认配置值。
type mockConfigDB struct{}

func (m *mockConfigDB) QueryOne(sqlStatement string) (rqlite.QueryResult, error) {
	return rqlite.QueryResult{}, nil
}

func (m *mockConfigDB) QueryOneParameterized(statement rqlite.ParameterizedStatement) (rqlite.QueryResult, error) {
	return rqlite.QueryResult{}, nil
}

func (m *mockConfigDB) WriteOneParameterized(statement rqlite.ParameterizedStatement) (rqlite.WriteResult, error) {
	return rqlite.WriteResult{}, nil
}

func (m *mockConfigDB) WriteParameterized(sqlStatements []rqlite.ParameterizedStatement) ([]rqlite.WriteResult, error) {
	results := make([]rqlite.WriteResult, len(sqlStatements))
	return results, nil
}

// setupConcurrentTest 创建测试用的 ConcurrentService、miniredis 和清理函数。
// 默认配置：max_per_user=5, max_global=50, max_per_datasource=10
func setupConcurrentTest(t *testing.T) (*ConcurrentService, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to create miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	configSvc := system_config.NewService(&mockConfigDB{})
	t.Cleanup(configSvc.Close)

	svc := NewConcurrentService(client, configSvc)

	return svc, mr, client
}

// TestConcurrentService_Acquire_Success 测试正常获取并发许可
func TestConcurrentService_Acquire_Success(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	release, err := svc.Acquire(ctx, 1)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if release == nil {
		t.Fatal("release function should not be nil")
	}

	// 验证用户计数为 1
	count, err := svc.GetUserConcurrent(ctx, 1)
	if err != nil {
		t.Fatalf("GetUserConcurrent failed: %v", err)
	}
	if count != 1 {
		t.Errorf("user concurrent count = %d, want 1", count)
	}

	// 释放后计数应为 0
	release()
	count, err = svc.GetUserConcurrent(ctx, 1)
	if err != nil {
		t.Fatalf("GetUserConcurrent after release failed: %v", err)
	}
	if count != 0 {
		t.Errorf("user concurrent count after release = %d, want 0", count)
	}
}

// TestConcurrentService_Acquire_ReleaseTwice 测试 release 函数的幂等性
func TestConcurrentService_Acquire_ReleaseTwice(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	release, err := svc.Acquire(ctx, 1)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	release()
	release() // 第二次调用应该是无操作的

	count, _ := svc.GetUserConcurrent(ctx, 1)
	if count != 0 {
		t.Errorf("after double release, count = %d, want 0", count)
	}
}

// TestConcurrentService_Acquire_UserLimitExceeded 测试用户并发限制
func TestConcurrentService_Acquire_UserLimitExceeded(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	// 默认 max_per_user = 5
	var releases []func()
	for i := 0; i < 5; i++ {
		release, err := svc.Acquire(ctx, 100) // 使用不同用户避免全局限制
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
		releases = append(releases, release)
	}

	// 第 6 次应该失败
	_, err := svc.Acquire(ctx, 100)
	if err != ErrConcurrentLimit {
		t.Errorf("expected ErrConcurrentLimit, got %v", err)
	}

	// 释放一个后应该能再次获取
	releases[0]()
	release, err := svc.Acquire(ctx, 100)
	if err != nil {
		t.Errorf("Acquire after release failed: %v", err)
	}
	if release == nil {
		t.Error("release should not be nil")
	}
}

// TestConcurrentService_AcquireForDatasource_Success 测试带数据源维度的并发获取
func TestConcurrentService_AcquireForDatasource_Success(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	release, err := svc.AcquireForDatasource(ctx, 1, 100)
	if err != nil {
		t.Fatalf("AcquireForDatasource failed: %v", err)
	}
	if release == nil {
		t.Fatal("release function should not be nil")
	}

	// 验证各维度计数
	userCount, _ := svc.GetUserConcurrent(ctx, 1)
	if userCount != 1 {
		t.Errorf("user count = %d, want 1", userCount)
	}

	globalCount, _ := svc.GetGlobalConcurrent(ctx)
	if globalCount != 1 {
		t.Errorf("global count = %d, want 1", globalCount)
	}

	dsCount, _ := svc.GetDatasourceConcurrent(ctx, 100)
	if dsCount != 1 {
		t.Errorf("datasource count = %d, want 1", dsCount)
	}

	release()

	// 释放后各维度应为 0
	userCount, _ = svc.GetUserConcurrent(ctx, 1)
	if userCount != 0 {
		t.Errorf("user count after release = %d, want 0", userCount)
	}

	dsCount, _ = svc.GetDatasourceConcurrent(ctx, 100)
	if dsCount != 0 {
		t.Errorf("datasource count after release = %d, want 0", dsCount)
	}
}

// TestConcurrentService_AcquireForDatasource_DatasourceLimitExceeded 测试数据源并发限制
func TestConcurrentService_AcquireForDatasource_DatasourceLimitExceeded(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	// 默认 max_per_datasource = 10
	// 使用不同用户避免用户限制
	var releases []func()
	for i := 0; i < 10; i++ {
		release, err := svc.AcquireForDatasource(ctx, int64(1000+i), 200)
		if err != nil {
			t.Fatalf("AcquireForDatasource %d failed: %v", i, err)
		}
		releases = append(releases, release)
	}

	// 第 11 次应该触发数据源限制
	_, err := svc.AcquireForDatasource(ctx, 9999, 200)
	if err != ErrDatasourceConcurrentLimit {
		t.Errorf("expected ErrDatasourceConcurrentLimit, got %v", err)
	}

	// 释放一个后应该能再次获取
	releases[0]()
	release, err := svc.AcquireForDatasource(ctx, 9999, 200)
	if err != nil {
		t.Errorf("AcquireForDatasource after release failed: %v", err)
	}
	if release == nil {
		t.Error("release should not be nil")
	}
}

// TestConcurrentService_AcquireForDatasource_ReleaseTwice 测试 release 幂等性
func TestConcurrentService_AcquireForDatasource_ReleaseTwice(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	release, err := svc.AcquireForDatasource(ctx, 1, 100)
	if err != nil {
		t.Fatalf("AcquireForDatasource failed: %v", err)
	}

	release()
	release() // 幂等，不应导致负数

	userCount, _ := svc.GetUserConcurrent(ctx, 1)
	if userCount != 0 {
		t.Errorf("after double release, user count = %d, want 0", userCount)
	}
}

// TestConcurrentService_SetCancelSignal 测试设置取消信号
func TestConcurrentService_SetCancelSignal(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	queryID := "test-query-001"
	ttl := 30 * time.Second

	err := svc.SetCancelSignal(ctx, queryID, ttl)
	if err != nil {
		t.Fatalf("SetCancelSignal failed: %v", err)
	}

	// 验证信号已设置
	cancelKey := "datasource:query:cancel:" + queryID
	val, err := client.Get(ctx, cancelKey).Result()
	if err != nil {
		t.Fatalf("failed to get cancel signal: %v", err)
	}
	if val != "1" {
		t.Errorf("cancel signal value = %q, want '1'", val)
	}

	// 验证 TTL 已设置
	ttlLeft := client.TTL(ctx, cancelKey).Val()
	if ttlLeft <= 0 {
		t.Error("cancel signal should have TTL set")
	}
}

// TestConcurrentService_GetUserConcurrent_NotSet 测试获取未设置的计数
func TestConcurrentService_GetUserConcurrent_NotSet(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	// 未设置过，应返回 redis.Nil 错误
	_, err := svc.GetUserConcurrent(ctx, 999)
	if err == nil {
		t.Error("expected error for non-existent key, got nil")
	}
}

// TestConcurrentService_GetGlobalConcurrent_NotSet 测试获取未设置的全局计数
func TestConcurrentService_GetGlobalConcurrent_NotSet(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	_, err := svc.GetGlobalConcurrent(ctx)
	if err == nil {
		t.Error("expected error for non-existent global key, got nil")
	}
}

// TestConcurrentService_GetDatasourceConcurrent_NotSet 测试获取未设置的数据源计数
func TestConcurrentService_GetDatasourceConcurrent_NotSet(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	_, err := svc.GetDatasourceConcurrent(ctx, 999)
	if err == nil {
		t.Error("expected error for non-existent datasource key, got nil")
	}
}

// TestConcurrentService_OnConfigChanged 测试配置变更通知
func TestConcurrentService_OnConfigChanged(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	configSvc := system_config.NewService(&mockConfigDB{})
	defer configSvc.Close()

	svc := NewConcurrentService(client, configSvc)

	// 通过 configSvc.Set 更新配置，会触发观察者通知
	if err := configSvc.Set(context.Background(), "datasource.max_concurrent_per_user", "3", 0); err != nil {
		t.Fatalf("failed to set config: %v", err)
	}

	// 等待观察者通知异步生效
	time.Sleep(100 * time.Millisecond)

	// 验证配置已更新：max_per_user 应该是 3
	ctx := context.Background()
	var releases []func()
	for i := 0; i < 3; i++ {
		release, err := svc.Acquire(ctx, 200)
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
		releases = append(releases, release)
	}

	// 第 4 次应该失败（限制已改为 3）
	_, err = svc.Acquire(ctx, 200)
	if err != ErrConcurrentLimit {
		t.Errorf("expected ErrConcurrentLimit with new limit 3, got %v", err)
	}

	// 不相关的配置变更应被忽略（直接调用 OnConfigChanged 测试逻辑）
	svc.OnConfigChanged("datasource.query_timeout", "60")
}

// TestConcurrentService_Acquire_GlobalLimitExceeded 测试全局并发限制
func TestConcurrentService_Acquire_GlobalLimitExceeded(t *testing.T) {
	// 使用自定义配置：全局限制设为较小值
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	configSvc := system_config.NewService(&mockConfigDB{})
	defer configSvc.Close()

	// 设置较小的全局限制
	if err := configSvc.Set(context.Background(), "datasource.max_concurrent_global", "2", 0); err != nil {
		t.Fatalf("failed to set config: %v", err)
	}

	svc := NewConcurrentService(client, configSvc)

	ctx := context.Background()
	// 全局限制为 2，使用不同用户
	release1, err := svc.Acquire(ctx, 1)
	if err != nil {
		t.Fatalf("Acquire 1 failed: %v", err)
	}
	release2, err := svc.Acquire(ctx, 2)
	if err != nil {
		t.Fatalf("Acquire 2 failed: %v", err)
	}

	// 第 3 次应该触发全局限制
	_, err = svc.Acquire(ctx, 3)
	if err != ErrConcurrentLimit {
		t.Errorf("expected ErrConcurrentLimit for global limit, got %v", err)
	}

	release1()
	release2()
}

// TestConcurrentService_AcquireForDatasource_GlobalLimitExceeded 测试带数据源的全局限制
func TestConcurrentService_AcquireForDatasource_GlobalLimitExceeded(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to create miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	configSvc := system_config.NewService(&mockConfigDB{})
	defer configSvc.Close()

	// 设置全局限制为 2
	if err := configSvc.Set(context.Background(), "datasource.max_concurrent_global", "2", 0); err != nil {
		t.Fatalf("failed to set config: %v", err)
	}

	svc := NewConcurrentService(client, configSvc)

	ctx := context.Background()
	release1, _ := svc.AcquireForDatasource(ctx, 1, 100)
	release2, _ := svc.AcquireForDatasource(ctx, 2, 200)

	// 第 3 次不同用户不同数据源，但全局限制已满
	_, err = svc.AcquireForDatasource(ctx, 3, 300)
	if err != ErrConcurrentLimit {
		t.Errorf("expected ErrConcurrentLimit for global limit, got %v", err)
	}

	release1()
	release2()
}

// TestConcurrentService_StartStopCalibration 测试启动和停止校准
func TestConcurrentService_StartStopCalibration(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	// 启动校准（使用短间隔便于测试）
	svc.calibrationInterval = 100 * time.Millisecond
	svc.StartCalibration(func() (userCounts map[int64]int64, globalCount int64, dsCounts map[int64]int64) {
		return map[int64]int64{}, 0, map[int64]int64{}
	})

	// 停止校准（不应 panic）
	svc.StopCalibration()
}

// TestConcurrentService_CalibrateCounters 测试计数器校准逻辑
func TestConcurrentService_CalibrateCounters(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// 先在 Redis 中设置一些不一致的计数
	client.Set(ctx, "datasource:query:concurrent:user:1", 10, 5*time.Minute)
	client.Set(ctx, "datasource:query:concurrent:global", 20, 5*time.Minute)
	client.Set(ctx, "datasource:query:concurrent:ds:100", 5, 5*time.Minute)

	// 设置校准函数返回实际计数（都是 0）
	svc.getActualCount = func() (userCounts map[int64]int64, globalCount int64, dsCounts map[int64]int64) {
		return map[int64]int64{1: 0}, 0, map[int64]int64{100: 0}
	}

	svc.calibrateCounters()

	// 校准后，计数为 0 的应该被删除
	userCount, err := svc.GetUserConcurrent(ctx, 1)
	if err == nil && userCount != 0 {
		t.Errorf("after calibration, user count = %d, want 0 (deleted)", userCount)
	}

	globalCount, err := svc.GetGlobalConcurrent(ctx)
	if err == nil && globalCount != 0 {
		t.Errorf("after calibration, global count = %d, want 0 (deleted)", globalCount)
	}
}

// TestConcurrentService_CalibrateCounters_NonZero 测试校准非零计数
func TestConcurrentService_CalibrateCounters_NonZero(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// 设置校准函数返回实际计数（非零）
	svc.getActualCount = func() (userCounts map[int64]int64, globalCount int64, dsCounts map[int64]int64) {
		return map[int64]int64{1: 3}, 5, map[int64]int64{100: 2}
	}

	svc.calibrateCounters()

	// 校准后计数应与实际一致
	userCount, _ := svc.GetUserConcurrent(ctx, 1)
	if userCount != 3 {
		t.Errorf("after calibration, user count = %d, want 3", userCount)
	}

	globalCount, _ := svc.GetGlobalConcurrent(ctx)
	if globalCount != 5 {
		t.Errorf("after calibration, global count = %d, want 5", globalCount)
	}

	dsCount, _ := svc.GetDatasourceConcurrent(ctx, 100)
	if dsCount != 2 {
		t.Errorf("after calibration, datasource count = %d, want 2", dsCount)
	}
}

// TestConcurrentService_CalibrateCounters_NoCallback 测试无回调时校准不执行
func TestConcurrentService_CalibrateCounters_NoCallback(t *testing.T) {
	svc, mr, client := setupConcurrentTest(t)
	defer mr.Close()
	defer client.Close()

	// getActualCount 为 nil，应直接返回不执行
	svc.getActualCount = nil
	svc.calibrateCounters() // 不应 panic
}
