package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

// APITokenHandler API Token 处理器
type APITokenHandler struct {
	apiTokenSvc *service.APITokenService
	auditSvc    *service.AuditLogService
}

// NewAPITokenHandler 创建 API Token 处理器
func NewAPITokenHandler(apiTokenSvc *service.APITokenService, auditSvc *service.AuditLogService) *APITokenHandler {
	return &APITokenHandler{
		apiTokenSvc: apiTokenSvc,
		auditSvc:    auditSvc,
	}
}

// contextToString 从 context 中获取字符串值
func contextToString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Generate 生成 API Token
func (h *APITokenHandler) Generate(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "未授权，请重新登录")
		return
	}

	uid, ok := userID.(int64)
	if !ok || uid <= 0 {
		BadRequest(c, "无效的用户ID")
		return
	}

	slog.Debug("APITokenHandler.Generate: entering", "module", "handler_api_token", "user_id", uid)

	plaintext, tokenInfo, err := h.apiTokenSvc.GenerateToken(c.Request.Context(), uid)
	if err != nil {
		slog.Error("APITokenHandler.Generate: failed to generate token", "module", "handler_api_token", "user_id", uid, "error", err)
		InternalServerError(c, "生成 API Token 失败")
		return
	}

	username, _ := c.Get("username")
	if uname, ok := username.(string); ok {
		c.Set("audit_resource_name", uname)
	}
	c.Set("audit_detail", "token_prefix="+tokenInfo.TokenPrefix)

	slog.Info("APITokenHandler.Generate: token generated", "module", "handler_api_token", "user_id", uid)

	Success(c, gin.H{
		"token":        plaintext,
		"token_prefix": tokenInfo.TokenPrefix,
		"created_at":   tokenInfo.CreatedAt.Format(TimeResponseFormat),
	})
}

// GetInfo 获取 API Token 信息
func (h *APITokenHandler) GetInfo(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "未授权，请重新登录")
		return
	}

	uid, ok := userID.(int64)
	if !ok || uid <= 0 {
		BadRequest(c, "无效的用户ID")
		return
	}

	slog.Debug("APITokenHandler.GetInfo: entering", "module", "handler_api_token", "user_id", uid)

	tokenInfo, err := h.apiTokenSvc.GetTokenInfo(c.Request.Context(), uid)
	if err != nil {
		if err == service.ErrAPITokenNotFound {
			Success(c, gin.H{
				"has_token": false,
			})
			return
		}
		slog.Error("APITokenHandler.GetInfo: failed to get token info", "module", "handler_api_token", "user_id", uid, "error", err)
		InternalServerError(c, "查询 API Token 信息失败")
		return
	}

	result := gin.H{
		"has_token":    true,
		"token_prefix": tokenInfo.TokenPrefix,
		"created_at":   tokenInfo.CreatedAt.Format(TimeResponseFormat),
	}
	if tokenInfo.LastUsedAt != nil {
		result["last_used_at"] = tokenInfo.LastUsedAt.Format(TimeResponseFormat)
	}

	Success(c, result)
}

// Reveal 查看 API Token 明文
func (h *APITokenHandler) Reveal(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "未授权，请重新登录")
		return
	}

	uid, ok := userID.(int64)
	if !ok || uid <= 0 {
		BadRequest(c, "无效的用户ID")
		return
	}

	slog.Debug("APITokenHandler.Reveal: entering", "module", "handler_api_token", "user_id", uid)

	// 先获取 token 信息用于审计日志
	tokenInfo, infoErr := h.apiTokenSvc.GetTokenInfo(c.Request.Context(), uid)
	if infoErr != nil {
		if infoErr == service.ErrAPITokenNotFound {
			NotFound(c, "API Token 不存在")
			return
		}
		slog.Error("APITokenHandler.Reveal: failed to get token info", "module", "handler_api_token", "user_id", uid, "error", infoErr)
		InternalServerError(c, "查看 API Token 失败")
		return
	}

	plaintext, err := h.apiTokenSvc.RevealToken(c.Request.Context(), uid)
	if err != nil {
		slog.Error("APITokenHandler.Reveal: failed to reveal token", "module", "handler_api_token", "user_id", uid, "error", err)
		InternalServerError(c, "查看 API Token 失败")
		return
	}

	// 写审计日志
	username, _ := c.Get("username")
	realName, _ := c.Get("real_name")
	role, _ := c.Get("role")
	domainID, _ := c.Get("current_domain_id")

	var auditDomainID *int64
	if did, ok := domainID.(int64); ok && did > 0 {
		auditDomainID = &did
	}

	auditLog := &model.AuditLog{
		UserID:        &uid,
		Username:      contextToString(username),
		RealName:      contextToString(realName),
		Role:          contextToString(role),
		DomainID:      auditDomainID,
		Action:        "reveal",
		Resource:      "api_token",
		ResourceName:  contextToString(username),
		Status:        "success",
		IPAddress:     c.ClientIP(),
		UserAgent:     truncateString(c.Request.UserAgent(), 500),
		RequestMethod: c.Request.Method,
		RequestPath:   c.Request.URL.Path,
		Detail:        "token_prefix=" + tokenInfo.TokenPrefix,
		CreatedAt:     time.Now(),
	}

	go func() {
		if err := h.auditSvc.Create(context.Background(), auditLog); err != nil {
			slog.Error("APITokenHandler.Reveal: failed to write audit log", "module", "handler_api_token", "error", err)
		}
	}()

	slog.Info("APITokenHandler.Reveal: token revealed", "module", "handler_api_token", "user_id", uid)

	Success(c, gin.H{
		"token": plaintext,
	})
}

// Revoke 撤销 API Token
func (h *APITokenHandler) Revoke(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "未授权，请重新登录")
		return
	}

	uid, ok := userID.(int64)
	if !ok || uid <= 0 {
		BadRequest(c, "无效的用户ID")
		return
	}

	slog.Debug("APITokenHandler.Revoke: entering", "module", "handler_api_token", "user_id", uid)

	// 先获取 token 信息用于审计日志
	tokenInfo, infoErr := h.apiTokenSvc.GetTokenInfo(c.Request.Context(), uid)
	if infoErr != nil {
		if infoErr == service.ErrAPITokenNotFound {
			NotFound(c, "API Token 不存在")
			return
		}
		slog.Error("APITokenHandler.Revoke: failed to get token info before revoke", "module", "handler_api_token", "user_id", uid, "error", infoErr)
		InternalServerError(c, "查询 API Token 信息失败")
		return
	}

	if err := h.apiTokenSvc.RevokeToken(c.Request.Context(), uid); err != nil {
		slog.Error("APITokenHandler.Revoke: failed to revoke token", "module", "handler_api_token", "user_id", uid, "error", err)
		InternalServerError(c, "撤销 API Token 失败")
		return
	}

	c.Set("audit_detail", "token_prefix="+tokenInfo.TokenPrefix)

	slog.Info("APITokenHandler.Revoke: token revoked", "module", "handler_api_token", "user_id", uid)

	Success(c, gin.H{})
}
