package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

// CertificateHandler handles certificate management endpoints.
type CertificateHandler struct {
	certSvc *service.CertificateService
	permSvc *service.PermissionService
}

// NewCertificateHandler creates a new CertificateHandler.
func NewCertificateHandler(certSvc *service.CertificateService, permSvc *service.PermissionService) *CertificateHandler {
	return &CertificateHandler{
		certSvc: certSvc,
		permSvc: permSvc,
	}
}

// List returns certificates with pagination.
// System admin can see all certificates; other users can only see their own.
func (h *CertificateHandler) List(c *gin.Context) {
	userID := extractUserID(c)
	if userID <= 0 {
		Unauthorized(c, "未授权访问")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	isAdmin, _ := h.permSvc.IsSystemAdmin(c.Request.Context(), userID)

	search := c.Query("search")
	summaries, total, err := h.certSvc.ListByUser(c.Request.Context(), userID, isAdmin, page, pageSize, search)
	if err != nil {
		slog.Error("failed to list certificates", "user_id", userID, "error", err)
		Fail(c, CodeQueryError, "获取证书列表失败")
		return
	}

	if summaries == nil {
		summaries = []*model.CertificateSummary{}
	}

	Success(c, gin.H{
		"items":     summaries,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Create uploads a new certificate.
func (h *CertificateHandler) Create(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		CaCert     string `json:"ca_cert"`
		ClientCert string `json:"client_cert"`
		ClientKey  string `json:"client_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	userID := extractUserID(c)
	if userID <= 0 {
		Unauthorized(c, "未授权访问")
		return
	}

	cert := &model.Certificate{
		Name:       req.Name,
		CaCert:     req.CaCert,
		ClientCert: req.ClientCert,
		ClientKey:  req.ClientKey,
		CreatedBy:  userID,
	}

	created, err := h.certSvc.Create(c.Request.Context(), cert)
	if err != nil {
		slog.Error("failed to create certificate", "name", req.Name, "user_id", userID, "error", err)
		Fail(c, CodeQueryError, "创建证书失败")
		return
	}

	slog.Info("certificate created", "id", created.ID, "name", created.Name, "user_id", userID)
	c.Set("audit_resource_id", strconv.FormatInt(created.ID, 10))
	c.Set("audit_resource_name", created.Name)
	Created(c, gin.H{"id": created.ID})
}

// Get retrieves a certificate by ID. The client_key is always masked in the response.
func (h *CertificateHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的证书ID")
		return
	}

	cert, err := h.certSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to get certificate", "id", id, "error", err)
		NotFound(c, "证书不存在")
		return
	}

	if !checkOwnership(c, h.permSvc, cert.CreatedBy) {
		return
	}

	Success(c, gin.H{
		"id":          cert.ID,
		"name":        cert.Name,
		"ca_cert":     cert.CaCert,
		"client_cert": cert.ClientCert,
		"client_key":  "******",
		"created_by":  cert.CreatedBy,
		"created_at":  cert.CreatedAt.Format(TimeResponseFormat),
		"updated_at":  cert.UpdatedAt.Format(TimeResponseFormat),
	})
}

// Update modifies an existing certificate.
func (h *CertificateHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的证书ID")
		return
	}

	var req struct {
		Name       *string `json:"name"`
		CaCert     *string `json:"ca_cert"`
		ClientCert *string `json:"client_cert"`
		ClientKey  *string `json:"client_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	cert, err := h.certSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to get certificate for update", "id", id, "error", err)
		NotFound(c, "证书不存在")
		return
	}

	if !checkOwnership(c, h.permSvc, cert.CreatedBy) {
		return
	}

	if req.Name != nil {
		cert.Name = *req.Name
	}
	if req.CaCert != nil {
		cert.CaCert = *req.CaCert
	}
	if req.ClientCert != nil {
		cert.ClientCert = *req.ClientCert
	}
	// 仅当用户主动提供新的 client_key 时才更新，避免解密后重新加密
	if req.ClientKey != nil && *req.ClientKey != "" && *req.ClientKey != "******" {
		cert.ClientKey = *req.ClientKey
	} else {
		// 未修改私钥，清空字段避免 service 层重新加密
		cert.ClientKey = ""
	}

	if err := h.certSvc.Update(c.Request.Context(), id, cert); err != nil {
		slog.Error("failed to update certificate", "id", id, "error", err)
		Fail(c, CodeQueryError, "更新证书失败")
		return
	}

	slog.Info("certificate updated", "id", id, "name", cert.Name)
	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	c.Set("audit_resource_name", cert.Name)
	Success(c, nil)
}

// Delete removes a certificate by ID.
func (h *CertificateHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的证书ID")
		return
	}

	cert, err := h.certSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to get certificate for delete", "id", id, "error", err)
		NotFound(c, "证书不存在")
		return
	}

	if !checkOwnership(c, h.permSvc, cert.CreatedBy) {
		return
	}

	if err := h.certSvc.Delete(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete certificate", "id", id, "error", err)
		Fail(c, CodeQueryError, "删除证书失败")
		return
	}

	slog.Info("certificate deleted", "id", id)
	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	c.Set("audit_resource_name", cert.Name)
	Success(c, nil)
}
