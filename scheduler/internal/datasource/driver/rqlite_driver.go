package driver

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	rqlite "github.com/rqlite/gorqlite"
)

type RqliteDriver struct {
	conn   *rqlite.Connection
	config DatasourceConfig
}

func NewRqliteDriver() Driver {
	return &RqliteDriver{}
}

func (d *RqliteDriver) Connect(ctx context.Context, config DatasourceConfig) error {
	d.config = config

	scheme := "http"
	if ssl, ok := config.Config["ssl"]; ok {
		if sslBool, ok := ssl.(bool); ok && sslBool {
			scheme = "https"
		}
	}

	slog.Debug("rqlite connecting", "mode", config.ConnectionMode, "host", config.Host, "port", config.Port, "ssl", scheme == "https")

	var conn *rqlite.Connection
	var err error

	if config.ConnectionMode == "multi" && config.RqliteHosts != "" {
		slog.Debug("rqlite multi-node mode", "hosts", config.RqliteHosts)
		hosts := strings.Split(config.RqliteHosts, ",")
		urls := make([]string, 0, len(hosts))
		for i, host := range hosts {
			hosts[i] = strings.TrimSpace(host)
			if !strings.Contains(hosts[i], ":") {
				hosts[i] = hosts[i] + ":4001"
			}
			u := &url.URL{
				Scheme: scheme,
				Host:   hosts[i],
			}
			if config.Username != "" && config.Password != "" {
				u.User = url.UserPassword(config.Username, config.Password)
			}
			urls = append(urls, u.String())
		}

		for _, u := range urls {
			conn, err = rqlite.Open(u)
			if err != nil {
				slog.Debug("rqlite node connect failed, trying next", "url", u, "error", err)
				continue
			}
			stmt := rqlite.ParameterizedStatement{Query: "SELECT 1"}
			qr, testErr := conn.QueryOneParameterized(stmt)
			if testErr == nil && qr.Err == nil {
				slog.Debug("rqlite node connected successfully", "url", u)
				break
			}
			slog.Debug("rqlite node test failed, trying next", "url", u, "error", testErr)
			conn.Close()
			conn = nil
		}
		if conn == nil {
			slog.Error("rqlite failed to connect to any node", "hosts", config.RqliteHosts)
			return fmt.Errorf("failed to connect to any rqlite node")
		}
	} else {
		port := config.Port
		if port == 0 {
			port = 4001
		}

		u := &url.URL{
			Scheme: scheme,
			Host:   fmt.Sprintf("%s:%d", config.Host, port),
		}
		if config.Username != "" && config.Password != "" {
			u.User = url.UserPassword(config.Username, config.Password)
		}

		conn, err = rqlite.Open(u.String())
		if err != nil {
			slog.Error("rqlite connection failed", "host", config.Host, "port", port, "error", err)
			return fmt.Errorf("failed to connect to rqlite: %w", err)
		}
	}

	d.conn = conn
	slog.Info("rqlite connected", "mode", config.ConnectionMode, "host", config.Host, "port", config.Port)
	return nil
}

func (d *RqliteDriver) TestConnection(ctx context.Context) error {
	if d.conn == nil {
		return fmt.Errorf("rqlite connection not established")
	}
	rows, err := d.conn.QueryOneParameterizedContext(ctx, rqlite.ParameterizedStatement{Query: "SELECT 1"})
	if err != nil {
		return fmt.Errorf("rqlite test connection failed: %w", err)
	}
	if rows.Err != nil {
		return fmt.Errorf("rqlite test connection failed: %w", rows.Err)
	}
	return nil
}

func (d *RqliteDriver) Close() error {
	if d.conn != nil {
		d.conn.Close()
	}
	return nil
}

func (d *RqliteDriver) Query(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	if d.conn == nil {
		return nil, fmt.Errorf("rqlite connection not established")
	}

	slog.Debug("rqlite executing query", "sql_preview", truncateSQL(query, 200))

	var qr rqlite.QueryResult
	var err error
	if len(args) > 0 {
		qr, err = d.conn.QueryOneParameterizedContext(ctx, rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: args,
		})
	} else {
		qr, err = d.conn.QueryOneParameterizedContext(ctx, rqlite.ParameterizedStatement{
			Query: query,
		})
	}
	if err != nil {
		slog.Error("rqlite query execution failed", "sql_preview", truncateSQL(query, 200), "error", err)
		return nil, fmt.Errorf("rqlite query error: %w", err)
	}
	if qr.Err != nil {
		slog.Error("rqlite query result error", "sql_preview", truncateSQL(query, 200), "error", qr.Err)
		return nil, fmt.Errorf("rqlite query error: %w", qr.Err)
	}

	columns := qr.Columns()
	var resultRows [][]interface{}
	for qr.Next() {
		slice, err := qr.Slice()
		if err != nil {
			return nil, fmt.Errorf("rqlite slice error: %w", err)
		}
		row := make([]interface{}, len(slice))
		for i, v := range slice {
			row[i] = v
		}
		resultRows = append(resultRows, row)
	}

	return &QueryResult{
		Columns:  columns,
		Rows:     resultRows,
		RowCount: int64(len(resultRows)),
	}, nil
}

func (d *RqliteDriver) GetDatabases(ctx context.Context) ([]string, error) {
	return []string{"main"}, nil
}

func (d *RqliteDriver) GetTables(ctx context.Context, database string) ([]TableInfo, error) {
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

func (d *RqliteDriver) GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error) {
	result, err := d.Query(ctx, fmt.Sprintf("PRAGMA table_info(%s)", escapeRqliteIdentifier(table)))
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

func (d *RqliteDriver) SupportsCancel() bool {
	return false
}

func (d *RqliteDriver) UseDatabase(ctx context.Context, database string) error {
	return nil
}

func escapeRqliteIdentifier(name string) string {
	return strings.ReplaceAll(name, `"`, `""`)
}
