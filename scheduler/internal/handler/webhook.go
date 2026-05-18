package handler

import (
	"encoding/json"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
	"github.com/lynnyq/bdopsflow/scheduler/internal/webhook"
)

type WebhookHandler struct {
	schedulerSvc *service.SchedulerService
	webhookSvc   *webhook.Service
}

func NewWebhookHandler(schedulerSvc *service.SchedulerService, webhookSvc *webhook.Service) *WebhookHandler {
	return &WebhookHandler{
		schedulerSvc: schedulerSvc,
		webhookSvc:   webhookSvc,
	}
}

func (h *WebhookHandler) Create(c *gin.Context) {
	var req struct {
		URL     string            `json:"url"`
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
		Events  []string          `json:"events"`
		Name    string            `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("WebhookHandler.Create: invalid request body", "error", err)
		BadRequest(c, err.Error())
		return
	}

	if safeString(req.URL) == "" {
		slog.Warn("WebhookHandler.Create: url is required")
		BadRequest(c, "url is required")
		return
	}

	SuccessWithMessage(c, "webhook configuration created", webhook.WebhookConfig{
		URL:     req.URL,
		Method:  req.Method,
		Headers: req.Headers,
		Events:  req.Events,
	})
}

func (h *WebhookHandler) List(c *gin.Context) {
	Success(c, gin.H{
		"webhooks": []interface{}{},
	})
}

func (h *WebhookHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("WebhookHandler.Delete: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("WebhookHandler.Delete: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	SuccessWithMessage(c, "webhook deleted", nil)
}

func (h *WebhookHandler) Test(c *gin.Context) {
	idStr := c.Param("id")
	_, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("WebhookHandler.Test: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("WebhookHandler.Test: invalid request body", "error", err)
		BadRequest(c, err.Error())
		return
	}

	config := webhook.WebhookConfig{
		URL:    req.URL,
		Method: "POST",
		Events: []string{"*"},
	}

	payload := webhook.WebhookPayload{
		Event:     "test",
		Timestamp: 0,
		TaskID:    0,
		Status:    "test",
		Output:    "This is a test webhook",
	}

	err = h.webhookSvc.Send(c.Request.Context(), config, payload)
	if err != nil {
		slog.Error("WebhookHandler.Test: failed to send webhook", "error", err)
		BadRequest(c, err.Error())
		return
	}

	SuccessWithMessage(c, "test webhook sent successfully", nil)
}

func (h *WebhookHandler) TriggerForTask(c *gin.Context) {
	var req struct {
		TaskID int64  `json:"task_id" binding:"required,min=1"`
		Event  string `json:"event"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "task_id为必填项，且必须为正整数")
		return
	}

	if req.TaskID <= 0 {
		BadRequest(c, "task_id必须为正整数")
		return
	}

	task, err := h.schedulerSvc.GetTaskByID(c.Request.Context(), req.TaskID)
	if err != nil {
		NotFound(c, "task not found")
		return
	}

	webhookConfigStr := task.WebhookConfig
	if webhookConfigStr == "" {
		BadRequest(c, "no webhook configured for this task")
		return
	}

	var config webhook.WebhookConfig
	if err := json.Unmarshal([]byte(webhookConfigStr), &config); err != nil {
		BadRequest(c, "invalid webhook config")
		return
	}

	payload := webhook.BuildPayload(req.Event, task.ID, "", task.Status, "", "", 0)
	if err := h.webhookSvc.Send(c.Request.Context(), config, payload); err != nil {
		InternalServerError(c, err.Error())
		return
	}

	SuccessWithMessage(c, "webhook triggered successfully", nil)
}
