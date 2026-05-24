package handler

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

type DatasourceHandler struct {
	dsService *datasource.DatasourceService
	manager   *datasource.Manager
	config    *datasource.ConfigService
}

func NewDatasourceHandler(dsService *datasource.DatasourceService, manager *datasource.Manager, config *datasource.ConfigService) *DatasourceHandler {
	return &DatasourceHandler{
		dsService: dsService,
		manager:   manager,
		config:    config,
	}
}

func (h *DatasourceHandler) List(c *gin.Context) {
	role, _ := c.Get("role")
	domainID, _ := c.Get("domain_id")
	userID, _ := c.Get("user_id")

	var queryDomainID int64
	if role == "system_admin" || role == "admin" {
		if d := c.Query("domain_id"); d != "" {
			queryDomainID, _ = strconv.ParseInt(d, 10, 64)
		}
	} else {
		queryDomainID = domainID.(int64)
	}

	dsType := c.Query("type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	datasources, total, err := h.dsService.Get(c.Request.Context(), queryDomainID, dsType, page, pageSize)
	if err != nil {
		Fail(c, CodeQueryError, "获取数据源列表失败")
		return
	}

	r := role.(string)
	if r == "user" {
		uID := userID.(int64)
		var filtered []*model.Datasource
		for _, ds := range datasources {
			hasPerm, err := h.dsService.CheckPermission(c.Request.Context(), uID, ds.ID, "query")
			if err == nil && hasPerm {
				filtered = append(filtered, ds)
			}
		}
		datasources = filtered
		total = int64(len(filtered))
	}

	items := make([]gin.H, 0, len(datasources))
	for _, ds := range datasources {
		item := gin.H{
			"id": ds.ID, "name": ds.Name, "type": ds.Type,
			"host": ds.Host, "port": ds.Port, "path": ds.Path,
			"database": ds.Database, "username": ds.Username,
			"auth_type": ds.AuthType, "description": ds.Description,
			"domain_id": ds.DomainID, "is_enabled": ds.IsEnabled,
			"allow_write_sql": ds.AllowWriteSQL,
			"test_status": ds.TestStatus, "last_test_at": formatTimePtr(ds.LastTestAt),
			"created_by": ds.CreatedBy, "updated_by": ds.UpdatedBy,
			"created_at": ds.CreatedAt, "updated_at": ds.UpdatedAt,
			"connection_mode": ds.ConnectionMode,
			"zk_hosts": ds.ZkHosts,
			"zk_path": ds.ZkPath,
			"rqlite_hosts": ds.RqliteHosts,
		}
		items = append(items, item)
	}

	Success(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *DatasourceHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的数据源ID")
		return
	}

	ds, err := h.dsService.GetByID(c.Request.Context(), id)
	if err != nil {
		NotFound(c, "数据源不存在")
		return
	}

	Success(c, gin.H{
		"id": ds.ID, "name": ds.Name, "type": ds.Type,
		"host": ds.Host, "port": ds.Port, "path": ds.Path,
		"database": ds.Database, "username": ds.Username,
		"password": "******", "auth_type": ds.AuthType,
		"config": ds.Config, "description": ds.Description,
		"domain_id": ds.DomainID, "is_enabled": ds.IsEnabled,
		"allow_write_sql": ds.AllowWriteSQL,
		"test_status": ds.TestStatus, "last_test_at": formatTimePtr(ds.LastTestAt),
		"created_by": ds.CreatedBy, "updated_by": ds.UpdatedBy,
		"created_at": ds.CreatedAt, "updated_at": ds.UpdatedAt,
		"connection_mode": ds.ConnectionMode,
		"zk_hosts": ds.ZkHosts,
		"zk_path": ds.ZkPath,
		"rqlite_hosts": ds.RqliteHosts,
	})
}

func (h *DatasourceHandler) Create(c *gin.Context) {
	var req struct {
		Name          string `json:"name" binding:"required"`
		Type          string `json:"type" binding:"required"`
		Host          string `json:"host"`
		Port          int    `json:"port"`
		Path          string `json:"path"`
		Database      string `json:"database"`
		Username      string `json:"username"`
		Password      string `json:"password"`
		AuthType      string `json:"auth_type"`
		ConnectionMode string `json:"connection_mode"`
		ZkHosts       string `json:"zk_hosts"`
		ZkPath        string `json:"zk_path"`
		RqliteHosts   string `json:"rqlite_hosts"`
		Config        string `json:"config"`
		Description   string `json:"description"`
		DomainID      int64  `json:"domain_id" binding:"required"`
		IsEnabled     *bool  `json:"is_enabled"`
		AllowWriteSQL *bool  `json:"allow_write_sql"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	if !driver.IsSupported(req.Type) {
		BadRequest(c, "不支持的数据源类型")
		return
	}

	if req.AuthType == "" {
		req.AuthType = "simple"
	}

	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	allowWriteSQL := false
	if req.AllowWriteSQL != nil {
		allowWriteSQL = *req.AllowWriteSQL
	}

	role, _ := c.Get("role")
	if allowWriteSQL && role != "system_admin" && role != "admin" {
		Fail(c, CodePermissionDenied, "仅系统管理员可启用DML语句权限")
		return
	}

	userID, _ := c.Get("user_id")
	ds := &model.Datasource{
		Name: req.Name, Type: req.Type, Host: req.Host, Port: req.Port,
		Path: req.Path, Database: req.Database, Username: req.Username,
		Password: req.Password, AuthType: req.AuthType, Config: req.Config,
		Description: req.Description, DomainID: req.DomainID,
		IsEnabled: isEnabled, AllowWriteSQL: allowWriteSQL,
		ConnectionMode: req.ConnectionMode,
		ZkHosts:        req.ZkHosts,
		ZkPath:         req.ZkPath,
		RqliteHosts:    req.RqliteHosts,
		CreatedBy: int64Ptr(userID.(int64)),
		UpdatedBy: int64Ptr(userID.(int64)),
	}

	testErr := h.manager.TestConnection(c.Request.Context(), ds)
	if testErr != nil {
		slog.Error("datasource test connection failed during create", "type", ds.Type, "host", ds.Host, "port", ds.Port, "error", testErr)
		FailWithData(c, CodeDatasourceConnectFailed, "连接测试失败，无法创建数据源", gin.H{
			"error": testErr.Error(),
		})
		return
	}

	ds.TestStatus = "success"
	now := time.Now()
	ds.LastTestAt = &now

	if err := h.dsService.Create(c.Request.Context(), ds); err != nil {
		if err == datasource.ErrDatasourceNameExists {
			Fail(c, CodeDatasourceNameExists, "该领域下已存在同名数据源")
			return
		}
		slog.Error("failed to create datasource", "name", ds.Name, "type", ds.Type, "error", err)
		Fail(c, CodeQueryError, "创建数据源失败")
		return
	}

	slog.Info("datasource created", "id", ds.ID, "name", ds.Name, "type", ds.Type)
	c.Set("audit_resource_id", strconv.FormatInt(ds.ID, 10))
	c.Set("audit_resource_name", ds.Name)
	Created(c, gin.H{"id": ds.ID})
}

func (h *DatasourceHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的数据源ID")
		return
	}

	var req struct {
		Name          *string `json:"name"`
		Host          *string `json:"host"`
		Port          *int    `json:"port"`
		Path          *string `json:"path"`
		Database      *string `json:"database"`
		Username      *string `json:"username"`
		Password      *string `json:"password"`
		AuthType      *string `json:"auth_type"`
		ConnectionMode *string `json:"connection_mode"`
		ZkHosts       *string `json:"zk_hosts"`
		ZkPath        *string `json:"zk_path"`
		RqliteHosts   *string `json:"rqlite_hosts"`
		Config        *string `json:"config"`
		Description   *string `json:"description"`
		IsEnabled     *bool   `json:"is_enabled"`
		AllowWriteSQL *bool   `json:"allow_write_sql"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	ds, err := h.dsService.GetByID(c.Request.Context(), id)
	if err != nil {
		NotFound(c, "数据源不存在")
		return
	}

	if req.Name != nil {
		ds.Name = *req.Name
	}
	if req.Host != nil {
		ds.Host = *req.Host
	}
	if req.Port != nil {
		ds.Port = *req.Port
	}
	if req.Path != nil {
		ds.Path = *req.Path
	}
	if req.Database != nil {
		ds.Database = *req.Database
	}
	if req.Username != nil {
		ds.Username = *req.Username
	}
	if req.Password != nil {
		ds.Password = *req.Password
	}
	if req.AuthType != nil {
		ds.AuthType = *req.AuthType
	}
	if req.ConnectionMode != nil {
		ds.ConnectionMode = *req.ConnectionMode
	}
	if req.ZkHosts != nil {
		ds.ZkHosts = *req.ZkHosts
	}
	if req.ZkPath != nil {
		ds.ZkPath = *req.ZkPath
	}
	if req.RqliteHosts != nil {
		ds.RqliteHosts = *req.RqliteHosts
	}
	if req.Config != nil {
		ds.Config = *req.Config
	}
	if req.Description != nil {
		ds.Description = *req.Description
	}
	if req.IsEnabled != nil {
		ds.IsEnabled = *req.IsEnabled
	}
	if req.AllowWriteSQL != nil {
		role, _ := c.Get("role")
		if *req.AllowWriteSQL && role != "system_admin" && role != "admin" {
			Fail(c, CodePermissionDenied, "仅系统管理员可启用DML语句权限")
			return
		}
		ds.AllowWriteSQL = *req.AllowWriteSQL
	}

	userID, _ := c.Get("user_id")
	ds.UpdatedBy = int64Ptr(userID.(int64))

	if err := h.dsService.Update(c.Request.Context(), ds); err != nil {
		slog.Error("failed to update datasource", "id", id, "error", err)
		Fail(c, CodeQueryError, "更新数据源失败")
		return
	}

	h.manager.RemoveDatasource(id)
	slog.Info("datasource updated", "id", id, "name", ds.Name)
	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	c.Set("audit_resource_name", ds.Name)
	Success(c, nil)
}

func (h *DatasourceHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的数据源ID")
		return
	}

	// 先获取数据源名称用于审计日志
	ds, _ := h.dsService.GetByID(c.Request.Context(), id)

	if err := h.dsService.Delete(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete datasource", "id", id, "error", err)
		Fail(c, CodeQueryError, "删除数据源失败")
		return
	}

	h.manager.RemoveDatasource(id)
	slog.Info("datasource deleted", "id", id)
	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	if ds != nil {
		c.Set("audit_resource_name", ds.Name)
	}
	Success(c, nil)
}

func (h *DatasourceHandler) TestConnection(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的数据源ID")
		return
	}

	if err := h.dsService.TestDatasource(c.Request.Context(), id); err != nil {
		slog.Error("datasource test connection failed", "id", id, "error", err)
		FailWithData(c, CodeDatasourceConnectFailed, "连接测试失败", gin.H{
			"error": err.Error(),
		})
		return
	}

	slog.Info("datasource test connection succeeded", "id", id)
	Success(c, gin.H{"status": "ok"})
}

func (h *DatasourceHandler) TestConnectionByParams(c *gin.Context) {
	var req struct {
		Type           string `json:"type" binding:"required"`
		Host           string `json:"host"`
		Port           int    `json:"port"`
		Path           string `json:"path"`
		Database       string `json:"database"`
		Username       string `json:"username"`
		Password       string `json:"password"`
		AuthType       string `json:"auth_type"`
		ConnectionMode string `json:"connection_mode"`
		ZkHosts        string `json:"zk_hosts"`
		ZkPath         string `json:"zk_path"`
		RqliteHosts    string `json:"rqlite_hosts"`
		Config         string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	if !driver.IsSupported(req.Type) {
		BadRequest(c, "不支持的数据源类型")
		return
	}

	ds := &model.Datasource{
		Type:           req.Type,
		Host:           req.Host,
		Port:           req.Port,
		Path:           req.Path,
		Database:       req.Database,
		Username:       req.Username,
		Password:       req.Password,
		AuthType:       req.AuthType,
		ConnectionMode: req.ConnectionMode,
		ZkHosts:        req.ZkHosts,
		ZkPath:         req.ZkPath,
		RqliteHosts:    req.RqliteHosts,
		Config:         req.Config,
		IsEnabled:     true,
	}

	if err := h.manager.TestConnection(c.Request.Context(), ds); err != nil {
		slog.Error("datasource test connection by params failed", "type", req.Type, "host", req.Host, "port", req.Port, "error", err)
		FailWithData(c, CodeDatasourceConnectFailed, "连接测试失败", gin.H{
			"error": err.Error(),
		})
		return
	}

	slog.Debug("datasource test connection by params succeeded", "type", req.Type, "host", req.Host)
	Success(c, gin.H{"status": "ok"})
}

func (h *DatasourceHandler) GrantPermission(c *gin.Context) {
	var req struct {
		RoleID         *int64 `json:"role_id"`
		UserID         *int64 `json:"user_id"`
		PermissionType string `json:"permission_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	if req.RoleID == nil && req.UserID == nil {
		BadRequest(c, "请选择授权对象（用户或角色）")
		return
	}

	if !datasource.IsValidPermissionType(req.PermissionType) {
		BadRequest(c, "无效的权限类型")
		return
	}

	dsID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的数据源ID")
		return
	}

	userID, _ := c.Get("user_id")
	perm := &model.DatasourcePermission{
		DatasourceID:   dsID,
		RoleID:         req.RoleID,
		UserID:         req.UserID,
		PermissionType: req.PermissionType,
		GrantedBy:      int64Ptr(userID.(int64)),
		GrantedAt:      "",
	}

	if err := h.dsService.GrantPermission(c.Request.Context(), perm); err != nil {
		if err == datasource.ErrPermissionExists {
			Fail(c, CodePermissionExists, "该权限已存在，请勿重复添加")
			return
		}
		if err == datasource.ErrInvalidPermissionType {
			BadRequest(c, "无效的权限类型")
			return
		}
		Fail(c, CodeQueryError, "添加权限失败")
		return
	}

	Created(c, nil)
}

func (h *DatasourceHandler) RevokePermission(c *gin.Context) {
	permID, err := strconv.ParseInt(c.Param("perm_id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的权限ID")
		return
	}

	if err := h.dsService.RevokePermission(c.Request.Context(), permID); err != nil {
		Fail(c, CodeQueryError, "撤销权限失败")
		return
	}

	Success(c, nil)
}

func (h *DatasourceHandler) UpdatePermission(c *gin.Context) {
	permID, err := strconv.ParseInt(c.Param("perm_id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的权限ID")
		return
	}

	var req struct {
		PermissionType string `json:"permission_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	if !datasource.IsValidPermissionType(req.PermissionType) {
		BadRequest(c, "无效的权限类型")
		return
	}

	if err := h.dsService.UpdatePermission(c.Request.Context(), permID, req.PermissionType); err != nil {
		if err == datasource.ErrPermissionNotFound {
			NotFound(c, "权限记录不存在")
			return
		}
		Fail(c, CodeQueryError, "修改权限失败")
		return
	}

	Success(c, nil)
}

func (h *DatasourceHandler) GetPermissions(c *gin.Context) {
	dsID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的数据源ID")
		return
	}

	perms, err := h.dsService.GetPermissions(c.Request.Context(), dsID)
	if err != nil {
		Fail(c, CodeQueryError, "获取权限列表失败")
		return
	}

	if perms == nil {
		perms = []*model.DatasourcePermission{}
	}
	Success(c, perms)
}

func (h *DatasourceHandler) SupportedTypes(c *gin.Context) {
	types := driver.SupportedTypes()
	Success(c, types)
}

func int64Ptr(v int64) *int64 {
	return &v
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(TimeResponseFormat)
}
