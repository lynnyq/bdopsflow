package driver

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gohive "github.com/beltran/gohive"
	"github.com/pkg/errors"
)

type HiveDriver struct {
	connection *gohive.Connection
	config     DatasourceConfig
	mu         sync.Mutex
	unhealthy  atomic.Bool
}

func NewHiveDriver() Driver {
	return &HiveDriver{}
}

func (d *HiveDriver) lockWithContext(ctx context.Context) error {
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
		return errors.Wrap(ctx.Err(), "hive connect cancelled")
	case result := <-resultCh:
		if result.err != nil {
			slog.Error("hive connection failed", "host", config.Host, "port", port, "auth", auth, "error", result.err)
			return errors.Wrap(result.err, "failed to connect to hive")
		}
		d.connection = result.conn
		slog.Info("hive connected", "host", config.Host, "port", port, "database", config.Database)
		return nil
	}
}

func (d *HiveDriver) TestConnection(ctx context.Context) error {
	if d.connection == nil {
		return errors.New("hive connection not established")
	}
	cursor := d.connection.Cursor()
	cursor.Exec(ctx, normalizeSQL("SELECT 1"))
	if cursor.Err != nil {
		execErr := cursor.Err
		cursor.Close()
		return errors.Wrap(execErr, "hive test connection failed")
	}
	cursor.Close()
	return nil
}

func (d *HiveDriver) Ping(ctx context.Context) error {
	if d.connection == nil {
		return errors.New("hive connection not established")
	}
	if d.unhealthy.Load() {
		return errors.New("hive connection marked as unhealthy")
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
			return errors.Wrap(execErr, "hive ping failed, connection may be stale")
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

func (d *HiveDriver) IsUnhealthy() bool {
	return d.unhealthy.Load()
}

func (d *HiveDriver) Close() error {
	if d.connection != nil {
		return d.connection.Close()
	}
	return nil
}

func (d *HiveDriver) Query(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	if d.connection == nil {
		return nil, errors.New("hive connection not established")
	}

	normalizedQuery := normalizeSQL(query)
	slog.Debug("hive executing query", "sql_preview", truncateSQL(normalizedQuery, 200))

	if err := d.lockWithContext(ctx); err != nil {
		return nil, errors.Wrap(err, "hive query lock timeout, another query is running")
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
			resultCh <- queryResult{result: nil, err: errors.Wrap(execErr, "hive query error")}
			return
		}

		description := cursor.Description()
		if cursor.Err != nil {
			descErr := cursor.Err
			cursor.Close()
			resultCh <- queryResult{nil, errors.Wrap(descErr, "hive get description error")}
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
			resultCh <- queryResult{nil, errors.New("hive query returned no columns, the SQL may contain errors or the table does not exist")}
			return
		}

		var rows [][]interface{}
		for cursor.HasMore(context.Background()) {
			rowMap := cursor.RowMap(context.Background())
			if cursor.Err != nil {
				fetchErr := cursor.Err
				cursor.Close()
				resultCh <- queryResult{nil, errors.Wrap(fetchErr, "hive fetch error")}
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
			resultCh <- queryResult{nil, errors.Wrap(finishErr, "hive query error")}
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
		slog.Warn("hive query cancelled by context, sending CancelOperation to Hive Server", "sql_preview", truncateSQL(normalizedQuery, 200), "error", ctx.Err())
		if queryCursor != nil {
			queryCursor.Cancel()
			queryCursor.Close()
		}
		go func() { <-resultCh }()
		return nil, errors.Wrap(ctx.Err(), "hive query cancelled")
	case res := <-resultCh:
		if res.err != nil {
			if isConnectionError(res.err) {
				d.unhealthy.Store(true)
				slog.Warn("hive connection error detected, marked as unhealthy", "sql_preview", truncateSQL(normalizedQuery, 200), "error", res.err)
			}
			slog.Error("hive query execution failed", "sql_preview", truncateSQL(normalizedQuery, 200), "error", res.err)
		}
		return res.result, res.err
	}
}

func (d *HiveDriver) GetDatabases(ctx context.Context) ([]string, error) {
	result, err := d.Query(ctx, normalizeSQL("SHOW DATABASES"))
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("hive query returned nil result for SHOW DATABASES")
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
	if result == nil {
		return nil, errors.New("hive query returned nil result for SHOW TABLES")
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
	if result == nil {
		return nil, errors.New("hive query returned nil result for DESCRIBE")
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
		return errors.New("hive connection not established")
	}

	if err := d.lockWithContext(ctx); err != nil {
		return errors.Wrap(err, "hive use database lock timeout, another query is running")
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
			cursor.Close()
			resultCh <- useResult{errors.Wrap(cursor.Err, "hive use database error")}
			return
		}
		cursor.Close()
		resultCh <- useResult{nil}
	}()

	select {
	case <-ctx.Done():
		go func() { <-resultCh }()
		return errors.Wrap(ctx.Err(), "hive use database cancelled")
	case res := <-resultCh:
		if res.err == nil {
			slog.Debug("hive switched database", "database", database)
		}
		return res.err
	}
}

func escapeHiveIdentifier(name string) string {
	return strings.ReplaceAll(name, "`", "``")
}
