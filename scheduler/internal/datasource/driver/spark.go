package driver

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	gohive "github.com/beltran/gohive"
	"github.com/pkg/errors"
)

type SparkDriver struct {
	connection *gohive.Connection
	config     DatasourceConfig
	mu         sync.Mutex
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

	slog.Debug("spark connecting", "host", config.Host, "port", port, "database", config.Database, "mode", config.ConnectionMode)

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
		return errors.Wrap(ctx.Err(), "spark connect cancelled")
	case result := <-resultCh:
		if result.err != nil {
			slog.Error("spark connection failed", "host", config.Host, "port", port, "error", result.err)
			return errors.Wrap(result.err, "failed to connect to spark")
		}
		d.connection = result.conn
		slog.Info("spark connected", "host", config.Host, "port", port, "database", config.Database)
		return nil
	}
}

func (d *SparkDriver) TestConnection(ctx context.Context) error {
	if d.connection == nil {
		return errors.New("spark connection not established")
	}
	cursor := d.connection.Cursor()
	cursor.Exec(ctx, normalizeSQL("SELECT 1"))
	if cursor.Err != nil {
		cursor.Close()
		return errors.Wrap(cursor.Err, "spark test connection failed")
	}
	cursor.Close()
	return nil
}

func (d *SparkDriver) Ping(ctx context.Context) error {
	if d.connection == nil {
		return errors.New("spark connection not established")
	}
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
		return nil, errors.New("spark connection not established")
	}

	normalizedQuery := normalizeSQL(query)
	slog.Debug("spark executing query", "sql_preview", truncateSQL(normalizedQuery, 200))

	d.mu.Lock()
	defer d.mu.Unlock()

	type queryResult struct {
		result *QueryResult
		err    error
	}
	resultCh := make(chan queryResult, 1)

	go func() {
		cursor := d.connection.Cursor()
		cursor.Exec(context.Background(), normalizedQuery)
		if cursor.Err != nil {
			cursor.Close()
			resultCh <- queryResult{nil, errors.Wrap(cursor.Err, "spark query error")}
			return
		}

		description := cursor.Description()
		if cursor.Err != nil {
			cursor.Close()
			resultCh <- queryResult{nil, errors.Wrap(cursor.Err, "spark get description error")}
			return
		}

		var columns []string
		for _, col := range description {
			if len(col) > 0 {
				columns = append(columns, col[0])
			}
		}

		var rows [][]interface{}
		for cursor.HasMore(context.Background()) {
			rowMap := cursor.RowMap(context.Background())
			if cursor.Err != nil {
				cursor.Close()
				resultCh <- queryResult{nil, errors.Wrap(cursor.Err, "spark fetch error")}
				return
			}
			row := make([]interface{}, len(columns))
			for i, col := range columns {
				row[i] = rowMap[col]
			}
			rows = append(rows, row)
		}
		if cursor.Err != nil {
			cursor.Close()
			resultCh <- queryResult{nil, errors.Wrap(cursor.Err, "spark query error")}
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
		slog.Warn("spark query cancelled by context", "sql_preview", truncateSQL(normalizedQuery, 200), "error", ctx.Err())
		go func() { <-resultCh }()
		return nil, errors.Wrap(ctx.Err(), "spark query cancelled")
	case res := <-resultCh:
		if res.err != nil {
			slog.Error("spark query execution failed", "sql_preview", truncateSQL(normalizedQuery, 200), "error", res.err)
		}
		return res.result, res.err
	}
}

func (d *SparkDriver) GetDatabases(ctx context.Context) ([]string, error) {
	result, err := d.Query(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("spark query returned nil result for SHOW DATABASES")
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
	if result == nil {
		return nil, errors.New("spark query returned nil result for SHOW TABLES")
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
	if result == nil {
		return nil, errors.New("spark query returned nil result for DESCRIBE")
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
		return errors.New("spark connection not established")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	type useResult struct {
		err error
	}
	resultCh := make(chan useResult, 1)

	go func() {
		cursor := d.connection.Cursor()
		cursor.Exec(context.Background(), fmt.Sprintf("USE %s", escapeHiveIdentifier(database)))
		if cursor.Err != nil {
			cursor.Close()
			resultCh <- useResult{errors.Wrap(cursor.Err, "spark use database error")}
			return
		}
		cursor.Close()
		resultCh <- useResult{nil}
	}()

	select {
	case <-ctx.Done():
		go func() { <-resultCh }()
		return errors.Wrap(ctx.Err(), "spark use database cancelled")
	case res := <-resultCh:
		if res.err == nil {
			slog.Debug("spark switched database", "database", database)
		}
		return res.err
	}
}
