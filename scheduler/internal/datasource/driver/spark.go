package driver

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	gohive "github.com/beltran/gohive"
	"github.com/pkg/errors"
)

type SparkDriver struct {
	connection *gohive.Connection
	config     DatasourceConfig
	mu         sync.Mutex
	unhealthy  atomic.Bool
}

func NewSparkDriver() Driver {
	return &SparkDriver{}
}

func (d *SparkDriver) lockWithContext(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	ch := make(chan struct{}, 1)
	go func() {
		d.mu.Lock()
		close(ch)
	}()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		<-ch
		d.mu.Unlock()
		return ctx.Err()
	}
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
		execErr := cursor.Err
		cursor.Close()
		return extractGohiveError(execErr, "spark test connection failed")
	}
	cursor.Close()
	return nil
}

func (d *SparkDriver) Ping(ctx context.Context) error {
	if d.connection == nil {
		return errors.New("spark connection not established")
	}
	if d.unhealthy.Load() {
		return errors.New("spark connection marked as unhealthy")
	}

	acquired := make(chan struct{}, 1)
	go func() {
		d.mu.Lock()
		close(acquired)
	}()

	select {
	case <-acquired:
		defer d.mu.Unlock()
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		cursor := d.connection.Cursor()
		cursor.Exec(pingCtx, normalizeSQL("SELECT 1"))
		if cursor.Err != nil {
			execErr := cursor.Err
			cursor.Close()
			d.unhealthy.Store(true)
			return extractGohiveError(execErr, "spark ping failed, connection may be stale")
		}
		cursor.Close()
		return nil
	case <-ctx.Done():
		go func() {
			<-acquired
			d.mu.Unlock()
		}()
		return nil
	}
}

func (d *SparkDriver) IsUnhealthy() bool {
	return d.unhealthy.Load()
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

	if err := d.lockWithContext(ctx); err != nil {
		return nil, errors.Wrap(err, "spark query lock timeout, another query is running")
	}
	defer d.mu.Unlock()

	type queryResult struct {
		result *QueryResult
		err    error
	}
	resultCh := make(chan queryResult, 1)

	var queryCursor *gohive.Cursor

	go func() {
		cursor := d.connection.Cursor()
		queryCursor = cursor
		cursor.Exec(context.Background(), normalizedQuery)
		if cursor.Err != nil {
			execErr := cursor.Err
			cursor.Close()
			resultCh <- queryResult{nil, extractGohiveError(execErr, "spark query error")}
			return
		}

		description := cursor.Description()
		if cursor.Err != nil {
			descErr := cursor.Err
			cursor.Close()
			resultCh <- queryResult{nil, extractGohiveError(descErr, "spark get description error")}
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
			resultCh <- queryResult{nil, errors.New("spark query returned no columns, the SQL may contain errors or the table does not exist")}
			return
		}

		var rows [][]interface{}
		for cursor.HasMore(context.Background()) {
			rowMap := cursor.RowMap(context.Background())
			if cursor.Err != nil {
				fetchErr := cursor.Err
				cursor.Close()
				resultCh <- queryResult{nil, extractGohiveError(fetchErr, "spark fetch error")}
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
			resultCh <- queryResult{nil, extractGohiveError(finishErr, "spark query error")}
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
		slog.Warn("spark query cancelled by context, sending CancelOperation to Spark Server", "sql_preview", truncateSQL(normalizedQuery, 200), "error", ctx.Err())
		if queryCursor != nil {
			queryCursor.Cancel()
			queryCursor.Close()
		}
		go func() { <-resultCh }()
		return nil, errors.Wrap(ctx.Err(), "spark query cancelled")
	case res := <-resultCh:
		if res.err != nil {
			if isConnectionError(res.err) {
				d.unhealthy.Store(true)
				slog.Warn("spark connection error detected, marked as unhealthy", "sql_preview", truncateSQL(normalizedQuery, 200), "error", res.err)
			}
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

	if err := d.lockWithContext(ctx); err != nil {
		return errors.Wrap(err, "spark use database lock timeout, another query is running")
	}
	defer d.mu.Unlock()

	type useResult struct {
		err error
	}
	resultCh := make(chan useResult, 1)

	go func() {
		cursor := d.connection.Cursor()
		cursor.Exec(context.Background(), fmt.Sprintf("USE %s", escapeHiveIdentifier(database)))
		if cursor.Err != nil {
			execErr := cursor.Err
			cursor.Close()
			resultCh <- useResult{extractGohiveError(execErr, "spark use database error")}
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
