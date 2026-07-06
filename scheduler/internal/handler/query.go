package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource"
	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
	"github.com/lynnyq/bdopsflow/scheduler/internal/metrics"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
	"github.com/redis/go-redis/v9"
)

type QueryHandler struct {
	dsService         *datasource.DatasourceService
	manager           *datasource.Manager
	configService     *sysconfig.Service
	cacheService      *datasource.CacheService
	concurrentService *datasource.ConcurrentService
	registry          queryRegistry

	// 运行时配置缓存（用于热更新）
	runtimeConfig struct {
		defaultLimit         int
		maxExportRows        int
		queryTimeout         int
		maxSQLLength         int
		maxConcurrentPerUser int
		maxConcurrentGlobal  int
		allowWriteSQL        bool
		metadataTimeout      int
		mu                   sync.RWMutex
	}
}

// queryRegistry 查询注册表接口，支持本地内存和 Redis 分布式实现
type queryRegistry interface {
	Register(query *RunningQuery)
	Get(queryID string) (*RunningQuery, bool)
	UpdateResult(queryID string, result *driver.QueryResult, execTime float64)
	UpdateError(queryID string, errMsg string, execTime float64)
	Cancel(queryID string) bool
	SetRunning(queryID string)
	Cleanup(maxAge time.Duration)
	StartCleanupLoop(interval, maxAge time.Duration)
	RegisterObserver(observer QueryObserver)
	UnregisterObserver(observer QueryObserver)
}

func NewQueryHandler(dsService *datasource.DatasourceService, manager *datasource.Manager, configService *sysconfig.Service, cacheService *datasource.CacheService, concurrentService *datasource.ConcurrentService, redisClient *redis.Client, nodeID string) *QueryHandler {
	var registry queryRegistry
	if redisClient != nil {
		registry = NewDistributedQueryRegistry(redisClient, nodeID)
		slog.Info("using distributed query registry (Redis-backed)", "node_id", nodeID)
	} else {
		registry = NewQueryRegistry()
		slog.Info("using local query registry (in-memory)")
	}
	registry.StartCleanupLoop(5*time.Minute, 30*time.Minute)

	h := &QueryHandler{
		dsService:         dsService,
		manager:           manager,
		configService:     configService,
		cacheService:      cacheService,
		concurrentService: concurrentService,
		registry:          registry,
	}

	// 初始化运行时配置
	h.refreshRuntimeConfig()

	// 注册为配置观察者，实现热更新
	configService.RegisterObserver(h)

	return h
}

// OnConfigChanged 实现 ConfigObserver 接口，配置变更时自动更新
func (h *QueryHandler) OnConfigChanged(key, value string) {
	h.refreshRuntimeConfig()
	slog.Info("query handler config updated", "key", key, "value", value)
}

// refreshRuntimeConfig 刷新运行时配置缓存
func (h *QueryHandler) refreshRuntimeConfig() {
	h.runtimeConfig.mu.Lock()
	defer h.runtimeConfig.mu.Unlock()

	h.runtimeConfig.defaultLimit = h.configService.GetInt("datasource.default_limit")
	h.runtimeConfig.maxExportRows = h.configService.GetInt("datasource.max_export_rows")
	h.runtimeConfig.queryTimeout = h.configService.GetInt("datasource.query_timeout")
	h.runtimeConfig.maxSQLLength = h.configService.GetInt("datasource.max_sql_length")
	h.runtimeConfig.maxConcurrentPerUser = h.configService.GetInt("datasource.max_concurrent_per_user")
	h.runtimeConfig.maxConcurrentGlobal = h.configService.GetInt("datasource.max_concurrent_global")
	h.runtimeConfig.allowWriteSQL = h.configService.GetBool("datasource.allow_write_sql")
	h.runtimeConfig.metadataTimeout = h.configService.GetInt("datasource.metadata_timeout")
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

	h.runtimeConfig.mu.RLock()
	maxSQLLength := h.runtimeConfig.maxSQLLength
	h.runtimeConfig.mu.RUnlock()

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

	// 读取 defaultLimit 用于缓存 key 隔离。
	// 缓存 key 包含 limit，确保不同行数限制的查询结果互不干扰，
	// 调整 default_limit 后新查询会生成新 key，不会命中旧缓存。
	h.runtimeConfig.mu.RLock()
	defaultLimitForCache := h.runtimeConfig.defaultLimit
	h.runtimeConfig.mu.RUnlock()

	if h.cacheService != nil {
		cached, hit, err := h.cacheService.Get(c.Request.Context(), req.DatasourceID, req.Database, req.SQL, defaultLimitForCache)
		if err == nil && hit {
			slog.Debug("query cache hit", "datasource_id", req.DatasourceID, "row_count", cached.RowCount)
			queryID := "q_" + time.Now().Format("20060102") + "_" + uuid.New().String()[:8]
			Success(c, gin.H{
				"query_id":       queryID,
				"status":         string(QueryStatusCompleted),
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
		release, err := h.concurrentService.AcquireForDatasource(c.Request.Context(), uid, req.DatasourceID)
		if err != nil {
			if err == datasource.ErrDatasourceConcurrentLimit {
				slog.Warn("datasource concurrent query limit reached", "user_id", userID, "datasource_id", req.DatasourceID)
				Fail(c, CodeConcurrentLimit, "该数据源并发查询数已达上限，请稍后重试")
				return
			}
			if err == datasource.ErrConcurrentLimit {
				slog.Warn("concurrent query limit reached", "user_id", userID, "datasource_id", req.DatasourceID)
				Fail(c, CodeConcurrentLimit, "并发查询数量已达上限，请稍后重试")
				return
			}
			if err == datasource.ErrPoolExhausted {
				slog.Warn("datasource connection pool exhausted", "user_id", userID, "datasource_id", req.DatasourceID)
				Fail(c, CodeConcurrentLimit, "数据源连接池已满，请稍后重试")
				return
			}
			slog.Error("concurrent check failed", "user_id", userID, "datasource_id", req.DatasourceID, "error", err)
			Fail(c, CodeQueryError, "并发检查失败，请稍后重试")
			return
		}
		defer release()
	}

	queryID := "q_" + time.Now().Format("20060102") + "_" + uuid.New().String()[:8]

	h.runtimeConfig.mu.RLock()
	queryTimeout := h.runtimeConfig.queryTimeout
	h.runtimeConfig.mu.RUnlock()

	if queryTimeout <= 0 {
		queryTimeout = 60
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(queryTimeout)*time.Second)

	domainID, _ := c.Get("current_domain_id")
	var dID int64
	if v, ok := domainID.(int64); ok {
		dID = v
	}

	runningQuery := &RunningQuery{
		QueryID:      queryID,
		DatasourceID: req.DatasourceID,
		Database:     req.Database,
		SQL:          req.SQL,
		UserID:       uid,
		Status:       QueryStatusPending,
		CancelFunc:   cancel,
	}
	h.registry.Register(runningQuery)

	go h.executeQuerySafe(ctx, cancel, queryID, ds, req, uid, dID)

	Success(c, gin.H{
		"query_id": queryID,
		"status":   string(QueryStatusPending),
	})
}

func (h *QueryHandler) executeQuerySafe(ctx context.Context, cancel context.CancelFunc, queryID string, ds *model.Datasource, req struct {
	DatasourceID int64  `json:"datasource_id" binding:"required"`
	SQL          string `json:"sql" binding:"required"`
	Database     string `json:"database"`
}, uid int64, domainID int64) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("query execution panic recovered", "query_id", queryID, "datasource_id", req.DatasourceID, "panic", r)
			h.registry.UpdateError(queryID, fmt.Sprintf("查询执行异常: %v", r), 0)
		}
	}()
	h.executeQuery(ctx, cancel, queryID, ds, req, uid, domainID)
}

func (h *QueryHandler) executeQuery(ctx context.Context, cancel context.CancelFunc, queryID string, ds *model.Datasource, req struct {
	DatasourceID int64  `json:"datasource_id" binding:"required"`
	SQL          string `json:"sql" binding:"required"`
	Database     string `json:"database"`
}, uid int64, domainID int64) {
	h.registry.SetRunning(queryID)

	drv, err := h.manager.GetDriver(ctx, ds)
	if err != nil {
		slog.Error("failed to get datasource driver", "query_id", queryID, "datasource_id", req.DatasourceID, "type", ds.Type, "error", err)
		h.registry.UpdateError(queryID, "连接数据源失败，请检查数据源配置", 0)
		return
	}

	startTime := time.Now()
	// 应用默认查询行数限制（从运行时配置读取，支持热更新）
	h.runtimeConfig.mu.RLock()
	defaultLimit := h.runtimeConfig.defaultLimit
	h.runtimeConfig.mu.RUnlock()

	querySQL := driver.ApplyLimitToSQL(req.SQL, defaultLimit, ds.Type)

	// 使用统一重试机制执行查询
	var result *driver.QueryResult
	retryCfg := driver.DefaultRetryConfig
	result, err = driver.WithRetry(ctx, retryCfg, func(queryCtx context.Context) (*driver.QueryResult, error) {
		return drv.QueryWithDB(queryCtx, querySQL, req.Database)
	}, ds.Type)

	execTime := time.Since(startTime).Seconds()

	metrics.DatasourceQueryDurationSeconds.Observe(execTime)

	history := &model.QueryHistory{
		QueryID:        queryID,
		DatasourceID:   &ds.ID,
		DatasourceName: ds.Name,
		SQLText:        req.SQL,
		Database:       req.Database,
		ExecutionTime:  execTime,
		ExecutedBy:     int64Ptr(uid),
		DomainID:       domainID,
	}

	if ctx.Err() == context.Canceled {
		history.Status = "cancelled"
		history.ErrorMessage = "查询已被用户取消"
		h.dsService.RecordQueryHistory(context.Background(), history)
		h.registry.UpdateError(queryID, "查询已被用户取消", execTime)
		metrics.DatasourceQueries.WithLabelValues(ds.Type, "cancelled").Inc()
		slog.Info("query cancelled by user", "query_id", queryID, "datasource_id", req.DatasourceID, "execution_time", execTime)
		return
	}

	if err != nil {
		history.Status = "failed"
		history.ErrorMessage = err.Error()
		h.dsService.RecordQueryHistory(context.Background(), history)
		slog.Error("query execution failed", "query_id", queryID, "datasource_id", req.DatasourceID, "datasource_name", ds.Name, "database", req.Database, "execution_time", execTime, "error", err)
		h.registry.UpdateError(queryID, err.Error(), execTime)
		metrics.DatasourceQueries.WithLabelValues(ds.Type, "failed").Inc()
		return
	}

	if result == nil {
		history.Status = "failed"
		history.ErrorMessage = "查询返回空结果"
		h.dsService.RecordQueryHistory(context.Background(), history)
		slog.Error("query returned nil result", "query_id", queryID, "datasource_id", req.DatasourceID, "datasource_name", ds.Name, "database", req.Database, "execution_time", execTime)
		h.registry.UpdateError(queryID, "查询返回空结果，请检查SQL语句或数据源连接", execTime)
		metrics.DatasourceQueries.WithLabelValues(ds.Type, "failed").Inc()
		return
	}

	history.Status = "success"
	history.RowCount = int(result.RowCount)
	h.dsService.RecordQueryHistory(context.Background(), history)
	slog.Info("query executed successfully", "query_id", queryID, "datasource_id", req.DatasourceID, "datasource_name", ds.Name, "database", req.Database, "row_count", result.RowCount, "execution_time", execTime)
	metrics.DatasourceQueries.WithLabelValues(ds.Type, "success").Inc()

	if h.cacheService != nil {
		if err := h.cacheService.Set(context.Background(), req.DatasourceID, req.Database, req.SQL, defaultLimit, result); err != nil {
			slog.Warn("failed to cache query result", "query_id", queryID, "datasource_id", req.DatasourceID, "error", err)
		}
	}

	h.registry.UpdateResult(queryID, result, execTime)
}

func (h *QueryHandler) GetMetadata(c *gin.Context) {
	startTime := time.Now()
	dsID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的数据源ID")
		return
	}

	level := c.Query("level")
	database := c.Query("database")
	table := c.Query("table")

	slog.Debug("GetMetadata: request received", "module", "query", "datasource_id", dsID, "level", level, "database", database, "table", table)

	ds, err := h.dsService.GetByID(c.Request.Context(), dsID)
	if err != nil {
		NotFound(c, "数据源不存在")
		return
	}

	metadataTimeout := 60
	h.runtimeConfig.mu.RLock()
	if mt := h.runtimeConfig.metadataTimeout; mt > 0 {
		metadataTimeout = mt
	}
	h.runtimeConfig.mu.RUnlock()
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(metadataTimeout)*time.Second)
	defer cancel()

	// GetDriver 使用独立的短超时，避免慢 Hive 源的 Ping/connect 阻塞整个 metadata 请求
	// metadata 请求是同步的，如果 GetDriver 阻塞会占用 HTTP 连接，导致浏览器连接池耗尽
	getDriverCtx, getDriverCancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	drv, err := h.manager.GetDriver(getDriverCtx, ds)
	getDriverCancel()
	if err != nil {
		slog.Warn("GetMetadata: failed to get driver", "module", "query", "datasource_id", dsID, "type", ds.Type, "error", err, "elapsed", time.Since(startTime))
		Fail(c, CodeDatasourceNotFound, "连接数据源失败，请检查数据源配置")
		return
	}

	// 检查连接池是否已满，满了直接返回错误，避免 acquire 阻塞占用浏览器连接
	// 浏览器每个域名最多 6 个并发连接，metadata 请求阻塞 30s 会导致连接池耗尽
	if pu, ok := drv.(driver.PoolConfigUpdater); ok {
		_, _, inUse, maxOpen := pu.GetPoolStats()
		if inUse >= maxOpen {
			slog.Warn("GetMetadata: connection pool fully occupied", "module", "query", "datasource_id", dsID, "type", ds.Type, "in_use", inUse, "max_open", maxOpen)
			Fail(c, CodeConcurrentLimit, "数据源连接池已满，请稍后重试")
			return
		}
	}

	switch level {
	case "databases":
		// 尝试从服务端缓存获取
		if h.cacheService != nil {
			cachedData, hit, cacheErr := h.cacheService.GetMetadata(ctx, dsID, "databases", "")
			if cacheErr != nil {
				slog.Debug("GetMetadata: databases cache lookup failed", "datasource_id", dsID, "error", cacheErr)
			}
			if hit && cachedData != nil {
				var dbs []string
				if err := json.Unmarshal(cachedData, &dbs); err == nil {
					slog.Debug("GetMetadata: databases cache hit", "datasource_id", dsID, "count", len(dbs), "elapsed", time.Since(startTime))
					Success(c, dbs)
					return
				}
			}
		}

		dbs, err := drv.GetDatabases(ctx)
		if err != nil {
			elapsed := time.Since(startTime)
			if isPoolBusyError(err) {
				slog.Warn("GetMetadata: databases pool busy", "module", "query", "datasource_id", dsID, "type", ds.Type, "elapsed", elapsed)
				Fail(c, CodeConcurrentLimit, "数据源连接池已满，请稍后重试")
			} else if ctx.Err() == context.DeadlineExceeded {
				slog.Warn("GetMetadata: databases timeout", "module", "query", "datasource_id", dsID, "type", ds.Type, "elapsed", elapsed)
				Fail(c, CodeQueryError, "获取数据库列表超时，请稍后重试")
			} else {
				slog.Error("GetMetadata: databases failed", "module", "query", "datasource_id", dsID, "type", ds.Type, "error", err, "elapsed", elapsed)
				Fail(c, CodeQueryError, "获取数据库列表失败: "+err.Error())
			}
			return
		}
		slog.Debug("GetMetadata: databases success", "module", "query", "datasource_id", dsID, "count", len(dbs), "elapsed", time.Since(startTime))

		// 写入服务端缓存
		if h.cacheService != nil {
			if data, marshalErr := json.Marshal(dbs); marshalErr == nil {
				if cacheErr := h.cacheService.SetMetadata(ctx, dsID, "databases", "", data); cacheErr != nil {
					slog.Debug("GetMetadata: failed to cache databases", "datasource_id", dsID, "error", cacheErr)
				}
			}
		}

		Success(c, dbs)
	case "tables":
		// 尝试从服务端缓存获取
		if h.cacheService != nil && database != "" {
			cachedData, hit, cacheErr := h.cacheService.GetMetadata(ctx, dsID, "tables", database)
			if cacheErr != nil {
				slog.Debug("GetMetadata: tables cache lookup failed", "datasource_id", dsID, "database", database, "error", cacheErr)
			}
			if hit && cachedData != nil {
				var tables []driver.TableInfo
				if err := json.Unmarshal(cachedData, &tables); err == nil {
					slog.Debug("GetMetadata: tables cache hit", "datasource_id", dsID, "database", database, "count", len(tables), "elapsed", time.Since(startTime))
					Success(c, tables)
					return
				}
			}
		}

		tables, err := drv.GetTables(ctx, database)
		if err != nil {
			elapsed := time.Since(startTime)
			if isPoolBusyError(err) {
				slog.Warn("GetMetadata: tables pool busy", "module", "query", "datasource_id", dsID, "database", database, "elapsed", elapsed)
				Fail(c, CodeConcurrentLimit, "数据源连接池已满，请稍后重试")
			} else if ctx.Err() == context.DeadlineExceeded {
				slog.Warn("GetMetadata: tables timeout", "module", "query", "datasource_id", dsID, "database", database, "elapsed", elapsed)
				Fail(c, CodeQueryError, "获取数据表列表超时，请稍后重试")
			} else {
				slog.Error("GetMetadata: tables failed", "module", "query", "datasource_id", dsID, "database", database, "error", err, "elapsed", elapsed)
				Fail(c, CodeQueryError, "获取数据表列表失败: "+err.Error())
			}
			return
		}
		slog.Debug("GetMetadata: tables success", "module", "query", "datasource_id", dsID, "database", database, "count", len(tables), "elapsed", time.Since(startTime))

		// 写入服务端缓存
		if h.cacheService != nil && database != "" {
			if data, marshalErr := json.Marshal(tables); marshalErr == nil {
				if cacheErr := h.cacheService.SetMetadata(ctx, dsID, "tables", database, data); cacheErr != nil {
					slog.Debug("GetMetadata: failed to cache tables", "datasource_id", dsID, "database", database, "error", cacheErr)
				}
			}
		}

		Success(c, tables)
	case "columns":
		// 尝试从服务端缓存获取
		if h.cacheService != nil && database != "" && table != "" {
			cachedData, hit, cacheErr := h.cacheService.GetMetadata(ctx, dsID, "columns", database+"."+table)
			if cacheErr != nil {
				slog.Debug("GetMetadata: columns cache lookup failed", "datasource_id", dsID, "database", database, "table", table, "error", cacheErr)
			}
			if hit && cachedData != nil {
				var columns []driver.ColumnInfo
				if err := json.Unmarshal(cachedData, &columns); err == nil {
					slog.Debug("GetMetadata: columns cache hit", "datasource_id", dsID, "database", database, "table", table, "count", len(columns), "elapsed", time.Since(startTime))
					Success(c, columns)
					return
				}
			}
		}

		columns, err := drv.GetColumns(ctx, database, table)
		if err != nil {
			elapsed := time.Since(startTime)
			if isPoolBusyError(err) {
				slog.Warn("GetMetadata: columns pool busy", "module", "query", "datasource_id", dsID, "database", database, "table", table, "elapsed", elapsed)
				Fail(c, CodeConcurrentLimit, "数据源连接池已满，请稍后重试")
			} else if ctx.Err() == context.DeadlineExceeded {
				slog.Warn("GetMetadata: columns timeout", "module", "query", "datasource_id", dsID, "database", database, "table", table, "elapsed", elapsed)
				Fail(c, CodeQueryError, "获取字段列表超时，请稍后重试")
			} else {
				slog.Error("GetMetadata: columns failed", "module", "query", "datasource_id", dsID, "database", database, "table", table, "error", err, "elapsed", elapsed)
				Fail(c, CodeQueryError, "获取字段列表失败: "+err.Error())
			}
			return
		}
		slog.Debug("GetMetadata: columns success", "module", "query", "datasource_id", dsID, "database", database, "table", table, "count", len(columns), "elapsed", time.Since(startTime))

		// 写入服务端缓存
		if h.cacheService != nil && database != "" && table != "" {
			if data, marshalErr := json.Marshal(columns); marshalErr == nil {
				if cacheErr := h.cacheService.SetMetadata(ctx, dsID, "columns", database+"."+table, data); cacheErr != nil {
					slog.Debug("GetMetadata: failed to cache columns", "datasource_id", dsID, "database", database, "table", table, "error", cacheErr)
				}
			}
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

	maxExportRows := 1000
	h.runtimeConfig.mu.RLock()
	maxExportRows = h.runtimeConfig.maxExportRows
	h.runtimeConfig.mu.RUnlock()

	// 安全兜底：配置未加载或为非正值时使用默认值，避免 ApplyLimitToSQL 不应用 LIMIT 导致全表导出
	if maxExportRows <= 0 {
		maxExportRows = 1000
	}

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

	// 注意：导出不复用普通查询的缓存。
	// 普通查询使用 datasource.default_limit 限制结果集，导出使用 datasource.max_export_rows，
	// 两者限制不同。缓存 key 不包含 limit，若复用会导致：
	//   1. 导出命中普通查询缓存时，结果被 defaultLimit 截断（maxExportRows 不生效）
	//   2. 导出结果写入缓存后，普通查询会拿到 maxExportRows 限制的结果（defaultLimit 失效）
	// 因此导出始终使用 maxExportRows 重新执行查询，保证系统设置动态生效。
	queryTimeout := 60
	h.runtimeConfig.mu.RLock()
	if qt := h.runtimeConfig.queryTimeout; qt > 0 {
		queryTimeout = qt
	}
	h.runtimeConfig.mu.RUnlock()

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(queryTimeout)*time.Second)
	defer cancel()

	drv, err := h.manager.GetDriver(ctx, ds)
	if err != nil {
		Fail(c, CodeDatasourceNotFound, "连接数据源失败，请检查数据源配置")
		return
	}

	// 应用导出行数限制到SQL层面
	querySQL := driver.ApplyLimitToSQL(req.SQL, maxExportRows, ds.Type)
	result, err := drv.QueryWithDB(ctx, querySQL, req.Database)
	if err != nil {
		Fail(c, CodeQueryError, fmt.Sprintf("查询执行失败: %v", err))
		return
	}

	if result == nil {
		Fail(c, CodeQueryError, "查询返回空结果，请检查SQL语句或数据源连接")
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=query_result.csv")
	c.Header("Transfer-Encoding", "chunked")

	// 写入 BOM 头
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	c.Writer.Flush()

	writer := csv.NewWriter(c.Writer)
	_ = writer.Write(result.Columns)
	writer.Flush()
	c.Writer.Flush()

	// 分批写入数据行，每 1000 行刷新一次，避免大内存分配
	const batchSize = 1000
	for i, row := range result.Rows {
		record := make([]string, len(row))
		for j, v := range row {
			if v == nil {
				record[j] = ""
			} else {
				record[j] = fmt.Sprintf("%v", v)
			}
		}
		_ = writer.Write(record)
		if (i+1)%batchSize == 0 {
			writer.Flush()
			c.Writer.Flush()
		}
	}
	writer.Flush()
	c.Writer.Flush()
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

	var datasourceID int64
	if dsID := c.Query("datasource_id"); dsID != "" {
		datasourceID, _ = strconv.ParseInt(dsID, 10, 64)
	}
	var executedBy int64
	if uid := c.Query("executed_by"); uid != "" {
		executedBy, _ = strconv.ParseInt(uid, 10, 64)
	}
	filterStatus := c.Query("status")
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")
	search := c.Query("search")

	histories, total, err := h.dsService.GetQueryHistory(c.Request.Context(), queryDomainID, page, pageSize, datasourceID, filterStatus, startTime, endTime, executedBy, search)
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

	search := c.Query("search")
	savedList, total, err := h.dsService.GetSavedSQL(c.Request.Context(), queryDomainID, listUID, page, pageSize, search)
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

func (h *QueryHandler) UpdateSavedSQL(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的SQL记录ID")
		return
	}

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
	isPublic := false
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}

	var updateUID int64
	if v, ok := userID.(int64); ok {
		updateUID = v
	}

	saved := &model.SavedSQL{
		Name:         req.Name,
		DatasourceID: req.DatasourceID,
		SQLText:      req.SQLText,
		Description:  req.Description,
		UpdatedBy:    int64Ptr(updateUID),
		IsPublic:     isPublic,
	}

	if err := h.dsService.UpdateSavedSQL(c.Request.Context(), id, saved); err != nil {
		Fail(c, CodeSavedSQLNotFound, "更新SQL记录失败")
		return
	}

	Success(c, nil)
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

func (h *QueryHandler) GetResult(c *gin.Context) {
	queryID := c.Param("query_id")
	if queryID == "" {
		BadRequest(c, "query_id不能为空")
		return
	}

	q, ok := h.registry.Get(queryID)
	if !ok {
		NotFound(c, "查询不存在或已过期")
		return
	}

	resp := gin.H{
		"query_id": q.QueryID,
		"status":   string(q.Status),
	}

	switch q.Status {
	case QueryStatusCompleted:
		if q.Result != nil {
			resp["columns"] = q.Result.Columns
			resp["rows"] = q.Result.Rows
			resp["row_count"] = q.Result.RowCount
		} else {
			resp["columns"] = []string{}
			resp["rows"] = [][]interface{}{}
			resp["row_count"] = 0
		}
		resp["execution_time"] = q.ExecutionTime
		resp["from_cache"] = q.FromCache
	case QueryStatusFailed:
		resp["error"] = q.Error
		resp["execution_time"] = q.ExecutionTime
	case QueryStatusCancelled:
		resp["error"] = q.Error
		resp["execution_time"] = q.ExecutionTime
	}

	Success(c, resp)
}

// StreamResult SSE 推送查询结果，替代轮询
func (h *QueryHandler) StreamResult(c *gin.Context) {
	queryID := c.Param("query_id")
	if queryID == "" {
		BadRequest(c, "query_id不能为空")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// 先检查查询是否已经完成
	q, ok := h.registry.Get(queryID)
	if !ok {
		c.SSEvent("error", gin.H{"query_id": queryID, "error": "查询不存在或已过期"})
		c.Writer.Flush()
		return
	}

	// 如果查询已经终态，直接推送结果
	if q.Status == QueryStatusCompleted || q.Status == QueryStatusFailed || q.Status == QueryStatusCancelled {
		h.sendSSEEvent(c, q)
		c.Writer.Flush()
		return
	}

	// 注册观察者等待状态变更
	ctx := c.Request.Context()
	eventCh := make(chan *RunningQuery, 10)

	observer := &sseQueryObserver{queryID: queryID, ch: eventCh}
	h.registry.RegisterObserver(observer)
	defer h.registry.UnregisterObserver(observer)

	for {
		select {
		case <-ctx.Done():
			return
		case q := <-eventCh:
			h.sendSSEEvent(c, q)
			c.Writer.Flush()
			// 终态后关闭连接
			if q.Status == QueryStatusCompleted || q.Status == QueryStatusFailed || q.Status == QueryStatusCancelled {
				return
			}
		}
	}
}

// sendSSEEvent 发送 SSE 事件
func (h *QueryHandler) sendSSEEvent(c *gin.Context, q *RunningQuery) {
	data := gin.H{
		"query_id": q.QueryID,
		"status":   string(q.Status),
	}

	switch q.Status {
	case QueryStatusCompleted:
		if q.Result != nil {
			data["columns"] = q.Result.Columns
			data["rows"] = q.Result.Rows
			data["row_count"] = q.Result.RowCount
		} else {
			data["columns"] = []string{}
			data["rows"] = [][]interface{}{}
			data["row_count"] = 0
		}
		data["execution_time"] = q.ExecutionTime
		data["from_cache"] = q.FromCache
	case QueryStatusFailed:
		data["error"] = q.Error
		data["execution_time"] = q.ExecutionTime
	case QueryStatusCancelled:
		data["error"] = q.Error
		data["execution_time"] = q.ExecutionTime
	}

	c.SSEvent("query_update", data)
}

// sseQueryObserver 实现 QueryObserver 接口，通过 channel 传递事件
type sseQueryObserver struct {
	queryID string
	ch      chan *RunningQuery
}

func (o *sseQueryObserver) OnQueryUpdate(queryID string, query *RunningQuery) {
	if queryID == o.queryID {
		select {
		case o.ch <- query:
		default:
			// channel 满则丢弃（SSE 客户端可能已断开）
		}
	}
}

func (h *QueryHandler) Cancel(c *gin.Context) {
	queryID := c.Param("query_id")
	if queryID == "" {
		BadRequest(c, "query_id不能为空")
		return
	}

	cancelled := h.registry.Cancel(queryID)
	if !cancelled {
		q, ok := h.registry.Get(queryID)
		if !ok {
			Fail(c, CodeQueryError, "查询不存在或已过期")
			return
		}
		if q.Status == QueryStatusCompleted {
			Fail(c, CodeQueryError, "查询已完成，无法取消")
			return
		}
		if q.Status == QueryStatusFailed {
			Fail(c, CodeQueryError, "查询已失败，无需取消")
			return
		}
		if q.Status == QueryStatusCancelled {
			Fail(c, CodeQueryError, "查询已取消")
			return
		}
		Fail(c, CodeQueryError, "取消查询失败")
		return
	}

	slog.Info("query cancel signal sent", "query_id", queryID)
	Success(c, nil)
}

// GetPoolStats 获取数据源连接池统计信息
func (h *QueryHandler) GetPoolStats(c *gin.Context) {
	dsID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的数据源ID")
		return
	}

	ds, err := h.dsService.GetByID(c.Request.Context(), dsID)
	if err != nil {
		NotFound(c, "数据源不存在")
		return
	}

	drv, err := h.manager.GetDriver(c.Request.Context(), ds)
	if err != nil {
		slog.Error("failed to get datasource driver for pool stats", "datasource_id", dsID, "error", err)
		Fail(c, CodeDatasourceConnectFailed, "连接数据源失败，无法获取连接池信息")
		return
	}

	poolUpdater, ok := drv.(driver.PoolConfigUpdater)
	if !ok {
		Success(c, gin.H{
			"datasource_id": dsID,
			"has_pool":      false,
			"message":       "该数据源类型不支持连接池统计",
		})
		return
	}

	openCount, idleCount, inUse, maxOpen := poolUpdater.GetPoolStats()
	poolCfg := poolUpdater.GetPoolConfig()

	// max_idle: database/sql 驱动 MaxIdleConns=MaxOpen（最大化复用），
	// Hive/Kyuubi/Spark 自定义池 MinIdle 为常驻连接数
	maxIdle := poolCfg.MaxOpen // database/sql 默认
	if poolCfg.MinIdle > 0 && poolCfg.MinIdle < poolCfg.MaxOpen {
		maxIdle = poolCfg.MinIdle // Hive/Kyuubi/Spark 自定义池
	}

	Success(c, gin.H{
		"datasource_id": dsID,
		"has_pool":      true,
		"pool_stats": gin.H{
			"open_count": openCount,
			"idle_count": idleCount,
			"in_use":     inUse,
			"max_open":   maxOpen,
		},
		"pool_config": gin.H{
			"max_open":     poolCfg.MaxOpen,
			"max_idle":     maxIdle,
			"max_lifetime": int(poolCfg.MaxLifetime.Seconds()),
		},
	})
}

// ClearCache 清除指定数据源的查询缓存和元数据缓存
func (h *QueryHandler) ClearCache(c *gin.Context) {
	dsID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "无效的数据源ID")
		return
	}

	ds, err := h.dsService.GetByID(c.Request.Context(), dsID)
	if err != nil {
		NotFound(c, "数据源不存在")
		return
	}

	if h.cacheService == nil {
		Success(c, gin.H{"datasource_id": dsID, "message": "缓存服务未启用"})
		return
	}

	// 清除查询结果缓存
	if err := h.cacheService.Invalidate(c.Request.Context(), dsID); err != nil {
		slog.Error("failed to invalidate query cache", "datasource_id", dsID, "datasource_name", ds.Name, "error", err)
		Fail(c, CodeQueryError, "清除查询缓存失败")
		return
	}

	// 清除元数据缓存
	if err := h.cacheService.InvalidateMetadata(c.Request.Context(), dsID); err != nil {
		slog.Error("failed to invalidate metadata cache", "datasource_id", dsID, "datasource_name", ds.Name, "error", err)
		Fail(c, CodeQueryError, "清除元数据缓存失败")
		return
	}

	slog.Info("cache cleared for datasource", "datasource_id", dsID, "datasource_name", ds.Name)
	Success(c, gin.H{"datasource_id": dsID, "message": "缓存已清除"})
}

// isPoolBusyError 判断错误是否为连接池暂时繁忙（非连接断开）
func isPoolBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "pool fully occupied")
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
