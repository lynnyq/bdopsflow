package executor

import (
	"context"
	"testing"
	"time"
)

func TestNewHTTPExecutor(t *testing.T) {
	executor := NewHTTPExecutor(30 * time.Second)
	if executor == nil {
		t.Error("expected non-nil HTTPExecutor")
	}
	if executor.client.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", executor.client.Timeout)
	}
}

func TestHTTPExecutor_Execute_InvalidConfig(t *testing.T) {
	executor := NewHTTPExecutor(5 * time.Second)
	_, err := executor.Execute(context.Background(), "invalid json")
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestHTTPExecutor_Execute_InvalidURL(t *testing.T) {
	executor := NewHTTPExecutor(1 * time.Second)
	config := `{"url": "http://invalid-url-12345.test", "method": "GET"}`
	result, err := executor.Execute(context.Background(), config)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
	if result.Status != "failed" {
		t.Errorf("expected status 'failed', got %s", result.Status)
	}
}

func TestHTTPExecutor_Execute_ContextTimeout(t *testing.T) {
	executor := NewHTTPExecutor(30 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	config := `{"url": "http://httpbin.org/delay/5", "method": "GET"}`
	result, err := executor.Execute(ctx, config)
	if err == nil {
		t.Error("expected error due to context timeout")
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
	if result.Status != "failed" {
		t.Errorf("expected status 'failed', got %s", result.Status)
	}
}

func TestNewShellExecutor(t *testing.T) {
	executor := NewShellExecutor()
	if executor == nil {
		t.Error("expected non-nil ShellExecutor")
	}
}

func TestShellExecutor_Execute_InvalidConfig(t *testing.T) {
	executor := NewShellExecutor()
	_, err := executor.Execute(context.Background(), "invalid json")
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestShellExecutor_Execute_Success(t *testing.T) {
	executor := NewShellExecutor()
	config := `{"script": "echo 'hello world'"}`
	result, err := executor.Execute(context.Background(), config)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected status 'success', got %s", result.Status)
	}
	if result.Output != "hello world\n" {
		t.Errorf("expected output 'hello world\\n', got %q", result.Output)
	}
}

func TestShellExecutor_Execute_Failure(t *testing.T) {
	executor := NewShellExecutor()
	config := `{"script": "exit 1"}`
	result, err := executor.Execute(context.Background(), config)
	if err == nil {
		t.Error("expected error for failing script")
	}
	if result.Status != "failed" {
		t.Errorf("expected status 'failed', got %s", result.Status)
	}
}

func TestShellExecutor_Execute_ContextCancel(t *testing.T) {
	executor := NewShellExecutor()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	config := `{"script": "sleep 10"}`
	result, err := executor.Execute(ctx, config)
	if err == nil {
		t.Error("expected error due to context cancel")
	}
	if result != nil && result.Status != "failed" {
		t.Errorf("expected status 'failed', got %s", result.Status)
	}
}

func TestNewExecutorFactory(t *testing.T) {
	factory := NewExecutorFactory()
	if factory == nil {
		t.Error("expected non-nil ExecutorFactory")
	}
	if factory.httpExecutor == nil {
		t.Error("expected non-nil httpExecutor")
	}
	if factory.shellExecutor == nil {
		t.Error("expected non-nil shellExecutor")
	}
}

func TestExecutorFactory_GetExecutor_HTTP(t *testing.T) {
	factory := NewExecutorFactory()
	executor, err := factory.GetExecutor("http")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if executor == nil {
		t.Error("expected non-nil executor")
	}
	if _, ok := executor.(*HTTPExecutor); !ok {
		t.Error("expected HTTPExecutor")
	}
}

func TestExecutorFactory_GetExecutor_Shell(t *testing.T) {
	factory := NewExecutorFactory()
	executor, err := factory.GetExecutor("shell")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if executor == nil {
		t.Error("expected non-nil executor")
	}
	if _, ok := executor.(*ShellExecutor); !ok {
		t.Error("expected ShellExecutor")
	}
}

func TestExecutorFactory_GetExecutor_Unknown(t *testing.T) {
	factory := NewExecutorFactory()
	executor, err := factory.GetExecutor("unknown")
	if err == nil {
		t.Error("expected error for unknown task type")
	}
	if executor != nil {
		t.Error("expected nil executor for unknown task type")
	}
}

func TestTaskResult_Times(t *testing.T) {
	result := &TaskResult{
		StartAt: time.Now(),
	}
	time.Sleep(10 * time.Millisecond)
	result.EndAt = time.Now()

	if result.EndAt.Before(result.StartAt) {
		t.Error("end time should be after start time")
	}
	if result.EndAt.Sub(result.StartAt) < 10*time.Millisecond {
		t.Error("duration should be at least 10ms")
	}
}