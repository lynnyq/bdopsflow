package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource"
)

type SystemConfigHandler struct {
	configService *datasource.ConfigService
}

func NewSystemConfigHandler(configService *datasource.ConfigService) *SystemConfigHandler {
	return &SystemConfigHandler{
		configService: configService,
	}
}

func (h *SystemConfigHandler) List(c *gin.Context) {
	configs := h.configService.GetAllWithMeta()
	Success(c, configs)
}

func (h *SystemConfigHandler) Update(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		BadRequest(c, "config key is required")
		return
	}

	var req struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	if err := h.configService.Set(c.Request.Context(), key, req.Value, userID.(int64)); err != nil {
		Fail(c, 400, err.Error())
		return
	}

	Success(c, nil)
}
