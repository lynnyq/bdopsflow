package driver

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	_ "github.com/trinodb/trino-go-client/trino"
)

type TrinoDriver struct {
	db     *sql.DB
	config DatasourceConfig
}

func NewTrinoDriver() Driver {
	return &TrinoDriver{}
}

func (d *TrinoDriver) Connect(ctx context.Context, config DatasourceConfig) error {
	d.config = config
	port := config.Port
	if port == 0 {
		port = 8080
	}

	dsn := d.buildDSN(port)
	slog.Debug("trino connecting", "host", config.Host, "port", port, "catalog", config.Database)
	db, err := sql.Open("trino", dsn)
	if err != nil {
		slog.Error("trino connection failed", "host", config.Host, "port", port, "error", err)
		return fmt.Errorf("failed to open trino connection: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		slog.Error("trino connection failed", "host", config.Host, "port", port, "error", err)
		return fmt.Errorf("failed to ping trino: %w", err)
	}
	d.db = db
	slog.Info("trino connected", "host", config.Host, "port", port, "catalog", config.Database)
	return nil
}

func (d *TrinoDriver) TestConnection(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("trino connection not established")
	}
	return d.db.PingContext(ctx)
}

func (d *TrinoDriver) Ping(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("trino connection not established")
	}
	return d.db.PingContext(ctx)
}

func (d *TrinoDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *TrinoDriver) Query(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	if d.db == nil {
		return nil, fmt.Errorf("trino connection not established")
	}
	slog.Debug("trino executing query", "sql_preview", truncateSQL(query, 200))
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		slog.Error("trino query execution failed", "sql_preview", truncateSQL(query, 200), "error", err)
		return nil, fmt.Errorf("trino query error: %w", err)
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
		for i, v := range values {
			row[i] = convertTrinoValue(v)
		}
		resultRows = append(resultRows, row)
	}

	return &QueryResult{
		Columns:  columns,
		Rows:     resultRows,
		RowCount: int64(len(resultRows)),
	}, nil
}

func (d *TrinoDriver) GetDatabases(ctx context.Context) ([]string, error) {
	result, err := d.Query(ctx, "SHOW CATALOGS")
	if err != nil {
		return nil, err
	}
	var databases []string
	for _, row := range result.Rows {
		if len(row) > 0 {
			databases = append(databases, fmt.Sprintf("%v", row[0]))
		}
	}
	return databases, nil
}

func (d *TrinoDriver) GetTables(ctx context.Context, database string) ([]TableInfo, error) {
	if database == "" {
		database = d.config.Database
	}
	result, err := d.Query(ctx, fmt.Sprintf("SHOW TABLES FROM %s", escapeTrinoIdentifier(database)))
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

func (d *TrinoDriver) GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error) {
	if database == "" {
		database = d.config.Database
	}
	result, err := d.Query(ctx, fmt.Sprintf("SHOW COLUMNS FROM %s.%s", escapeTrinoIdentifier(database), escapeTrinoIdentifier(table)))
	if err != nil {
		return nil, err
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

func (d *TrinoDriver) SupportsCancel() bool {
	return true
}

func (d *TrinoDriver) UseDatabase(ctx context.Context, database string) error {
	if database == "" {
		return nil
	}
	if d.db == nil {
		return fmt.Errorf("trino connection not established")
	}

	parts := strings.SplitN(database, ".", 2)
	if len(parts) == 2 {
		_, err := d.db.ExecContext(ctx, fmt.Sprintf("USE %s.%s", escapeTrinoIdentifier(parts[0]), escapeTrinoIdentifier(parts[1])))
		if err != nil {
			return fmt.Errorf("trino use database error: %w", err)
		}
	} else if d.config.Database != "" {
		catalog := d.config.Database
		if idx := strings.Index(catalog, "."); idx >= 0 {
			catalog = catalog[:idx]
		}
		_, err := d.db.ExecContext(ctx, fmt.Sprintf("USE %s.%s", escapeTrinoIdentifier(catalog), escapeTrinoIdentifier(database)))
		if err != nil {
			return fmt.Errorf("trino use database error: %w", err)
		}
	} else {
		_, err := d.db.ExecContext(ctx, fmt.Sprintf("USE %s", escapeTrinoIdentifier(database)))
		if err != nil {
			return fmt.Errorf("trino use database error: %w", err)
		}
	}
	slog.Debug("trino switched database", "database", database)
	return nil
}

func (d *TrinoDriver) buildDSN(port int) string {
	scheme := "http"
	if _, ok := d.config.Config["ssl"]; ok {
		scheme = "https"
	}

	u := &url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s:%d", d.config.Host, port),
	}

	if d.config.Username != "" {
		if d.config.Password != "" {
			u.User = url.UserPassword(d.config.Username, d.config.Password)
		} else {
			u.User = url.User(d.config.Username)
		}
	}

	q := u.Query()
	if d.config.Database != "" {
		parts := strings.SplitN(d.config.Database, ".", 2)
		q.Set("catalog", parts[0])
		if len(parts) > 1 {
			q.Set("schema", parts[1])
		}
	}
	if d.config.AuthType == "ldap" {
		q.Set("SSL", "true")
	}
	u.RawQuery = q.Encode()

	return u.String()
}

func convertTrinoValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	default:
		return val
	}
}

func escapeTrinoIdentifier(name string) string {
	return strings.ReplaceAll(name, `"`, `""`)
}
