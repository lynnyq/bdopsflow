package handler

import (
	"encoding/json"
	"net/http"

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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "webhook configuration created",
		"config": webhook.WebhookConfig{
			URL:     req.URL,
			Method:  req.Method,
			Headers: req.Headers,
			Events:  req.Events,
		},
	})
}

func (h *WebhookHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"webhooks": []interface{}{},
	})
}

func (h *WebhookHandler) Delete(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "webhook deleted",
	})
}

func (h *WebhookHandler) Test(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	err := h.webhookSvc.Send(c.Request.Context(), config, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "test webhook sent successfully",
	})
}

func (h *WebhookHandler) TriggerForTask(c *gin.Context) {
	var req struct {
		TaskID int64  `json:"task_id"`
		Event  string `json:"event"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.schedulerSvc.GetTaskByID(c.Request.Context(), req.TaskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	webhookConfigStr := task.WebhookConfig
	if webhookConfigStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no webhook configured for this task"})
		return
	}

	var config webhook.WebhookConfig
	if err := json.Unmarshal([]byte(webhookConfigStr), &config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook config"})
		return
	}

	payload := webhook.BuildPayload(req.Event, task.ID, "", task.Status, "", "", 0)
	if err := h.webhookSvc.Send(c.Request.Context(), config, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "webhook triggered successfully",
	})
}
