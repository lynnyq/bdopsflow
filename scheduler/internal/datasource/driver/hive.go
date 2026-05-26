package driver

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	gohive "github.com/beltran/gohive"
)

type HiveDriver struct {
	connection *gohive.Connection
	config     DatasourceConfig
}

func NewHiveDriver() Driver {
	return &HiveDriver{}
}

func (d *HiveDriver) Connect(ctx context.Context, config DatasourceConfig) error {
	d.config = config
	port := config.Port
	if port == 0 {
		port = 10000
	}

	slog.Debug("hive connecting", "host", config.Host, "port", port, "database", config.Database, "auth_type", config.AuthType, "mode", config.ConnectionMode)

	configuration := gohive.NewConnectConfiguration()
	configuration.Username = config.Username
	configuration.Password = config.Password
	configuration.Service = "hive"
	configuration.TransportMode = "binary"

	if config.Database != "" {
		configuration.Database = config.Database
	}

	if config.ZookeeperNamespace != "" {
		configuration.ZookeeperNamespace = config.ZookeeperNamespace
	}

	auth := "NONE"
	if config.AuthType == "ldap" {
		auth = "LDAP"
	} else if config.AuthType == "simple" || config.AuthType == "" {
		auth = "NONE"
	}

	type connectResult struct {
		conn *gohive.Connection
		err  error
	}
	resultCh := make(chan connectResult, 1)

	go func() {
		var connection *gohive.Connection
		var err error
		if config.ConnectionMode == "zookeeper" && config.ZookeeperQuorum != "" {
			connection, err = gohive.ConnectZookeeper(config.ZookeeperQuorum, auth, configuration)
		} else {
			connection, err = gohive.Connect(config.Host, port, auth, configuration)
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
		return fmt.Errorf("hive connect cancelled: %w", ctx.Err())
	case result := <-resultCh:
		if result.err != nil {
			slog.Error("hive connection failed", "host", config.Host, "port", port, "auth", auth, "error", result.err)
			return fmt.Errorf("failed to connect to hive: %w", result.err)
		}
		d.connection = result.conn
		slog.Info("hive connected", "host", config.Host, "port", port, "database", config.Database)
		return nil
	}
}

func (d *HiveDriver) TestConnection(ctx context.Context) error {
	if d.connection == nil {
		return fmt.Errorf("hive connection not established")
	}
	cursor := d.connection.Cursor()
	cursor.Exec(ctx, normalizeSQL("SELECT 1"))
	if cursor.Err != nil {
		cursor.Close()
		return fmt.Errorf("hive test connection failed: %w", cursor.Err)
	}
	cursor.Close()
	return nil
}

func (d *HiveDriver) Close() error {
	if d.connection != nil {
		return d.connection.Close()
	}
	return nil
}

func (d *HiveDriver) Query(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	if d.connection == nil {
		return nil, fmt.Errorf("hive connection not established")
	}

	normalizedQuery := normalizeSQL(query)
	slog.Debug("hive executing query", "sql_preview", truncateSQL(normalizedQuery, 200))

	cursor := d.connection.Cursor()
	cursor.Exec(ctx, normalizedQuery)
	if cursor.Err != nil {
		cursor.Close()
		slog.Error("hive query execution failed", "sql_preview", truncateSQL(normalizedQuery, 200), "error", cursor.Err)
		return nil, fmt.Errorf("hive query error: %w", cursor.Err)
	}
	defer cursor.Close()

	description := cursor.Description()
	if cursor.Err != nil {
		return nil, fmt.Errorf("hive get description error: %w", cursor.Err)
	}

	var columns []string
	for _, col := range description {
		if len(col) > 0 {
			columns = append(columns, col[0])
		}
	}

	var rows [][]interface{}
	for cursor.HasMore(ctx) {
		rowMap := cursor.RowMap(ctx)
		if cursor.Err != nil {
			return nil, fmt.Errorf("hive fetch error: %w", cursor.Err)
		}
		row := make([]interface{}, len(columns))
		for i, col := range columns {
			row[i] = rowMap[col]
		}
		rows = append(rows, row)
	}

	return &QueryResult{
		Columns:  columns,
		Rows:     rows,
		RowCount: int64(len(rows)),
	}, nil
}

func (d *HiveDriver) GetDatabases(ctx context.Context) ([]string, error) {
	result, err := d.Query(ctx, normalizeSQL("SHOW DATABASES"))
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

func (d *HiveDriver) GetTables(ctx context.Context, database string) ([]TableInfo, error) {
	if database == "" {
		database = d.config.Database
	}
	result, err := d.Query(ctx, fmt.Sprintf("SHOW TABLES IN %s", escapeHiveIdentifier(database)))
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

func (d *HiveDriver) GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error) {
	if database == "" {
		database = d.config.Database
	}
	result, err := d.Query(ctx, fmt.Sprintf("DESCRIBE %s.%s", escapeHiveIdentifier(database), escapeHiveIdentifier(table)))
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

func (d *HiveDriver) SupportsCancel() bool {
	return true
}

func (d *HiveDriver) UseDatabase(ctx context.Context, database string) error {
	if database == "" {
		return nil
	}
	if d.connection == nil {
		return fmt.Errorf("hive connection not established")
	}
	cursor := d.connection.Cursor()
	cursor.Exec(ctx, fmt.Sprintf("USE %s", escapeHiveIdentifier(database)))
	if cursor.Err != nil {
		cursor.Close()
		return fmt.Errorf("hive use database error: %w", cursor.Err)
	}
	cursor.Close()
	slog.Debug("hive switched database", "database", database)
	return nil
}

func escapeHiveIdentifier(name string) string {
	return strings.ReplaceAll(name, "`", "``")
}
