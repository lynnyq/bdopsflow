package handler

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type WebhookHandler struct {
	webhookSvc *service.WebhookService
}

func NewWebhookHandler(webhookSvc *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhookSvc: webhookSvc}
}

func (h *WebhookHandler) Create(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		URL         string `json:"url" binding:"required"`
		Method      string `json:"method"`
		Headers     string `json:"headers"`
		Secret      string `json:"secret"`
		DomainID    int64  `json:"domain_id" binding:"required"`
		IsEnabled   *bool  `json:"is_enabled"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("WebhookHandler.Create: invalid request body", "error", err)
		BadRequest(c, "名称和URL为必填项")
		return
	}

	if req.Method == "" {
		req.Method = "POST"
	}
	if req.Headers == "" {
		req.Headers = "{}"
	}

	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	userID, _ := c.Get("user_id")
	var createdBy *int64
	if uid, ok := userID.(int64); ok {
		createdBy = &uid
	}

	webhook := &model.Webhook{
		Name:        req.Name,
		URL:         req.URL,
		Method:      req.Method,
		Headers:     req.Headers,
		Secret:      req.Secret,
		DomainID:    req.DomainID,
		IsEnabled:   isEnabled,
		Description: req.Description,
		CreatedBy:   createdBy,
	}

	created, err := h.webhookSvc.Create(c.Request.Context(), webhook)
	if err != nil {
		slog.Error("WebhookHandler.Create: failed to create webhook", "error", err)
		InternalServerError(c, "创建Webhook失败")
		return
	}

	SuccessWithMessage(c, "webhook created", created)
}

func (h *WebhookHandler) List(c *gin.Context) {
	domainIDStr := c.Query("domain_id")
	if domainIDStr == "" {
		BadRequest(c, "domain_id为必填项")
		return
	}

	domainID, err := strconv.ParseInt(domainIDStr, 10, 64)
	if err != nil {
		BadRequest(c, "domain_id格式错误")
		return
	}

	webhooks, err := h.webhookSvc.List(c.Request.Context(), domainID)
	if err != nil {
		slog.Error("WebhookHandler.List: failed to list webhooks", "error", err, "domain_id", domainID)
		InternalServerError(c, "查询Webhook列表失败")
		return
	}

	if webhooks == nil {
		webhooks = []model.Webhook{}
	}

	Success(c, gin.H{
		"items": webhooks,
	})
}

func (h *WebhookHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		BadRequest(c, "无效的Webhook ID")
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		URL         string `json:"url" binding:"required"`
		Method      string `json:"method"`
		Headers     string `json:"headers"`
		Secret      string `json:"secret"`
		DomainID    int64  `json:"domain_id" binding:"required"`
		IsEnabled   *bool  `json:"is_enabled"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("WebhookHandler.Update: invalid request body", "error", err)
		BadRequest(c, "名称和URL为必填项")
		return
	}

	if req.Method == "" {
		req.Method = "POST"
	}
	if req.Headers == "" {
		req.Headers = "{}"
	}

	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	webhook := &model.Webhook{
		Name:        req.Name,
		URL:         req.URL,
		Method:      req.Method,
		Headers:     req.Headers,
		Secret:      req.Secret,
		DomainID:    req.DomainID,
		IsEnabled:   isEnabled,
		Description: req.Description,
	}

	if err := h.webhookSvc.Update(c.Request.Context(), id, webhook); err != nil {
		slog.Error("WebhookHandler.Update: failed to update webhook", "error", err, "id", id)
		if err.Error() == "webhook not found" {
			NotFound(c, "Webhook不存在")
			return
		}
		InternalServerError(c, "更新Webhook失败")
		return
	}

	SuccessWithMessage(c, "webhook updated", nil)
}

func (h *WebhookHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		BadRequest(c, "无效的Webhook ID")
		return
	}

	if err := h.webhookSvc.Delete(c.Request.Context(), id); err != nil {
		slog.Error("WebhookHandler.Delete: failed to delete webhook", "error", err, "id", id)
		if err.Error() == "webhook not found" {
			NotFound(c, "Webhook不存在")
			return
		}
		InternalServerError(c, "删除Webhook失败")
		return
	}

	SuccessWithMessage(c, "webhook deleted", nil)
}

func (h *WebhookHandler) Test(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		BadRequest(c, "无效的Webhook ID")
		return
	}

	result, err := h.webhookSvc.Test(c.Request.Context(), id)
	if err != nil {
		slog.Error("WebhookHandler.Test: failed to test webhook", "error", err, "id", id)
		if err.Error() == "webhook not found" {
			NotFound(c, "Webhook不存在")
			return
		}
		Fail(c, 500, fmt.Sprintf("测试Webhook失败: %s", err.Error()))
		return
	}

	if result.Error != "" {
		FailWithData(c, 500, "Webhook测试失败", result)
		return
	}

	SuccessWithMessage(c, "test webhook sent successfully", result)
}
