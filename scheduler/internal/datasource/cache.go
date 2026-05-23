package datasource

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
)

type CacheService struct {
	redis  *redis.Client
	config *ConfigService
}

func NewCacheService(redis *redis.Client, config *ConfigService) *CacheService {
	return &CacheService{
		redis:  redis,
		config: config,
	}
}

func (s *CacheService) Get(ctx context.Context, datasourceID int64, sql string) (*driver.QueryResult, bool, error) {
	ttl := s.config.GetInt("datasource.cache_ttl")
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
	ttl := s.config.GetInt("datasource.cache_ttl")
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
