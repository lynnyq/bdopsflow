package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"
)

type TaskConfig struct {
	URL    string            `json:"url"`
	Method string            `json:"method"`
	Header map[string]string `json:"header"`
	Body   string            `json:"body"`
	Script string            `json:"script"`
}

type TaskResult struct {
	Status  string
	Output  string
	Error   string
	StartAt time.Time
	EndAt   time.Time
}

type TaskExecutor interface {
	Execute(ctx context.Context, config string) (*TaskResult, error)
}

type HTTPExecutor struct {
	client *http.Client
}

func NewHTTPExecutor(timeout time.Duration) *HTTPExecutor {
	return &HTTPExecutor{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (e *HTTPExecutor) Execute(ctx context.Context, configStr string) (*TaskResult, error) {
	var config TaskConfig
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	result := &TaskResult{
		StartAt: time.Now(),
	}

	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, bytes.NewBufferString(config.Body))
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		result.EndAt = time.Now()
		return result, err
	}

	for key, value := range config.Header {
		req.Header.Set(key, value)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		result.EndAt = time.Now()
		return result, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		result.EndAt = time.Now()
		return result, err
	}

	result.Status = "success"
	result.Output = string(body)
	result.EndAt = time.Now()

	return result, nil
}

type ShellExecutor struct{}

func NewShellExecutor() *ShellExecutor {
	return &ShellExecutor{}
}

func (e *ShellExecutor) Execute(ctx context.Context, configStr string) (*TaskResult, error) {
	var config TaskConfig
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	result := &TaskResult{
		StartAt: time.Now(),
	}

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", config.Script)

	output, err := cmd.Output()
	if err != nil {
		result.Status = "failed"
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Error = string(exitErr.Stderr)
		} else {
			result.Error = err.Error()
		}
		result.EndAt = time.Now()
		return result, err
	}

	result.Status = "success"
	result.Output = string(output)
	result.EndAt = time.Now()

	return result, nil
}

type ExecutorFactory struct {
	httpExecutor *HTTPExecutor
	shellExecutor *ShellExecutor
}

func NewExecutorFactory() *ExecutorFactory {
	return &ExecutorFactory{
		httpExecutor: NewHTTPExecutor(5 * time.Minute),
		shellExecutor: NewShellExecutor(),
	}
}

func (f *ExecutorFactory) GetExecutor(taskType string) (TaskExecutor, error) {
	switch taskType {
	case "http":
		return f.httpExecutor, nil
	case "shell":
		return f.shellExecutor, nil
	default:
		return nil, fmt.Errorf("unknown task type: %s", taskType)
	}
}
