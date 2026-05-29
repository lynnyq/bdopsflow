package datasource

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type ConcurrentService struct {
	redis  *redis.Client
	config *ConfigService
}

func NewConcurrentService(redis *redis.Client, config *ConfigService) *ConcurrentService {
	return &ConcurrentService{
		redis:  redis,
		config: config,
	}
}

func (s *ConcurrentService) Acquire(ctx context.Context, userID int64) (func(), error) {
	maxPerUser := s.config.GetInt("datasource.max_concurrent_per_user")
	maxGlobal := s.config.GetInt("datasource.max_concurrent_global")
	if maxPerUser <= 0 {
		maxPerUser = 5
	}
	if maxGlobal <= 0 {
		maxGlobal = 50
	}

	userKey := fmt.Sprintf("datasource:query:concurrent:user:%d", userID)
	globalKey := "datasource:query:concurrent:global"

	userCount, err := s.redis.Incr(ctx, userKey).Result()
	if err != nil {
		return nil, fmt.Errorf("concurrent check error: %w", err)
	}
	if userCount == 1 {
		s.redis.Expire(ctx, userKey, 5*time.Minute)
	}

	if userCount > int64(maxPerUser) {
		s.redis.Decr(ctx, userKey)
		return nil, ErrConcurrentLimit
	}

	globalCount, err := s.redis.Incr(ctx, globalKey).Result()
	if err != nil {
		s.redis.Decr(ctx, userKey)
		return nil, fmt.Errorf("concurrent check error: %w", err)
	}
	if globalCount == 1 {
		s.redis.Expire(ctx, globalKey, 5*time.Minute)
	}

	if globalCount > int64(maxGlobal) {
		s.redis.Decr(ctx, userKey)
		s.redis.Decr(ctx, globalKey)
		return nil, ErrConcurrentLimit
	}

	released := false
	release := func() {
		if released {
			return
		}
		released = true
		s.redis.Decr(ctx, userKey)
		s.redis.Decr(ctx, globalKey)
	}

	return release, nil
}

func (s *ConcurrentService) GetUserConcurrent(ctx context.Context, userID int64) (int64, error) {
	userKey := fmt.Sprintf("datasource:query:concurrent:user:%d", userID)
	return s.redis.Get(ctx, userKey).Int64()
}

func (s *ConcurrentService) GetGlobalConcurrent(ctx context.Context) (int64, error) {
	globalKey := "datasource:query:concurrent:global"
	return s.redis.Get(ctx, globalKey).Int64()
}

func (s *ConcurrentService) SetCancelSignal(ctx context.Context, queryID string, ttl time.Duration) error {
	cancelKey := fmt.Sprintf("datasource:query:cancel:%s", queryID)
	return s.redis.Set(ctx, cancelKey, "1", ttl).Err()
}

func (s *ConcurrentService) HasCancelSignal(ctx context.Context, queryID string) bool {
	if queryID == "" {
		return false
	}
	cancelKey := fmt.Sprintf("datasource:query:cancel:%s", queryID)
	val, err := s.redis.Get(ctx, cancelKey).Result()
	if err != nil {
		return false
	}
	return val == "1"
}
