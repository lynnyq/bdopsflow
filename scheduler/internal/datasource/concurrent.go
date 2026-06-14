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

	// 校准相关
	calibrationInterval time.Duration
	stopCalibration     chan struct{}
	getActualCount      func() (userCounts map[int64]int64, globalCount int64, dsCounts map[int64]int64)
}

func NewConcurrentService(redis *redis.Client, config *sysconfig.Service) *ConcurrentService {
	s := &ConcurrentService{
		redis:               redis,
		config:              config,
		calibrationInterval: 5 * time.Minute,
		stopCalibration:     make(chan struct{}),
	}

	// 初始化运行时配置
	s.refreshRuntimeConfig()

	// 注册为配置观察者
	config.RegisterObserver(s)

	return s
}

// StartCalibration 启动定期校准任务
func (s *ConcurrentService) StartCalibration(getActualCount func() (userCounts map[int64]int64, globalCount int64, dsCounts map[int64]int64)) {
	s.getActualCount = getActualCount

	go func() {
		ticker := time.NewTicker(s.calibrationInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.calibrateCounters()
			case <-s.stopCalibration:
				slog.Info("concurrent service calibration stopped")
				return
			}
		}
	}()

	slog.Info("concurrent service calibration started", "interval", s.calibrationInterval)
}

// StopCalibration 停止校准任务
func (s *ConcurrentService) StopCalibration() {
	close(s.stopCalibration)
}

// calibrateCounters 基于实际运行的查询数量校准计数器
func (s *ConcurrentService) calibrateCounters() {
	if s.getActualCount == nil {
		return
	}

	userCounts, globalCount, dsCounts := s.getActualCount()
	ctx := context.Background()

	// 校准用户并发计数
	for userID, actualCount := range userCounts {
		userKey := fmt.Sprintf("datasource:query:concurrent:user:%d", userID)
		redisCount, err := s.redis.Get(ctx, userKey).Int64()
		if err != nil && err != redis.Nil {
			slog.Warn("failed to get user concurrent count for calibration", "user_id", userID, "error", err)
			continue
		}

		if redisCount != actualCount {
			slog.Warn("calibrating user concurrent count",
				"user_id", userID,
				"redis_count", redisCount,
				"actual_count", actualCount,
				"diff", redisCount-actualCount)

			if actualCount == 0 {
				s.redis.Del(ctx, userKey)
			} else {
				s.redis.Set(ctx, userKey, actualCount, 5*time.Minute)
			}
		}
	}

	// 校准全局并发计数
	redisGlobalCount, err := s.redis.Get(ctx, "datasource:query:concurrent:global").Int64()
	if err != nil && err != redis.Nil {
		slog.Warn("failed to get global concurrent count for calibration", "error", err)
	} else if redisGlobalCount != globalCount {
		slog.Warn("calibrating global concurrent count",
			"redis_count", redisGlobalCount,
			"actual_count", globalCount,
			"diff", redisGlobalCount-globalCount)

		if globalCount == 0 {
			s.redis.Del(ctx, "datasource:query:concurrent:global")
		} else {
			s.redis.Set(ctx, "datasource:query:concurrent:global", globalCount, 5*time.Minute)
		}
	}

	// 校准数据源并发计数
	for dsID, actualCount := range dsCounts {
		dsKey := fmt.Sprintf("datasource:query:concurrent:ds:%d", dsID)
		redisCount, err := s.redis.Get(ctx, dsKey).Int64()
		if err != nil && err != redis.Nil {
			slog.Warn("failed to get datasource concurrent count for calibration", "datasource_id", dsID, "error", err)
			continue
		}

		if redisCount != actualCount {
			slog.Warn("calibrating datasource concurrent count",
				"datasource_id", dsID,
				"redis_count", redisCount,
				"actual_count", actualCount,
				"diff", redisCount-actualCount)

			if actualCount == 0 {
				s.redis.Del(ctx, dsKey)
			} else {
				s.redis.Set(ctx, dsKey, actualCount, 5*time.Minute)
			}
		}
	}
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
