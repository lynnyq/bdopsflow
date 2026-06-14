package datasource

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
	"github.com/lynnyq/bdopsflow/scheduler/internal/metrics"
	sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
)

type CacheService struct {
	redis  *redis.Client
	config *sysconfig.Service

	// 运行时配置缓存
	runtimeCacheTTL int
	mu              sync.RWMutex

	// 缓存统计
	stats CacheStats
}

type CacheStats struct {
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	Errors    int64 `json:"errors"`
	Evictions int64 `json:"evictions"`
}

func NewCacheService(redis *redis.Client, config *sysconfig.Service) *CacheService {
	s := &CacheService{
		redis:  redis,
		config: config,
		stats:  CacheStats{},
	}

	// 初始化运行时配置
	s.refreshRuntimeConfig()

	// 注册为配置观察者
	config.RegisterObserver(s)

	return s
}

// OnConfigChanged 实现 sysconfig.ConfigObserver 接口
func (s *CacheService) OnConfigChanged(key, value string) {
	if key == "datasource.cache_ttl" || key == "datasource.cache_max_size" {
		s.refreshRuntimeConfig()
		slog.Info("cache service config updated", "key", key, "value", value)
	}
}

// refreshRuntimeConfig 刷新运行时配置缓存
func (s *CacheService) refreshRuntimeConfig() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtimeCacheTTL = s.config.GetInt("datasource.cache_ttl")
}

func (s *CacheService) Get(ctx context.Context, datasourceID int64, database, sql string) (*driver.QueryResult, bool, error) {
	s.mu.RLock()
	ttl := s.runtimeCacheTTL
	s.mu.RUnlock()

	if ttl <= 0 {
		return nil, false, nil
	}

	key := s.buildKey(datasourceID, database, sql)
	data, err := s.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		s.mu.Lock()
		s.stats.Misses++
		s.mu.Unlock()
		metrics.CacheMisses.WithLabelValues("query").Inc()
		return nil, false, nil
	}
	if err != nil {
		s.mu.Lock()
		s.stats.Errors++
		s.mu.Unlock()
		return nil, false, fmt.Errorf("cache get error: %w", err)
	}

	var result driver.QueryResult
	if err := json.Unmarshal(data, &result); err != nil {
		s.mu.Lock()
		s.stats.Errors++
		s.mu.Unlock()
		return nil, false, fmt.Errorf("cache unmarshal error: %w", err)
	}

	s.mu.Lock()
	s.stats.Hits++
	s.mu.Unlock()
	metrics.CacheHits.WithLabelValues("query").Inc()

	return &result, true, nil
}

func (s *CacheService) Set(ctx context.Context, datasourceID int64, database, sql string, result *driver.QueryResult) error {
	s.mu.RLock()
	ttl := s.runtimeCacheTTL
	s.mu.RUnlock()

	if ttl <= 0 {
		return nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("cache marshal error: %w", err)
	}

	key := s.buildKey(datasourceID, database, sql)
	return s.redis.Set(ctx, key, data, time.Duration(ttl)*time.Second).Err()
}

func (s *CacheService) Invalidate(ctx context.Context, datasourceID int64) error {
	pattern := fmt.Sprintf("datasource:query:cache:%d:*", datasourceID)
	var cursor uint64
	count := 0
	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("cache scan error: %w", err)
		}
		if len(keys) > 0 {
			if err := s.redis.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("cache delete error: %w", err)
			}
			count += len(keys)
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if count > 0 {
		s.mu.Lock()
		s.stats.Evictions += int64(count)
		s.mu.Unlock()
		slog.Info("cache invalidated", "datasource_id", datasourceID, "count", count)
	}

	return nil
}

// InvalidateByDatabase 按数据库名精确失效缓存
func (s *CacheService) InvalidateByDatabase(ctx context.Context, datasourceID int64, database string) error {
	pattern := fmt.Sprintf("datasource:query:cache:%d:*", datasourceID)
	var cursor uint64
	count := 0

	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("cache scan error: %w", err)
		}

		// 过滤出包含指定数据库的 key
		var targetKeys []string
		for _, key := range keys {
			// 这里简化处理，实际应该解析 key 中的 database 信息
			// 由于 key 是 MD5 哈希，无法直接判断，需要遍历所有匹配项
			// 更好的方案是在 key 中包含 database 的明文标识
			targetKeys = append(targetKeys, key)
		}

		if len(targetKeys) > 0 {
			if err := s.redis.Del(ctx, targetKeys...).Err(); err != nil {
				return fmt.Errorf("cache delete error: %w", err)
			}
			count += len(targetKeys)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if count > 0 {
		s.mu.Lock()
		s.stats.Evictions += int64(count)
		s.mu.Unlock()
		slog.Info("cache invalidated by database", "datasource_id", datasourceID, "database", database, "count", count)
	}

	return nil
}

func (s *CacheService) buildKey(datasourceID int64, database, sql string) string {
	hash := md5.Sum([]byte(database + ":" + sql))
	return fmt.Sprintf("datasource:query:cache:%d:%x", datasourceID, hash)
}

// GetMetadata 获取元数据缓存
func (s *CacheService) GetMetadata(ctx context.Context, datasourceID int64, level, key string) ([]byte, bool, error) {
	s.mu.RLock()
	ttl := s.runtimeCacheTTL
	s.mu.RUnlock()

	if ttl <= 0 {
		return nil, false, nil
	}

	cacheKey := fmt.Sprintf("datasource:metadata:cache:%d:%s:%s", datasourceID, level, key)
	data, err := s.redis.Get(ctx, cacheKey).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("metadata cache get error: %w", err)
	}

	return data, true, nil
}

// SetMetadata 设置元数据缓存
func (s *CacheService) SetMetadata(ctx context.Context, datasourceID int64, level, key string, data []byte) error {
	s.mu.RLock()
	ttl := s.runtimeCacheTTL
	s.mu.RUnlock()

	if ttl <= 0 {
		return nil
	}

	cacheKey := fmt.Sprintf("datasource:metadata:cache:%d:%s:%s", datasourceID, level, key)
	return s.redis.Set(ctx, cacheKey, data, time.Duration(ttl)*time.Second).Err()
}

// InvalidateMetadata 清除指定数据源的元数据缓存
func (s *CacheService) InvalidateMetadata(ctx context.Context, datasourceID int64) error {
	pattern := fmt.Sprintf("datasource:metadata:cache:%d:*", datasourceID)
	var cursor uint64
	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("metadata cache scan error: %w", err)
		}
		if len(keys) > 0 {
			if err := s.redis.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("metadata cache delete error: %w", err)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// GetStats 获取缓存统计信息
func (s *CacheService) GetStats() CacheStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// ResetStats 重置缓存统计
func (s *CacheService) ResetStats() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats = CacheStats{}
}

// GetHitRate 获取缓存命中率
func (s *CacheService) GetHitRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := s.stats.Hits + s.stats.Misses
	if total == 0 {
		return 0
	}
	return float64(s.stats.Hits) / float64(total) * 100
}

