package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/lynnyq/bdopsflow/executor/internal/grpcclient"
	"github.com/lynnyq/bdopsflow/executor/internal/pool"
	pb "github.com/lynnyq/bdopsflow/proto"
)

// 正在运行的任务信息
type RunningTaskInfo struct {
	task            *pb.Task
	startTime       time.Time
	currentProgress int32
	progressMsg     string
}

type TaskExecutor struct {
	taskPool     *pool.Pool
	runningTasks sync.Map // map[string]*RunningTaskInfo
}

func NewTaskExecutor(taskPool *pool.Pool) *TaskExecutor {
	return &TaskExecutor{
		taskPool: taskPool,
	}
}

func (e *TaskExecutor) UpdateCapacity(newCapacity int32) error {
	return e.taskPool.UpdateCapacity(newCapacity)
}

func (e *TaskExecutor) GetRunningExecutionIds() []string {
	var ids []string
	e.runningTasks.Range(func(key, value interface{}) bool {
		if id, ok := key.(string); ok {
			ids = append(ids, id)
		}
		return true
	})
	return ids
}

// 新增：获取正在运行任务的详细状态
func (e *TaskExecutor) GetRunningTaskStates() []*pb.RunningTaskState {
	var states []*pb.RunningTaskState
	e.runningTasks.Range(func(key, value interface{}) bool {
		if executionId, ok := key.(string); ok {
			if info, ok := value.(*RunningTaskInfo); ok {
				state := &pb.RunningTaskState{
					ExecutionId:     executionId,
					TaskId:          info.task.TaskId,
					Progress:        info.currentProgress,
					ProgressMessage: info.progressMsg,
					StartTime:       info.startTime.Unix(),
					Status:          "running",
				}
				states = append(states, state)
			}
		}
		return true
	})
	return states
}

func (e *TaskExecutor) addRunningTask(executionId string, task *pb.Task) {
	e.runningTasks.Store(executionId, &RunningTaskInfo{
		task:            task,
		startTime:       time.Now(),
		currentProgress: 0,
		progressMsg:     "Task started",
	})
	slog.Debug("added task to running list", "execution_id", executionId, "running_count", e.getRunningCount())
}

func (e *TaskExecutor) removeRunningTask(executionId string) {
	e.runningTasks.Delete(executionId)
	slog.Debug("removed task from running list", "execution_id", executionId, "running_count", e.getRunningCount())
}

// 更新任务进度
func (e *TaskExecutor) updateTaskProgress(executionId string, progress int32, msg string) {
	if val, ok := e.runningTasks.Load(executionId); ok {
		if info, ok := val.(*RunningTaskInfo); ok {
			info.currentProgress = progress
			info.progressMsg = msg
			e.runningTasks.Store(executionId, info)
		}
	}
}

func (e *TaskExecutor) getRunningCount() int {
	count := 0
	e.runningTasks.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func (e *TaskExecutor) Execute(ctx context.Context, task *pb.Task, client *grpcclient.MultiClient) {
	slog.Info("task execution started",
		"execution_id", task.ExecutionId,
		"task_id", task.TaskId,
		"type", task.Type,
	)

	sendLog(client, task, "info", "Task execution started")
	e.addRunningTask(task.ExecutionId, task)
	defer e.removeRunningTask(task.ExecutionId)

	if e.taskPool != nil {
		e.taskPool.IncRunning()
		defer e.taskPool.DecRunning()
	}

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
	if client != nil {
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
}

func (e *TaskExecutor) GetRunningTasks() int32 {
	if e.taskPool != nil {
		return e.taskPool.Running()
	}
	return 0
}

func (e *TaskExecutor) executeTask(ctx context.Context, task *pb.Task, client *grpcclient.MultiClient) (string, error) {
	var execCtx context.Context
	var cancel context.CancelFunc

	if task.TimeoutSeconds > 0 {
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(task.TimeoutSeconds)*time.Second)
		defer cancel()
	} else {
		execCtx = ctx
	}

	switch task.Type {
	case "http":
		return e.executeHTTP(execCtx, task, client)
	case "shell":
		return e.executeShell(execCtx, task, client)
	default:
		return "", fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

func (e *TaskExecutor) executeHTTP(ctx context.Context, task *pb.Task, client *grpcclient.MultiClient) (string, error) {
	var config struct {
		URL     string `json:"url"`
		Method  string `json:"method"`
		Body    string `json:"body"`
		Headers string `json:"headers"`
	}
	if err := json.Unmarshal([]byte(task.Config), &config); err != nil {
		return "", fmt.Errorf("invalid http config: %w", err)
	}

	if config.Method == "" {
		config.Method = "GET"
	}

	sendLog(client, task, "info", fmt.Sprintf("🚀 Sending HTTP %s request to: %s", config.Method, config.URL))

	if config.Headers != "" {
		sendLog(client, task, "info", fmt.Sprintf("📋 Request headers: %s", config.Headers))
	}
	if config.Body != "" && config.Method != "GET" {
		sendLog(client, task, "info", fmt.Sprintf("📦 Request body: %s", config.Body))
	}

	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, bytes.NewBuffer([]byte(config.Body)))
	if err != nil {
		sendLog(client, task, "error", fmt.Sprintf("❌ Failed to create request: %v", err))
		return "", fmt.Errorf("create request failed: %w", err)
	}

	if config.Headers != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(config.Headers), &headers); err == nil {
			for key, value := range headers {
				req.Header.Set(key, value)
			}
		}
	}

	var httpClient *http.Client
	if task.TimeoutSeconds > 0 {
		httpClient = &http.Client{Timeout: time.Duration(task.TimeoutSeconds) * time.Second}
	} else {
		httpClient = &http.Client{}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		sendLog(client, task, "error", fmt.Sprintf("❌ HTTP request failed: %v", err))
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		sendLog(client, task, "error", fmt.Sprintf("❌ Failed to read response body: %v", err))
		return "", fmt.Errorf("read response body failed: %w", err)
	}
	body := string(bodyBytes)

	contentType := resp.Header.Get("Content-Type")

	if resp.StatusCode >= 400 {
		sendLog(client, task, "error", fmt.Sprintf("❌ HTTP Error Response"))
		sendLog(client, task, "error", fmt.Sprintf("Status: %d %s", resp.StatusCode, resp.Status))
		sendLog(client, task, "error", fmt.Sprintf("Content-Type: %s", contentType))

		if strings.Contains(strings.ToLower(contentType), "json") {
			var jsonData interface{}
			if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
				formattedJSON, _ := json.MarshalIndent(jsonData, "", "  ")
				sendLog(client, task, "error", fmt.Sprintf("Response Body (JSON):\n%s", string(formattedJSON)))
			} else {
				sendOutputLog(client, task, "stderr", body)
			}
		} else {
			sendOutputLog(client, task, "stderr", body)
		}

		return body, fmt.Errorf("http status %d: %s", resp.StatusCode, body)
	}

	sendLog(client, task, "info", fmt.Sprintf("✅ HTTP Success Response"))
	sendLog(client, task, "info", fmt.Sprintf("Status: %d %s", resp.StatusCode, resp.Status))
	sendLog(client, task, "info", fmt.Sprintf("Content-Type: %s", contentType))
	sendLog(client, task, "info", fmt.Sprintf("Response size: %d bytes", len(body)))

	if strings.Contains(strings.ToLower(contentType), "json") {
		var jsonData interface{}
		if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
			formattedJSON, prettyJSONErr := json.MarshalIndent(jsonData, "", "  ")
			if prettyJSONErr == nil {
				sendLog(client, task, "info", fmt.Sprintf("📄 Response Body (JSON):"))
				sendOutputLog(client, task, "stdout", string(formattedJSON))
			} else {
				sendOutputLog(client, task, "stdout", body)
			}
		} else {
			sendOutputLog(client, task, "stdout", body)
		}
	} else if strings.Contains(strings.ToLower(contentType), "text") ||
		strings.Contains(strings.ToLower(contentType), "html") {
		sendLog(client, task, "info", fmt.Sprintf("📄 Response Body (Text):"))
		sendOutputLog(client, task, "stdout", body)
	} else {
		if len(body) > 0 {
			sendLog(client, task, "info", fmt.Sprintf("📄 Response Body (Binary/Unknown, %d bytes)", len(body)))
			previewLen := len(body)
			if previewLen > 1024 {
				previewLen = 1024
				sendOutputLog(client, task, "stdout", body[:previewLen]+"\n... (truncated)")
			} else {
				sendOutputLog(client, task, "stdout", body)
			}
		}
	}

	return body, nil
}

func (e *TaskExecutor) executeShell(ctx context.Context, task *pb.Task, client *grpcclient.MultiClient) (string, error) {
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

	var fullOutput, fullError bytes.Buffer

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				fullOutput.WriteString(chunk)
				sendOutputLog(client, task, "stdout", chunk)
			}
			if err != nil {
				break
			}
		}
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				fullError.WriteString(chunk)
				sendOutputLog(client, task, "stderr", chunk)
			}
			if err != nil {
				break
			}
		}
	}()

	err = cmd.Wait()

	output := fullOutput.String()
	stderrOutput := fullError.String()

	if err != nil {
		sendLog(client, task, "error", fmt.Sprintf("Shell execution error: %v", err))
		// 失败时 output 为空，标准输出和标准错误都放在返回的 error 中
		return "", fmt.Errorf("shell execution failed: %w, stdout: %s, stderr: %s", err, output, stderrOutput)
	}

	sendLog(client, task, "info", fmt.Sprintf("Shell execution completed, output length: %d", len(output)))
	return output, nil
}

func sendOutputLog(client *grpcclient.MultiClient, task *pb.Task, logType string, message string) {
	if client == nil {
		return
	}
	err := client.ReportLog(&pb.ReportTaskLogRequest{
		ExecutionId: task.ExecutionId,
		TaskId:      task.TaskId,
		LogLevel:    logType,
		LogContent:  message,
		Timestamp:   time.Now().Unix(),
	})
	if err != nil {
		slog.Error("failed to report output log", "error", err, "execution_id", task.ExecutionId)
	}
}

func sendLog(client *grpcclient.MultiClient, task *pb.Task, level string, message string) {
	if client == nil {
		return
	}
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
