package datasource

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
)

type ConcurrentService struct {
	redis  *redis.Client
	config *sysconfig.Service

	// 运行时配置缓存
	runtimeMaxPerUser int
	runtimeMaxGlobal  int
	mu                sync.RWMutex
}

func NewConcurrentService(redis *redis.Client, config *sysconfig.Service) *ConcurrentService {
	s := &ConcurrentService{
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
func (s *ConcurrentService) OnConfigChanged(key, value string) {
	if key == "datasource.max_concurrent_per_user" || key == "datasource.max_concurrent_global" {
		s.refreshRuntimeConfig()
		slog.Info("concurrent service config updated", "key", key, "value", value)
	}
}

// refreshRuntimeConfig 刷新运行时配置缓存
func (s *ConcurrentService) refreshRuntimeConfig() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtimeMaxPerUser = s.config.GetInt("datasource.max_concurrent_per_user")
	s.runtimeMaxGlobal = s.config.GetInt("datasource.max_concurrent_global")

	if s.runtimeMaxPerUser <= 0 {
		s.runtimeMaxPerUser = 5
	}
	if s.runtimeMaxGlobal <= 0 {
		s.runtimeMaxGlobal = 50
	}
}

func (s *ConcurrentService) Acquire(ctx context.Context, userID int64) (func(), error) {
	s.mu.RLock()
	maxPerUser := s.runtimeMaxPerUser
	maxGlobal := s.runtimeMaxGlobal
	s.mu.RUnlock()

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
