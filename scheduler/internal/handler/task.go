package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

const (
	DateTimeFormat = "2006-01-02 15:04:05"
	TimeResponseFormat = time.RFC3339
)

func safeString(s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimSpace(s)
}

func safeTimePtr(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.UTC().Format(TimeResponseFormat)
	return &s
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

	defer func() {
		if r := recover(); r != nil {
			slog.Error("TaskHandler.List: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	slog.Debug("TaskHandler.List: handling request")

	tasks, err := h.svc.ListTasks(ctx)
	if err != nil {
		slog.Error("TaskHandler.List: failed to list tasks", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tasks)
}

func (h *TaskHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("TaskHandler.Get: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("TaskHandler.Get: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	ctx := c.Request.Context()
	task, err := h.svc.GetTaskByID(ctx, id)
	if err != nil {
		slog.Error("TaskHandler.Get: failed to get task", "id", id, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, task)
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
		slog.Warn("TaskHandler.Create: invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if safeString(req.Name) == "" {
		slog.Warn("TaskHandler.Create: name is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if safeString(req.Type) == "" {
		slog.Warn("TaskHandler.Create: type is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "type is required"})
		return
	}

	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = 300
	}
	if req.RetryCount <= 0 {
		req.RetryCount = 3
	}
	if req.RetryInterval <= 0 {
		req.RetryInterval = 5
	}
	if req.DomainID <= 0 {
		req.DomainID = 1
	}

	var query string
	var args []interface{}
	now := time.Now().Format(DateTimeFormat)
	ts := int64(req.TimeoutSeconds)
	rc := int64(req.RetryCount)
	ri := int64(req.RetryInterval)

	if req.WorkflowID != nil && *req.WorkflowID > 0 {
		query = `
			INSERT INTO tasks (workflow_id, name, type, config, cron_expression, timeout_seconds,
			                  retry_count, retry_interval, is_enabled, status, domain_id, webhook_config,
			                  created_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, 'pending', ?, ?, 1, ?, ?)
		`
		args = []interface{}{
			*req.WorkflowID, safeString(req.Name), safeString(req.Type), safeString(req.Config),
			safeString(req.CronExpression), ts, rc, ri, req.DomainID, safeString(req.WebhookConfig), now, now,
		}
	} else {
		query = `
			INSERT INTO tasks (name, type, config, cron_expression, timeout_seconds,
			                  retry_count, retry_interval, is_enabled, status, domain_id, webhook_config,
			                  created_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 1, 'pending', ?, ?, 1, ?, ?)
		`
		args = []interface{}{
			safeString(req.Name), safeString(req.Type), safeString(req.Config),
			safeString(req.CronExpression), ts, rc, ri, req.DomainID, safeString(req.WebhookConfig), now, now,
		}
	}

	ctx := c.Request.Context()
	task, err := h.svc.CreateTask(ctx, query, args...)
	if err != nil {
		slog.Error("TaskHandler.Create: failed to create task", "name", req.Name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("TaskHandler.Create: task created", "task_id", task.ID, "name", task.Name)
	c.JSON(http.StatusCreated, task)
}

func (h *TaskHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("TaskHandler.Update: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("TaskHandler.Update: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	var task model.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		slog.Warn("TaskHandler.Update: invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	err = h.svc.UpdateTask(ctx, id, &task)
	if err != nil {
		slog.Error("TaskHandler.Update: failed to update task", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	task.ID = id
	slog.Info("TaskHandler.Update: task updated", "id", id)
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("TaskHandler.Delete: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("TaskHandler.Delete: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	ctx := c.Request.Context()
	err = h.svc.DeleteTask(ctx, id)
	if err != nil {
		slog.Error("TaskHandler.Delete: failed to delete task", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("TaskHandler.Delete: task deleted", "id", id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *TaskHandler) Trigger(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("TaskHandler.Trigger: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("TaskHandler.Trigger: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	executionID, err := h.svc.TriggerTask(c.Request.Context(), id)
	if err != nil {
		slog.Error("TaskHandler.Trigger: failed to trigger task", "task_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("TaskHandler.Trigger: task triggered", "task_id", id, "execution_id", executionID)
	c.JSON(http.StatusOK, gin.H{"message": "triggered", "execution_id": executionID})
}

func (h *TaskHandler) Executions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("TaskHandler.Executions: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("TaskHandler.Executions: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	ctx := c.Request.Context()
	executions, err := h.svc.GetTaskExecutions(ctx, id)
	if err != nil {
		slog.Error("TaskHandler.Executions: failed to get executions", "task_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var response []*TaskExecutionResponse
	for _, exec := range executions {
		response = append(response, toTaskExecutionResponse(exec))
	}

	c.JSON(http.StatusOK, response)
}

func (h *TaskHandler) StreamLogs(c *gin.Context) {
	executionID := c.Query("execution_id")
	if safeString(executionID) == "" {
		slog.Warn("TaskHandler.StreamLogs: execution_id required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "execution_id required"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	slog.Info("TaskHandler.StreamLogs: starting stream", "execution_id", executionID)

	ctx := c.Request.Context()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastLogID int64
	var lastOutputHash uint64
	var lastErrorHash uint64

	for {
		select {
		case <-ctx.Done():
			slog.Debug("TaskHandler.StreamLogs: context cancelled", "execution_id", executionID)
			return
		case <-ticker.C:
			logs, err := h.svc.GetTaskLogs(ctx, executionID)
			if err != nil {
				slog.Warn("TaskHandler.StreamLogs: failed to get logs", "execution_id", executionID, "error", err)
			} else {
				for _, log := range logs {
					if log.ID > lastLogID {
						lastLogID = log.ID
						data := fmt.Sprintf(`{"id":%d,"execution_id":"%s","task_id":%d,"node_id":"%s","log_level":"%s","message":"%s","log_time":"%s"}`,
							log.ID, log.ExecutionID, log.TaskID, log.NodeID, log.LogLevel,
							escapeJSON(log.Message), log.LogTime.UTC().Format(TimeResponseFormat))
						c.Writer.Write([]byte("data: " + data + "\n\n"))
						c.Writer.Flush()
					}
				}
			}

			if len(logs) > 0 {
				taskID := logs[0].TaskID
				executions, execErr := h.svc.GetTaskExecutions(ctx, taskID)
				if execErr != nil {
					slog.Warn("TaskHandler.StreamLogs: failed to get executions", "task_id", taskID, "error", execErr)
				} else {
					for _, exec := range executions {
						if exec.ExecutionID == executionID {
							outputHash := fnvHash(exec.Output)
							errorHash := fnvHash(exec.Error)

							if outputHash != lastOutputHash || errorHash != lastErrorHash {
								lastOutputHash = outputHash
								lastErrorHash = errorHash

								data, _ := json.Marshal(map[string]interface{}{
									"type":       "execution_update",
									"status":     exec.Status,
									"output":     safeString(exec.Output),
									"error":      safeString(exec.Error),
									"start_time": safeTimePtr(exec.StartTime.Time),
									"end_time":   safeTimePtr(exec.EndTime.Time),
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

type TaskExecutionResponse struct {
	ID           int64   `json:"id"`
	TaskID       int64   `json:"task_id"`
	ExecutionID  string  `json:"execution_id"`
	ExecutorID   string  `json:"executor_id"`
	ExecutorName *string `json:"executor_name,omitempty"`
	TaskName    *string `json:"task_name,omitempty"`
	TaskType    *string `json:"task_type,omitempty"`
	Status      string  `json:"status"`
	StartTime   *string `json:"start_time,omitempty"`
	EndTime     *string `json:"end_time,omitempty"`
	Output      string  `json:"output,omitempty"`
	Error       string  `json:"error,omitempty"`
	RetryTimes  int32   `json:"retry_times"`
	CreatedAt   string  `json:"created_at"`
}

func toTaskExecutionResponse(exec *model.TaskExecution) *TaskExecutionResponse {
	resp := &TaskExecutionResponse{
		ID:          exec.ID,
		TaskID:      exec.TaskID,
		ExecutionID: exec.ExecutionID,
		ExecutorID:  exec.ExecutorID,
		Status:      exec.Status,
		Output:      exec.Output,
		Error:       exec.Error,
		RetryTimes:  exec.RetryTimes,
		CreatedAt:   exec.CreatedAt.UTC().Format(TimeResponseFormat),
	}

	if exec.StartTime.Valid {
		resp.StartTime = safeTimePtr(exec.StartTime.Time)
	}
	if exec.EndTime.Valid {
		resp.EndTime = safeTimePtr(exec.EndTime.Time)
	}

	return resp
}

type TaskLogResponse struct {
	ID          int64  `json:"id"`
	ExecutionID string `json:"execution_id"`
	TaskID      int64  `json:"task_id"`
	ExecutorID  string `json:"executor_id,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
	LogLevel    string `json:"log_level,omitempty"`
	Message     string `json:"message,omitempty"`
	LogTime     string `json:"log_time"`
}

func toTaskLogResponse(tl *model.TaskLog) *TaskLogResponse {
	return &TaskLogResponse{
		ID:          tl.ID,
		ExecutionID: tl.ExecutionID,
		TaskID:      tl.TaskID,
		ExecutorID:  tl.ExecutorID,
		NodeID:      tl.NodeID,
		LogLevel:    tl.LogLevel,
		Message:     tl.Message,
		LogTime:     tl.LogTime.UTC().Format(TimeResponseFormat),
	}
}

func (h *TaskHandler) ExecutionLogs(c *gin.Context) {
	executionID := c.Param("executionId")
	if safeString(executionID) == "" {
		slog.Warn("TaskHandler.ExecutionLogs: executionId required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "executionId required"})
		return
	}

	ctx := c.Request.Context()
	logs, err := h.svc.GetTaskLogs(ctx, executionID)
	if err != nil {
		slog.Error("TaskHandler.ExecutionLogs: failed to get logs", "execution_id", executionID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var response []*TaskLogResponse
	for _, log := range logs {
		response = append(response, toTaskLogResponse(log))
	}

	c.JSON(http.StatusOK, response)
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}
