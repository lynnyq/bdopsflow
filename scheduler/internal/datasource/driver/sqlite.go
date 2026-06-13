package driver

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteDriver struct {
	db     *sql.DB
	config DatasourceConfig
}

func NewSQLiteDriver() Driver {
	return &SQLiteDriver{}
}

func (d *SQLiteDriver) Connect(ctx context.Context, config DatasourceConfig) error {
	d.config = config
	if config.Path == "" {
		return fmt.Errorf("sqlite path is required")
	}
	absPath, err := filepath.Abs(config.Path)
	if err != nil {
		return fmt.Errorf("failed to resolve sqlite path: %w", err)
	}
	db, err := sql.Open("sqlite3", absPath+"?mode=ro")
	if err != nil {
		return fmt.Errorf("failed to open sqlite: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping sqlite: %w", err)
	}
	d.db = db
	return nil
}

func (d *SQLiteDriver) TestConnection(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("sqlite connection not established")
	}
	return d.db.PingContext(ctx)
}

func (d *SQLiteDriver) Ping(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("sqlite connection not established")
	}
	return d.db.PingContext(ctx)
}

func (d *SQLiteDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *SQLiteDriver) Query(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	if d.db == nil {
		return nil, fmt.Errorf("sqlite connection not established")
	}
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite query error: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var resultRows [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		row := make([]interface{}, len(columns))
		copy(row, values)
		resultRows = append(resultRows, row)
	}

	return &QueryResult{
		Columns:  columns,
		Rows:     resultRows,
		RowCount: int64(len(resultRows)),
	}, nil
}

func (d *SQLiteDriver) GetDatabases(ctx context.Context) ([]string, error) {
	return []string{d.config.Path}, nil
}

func (d *SQLiteDriver) GetTables(ctx context.Context, database string) ([]TableInfo, error) {
	result, err := d.Query(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	var tables []TableInfo
	for _, row := range result.Rows {
		if len(row) > 0 {
			tables = append(tables, TableInfo{Name: fmt.Sprintf("%v", row[0])})
		}
	}
	return tables, nil
}

func (d *SQLiteDriver) GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", escapeSQLiteIdentifier(table))
	result, err := d.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	var columns []ColumnInfo
	for _, row := range result.Rows {
		if len(row) >= 6 {
			columns = append(columns, ColumnInfo{
				Name:     fmt.Sprintf("%v", row[1]),
				Type:     fmt.Sprintf("%v", row[2]),
				Nullable: fmt.Sprintf("%v", row[3]) == "0",
			})
		}
	}
	return columns, nil
}

func (d *SQLiteDriver) SupportsCancel() bool {
	return true
}

func (d *SQLiteDriver) QueryWithDB(ctx context.Context, query string, database string) (*QueryResult, error) {
	return d.Query(ctx, query)
}

func (d *SQLiteDriver) UseDatabase(ctx context.Context, database string) error {
	return nil
}

func escapeSQLiteIdentifier(name string) string {
	return strings.ReplaceAll(name, `"`, `""`)
}

// UpdatePoolConfig 动态更新连接池配置
func (d *SQLiteDriver) UpdatePoolConfig(cfg PoolConfig) {
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
func (d *SQLiteDriver) GetPoolConfig() PoolConfig {
	cfg := DefaultPoolConfig()
	if d.db != nil {
		stats := d.db.Stats()
		cfg.MaxOpen = stats.MaxOpenConnections
	}
	return cfg
}

// GetPoolStats 获取连接池统计信息
func (d *SQLiteDriver) GetPoolStats() (openCount int, idleCount int, inUse int, maxOpen int) {
	if d.db == nil {
		return 0, 0, 0, 0
	}
	stats := d.db.Stats()
	return stats.OpenConnections, stats.Idle, stats.InUse, stats.MaxOpenConnections
}
