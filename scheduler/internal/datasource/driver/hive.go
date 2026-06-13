package driver

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	gohive "github.com/beltran/gohive"
	"github.com/pkg/errors"
)

// HiveDriver 使用连接池支持多用户并发查询。
// 每个查询从池中获取独立连接，设置 database context 后执行，完成后归还。
// 参考 Superset 的 SQLAlchemy 连接池设计，解决以下问题：
// 1. 单连接+互斥锁导致多用户查询串行执行
// 2. UseDatabase 修改共享连接状态导致不同用户 database context 混乱
// 3. 长查询阻塞其他用户的查询和 metadata 获取
type HiveDriver struct {
	pool      *hiveConnPool
	config    DatasourceConfig
	unhealthy atomic.Bool
	defaultDB string // 默认 database（从配置中获取）
}

func NewHiveDriver() Driver {
	return &HiveDriver{}
}

// createHiveConnection 创建新的 Hive 连接（用于连接池）
func (d *HiveDriver) createHiveConnection(ctx context.Context) (*gohive.Connection, error) {
	port := d.config.Port
	if port == 0 {
		port = 10000
	}

	configuration := gohive.NewConnectConfiguration()
	configuration.Username = d.config.Username
	configuration.Password = d.config.Password
	configuration.Service = "hive"
	configuration.TransportMode = "binary"
	if d.config.Database != "" {
		configuration.Database = d.config.Database
	}
	if d.config.ZookeeperNamespace != "" {
		configuration.ZookeeperNamespace = d.config.ZookeeperNamespace
	}

	auth := "NONE"
	if d.config.AuthType == "ldap" {
		auth = "LDAP"
	}

	type connectResult struct {
		conn *gohive.Connection
		err  error
	}
	resultCh := make(chan connectResult, 1)

	go func() {
		var connection *gohive.Connection
		var err error
		if d.config.ConnectionMode == "zookeeper" && d.config.ZookeeperQuorum != "" {
			connection, err = gohive.ConnectZookeeper(d.config.ZookeeperQuorum, auth, configuration)
		} else {
			connection, err = gohive.Connect(d.config.Host, port, auth, configuration)
		}
		resultCh <- connectResult{conn: connection, err: err}
	}()

	select {
	case <-ctx.Done():
		go func() {
			if res := <-resultCh; res.conn != nil {
				res.conn.Close()
			}
		}()
		return nil, errors.Wrap(ctx.Err(), "hive connect cancelled")
	case result := <-resultCh:
		if result.err != nil {
			return nil, errors.Wrap(result.err, "failed to connect to hive")
		}
		return result.conn, nil
	}
}

func (d *HiveDriver) Connect(ctx context.Context, config DatasourceConfig) error {
	d.config = config
	d.defaultDB = config.Database

	port := config.Port
	if port == 0 {
		port = 10000
	}

	slog.Debug("hive connecting", "host", config.Host, "port", port, "database", config.Database, "auth_type", config.AuthType, "mode", config.ConnectionMode)

	// 创建连接池配置
	poolCfg := DefaultPoolConfig()
	if cfg, ok := config.Config["hive_pool_size"]; ok {
		if size, ok := cfg.(float64); ok && size > 0 {
			poolCfg.MaxOpen = int(size)
		}
	}
	if cfg, ok := config.Config["hive_pool_min_idle"]; ok {
		if size, ok := cfg.(float64); ok && size >= 0 {
			poolCfg.MinIdle = int(size)
		}
	}
	if cfg, ok := config.Config["hive_pool_max_lifetime"]; ok {
		if seconds, ok := cfg.(float64); ok && seconds > 0 {
			poolCfg.MaxLifetime = time.Duration(seconds) * time.Second
		}
	}
	d.pool = newHiveConnPool(poolCfg, d.createHiveConnection)

	// 预热：创建初始连接放入池中
	initialConn, err := d.createHiveConnection(ctx)
	if err != nil {
		slog.Error("hive initial connection failed", "host", config.Host, "port", port, "error", err)
		return errors.Wrap(err, "failed to connect to hive")
	}
	d.pool.put(initialConn, d.defaultDB)

	// 预热额外的 MinIdle-1 个连接
	cfg := d.pool.GetConfig()
	for i := 1; i < cfg.MinIdle; i++ {
		conn, connErr := d.createHiveConnection(ctx)
		if connErr != nil {
			slog.Warn("hive pre-warm connection failed", "index", i, "error", connErr)
			break
		}
		d.pool.put(conn, d.defaultDB)
	}

	slog.Info("hive connected, pool initialized", "host", config.Host, "port", port, "database", config.Database, "pool_config", fmt.Sprintf("max=%d min_idle=%d max_lifetime=%v", cfg.MaxOpen, cfg.MinIdle, cfg.MaxLifetime))

	return nil
}

func (d *HiveDriver) TestConnection(ctx context.Context) error {
	if d.pool == nil {
		return errors.New("hive connection pool not initialized")
	}
	pc, err := d.pool.acquire(ctx)
	if err != nil {
		return errors.Wrap(err, "hive test connection failed")
	}
	defer d.pool.release(pc)

	cursor := pc.conn.Cursor()
	cursor.Exec(ctx, normalizeSQL("SELECT 1"))
	if cursor.Err != nil {
		execErr := cursor.Err
		cursor.Close()
		return extractGohiveError(execErr, "hive test connection failed")
	}
	cursor.Close()
	return nil
}

func (d *HiveDriver) Ping(ctx context.Context) error {
	if d.unhealthy.Load() {
		return errors.New("hive connection marked as unhealthy")
	}
	if d.pool == nil {
		return errors.New("hive connection pool not initialized")
	}

	pc, err := d.pool.acquireWithTimeout(ctx, 5*time.Second)
	if err != nil {
		d.unhealthy.Store(true)
		return errors.Wrap(err, "hive ping failed, cannot acquire connection")
	}
	defer d.pool.release(pc)
	return nil
}

func (d *HiveDriver) IsUnhealthy() bool {
	return d.unhealthy.Load()
}

// UpdatePoolConfig 动态更新连接池配置
func (d *HiveDriver) UpdatePoolConfig(cfg PoolConfig) {
	if d.pool != nil {
		d.pool.UpdateConfig(cfg)
	}
}

// GetPoolConfig 获取当前连接池配置
func (d *HiveDriver) GetPoolConfig() PoolConfig {
	if d.pool != nil {
		return d.pool.GetConfig()
	}
	return DefaultPoolConfig()
}

// GetPoolStats 获取连接池统计信息
func (d *HiveDriver) GetPoolStats() (openCount int, idleCount int, inUse int, maxOpen int) {
	if d.pool != nil {
		return d.pool.stats()
	}
	return 0, 0, 0, 0
}

func (d *HiveDriver) Close() error {
	if d.pool != nil {
		d.pool.close()
	}
	return nil
}

// Query 执行查询（使用默认 database context）。
// 向后兼容，推荐使用 QueryWithDB。
func (d *HiveDriver) Query(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	return d.QueryWithDB(ctx, query, d.defaultDB)
}

// QueryWithDB 在指定 database context 下执行查询。
// 从连接池获取独立连接，设置 database context，执行查询，归还连接。
// 不同用户的查询互不阻塞，database context 完全隔离。
func (d *HiveDriver) QueryWithDB(ctx context.Context, query string, database string) (*QueryResult, error) {
	if d.pool == nil {
		return nil, errors.New("hive connection pool not initialized")
	}

	normalizedQuery := normalizeSQL(query)
	slog.Debug("hive executing query", "sql_preview", truncateSQL(normalizedQuery, 200), "database", database)

	// 从连接池获取连接
	pc, err := d.pool.acquire(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "hive acquire connection failed")
	}

	// 设置 database context
	if database != "" {
		if useErr := pc.ensureDatabase(ctx, database); useErr != nil {
			d.pool.discard(pc)
			return nil, errors.Wrap(useErr, "hive switch database failed")
		}
	}

	// 设置服务端查询超时（双重保障）
	if _, timeoutSQL := extractQueryTimeout(ctx, "SET hive.execution.engine.query.timeout=", 5*time.Second); timeoutSQL != "" {
		pc.setQueryTimeout(ctx, timeoutSQL)
	}

	// 执行查询
	type queryResult struct {
		result *QueryResult
		err    error
	}
	resultCh := make(chan queryResult, 1)
	var queryCursor *gohive.Cursor

	go func() {
		cursor := pc.conn.Cursor()
		queryCursor = cursor
		cursor.Exec(context.Background(), normalizedQuery)
		if cursor.Err != nil {
			execErr := cursor.Err
			cursor.Close()
			resultCh <- queryResult{result: nil, err: extractGohiveError(execErr, "hive query error")}
			return
		}

		description := cursor.Description()
		if cursor.Err != nil {
			descErr := cursor.Err
			cursor.Close()
			resultCh <- queryResult{nil, extractGohiveError(descErr, "hive get description error")}
			return
		}

		var columns []string
		for _, col := range description {
			if len(col) > 0 {
				columns = append(columns, col[0])
			}
		}

		if len(columns) == 0 {
			cursor.Close()
			resultCh <- queryResult{nil, errors.New("hive query returned no columns, the SQL may contain errors or the table does not exist")}
			return
		}

		var rows [][]interface{}
		for cursor.HasMore(context.Background()) {
			rowMap := cursor.RowMap(context.Background())
			if cursor.Err != nil {
				fetchErr := cursor.Err
				cursor.Close()
				resultCh <- queryResult{nil, extractGohiveError(fetchErr, "hive fetch error")}
				return
			}
			row := make([]interface{}, len(columns))
			for i, col := range columns {
				row[i] = rowMap[col]
			}
			rows = append(rows, row)
		}
		if cursor.Err != nil {
			finishErr := cursor.Err
			cursor.Close()
			resultCh <- queryResult{nil, extractGohiveError(finishErr, "hive query error")}
			return
		}
		cursor.Close()

		resultCh <- queryResult{&QueryResult{
			Columns:  columns,
			Rows:     rows,
			RowCount: int64(len(rows)),
		}, nil}
	}()

	select {
	case <-ctx.Done():
		slog.Warn("hive query cancelled by context, sending CancelOperation to Hive Server", "sql_preview", truncateSQL(normalizedQuery, 200), "error", ctx.Err())
		if queryCursor != nil {
			queryCursor.Cancel()
			queryCursor.Close()
		}
		go func() { <-resultCh }()
		// 取消后归还连接（连接可能处于不确定状态，丢弃）
		d.pool.discard(pc)
		return nil, errors.Wrap(ctx.Err(), "hive query cancelled")
	case res := <-resultCh:
		if res.err != nil {
			if isConnectionError(res.err) {
				d.pool.discard(pc)
				d.unhealthy.Store(true)
				slog.Warn("hive connection error detected, discarded from pool", "sql_preview", truncateSQL(normalizedQuery, 200), "error", res.err)
			} else {
				d.pool.release(pc)
			}
			slog.Error("hive query execution failed", "sql_preview", truncateSQL(normalizedQuery, 200), "error", res.err)
		} else {
			d.pool.release(pc)
		}
		return res.result, res.err
	}
}

func (d *HiveDriver) GetDatabases(ctx context.Context) ([]string, error) {
	result, err := d.Query(ctx, normalizeSQL("SHOW DATABASES"))
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("hive query returned nil result for SHOW DATABASES")
	}
	var databases []string
	for _, row := range result.Rows {
		if len(row) > 0 {
			databases = append(databases, fmt.Sprintf("%v", row[0]))
		}
	}
	return databases, nil
}

func (d *HiveDriver) GetTables(ctx context.Context, database string) ([]TableInfo, error) {
	if database == "" {
		database = d.config.Database
	}
	result, err := d.Query(ctx, fmt.Sprintf("SHOW TABLES IN %s", escapeHiveIdentifier(database)))
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("hive query returned nil result for SHOW TABLES")
	}
	var tables []TableInfo
	for _, row := range result.Rows {
		if len(row) > 0 {
			tables = append(tables, TableInfo{Name: fmt.Sprintf("%v", row[0])})
		}
	}
	return tables, nil
}

func (d *HiveDriver) GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error) {
	if database == "" {
		database = d.config.Database
	}
	result, err := d.Query(ctx, fmt.Sprintf("DESCRIBE %s.%s", escapeHiveIdentifier(database), escapeHiveIdentifier(table)))
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("hive query returned nil result for DESCRIBE")
	}
	var columns []ColumnInfo
	for _, row := range result.Rows {
		if len(row) >= 2 {
			columns = append(columns, ColumnInfo{
				Name: fmt.Sprintf("%v", row[0]),
				Type: fmt.Sprintf("%v", row[1]),
			})
		}
	}
	return columns, nil
}

func (d *HiveDriver) SupportsCancel() bool {
	return true
}

// UseDatabase 保留向后兼容，但不再推荐使用。
// 在连接池架构下，UseDatabase 只设置默认 database，
// 实际查询时应使用 QueryWithDB 指定 database。
func (d *HiveDriver) UseDatabase(ctx context.Context, database string) error {
	if database == "" {
		return nil
	}
	// 仅更新默认 database，不修改任何连接状态
	d.defaultDB = database
	slog.Debug("hive default database updated", "database", database)
	return nil
}

func escapeHiveIdentifier(name string) string {
	return strings.ReplaceAll(name, "`", "``")
}
