package driver

import (
	"context"
	"fmt"
	"strings"
	"sync"

	gohive "github.com/beltran/gohive"
	"github.com/pkg/errors"
)

type UnhealthyChecker interface {
	IsUnhealthy() bool
}

type Driver interface {
	Connect(ctx context.Context, config DatasourceConfig) error
	TestConnection(ctx context.Context) error
	Ping(ctx context.Context) error
	Close() error
	Query(ctx context.Context, sql string, args ...interface{}) (*QueryResult, error)
	GetDatabases(ctx context.Context) ([]string, error)
	GetTables(ctx context.Context, database string) ([]TableInfo, error)
	GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error)
	SupportsCancel() bool
	UseDatabase(ctx context.Context, database string) error
}

type DatasourceConfig struct {
	Type               string                 `json:"type"`
	Host               string                 `json:"host"`
	Port               int                    `json:"port"`
	Path               string                 `json:"path"`
	Database           string                 `json:"database"`
	Username           string                 `json:"username"`
	Password           string                 `json:"password"`
	AuthType           string                 `json:"auth_type"`
	ConnectionMode     string                 `json:"connection_mode"`
	ZookeeperQuorum    string                 `json:"zookeeper_quorum"`
	ZookeeperNamespace string                 `json:"zookeeper_namespace"`
	RqliteHosts        string                 `json:"rqlite_hosts"`
	Config             map[string]interface{} `json:"config"`
}

type QueryResult struct {
	Columns  []string        `json:"columns"`
	Rows     [][]interface{} `json:"rows"`
	RowCount int64           `json:"row_count"`
}

type TableInfo struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Comment  string `json:"comment"`
	Nullable bool   `json:"nullable"`
}

var driverRegistry = make(map[string]DriverFactory)
var registryMu sync.RWMutex

type DriverFactory func() Driver

func RegisterDriver(dsType string, factory DriverFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	driverRegistry[dsType] = factory
}

func GetDriver(dsType string) (Driver, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	factory, ok := driverRegistry[dsType]
	if !ok {
		return nil, fmt.Errorf("unsupported datasource type: %s", dsType)
	}
	return factory(), nil
}

func SupportedTypes() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	types := make([]string, 0, len(driverRegistry))
	for t := range driverRegistry {
		types = append(types, t)
	}
	return types
}

func IsSupported(dsType string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := driverRegistry[dsType]
	return ok
}

func init() {
	RegisterDriver("mysql", NewMySQLDriver)
	RegisterDriver("sqlite", NewSQLiteDriver)
	RegisterDriver("hive", NewHiveDriver)
	RegisterDriver("kyuubi", NewKyuubiDriver)
	RegisterDriver("spark", NewSparkDriver)
	RegisterDriver("trino", NewTrinoDriver)
	RegisterDriver("starrocks", NewStarRocksDriver)
	RegisterDriver("doris", NewDorisDriver)
	RegisterDriver("rqlite", NewRqliteDriver)
}

func normalizeSQL(sql string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sql), ";"))
}

func ExtractLastStatement(sql string) string {
	segments := strings.Split(sql, ";")
	var last string
	for i := len(segments) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(segments[i])
		if trimmed != "" {
			last = trimmed
			break
		}
	}
	if last == "" {
		return ""
	}
	return last
}

func NormalizeSQLForType(sql string, dsType string) string {
	switch dsType {
	case "hive", "kyuubi", "spark":
		return ExtractLastStatement(sql)
	default:
		return normalizeSQL(sql)
	}
}

func truncateSQL(sql string, maxLen int) string {
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen] + "..."
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "ttransport") ||
		strings.Contains(msg, "transport error") ||
		strings.Contains(msg, "eof")
}

func extractGohiveError(err error, wrapMsg string) error {
	if err == nil {
		return nil
	}
	var hiveErr gohive.HiveError
	if errors.As(err, &hiveErr) {
		msg := hiveErr.Message
		if msg == "" {
			msg = err.Error()
		}
		if hiveErr.ErrorCode > 0 {
			return fmt.Errorf("%s: %s (errorCode: %d)", wrapMsg, msg, hiveErr.ErrorCode)
		}
		return fmt.Errorf("%s: %s", wrapMsg, msg)
	}
	errStr := err.Error()
	if strings.Contains(errStr, "operation in state") && strings.Contains(errStr, "without task status or error message") {
		return fmt.Errorf("%s: 查询执行失败，Hive未返回详细错误信息，请检查SQL语法或数据源权限", wrapMsg)
	}
	return errors.Wrap(err, wrapMsg)
}
