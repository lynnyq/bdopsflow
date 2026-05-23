package cron

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestNewCronScheduler(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	cs := NewCronScheduler(nil, client)

	if cs == nil {
		t.Error("NewCronScheduler should return a non-nil scheduler")
	}

	if cs.cron == nil {
		t.Error("Cron scheduler should not be nil")
	}

	if cs.taskEntries == nil {
		t.Error("taskEntries should be initialized")
	}

	if len(cs.taskEntries) != 0 {
		t.Error("taskEntries should be empty initially")
	}
}

func TestCronScheduler_PauseResume(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	defer func() {
		ctx := context.Background()
		client.FlushDB(ctx)
		client.Close()
	}()

	cs := NewCronScheduler(nil, client)

	if cs.IsPaused() {
		t.Error("Scheduler should not be paused initially")
	}

	cs.Pause()

	if !cs.IsPaused() {
		t.Error("Scheduler should be paused after calling Pause()")
	}

	cs.Resume()

	if cs.IsPaused() {
		t.Error("Scheduler should not be paused after calling Resume()")
	}
}

func TestCronScheduler_GetUptime(t *testing.T) {
	cs := NewCronScheduler(nil, nil)

	time.Sleep(100 * time.Millisecond)

	uptime := cs.GetUptime()

	if uptime < 100*time.Millisecond {
		t.Errorf("Expected uptime >= 100ms, got %v", uptime)
	}
}

func TestCronScheduler_RegisterUnregisterTask(t *testing.T) {
	cs := NewCronScheduler(nil, nil)

	err := cs.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer cs.Stop()

	taskID := int64(1)
	cronExpr := "0 * * * * *"

	cs.RegisterTask(taskID, cronExpr)

	cs.mu.RLock()
	_, exists := cs.taskEntries[taskID]
	cs.mu.RUnlock()

	if !exists {
		t.Error("Task should be registered")
	}

	cs.UnregisterTask(taskID)

	cs.mu.RLock()
	_, exists = cs.taskEntries[taskID]
	cs.mu.RUnlock()

	if exists {
		t.Error("Task should be unregistered")
	}
}

func TestCronScheduler_RegisterTask_InvalidCron(t *testing.T) {
	cs := NewCronScheduler(nil, nil)

	err := cs.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer cs.Stop()

	taskID := int64(2)
	invalidCron := "invalid cron expression"

	cs.RegisterTask(taskID, invalidCron)

	cs.mu.RLock()
	_, exists := cs.taskEntries[taskID]
	cs.mu.RUnlock()

	if exists {
		t.Error("Invalid cron expression should not register task")
	}
}

func TestCronScheduler_RegisterTask_5Field(t *testing.T) {
	cs := NewCronScheduler(nil, nil)

	err := cs.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer cs.Stop()

	taskID := int64(3)
	
	standard5Field := "0 * * * *"

	cs.RegisterTask(taskID, standard5Field)

	cs.mu.RLock()
	_, exists := cs.taskEntries[taskID]
	cs.mu.RUnlock()

	if !exists {
		t.Error("5-field cron expression should be registered")
	}
}

func TestCronScheduler_RegisterTask_Duplicate(t *testing.T) {
	cs := NewCronScheduler(nil, nil)

	err := cs.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer cs.Stop()

	taskID := int64(4)
	cronExpr1 := "0 * * * * *"
	cronExpr2 := "30 * * * * *"

	cs.RegisterTask(taskID, cronExpr1)
	
	cs.RegisterTask(taskID, cronExpr2)

	cs.mu.RLock()
	entryID, exists := cs.taskEntries[taskID]
	cs.mu.RUnlock()

	if !exists {
		t.Error("Task should still be registered after re-registration")
	}

	if entryID == 0 {
		t.Error("EntryID should not be zero")
	}
}

func TestCronScheduler_UnregisterNonExistent(t *testing.T) {
	cs := NewCronScheduler(nil, nil)

	cs.UnregisterTask(int64(999))

	t.Log("Unregistering non-existent task should not panic")
}

func TestAcquireReleaseLock_NilRedis(t *testing.T) {
	cs := NewCronScheduler(nil, nil)

	ctx := context.Background()
	taskID := int64(1)
	lockTTL := 30 * time.Second

	acquired, err := cs.acquireTaskLock(ctx, taskID, lockTTL)
	if err != nil {
		t.Errorf("acquireTaskLock with nil redis should not return error: %v", err)
	}
	if !acquired {
		t.Error("acquireTaskLock with nil redis should return true")
	}

	cs.releaseTaskLock(ctx, taskID)
}

func TestAcquireReleaseLock_WithRedis(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	defer func() {
		ctx := context.Background()
		client.FlushDB(ctx)
		client.Close()
	}()

	cs := NewCronScheduler(nil, client)

	ctx := context.Background()
	taskID := int64(1)
	lockTTL := 30 * time.Second

	acquired, err := cs.acquireTaskLock(ctx, taskID, lockTTL)
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}
	if !acquired {
		t.Error("Should acquire lock on first attempt")
	}

	acquired, err = cs.acquireTaskLock(ctx, taskID, lockTTL)
	if err != nil {
		t.Errorf("Second acquire should not return error: %v", err)
	}
	if acquired {
		t.Error("Should not acquire lock on second attempt")
	}

	cs.releaseTaskLock(ctx, taskID)

	acquired, err = cs.acquireTaskLock(ctx, taskID, lockTTL)
	if err != nil {
		t.Errorf("Should be able to acquire after release: %v", err)
	}
	if !acquired {
		t.Error("Should acquire lock after release")
	}
}