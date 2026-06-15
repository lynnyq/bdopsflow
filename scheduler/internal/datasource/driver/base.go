package driver

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	gohive "github.com/beltran/gohive"
	"github.com/pkg/errors"
)

type UnhealthyChecker interface {
	IsUnhealthy() bool
	MarkUnhealthy()
}

// PoolConfigUpdater 连接池配置动态更新接口
// 支持所有使用连接池的驱动（Hive/Kyuubi/Spark 自定义池，以及 database/sql 内置池）
type PoolConfigUpdater interface {
	UpdatePoolConfig(cfg PoolConfig)
	GetPoolConfig() PoolConfig
	GetPoolStats() (openCount int, idleCount int, inUse int, maxOpen int)
}

type Driver interface {
	Connect(ctx context.Context, config DatasourceConfig) error
	TestConnection(ctx context.Context) error
	Ping(ctx context.Context) error
	Close() error
	Query(ctx context.Context, sql string, args ...interface{}) (*QueryResult, error)
	QueryWithDB(ctx context.Context, sql string, database string) (*QueryResult, error)
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

// ApplyLimitToSQL 参考Superset的方式，在SQL语句末尾添加LIMIT
// Superset 使用 sqlglot 解析 SQL，只有 SELECT/WITH 语句才会添加 LIMIT，
// SHOW/DESCRIBE/EXPLAIN 等语句不支持 LIMIT，不添加。
// 支持多种SQL语法：
//   - MySQL/PostgreSQL/SQLite/StarRocks/Doris/Trino: LIMIT x 或 LIMIT x OFFSET y
//   - Hive/Kyuubi/Spark: 仅支持 LIMIT x（不支持 OFFSET，会自动去除）
// 如果用户SQL中已有LIMIT，则取用户LIMIT和系统限制的较小值
func ApplyLimitToSQL(sql string, limit int, dsType string) string {
	if limit <= 0 {
		return sql
	}

	normalized := NormalizeSQLForType(sql, dsType)
	upperSQL := strings.ToUpper(normalized)

	// 参考 Superset：只对 SELECT 和 WITH(CTE) 语句添加 LIMIT
	// SHOW/DESCRIBE/DESC/EXPLAIN 等语句不支持 LIMIT，不添加
	if !strings.HasPrefix(upperSQL, "SELECT ") &&
		!strings.HasPrefix(upperSQL, "WITH ") {
		return sql
	}

	// 检查是否已经有LIMIT子句
	if strings.Contains(upperSQL, " LIMIT ") {
		// 提取用户指定的LIMIT值
		userLimit := extractUserLimit(upperSQL)
		if userLimit > 0 {
			// 如果用户LIMIT大于系统限制，则替换为系统限制
			if limit < userLimit {
				// 替换SQL中的LIMIT值为系统限制
				// 对于Hive/SparkSQL，如果原SQL包含OFFSET，会自动去除
				return replaceLimitValue(normalized, userLimit, limit)
			}
			// 用户LIMIT小于等于系统限制，保持原样
			return sql
		}
		// 如果无法提取LIMIT值，添加系统限制
		return fmt.Sprintf("%s LIMIT %d", normalized, limit)
	}

	// 没有LIMIT子句，添加系统限制
	// 对于Hive/SparkSQL，仅使用 LIMIT（不支持 OFFSET）
	switch dsType {
	case "mysql", "sqlite", "rqlite", "starrocks", "doris", "trino":
		// 标准LIMIT语法
		return fmt.Sprintf("%s LIMIT %d", normalized, limit)
	case "hive", "kyuubi", "spark":
		// Hive/Spark仅支持 LIMIT
		return fmt.Sprintf("%s LIMIT %d", normalized, limit)
	default:
		// 默认使用LIMIT
		return fmt.Sprintf("%s LIMIT %d", normalized, limit)
	}
}

// extractUserLimit 从SQL中提取用户指定的LIMIT值
// 支持格式：LIMIT 100, LIMIT 100 OFFSET 200, LIMIT 100,200
func extractUserLimit(sql string) int {
	// 匹配 LIMIT 数字（可能包含逗号分隔或OFFSET）的模式
	// 支持: LIMIT 100, LIMIT 100 OFFSET 200, LIMIT 100,200
	re := regexp.MustCompile(`(?i)\s+LIMIT\s+(\d+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) >= 2 {
		limit, err := strconv.Atoi(matches[1])
		if err == nil {
			return limit
		}
	}
	return 0
}

// replaceLimitValue 替换SQL中的LIMIT值为新值
// 如果原SQL包含OFFSET，替换时保留LIMIT部分（去除OFFSET以兼容Hive/SparkSQL）
func replaceLimitValue(sql string, oldLimit, newLimit int) string {
	// 先尝试匹配带OFFSET的情况：LIMIT 2000 OFFSET 100 或 LIMIT 2000,100
	offsetPattern := regexp.MustCompile(fmt.Sprintf(`(?i)\s+LIMIT\s+%d\s*,?\s*\d*\s*(?:OFFSET\s+\d+)?`, oldLimit))
	if offsetPattern.MatchString(sql) {
		// 替换为简单的 LIMIT newLimit（去除 OFFSET）
		return offsetPattern.ReplaceAllString(sql, fmt.Sprintf(" LIMIT %d", newLimit))
	}

	// 普通情况：LIMIT 2000
	pattern := regexp.MustCompile(fmt.Sprintf(`(?i)\s+LIMIT\s+%d\b`, oldLimit))
	return pattern.ReplaceAllString(sql, fmt.Sprintf(" LIMIT %d", newLimit))
}
