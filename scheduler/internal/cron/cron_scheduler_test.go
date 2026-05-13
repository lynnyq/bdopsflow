package cron

import (
	"testing"
	"time"
)

func TestCronScheduler_New(t *testing.T) {
	// 测试创建新调度器
	scheduler := NewCronScheduler(nil, nil)
	if scheduler == nil {
		t.Fatal("Expected scheduler to be created, got nil")
	}
}

func TestCronScheduler_RegisterAndUnregister(t *testing.T) {
	scheduler := NewCronScheduler(nil, nil)
	
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	taskID := int64(1)
	
	// 测试注册任务
	scheduler.RegisterTask(taskID, "@every 1s")
	
	// 测试取消注册任务
	scheduler.UnregisterTask(taskID)
}

func TestCronScheduler_CronExpressionFormats(t *testing.T) {
	scheduler := NewCronScheduler(nil, nil)
	
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	testCases := []struct {
		name     string
		cronExpr string
	}{
		{
			name:     "simple @every",
			cronExpr: "@every 10s",
		},
		{
			name:     "6-field with seconds",
			cronExpr: "0/30 * * * * *",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskID := time.Now().UnixNano()
			// 应该能成功注册
			scheduler.RegisterTask(taskID, tc.cronExpr)
		})
	}
}
