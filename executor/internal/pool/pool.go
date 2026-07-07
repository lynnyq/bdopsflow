package pool

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

type TaskFunc func(ctx context.Context) error

type Pool struct {
	capacity int32
	running  int64
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.RWMutex
	workers  []context.CancelFunc
	taskCh   chan TaskFunc
	wg       sync.WaitGroup
}

func NewPool(capacity int32) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		capacity: capacity,
		taskCh:   make(chan TaskFunc, capacity*2),
		ctx:      ctx,
		cancel:   cancel,
		workers:  make([]context.CancelFunc, 0, capacity),
	}
}

func (p *Pool) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := int32(0); i < p.capacity; i++ {
		p.startWorker()
	}
}

func (p *Pool) startWorker() {
	workerCtx, workerCancel := context.WithCancel(p.ctx)
	p.workers = append(p.workers, workerCancel)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			// 在读 channel 前重新读取 taskCh 引用，保证后续 UpdateCapacity 替换队列后
			// 新的任务会被发送到新队列。
			ch := p.getTaskCh()
			select {
			case task, ok := <-ch:
				if !ok {
					return
				}
				atomic.AddInt64(&p.running, 1)
				if err := task(p.ctx); err != nil {
					slog.Error("pool task execution failed", "error", err)
				}
				atomic.AddInt64(&p.running, -1)
			case <-workerCtx.Done():
				return
			}
		}
	}()
}

// getTaskCh 安全地获取当前任务 channel 引用。
func (p *Pool) getTaskCh() chan TaskFunc {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.taskCh
}

func (p *Pool) Submit(task TaskFunc) error {
	ch := p.getTaskCh()
	select {
	case ch <- task:
		return nil
	default:
		return fmt.Errorf("task queue full")
	}
}

func (p *Pool) Running() int32 {
	return int32(atomic.LoadInt64(&p.running))
}

func (p *Pool) IncRunning() {
	atomic.AddInt64(&p.running, 1)
}

func (p *Pool) DecRunning() {
	atomic.AddInt64(&p.running, -1)
}

func (p *Pool) Stop() {
	p.cancel()
	p.mu.Lock()
	close(p.taskCh)
	p.mu.Unlock()
	p.wg.Wait()
}

func (p *Pool) UpdateCapacity(newCapacity int32) error {
	if newCapacity <= 0 {
		return fmt.Errorf("capacity must be positive")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	oldCapacity := p.capacity
	if newCapacity == oldCapacity {
		return nil
	}

	// 如果新容量大于旧容量，增加 worker
	if newCapacity > oldCapacity {
		slog.Info("increasing pool capacity",
			"old_capacity", oldCapacity, "new_capacity", newCapacity)
		for i := oldCapacity; i < newCapacity; i++ {
			p.startWorker()
		}
	} else {
		// 如果新容量小于旧容量，减少 worker
		slog.Info("decreasing pool capacity",
			"old_capacity", oldCapacity, "new_capacity", newCapacity)
		workersToStop := oldCapacity - newCapacity
		for i := int32(0); i < workersToStop && i < int32(len(p.workers)); i++ {
			if len(p.workers) > 0 {
				cancel := p.workers[len(p.workers)-1]
				p.workers = p.workers[:len(p.workers)-1]
				cancel()
			}
		}
	}

	p.capacity = newCapacity

	// 更新队列大小，重新创建队列
	oldQueue := p.taskCh
	p.taskCh = make(chan TaskFunc, newCapacity*2)

	// 移动已有的任务到新队列
	close(oldQueue)
	for task := range oldQueue {
		select {
		case p.taskCh <- task:
		default:
			slog.Warn("task dropped during capacity update")
		}
	}

	return nil
}

func (p *Pool) Capacity() int32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.capacity
}
