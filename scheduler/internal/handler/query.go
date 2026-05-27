package handler

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

type QueryHandler struct {
	dsService         *datasource.DatasourceService
	manager           *datasource.Manager
	config            *datasource.ConfigService
	cacheService      *datasource.CacheService
	concurrentService *datasource.ConcurrentService
}

func NewQueryHandler(dsService *datasource.DatasourceService, manager *datasource.Manager, config *datasource.ConfigService, cacheService *datasource.CacheService, concurrentService *datasource.ConcurrentService) *QueryHandler {
	return &QueryHandler{
		dsService:         dsService,
		manager:           manager,
		config:            config,
		cacheService:      cacheService,
		concurrentService: concurrentService,
	}
}

func (h *QueryHandler) Execute(c *gin.Context) {
	var req struct {
		DatasourceID int64  `json:"datasource_id" binding:"required"`
		SQL          string `json:"sql" binding:"required"`
		Database     string `json:"database"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	sqlPreview := req.SQL
	if len(sqlPreview) > 200 {
		sqlPreview = sqlPreview[:200] + "..."
	}
	slog.Debug("query execute request", "datasource_id", req.DatasourceID, "database", req.Database, "sql_preview", sqlPreview)

	maxSQLLength := h.config.GetInt("datasource.max_sql_length")
	if maxSQLLength > 0 && len(req.SQL) > maxSQLLength {
		slog.Warn("sql exceeds max length", "datasource_id", req.DatasourceID, "sql_length", len(req.SQL), "max_length", maxSQLLength)
		Fail(c, CodeQueryNoDatasource, "SQL语句超过最大长度限制")
		return
	}

	ds, err := h.dsService.GetByID(c.Request.Context(), req.DatasourceID)
	if err != nil {
		slog.Error("datasource not found", "datasource_id", req.DatasourceID, "error", err)
		NotFound(c, "数据源不存在")
		return
	}
	if !ds.IsEnabled {
		slog.Warn("datasource is disabled", "datasource_id", req.DatasourceID, "name", ds.Name)
		Fail(c, CodeQueryDisabled, "数据源已被禁用，无法执行查询")
		return
	}

	if !h.isSelectOnly(req.SQL, ds.AllowWriteSQL) {
		slog.Warn("sql type not allowed", "datasource_id", req.DatasourceID, "sql_preview", sqlPreview)
		Fail(c, CodeQueryConnectFailed, "仅允许执行SELECT查询，DML/DDL操作需要数据源管理员启用写入权限")
		return
	}

	if h.cacheService != nil {
		cached, hit, err := h.cacheService.Get(c.Request.Context(), req.DatasourceID, req.SQL)
		if err == nil && hit {
			slog.Debug("query cache hit", "datasource_id", req.DatasourceID, "row_count", cached.RowCount)
			Success(c, gin.H{
				"columns":        cached.Columns,
				"rows":           cached.Rows,
				"row_count":      cached.RowCount,
				"from_cache":     true,
				"execution_time": 0,
			})
			return
		}
		if err != nil {
			slog.Debug("query cache miss", "datasource_id", req.DatasourceID, "error", err)
		}
	}

	userID, _ := c.Get("user_id")
	var uid int64
	if v, ok := userID.(int64); ok {
		uid = v
	}

	if h.concurrentService != nil {
		release, err := h.concurrentService.Acquire(c.Request.Context(), uid)
		if err != nil {
			if err == datasource.ErrConcurrentLimit {
				slog.Warn("concurrent query limit reached", "user_id", userID, "datasource_id", req.DatasourceID)
				Fail(c, CodeQuerySelectOnly, "并发查询数量已达上限，请稍后重试")
				return
			}
			slog.Error("concurrent check failed", "user_id", userID, "datasource_id", req.DatasourceID, "error", err)
			Fail(c, CodeQueryError, "并发检查失败，请稍后重试")
			return
		}
		defer release()
	}

	queryTimeout := h.config.GetInt("datasource.query_timeout")
	if queryTimeout <= 0 {
		queryTimeout = 60
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(queryTimeout)*time.Second)
	defer cancel()

	drv, err := h.manager.GetDriver(ctx, ds)
	if err != nil {
		slog.Error("failed to get datasource driver", "datasource_id", req.DatasourceID, "type", ds.Type, "error", err)
		Fail(c, CodeDatasourceNotFound, "连接数据源失败，请检查数据源配置")
		return
	}

	if req.Database != "" {
		if useErr := drv.UseDatabase(ctx, req.Database); useErr != nil {
			slog.Error("failed to switch database", "datasource_id", req.DatasourceID, "database", req.Database, "error", useErr)
			Fail(c, CodeQueryError, fmt.Sprintf("切换数据库失败: %v", useErr))
			return
		}
		slog.Debug("switched database context", "datasource_id", req.DatasourceID, "database", req.Database)
	}

	startTime := time.Now()
	normalizedSQL := driver.NormalizeSQLForType(req.SQL, ds.Type)
	result, err := drv.Query(ctx, normalizedSQL)
	execTime := time.Since(startTime).Seconds()

	if req.Database != "" && ds.Database != req.Database {
		if restoreErr := drv.UseDatabase(ctx, ds.Database); restoreErr != nil {
			slog.Warn("failed to restore database context", "datasource_id", req.DatasourceID, "original_database", ds.Database, "error", restoreErr)
		}
	}

	domainID, _ := c.Get("current_domain_id")
	var dID int64
	if v, ok := domainID.(int64); ok {
		dID = v
	}

	history := &model.QueryHistory{
		QueryID:        "q_" + time.Now().Format("20060102") + "_" + uuid.New().String()[:8],
		DatasourceID:   &ds.ID,
		DatasourceName: ds.Name,
		SQLText:        req.SQL,
		Database:       req.Database,
		ExecutionTime:  execTime,
		ExecutedBy:     int64Ptr(uid),
		DomainID:       dID,
	}

	if err != nil {
		history.Status = "failed"
		history.ErrorMessage = err.Error()
		h.dsService.RecordQueryHistory(c.Request.Context(), history)
		slog.Error("query execution failed", "datasource_id", req.DatasourceID, "datasource_name", ds.Name, "database", req.Database, "execution_time", execTime, "error", err)
		FailWithData(c, CodeQueryError, fmt.Sprintf("查询执行失败: %v", err), gin.H{"execution_time": execTime})
		return
	}

	history.Status = "success"
	history.RowCount = int(result.RowCount)
	h.dsService.RecordQueryHistory(c.Request.Context(), history)
	slog.Info("query executed successfully", "datasource_id", req.DatasourceID, "datasource_name", ds.Name, "database", req.Database, "row_count", result.RowCount, "execution_time", execTime)

	if h.cacheService != nil && result != nil {
		if err := h.cacheService.Set(c.Request.Context(), req.DatasourceID, req.SQL, result); err != nil {
			slog.Warn("failed to cache query result", "datasource_id", req.DatasourceID, "error", err)
		}
	}

	Success(c, gin.H{
		"columns":        result.Columns,
		"rows":           result.Rows,
		"row_count":      result.RowCount,
		"execution_time": execTime,
	})
}

func (h *QueryHandler) GetMetadata(c *gin.Context) {
	dsID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的数据源ID")
		return
	}

	level := c.Query("level")
	database := c.Query("database")
	table := c.Query("table")

	ds, err := h.dsService.GetByID(c.Request.Context(), dsID)
	if err != nil {
		NotFound(c, "数据源不存在")
		return
	}

	metadataTimeout := h.config.GetInt("datasource.metadata_timeout")
	if metadataTimeout <= 0 {
		metadataTimeout = 30
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(metadataTimeout)*time.Second)
	defer cancel()

	drv, err := h.manager.GetDriver(ctx, ds)
	if err != nil {
		Fail(c, CodeDatasourceNotFound, "连接数据源失败，请检查数据源配置")
		return
	}

	switch level {
	case "databases":
		dbs, err := drv.GetDatabases(ctx)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				Fail(c, CodeQueryError, "获取数据库列表超时，请检查数据源连接")
			} else {
				Fail(c, CodeQueryError, "获取数据库列表失败")
			}
			return
		}
		Success(c, dbs)
	case "tables":
		tables, err := drv.GetTables(ctx, database)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				Fail(c, CodeQueryError, "获取数据表列表超时，请检查数据源连接")
			} else {
				Fail(c, CodeQueryError, "获取数据表列表失败")
			}
			return
		}
		Success(c, tables)
	case "columns":
		columns, err := drv.GetColumns(ctx, database, table)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				Fail(c, CodeQueryError, "获取字段列表超时，请检查数据源连接")
			} else {
				Fail(c, CodeQueryError, "获取字段列表失败")
			}
			return
		}
		Success(c, columns)
	default:
		BadRequest(c, "无效的元数据层级，必须是 databases/tables/columns")
	}
}

func (h *QueryHandler) ExportCSV(c *gin.Context) {
	var req struct {
		DatasourceID int64  `json:"datasource_id" binding:"required"`
		SQL          string `json:"sql" binding:"required"`
		Database     string `json:"database"`
		MaxRows      int    `json:"max_rows"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	maxExportRows := h.config.GetInt("datasource.max_export_rows")
	if req.MaxRows > 0 && req.MaxRows < maxExportRows {
		maxExportRows = req.MaxRows
	}

	ds, err := h.dsService.GetByID(c.Request.Context(), req.DatasourceID)
	if err != nil {
		NotFound(c, "数据源不存在")
		return
	}

	if !h.isSelectOnly(req.SQL, ds.AllowWriteSQL) {
		Fail(c, CodeQueryConnectFailed, "仅允许执行SELECT查询，DML/DDL操作需要数据源管理员启用写入权限")
		return
	}

	var result *driver.QueryResult

	if h.cacheService != nil {
		cached, hit, cacheErr := h.cacheService.Get(c.Request.Context(), req.DatasourceID, req.SQL)
		if cacheErr != nil {
			slog.Warn("export: cache lookup failed, will re-execute query", "error", cacheErr)
		}
		if hit && cached != nil {
			result = cached
		}
	}

	if result == nil {
		queryTimeout := h.config.GetInt("datasource.query_timeout")
		if queryTimeout <= 0 {
			queryTimeout = 60
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(queryTimeout)*time.Second)
		defer cancel()

		drv, err := h.manager.GetDriver(ctx, ds)
		if err != nil {
			Fail(c, CodeDatasourceNotFound, "连接数据源失败，请检查数据源配置")
			return
		}

		if req.Database != "" {
			if useErr := drv.UseDatabase(ctx, req.Database); useErr != nil {
				slog.Error("export: failed to switch database", "datasource_id", req.DatasourceID, "database", req.Database, "error", useErr)
				Fail(c, CodeQueryError, fmt.Sprintf("切换数据库失败: %v", useErr))
				return
			}
		}

		result, err = drv.Query(ctx, req.SQL)
		if err != nil {
			Fail(c, CodeQueryError, fmt.Sprintf("查询执行失败: %v", err))
			return
		}

		if req.Database != "" && ds.Database != req.Database {
			if restoreErr := drv.UseDatabase(ctx, ds.Database); restoreErr != nil {
				slog.Warn("export: failed to restore database context", "datasource_id", req.DatasourceID, "original_database", ds.Database, "error", restoreErr)
			}
		}

		if h.cacheService != nil && result != nil {
			if err := h.cacheService.Set(c.Request.Context(), req.DatasourceID, req.SQL, result); err != nil {
				slog.Warn("export: failed to cache query result", "error", err)
			}
		}
	}

	if int(result.RowCount) > maxExportRows {
		FailWithData(c, CodeQueryTimeout, "export row count exceeds maximum limit", gin.H{"max": maxExportRows})
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=query_result.csv")
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(c.Writer)
	writer.Write(result.Columns)
	for _, row := range result.Rows {
		record := make([]string, len(row))
		for i, v := range row {
			if v == nil {
				record[i] = ""
			} else {
				record[i] = fmt.Sprintf("%v", v)
			}
		}
		writer.Write(record)
	}
	writer.Flush()
}

func (h *QueryHandler) GetHistory(c *gin.Context) {
	domainID, _ := c.Get("current_domain_id")
	role, _ := c.Get("role")

	var userRole string
	if v, ok := role.(string); ok {
		userRole = v
	}

	var currentDomainID int64
	if v, ok := domainID.(int64); ok {
		currentDomainID = v
	}

	var queryDomainID int64
	if userRole == "system_admin" || userRole == "admin" {
		if d := c.Query("domain_id"); d != "" {
			queryDomainID, _ = strconv.ParseInt(d, 10, 64)
		}
	} else {
		queryDomainID = currentDomainID
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	histories, total, err := h.dsService.GetQueryHistory(c.Request.Context(), queryDomainID, page, pageSize)
	if err != nil {
		Fail(c, CodeQueryError, "获取查询历史失败")
		return
	}

	Success(c, gin.H{
		"items":     histories,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// DeleteQueryHistory deletes a single query history record
func (h *QueryHandler) DeleteQueryHistory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的历史记录ID")
		return
	}

	if err := h.dsService.DeleteQueryHistory(c.Request.Context(), id); err != nil {
		Fail(c, CodeQueryHistoryNotFound, "删除查询历史失败")
		return
	}

	Success(c, nil)
}

// BatchDeleteQueryHistory batch deletes query history records
func (h *QueryHandler) BatchDeleteQueryHistory(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	if len(req.IDs) == 0 {
		BadRequest(c, "请选择要删除的记录")
		return
	}

	if err := h.dsService.BatchDeleteQueryHistory(c.Request.Context(), req.IDs); err != nil {
		Fail(c, CodeSavedSQLNotFound, "批量删除查询历史失败")
		return
	}

	Success(c, nil)
}

func (h *QueryHandler) SaveSQL(c *gin.Context) {
	var req struct {
		Name         string `json:"name" binding:"required"`
		DatasourceID int64  `json:"datasource_id" binding:"required"`
		SQLText      string `json:"sql_text" binding:"required"`
		Description  string `json:"description"`
		IsPublic     *bool  `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	domainID, _ := c.Get("current_domain_id")
	isPublic := false
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}

	var saveUID int64
	if v, ok := userID.(int64); ok {
		saveUID = v
	}
	var saveDomainID int64
	if v, ok := domainID.(int64); ok {
		saveDomainID = v
	}

	saved := &model.SavedSQL{
		Name: req.Name, DatasourceID: req.DatasourceID,
		SQLText: req.SQLText, Description: req.Description,
		CreatedBy: int64Ptr(saveUID), UpdatedBy: int64Ptr(saveUID),
		DomainID: saveDomainID, IsPublic: isPublic,
	}

	if err := h.dsService.CreateSavedSQL(c.Request.Context(), saved); err != nil {
		Fail(c, CodeQueryError, "保存SQL失败")
		return
	}

	Created(c, gin.H{"id": saved.ID})
}

func (h *QueryHandler) ListSavedSQL(c *gin.Context) {
	domainID, _ := c.Get("current_domain_id")
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")

	var userRole string
	if v, ok := role.(string); ok {
		userRole = v
	}

	var currentDomainID int64
	if v, ok := domainID.(int64); ok {
		currentDomainID = v
	}

	var listUID int64
	if v, ok := userID.(int64); ok {
		listUID = v
	}

	var queryDomainID int64
	if userRole == "system_admin" || userRole == "admin" {
		if d := c.Query("domain_id"); d != "" {
			queryDomainID, _ = strconv.ParseInt(d, 10, 64)
		}
	} else {
		queryDomainID = currentDomainID
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	savedList, total, err := h.dsService.GetSavedSQL(c.Request.Context(), queryDomainID, listUID, page, pageSize)
	if err != nil {
		Fail(c, CodeQueryError, "获取已保存SQL列表失败")
		return
	}

	Success(c, gin.H{
		"items":     savedList,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *QueryHandler) DeleteSavedSQL(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的SQL记录ID")
		return
	}

	if err := h.dsService.DeleteSavedSQL(c.Request.Context(), id); err != nil {
		Fail(c, CodeQueryError, "删除SQL记录失败")
		return
	}

	Success(c, nil)
}

func (h *QueryHandler) Cancel(c *gin.Context) {
	queryID := c.Param("query_id")
	if queryID == "" {
		BadRequest(c, "query_id不能为空")
		return
	}

	if h.concurrentService != nil {
		if err := h.concurrentService.SetCancelSignal(c.Request.Context(), queryID, 5*time.Minute); err != nil {
			Fail(c, CodeQueryError, "取消查询失败")
			return
		}
	}

	Success(c, nil)
}

func (h *QueryHandler) isSelectOnly(sql string, allowWriteSQL bool) bool {
	if allowWriteSQL {
		return true
	}
	trimmed := strings.ToUpper(strings.TrimSpace(sql))
	trimmed = joinSpaces(trimmed)
	return strings.HasPrefix(trimmed, "SELECT ") || strings.HasPrefix(trimmed, "WITH ") || strings.HasPrefix(trimmed, "EXPLAIN ") || strings.HasPrefix(trimmed, "SHOW ") || strings.HasPrefix(trimmed, "DESCRIBE ") || strings.HasPrefix(trimmed, "DESC ") || trimmed == "SELECT" || trimmed == "DESC" || trimmed == "SHOW" || strings.HasPrefix(trimmed, "PRAGMA")
}

func joinSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return b.String()
}
