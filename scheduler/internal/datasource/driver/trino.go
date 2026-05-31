package driver

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	trino "github.com/trinodb/trino-go-client/trino"
	"github.com/pkg/errors"
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
	query = normalizeTrinoSQL(query)
	slog.Debug("trino executing query", "sql_preview", truncateSQL(query, 200))
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		slog.Error("trino query execution failed", "sql_preview", truncateSQL(query, 200), "error", err)
		var qf *trino.ErrQueryFailed
		if errors.As(err, &qf) && qf.Reason != nil {
			return nil, fmt.Errorf("trino query error: %s", strings.Trim(qf.Reason.Error(), `"`))
		}
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
	var catalogs []string
	if d.config.Database != "" {
		defaultCatalog := d.config.Database
		if idx := strings.Index(defaultCatalog, "."); idx >= 0 {
			defaultCatalog = defaultCatalog[:idx]
		}
		catalogs = []string{defaultCatalog}
	} else {
		result, err := d.Query(ctx, "SHOW CATALOGS")
		if err != nil {
			return nil, err
		}
		for _, row := range result.Rows {
			if len(row) > 0 {
				catalogs = append(catalogs, fmt.Sprintf("%v", row[0]))
			}
		}
	}

	var schemas []string
	for _, catalog := range catalogs {
		schemaResult, err := d.Query(ctx, fmt.Sprintf("SHOW SCHEMAS FROM %s", escapeTrinoIdentifier(catalog)))
		if err != nil {
			slog.Warn("trino failed to list schemas for catalog, skipping", "catalog", catalog, "error", err)
			continue
		}
		for _, row := range schemaResult.Rows {
			if len(row) > 0 {
				schemaName := fmt.Sprintf("%v", row[0])
				if schemaName == "information_schema" {
					continue
				}
				schemas = append(schemas, catalog+"."+schemaName)
			}
		}
	}
	return schemas, nil
}

func (d *TrinoDriver) GetTables(ctx context.Context, database string) ([]TableInfo, error) {
	if database == "" {
		database = d.config.Database
	}
	if database == "" {
		return nil, fmt.Errorf("trino: database (catalog.schema) is required")
	}
	result, err := d.Query(ctx, fmt.Sprintf("SHOW TABLES FROM %s", buildTrinoQualifiedName(database, "")))
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
	if database == "" {
		return nil, fmt.Errorf("trino: database (catalog.schema) is required")
	}
	result, err := d.Query(ctx, fmt.Sprintf("SHOW COLUMNS FROM %s", buildTrinoQualifiedName(database, table)))
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

func buildTrinoQualifiedName(database, table string) string {
	parts := strings.SplitN(database, ".", 2)
	if len(parts) == 2 {
		if table != "" {
			return fmt.Sprintf(`"%s"."%s"."%s"`, escapeTrinoIdentifier(parts[0]), escapeTrinoIdentifier(parts[1]), escapeTrinoIdentifier(table))
		}
		return fmt.Sprintf(`"%s"."%s"`, escapeTrinoIdentifier(parts[0]), escapeTrinoIdentifier(parts[1]))
	}
	if table != "" {
		return fmt.Sprintf(`"%s"."%s"`, escapeTrinoIdentifier(database), escapeTrinoIdentifier(table))
	}
	return fmt.Sprintf(`"%s"`, escapeTrinoIdentifier(database))
}

func normalizeTrinoSQL(sql string) string {
	var buf strings.Builder
	buf.Grow(len(sql))
	inSingleQuote := false
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		if ch == '\'' {
			inSingleQuote = !inSingleQuote
			buf.WriteByte(ch)
		} else if ch == '`' && !inSingleQuote {
			buf.WriteByte('"')
		} else {
			buf.WriteByte(ch)
		}
	}
	return buf.String()
}
