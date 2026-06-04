package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource"
)

type WeComHandler struct {
	configService *datasource.ConfigService
}

func NewWeComHandler(configService *datasource.ConfigService) *WeComHandler {
	return &WeComHandler{
		configService: configService,
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
	DeliveryID string              `json:"delivery_id"`
	Event      string              `json:"event"`
	Execution  BdopsFlowExecution  `json:"execution"`
	Task       BdopsFlowTask       `json:"task"`
	Timestamp  int64              `json:"timestamp"`
}

type WeComRobotResponse struct {
	RetCode string `json:"retCode"`
	RetMsg  string `json:"retMsg"`
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

	err := h.sendRobotMarkdownMsg(wxGroupID, msg)
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

func (h *WeComHandler) buildMarkdownMessage(eventData BdopsFlowEvent) string {
	durationSec := float64(eventData.Execution.DurationMs) / 1000
	// 执行日志和错误日志超过1000个字符时截取前1000个字符再加...
	output := truncateString(eventData.Execution.Output, 1000)
	errorMsg := truncateString(eventData.Execution.Error, 1000)

	// 根据状态添加颜色标签和表情
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

	// 根据状态添加日志显示
	var logsDisplay string
	if eventData.Execution.Status == "failed" && errorMsg != "" {
		logsDisplay = "无"
	} else if output != "" {
		logsDisplay = output
	} else {
		logsDisplay = "无"
	}

	// 根据状态添加错误日志显示
	var errorDisplay string
	if eventData.Execution.Status == "failed" && errorMsg != "" {
		errorDisplay = errorMsg
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

func (h *WeComHandler) sendRobotMarkdownMsg(groupID string, msg string) error {
	robotURL := h.configService.Get("wecom.robot_url")
	if robotURL == "" {
		return fmt.Errorf("wecom robot url is not configured")
	}

	data := map[string]interface{}{
		"groupId":      groupID,
		"fromChannel":  "HDP",
		"reqData": map[string]interface{}{
			"msgtype": "markdown",
			"ewechatMsg": map[string]string{
				"content": msg,
			},
		},
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	resp, err := http.Post(robotURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var result WeComRobotResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.RetCode != "0000" || result.RetMsg != "success" {
		return fmt.Errorf("message send failed: retCode=%s, retMsg=%s", result.RetCode, result.RetMsg)
	}

	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
