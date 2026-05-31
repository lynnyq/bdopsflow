package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

const (
	TimeResponseFormat     = time.RFC3339Nano
	ExecutorHeartbeatTimeout = 30 // 心跳超时时间（秒）
)

// isExecutorOnline 检查执行器是否真正在线（考虑心跳超时）
func isExecutorOnline(exec *model.Executor) bool {
	if exec.Status != "online" {
		return false
	}
	if !exec.LastHeartbeat.Valid {
		return false
	}
	localTime := service.ConvertToLocalTime(exec.LastHeartbeat.Time)
	return time.Since(localTime) <= time.Duration(ExecutorHeartbeatTimeout)*time.Second
}

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
	s := t.Format(TimeResponseFormat)
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
			Fail(c, CodeInternalError, "internal server error")
		}
	}()

	slog.Debug("TaskHandler.List: handling request")

	domainID, _ := c.Get("current_domain_id")
	userRole, _ := c.Get("role")

	var dID int64
	var role string
	if v, ok := domainID.(int64); ok {
		dID = v
	}
	if v, ok := userRole.(string); ok {
		role = v
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	bdopsflow_tasks, total, err := h.svc.ListTasks(ctx, dID, role, page, pageSize)
	if err != nil {
		slog.Error("TaskHandler.List: failed to list bdopsflow_tasks", "error", err)
		FailFromError(c, err)
		return
	}

	Success(c, gin.H{"items": bdopsflow_tasks, "total": total})
}

func (h *TaskHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("TaskHandler.Get: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("TaskHandler.Get: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	ctx := c.Request.Context()
	task, err := h.svc.GetTaskByID(ctx, id)
	if err != nil {
		slog.Error("TaskHandler.Get: failed to get task", "id", id, "error", err)
		NotFound(c, "task not found")
		return
	}
	Success(c, task)
}

func (h *TaskHandler) Create(c *gin.Context) {
	var req struct {
		Name               string      `json:"name"`
		Type               string      `json:"type"`
		Config             interface{} `json:"config"`
		CronExpression     string      `json:"cron_expression"`
		TimeoutSeconds     int32       `json:"timeout_seconds"`
		RetryMax           int32       `json:"retry_max"`
		RetryDelaySeconds  int32       `json:"retry_delay_seconds"`
		RetryCount         int32       `json:"retry_count"`
		RetryInterval      int32       `json:"retry_interval"`
		IsEnabled          bool        `json:"is_enabled"`
		DomainID           int64       `json:"domain_id"`
		WebhookID          *int64  `json:"webhook_id"`
		WebhookEvents      string  `json:"webhook_events"`
		AssignedExecutorID int64     `json:"assigned_executor_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("TaskHandler.Create: invalid request body", "error", err)
		BadRequest(c, err.Error())
		return
	}

	if safeString(req.Name) == "" {
		slog.Warn("TaskHandler.Create: name is required")
		BadRequest(c, "name is required")
		return
	}
	if safeString(req.Type) == "" {
		slog.Warn("TaskHandler.Create: type is required")
		BadRequest(c, "type is required")
		return
	}

	validTypes := map[string]bool{"http": true, "shell": true}
	if !validTypes[req.Type] {
		slog.Warn("TaskHandler.Create: invalid task type", "type", req.Type)
		BadRequest(c, "无效的任务类型，支持的类型：http、shell")
		return
	}

	// 处理 Config，支持对象和字符串
	configStr := ""
	if req.Config != nil {
		if str, ok := req.Config.(string); ok {
			configStr = str
		} else {
			configBytes, _ := json.Marshal(req.Config)
			configStr = string(configBytes)
		}
	}

	// 兼容新旧字段名
	retryCount := req.RetryCount
	if retryCount < 0 {
		retryCount = req.RetryMax
	}
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount > 10 {
		retryCount = 10
	}

	retryInterval := req.RetryInterval
	if retryInterval < 0 {
		retryInterval = req.RetryDelaySeconds
	}
	if retryInterval <= 0 && retryCount > 0 {
		retryInterval = 5
	}
	if retryInterval > 3600 {
		retryInterval = 3600
	}

	timeoutSeconds := req.TimeoutSeconds
	if timeoutSeconds < 0 {
		timeoutSeconds = 0
	}
	if timeoutSeconds > 86400 {
		timeoutSeconds = 86400
	}

	domainID := req.DomainID
	if domainID <= 0 {
		if jwtDomainID, ok := c.Get("current_domain_id"); ok {
			if v, typeOk := jwtDomainID.(int64); typeOk && v > 0 {
				domainID = v
			}
		}
	}
	if domainID <= 0 {
		domainID = 1
	}

	// 处理 AssignedExecutorID，如果为 0 或未指定则设置为 NULL
	var assignedExecutorID interface{}
	if req.AssignedExecutorID > 0 {
		assignedExecutorID = req.AssignedExecutorID
	} else {
		assignedExecutorID = nil
	}

	ctx := c.Request.Context()

	// 检查是否有可用的执行器
	hasAvailableExecutors := true
	executors, err := h.svc.ListExecutorsByDomain(ctx, domainID)
	if err != nil {
		slog.Warn("TaskHandler.Create: failed to list executors", "error", err)
		hasAvailableExecutors = false
	} else {
		availableExecutors := 0
		for _, exec := range executors {
			if isExecutorOnline(exec) && exec.CurrentLoad < exec.Capacity {
				availableExecutors++
			}
		}
		hasAvailableExecutors = availableExecutors > 0
	}

	var query string
	var args []interface{}
	now := time.Now().Format(service.DateTimeFormat)
	ts := int64(timeoutSeconds)
	rc := int64(retryCount)
	ri := int64(retryInterval)

	isEnabled := int64(0)
	if req.IsEnabled && hasAvailableExecutors {
		isEnabled = 1
	}

	query = `
		INSERT INTO bdopsflow_tasks (name, type, config, cron_expression, timeout_seconds,
		                  retry_count, retry_interval, is_enabled, status, domain_id, webhook_id, webhook_events,
		                  assigned_executor_id, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, ?, ?, ?, 1, ?, ?)
	`
	args = []interface{}{
		safeString(req.Name), safeString(req.Type), safeString(configStr),
		safeString(req.CronExpression), ts, rc, ri, isEnabled, domainID, req.WebhookID, req.WebhookEvents,
		assignedExecutorID, now, now,
	}

	task, err := h.svc.CreateTask(ctx, query, args...)
	if err != nil {
		slog.Error("TaskHandler.Create: failed to create task", "name", req.Name, "error", err)
		FailFromError(c, err)
		return
	}

	slog.Info("TaskHandler.Create: task created", "task_id", task.ID, "name", task.Name)

	// 返回任务和是否有可用执行器的信息
	Created(c, gin.H{
		"task": task,
		"has_available_executors": hasAvailableExecutors,
	})
}

func (h *TaskHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("TaskHandler.Update: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("TaskHandler.Update: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	ctx := c.Request.Context()
	// 先获取当前任务
	currentTask, err := h.svc.GetTaskByID(ctx, id)
	if err != nil {
		slog.Error("TaskHandler.Update: task not found", "id", id, "error", err)
		NotFound(c, "task not found")
		return
	}

	var req struct {
		Name               string      `json:"name"`
		Type               string      `json:"type"`
		Config             interface{} `json:"config"`
		CronExpression     string      `json:"cron_expression"`
		TimeoutSeconds     int32       `json:"timeout_seconds"`
		RetryMax           int32       `json:"retry_max"`
		RetryDelaySeconds  int32       `json:"retry_delay_seconds"`
		RetryCount         int32       `json:"retry_count"`
		RetryInterval      int32       `json:"retry_interval"`
		IsEnabled          *bool       `json:"is_enabled"`
		DomainID           int64       `json:"domain_id"`
		WebhookID          *int64  `json:"webhook_id"`
		WebhookEvents      string  `json:"webhook_events"`
		AssignedExecutorID int64       `json:"assigned_executor_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("TaskHandler.Update: invalid request body", "error", err)
		BadRequest(c, err.Error())
		return
	}

	// 更新字段
	if req.Name != "" {
		currentTask.Name = req.Name
	}
	if req.Type != "" {
		validTypes := map[string]bool{"http": true, "shell": true}
		if !validTypes[req.Type] {
			slog.Warn("TaskHandler.Update: invalid task type", "type", req.Type)
			BadRequest(c, "无效的任务类型，支持的类型：http、shell")
			return
		}
		currentTask.Type = req.Type
	}

	// 处理 Config，支持对象和字符串
	if req.Config != nil {
		if str, ok := req.Config.(string); ok {
			currentTask.Config = str
		} else {
			configBytes, _ := json.Marshal(req.Config)
			currentTask.Config = string(configBytes)
		}
	}

	if req.CronExpression != "" || (req.CronExpression == "" && req.Config != nil) { // 允许清空
		currentTask.CronExpression = req.CronExpression
	}
	if req.TimeoutSeconds >= 0 {
		currentTask.TimeoutSeconds = req.TimeoutSeconds
		if currentTask.TimeoutSeconds > 86400 {
			currentTask.TimeoutSeconds = 86400
		}
	}

	if req.RetryCount >= 0 {
		currentTask.RetryCount = req.RetryCount
		if currentTask.RetryCount > 10 {
			currentTask.RetryCount = 10
		}
	} else if req.RetryMax >= 0 {
		currentTask.RetryCount = req.RetryMax
		if currentTask.RetryCount > 10 {
			currentTask.RetryCount = 10
		}
	}

	if req.RetryInterval >= 0 {
		currentTask.RetryInterval = req.RetryInterval
		if currentTask.RetryInterval > 3600 {
			currentTask.RetryInterval = 3600
		}
	} else if req.RetryDelaySeconds >= 0 {
		currentTask.RetryInterval = req.RetryDelaySeconds
		if currentTask.RetryInterval > 3600 {
			currentTask.RetryInterval = 3600
		}
	}
	if currentTask.RetryCount > 0 && currentTask.RetryInterval <= 0 {
		currentTask.RetryInterval = 5
	}

	// 布尔值总是更新（如果提供）
	hasAvailableExecutors := true
	if req.IsEnabled != nil && *req.IsEnabled {
		// 检查是否有可用的执行器
		domainID := currentTask.DomainID
		if req.DomainID > 0 {
			domainID = req.DomainID
		}
		executors, err := h.svc.ListExecutorsByDomain(ctx, domainID)
		if err != nil {
			slog.Warn("TaskHandler.Update: failed to list executors", "error", err)
			hasAvailableExecutors = false
		} else {
			availableExecutors := 0
			for _, exec := range executors {
				if isExecutorOnline(exec) && exec.CurrentLoad < exec.Capacity {
					availableExecutors++
				}
			}
			hasAvailableExecutors = availableExecutors > 0
		}
		if hasAvailableExecutors {
			currentTask.IsEnabled = true
		} else {
			currentTask.IsEnabled = false
		}
	} else if req.IsEnabled != nil {
		currentTask.IsEnabled = *req.IsEnabled
	}

	if req.DomainID > 0 {
		currentTask.DomainID = req.DomainID
	}
	if req.WebhookID != nil {
		currentTask.WebhookID = req.WebhookID
	}
	if req.WebhookEvents != "" {
		currentTask.WebhookEvents = req.WebhookEvents
	}
	// 更新 AssignedExecutorID（允许设置为空字符串来清除）
	currentTask.AssignedExecutorID = req.AssignedExecutorID

	slog.Info("TaskHandler.Update: updating task",
		"id", id,
		"is_enabled", currentTask.IsEnabled,
		"cron_expression", currentTask.CronExpression)

	err = h.svc.UpdateTask(ctx, id, currentTask)
	if err != nil {
		slog.Error("TaskHandler.Update: failed to update task", "id", id, "error", err)
		FailFromError(c, err)
		return
	}

	// 返回更新后的任务
	updatedTask, err := h.svc.GetTaskByID(ctx, id)
	if err != nil {
		slog.Error("TaskHandler.Update: failed to get updated task", "id", id, "error", err)
	} else {
		Success(c, gin.H{
			"task": updatedTask,
			"has_available_executors": hasAvailableExecutors,
		})
		return
	}

	slog.Info("TaskHandler.Update: task updated", "id", id)
	Success(c, gin.H{
		"task": currentTask,
		"has_available_executors": hasAvailableExecutors,
	})
}

func (h *TaskHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("TaskHandler.Delete: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("TaskHandler.Delete: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	ctx := c.Request.Context()
	err = h.svc.DeleteTask(ctx, id)
	if err != nil {
		slog.Error("TaskHandler.Delete: failed to delete task", "id", id, "error", err)
		FailFromError(c, err)
		return
	}

	slog.Info("TaskHandler.Delete: task deleted", "id", id)
	SuccessWithMessage(c, "deleted", nil)
}

func (h *TaskHandler) Trigger(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("TaskHandler.Trigger: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("TaskHandler.Trigger: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	if !h.svc.IsLeader() {
		h.forwardToLeader(c, c.Request.Method, c.Request.URL.Path, c.Request.Body)
		return
	}

	task, getErr := h.svc.GetTaskByID(c.Request.Context(), id)
	if getErr != nil {
		slog.Warn("TaskHandler.Trigger: task not found", "task_id", id, "error", getErr)
		BadRequest(c, "task not found")
		return
	}

	userDomainID, _ := c.Get("current_domain_id")
	userRole, _ := c.Get("role")
	domainID, _ := userDomainID.(int64)
	role, _ := userRole.(string)
	isSystemAdmin := role == "system_admin" || role == "admin"
	if !isSystemAdmin && task.DomainID != domainID {
		slog.Warn("TaskHandler.Trigger: permission denied",
			"task_id", id,
			"task_domain_id", task.DomainID,
			"user_domain_id", domainID,
		)
		Forbidden(c, "permission denied")
		return
	}

	executionID, err := h.svc.TriggerTask(c.Request.Context(), id)
	if err != nil {
		errMsg := err.Error()
		slog.Error("TaskHandler.Trigger: failed to trigger task", "task_id", id, "error", errMsg)

		if strings.Contains(errMsg, "already running") || strings.Contains(errMsg, "already being executed") {
			Fail(c, CodeTaskRunning, "任务正在运行中，请等待当前执行完成")
			return
		}

		domainName := h.svc.GetDomainName(c.Request.Context(), task.DomainID)
		if strings.Contains(errMsg, "no available executor") {
			Fail(c, CodeNoAvailableExecutor, fmt.Sprintf("%s 没有可用的执行器，请联系管理员为 %s 分配执行器", domainName, domainName))
			return
		}

		if strings.Contains(errMsg, "not online") {
			Fail(c, CodeExecutorOffline, "执行器不在线，请检查执行器状态")
			return
		}

		if strings.Contains(errMsg, "no capacity") {
			Fail(c, CodeExecutorNoCapacity, "执行器容量已满，请稍后重试或联系管理员扩容")
			return
		}

		if strings.Contains(errMsg, "dispatch failed") {
			Fail(c, CodeDispatchFailed, "任务分发失败，请检查执行器连接状态")
			return
		}

		Fail(c, CodeInternalError, errMsg)
		return
	}

	slog.Info("TaskHandler.Trigger: task triggered", "task_id", id, "execution_id", executionID)
	Success(c, gin.H{"message": "triggered", "execution_id": executionID})
}

func (h *TaskHandler) forwardToLeader(c *gin.Context, method, path string, body io.Reader) {
	ctx := context.WithValue(c.Request.Context(), "authorization", c.GetHeader("Authorization"))
	ctx = context.WithValue(ctx, "content_type", c.GetHeader("Content-Type"))

	// 保持完整的 URL，包括 query 参数
	fullPath := c.Request.URL.RequestURI()

	respBody, statusCode, err := h.svc.ForwardToLeader(ctx, method, fullPath, body)
	if err != nil {
		slog.Error("TaskHandler: failed to forward request to leader",
			"method", method,
			"path", path,
			"error", err,
		)
		ServiceUnavailable(c, fmt.Sprintf("当前节点非主节点，转发请求到主节点失败: %v", err))
		return
	}

	c.Data(statusCode, "application/json", respBody)
}

func (h *TaskHandler) Executions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("TaskHandler.Executions: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("TaskHandler.Executions: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	ctx := c.Request.Context()
	executions, err := h.svc.GetTaskExecutions(ctx, id)
	if err != nil {
		slog.Error("TaskHandler.Executions: failed to get executions", "task_id", id, "error", err)
		FailFromError(c, err)
		return
	}

	var response []*TaskExecutionResponse
	for _, exec := range executions {
		response = append(response, toTaskExecutionResponse(exec))
	}

	Success(c, response)
}

func (h *TaskHandler) StreamLogs(c *gin.Context) {
	executionID := c.Query("execution_id")
	if safeString(executionID) == "" {
		slog.Warn("TaskHandler.StreamLogs: execution_id required")
		BadRequest(c, "execution_id required")
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
	execCheckCounter := 0

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
							escapeJSON(log.Message), log.LogTime.Format(TimeResponseFormat))
						c.Writer.Write([]byte("data: " + data + "\n\n"))
						c.Writer.Flush()
					}
				}
			}

			execCheckCounter++
			if len(logs) > 0 && execCheckCounter%3 == 0 {
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
	ExecutorID   int64   `json:"executor_id"`
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
		CreatedAt:   exec.CreatedAt.Format(TimeResponseFormat),
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
	ExecutorID  int64  `json:"executor_id,omitempty"`
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
		LogTime:     tl.LogTime.Format(TimeResponseFormat),
	}
}

func (h *TaskHandler) ExecutionLogs(c *gin.Context) {
	executionID := c.Param("execution_id")
	if safeString(executionID) == "" {
		slog.Warn("TaskHandler.ExecutionLogs: executionId required")
		BadRequest(c, "executionId required")
		return
	}

	ctx := c.Request.Context()
	logs, err := h.svc.GetTaskLogs(ctx, executionID)
	if err != nil {
		slog.Error("TaskHandler.ExecutionLogs: failed to get logs", "execution_id", executionID, "error", err)
		FailFromError(c, err)
		return
	}

	var response []*TaskLogResponse
	for _, log := range logs {
		response = append(response, toTaskLogResponse(log))
	}

	Success(c, response)
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}
