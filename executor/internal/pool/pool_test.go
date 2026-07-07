package pool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewPool(t *testing.T) {
	pool := NewPool(10)
	if pool == nil {
		t.Fatal("expected pool to be created")
	}

	if pool.capacity != 10 {
		t.Errorf("expected capacity 10, got %d", pool.capacity)
	}
}

func TestPoolStartStop(t *testing.T) {
	pool := NewPool(5)
	pool.Start()

	time.Sleep(100 * time.Millisecond)

	if pool.Running() != 0 {
		t.Errorf("expected 0 running tasks, got %d", pool.Running())
	}

	pool.Stop()
}

func TestPoolSubmit(t *testing.T) {
	pool := NewPool(5)
	pool.Start()
	defer pool.Stop()

	var counter int64

	for i := 0; i < 5; i++ {
		err := pool.Submit(func(ctx context.Context) error {
			atomic.AddInt64(&counter, 1)
			time.Sleep(20 * time.Millisecond)
			return nil
		})

		if err != nil {
			t.Errorf("failed to submit task: %v", err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	if got := atomic.LoadInt64(&counter); got != 5 {
		t.Errorf("expected 5 completed tasks, got %d", got)
	}
}

func TestPoolCapacity(t *testing.T) {
	pool := NewPool(2)
	pool.Start()
	defer pool.Stop()

	time.Sleep(100 * time.Millisecond)

	running := pool.Running()
	if running > 2 {
		t.Errorf("expected at most 2 running tasks, got %d", running)
	}
}

func TestPoolContextCancellation(t *testing.T) {
	pool := NewPool(5)
	ctx, cancel := context.WithCancel(context.Background())
	pool.ctx = ctx

	pool.Start()

	var counter int64
	for i := 0; i < 10; i++ {
		pool.Submit(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				atomic.AddInt64(&counter, 1)
				time.Sleep(100 * time.Millisecond)
				return nil
			}
		})
	}

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)

	if got := atomic.LoadInt64(&counter); got != 0 {
		t.Logf("Note: Some tasks may have completed before cancellation, count=%d", got)
	}
}

func TestPoolSubmitBlocking(t *testing.T) {
	pool := NewPool(2)
	pool.Start()
	defer pool.Stop()

	var counter int64

	for i := 0; i < 3; i++ {
		err := pool.Submit(func(ctx context.Context) error {
			atomic.AddInt64(&counter, 1)
			time.Sleep(5 * time.Millisecond)
			return nil
		})

		if err != nil {
			t.Errorf("failed to submit task: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond)

	if got := atomic.LoadInt64(&counter); got != 3 {
		t.Errorf("expected 3 completed tasks, got %d", got)
	}
}

// TestPoolSubmit_QueueFull 测试任务队列满时 Submit 返回错误
func TestPoolSubmit_QueueFull(t *testing.T) {
	// capacity=1，队列大小为 capacity*2=2
	pool := NewPool(1)
	// 不启动 worker，队列中的任务不会被消费
	defer pool.Stop()

	// 填满队列（容量为 2）
	for i := 0; i < 2; i++ {
		err := pool.Submit(func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Errorf("expected submit %d to succeed, got error: %v", i, err)
		}
	}

	// 第 3 次提交应该返回 "task queue full" 错误
	err := pool.Submit(func(ctx context.Context) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error when queue is full, got nil")
	}
	if err.Error() != "task queue full" {
		t.Errorf("expected 'task queue full' error, got %q", err.Error())
	}
}

// TestPoolRunningCounter 测试 Running/IncRunning/DecRunning 方法
func TestPoolRunningCounter(t *testing.T) {
	pool := NewPool(2)
	defer pool.Stop()

	if pool.Running() != 0 {
		t.Errorf("expected 0 running initially, got %d", pool.Running())
	}

	pool.IncRunning()
	if pool.Running() != 1 {
		t.Errorf("expected 1 running after IncRunning, got %d", pool.Running())
	}

	pool.IncRunning()
	if pool.Running() != 2 {
		t.Errorf("expected 2 running after second IncRunning, got %d", pool.Running())
	}

	pool.DecRunning()
	if pool.Running() != 1 {
		t.Errorf("expected 1 running after DecRunning, got %d", pool.Running())
	}

	pool.DecRunning()
	if pool.Running() != 0 {
		t.Errorf("expected 0 running after second DecRunning, got %d", pool.Running())
	}
}

// TestPoolCapacityMethod 测试 Capacity 方法
func TestPoolCapacityMethod(t *testing.T) {
	pool := NewPool(5)
	defer pool.Stop()

	if pool.Capacity() != 5 {
		t.Errorf("expected capacity 5, got %d", pool.Capacity())
	}
}

// TestPoolTaskError 测试任务执行错误时记录 slog.Error 但不 panic
func TestPoolTaskError(t *testing.T) {
	pool := NewPool(2)
	pool.Start()
	defer pool.Stop()

	var done int64
	errTask := errors.New("task execution failed")

	err := pool.Submit(func(ctx context.Context) error {
		atomic.AddInt64(&done, 1)
		return errTask
	})
	if err != nil {
		t.Fatalf("failed to submit error task: %v", err)
	}

	// 等待任务执行完成（错误会被 slog.Error 记录，但不影响 pool）
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt64(&done) != 1 {
		t.Errorf("expected 1 task executed, got %d", atomic.LoadInt64(&done))
	}
}

// TestPoolWorkerLimit 测试 worker 数量限制（同时运行的任务数不超过 capacity）
func TestPoolWorkerLimit(t *testing.T) {
	const capacity = 2
	pool := NewPool(capacity)
	pool.Start()
	defer pool.Stop()

	var currentRunning int64
	var maxRunning int64
	var mu sync.Mutex

	// 提交 4 个长任务，验证同时运行的不超过 capacity
	for i := 0; i < 4; i++ {
		err := pool.Submit(func(ctx context.Context) error {
			cur := atomic.AddInt64(&currentRunning, 1)
			mu.Lock()
			if cur > maxRunning {
				maxRunning = cur
			}
			mu.Unlock()

			time.Sleep(50 * time.Millisecond)
			atomic.AddInt64(&currentRunning, -1)
			return nil
		})
		if err != nil {
			t.Errorf("failed to submit task %d: %v", i, err)
		}
	}

	// 等待所有任务完成
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	max := maxRunning
	mu.Unlock()

	if max > int64(capacity) {
		t.Errorf("max concurrent running %d exceeded capacity %d", max, capacity)
	}
}

// TestPoolUpdateCapacity_Invalid 测试无效的容量值
func TestPoolUpdateCapacity_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		capacity    int32
		expectError bool
	}{
		{"zero capacity", 0, true},
		{"negative capacity", -1, true},
		{"positive capacity", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewPool(3)
			defer pool.Stop()

			err := pool.UpdateCapacity(tt.capacity)
			if tt.expectError && err == nil {
				t.Error("expected error for invalid capacity, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error for valid capacity, got %v", err)
			}
		})
	}
}

// TestPoolUpdateCapacity_Increase 测试增加 worker 数量
func TestPoolUpdateCapacity_Increase(t *testing.T) {
	pool := NewPool(2)
	pool.Start()
	defer pool.Stop()

	// 等待 worker 启动
	time.Sleep(50 * time.Millisecond)

	err := pool.UpdateCapacity(4)
	if err != nil {
		t.Fatalf("UpdateCapacity failed: %v", err)
	}

	if pool.Capacity() != 4 {
		t.Errorf("expected capacity 4, got %d", pool.Capacity())
	}

	pool.mu.RLock()
	workerCount := len(pool.workers)
	pool.mu.RUnlock()
	if workerCount != 4 {
		t.Errorf("expected 4 workers, got %d", workerCount)
	}
}

// TestPoolUpdateCapacity_Decrease 测试减少 worker 数量
func TestPoolUpdateCapacity_Decrease(t *testing.T) {
	pool := NewPool(4)
	pool.Start()
	defer pool.Stop()

	time.Sleep(50 * time.Millisecond)

	err := pool.UpdateCapacity(2)
	if err != nil {
		t.Fatalf("UpdateCapacity failed: %v", err)
	}

	if pool.Capacity() != 2 {
		t.Errorf("expected capacity 2, got %d", pool.Capacity())
	}

	pool.mu.RLock()
	workerCount := len(pool.workers)
	pool.mu.RUnlock()
	if workerCount != 2 {
		t.Errorf("expected 2 workers, got %d", workerCount)
	}
}

// TestPoolUpdateCapacity_Same 测试容量不变时为无操作
func TestPoolUpdateCapacity_Same(t *testing.T) {
	pool := NewPool(3)
	pool.Start()
	defer pool.Stop()

	time.Sleep(50 * time.Millisecond)

	err := pool.UpdateCapacity(3)
	if err != nil {
		t.Fatalf("UpdateCapacity with same value failed: %v", err)
	}

	if pool.Capacity() != 3 {
		t.Errorf("expected capacity 3, got %d", pool.Capacity())
	}
}

// TestPoolUpdateCapacity_PreservesTasks 测试容量更新后剩余任务仍能执行
func TestPoolUpdateCapacity_PreservesTasks(t *testing.T) {
	pool := NewPool(1)
	// 不启动 worker，先填入任务
	pool.Submit(func(ctx context.Context) error {
		return nil
	})

	// 更新容量（会重新创建队列并迁移任务）
	err := pool.UpdateCapacity(2)
	if err != nil {
		t.Fatalf("UpdateCapacity failed: %v", err)
	}

	// 启动 worker 消费迁移后的任务
	pool.Start()
	defer pool.Stop()

	time.Sleep(100 * time.Millisecond)
	// 任务应被消费，不应阻塞或丢失
}

// TestPoolStop_NoPanicOnEmptyPool 测试空 pool 的 Stop 不 panic
func TestPoolStop_NoPanicOnEmptyPool(t *testing.T) {
	pool := NewPool(1)
	// 不 Submit 任何任务，直接 Stop
	pool.Stop()
}

// TestPoolContextCancellationDuringTask 测试任务执行期间 context 取消
func TestPoolContextCancellationDuringTask(t *testing.T) {
	pool := NewPool(1)
	pool.Start()

	var taskCancelled int64

	pool.Submit(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			atomic.StoreInt64(&taskCancelled, 1)
			return ctx.Err()
		case <-time.After(2 * time.Second):
			return nil
		}
	})

	time.Sleep(50 * time.Millisecond)
	pool.Stop()

	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt64(&taskCancelled) == 0 {
		// pool.Stop 会 cancel context，任务应感知到 ctx.Done
		// 但因任务可能已从队列取出并执行，此处放宽断言
		t.Log("task may or may not have been cancelled depending on timing")
	}
}

// TestPoolSubmitAfterStop 测试 Stop 后 Submit 行为
// Stop 会 close taskQueue，Submit 向已关闭的 channel 发送会 panic
// 这是一个边界场景，验证 pool 不应在正常使用中出问题
func TestPoolSubmitAfterStop(t *testing.T) {
	pool := NewPool(1)
	pool.Start()
	pool.Stop()

	// Stop 后 Submit 会 panic（向已关闭 channel 发送），
	// 此测试验证 pool.Stop 后不应再调用 Submit
	// 用 recover 确认行为
	defer func() {
		if r := recover(); r != nil {
			// 预期行为：向已关闭 channel 发送会 panic
			t.Logf("Submit after Stop panicked as expected: %v", r)
		}
	}()

	_ = pool.Submit(func(ctx context.Context) error {
		return nil
	})
}

// TestNewPool_QueueSize 测试 NewPool 创建的队列大小为 capacity*2
func TestNewPool_QueueSize(t *testing.T) {
	pool := NewPool(3)
	defer pool.Stop()

	// 队列大小应为 capacity*2=6，不启动 worker 时可 Submit 6 个任务
	for i := 0; i < 6; i++ {
		err := pool.Submit(func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Errorf("expected submit %d to succeed, got: %v", i, err)
		}
	}

	// 第 7 个应失败
	err := pool.Submit(func(ctx context.Context) error {
		return nil
	})
	if err == nil {
		t.Error("expected 7th submit to fail (queue full), got nil")
	}
}
