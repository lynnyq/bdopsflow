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
	sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
)

type CacheService struct {
	redis  *redis.Client
	config *sysconfig.Service

	// 运行时配置缓存
	runtimeCacheTTL int
	mu              sync.RWMutex
}

func NewCacheService(redis *redis.Client, config *sysconfig.Service) *CacheService {
	s := &CacheService{
		redis:  redis,
		config: config,
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

func (s *CacheService) Get(ctx context.Context, datasourceID int64, sql string) (*driver.QueryResult, bool, error) {
	s.mu.RLock()
	ttl := s.runtimeCacheTTL
	s.mu.RUnlock()

	if ttl <= 0 {
		return nil, false, nil
	}

	key := s.buildKey(datasourceID, sql)
	data, err := s.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("cache get error: %w", err)
	}

	var result driver.QueryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false, fmt.Errorf("cache unmarshal error: %w", err)
	}

	return &result, true, nil
}

func (s *CacheService) Set(ctx context.Context, datasourceID int64, sql string, result *driver.QueryResult) error {
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

	key := s.buildKey(datasourceID, sql)
	return s.redis.Set(ctx, key, data, time.Duration(ttl)*time.Second).Err()
}

func (s *CacheService) Invalidate(ctx context.Context, datasourceID int64) error {
	pattern := fmt.Sprintf("datasource:query:cache:%d:*", datasourceID)
	var cursor uint64
	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("cache scan error: %w", err)
		}
		if len(keys) > 0 {
			if err := s.redis.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("cache delete error: %w", err)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (s *CacheService) buildKey(datasourceID int64, sql string) string {
	hash := md5.Sum([]byte(sql))
	return fmt.Sprintf("datasource:query:cache:%d:%x", datasourceID, hash)
}
