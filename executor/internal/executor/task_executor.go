package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
	"github.com/lynnyq/bdopsflow/executor/internal/grpcclient"
)

type TaskExecutor struct {
	executorID string
}

func NewTaskExecutor(executorID string) *TaskExecutor {
	return &TaskExecutor{executorID: executorID}
}

func (e *TaskExecutor) Execute(ctx context.Context, task *pb.Task, client *grpcclient.Client) {
	slog.Info("task execution started",
		"execution_id", task.ExecutionId,
		"task_id", task.TaskId,
		"type", task.Type,
	)

	sendLog(client, task, "info", "Task execution started")

	output, err := e.executeTask(ctx, task, client)

	status := "success"
	errorMsg := ""
	if err != nil {
		status = "failed"
		errorMsg = err.Error()
		slog.Error("task execution failed",
			"execution_id", task.ExecutionId,
			"task_id", task.TaskId,
			"error", err,
		)
		sendLog(client, task, "error", fmt.Sprintf("Task execution failed: %v", err))
	}

	slog.Info("task execution finished",
		"execution_id", task.ExecutionId,
		"task_id", task.TaskId,
		"status", status,
	)
	sendLog(client, task, "info", fmt.Sprintf("Task execution finished, status: %s", status))

	now := time.Now().Unix()
	client.ReportResult(&pb.ReportTaskResultRequest{
		ExecutionId: task.ExecutionId,
		TaskId:      task.TaskId,
		Status:      status,
		Output:      output,
		Error:       errorMsg,
		StartTime:   now,
		EndTime:     time.Now().Unix(),
		RetryTimes:  0,
	})
}

func (e *TaskExecutor) executeTask(ctx context.Context, task *pb.Task, client *grpcclient.Client) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(task.TimeoutSeconds)*time.Second)
	defer cancel()

	switch task.Type {
	case "http":
		return e.executeHTTP(timeoutCtx, task, client)
	case "shell":
		return e.executeShell(timeoutCtx, task, client)
	default:
		return "", fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

func (e *TaskExecutor) executeHTTP(ctx context.Context, task *pb.Task, client *grpcclient.Client) (string, error) {
	var config struct {
		URL    string `json:"url"`
		Method string `json:"method"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal([]byte(task.Config), &config); err != nil {
		return "", fmt.Errorf("invalid http config: %w", err)
	}

	if config.Method == "" {
		config.Method = "GET"
	}

	sendLog(client, task, "info", fmt.Sprintf("HTTP %s %s", config.Method, config.URL))

	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, bytes.NewBuffer([]byte(config.Body)))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	httpClient := &http.Client{Timeout: time.Duration(task.TimeoutSeconds) * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	body := buf.String()

	sendLog(client, task, "info", fmt.Sprintf("HTTP response: %d, body length: %d", resp.StatusCode, len(body)))

	if resp.StatusCode >= 400 {
		return body, fmt.Errorf("http status %d: %s", resp.StatusCode, body)
	}

	return body, nil
}

func (e *TaskExecutor) executeShell(ctx context.Context, task *pb.Task, client *grpcclient.Client) (string, error) {
	var config struct {
		Script string `json:"script"`
	}
	if err := json.Unmarshal([]byte(task.Config), &config); err != nil {
		return "", fmt.Errorf("invalid shell config: %w", err)
	}

	if config.Script == "" {
		return "", fmt.Errorf("shell script is empty")
	}

	sendLog(client, task, "info", "Executing shell script")

	cmd := exec.CommandContext(ctx, "bash", "-c", config.Script)
	
	// 创建实时读取器
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start command: %w", err)
	}

	// 用于收集完整输出
	var fullOutput, fullError bytes.Buffer
	
	// 启动 goroutine 实时读取并发送 stdout
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				fullOutput.WriteString(chunk)
				// 发送特殊日志标识是 stdout
				sendOutputLog(client, task, "stdout", chunk)
			}
			if err != nil {
				break
			}
		}
	}()
	
	// 启动 goroutine 实时读取并发送 stderr
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				fullError.WriteString(chunk)
				// 发送特殊日志标识是 stderr
				sendOutputLog(client, task, "stderr", chunk)
			}
			if err != nil {
				break
			}
		}
	}()

	// 等待命令完成
	err = cmd.Wait()

	// 构建最终输出
	output := fullOutput.String()
	if fullError.Len() > 0 {
		output += "\n[stderr]\n" + fullError.String()
	}

	if err != nil {
		sendLog(client, task, "error", fmt.Sprintf("Shell execution error: %v", err))
		return output, fmt.Errorf("shell execution failed: %w, stderr: %s", err, fullError.String())
	}

	sendLog(client, task, "info", fmt.Sprintf("Shell execution completed, output length: %d", len(output)))
	return output, nil
}

// 发送 stdout/stderr 的特殊日志
func sendOutputLog(client *grpcclient.Client, task *pb.Task, logType string, message string) {
	err := client.ReportLog(&pb.ReportTaskLogRequest{
		ExecutionId: task.ExecutionId,
		TaskId:      task.TaskId,
		LogLevel:    logType, // 特殊类型：stdout 或 stderr
		LogContent:  message,
		Timestamp:   time.Now().Unix(),
	})
	if err != nil {
		slog.Error("failed to report output log", "error", err, "execution_id", task.ExecutionId)
	}
}

func sendLog(client *grpcclient.Client, task *pb.Task, level string, message string) {
	err := client.ReportLog(&pb.ReportTaskLogRequest{
		ExecutionId: task.ExecutionId,
		TaskId:      task.TaskId,
		LogLevel:    level,
		LogContent:  message,
		Timestamp:   time.Now().Unix(),
	})
	if err != nil {
		slog.Error("failed to report log", "error", err, "execution_id", task.ExecutionId)
	}
}