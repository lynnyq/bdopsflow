package datasource

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	rqlite "github.com/rqlite/gorqlite"
)

var defaultConfigValues = map[string]string{
	"datasource.default_limit":           "1000",
	"datasource.max_export_rows":         "1000",
	"datasource.cache_ttl":               "300",
	"datasource.cache_max_size":          "100",
	"datasource.query_timeout":           "60",
	"datasource.max_concurrent_per_user": "5",
	"datasource.max_concurrent_global":   "50",
	"datasource.allow_write_sql":         "false",
	"datasource.history_retention_days":  "30",
	"datasource.connection_max_idle":     "5",
	"datasource.connection_max_open":     "10",
	"datasource.connection_max_lifetime": "1800",
	"datasource.max_sql_length":          "65536",
	"datasource.max_cell_size":           "65536",
	"datasource.health_check_interval":   "300",
	"datasource.test_timeout":            "10",
	"web.enabled":                        "false",
	"wecom.robot_url":                    "https://qyapi.weixin.qq.com/cgi-bin/webhook/send",
}

type ConfigMeta struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Description  string `json:"description"`
	Type         string `json:"type"`
	DefaultValue string `json:"default_value"`
	Value        string `json:"value"`
	MinValue     *int   `json:"min_value,omitempty"`
	MaxValue     *int   `json:"max_value,omitempty"`
	Unit         string `json:"unit,omitempty"`
	Group        string `json:"group"`
}

var configMetaList = []ConfigMeta{
	{
		Key: "web.enabled", Label: "启用内置 Web 服务",
		Description: "启用后，可通过调度器监听端口直接访问 Web UI，无需单独部署前端。禁用后仅提供 API 服务。",
		Type:        "boolean", DefaultValue: "false",
		Group: "系统",
	},
	{
		Key: "wecom.robot_url", Label: "企业微信群机器人 URL",
		Description: "用于推送 BDopsFlow 任务执行通知到企业微信群的机器人接口地址",
		Type:        "text", DefaultValue: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send",
		Group: "消息通知",
	},
	{
		Key: "datasource.default_limit", Label: "默认查询行数",
		Description: "SQL 查询默认返回的最大行数，限制单次查询结果集大小，防止返回过多数据导致前端卡顿",
		Type:        "number", DefaultValue: "1000", MinValue: intPtr(1), MaxValue: intPtr(100000), Unit: "行",
		Group: "查询",
	},
	{
		Key: "datasource.max_export_rows", Label: "最大导出行数",
		Description: "CSV 导出时允许的最大行数，导出超过此限制将截断结果",
		Type:        "number", DefaultValue: "1000", MinValue: intPtr(1), MaxValue: intPtr(1000000), Unit: "行",
		Group: "查询",
	},
	{
		Key: "datasource.query_timeout", Label: "查询超时时间",
		Description: "单次 SQL 查询的最大执行时间，超时后自动取消查询并返回错误",
		Type:        "number", DefaultValue: "60", MinValue: intPtr(1), MaxValue: intPtr(3600), Unit: "秒",
		Group: "查询",
	},
	{
		Key: "datasource.max_sql_length", Label: "最大 SQL 长度",
		Description: "允许提交的 SQL 语句最大字符数，防止超长 SQL 影响系统性能",
		Type:        "number", DefaultValue: "65536", MinValue: intPtr(1024), MaxValue: intPtr(1048576), Unit: "字符",
		Group: "查询",
	},
	{
		Key: "datasource.max_cell_size", Label: "最大单元格大小",
		Description: "查询结果中单个单元格数据的最大字节数，超过此大小将截断显示",
		Type:        "number", DefaultValue: "65536", MinValue: intPtr(1024), MaxValue: intPtr(10485760), Unit: "字节",
		Group: "查询",
	},
	{
		Key: "datasource.max_concurrent_per_user", Label: "单用户最大并发查询",
		Description: "单个用户同时执行查询的最大数量，超过限制将排队等待",
		Type:        "number", DefaultValue: "5", MinValue: intPtr(1), MaxValue: intPtr(50), Unit: "个",
		Group: "并发",
	},
	{
		Key: "datasource.max_concurrent_global", Label: "全局最大并发查询",
		Description: "系统全局同时执行查询的最大数量，超过限制将排队等待",
		Type:        "number", DefaultValue: "50", MinValue: intPtr(1), MaxValue: intPtr(500), Unit: "个",
		Group: "并发",
	},
	{
		Key: "datasource.allow_write_sql", Label: "允许 DML 语句（全局）",
		Description: "全局开关，控制是否允许执行 INSERT/UPDATE/DELETE 等 DML 语句。注意：每个数据源可独立设置 DML 权限，此选项为全局兜底控制",
		Type:        "boolean", DefaultValue: "false",
		Group: "安全",
	},
	{
		Key: "datasource.cache_ttl", Label: "缓存过期时间",
		Description: "数据源元数据（表结构、列信息等）缓存的存活时间，过期后下次查询将重新获取",
		Type:        "number", DefaultValue: "300", MinValue: intPtr(0), MaxValue: intPtr(86400), Unit: "秒",
		Group: "缓存",
	},
	{
		Key: "datasource.cache_max_size", Label: "缓存最大条目数",
		Description: "元数据缓存的最大条目数量，超过后采用 LRU 淘汰策略",
		Type:        "number", DefaultValue: "100", MinValue: intPtr(1), MaxValue: intPtr(10000), Unit: "条",
		Group: "缓存",
	},
	{
		Key: "datasource.connection_max_idle", Label: "最大空闲连接数",
		Description: "每个数据源连接池中允许保持的最大空闲连接数",
		Type:        "number", DefaultValue: "5", MinValue: intPtr(1), MaxValue: intPtr(100), Unit: "个",
		Group: "连接池",
	},
	{
		Key: "datasource.connection_max_open", Label: "最大打开连接数",
		Description: "每个数据源连接池中允许的最大打开连接数，包括活跃和空闲连接",
		Type:        "number", DefaultValue: "10", MinValue: intPtr(1), MaxValue: intPtr(200), Unit: "个",
		Group: "连接池",
	},
	{
		Key: "datasource.connection_max_lifetime", Label: "连接最大生命周期",
		Description: "连接池中连接的最大存活时间，超时后连接将被关闭并重建，防止长时间使用同一连接",
		Type:        "number", DefaultValue: "1800", MinValue: intPtr(60), MaxValue: intPtr(86400), Unit: "秒",
		Group: "连接池",
	},
	{
		Key: "datasource.health_check_interval", Label: "健康检查间隔",
		Description: "数据源连接健康检查的执行间隔，定期检测连接是否可用",
		Type:        "number", DefaultValue: "300", MinValue: intPtr(30), MaxValue: intPtr(3600), Unit: "秒",
		Group: "连接池",
	},
	{
		Key: "datasource.test_timeout", Label: "测试连接超时",
		Description: "测试数据源连接时的超时时间，超时未响应视为连接失败",
		Type:        "number", DefaultValue: "10", MinValue: intPtr(1), MaxValue: intPtr(120), Unit: "秒",
		Group: "连接池",
	},
	{
		Key: "datasource.history_retention_days", Label: "历史记录保留天数",
		Description: "查询历史记录的保留天数，超过此天数的记录将被自动清理",
		Type:        "number", DefaultValue: "30", MinValue: intPtr(1), MaxValue: intPtr(365), Unit: "天",
		Group: "其他",
	},
}

func intPtr(v int) *int {
	return &v
}

type ConfigService struct {
	db    database.DB
	cache map[string]string
	mu    sync.RWMutex
}

func NewConfigService(db database.DB) *ConfigService {
	s := &ConfigService{
		db:    db,
		cache: make(map[string]string),
	}
	// 加载配置
	if err := s.Reload(context.Background()); err != nil {
		slog.Warn("failed to load datasource config, using defaults", "error", err)
		s.cache = defaultConfigValues
	}
	return s
}

func (s *ConfigService) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.cache[key]; ok {
		return v
	}
	if v, ok := defaultConfigValues[key]; ok {
		return v
	}
	return ""
}

func (s *ConfigService) GetInt(key string) int {
	v := s.Get(key)
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

func (s *ConfigService) GetBool(key string) bool {
	v := s.Get(key)
	return v == "true" || v == "1"
}

func (s *ConfigService) GetAll() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range s.cache {
		result[k] = v
	}
	return result
}

func (s *ConfigService) GetAllWithMeta() []ConfigMeta {
	s.mu.RLock()
	cacheCopy := make(map[string]string)
	for k, v := range s.cache {
		cacheCopy[k] = v
	}
	s.mu.RUnlock()

	result := make([]ConfigMeta, len(configMetaList))
	for i, meta := range configMetaList {
		m := meta
		if v, ok := cacheCopy[meta.Key]; ok {
			m.Value = v
		} else {
			m.Value = meta.DefaultValue
		}
		result[i] = m
	}
	return result
}

func (s *ConfigService) Set(ctx context.Context, key, value string, changedBy int64) error {
	var validators = map[string]func(string) error{
		"datasource.cache_ttl": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return fmt.Errorf("must be non-negative integer")
			}
			return nil
		},
		"datasource.query_timeout": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be positive integer")
			}
			return nil
		},
		"datasource.max_concurrent_per_user": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be positive integer")
			}
			return nil
		},
		"datasource.max_concurrent_global": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be positive integer")
			}
			return nil
		},
		"datasource.default_limit": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be positive integer")
			}
			return nil
		},
		"datasource.max_export_rows": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be positive integer")
			}
			return nil
		},
		"datasource.allow_write_sql": func(v string) error {
			if v != "true" && v != "false" {
				return fmt.Errorf("must be true or false")
			}
			return nil
		},
		"web.enabled": func(v string) error {
			if v != "true" && v != "false" {
				return fmt.Errorf("must be true or false")
			}
			return nil
		},
	}

	if validator, ok := validators[key]; ok {
		if err := validator(value); err != nil {
			return fmt.Errorf("invalid value for %s: %w", key, err)
		}
	}

	oldValue := s.Get(key)

	// 首先尝试更新
	now := time.Now().Format(dsDateTimeFormat)
	result, err := s.db.WriteOneParameterized(
		rqlite.ParameterizedStatement{
			Query:     "UPDATE bdopsflow_system_config SET config_value = ?, updated_at = ? WHERE config_key = ?",
			Arguments: []interface{}{value, now, key},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}
	if result.Err != nil {
		return fmt.Errorf("failed to update config: %w", result.Err)
	}

	// 如果没有行被更新，说明配置不存在，需要插入
	if result.RowsAffected == 0 {
		slog.Info("config not found, inserting new", "key", key, "value", value)
		_, err = s.db.WriteOneParameterized(
			rqlite.ParameterizedStatement{
				Query:     "INSERT INTO bdopsflow_system_config (config_key, config_value, updated_at) VALUES (?, ?, ?)",
				Arguments: []interface{}{key, value, now},
			},
		)
		if err != nil {
			return fmt.Errorf("failed to insert config: %w", err)
		}
	}

	_, err = s.db.WriteOneParameterized(
		rqlite.ParameterizedStatement{
			Query:     "INSERT INTO bdopsflow_system_config_history (config_key, old_value, new_value, changed_by, changed_at) VALUES (?, ?, ?, ?, ?)",
			Arguments: []interface{}{key, oldValue, value, changedBy, now},
		},
	)
	if err != nil {
		slog.Warn("failed to record config history", "key", key, "error", err)
	}

	s.mu.Lock()
	s.cache[key] = value
	s.mu.Unlock()

	return nil
}

func (s *ConfigService) Reload(ctx context.Context) error {
	qr, err := s.db.QueryOneParameterized(
		rqlite.ParameterizedStatement{
			Query: "SELECT config_key, config_value FROM bdopsflow_system_config",
		},
	)
	if err != nil {
		return fmt.Errorf("failed to query config: %w", err)
	}
	if qr.Err != nil {
		return fmt.Errorf("failed to query config: %w", qr.Err)
	}

	newCache := make(map[string]string)
	// 先复制默认配置
	for k, v := range defaultConfigValues {
		newCache[k] = v
	}
	// 再用数据库中的配置覆盖
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			return fmt.Errorf("failed to slice config row: %w", err)
		}
		key, _ := row[0].(string)
		value, _ := row[1].(string)
		newCache[key] = value
	}

	s.mu.Lock()
	s.cache = newCache
	s.mu.Unlock()

	return nil
}

func (s *ConfigService) StartReloadTicker(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := s.Reload(context.Background()); err != nil {
				slog.Warn("failed to reload datasource config", "error", err)
			}
		}
	}()
}
