package driver

import (
	"context"
	"fmt"
	"log/slog"

	gohive "github.com/beltran/gohive"
)

type SparkDriver struct {
	connection *gohive.Connection
	config     DatasourceConfig
}

func NewSparkDriver() Driver {
	return &SparkDriver{}
}

func (d *SparkDriver) Connect(ctx context.Context, config DatasourceConfig) error {
	d.config = config
	port := config.Port
	if port == 0 {
		port = 10016
	}

	configuration := gohive.NewConnectConfiguration()
	configuration.Username = config.Username
	configuration.Password = config.Password
	configuration.Service = "spark"
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
	}

	var connection *gohive.Connection
	var err error

	slog.Debug("spark connecting", "host", config.Host, "port", port, "database", config.Database, "mode", config.ConnectionMode)

	if config.ConnectionMode == "zookeeper" && config.ZookeeperQuorum != "" {
		connection, err = gohive.ConnectZookeeper(config.ZookeeperQuorum, auth, configuration)
	} else {
		connection, err = gohive.Connect(config.Host, port, auth, configuration)
	}

	if err != nil {
		slog.Error("spark connection failed", "host", config.Host, "port", port, "error", err)
		return fmt.Errorf("failed to connect to spark: %w", err)
	}

	d.connection = connection
	slog.Info("spark connected", "host", config.Host, "port", port, "database", config.Database)
	return nil
}

func (d *SparkDriver) TestConnection(ctx context.Context) error {
	if d.connection == nil {
		return fmt.Errorf("spark connection not established")
	}
	cursor := d.connection.Cursor()
	cursor.Exec(ctx, normalizeSQL("SELECT 1"))
	if cursor.Err != nil {
		cursor.Close()
		return fmt.Errorf("spark test connection failed: %w", cursor.Err)
	}
	cursor.Close()
	return nil
}

func (d *SparkDriver) Close() error {
	if d.connection != nil {
		return d.connection.Close()
	}
	return nil
}

func (d *SparkDriver) Query(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	if d.connection == nil {
		return nil, fmt.Errorf("spark connection not established")
	}
	cursor := d.connection.Cursor()
	slog.Debug("spark executing query", "sql_preview", truncateSQL(normalizeSQL(query), 200))
	cursor.Exec(ctx, normalizeSQL(query))
	if cursor.Err != nil {
		slog.Error("spark query execution failed", "sql_preview", truncateSQL(normalizeSQL(query), 200), "error", cursor.Err)
		cursor.Close()
		return nil, fmt.Errorf("spark query error: %w", cursor.Err)
	}
	defer cursor.Close()

	description := cursor.Description()
	if cursor.Err != nil {
		return nil, fmt.Errorf("spark get description error: %w", cursor.Err)
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
			return nil, fmt.Errorf("spark fetch error: %w", cursor.Err)
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

func (d *SparkDriver) GetDatabases(ctx context.Context) ([]string, error) {
	result, err := d.Query(ctx, "SHOW DATABASES")
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

func (d *SparkDriver) GetTables(ctx context.Context, database string) ([]TableInfo, error) {
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

func (d *SparkDriver) GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error) {
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

func (d *SparkDriver) SupportsCancel() bool {
	return true
}

func (d *SparkDriver) UseDatabase(ctx context.Context, database string) error {
	if database == "" {
		return nil
	}
	if d.connection == nil {
		return fmt.Errorf("spark connection not established")
	}
	cursor := d.connection.Cursor()
	cursor.Exec(ctx, fmt.Sprintf("USE %s", escapeHiveIdentifier(database)))
	if cursor.Err != nil {
		cursor.Close()
		return fmt.Errorf("spark use database error: %w", cursor.Err)
	}
	cursor.Close()
	slog.Debug("spark switched database", "database", database)
	return nil
}
