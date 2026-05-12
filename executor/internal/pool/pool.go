package pool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

type TaskFunc func(ctx context.Context) error

type Pool struct {
	capacity    int32
	running     int32
	taskQueue   chan TaskFunc
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewPool(capacity int32) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		capacity:  capacity,
		taskQueue: make(chan TaskFunc, capacity*2),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (p *Pool) Start() {
	for i := int32(0); i < p.capacity; i++ {
		p.wg.Add(1)
		go p.worker()
	}
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

func (p *Pool) Stop() {
	p.cancel()
	close(p.taskQueue)
	p.wg.Wait()
}
