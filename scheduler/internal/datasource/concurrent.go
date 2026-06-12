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
	runtimeMaxPerUser        int
	runtimeMaxGlobal         int
	runtimeMaxPerDatasource  int
	mu                       sync.RWMutex
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
	if key == "datasource.max_concurrent_per_user" || key == "datasource.max_concurrent_global" || key == "datasource.max_concurrent_per_datasource" {
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
	s.runtimeMaxPerDatasource = s.config.GetInt("datasource.max_concurrent_per_datasource")

	if s.runtimeMaxPerUser <= 0 {
		s.runtimeMaxPerUser = 5
	}
	if s.runtimeMaxGlobal <= 0 {
		s.runtimeMaxGlobal = 50
	}
	if s.runtimeMaxPerDatasource <= 0 {
		s.runtimeMaxPerDatasource = 10
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

// AcquireForDatasource 获取并发查询许可，包含全局、用户和数据源维度的限制
func (s *ConcurrentService) AcquireForDatasource(ctx context.Context, userID int64, datasourceID int64) (func(), error) {
	s.mu.RLock()
	maxPerUser := s.runtimeMaxPerUser
	maxGlobal := s.runtimeMaxGlobal
	maxPerDatasource := s.runtimeMaxPerDatasource
	s.mu.RUnlock()

	userKey := fmt.Sprintf("datasource:query:concurrent:user:%d", userID)
	globalKey := "datasource:query:concurrent:global"
	dsKey := fmt.Sprintf("datasource:query:concurrent:ds:%d", datasourceID)

	// 1. 检查用户并发限制
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

	// 2. 检查全局并发限制
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

	// 3. 检查数据源并发限制
	dsCount, err := s.redis.Incr(ctx, dsKey).Result()
	if err != nil {
		s.redis.Decr(ctx, userKey)
		s.redis.Decr(ctx, globalKey)
		return nil, fmt.Errorf("concurrent check error: %w", err)
	}
	if dsCount == 1 {
		s.redis.Expire(ctx, dsKey, 5*time.Minute)
	}

	if dsCount > int64(maxPerDatasource) {
		s.redis.Decr(ctx, userKey)
		s.redis.Decr(ctx, globalKey)
		s.redis.Decr(ctx, dsKey)
		return nil, ErrDatasourceConcurrentLimit
	}

	released := false
	release := func() {
		if released {
			return
		}
		released = true
		s.redis.Decr(ctx, userKey)
		s.redis.Decr(ctx, globalKey)
		s.redis.Decr(ctx, dsKey)
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

func (s *ConcurrentService) GetDatasourceConcurrent(ctx context.Context, datasourceID int64) (int64, error) {
	dsKey := fmt.Sprintf("datasource:query:concurrent:ds:%d", datasourceID)
	return s.redis.Get(ctx, dsKey).Int64()
}

func (s *ConcurrentService) SetCancelSignal(ctx context.Context, queryID string, ttl time.Duration) error {
	cancelKey := fmt.Sprintf("datasource:query:cancel:%s", queryID)
	return s.redis.Set(ctx, cancelKey, "1", ttl).Err()
}
