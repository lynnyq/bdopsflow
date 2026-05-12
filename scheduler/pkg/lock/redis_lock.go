package lock

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLock struct {
	client *redis.Client
	prefix string
}

func NewRedisLock(client *redis.Client) *RedisLock {
	return &RedisLock{
		client: client,
		prefix: "bdopsflow:lock:",
	}
}

func (l *RedisLock) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	fullKey := l.prefix + key
	ok, err := l.client.SetNX(ctx, fullKey, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("try lock failed: %w", err)
	}
	return ok, nil
}

func (l *RedisLock) Unlock(ctx context.Context, key string) error {
	fullKey := l.prefix + key
	_, err := l.client.Del(ctx, fullKey).Result()
	if err != nil {
		return fmt.Errorf("unlock failed: %w", err)
	}
	return nil
}

func (l *RedisLock) KeepAlive(ctx context.Context, key string, ttl time.Duration) error {
	fullKey := l.prefix + key
	_, err := l.client.Expire(ctx, fullKey, ttl).Result()
	if err != nil {
		return fmt.Errorf("keep alive failed: %w", err)
	}
	return nil
}
