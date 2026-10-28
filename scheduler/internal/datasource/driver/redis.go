package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis 连接模式常量
const (
	redisModeSingle   = "single"
	redisModeSentinel = "sentinel"
	redisModeCluster  = "cluster"
)

// Redis 只读命令集合（allow_write_sql=false 时仅允许这些命令）
var redisReadOnlyCommands = map[string]bool{
	"GET": true, "MGET": true, "STRLEN": true, "GETRANGE": true,
	"KEYS": true, "SCAN": true, "RANDOMKEY": true, "TYPE": true,
	"TTL": true, "PTTL": true, "EXPIRETIME": true, "PEXPIRETIME": true,
	"EXISTS": true, "OBJECT": true, "MEMORY": true,
	"LRANGE": true, "LINDEX": true, "LLEN": true, "LPOS": true,
	"SMEMBERS": true, "SISMEMBER": true, "SMISMEMBER": true, "SCARD": true,
	"SRANDMEMBER": true, "SSCAN": true,
	"HGET": true, "HMGET": true, "HGETALL": true, "HKEYS": true,
	"HVALS": true, "HLEN": true, "HEXISTS": true, "HSCAN": true, "HSTRLEN": true,
	"ZRANGE": true, "ZREVRANGE": true, "ZRANGEBYSCORE": true, "ZREVRANGEBYSCORE": true,
	"ZRANGEBYLEX": true, "ZREVRANGEBYLEX": true, "ZSCORE": true, "ZMSCORE": true,
	"ZRANK": true, "ZREVRANK": true, "ZCARD": true, "ZCOUNT": true,
	"ZLEXCOUNT": true, "ZSCAN": true, "ZRANDMEMBER": true,
	"PFCOUNT":  true,
	"BITCOUNT": true, "BITPOS": true, "GETBIT": true,
	"GEOHASH": true, "GEOPOS": true, "GEODIST": true, "GEORADIUS": true, "GEORADIUSBYMEMBER": true,
	"CLUSTER": true, "INFO": true, "CONFIG": true, "CLIENT": true, "TIME": true,
	"DBSIZE": true, "LASTSAVE": true, "ROLE": true, "COMMAND": true,
	"PING": true, "ECHO": true, "SELECT": true, "QUIT": true,
}

// IsRedisReadOnlyCommand 判断指定 Redis 命令名是否为只读命令。
// 命令名应为大写形式，例如 "GET"、"SET"。
func IsRedisReadOnlyCommand(cmdName string) bool {
	return redisReadOnlyCommands[cmdName]
}

// RedisDriver Redis 数据源驱动，支持单节点、哨兵、集群三种模式。
type RedisDriver struct {
	client redis.UniversalClient
	config DatasourceConfig
}

// NewRedisDriver 创建 Redis 驱动实例。
func NewRedisDriver() Driver {
	return &RedisDriver{}
}

// Connect 建立 Redis 连接，根据 connection_mode 选择单节点/哨兵/集群模式。
func (d *RedisDriver) Connect(ctx context.Context, config DatasourceConfig) error {
	d.config = config

	mode := config.ConnectionMode
	if mode == "" {
		mode = redisModeSingle
	}

	var client redis.UniversalClient
	var err error

	switch mode {
	case redisModeCluster:
		client, err = d.connectCluster(ctx)
	case redisModeSentinel:
		client, err = d.connectSentinel(ctx)
	default:
		client, err = d.connectSingle(ctx)
	}

	if err != nil {
		return fmt.Errorf("failed to connect redis: %w", err)
	}

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		slog.Error("redis ping failed", "mode", mode, "error", err)
		return fmt.Errorf("failed to ping redis: %w", err)
	}

	d.client = client
	slog.Info("redis connected", "mode", mode, "host", config.Host, "port", config.Port)
	return nil
}

// connectSingle 单节点模式连接
func (d *RedisDriver) connectSingle(ctx context.Context) (redis.UniversalClient, error) {
	port := d.config.Port
	if port == 0 {
		port = 6379
	}
	addr := fmt.Sprintf("%s:%d", d.config.Host, port)

	opts := &redis.Options{
		Addr:     addr,
		Password: d.config.Password,
		DB:       parseRedisDB(d.config.Database),
	}

	if timeout := getConfigInt(d.config.Config, "dial_timeout"); timeout > 0 {
		opts.DialTimeout = durationFromSeconds(timeout)
	}
	if timeout := getConfigInt(d.config.Config, "read_timeout"); timeout > 0 {
		opts.ReadTimeout = durationFromSeconds(timeout)
	}
	if timeout := getConfigInt(d.config.Config, "write_timeout"); timeout > 0 {
		opts.WriteTimeout = durationFromSeconds(timeout)
	}

	slog.Debug("redis single mode connecting", "addr", addr, "db", opts.DB)
	return redis.NewClient(opts), nil
}

// connectSentinel 哨兵模式连接
func (d *RedisDriver) connectSentinel(ctx context.Context) (redis.UniversalClient, error) {
	sentinelAddrs := getConfigStringSlice(d.config.Config, "sentinel_addrs")
	if len(sentinelAddrs) == 0 {
		return nil, fmt.Errorf("sentinel mode requires sentinel_addrs in config")
	}
	masterName := getConfigString(d.config.Config, "master_name")
	if masterName == "" {
		return nil, fmt.Errorf("sentinel mode requires master_name in config")
	}

	sentinelPassword := getConfigString(d.config.Config, "sentinel_password")

	opts := &redis.FailoverOptions{
		MasterName:       masterName,
		SentinelAddrs:    sentinelAddrs,
		Password:         d.config.Password,
		SentinelPassword: sentinelPassword,
		DB:               parseRedisDB(d.config.Database),
	}

	slog.Debug("redis sentinel mode connecting", "sentinel_addrs", sentinelAddrs, "master", masterName, "db", opts.DB)
	return redis.NewFailoverClient(opts), nil
}

// connectCluster 集群模式连接
func (d *RedisDriver) connectCluster(ctx context.Context) (redis.UniversalClient, error) {
	clusterAddrs := getConfigStringSlice(d.config.Config, "cluster_addrs")
	if len(clusterAddrs) == 0 {
		return nil, fmt.Errorf("cluster mode requires cluster_addrs in config")
	}

	opts := &redis.ClusterOptions{
		Addrs:    clusterAddrs,
		Password: d.config.Password,
	}

	slog.Debug("redis cluster mode connecting", "addrs", clusterAddrs)
	return redis.NewClusterClient(opts), nil
}

// TestConnection 测试 Redis 连接（执行 PING）。
func (d *RedisDriver) TestConnection(ctx context.Context) error {
	if d.client == nil {
		return fmt.Errorf("redis connection not established")
	}
	return d.client.Ping(ctx).Err()
}

// Ping 轻量级连接存活检查。
func (d *RedisDriver) Ping(ctx context.Context) error {
	if d.client == nil {
		return fmt.Errorf("redis connection not established")
	}
	return d.client.Ping(ctx).Err()
}

// Close 关闭 Redis 连接。
func (d *RedisDriver) Close() error {
	if d.client != nil {
		return d.client.Close()
	}
	return nil
}

// Query 执行 Redis 命令。
// 输入的 query 参数为 Redis 命令字符串，例如 "GET key"、"SET key value"、"KEYS *"。
// 支持带引号的字符串参数，例如 "SET key \"hello world\""。
func (d *RedisDriver) Query(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	if d.client == nil {
		return nil, &DatasourceError{
			Err:            fmt.Errorf("redis connection not established"),
			Category:       ErrCategoryConnection,
			DatasourceType: "redis",
			Retryable:      false,
		}
	}

	cmdStr := strings.TrimSpace(query)
	if cmdStr == "" {
		return nil, &DatasourceError{
			Err:            fmt.Errorf("empty redis command"),
			Category:       ErrCategoryQuery,
			DatasourceType: "redis",
			Retryable:      false,
		}
	}

	cmdArgs, err := parseRedisCommand(cmdStr)
	if err != nil {
		return nil, &DatasourceError{
			Err:            fmt.Errorf("failed to parse redis command: %w", err),
			Category:       ErrCategoryQuery,
			DatasourceType: "redis",
			Retryable:      false,
		}
	}
	if len(cmdArgs) == 0 {
		return nil, &DatasourceError{
			Err:            fmt.Errorf("empty redis command"),
			Category:       ErrCategoryQuery,
			DatasourceType: "redis",
			Retryable:      false,
		}
	}

	slog.Debug("redis executing command", "command", truncateSQL(cmdStr, 200))

	// 构造 interface{} 切片供 go-redis Do 使用
	ifaceArgs := make([]interface{}, len(cmdArgs))
	for i, a := range cmdArgs {
		ifaceArgs[i] = a
	}

	rawResult, err := d.client.Do(ctx, ifaceArgs...).Result()
	if err != nil {
		slog.Error("redis command execution failed", "command", truncateSQL(cmdStr, 200), "error", err)
		return nil, ClassifyError(err, "redis")
	}

	columns := []string{"result"}
	row := []interface{}{formatRedisResult(rawResult)}

	return NormalizeQueryResult(&QueryResult{
		Columns:  columns,
		Rows:     [][]interface{}{row},
		RowCount: 1,
	}), nil
}

// QueryWithDB 执行 Redis 命令，执行前切换到指定数据库（仅单节点/哨兵模式有效）。
func (d *RedisDriver) QueryWithDB(ctx context.Context, query string, database string) (*QueryResult, error) {
	if database == "" {
		return d.Query(ctx, query)
	}
	// 集群模式不支持多数据库
	if d.config.ConnectionMode == redisModeCluster {
		return d.Query(ctx, query)
	}
	if err := d.UseDatabase(ctx, database); err != nil {
		return nil, err
	}
	result, err := d.Query(ctx, query)
	// 切回原数据库
	originalDB := parseRedisDB(d.config.Database)
	if d.config.Database != "" && database != d.config.Database {
		if restoreErr := d.UseDatabase(ctx, strconv.Itoa(originalDB)); restoreErr != nil {
			slog.Warn("failed to restore redis database after query", "error", restoreErr)
		}
	}
	return result, err
}

// TryQueryWithDB 非阻塞版本的 QueryWithDB，go-redis 内置连接池不会长时间阻塞，直接委托。
func (d *RedisDriver) TryQueryWithDB(ctx context.Context, query string, database string) (*QueryResult, error) {
	return d.QueryWithDB(ctx, query, database)
}

// GetDatabases 获取可用数据库列表。
// 单节点/哨兵模式返回 0-15（默认 16 个数据库），集群模式仅返回 0。
func (d *RedisDriver) GetDatabases(ctx context.Context) ([]string, error) {
	if d.config.ConnectionMode == redisModeCluster {
		return []string{"0"}, nil
	}
	dbs := make([]string, 0, 16)
	for i := 0; i < 16; i++ {
		dbs = append(dbs, strconv.Itoa(i))
	}
	return dbs, nil
}

// GetTables Redis 无表概念，返回当前数据库的 key 列表（使用 SCAN 避免阻塞）。
func (d *RedisDriver) GetTables(ctx context.Context, database string) ([]TableInfo, error) {
	if database != "" && d.config.ConnectionMode != redisModeCluster {
		if err := d.UseDatabase(ctx, database); err != nil {
			return nil, err
		}
	}

	var keys []string
	var cursor uint64
	for {
		var batch []string
		var err error
		batch, cursor, err = d.client.Scan(ctx, cursor, "*", 100).Result()
		if err != nil {
			return nil, ClassifyError(err, "redis")
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}

	tables := make([]TableInfo, 0, len(keys))
	for _, key := range keys {
		tables = append(tables, TableInfo{Name: key})
	}
	return tables, nil
}

// GetColumns Redis 无列概念，返回空列表。
func (d *RedisDriver) GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error) {
	return []ColumnInfo{}, nil
}

// SupportsCancel Redis 不支持单独取消单个命令（依赖 context 取消）。
func (d *RedisDriver) SupportsCancel() bool {
	return false
}

// UseDatabase 切换 Redis 数据库（执行 SELECT 命令）。集群模式不支持。
func (d *RedisDriver) UseDatabase(ctx context.Context, database string) error {
	if database == "" {
		return nil
	}
	if d.client == nil {
		return fmt.Errorf("redis connection not established")
	}
	if d.config.ConnectionMode == redisModeCluster {
		return nil
	}
	dbNum, err := strconv.Atoi(database)
	if err != nil {
		return fmt.Errorf("invalid redis database number: %s", database)
	}
	if err := d.client.Do(ctx, "SELECT", dbNum).Err(); err != nil {
		return fmt.Errorf("redis select database error: %w", err)
	}
	slog.Debug("redis switched database", "database", dbNum)
	return nil
}

// parseRedisDB 解析 Redis 数据库编号，非法值返回 0。
func parseRedisDB(dbStr string) int {
	if dbStr == "" {
		return 0
	}
	db, err := strconv.Atoi(dbStr)
	if err != nil || db < 0 {
		return 0
	}
	return db
}

// parseRedisCommand 解析 Redis 命令字符串，支持空格分隔和引号包裹的参数。
// 例如：'GET key' -> ["GET", "key"]
//
//	'SET key "hello world"' -> ["SET", "key", "hello world"]
func parseRedisCommand(cmd string) ([]string, error) {
	var args []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]

		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && (inSingleQuote || inDoubleQuote) {
			escaped = true
			continue
		}

		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}

		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		if (ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r') && !inSingleQuote && !inDoubleQuote {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(ch)
	}

	if inSingleQuote || inDoubleQuote {
		return nil, fmt.Errorf("unterminated quoted string in command")
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args, nil
}

// formatRedisResult 将 go-redis 返回的原始结果格式化为可读字符串。
func formatRedisResult(result interface{}) string {
	if result == nil {
		return "(nil)"
	}

	switch v := result.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "1"
		}
		return "0"
	case error:
		return "(error) " + v.Error()
	case []interface{}:
		// 数组结果格式化为 JSON 数组展示
		formatted := make([]interface{}, len(v))
		for i, item := range v {
			formatted[i] = formatRedisResult(item)
		}
		data, err := json.Marshal(formatted)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// getConfigString 从 Config map 中获取字符串值。
func getConfigString(config map[string]interface{}, key string) string {
	if config == nil {
		return ""
	}
	v, ok := config[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}

// getConfigStringSlice 从 Config map 中获取字符串切片值。
// 支持逗号分隔的字符串或数组两种格式。
func getConfigStringSlice(config map[string]interface{}, key string) []string {
	if config == nil {
		return nil
	}
	v, ok := config[key]
	if !ok {
		return nil
	}

	switch val := v.(type) {
	case string:
		parts := strings.Split(val, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	case []string:
		return val
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	default:
		return nil
	}
}

// getConfigInt 从 Config map 中获取整数值。
func getConfigInt(config map[string]interface{}, key string) int {
	if config == nil {
		return 0
	}
	v, ok := config[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		n, err := strconv.Atoi(val)
		if err == nil {
			return n
		}
	}
	return 0
}

// durationFromSeconds 将秒数转换为 time.Duration。
func durationFromSeconds(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}
