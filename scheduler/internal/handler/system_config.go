package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
)

type SystemConfigHandler struct {
	configService *sysconfig.Service
}

func NewSystemConfigHandler(configService *sysconfig.Service) *SystemConfigHandler {
	return &SystemConfigHandler{
		configService: configService,
	}
}

func (h *SystemConfigHandler) List(c *gin.Context) {
	slog.Debug("SystemConfigHandler.List: entering", "module", "handler_system_config")
	configs := h.configService.GetAllWithMeta()
	Success(c, configs)
}

func (h *SystemConfigHandler) Update(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		BadRequest(c, "config key is required")
		return
	}

	slog.Debug("SystemConfigHandler.Update: entering", "module", "handler_system_config", "key", key)

	var req struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	var configUID int64
	if v, ok := userID.(int64); ok {
		configUID = v
	}
	if err := h.configService.Set(c.Request.Context(), key, req.Value, configUID); err != nil {
		slog.Error("SystemConfigHandler.Update: failed to update config", "module", "handler_system_config", "key", key, "error", err)
		Fail(c, CodeBadRequest, err.Error())
		return
	}

	slog.Info("SystemConfigHandler.Update: config updated successfully", "module", "handler_system_config", "key", key, "user_id", userID)
	Success(c, nil)
}
