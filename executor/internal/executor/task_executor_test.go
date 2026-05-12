package executor

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestHTTPExecutor(t *testing.T) {
	executor := NewHTTPExecutor()

	config := TaskConfig{
		URL:    "https://httpbin.org/get",
		Method: "GET",
		Header: map[string]string{
			"User-Agent": "BDopsFlow-Test",
		},
	}

	configJSON, _ := json.Marshal(config)

	ctx := context.Background()
	result, err := executor.Execute(ctx, string(configJSON), 30*time.Second)

	if err != nil {
		t.Logf("HTTP request failed (may be network issue): %v", err)
	}

	if result != nil {
		if result.Status == "" {
			t.Error("expected status to be set")
		}
		t.Logf("Result: Status=%s, Duration=%v", result.Status, result.Duration)
	}
}

func TestHTTPExecutorParseError(t *testing.T) {
	executor := NewHTTPExecutor()

	ctx := context.Background()
	_, err := executor.Execute(ctx, "invalid json", 30*time.Second)

	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestHTTPExecutorTimeout(t *testing.T) {
	executor := NewHTTPExecutor()

	config := TaskConfig{
		URL:    "http://httpbin.org/delay/10",
		Method: "GET",
	}

	configJSON, _ := json.Marshal(config)

	ctx := context.Background()
	result, err := executor.Execute(ctx, string(configJSON), 1*time.Second)

	if err == nil && result != nil {
		if result.Status != "timeout" {
			t.Errorf("expected timeout status, got %s", result.Status)
		}
	}
}

func TestShellExecutor(t *testing.T) {
	executor := NewShellExecutor()

	config := TaskConfig{
		Script: "echo 'Hello World' && date",
	}

	configJSON, _ := json.Marshal(config)

	ctx := context.Background()
	result, err := executor.Execute(ctx, string(configJSON), 10*time.Second)

	if err != nil {
		t.Errorf("shell execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected result to be non-nil")
	}

	if result.Status != "success" {
		t.Errorf("expected success status, got %s: %s", result.Status, result.Error)
	}

	if result.Output == "" {
		t.Error("expected output to be non-empty")
	}

	t.Logf("Shell output: %s", result.Output)
}

func TestShellExecutorParseError(t *testing.T) {
	executor := NewShellExecutor()

	ctx := context.Background()
	_, err := executor.Execute(ctx, "invalid json", 10*time.Second)

	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestShellExecutorTimeout(t *testing.T) {
	executor := NewShellExecutor()

	config := TaskConfig{
		Script: "sleep 10",
	}

	configJSON, _ := json.Marshal(config)

	ctx := context.Background()
	result, err := executor.Execute(ctx, string(configJSON), 1*time.Second)

	if err == nil && result != nil {
		if result.Status != "timeout" {
			t.Errorf("expected timeout status, got %s", result.Status)
		}
	}
}

func TestRetryExecutor(t *testing.T) {
	innerExecutor := &FailingExecutor{failCount: 2}
	retryExecutor := NewRetryExecutor(innerExecutor, 3, 100*time.Millisecond)

	config := TaskConfig{
		Script: "echo test",
	}
	configJSON, _ := json.Marshal(config)

	ctx := context.Background()
	result, err := retryExecutor.Execute(ctx, string(configJSON), 10*time.Second)

	if err != nil {
		t.Errorf("retry execution failed: %v", err)
	}

	if result != nil && result.Status != "success" {
		t.Errorf("expected success status after retries, got %s", result.Status)
	}
}

type FailingExecutor struct {
	failCount int
	callCount int
}

func (e *FailingExecutor) Execute(ctx context.Context, config string, timeout time.Duration) (*TaskResult, error) {
	e.callCount++
	if e.callCount <= e.failCount {
		return &TaskResult{
			Status:  "failed",
			Error:   "intentional failure",
		}, nil
	}
	return &TaskResult{
		Status: "success",
		Output: "success after retries",
	}, nil
}

func TestExecutorFactory(t *testing.T) {
	factory := NewExecutorFactory()

	httpExecutor, err := factory.GetExecutor("http")
	if err != nil {
		t.Errorf("failed to get HTTP executor: %v", err)
	}
	if httpExecutor == nil {
		t.Error("expected HTTP executor to be non-nil")
	}

	shellExecutor, err := factory.GetExecutor("shell")
	if err != nil {
		t.Errorf("failed to get Shell executor: %v", err)
	}
	if shellExecutor == nil {
		t.Error("expected Shell executor to be non-nil")
	}

	_, err = factory.GetExecutor("unknown")
	if err == nil {
		t.Error("expected error for unknown task type")
	}
}

func TestTaskConfigSerialization(t *testing.T) {
	httpConfig := TaskConfig{
		URL:    "http://example.com",
		Method: "POST",
		Header: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"key":"value"}`,
	}

	jsonData, err := json.Marshal(httpConfig)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	var decoded TaskConfig
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if decoded.URL != httpConfig.URL {
		t.Errorf("expected URL '%s', got '%s'", httpConfig.URL, decoded.URL)
	}

	if decoded.Method != httpConfig.Method {
		t.Errorf("expected Method '%s', got '%s'", httpConfig.Method, decoded.Method)
	}
}
