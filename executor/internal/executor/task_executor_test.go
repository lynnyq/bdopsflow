package executor

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
)

func TestTaskExecutor_ExecuteHTTP(t *testing.T) {
	executor := NewTaskExecutor(nil)

	httpConfig := map[string]string{
		"url":    "https://httpbin.org/get",
		"method": "GET",
	}
	configJSON, _ := json.Marshal(httpConfig)

	task := &pb.Task{
		ExecutionId:    "exec-1",
		TaskId:         1,
		Type:           "http",
		Config:         string(configJSON),
		TimeoutSeconds: 30,
	}

	ctx := context.Background()
	executor.Execute(ctx, task, nil)
}

func TestTaskExecutor_ExecuteHTTPParseError(t *testing.T) {
	executor := NewTaskExecutor(nil)

	task := &pb.Task{
		ExecutionId:    "exec-1",
		TaskId:         1,
		Type:           "http",
		Config:         "invalid json",
		TimeoutSeconds: 30,
	}

	ctx := context.Background()
	executor.Execute(ctx, task, nil)
}

func TestTaskExecutor_ExecuteShell(t *testing.T) {
	executor := NewTaskExecutor(nil)

	shellConfig := map[string]string{
		"script": "echo 'Hello World'",
	}
	configJSON, _ := json.Marshal(shellConfig)

	task := &pb.Task{
		ExecutionId:    "exec-1",
		TaskId:         1,
		Type:           "shell",
		Config:         string(configJSON),
		TimeoutSeconds: 10,
	}

	ctx := context.Background()
	executor.Execute(ctx, task, nil)
}

func TestTaskExecutor_ExecuteShellParseError(t *testing.T) {
	executor := NewTaskExecutor(nil)

	task := &pb.Task{
		ExecutionId:    "exec-1",
		TaskId:         1,
		Type:           "shell",
		Config:         "invalid json",
		TimeoutSeconds: 10,
	}

	ctx := context.Background()
	executor.Execute(ctx, task, nil)
}

func TestTaskExecutor_ExecuteUnknownType(t *testing.T) {
	executor := NewTaskExecutor(nil)

	task := &pb.Task{
		ExecutionId:    "exec-1",
		TaskId:         1,
		Type:           "unknown",
		Config:         "{}",
		TimeoutSeconds: 10,
	}

	ctx := context.Background()
	executor.Execute(ctx, task, nil)
}

func TestTaskExecutor_ExecuteHTTPTimeout(t *testing.T) {
	executor := NewTaskExecutor(nil)

	httpConfig := map[string]string{
		"url":    "http://httpbin.org/delay/10",
		"method": "GET",
	}
	configJSON, _ := json.Marshal(httpConfig)

	task := &pb.Task{
		ExecutionId:    "exec-1",
		TaskId:         1,
		Type:           "http",
		Config:         string(configJSON),
		TimeoutSeconds: 1,
	}

	ctx := context.Background()
	start := time.Now()
	executor.Execute(ctx, task, nil)
	duration := time.Since(start)

	if duration > 5*time.Second {
		t.Errorf("expected timeout within 5 seconds, got %v", duration)
	}
}

func TestTaskExecutor_ExecuteShellTimeout(t *testing.T) {
	executor := NewTaskExecutor(nil)

	shellConfig := map[string]string{
		"script": "sleep 10",
	}
	configJSON, _ := json.Marshal(shellConfig)

	task := &pb.Task{
		ExecutionId:    "exec-1",
		TaskId:         1,
		Type:           "shell",
		Config:         string(configJSON),
		TimeoutSeconds: 1,
	}

	ctx := context.Background()
	start := time.Now()
	executor.Execute(ctx, task, nil)
	duration := time.Since(start)

	if duration > 5*time.Second {
		t.Errorf("expected timeout within 5 seconds, got %v", duration)
	}
}
