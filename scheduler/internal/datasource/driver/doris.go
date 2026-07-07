package driver

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type DorisDriver struct {
	db     *sql.DB
	config DatasourceConfig
}

func NewDorisDriver() Driver {
	return &DorisDriver{}
}

func (d *DorisDriver) Connect(ctx context.Context, config DatasourceConfig) error {
	d.config = config
	dsn := d.buildDSN()
	port := config.Port
	if port == 0 {
		port = 9030
	}
	slog.Debug("doris connecting", "host", config.Host, "port", port, "database", config.Database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		slog.Error("doris connection failed", "host", config.Host, "port", port, "error", err)
		return fmt.Errorf("failed to open doris connection: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		slog.Error("doris connection failed", "host", config.Host, "port", port, "error", err)
		return fmt.Errorf("failed to ping doris: %w", err)
	}
	d.db = db
	slog.Info("doris connected", "host", config.Host, "port", port, "database", config.Database)
	return nil
}

func (d *DorisDriver) TestConnection(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("doris connection not established")
	}
	return d.db.PingContext(ctx)
}

func (d *DorisDriver) Ping(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("doris connection not established")
	}
	return d.db.PingContext(ctx)
}

func (d *DorisDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *DorisDriver) Query(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	if d.db == nil {
		return nil, &DatasourceError{
			Err:            fmt.Errorf("doris connection not established"),
			Category:       ErrCategoryConnection,
			DatasourceType: "doris",
			Retryable:      false,
		}
	}
	slog.Debug("doris executing query", "sql_preview", truncateSQL(query, 200))
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		slog.Error("doris query execution failed", "sql_preview", truncateSQL(query, 200), "error", err)
		return nil, ClassifyError(err, "doris")
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, ClassifyError(err, "doris")
	}

	var resultRows [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, ClassifyError(err, "doris")
		}
		row := make([]interface{}, len(columns))
		for i, v := range values {
			row[i] = convertMySQLValue(v)
		}
		resultRows = append(resultRows, row)
	}

	return &QueryResult{
		Columns:  columns,
		Rows:     resultRows,
		RowCount: int64(len(resultRows)),
	}, nil
}

func (d *DorisDriver) GetDatabases(ctx context.Context) ([]string, error) {
	result, err := d.Query(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	var databases []string
	for _, row := range result.Rows {
		if len(row) > 0 {
			if s, ok := row[0].(string); ok {
				databases = append(databases, s)
			}
		}
	}
	return databases, nil
}

func (d *DorisDriver) GetTables(ctx context.Context, database string) ([]TableInfo, error) {
	if database == "" {
		database = d.config.Database
	}
	query := "SELECT TABLE_NAME, TABLE_COMMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? ORDER BY TABLE_NAME"
	result, err := d.Query(ctx, query, database)
	if err != nil {
		return nil, err
	}
	var tables []TableInfo
	for _, row := range result.Rows {
		if len(row) >= 1 {
			info := TableInfo{Name: fmt.Sprintf("%v", row[0])}
			if len(row) >= 2 {
				info.Comment = fmt.Sprintf("%v", row[1])
			}
			tables = append(tables, info)
		}
	}
	return tables, nil
}

func (d *DorisDriver) GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error) {
	if database == "" {
		database = d.config.Database
	}
	query := "SELECT COLUMN_NAME, COLUMN_TYPE, COLUMN_COMMENT, IS_NULLABLE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION"
	result, err := d.Query(ctx, query, database, table)
	if err != nil {
		return nil, err
	}
	var columns []ColumnInfo
	for _, row := range result.Rows {
		if len(row) >= 4 {
			columns = append(columns, ColumnInfo{
				Name:     fmt.Sprintf("%v", row[0]),
				Type:     fmt.Sprintf("%v", row[1]),
				Comment:  fmt.Sprintf("%v", row[2]),
				Nullable: fmt.Sprintf("%v", row[3]) == "YES",
			})
		}
	}
	return columns, nil
}

func (d *DorisDriver) SupportsCancel() bool {
	return true
}

func (d *DorisDriver) QueryWithDB(ctx context.Context, query string, database string) (*QueryResult, error) {
	if database == "" {
		return d.Query(ctx, query)
	}
	if err := d.UseDatabase(ctx, database); err != nil {
		return nil, err
	}
	result, err := d.Query(ctx, query)
	if d.config.Database != "" && d.config.Database != database {
		if restoreErr := d.UseDatabase(ctx, d.config.Database); restoreErr != nil {
			slog.Warn("failed to restore database after query", "database", d.config.Database, "error", restoreErr)
		}
	}
	return result, err
}

// TryQueryWithDB 非阻塞版本的 QueryWithDB。database/sql 内置连接池不会长时间阻塞，直接委托给 QueryWithDB。
func (d *DorisDriver) TryQueryWithDB(ctx context.Context, query string, database string) (*QueryResult, error) {
	return d.QueryWithDB(ctx, query, database)
}

func (d *DorisDriver) UseDatabase(ctx context.Context, database string) error {
	if database == "" {
		return nil
	}
	if d.db == nil {
		return fmt.Errorf("doris connection not established")
	}
	_, err := d.db.ExecContext(ctx, fmt.Sprintf("USE %s", escapeMySQLIdentifier(database)))
	if err != nil {
		return fmt.Errorf("doris use database error: %w", err)
	}
	slog.Debug("doris switched database", "database", database)
	return nil
}

func (d *DorisDriver) buildDSN() string {
	host := d.config.Host
	port := d.config.Port
	if port == 0 {
		port = 9030
	}
	user := d.config.Username
	pass := d.config.Password
	dbName := d.config.Database

	params := []string{"charset=utf8mb4", "parseTime=true", "loc=Local"}
	if ssl, ok := d.config.Config["ssl"].(bool); ok && ssl {
		params = append(params, "tls=true")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s", user, pass, host, port, dbName, strings.Join(params, "&"))
	return dsn
}

// UpdatePoolConfig 动态更新连接池配置
func (d *DorisDriver) UpdatePoolConfig(cfg PoolConfig) {
	if d.db == nil {
		return
	}
	d.db.SetMaxOpenConns(cfg.MaxOpen)
	d.db.SetMaxIdleConns(cfg.MaxOpen)
	if cfg.MaxLifetime > 0 {
		d.db.SetConnMaxLifetime(cfg.MaxLifetime)
	}
}

// GetPoolConfig 获取当前连接池配置
func (d *DorisDriver) GetPoolConfig() PoolConfig {
	cfg := DefaultPoolConfig()
	if d.db != nil {
		stats := d.db.Stats()
		cfg.MaxOpen = stats.MaxOpenConnections
	}
	return cfg
}

// GetPoolStats 获取连接池统计信息
func (d *DorisDriver) GetPoolStats() (openCount int, idleCount int, inUse int, maxOpen int) {
	if d.db == nil {
		return 0, 0, 0, 0
	}
	stats := d.db.Stats()
	return stats.OpenConnections, stats.Idle, stats.InUse, stats.MaxOpenConnections
}
