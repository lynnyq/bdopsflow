package handler

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

// GrafanaDashboardHandler Grafana 图表嵌入配置 HTTP 处理器
type GrafanaDashboardHandler struct {
	svc *service.GrafanaDashboardService
}

// NewGrafanaDashboardHandler 创建 Grafana 图表 handler
func NewGrafanaDashboardHandler(svc *service.GrafanaDashboardService) *GrafanaDashboardHandler {
	return &GrafanaDashboardHandler{svc: svc}
}

// ListEnabled 列出启用的图表（Dashboard 嵌入用，所有登录用户可访问）
func (h *GrafanaDashboardHandler) ListEnabled(c *gin.Context) {
	items, err := h.svc.ListEnabled(c.Request.Context())
	if err != nil {
		slog.Error("GrafanaDashboardHandler.ListEnabled: failed", "error", err)
		FailFromError(c, err)
		return
	}
	if items == nil {
		items = []model.GrafanaDashboard{}
	}
	Success(c, gin.H{"items": items})
}

// ListAll 列出全部图表（管理页用）
func (h *GrafanaDashboardHandler) ListAll(c *gin.Context) {
	items, err := h.svc.ListAll(c.Request.Context())
	if err != nil {
		slog.Error("GrafanaDashboardHandler.ListAll: failed", "error", err)
		FailFromError(c, err)
		return
	}
	if items == nil {
		items = []model.GrafanaDashboard{}
	}
	Success(c, gin.H{"items": items})
}

// GetByID 获取单个图表
func (h *GrafanaDashboardHandler) GetByID(c *gin.Context) {
	id, err := parseGrafanaID(c)
	if err != nil {
		return
	}

	item, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrGrafanaDashboardNotFound) {
			NotFound(c, "图表不存在")
			return
		}
		slog.Error("GrafanaDashboardHandler.GetByID: failed", "error", err, "id", id)
		FailFromError(c, err)
		return
	}
	Success(c, item)
}

// Create 新建图表
func (h *GrafanaDashboardHandler) Create(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		EmbedURL    string `json:"embed_url" binding:"required"`
		Width       string `json:"width"`
		HeightPx    int    `json:"height_px"`
		SortOrder   int    `json:"sort_order"`
		Enabled     *int   `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("GrafanaDashboardHandler.Create: invalid request body", "error", err)
		BadRequest(c, "名称和嵌入URL为必填项")
		return
	}

	// 设置默认值
	width := req.Width
	if width == "" {
		width = "full"
	}
	heightPx := req.HeightPx
	if heightPx == 0 {
		heightPx = 300
	}
	enabled := 1
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	d := &model.GrafanaDashboard{
		Name:        req.Name,
		Description: req.Description,
		EmbedURL:    req.EmbedURL,
		Width:       width,
		HeightPx:    heightPx,
		SortOrder:   req.SortOrder,
		Enabled:     enabled,
	}

	created, err := h.svc.Create(c.Request.Context(), d)
	if err != nil {
		slog.Error("GrafanaDashboardHandler.Create: failed", "error", err)
		FailFromError(c, err)
		return
	}
	slog.Info("grafana dashboard created", "id", created.ID, "name", created.Name)
	SuccessWithMessage(c, "图表创建成功", created)
}

// Update 更新图表
func (h *GrafanaDashboardHandler) Update(c *gin.Context) {
	id, err := parseGrafanaID(c)
	if err != nil {
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		EmbedURL    string `json:"embed_url" binding:"required"`
		Width       string `json:"width" binding:"required"`
		HeightPx    int    `json:"height_px" binding:"required"`
		SortOrder   int    `json:"sort_order"`
		Enabled     *int   `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("GrafanaDashboardHandler.Update: invalid request body", "error", err)
		BadRequest(c, "名称、嵌入URL、宽度、高度为必填项")
		return
	}

	enabled := 1
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	d := &model.GrafanaDashboard{
		Name:        req.Name,
		Description: req.Description,
		EmbedURL:    req.EmbedURL,
		Width:       req.Width,
		HeightPx:    req.HeightPx,
		SortOrder:   req.SortOrder,
		Enabled:     enabled,
	}

	if err := h.svc.Update(c.Request.Context(), id, d); err != nil {
		if errors.Is(err, service.ErrGrafanaDashboardNotFound) {
			NotFound(c, "图表不存在")
			return
		}
		slog.Error("GrafanaDashboardHandler.Update: failed", "error", err, "id", id)
		FailFromError(c, err)
		return
	}
	SuccessWithMessage(c, "图表更新成功", nil)
}

// Delete 删除图表
func (h *GrafanaDashboardHandler) Delete(c *gin.Context) {
	id, err := parseGrafanaID(c)
	if err != nil {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrGrafanaDashboardNotFound) {
			NotFound(c, "图表不存在")
			return
		}
		slog.Error("GrafanaDashboardHandler.Delete: failed", "error", err, "id", id)
		FailFromError(c, err)
		return
	}
	SuccessWithMessage(c, "图表删除成功", nil)
}

// Reorder 批量更新排序
func (h *GrafanaDashboardHandler) Reorder(c *gin.Context) {
	var req struct {
		Items []model.GrafanaReorderItem `json:"items" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("GrafanaDashboardHandler.Reorder: invalid request body", "error", err)
		BadRequest(c, "items 为必填项")
		return
	}

	if err := h.svc.Reorder(c.Request.Context(), req.Items); err != nil {
		slog.Error("GrafanaDashboardHandler.Reorder: failed", "error", err)
		FailFromError(c, err)
		return
	}
	SuccessWithMessage(c, "排序更新成功", nil)
}

// parseGrafanaID 从 path param 解析 id
func parseGrafanaID(c *gin.Context) (int64, error) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		BadRequest(c, "无效的图表 ID")
		return 0, err
	}
	return id, nil
}
