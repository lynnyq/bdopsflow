package lock

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisLock_TryLock(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		ttl     time.Duration
		want    bool
		wantErr bool
	}{
		{
			name:    "lock success",
			key:     "test-key",
			ttl:     30 * time.Second,
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := redis.NewClient(&redis.Options{
				Addr: "localhost:6379",
				DB:   15,
			})
			lock := NewRedisLock(client)

			err := client.FlushDB(ctx).Err()
			if err != nil {
				t.Skip("Redis not available, skipping test")
			}

			got, err := lock.TryLock(ctx, tt.key, tt.ttl)

			if (err != nil) != tt.wantErr {
				t.Errorf("TryLock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("TryLock() = %v, want %v", got, tt.want)
			}

			client.FlushDB(ctx)
			client.Close()
		})
	}
}

func TestRedisLock_Unlock(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer client.Close()

	err := client.FlushDB(ctx).Err()
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}

	lock := NewRedisLock(client)

	err = lock.Unlock(ctx, "test-key")
	if err != nil {
		t.Errorf("Unlock() error = %v, want no error", err)
	}
}

func TestRedisLock_KeepAlive(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer client.Close()

	err := client.FlushDB(ctx).Err()
	if err != nil {
		t.Skip("Redis not available, skipping test")
	}

	lock := NewRedisLock(client)

	err = lock.KeepAlive(ctx, "test-key", 30*time.Second)
	if err != nil {
		t.Errorf("KeepAlive() error = %v, want no error", err)
	}
}

func TestRedisLock_Prefix(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	lock := NewRedisLock(client)
	client.Close()

	if lock.prefix != "bdopsflow:lock:" {
		t.Errorf("expected prefix 'bdopsflow:lock:', got '%s'", lock.prefix)
	}
}
