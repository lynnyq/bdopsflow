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
	"syscall"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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
	cancel          context.CancelFunc // 取消任务的函数
}

type TaskExecutor struct {
	taskPool     *pool.Pool
	runningTasks sync.Map     // map[string]*RunningTaskInfo
	httpClient   *http.Client // 共享的 HTTP 客户端，复用连接池
	taskWg       sync.WaitGroup // 跟踪所有正在执行的任务
	shutdownMu   sync.Mutex   // 保护 shutdown 状态
	shuttingDown bool         // 标记是否正在关闭
}

func NewTaskExecutor(taskPool *pool.Pool) *TaskExecutor {
	// 配置共享的 HTTP Transport，启用连接池复用
	transport := &http.Transport{
		MaxIdleConns:        100,              // 最大空闲连接数
		MaxIdleConnsPerHost: 10,               // 每个主机最大空闲连接数
		IdleConnTimeout:     90 * time.Second, // 空闲连接超时时间
		DisableKeepAlives:   false,            // 启用 keep-alive
	}

	return &TaskExecutor{
		taskPool: taskPool,
		httpClient: &http.Client{
			Transport: transport,
			// 注意：不在这里设置 Timeout，由 context 控制每个请求的超时
		},
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

func (e *TaskExecutor) addRunningTask(executionId string, task *pb.Task, cancel context.CancelFunc) {
	e.runningTasks.Store(executionId, &RunningTaskInfo{
		task:            task,
		startTime:       time.Now(),
		currentProgress: 0,
		progressMsg:     "Task started",
		cancel:          cancel,
	})
	slog.Debug("added task to running list", "execution_id", executionId, "running_count", e.getRunningCount())
}

func (e *TaskExecutor) removeRunningTask(executionId string) {
	e.runningTasks.Delete(executionId)
	slog.Debug("removed task from running list", "execution_id", executionId, "running_count", e.getRunningCount())
}

// 取消指定的任务
func (e *TaskExecutor) CancelTask(executionId string) bool {
	if val, ok := e.runningTasks.Load(executionId); ok {
		if info, ok := val.(*RunningTaskInfo); ok {
			if info.cancel != nil {
				info.cancel()
				slog.Info("task cancellation executed", "execution_id", executionId)
				return true
			}
		}
	}
	return false
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
	// 检查是否正在关闭
	e.shutdownMu.Lock()
	if e.shuttingDown {
		e.shutdownMu.Unlock()
		slog.Warn("executor is shutting down, rejecting task", "execution_id", task.ExecutionId)
		return
	}
	e.taskWg.Add(1)
	e.shutdownMu.Unlock()

	defer e.taskWg.Done()

	executorName := ""
	if client != nil {
		executorName = client.GetExecutorName()
	}

	slog.Info("task execution started",
		"execution_id", task.ExecutionId,
		"task_id", task.TaskId,
		"type", task.Type,
		"executor_name", executorName,
	)

	// 创建可取消的 context，同时支持超时和手动取消
	var execCtx context.Context
	var cancel context.CancelFunc

	if task.TimeoutSeconds > 0 {
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(task.TimeoutSeconds)*time.Second)
	} else {
		execCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	if executorName != "" {
		sendLog(client, task, "info", fmt.Sprintf("Task execution started on executor: %s", executorName))
	} else {
		sendLog(client, task, "info", "Task execution started")
	}
	e.addRunningTask(task.ExecutionId, task, cancel)
	defer e.removeRunningTask(task.ExecutionId)

	if e.taskPool != nil {
		e.taskPool.IncRunning()
		defer e.taskPool.DecRunning()
	}

	output, err := e.executeTask(execCtx, task, client)

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
		if err := client.ReportResult(&pb.ReportTaskResultRequest{
			ExecutionId: task.ExecutionId,
			TaskId:      task.TaskId,
			Status:      status,
			Output:      sanitizeUTF8(output),
			Error:       sanitizeUTF8(errorMsg),
			StartTime:   now,
			EndTime:     time.Now().Unix(),
			RetryTimes:  0,
		}); err != nil {
			slog.Error("failed to report task result",
				"execution_id", task.ExecutionId,
				"task_id", task.TaskId,
				"error", err,
			)
		}
	}
}

func (e *TaskExecutor) GetRunningTasks() int32 {
	if e.taskPool != nil {
		return e.taskPool.Running()
	}
	return 0
}

// Shutdown 优雅关闭执行器，等待所有任务完成或超时
func (e *TaskExecutor) Shutdown(timeout time.Duration) {
	e.shutdownMu.Lock()
	if e.shuttingDown {
		e.shutdownMu.Unlock()
		return
	}
	e.shuttingDown = true
	e.shutdownMu.Unlock()

	slog.Info("executor shutdown started", "timeout", timeout)

	// 取消所有正在执行的任务
	e.runningTasks.Range(func(key, value interface{}) bool {
		if executionId, ok := key.(string); ok {
			if info, ok := value.(*RunningTaskInfo); ok && info.cancel != nil {
				slog.Info("cancelling task during shutdown", "execution_id", executionId)
				info.cancel()
			}
		}
		return true
	})

	// 等待所有任务完成，带超时控制
	done := make(chan struct{})
	go func() {
		e.taskWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("executor shutdown completed gracefully")
	case <-time.After(timeout):
		slog.Warn("executor shutdown timeout exceeded, forcing shutdown", "timeout", timeout)
		// 记录仍在运行的任务
		runningCount := 0
		e.runningTasks.Range(func(key, value interface{}) bool {
			if executionId, ok := key.(string); ok {
				runningCount++
				slog.Warn("task still running after shutdown timeout", "execution_id", executionId)
			}
			return true
		})
		if runningCount > 0 {
			slog.Warn("forcing shutdown with running tasks", "count", runningCount)
		}
	}
}

func (e *TaskExecutor) executeTask(ctx context.Context, task *pb.Task, client *grpcclient.MultiClient) (string, error) {
	switch task.Type {
	case "http":
		return e.executeHTTP(ctx, task, client)
	case "shell":
		return e.executeShell(ctx, task, client)
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

	// 使用共享的 HTTP 客户端，通过 context 控制超时时间
	resp, err := e.httpClient.Do(req)
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

	// 使用进程组，确保取消时能杀死整个进程树（包括子进程）
	cmd := exec.CommandContext(ctx, "bash", "-c", config.Script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // 创建新的进程组
	}

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

	// 监听 context 取消信号，杀死整个进程组
	go func() {
		<-ctx.Done()
		if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
			// 向进程组发送 SIGTERM，杀死整个进程树
			if cmd.Process != nil {
				pgid, err := syscall.Getpgid(cmd.Process.Pid)
				if err == nil {
					// 向进程组发送信号（负 PID 表示进程组）
					if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
						slog.Warn("failed to send SIGTERM to process group",
							"execution_id", task.ExecutionId,
							"pgid", pgid,
							"error", err)
						// 降级：直接杀死主进程
						cmd.Process.Kill()
					} else {
						slog.Info("sent SIGTERM to process group",
							"execution_id", task.ExecutionId,
							"pgid", pgid)
						// 等待一小段时间，如果进程还没退出，发送 SIGKILL
						time.Sleep(3 * time.Second)
						if cmd.ProcessState == nil {
							syscall.Kill(-pgid, syscall.SIGKILL)
							slog.Info("sent SIGKILL to process group",
								"execution_id", task.ExecutionId,
								"pgid", pgid)
						}
					}
				} else {
					cmd.Process.Kill()
				}
			}
		}
	}()

	// 使用线程安全的 buffer 和互斥锁保护并发写入
	var fullOutput, fullError bytes.Buffer
	var outputMu, errorMu sync.Mutex

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				outputMu.Lock()
				fullOutput.WriteString(chunk)
				outputMu.Unlock()
				sendOutputLog(client, task, "stdout", chunk)
			}
			if err != nil {
				break
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				errorMu.Lock()
				fullError.WriteString(chunk)
				errorMu.Unlock()
				sendOutputLog(client, task, "stderr", chunk)
			}
			if err != nil {
				break
			}
		}
	}()

	err = cmd.Wait()
	wg.Wait() // 等待所有读取 goroutine 完成

	output := fullOutput.String()
	stderrOutput := fullError.String()

	// 检查是否是被取消导致的退出
	if ctx.Err() == context.Canceled {
		sendLog(client, task, "info", "Task cancelled by user, process terminated")
		return output, fmt.Errorf("task cancelled by user")
	}

	if err != nil {
		sendLog(client, task, "error", fmt.Sprintf("Shell execution error: %v", err))
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
		LogContent:  sanitizeUTF8(message),
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
		LogContent:  sanitizeUTF8(message),
		Timestamp:   time.Now().Unix(),
	})
	if err != nil {
		slog.Error("failed to report log", "error", err, "execution_id", task.ExecutionId)
	}
}

func sanitizeUTF8(s string) string {
	if s == "" {
		return s
	}
	if utf8.ValidString(s) {
		return s
	}
	var result []rune
	for _, r := range s {
		if r != '\uFFFD' {
			result = append(result, r)
		}
	}
	return string(result)
}
