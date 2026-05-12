package cron

import (
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

func TestNewCronScheduler(t *testing.T) {
	// 只需测试类型和初始化逻辑，无需完整的数据库连接
	svc := &service.SchedulerService{}
	scheduler := NewCronScheduler(svc)
	if scheduler == nil {
		t.Fatal("expected scheduler to be created")
	}
	if scheduler.svc != svc {
		t.Fatal("expected service to be set correctly")
	}
}

func TestCronScheduler_StartStop(t *testing.T) {
	// 测试 Start 和 Stop 方法不会 panic
	svc := &service.SchedulerService{}
	scheduler := NewCronScheduler(svc)

	// 启动调度器
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("expected no error on start, got: %v", err)
	}

	// 停止调度器
	scheduler.Stop()
}
