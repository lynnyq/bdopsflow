package cron

import (
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

func TestNewCronScheduler(t *testing.T) {
	svc := &service.SchedulerService{}
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	scheduler := NewCronScheduler(svc, redisClient)
	if scheduler == nil {
		t.Fatal("expected scheduler to be created")
	}
	if scheduler.svc != svc {
		t.Fatal("expected service to be set correctly")
	}
}

func TestCronScheduler_StartStop(t *testing.T) {
	svc := &service.SchedulerService{}
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	scheduler := NewCronScheduler(svc, redisClient)

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("expected no error on start, got: %v", err)
	}

	scheduler.Stop()
}
