package handler

import (
	"encoding/json"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

// ProtoFileHandler handles proto file management endpoints.
type ProtoFileHandler struct {
	protoSvc *service.ProtoService
	grpcExec *service.GRPCExecutor
	certSvc  *service.CertificateService
	permSvc  *service.PermissionService
}

// NewProtoFileHandler creates a new ProtoFileHandler instance.
func NewProtoFileHandler(protoSvc *service.ProtoService, grpcExec *service.GRPCExecutor, certSvc *service.CertificateService, permSvc *service.PermissionService) *ProtoFileHandler {
	return &ProtoFileHandler{
		protoSvc: protoSvc,
		grpcExec: grpcExec,
		certSvc:  certSvc,
		permSvc:  permSvc,
	}
}

// List returns proto files with pagination.
// System admin can see all proto files; other users can only see their own.
func (h *ProtoFileHandler) List(c *gin.Context) {
	uID := extractUserID(c)
	if uID <= 0 {
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

	isAdmin, _ := h.permSvc.IsSystemAdmin(c.Request.Context(), uID)

	protoFiles, total, err := h.protoSvc.ListByUser(c.Request.Context(), uID, isAdmin, page, pageSize)
	if err != nil {
		slog.Error("failed to list proto files", "user_id", uID, "error", err)
		Fail(c, CodeQueryError, "获取Proto文件列表失败")
		return
	}

	if protoFiles == nil {
		protoFiles = []*model.ProtoFile{}
	}

	items := make([]gin.H, 0, len(protoFiles))
	for _, pf := range protoFiles {
		items = append(items, protoFileToMap(pf))
	}

	Success(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Create uploads a new proto file.
func (h *ProtoFileHandler) Create(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		Content      string `json:"content" binding:"required"`
		Dependencies []int64 `json:"dependencies"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	uID := extractUserID(c)

	depJSON := "[]"
	if len(req.Dependencies) > 0 {
		depBytes, err := jsonMarshal(req.Dependencies)
		if err != nil {
			slog.Error("failed to marshal dependencies", "error", err)
			Fail(c, CodeInternalError, "序列化依赖失败")
			return
		}
		depJSON = depBytes
	}

	pf := &model.ProtoFile{
		Name:         req.Name,
		Content:      req.Content,
		Dependencies: depJSON,
		CreatedBy:    uID,
	}

	created, err := h.protoSvc.Create(c.Request.Context(), pf)
	if err != nil {
		slog.Error("failed to create proto file", "name", req.Name, "error", err)
		Fail(c, CodeQueryError, "创建Proto文件失败")
		return
	}

	// Parse the proto file content immediately
	if parseErr := h.protoSvc.ParseAndSave(c.Request.Context(), created.ID); parseErr != nil {
		slog.Warn("failed to parse proto file on create", "id", created.ID, "error", parseErr)
	}

	slog.Info("proto file created", "id", created.ID, "name", req.Name)
	c.Set("audit_resource_id", strconv.FormatInt(created.ID, 10))
	c.Set("audit_resource_name", req.Name)
	Created(c, gin.H{"id": created.ID})
}

// Get retrieves a proto file by ID.
func (h *ProtoFileHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的Proto文件ID")
		return
	}

	pf, err := h.protoSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		NotFound(c, "Proto文件不存在")
		return
	}

	if !checkOwnership(c, h.permSvc, pf.CreatedBy) {
		return
	}

	Success(c, protoFileToMap(pf))
}

// Update modifies an existing proto file.
func (h *ProtoFileHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的Proto文件ID")
		return
	}

	var req struct {
		Name         *string `json:"name"`
		Content      *string `json:"content"`
		Dependencies []int64 `json:"dependencies"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	pf, err := h.protoSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		NotFound(c, "Proto文件不存在")
		return
	}

	if !checkOwnership(c, h.permSvc, pf.CreatedBy) {
		return
	}

	contentChanged := false
	if req.Name != nil {
		pf.Name = *req.Name
	}
	if req.Content != nil {
		if pf.Content != *req.Content {
			contentChanged = true
		}
		pf.Content = *req.Content
	}
	if req.Dependencies != nil {
		depBytes, marshalErr := jsonMarshal(req.Dependencies)
		if marshalErr != nil {
			slog.Error("failed to marshal dependencies", "error", marshalErr)
			Fail(c, CodeInternalError, "序列化依赖失败")
			return
		}
		pf.Dependencies = depBytes
	}

	if err := h.protoSvc.Update(c.Request.Context(), id, pf); err != nil {
		slog.Error("failed to update proto file", "id", id, "error", err)
		Fail(c, CodeQueryError, "更新Proto文件失败")
		return
	}

	// Re-parse if content changed
	if contentChanged {
		if parseErr := h.protoSvc.ParseAndSave(c.Request.Context(), id); parseErr != nil {
			slog.Warn("failed to parse proto file on update", "id", id, "error", parseErr)
		}
	}

	slog.Info("proto file updated", "id", id, "name", pf.Name)
	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	c.Set("audit_resource_name", pf.Name)
	Success(c, nil)
}

// Delete removes a proto file by ID.
func (h *ProtoFileHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的Proto文件ID")
		return
	}

	pf, getErr := h.protoSvc.GetByID(c.Request.Context(), id)

	userID := extractUserID(c)
	isAdmin := false
	if userID > 0 {
		adminCheck, adminErr := h.permSvc.IsSystemAdmin(c.Request.Context(), userID)
		if adminErr == nil {
			isAdmin = adminCheck
		}
	}

	if !isAdmin {
		if getErr != nil {
			NotFound(c, "Proto文件不存在")
			return
		}
		if pf.CreatedBy != userID {
			Forbidden(c, "无权删除该Proto文件")
			return
		}
	}

	if err := h.protoSvc.Delete(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete proto file", "id", id, "error", err)
		Fail(c, CodeQueryError, "删除Proto文件失败")
		return
	}

	slog.Info("proto file deleted", "id", id)
	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	if pf != nil {
		c.Set("audit_resource_name", pf.Name)
	}
	Success(c, nil)
}

// Parse parses proto file content and returns the parsed result.
func (h *ProtoFileHandler) Parse(c *gin.Context) {
	var req struct {
		Content      string   `json:"content" binding:"required"`
		Dependencies []string `json:"dependencies"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	result, err := h.protoSvc.ParseProto(c.Request.Context(), req.Content, req.Dependencies)
	if err != nil {
		slog.Error("failed to parse proto content", "error", err)
		Fail(c, CodeQueryError, "解析Proto文件失败")
		return
	}

	Success(c, result)
}

// Reflect performs gRPC Server Reflection to discover services on a target server.
func (h *ProtoFileHandler) Reflect(c *gin.Context) {
	var req struct {
		Address       string `json:"address" binding:"required"`
		TLSMode       string `json:"tls_mode"`
		CertificateID *int64 `json:"certificate_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	tlsMode := req.TLSMode
	if tlsMode == "" {
		tlsMode = "insecure"
	}

	var cert *model.Certificate
	if req.CertificateID != nil && *req.CertificateID > 0 {
		loadedCert, certErr := h.certSvc.GetByID(c.Request.Context(), *req.CertificateID)
		if certErr != nil {
			slog.Error("failed to load certificate", "certificate_id", *req.CertificateID, "error", certErr)
			Fail(c, CodeQueryError, "加载证书失败")
			return
		}
		cert = loadedCert
	}

	services, err := h.grpcExec.ReflectServices(c.Request.Context(), req.Address, tlsMode, cert)
	if err != nil {
		slog.Error("gRPC reflection failed", "address", req.Address, "error", err)
		FailWithData(c, CodeQueryError, "gRPC反射失败", gin.H{
			"error": err.Error(),
		})
		return
	}

	if services == nil {
		services = []model.ProtoService{}
	}

	Success(c, gin.H{
		"services": services,
	})
}

// Fields returns detailed message field definitions for a proto file.
func (h *ProtoFileHandler) Fields(c *gin.Context) {
	var req struct {
		ProtoFileID int64 `json:"proto_file_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	pf, err := h.protoSvc.GetByID(c.Request.Context(), req.ProtoFileID)
	if err != nil {
		Fail(c, CodeQueryError, "Proto文件不存在")
		return
	}

	depContents := make(map[string]string)
	if pf.Dependencies != "" {
		var depIDs []int64
		if jsonErr := json.Unmarshal([]byte(pf.Dependencies), &depIDs); jsonErr == nil {
			for _, depID := range depIDs {
				dep, depErr := h.protoSvc.GetByID(c.Request.Context(), depID)
				if depErr != nil {
					slog.Warn("failed to load dependency proto file", "dep_id", depID, "error", depErr)
					continue
				}
				depContents[dep.Name] = dep.Content
			}
		}
	}

	messages, err := h.grpcExec.GenerateFields(pf.Content, depContents)
	if err != nil {
		slog.Error("failed to generate fields", "proto_file_id", req.ProtoFileID, "error", err)
		FailWithData(c, CodeQueryError, "生成字段定义失败", gin.H{
			"error": err.Error(),
		})
		return
	}

	if messages == nil {
		messages = []model.ProtoMessageDef{}
	}

	Success(c, gin.H{
		"messages": messages,
	})
}

// Template generates a JSON request body template for a given service and method from a proto file.
func (h *ProtoFileHandler) Template(c *gin.Context) {
	var req struct {
		ProtoFileID int64  `json:"proto_file_id" binding:"required"`
		Service     string `json:"service" binding:"required"`
		Method      string `json:"method" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	// Load proto file content and dependencies
	pf, err := h.protoSvc.GetByID(c.Request.Context(), req.ProtoFileID)
	if err != nil {
		Fail(c, CodeQueryError, "Proto文件不存在")
		return
	}

	depContents := make(map[string]string)
	if pf.Dependencies != "" {
		var depIDs []int64
		if jsonErr := json.Unmarshal([]byte(pf.Dependencies), &depIDs); jsonErr == nil {
			for _, depID := range depIDs {
				dep, depErr := h.protoSvc.GetByID(c.Request.Context(), depID)
				if depErr != nil {
					slog.Warn("failed to load dependency proto file", "dep_id", depID, "error", depErr)
					continue
				}
				depContents[dep.Name] = dep.Content
			}
		}
	}

	templateJSON, err := h.grpcExec.GenerateTemplate(pf.Content, depContents, req.Service, req.Method)
	if err != nil {
		slog.Error("failed to generate template", "proto_file_id", req.ProtoFileID, "service", req.Service, "method", req.Method, "error", err)
		FailWithData(c, CodeQueryError, "生成请求模板失败", gin.H{
			"error": err.Error(),
		})
		return
	}

	Success(c, gin.H{
		"template": templateJSON,
	})
}

// protoFileToMap converts a ProtoFile model to a gin.H map for response.
func protoFileToMap(pf *model.ProtoFile) gin.H {
	return gin.H{
		"id":              pf.ID,
		"name":            pf.Name,
		"content":         pf.Content,
		"file_hash":       pf.FileHash,
		"parsed_result":   pf.ParsedResult,
		"dependencies":    pf.Dependencies,
		"created_by":      pf.CreatedBy,
		"created_by_name": pf.CreatedByName,
		"created_at":      pf.CreatedAt.Format(TimeResponseFormat),
		"updated_at":      pf.UpdatedAt.Format(TimeResponseFormat),
	}
}

// jsonMarshal marshals v to JSON string.
func jsonMarshal(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
