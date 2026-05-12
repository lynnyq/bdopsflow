package handler

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

// TaskExecutionResponse 用于 API 返回的任务执行记录
type TaskExecutionResponse struct {
	ID          int64   `json:"id"`
	TaskID      int64   `json:"task_id"`
	ExecutionID string  `json:"execution_id"`
	ExecutorID  string  `json:"executor_id"`
	Status      string  `json:"status"`
	StartTime   *string `json:"start_time"`
	EndTime     *string `json:"end_time"`
	Output      string  `json:"output"`
	Error       string  `json:"error"`
	RetryTimes   int32  `json:"retry_times"`
	CreatedAt    string  `json:"created_at"`
}

func toTaskExecutionResponse(exec *model.TaskExecution) *TaskExecutionResponse {
	resp := &TaskExecutionResponse{
		ID:          exec.ID,
		TaskID:       exec.TaskID,
		ExecutionID: exec.ExecutionID,
		ExecutorID:  exec.ExecutorID,
		Status:      exec.Status,
		Output:      exec.Output,
		Error:       exec.Error,
		RetryTimes:   exec.RetryTimes,
		CreatedAt:    exec.CreatedAt.Format(time.RFC3339),
	}

	if exec.StartTime.Valid {
		t := exec.StartTime.Time.Format(time.RFC3339)
		resp.StartTime = &t
	}

	if exec.EndTime.Valid {
		t := exec.EndTime.Time.Format(time.RFC3339)
		resp.EndTime = &t
	}

	return resp
}

type TaskHandler struct {
	svc TaskServicer
}

func NewTaskHandler(svc *service.SchedulerService) *TaskHandler {
	return &TaskHandler{svc: svc}
}

func newTaskHandlerWithSvc(svc TaskServicer) *TaskHandler {
	return &TaskHandler{svc: svc}
}

func (h *TaskHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	tasks, err := h.svc.ListTasks(ctx)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, tasks)
}

func (h *TaskHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	task, err := h.svc.GetTaskByID(ctx, id)
	if err != nil {
		c.JSON(404, gin.H{"error": "task not found"})
		return
	}
	c.JSON(200, task)
}

func (h *TaskHandler) Create(c *gin.Context) {
	var req struct {
		WorkflowID     *int64 `json:"workflow_id"`
		Name           string `json:"name"`
		Type           string `json:"type"`
		Config         string `json:"config"`
		CronExpression string `json:"cron_expression"`
		TimeoutSeconds int32  `json:"timeout_seconds"`
		RetryCount     int32  `json:"retry_count"`
		RetryInterval  int32  `json:"retry_interval"`
		DomainID       int64  `json:"domain_id"`
		WebhookConfig  string `json:"webhook_config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = 300
	}
	if req.RetryCount == 0 {
		req.RetryCount = 3
	}
	if req.RetryInterval == 0 {
		req.RetryInterval = 5
	}
	if req.DomainID == 0 {
		req.DomainID = 1
	}

	var query string
	var args []interface{}
	now := time.Now().Format("2006-01-02 15:04:05")
	ts := int64(req.TimeoutSeconds)
	rc := int64(req.RetryCount)
	ri := int64(req.RetryInterval)

	if req.WorkflowID != nil {
		query = `
			INSERT INTO tasks (workflow_id, name, type, config, cron_expression, timeout_seconds,
			                  retry_count, retry_interval, is_enabled, status, domain_id, webhook_config,
			                  created_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, 'pending', ?, ?, 1, ?, ?)
		`
		args = []interface{}{
			*req.WorkflowID, req.Name, req.Type, req.Config, req.CronExpression, ts,
			rc, ri, req.DomainID, req.WebhookConfig, now, now,
		}
	} else {
		query = `
			INSERT INTO tasks (name, type, config, cron_expression, timeout_seconds,
			                  retry_count, retry_interval, is_enabled, status, domain_id, webhook_config,
			                  created_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 1, 'pending', ?, ?, 1, ?, ?)
		`
		args = []interface{}{
			req.Name, req.Type, req.Config, req.CronExpression, ts,
			rc, ri, req.DomainID, req.WebhookConfig, now, now,
		}
	}

	ctx := c.Request.Context()
	task, err := h.svc.CreateTask(ctx, query, args...)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, task)
}

func (h *TaskHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	var task model.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	err = h.svc.UpdateTask(ctx, id, &task)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	task.ID = id
	c.JSON(200, task)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	err = h.svc.DeleteTask(ctx, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "deleted"})
}

func (h *TaskHandler) Trigger(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	executionID, err := h.svc.TriggerTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "triggered", "execution_id": executionID})
}

func (h *TaskHandler) Executions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	executions, err := h.svc.GetTaskExecutions(ctx, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var response []*TaskExecutionResponse
	for _, exec := range executions {
		response = append(response, toTaskExecutionResponse(exec))
	}

	c.JSON(200, response)
}

func (h *TaskHandler) StreamLogs(c *gin.Context) {
	executionID := c.Query("execution_id")
	if executionID == "" {
		c.JSON(400, gin.H{"error": "execution_id required"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ctx := c.Request.Context()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastLogID int64
	var lastOutputHash uint64
	var lastErrorHash uint64

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 1. 获取并发送新日志
			logs, err := h.svc.GetTaskLogs(ctx, executionID)
			if err == nil {
				for _, log := range logs {
					if log.ID > lastLogID {
						lastLogID = log.ID
						data := fmt.Sprintf(`{"id":%d,"execution_id":"%s","task_id":%d,"node_id":"%s","log_level":"%s","message":"%s","log_time":"%s"}`,
							log.ID, log.ExecutionID, log.TaskID, log.NodeID, log.LogLevel,
							escapeJSON(log.Message), log.LogTime.UTC().Format(time.RFC3339))
						c.Writer.Write([]byte("data: " + data + "\n\n"))
						c.Writer.Flush()
					}
				}
			}

			// 2. 获取并发送最新的执行信息（包含 output 和 error）
			// 我们通过 GetTaskExecutions 获取，因为已经有一个 task_id，不过我们需要获取单个 execution
			// 为了简化，我们先通过所有日志关联的 task_id 来获取
			if len(logs) > 0 {
				taskID := logs[0].TaskID
				executions, execErr := h.svc.GetTaskExecutions(ctx, taskID)
				if execErr == nil {
					for _, exec := range executions {
						if exec.ExecutionID == executionID {
							// 计算 hash 判断是否变化
							outputHash := fnvHash(exec.Output)
							errorHash := fnvHash(exec.Error)

							if outputHash != lastOutputHash || errorHash != lastErrorHash {
								lastOutputHash = outputHash
								lastErrorHash = errorHash

								var outputStr *string
								if exec.Output != "" {
									outputStr = &exec.Output
								}
								var errorStr *string
								if exec.Error != "" {
									errorStr = &exec.Error
								}

								// 构造 execution 更新事件
								var startTime, endTime *string
								if exec.StartTime.Valid {
									t := exec.StartTime.Time.UTC().Format(time.RFC3339)
									startTime = &t
								}
								if exec.EndTime.Valid {
									t := exec.EndTime.Time.UTC().Format(time.RFC3339)
									endTime = &t
								}
								data, _ := json.Marshal(map[string]interface{}{
									"type":       "execution_update",
									"status":     exec.Status,
									"output":     outputStr,
									"error":      errorStr,
									"start_time": startTime,
									"end_time":   endTime,
								})
								c.Writer.Write([]byte("data: " + string(data) + "\n\n"))
								c.Writer.Flush()
							}
							break
						}
					}
				}
			}

			c.Writer.Write([]byte(": heartbeat\n\n"))
			c.Writer.Flush()
		}
	}
}

func fnvHash(s string) uint64 {
	const offset64 = 14695981039346656037
	const prime64 = 1099511628211
	var h uint64 = offset64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}

// TaskLogResponse 用于 API 返回的任务日志
type TaskLogResponse struct {
	ID          int64  `json:"id"`
	ExecutionID string `json:"execution_id"`
	TaskID      int64  `json:"task_id"`
	NodeID      string `json:"node_id"`
	LogLevel    string `json:"log_level"`
	Message     string `json:"message"`
	LogTime     string `json:"log_time"`
}

func toTaskLogResponse(tl *model.TaskLog) *TaskLogResponse {
	return &TaskLogResponse{
		ID:          tl.ID,
		ExecutionID: tl.ExecutionID,
		TaskID:      tl.TaskID,
		NodeID:      tl.NodeID,
		LogLevel:    tl.LogLevel,
		Message:     tl.Message,
		LogTime:     tl.LogTime.UTC().Format(time.RFC3339),
	}
}

func (h *TaskHandler) ExecutionLogs(c *gin.Context) {
	executionID := c.Param("executionId")
	if executionID == "" {
		c.JSON(400, gin.H{"error": "executionId required"})
		return
	}

	ctx := c.Request.Context()
	logs, err := h.svc.GetTaskLogs(ctx, executionID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var response []*TaskLogResponse
	for _, log := range logs {
		response = append(response, toTaskLogResponse(log))
	}

	c.JSON(200, response)
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}
