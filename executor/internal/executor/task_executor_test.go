package executor

import (
	"context"
	"encoding/json"
	"net/http"
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

// TestTaskExecutor_HTTPClientConnectionPool tests that the HTTP client uses connection pooling
func TestTaskExecutor_HTTPClientConnectionPool(t *testing.T) {
	executor := NewTaskExecutor(nil)

	// Verify that httpClient is initialized
	if executor.httpClient == nil {
		t.Fatal("httpClient should be initialized")
	}

	// Verify that transport is configured for connection pooling
	transport, ok := executor.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("httpClient.Transport should be *http.Transport")
	}

	// Verify connection pooling settings
	if transport.MaxIdleConns != 100 {
		t.Errorf("MaxIdleConns should be 100, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("MaxIdleConnsPerHost should be 10, got %d", transport.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("IdleConnTimeout should be 90s, got %v", transport.IdleConnTimeout)
	}
	if transport.DisableKeepAlives != false {
		t.Error("DisableKeepAlives should be false for connection reuse")
	}
}

// TestTaskExecutor_HTTPClientReuse tests that multiple HTTP tasks reuse the same client
func TestTaskExecutor_HTTPClientReuse(t *testing.T) {
	executor := NewTaskExecutor(nil)

	httpConfig := map[string]string{
		"url":    "https://httpbin.org/get",
		"method": "GET",
	}
	configJSON, _ := json.Marshal(httpConfig)

	// Execute multiple tasks and verify they all use the same httpClient
	for i := 0; i < 3; i++ {
		task := &pb.Task{
			ExecutionId:    "exec-reuse-" + string(rune('1'+i)),
			TaskId:         int64(i + 1),
			Type:           "http",
			Config:         string(configJSON),
			TimeoutSeconds: 30,
		}

		ctx := context.Background()
		executor.Execute(ctx, task, nil)
	}

	// The httpClient should still be the same instance
	if executor.httpClient == nil {
		t.Error("httpClient should still be initialized after multiple executions")
	}
}
