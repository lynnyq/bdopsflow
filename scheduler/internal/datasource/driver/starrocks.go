package driver

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type StarRocksDriver struct {
	db     *sql.DB
	config DatasourceConfig
}

func NewStarRocksDriver() Driver {
	return &StarRocksDriver{}
}

func (d *StarRocksDriver) Connect(ctx context.Context, config DatasourceConfig) error {
	d.config = config
	dsn := d.buildDSN()
	port := config.Port
	if port == 0 {
		port = 9030
	}
	slog.Debug("starrocks connecting", "host", config.Host, "port", port, "database", config.Database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		slog.Error("starrocks connection failed", "host", config.Host, "port", port, "error", err)
		return fmt.Errorf("failed to open starrocks connection: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		slog.Error("starrocks connection failed", "host", config.Host, "port", port, "error", err)
		return fmt.Errorf("failed to ping starrocks: %w", err)
	}
	d.db = db
	slog.Info("starrocks connected", "host", config.Host, "port", port, "database", config.Database)
	return nil
}

func (d *StarRocksDriver) TestConnection(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("starrocks connection not established")
	}
	return d.db.PingContext(ctx)
}

func (d *StarRocksDriver) Ping(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("starrocks connection not established")
	}
	return d.db.PingContext(ctx)
}

func (d *StarRocksDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *StarRocksDriver) Query(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	if d.db == nil {
		return nil, &DatasourceError{
			Err:            fmt.Errorf("starrocks connection not established"),
			Category:       ErrCategoryConnection,
			DatasourceType: "starrocks",
			Retryable:      false,
		}
	}
	slog.Debug("starrocks executing query", "sql_preview", truncateSQL(query, 200))
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		slog.Error("starrocks query execution failed", "sql_preview", truncateSQL(query, 200), "error", err)
		return nil, ClassifyError(err, "starrocks")
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, ClassifyError(err, "starrocks")
	}

	var resultRows [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, ClassifyError(err, "starrocks")
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

func (d *StarRocksDriver) GetDatabases(ctx context.Context) ([]string, error) {
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

func (d *StarRocksDriver) GetTables(ctx context.Context, database string) ([]TableInfo, error) {
	if database == "" {
		database = d.config.Database
	}
	query := fmt.Sprintf("SELECT TABLE_NAME, TABLE_COMMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA = '%s' ORDER BY TABLE_NAME", escapeSQLString(database))
	result, err := d.Query(ctx, query)
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

func (d *StarRocksDriver) GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error) {
	if database == "" {
		database = d.config.Database
	}
	query := fmt.Sprintf(
		"SELECT COLUMN_NAME, COLUMN_TYPE, COLUMN_COMMENT, IS_NULLABLE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s' ORDER BY ORDINAL_POSITION",
		escapeSQLString(database), escapeSQLString(table),
	)
	result, err := d.Query(ctx, query)
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

func (d *StarRocksDriver) SupportsCancel() bool {
	return true
}

func (d *StarRocksDriver) QueryWithDB(ctx context.Context, query string, database string) (*QueryResult, error) {
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

func (d *StarRocksDriver) UseDatabase(ctx context.Context, database string) error {
	if database == "" {
		return nil
	}
	if d.db == nil {
		return fmt.Errorf("starrocks connection not established")
	}
	_, err := d.db.ExecContext(ctx, fmt.Sprintf("USE %s", escapeMySQLIdentifier(database)))
	if err != nil {
		return fmt.Errorf("starrocks use database error: %w", err)
	}
	slog.Debug("starrocks switched database", "database", database)
	return nil
}

func (d *StarRocksDriver) buildDSN() string {
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
func (d *StarRocksDriver) UpdatePoolConfig(cfg PoolConfig) {
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
func (d *StarRocksDriver) GetPoolConfig() PoolConfig {
	cfg := DefaultPoolConfig()
	if d.db != nil {
		stats := d.db.Stats()
		cfg.MaxOpen = stats.MaxOpenConnections
	}
	return cfg
}

// GetPoolStats 获取连接池统计信息
func (d *StarRocksDriver) GetPoolStats() (openCount int, idleCount int, inUse int, maxOpen int) {
	if d.db == nil {
		return 0, 0, 0, 0
	}
	stats := d.db.Stats()
	return stats.OpenConnections, stats.Idle, stats.InUse, stats.MaxOpenConnections
}
