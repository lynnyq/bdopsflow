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
	capacity  int32
	running   int32
	taskQueue chan TaskFunc
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
	workers   []context.CancelFunc
}

func NewPool(capacity int32) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		capacity:  capacity,
		taskQueue: make(chan TaskFunc, capacity*2),
		ctx:       ctx,
		cancel:    cancel,
		workers:   make([]context.CancelFunc, 0, capacity),
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
			select {
			case task, ok := <-p.taskQueue:
				if !ok {
					return
				}
				atomic.AddInt32(&p.running, 1)
				_ = task(p.ctx)
				atomic.AddInt32(&p.running, -1)
			case <-workerCtx.Done():
				return
			}
		}
	}()
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}
			atomic.AddInt32(&p.running, 1)
			_ = task(p.ctx)
			atomic.AddInt32(&p.running, -1)
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *Pool) Submit(task TaskFunc) error {
	select {
	case p.taskQueue <- task:
		return nil
	default:
		return fmt.Errorf("task queue full")
	}
}

func (p *Pool) Running() int32 {
	return atomic.LoadInt32(&p.running)
}

func (p *Pool) IncRunning() {
	atomic.AddInt32(&p.running, 1)
}

func (p *Pool) DecRunning() {
	atomic.AddInt32(&p.running, -1)
}

func (p *Pool) Stop() {
	p.cancel()
	close(p.taskQueue)
	p.wg.Wait()
}

func (p *Pool) UpdateCapacity(newCapacity int32) error {
	if newCapacity <= 0 {
		return fmt.Errorf("capacity must be positive")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	oldCapacity := atomic.LoadInt32(&p.capacity)
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

	atomic.StoreInt32(&p.capacity, newCapacity)

	// 更新队列大小，但需要重新创建队列
	if newCapacity != oldCapacity {
		oldQueue := p.taskQueue
		p.taskQueue = make(chan TaskFunc, newCapacity*2)

		// 移动已有的任务到新队列
		close(oldQueue)
		for task := range oldQueue {
			select {
			case p.taskQueue <- task:
			default:
				slog.Warn("task dropped during capacity update")
			}
		}
	}

	return nil
}

func (p *Pool) Capacity() int32 {
	return atomic.LoadInt32(&p.capacity)
}
