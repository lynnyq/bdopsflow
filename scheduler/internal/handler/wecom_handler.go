package handler

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/wecom"
	sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
)

type WeComHandler struct {
	configService *sysconfig.Service
	wecomService  *wecom.WeComService
}

func NewWeComHandler(configService *sysconfig.Service) *WeComHandler {
	return &WeComHandler{
		configService: configService,
		wecomService:  wecom.NewService(configService),
	}
}

type BdopsFlowExecution struct {
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error"`
	ID         string `json:"id"`
	Output     string `json:"output"`
	Status     string `json:"status"`
}

type BdopsFlowTask struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type BdopsFlowEvent struct {
	DeliveryID string             `json:"delivery_id"`
	Event      string             `json:"event"`
	Execution  BdopsFlowExecution `json:"execution"`
	Task       BdopsFlowTask      `json:"task"`
	Timestamp  int64              `json:"timestamp"`
}

type SendAppMsgRequest struct {
	AgentID      int      `json:"agent_id"`
	MsgType      string   `json:"msg_type"`
	MsgContent   string   `json:"msg_content"`
	PhoneNumList []string `json:"phone_num_list"`
}

type SendImageMsgRequest struct {
	GroupID       string `json:"group_id"`
	ImageBase64   string `json:"image_base64"`
}

type SendTextPeopleMsgRequest struct {
	GroupID      string `json:"group_id"`
	Msg          string `json:"msg"`
	PhoneNumber  string `json:"phone_number"`
}

type SendChatMsgRequest struct {
	ChatID string `json:"chat_id"`
	Msg    string `json:"msg"`
}

type CreateChatGroupRequest struct {
	ChatName string   `json:"chat_name"`
	UserList []string `json:"user_list"`
}

type GetChatGroupInfoRequest struct {
	ChatID string `json:"chat_id"`
}

type UpdateChatGroupRequest struct {
	ChatID        string   `json:"chat_id"`
	OwnerID       string   `json:"owner_id"`
	AddUserList   []string `json:"add_user_list"`
	DelUserList   []string `json:"del_user_list"`
	ChatName      string   `json:"chat_name"`
}

func (h *WeComHandler) SendWeComMessage(c *gin.Context) {
	wxGroupID := c.Param("wx_group_id")
	if wxGroupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "wx_group_id is required",
		})
		return
	}

	var eventData BdopsFlowEvent
	if err := c.ShouldBindJSON(&eventData); err != nil {
		slog.Error("invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request body",
		})
		return
	}

	msg := h.buildMarkdownMessage(eventData)
	slog.Info("sending wecom message", "wx_group_id", wxGroupID, "msg_preview", truncateString(msg, 100))

	err := h.wecomService.SendRobotMarkdownMsg(wxGroupID, msg)
	if err != nil {
		slog.Error("failed to send wecom message", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": fmt.Sprintf("failed to send message: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "message sent successfully",
	})
}

func (h *WeComHandler) SendAppMsg(c *gin.Context) {
	var req SendAppMsgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request body",
		})
		return
	}

	err := h.wecomService.SendAppMsg(req.AgentID, req.MsgType, req.MsgContent, req.PhoneNumList)
	if err != nil {
		slog.Error("failed to send app message", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": fmt.Sprintf("failed to send app message: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "app message sent successfully",
	})
}

func (h *WeComHandler) SendRobotImageMsg(c *gin.Context) {
	var req SendImageMsgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request body",
		})
		return
	}

	imageBytes, err := base64.StdEncoding.DecodeString(req.ImageBase64)
	if err != nil {
		slog.Error("invalid base64 image data", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid base64 image data",
		})
		return
	}

	err = h.wecomService.SendRobotImageMsg(req.GroupID, imageBytes)
	if err != nil {
		slog.Error("failed to send image message", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": fmt.Sprintf("failed to send image message: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "image message sent successfully",
	})
}

func (h *WeComHandler) SendRobotTextPeopleMsg(c *gin.Context) {
	var req SendTextPeopleMsgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request body",
		})
		return
	}

	err := h.wecomService.SendRobotTextPeopleMsg(req.GroupID, req.Msg, req.PhoneNumber)
	if err != nil {
		slog.Error("failed to send text people message", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": fmt.Sprintf("failed to send text people message: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "text people message sent successfully",
	})
}

type SendRobotMarkdownMsgRequest struct {
	GroupID string `json:"group_id"`
	Msg     string `json:"msg"`
}

func (h *WeComHandler) SendRobotMarkdownMsg(c *gin.Context) {
	var req SendRobotMarkdownMsgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request body",
		})
		return
	}

	err := h.wecomService.SendRobotMarkdownMsg(req.GroupID, req.Msg)
	if err != nil {
		slog.Error("failed to send robot markdown message", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": fmt.Sprintf("failed to send robot markdown message: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "robot markdown message sent successfully",
	})
}

func (h *WeComHandler) SendChatMarkdownMsg(c *gin.Context) {
	var req SendChatMsgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request body",
		})
		return
	}

	err := h.wecomService.SendChatMarkdownMsg(req.ChatID, req.Msg)
	if err != nil {
		slog.Error("failed to send chat markdown message", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": fmt.Sprintf("failed to send chat markdown message: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "chat markdown message sent successfully",
	})
}

func (h *WeComHandler) CreateChatGroup(c *gin.Context) {
	var req CreateChatGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request body",
		})
		return
	}

	result, err := h.wecomService.CreateChatGroup(req.ChatName, req.UserList)
	if err != nil {
		slog.Error("failed to create chat group", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": fmt.Sprintf("failed to create chat group: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "chat group created successfully",
		"data":    result,
	})
}

func (h *WeComHandler) GetChatGroupInfo(c *gin.Context) {
	chatID := c.Param("chat_id")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "chat_id is required",
		})
		return
	}

	result, err := h.wecomService.GetChatGroupInfo(chatID)
	if err != nil {
		slog.Error("failed to get chat group info", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": fmt.Sprintf("failed to get chat group info: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    result,
	})
}

func (h *WeComHandler) UpdateChatGroup(c *gin.Context) {
	var req UpdateChatGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request body",
		})
		return
	}

	if req.ChatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "chat_id is required",
		})
		return
	}

	result, err := h.wecomService.UpdateChatGroup(req.ChatID, req.OwnerID, req.AddUserList, req.DelUserList, req.ChatName)
	if err != nil {
		slog.Error("failed to update chat group", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": fmt.Sprintf("failed to update chat group: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "chat group updated successfully",
		"data":    result,
	})
}

func (h *WeComHandler) buildMarkdownMessage(eventData BdopsFlowEvent) string {
	durationSec := float64(eventData.Execution.DurationMs) / 1000
	output := truncateString(eventData.Execution.Output, 1000)
	errorMsg := truncateString(eventData.Execution.Error, 1000)

	var statusDisplay string
	var statusIcon string
	switch eventData.Execution.Status {
	case "failed":
		statusIcon = "❌"
		statusDisplay = fmt.Sprintf(`<font color="warning">%s</font>`, eventData.Execution.Status)
	case "success":
		statusIcon = "✅"
		statusDisplay = fmt.Sprintf(`<font color="info">%s</font>`, eventData.Execution.Status)
	default:
		statusIcon = "⚙️"
		statusDisplay = eventData.Execution.Status
	}

	var logsDisplay string
	if eventData.Execution.Status == "failed" && errorMsg != "" {
		logsDisplay = "无"
	} else if output != "" {
		logsDisplay = fmt.Sprintf(`<font color="info">%s</font>`, output)
	} else {
		logsDisplay = "无"
	}

	var errorDisplay string
	if eventData.Execution.Status == "failed" && errorMsg != "" {
		errorDisplay = fmt.Sprintf(`<font color="warning">%s</font>`, errorMsg)
	} else {
		errorDisplay = "无"
	}

	return fmt.Sprintf(`### %s BDopsFlow 任务执行通知

> **🔧 任务信息**
> **📝 任务ID:** %d
> **📌 任务名称:** %s

> **📊 执行结果**
> **状态:** %s %s
> **⏱️ 执行耗时:** %.2f 秒

> **📋 执行日志**
> %s

> **⚠️ 错误日志**
> %s`,
		statusIcon,
		eventData.Task.ID,
		eventData.Task.Name,
		statusDisplay,
		statusIcon,
		durationSec,
		logsDisplay,
		errorDisplay,
	)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
