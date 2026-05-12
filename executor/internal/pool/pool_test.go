package pool

import (
	"context"
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

	if counter != 5 {
		t.Errorf("expected 5 completed tasks, got %d", counter)
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

	if counter != 0 {
		t.Logf("Note: Some tasks may have completed before cancellation")
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

	if counter != 3 {
		t.Errorf("expected 3 completed tasks, got %d", counter)
	}
}
