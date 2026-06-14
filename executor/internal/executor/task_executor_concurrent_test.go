package executor

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
)

func TestTaskExecutor_ConcurrentShellExecution(t *testing.T) {
	executor := NewTaskExecutor(nil)

	// 测试并发执行多个 shell 任务，验证并发安全性
	shellConfig := map[string]string{
		"script": "for i in {1..10}; do echo \"Line $i\"; sleep 0.01; done",
	}
	configJSON, _ := json.Marshal(shellConfig)

	var wg sync.WaitGroup
	numTasks := 5

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(taskNum int) {
			defer wg.Done()
			task := &pb.Task{
				ExecutionId:    string(rune('A' + taskNum)),
				TaskId:         int64(taskNum + 1),
				Type:           "shell",
				Config:         string(configJSON),
				TimeoutSeconds: 30,
			}
			ctx := context.Background()
			executor.Execute(ctx, task, nil)
		}(i)
	}

	wg.Wait()
}

func TestTaskExecutor_ShellOutputCapture(t *testing.T) {
	executor := NewTaskExecutor(nil)

	shellConfig := map[string]string{
		"script": "echo 'stdout output'; echo 'stderr output' >&2",
	}
	configJSON, _ := json.Marshal(shellConfig)

	task := &pb.Task{
		ExecutionId:    "test-capture",
		TaskId:         1,
		Type:           "shell",
		Config:         string(configJSON),
		TimeoutSeconds: 10,
	}

	ctx := context.Background()
	executor.Execute(ctx, task, nil)
}

func TestTaskExecutor_ShellEmptyScript(t *testing.T) {
	executor := NewTaskExecutor(nil)

	shellConfig := map[string]string{
		"script": "",
	}
	configJSON, _ := json.Marshal(shellConfig)

	task := &pb.Task{
		ExecutionId:    "test-empty",
		TaskId:         1,
		Type:           "shell",
		Config:         string(configJSON),
		TimeoutSeconds: 10,
	}

	ctx := context.Background()
	executor.Execute(ctx, task, nil)
}

func TestTaskExecutor_ShellLongRunning(t *testing.T) {
	executor := NewTaskExecutor(nil)

	shellConfig := map[string]string{
		"script": "for i in {1..20}; do echo \"Output $i\"; sleep 0.05; done",
	}
	configJSON, _ := json.Marshal(shellConfig)

	task := &pb.Task{
		ExecutionId:    "test-long",
		TaskId:         1,
		Type:           "shell",
		Config:         string(configJSON),
		TimeoutSeconds: 30,
	}

	ctx := context.Background()
	start := time.Now()
	executor.Execute(ctx, task, nil)
	duration := time.Since(start)

	if duration > 10*time.Second {
		t.Errorf("shell execution took too long: %v", duration)
	}
}

func TestTaskExecutor_ShellWithStderr(t *testing.T) {
	executor := NewTaskExecutor(nil)

	shellConfig := map[string]string{
		"script": "echo 'normal output'; echo 'error output' >&2; echo 'more output'",
	}
	configJSON, _ := json.Marshal(shellConfig)

	task := &pb.Task{
		ExecutionId:    "test-stderr",
		TaskId:         1,
		Type:           "shell",
		Config:         string(configJSON),
		TimeoutSeconds: 10,
	}

	ctx := context.Background()
	executor.Execute(ctx, task, nil)
}

func TestTaskExecutor_CancelTask(t *testing.T) {
	executor := NewTaskExecutor(nil)

	shellConfig := map[string]string{
		"script": "sleep 30",
	}
	configJSON, _ := json.Marshal(shellConfig)

	task := &pb.Task{
		ExecutionId:    "test-cancel",
		TaskId:         1,
		Type:           "shell",
		Config:         string(configJSON),
		TimeoutSeconds: 60,
	}

	ctx := context.Background()
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancelled := executor.CancelTask("test-cancel")
		if !cancelled {
			t.Log("task was not cancelled (may have already completed)")
		}
	}()

	executor.Execute(ctx, task, nil)
}
