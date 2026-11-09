package handler

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

// NavigationLinkHandler 导航快捷链接 HTTP 处理器
type NavigationLinkHandler struct {
	svc *service.NavigationLinkService
}

// NewNavigationLinkHandler 创建导航链接 handler
func NewNavigationLinkHandler(svc *service.NavigationLinkService) *NavigationLinkHandler {
	return &NavigationLinkHandler{svc: svc}
}

// ListEnabled 列出启用的导航链接（Dashboard 首页用，所有登录用户可访问）
func (h *NavigationLinkHandler) ListEnabled(c *gin.Context) {
	items, err := h.svc.ListEnabled(c.Request.Context())
	if err != nil {
		slog.Error("NavigationLinkHandler.ListEnabled: failed", "error", err)
		FailFromError(c, err)
		return
	}
	if items == nil {
		items = []model.NavigationLink{}
	}
	Success(c, gin.H{"items": items})
}

// ListAll 列出全部导航链接（管理页用）
func (h *NavigationLinkHandler) ListAll(c *gin.Context) {
	items, err := h.svc.ListAll(c.Request.Context())
	if err != nil {
		slog.Error("NavigationLinkHandler.ListAll: failed", "error", err)
		FailFromError(c, err)
		return
	}
	if items == nil {
		items = []model.NavigationLink{}
	}
	Success(c, gin.H{"items": items})
}

// GetByID 获取单个导航链接
func (h *NavigationLinkHandler) GetByID(c *gin.Context) {
	id, err := parseNavID(c)
	if err != nil {
		return
	}

	item, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrNavigationLinkNotFound) {
			NotFound(c, "导航链接不存在")
			return
		}
		slog.Error("NavigationLinkHandler.GetByID: failed", "error", err, "id", id)
		FailFromError(c, err)
		return
	}
	Success(c, item)
}

// Create 新建导航链接。创建后异步触发 favicon 抓取（不阻塞主请求）。
func (h *NavigationLinkHandler) Create(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		URL         string `json:"url" binding:"required"`
		Width       string `json:"width"`
		DisplayMode string `json:"display_mode"`
		HeightPx    int    `json:"height_px"`
		SortOrder   int    `json:"sort_order"`
		Enabled     *int   `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("NavigationLinkHandler.Create: invalid request body", "error", err)
		BadRequest(c, "名称和URL为必填项")
		return
	}

	width := req.Width
	if width == "" {
		width = model.NavigationLinkWidthThird
	}
	displayMode := req.DisplayMode
	if displayMode == "" {
		displayMode = model.NavigationLinkDisplayModeNormal
	}
	enabled := 1
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	d := &model.NavigationLink{
		Name:        req.Name,
		Description: req.Description,
		URL:         req.URL,
		Width:       width,
		DisplayMode: displayMode,
		HeightPx:    req.HeightPx,
		SortOrder:   req.SortOrder,
		Enabled:     enabled,
	}

	created, err := h.svc.Create(c.Request.Context(), d)
	if err != nil {
		slog.Error("NavigationLinkHandler.Create: failed", "error", err)
		FailFromError(c, err)
		return
	}
	slog.Info("navigation link created", "id", created.ID, "name", created.Name)
	SuccessWithMessage(c, "导航链接创建成功，正在后台获取图标", created)
}

// Update 更新导航链接
func (h *NavigationLinkHandler) Update(c *gin.Context) {
	id, err := parseNavID(c)
	if err != nil {
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		URL         string `json:"url" binding:"required"`
		Width       string `json:"width" binding:"required"`
		DisplayMode string `json:"display_mode" binding:"required"`
		HeightPx    int    `json:"height_px"`
		SortOrder   int    `json:"sort_order"`
		Enabled     *int   `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("NavigationLinkHandler.Update: invalid request body", "error", err)
		BadRequest(c, "名称、URL、宽度、密度为必填项")
		return
	}

	enabled := 1
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	d := &model.NavigationLink{
		Name:        req.Name,
		Description: req.Description,
		URL:         req.URL,
		Width:       req.Width,
		DisplayMode: req.DisplayMode,
		HeightPx:    req.HeightPx,
		SortOrder:   req.SortOrder,
		Enabled:     enabled,
	}

	if err := h.svc.Update(c.Request.Context(), id, d); err != nil {
		if errors.Is(err, service.ErrNavigationLinkNotFound) {
			NotFound(c, "导航链接不存在")
			return
		}
		slog.Error("NavigationLinkHandler.Update: failed", "error", err, "id", id)
		FailFromError(c, err)
		return
	}
	SuccessWithMessage(c, "导航链接更新成功", nil)
}

// Delete 删除导航链接
func (h *NavigationLinkHandler) Delete(c *gin.Context) {
	id, err := parseNavID(c)
	if err != nil {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrNavigationLinkNotFound) {
			NotFound(c, "导航链接不存在")
			return
		}
		slog.Error("NavigationLinkHandler.Delete: failed", "error", err, "id", id)
		FailFromError(c, err)
		return
	}
	SuccessWithMessage(c, "导航链接删除成功", nil)
}

// Reorder 批量更新排序
func (h *NavigationLinkHandler) Reorder(c *gin.Context) {
	var req struct {
		Items []model.NavigationReorderItem `json:"items" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("NavigationLinkHandler.Reorder: invalid request body", "error", err)
		BadRequest(c, "items 为必填项")
		return
	}

	if err := h.svc.Reorder(c.Request.Context(), req.Items); err != nil {
		slog.Error("NavigationLinkHandler.Reorder: failed", "error", err)
		FailFromError(c, err)
		return
	}
	SuccessWithMessage(c, "排序更新成功", nil)
}

// RefreshIcon 手动触发重新抓取指定导航的 favicon
func (h *NavigationLinkHandler) RefreshIcon(c *gin.Context) {
	id, err := parseNavID(c)
	if err != nil {
		return
	}

	d, err := h.svc.RefreshIcon(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrNavigationLinkNotFound) {
			NotFound(c, "导航链接不存在")
			return
		}
		slog.Error("NavigationLinkHandler.RefreshIcon: failed", "error", err, "id", id)
		FailFromError(c, err)
		return
	}
	SuccessWithMessage(c, "图标刷新成功", d)
}

// parseNavID 从 path param 解析导航链接 ID
func parseNavID(c *gin.Context) (int64, error) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		BadRequest(c, "无效的导航链接 ID")
		return 0, err
	}
	return id, nil
}
