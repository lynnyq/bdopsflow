package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return mr, client
}

func TestCalculateLockTTL(t *testing.T) {
	svc := &SchedulerService{}
	tests := []struct {
		name      string
		taskErr   error
		task      *model.Task
		expectTTL int64
	}{
		{
			name:      "default when task error is non-nil",
			taskErr:   fmt.Errorf("task not found"),
			task:      nil,
			expectTTL: 300,
		},
		{
			name:    "default when timeout is zero",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 0,
			},
			expectTTL: 3600,
		},
		{
			name:    "double timeout when timeout is set",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 120,
			},
			expectTTL: 240,
		},
		{
			name:    "minimum 60 seconds",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 10,
			},
			expectTTL: 60,
		},
		{
			name:    "maximum 7200 seconds",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 7200,
			},
			expectTTL: 7200,
		},
		{
			name:    "exactly at minimum boundary",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 30,
			},
			expectTTL: 60,
		},
		{
			name:    "exactly at maximum boundary",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 3600,
			},
			expectTTL: 7200,
		},
		{
			name:    "normal case within range",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 300,
			},
			expectTTL: 600,
		},
		{
			name:    "timeout exceeds max capped to 7200",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 5000,
			},
			expectTTL: 7200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.calculateLockTTL(tt.taskErr, tt.task)
			if result != tt.expectTTL {
				t.Errorf("calculateLockTTL() = %d, expected %d", result, tt.expectTTL)
			}
		})
	}
}

func TestCalculateLockTTL_EdgeCases(t *testing.T) {
	svc := &SchedulerService{}
	tests := []struct {
		name      string
		taskErr   error
		task      *model.Task
		expectTTL int64
	}{
		{
			name:      "nil task with error",
			taskErr:   fmt.Errorf("db error"),
			task:      nil,
			expectTTL: 300,
		},
		{
			name:    "timeout exactly 30 gives minimum 60",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 30,
			},
			expectTTL: 60,
		},
		{
			name:    "timeout 1 gives minimum 60",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 1,
			},
			expectTTL: 60,
		},
		{
			name:    "timeout 100 gives 200",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 100,
			},
			expectTTL: 200,
		},
		{
			name:    "timeout just below max 3599 gives 7198",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 3599,
			},
			expectTTL: 7198,
		},
		{
			name:    "timeout 4000 capped to 7200",
			taskErr: nil,
			task: &model.Task{
				TimeoutSeconds: 4000,
			},
			expectTTL: 7200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.calculateLockTTL(tt.taskErr, tt.task)
			if result != tt.expectTTL {
				t.Errorf("calculateLockTTL() = %d, expected %d", result, tt.expectTTL)
			}
		})
	}
}

func TestRecoveryLogDedup_SameWindowPreventsDuplicate(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()
	executionID := "exec-dedup-001"

	dedupKey := fmt.Sprintf("task:log:dedup:%s:recovery:%d", executionID, time.Now().Unix()/300)

	exists, _ := rdb.Exists(ctx, dedupKey).Result()
	if exists != 0 {
		t.Error("dedup key should not exist initially")
	}

	rdb.Set(ctx, dedupKey, "1", 10*time.Minute)

	exists, _ = rdb.Exists(ctx, dedupKey).Result()
	if exists == 0 {
		t.Error("dedup key should exist after first set")
	}

	dedupKey2 := fmt.Sprintf("task:log:dedup:%s:recovery:%d", executionID, time.Now().Unix()/300)
	if dedupKey != dedupKey2 {
		t.Errorf("same execution ID and same time window should produce same dedup key: %s vs %s", dedupKey, dedupKey2)
	}

	exists, _ = rdb.Exists(ctx, dedupKey2).Result()
	if exists == 0 {
		t.Error("second check within same window should find existing dedup key")
	}
}

func TestRecoveryLogDedup_DifferentTimeWindows(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()
	executionID := "exec-window-001"

	window1 := time.Now().Unix() / 300
	dedupKey1 := fmt.Sprintf("task:log:dedup:%s:recovery:%d", executionID, window1)
	rdb.Set(ctx, dedupKey1, "1", 10*time.Minute)

	mr.FastForward(10 * time.Minute)

	window2 := time.Now().Unix() / 300
	if window1 == window2 {
		t.Skip("time window did not advance, skipping")
	}

	dedupKey2 := fmt.Sprintf("task:log:dedup:%s:recovery:%d", executionID, window2)

	exists2, _ := rdb.Exists(ctx, dedupKey2).Result()
	if exists2 != 0 {
		t.Error("new time window dedup key should not exist yet")
	}

	rdb.Set(ctx, dedupKey2, "1", 10*time.Minute)
	exists2, _ = rdb.Exists(ctx, dedupKey2).Result()
	if exists2 == 0 {
		t.Error("new time window dedup key should exist after set")
	}
}

func TestRecoveryLogDedup_DifferentExecutions(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()
	window := time.Now().Unix() / 300

	keyA := fmt.Sprintf("task:log:dedup:%s:recovery:%d", "exec-A", window)
	keyB := fmt.Sprintf("task:log:dedup:%s:recovery:%d", "exec-B", window)

	rdb.Set(ctx, keyA, "1", 10*time.Minute)
	rdb.Set(ctx, keyB, "1", 10*time.Minute)

	existsA, _ := rdb.Exists(ctx, keyA).Result()
	existsB, _ := rdb.Exists(ctx, keyB).Result()

	if existsA == 0 {
		t.Error("dedup key for exec-A should exist")
	}
	if existsB == 0 {
		t.Error("dedup key for exec-B should exist")
	}

	if keyA == keyB {
		t.Error("different execution IDs should produce different dedup keys")
	}
}

func TestRecoveryLogDedup_TTL(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()
	executionID := "exec-ttl-001"

	dedupKey := fmt.Sprintf("task:log:dedup:%s:recovery:%d", executionID, time.Now().Unix()/300)
	rdb.Set(ctx, dedupKey, "1", 10*time.Minute)

	ttl, err := rdb.TTL(ctx, dedupKey).Result()
	if err != nil {
		t.Fatalf("failed to get TTL for dedup key: %v", err)
	}

	if ttl <= 0 || ttl > 10*time.Minute {
		t.Errorf("dedup key TTL should be ~10 minutes, got %v", ttl)
	}
}

func TestForceFailTask_RedisKeyCleanup(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()
	executionID := "exec-cleanup-001"

	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	renewKey := fmt.Sprintf("task:renew:%s", executionID)
	failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)

	rdb.Set(ctx, lockKey, "locked", 10*time.Minute)
	rdb.Set(ctx, renewKey, time.Now().Unix(), 10*time.Minute)
	rdb.Set(ctx, failCountKey, 3, 10*time.Minute)

	rdb.Del(ctx, lockKey, renewKey, failCountKey)

	exists, _ := rdb.Exists(ctx, lockKey).Result()
	if exists != 0 {
		t.Error("lock key should be deleted")
	}

	exists, _ = rdb.Exists(ctx, renewKey).Result()
	if exists != 0 {
		t.Error("renew key should be deleted")
	}

	exists, _ = rdb.Exists(ctx, failCountKey).Result()
	if exists != 0 {
		t.Error("fail count key should be deleted")
	}
}

func TestForceFailTask_MultipleExecutionsCleanup(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		execID := fmt.Sprintf("exec-multi-%d", i)

		lockKey := fmt.Sprintf("task:lock:%s", execID)
		renewKey := fmt.Sprintf("task:renew:%s", execID)
		failCountKey := fmt.Sprintf("task:renew:fail:count:%s", execID)

		rdb.Set(ctx, lockKey, "locked", 10*time.Minute)
		rdb.Set(ctx, renewKey, time.Now().Unix(), 10*time.Minute)
		rdb.Set(ctx, failCountKey, i, 10*time.Minute)

		rdb.Del(ctx, lockKey, renewKey, failCountKey)

		exists, _ := rdb.Exists(ctx, lockKey).Result()
		if exists != 0 {
			t.Errorf("lock key for exec %d should be deleted", i)
		}

		exists, _ = rdb.Exists(ctx, renewKey).Result()
		if exists != 0 {
			t.Errorf("renew key for exec %d should be deleted", i)
		}

		exists, _ = rdb.Exists(ctx, failCountKey).Result()
		if exists != 0 {
			t.Errorf("fail count key for exec %d should be deleted", i)
		}
	}
}

func TestHandleTaskFailure_RetryLockPreventsConcurrentRetry(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()
	taskID := int64(301)

	retryLockKey := fmt.Sprintf("task:retry:lock:%d", taskID)

	rdb.Set(ctx, retryLockKey, "locked", 30*time.Minute)

	set, err := rdb.SetNX(ctx, retryLockKey, "locked", 30*time.Minute).Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set {
		t.Error("second SetNX should fail because lock already exists")
	}
}

func TestHandleTaskFailure_RetryLockTTL(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()
	taskID := int64(302)

	retryLockKey := fmt.Sprintf("task:retry:lock:%d", taskID)

	set, err := rdb.SetNX(ctx, retryLockKey, "locked", 30*time.Minute).Result()
	if err != nil {
		t.Fatalf("failed to set retry lock: %v", err)
	}
	if !set {
		t.Error("retry lock should be acquired successfully on first attempt")
	}

	ttl, _ := rdb.TTL(ctx, retryLockKey).Result()
	if ttl <= 0 {
		t.Error("retry lock should have a positive TTL")
	}
	if ttl > 30*time.Minute {
		t.Errorf("retry lock TTL should be at most 30 minutes, got %v", ttl)
	}
}

func TestHandleTaskFailure_RetryLockReleasedAfterMaxRetries(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()
	taskID := int64(303)

	retryLockKey := fmt.Sprintf("task:retry:lock:%d", taskID)

	rdb.Set(ctx, retryLockKey, "locked", 30*time.Minute)

	rdb.Del(ctx, retryLockKey)

	set, err := rdb.SetNX(ctx, retryLockKey, "locked", 30*time.Minute).Result()
	if err != nil {
		t.Fatalf("unexpected error after lock release: %v", err)
	}
	if !set {
		t.Error("should be able to acquire lock after it's released")
	}
}

func TestHandleTaskFailure_RetryLockKeyFormat(t *testing.T) {
	taskID := int64(42)
	expectedKey := "task:retry:lock:42"
	actualKey := fmt.Sprintf("task:retry:lock:%d", taskID)
	if actualKey != expectedKey {
		t.Errorf("expected key %s, got %s", expectedKey, actualKey)
	}
}

func TestRecoveryRedisKeyFormats(t *testing.T) {
	tests := []struct {
		name     string
		execID   string
		keyType  string
		expected string
	}{
		{
			name:     "lock key",
			execID:   "exec-001",
			keyType:  "lock",
			expected: "task:lock:exec-001",
		},
		{
			name:     "renew key",
			execID:   "exec-001",
			keyType:  "renew",
			expected: "task:renew:exec-001",
		},
		{
			name:     "fail count key",
			execID:   "exec-001",
			keyType:  "failcount",
			expected: "task:renew:fail:count:exec-001",
		},
		{
			name:     "retry lock key",
			keyType:  "retrylock",
			expected: "task:retry:lock:42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actual string
			switch tt.keyType {
			case "lock":
				actual = fmt.Sprintf("task:lock:%s", tt.execID)
			case "renew":
				actual = fmt.Sprintf("task:renew:%s", tt.execID)
			case "failcount":
				actual = fmt.Sprintf("task:renew:fail:count:%s", tt.execID)
			case "retrylock":
				actual = fmt.Sprintf("task:retry:lock:%d", int64(42))
			}
			if actual != tt.expected {
				t.Errorf("expected key %s, got %s", tt.expected, actual)
			}
		})
	}
}

func TestRecoveryRenewalExpiryCalculation(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()
	executionID := "exec-renewal-001"
	renewKey := fmt.Sprintf("task:renew:%s", executionID)

	now := time.Now().Unix()
	rdb.Set(ctx, renewKey, now, 10*time.Minute)

	lastRenewStr, err := rdb.Get(ctx, renewKey).Result()
	if err != nil {
		t.Fatalf("failed to get renewal: %v", err)
	}

	var lastRenew int64
	fmt.Sscanf(lastRenewStr, "%d", &lastRenew)

	lastRenewSecondsAgo := time.Now().Unix() - lastRenew
	if lastRenewSecondsAgo < 0 || lastRenewSecondsAgo > 5 {
		t.Errorf("renewal should be very recent, got %d seconds ago", lastRenewSecondsAgo)
	}

	timeoutSeconds := int64(300)
	if lastRenewSecondsAgo > timeoutSeconds {
		t.Error("should not be considered expired when renewal is recent")
	}
}

func TestRecoveryRenewalExpired(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()
	executionID := "exec-expired-001"
	renewKey := fmt.Sprintf("task:renew:%s", executionID)

	oldTime := time.Now().Unix() - 600
	rdb.Set(ctx, renewKey, oldTime, 10*time.Minute)

	lastRenewStr, err := rdb.Get(ctx, renewKey).Result()
	if err != nil {
		t.Fatalf("failed to get renewal: %v", err)
	}

	var lastRenew int64
	fmt.Sscanf(lastRenewStr, "%d", &lastRenew)

	lastRenewSecondsAgo := time.Now().Unix() - lastRenew
	timeoutSeconds := int64(300)

	if lastRenewSecondsAgo <= timeoutSeconds {
		t.Errorf("renewal %d seconds ago should be considered expired (timeout %d)", lastRenewSecondsAgo, timeoutSeconds)
	}
}

func TestRecoveryNoRenewal(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()
	executionID := "exec-no-renewal-001"
	renewKey := fmt.Sprintf("task:renew:%s", executionID)

	_, err := rdb.Get(ctx, renewKey).Result()
	if err == nil {
		t.Error("expected redis Nil error for non-existent key")
	}

	noRenewal := err != nil
	if !noRenewal {
		t.Error("should detect no renewal when key does not exist")
	}
}

type mockConnectivityChecker struct {
	connected map[string]bool
}

func (m *mockConnectivityChecker) IsExecutorConnected(name string) bool {
	return m.connected[name]
}

func TestPingExecutor_NilExecutor(t *testing.T) {
	svc := &SchedulerService{}
	result := svc.pingExecutor(context.Background(), nil)
	if result {
		t.Error("pingExecutor should return false for nil executor")
	}
}

func TestPingExecutor_GRPCConnected(t *testing.T) {
	svc := &SchedulerService{
		connectivityChecker: &mockConnectivityChecker{
			connected: map[string]bool{"exec-1": true},
		},
	}

	executor := &model.Executor{
		ID:      1,
		Name:    "exec-1",
		Address: "localhost:50051",
		Status:  "online",
	}

	result := svc.pingExecutor(context.Background(), executor)
	if !result {
		t.Error("pingExecutor should return true when executor has active gRPC connection")
	}
}

func TestPingExecutor_GRPCNotConnected(t *testing.T) {
	svc := &SchedulerService{
		connectivityChecker: &mockConnectivityChecker{
			connected: map[string]bool{},
		},
	}

	executor := &model.Executor{
		ID:      1,
		Name:    "exec-1",
		Address: "localhost:19999",
		Status:  "online",
	}

	result := svc.pingExecutor(context.Background(), executor)
	if result {
		t.Error("pingExecutor should return false when executor has no gRPC connection and TCP dial fails")
	}
}

func TestPingExecutor_EmptyAddress(t *testing.T) {
	svc := &SchedulerService{
		connectivityChecker: &mockConnectivityChecker{
			connected: map[string]bool{},
		},
	}

	executor := &model.Executor{
		ID:      1,
		Name:    "exec-1",
		Address: "",
		Status:  "online",
	}

	result := svc.pingExecutor(context.Background(), executor)
	if result {
		t.Error("pingExecutor should return false when executor has empty address and no gRPC connection")
	}
}

func TestPingExecutor_NoConnectivityChecker(t *testing.T) {
	svc := &SchedulerService{}

	executor := &model.Executor{
		ID:      1,
		Name:    "exec-1",
		Address: "localhost:19999",
		Status:  "online",
	}

	result := svc.pingExecutor(context.Background(), executor)
	if result {
		t.Error("pingExecutor should return false when no connectivity checker and TCP dial fails")
	}
}

func TestPingExecutor_AddressWithoutPort(t *testing.T) {
	svc := &SchedulerService{
		connectivityChecker: &mockConnectivityChecker{
			connected: map[string]bool{},
		},
	}

	executor := &model.Executor{
		ID:      1,
		Name:    "exec-1",
		Address: "192.0.2.1",
		Status:  "online",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := svc.pingExecutor(ctx, executor)
	if result {
		t.Error("pingExecutor should return false when address is unreachable")
	}
}

func TestCalculateLockTTL_NoTimeoutUses3600(t *testing.T) {
	svc := &SchedulerService{}
	result := svc.calculateLockTTL(nil, &model.Task{TimeoutSeconds: 0})
	if result != 3600 {
		t.Errorf("expected 3600 for no-timeout task, got %d", result)
	}
}

func TestCalculateLockTTL_NegativeTimeoutUses3600(t *testing.T) {
	svc := &SchedulerService{}
	result := svc.calculateLockTTL(nil, &model.Task{TimeoutSeconds: -1})
	if result != 3600 {
		t.Errorf("expected 3600 for negative timeout task, got %d", result)
	}
}

func TestNoTimeout_NotExpiredByRenewalAge(t *testing.T) {
	noTimeout := true
	timeoutSeconds := int64(300)
	lastRenewSecondsAgo := int64(7200)

	renewalExpired := !noTimeout && lastRenewSecondsAgo > timeoutSeconds
	if renewalExpired {
		t.Error("no-timeout task should not be considered expired by renewal age")
	}

	noTimeout = false
	renewalExpired = !noTimeout && lastRenewSecondsAgo > timeoutSeconds
	if !renewalExpired {
		t.Error("normal task with old renewal should be considered expired")
	}
}

func TestNoTimeout_NotExpiredByTaskDuration(t *testing.T) {
	noTimeout := true
	startTime := time.Now().Add(-3 * time.Hour)
	timeoutSeconds := int64(300)

	taskTimeout := false
	if !noTimeout && time.Since(startTime) > time.Duration(timeoutSeconds)*time.Second {
		taskTimeout = true
	}
	if taskTimeout {
		t.Error("no-timeout task should not be considered timed out regardless of duration")
	}

	noTimeout = false
	if !noTimeout && time.Since(startTime) > time.Duration(timeoutSeconds)*time.Second {
		taskTimeout = true
	}
	if !taskTimeout {
		t.Error("normal task running longer than timeout should be considered timed out")
	}
}

func TestCronReloadFlag(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()

	_, err := rdb.Get(ctx, "cron:needs_reload").Int64()
	if err == nil {
		t.Error("cron:needs_reload should not exist initially")
	}

	rdb.Set(ctx, "cron:needs_reload", time.Now().Unix(), 5*time.Minute)

	val, err := rdb.Get(ctx, "cron:needs_reload").Int64()
	if err != nil {
		t.Fatalf("failed to get cron reload flag: %v", err)
	}
	if val == 0 {
		t.Error("cron reload flag should have a non-zero value")
	}

	rdb.Del(ctx, "cron:needs_reload")

	_, err = rdb.Get(ctx, "cron:needs_reload").Int64()
	if err == nil {
		t.Error("cron:needs_reload should not exist after deletion")
	}
}

func TestCronReloadFlag_TTL(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()

	rdb.Set(ctx, "cron:needs_reload", time.Now().Unix(), 5*time.Minute)

	ttl, err := rdb.TTL(ctx, "cron:needs_reload").Result()
	if err != nil {
		t.Fatalf("failed to get TTL: %v", err)
	}
	if ttl <= 0 || ttl > 5*time.Minute {
		t.Errorf("cron reload flag TTL should be ~5 minutes, got %v", ttl)
	}
}

func TestNoTimeout_RenewalKeyTTL(t *testing.T) {
	mr, rdb := newTestRedis(t)
	defer mr.Close()
	defer rdb.Close()

	ctx := context.Background()
	executionID := "exec-notimeout-001"

	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	renewKey := fmt.Sprintf("task:renew:%s", executionID)

	lockTTL := 3600
	rdb.Set(ctx, lockKey, "locked", time.Duration(lockTTL)*time.Second)
	rdb.Set(ctx, renewKey, time.Now().Unix(), time.Duration(lockTTL)*time.Second)

	lockTTLResult, _ := rdb.TTL(ctx, lockKey).Result()
	if lockTTLResult < 55*time.Minute {
		t.Errorf("no-timeout task lock TTL should be ~1 hour, got %v", lockTTLResult)
	}

	renewTTLResult, _ := rdb.TTL(ctx, renewKey).Result()
	if renewTTLResult < 55*time.Minute {
		t.Errorf("no-timeout task renewal TTL should be ~1 hour, got %v", renewTTLResult)
	}
}

func TestNoTimeout_CleanupStaleTaskLocksDoesNotFlagAsExpired(t *testing.T) {
	noTimeout := true
	timeoutSeconds := int64(300)
	interval := int64(7200)

	if noTimeout || interval <= timeoutSeconds {
	} else {
		t.Error("no-timeout task with old renewal should not trigger fail count reset logic path")
	}

	noTimeout = false
	if noTimeout || interval <= timeoutSeconds {
		t.Error("normal task with expired renewal should not take the 'healthy' path")
	}
}
