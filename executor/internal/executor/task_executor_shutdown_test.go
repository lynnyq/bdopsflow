package executor

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
)

func TestTaskExecutor_Shutdown_RejectsNewTasks(t *testing.T) {
	e := NewTaskExecutor(nil)

	// 先启动一个任务
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx := context.Background()
		config, _ := json.Marshal(map[string]string{"script": "sleep 0.5"})
		task := &pb.Task{
			ExecutionId:    "test-1",
			TaskId:         1,
			Type:           "shell",
			Config:         string(config),
			TimeoutSeconds: 5,
		}
		e.Execute(ctx, task, nil)
	}()

	// 等待任务开始
	time.Sleep(100 * time.Millisecond)

	// 触发关闭
	go e.Shutdown(2 * time.Second)

	// 等待关闭完成
	time.Sleep(200 * time.Millisecond)

	// 新任务应该被拒绝
	e.shutdownMu.Lock()
	if !e.shuttingDown {
		t.Error("expected shuttingDown to be true")
	}
	e.shutdownMu.Unlock()

	wg.Wait()
}

func TestTaskExecutor_Shutdown_CancelsRunningTasks(t *testing.T) {
	e := NewTaskExecutor(nil)

	// 启动一个长时间运行的任务
	done := make(chan struct{})
	go func() {
		ctx := context.Background()
		config, _ := json.Marshal(map[string]string{"script": "sleep 10"})
		task := &pb.Task{
			ExecutionId:    "test-long",
			TaskId:         1,
			Type:           "shell",
			Config:         string(config),
			TimeoutSeconds: 30,
		}
		e.Execute(ctx, task, nil)
		close(done)
	}()

	// 等待任务开始
	time.Sleep(200 * time.Millisecond)

	// 触发关闭，应该取消正在运行的任务
	e.Shutdown(2 * time.Second)

	// 任务应该已经完成（被取消）
	select {
	case <-done:
		// 正常
	case <-time.After(3 * time.Second):
		t.Error("task should have been cancelled by shutdown")
	}
}

func TestTaskExecutor_Shutdown_Idempotent(t *testing.T) {
	e := NewTaskExecutor(nil)

	// 多次调用Shutdown不应该panic
	e.Shutdown(1 * time.Second)
	e.Shutdown(1 * time.Second)
	e.Shutdown(1 * time.Second)
}

func TestTaskExecutor_Shutdown_Timeout(t *testing.T) {
	e := NewTaskExecutor(nil)

	// 启动一个长时间运行的任务
	go func() {
		ctx := context.Background()
		config, _ := json.Marshal(map[string]string{"script": "sleep 30"})
		task := &pb.Task{
			ExecutionId:    "test-timeout",
			TaskId:         1,
			Type:           "shell",
			Config:         string(config),
			TimeoutSeconds: 60,
		}
		e.Execute(ctx, task, nil)
	}()

	// 等待任务开始
	time.Sleep(200 * time.Millisecond)

	// 使用短超时关闭
	start := time.Now()
	e.Shutdown(500 * time.Millisecond)
	elapsed := time.Since(start)

	// 应该在超时后返回，不应该等待太久
	if elapsed > 2*time.Second {
		t.Errorf("shutdown took too long: %v", elapsed)
	}
}
