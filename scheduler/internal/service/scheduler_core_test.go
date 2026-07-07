package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
)

// ============ ForwardToLeader ============

func TestSchedulerService_ForwardToLeader(t *testing.T) {
	ctx := context.Background()

	t.Run("resolver 未配置", func(t *testing.T) {
		svc := &SchedulerService{}
		_, status, err := svc.ForwardToLeader(ctx, "GET", "/api/v1/tasks", nil)
		if err == nil {
			t.Fatal("期望返回错误")
		}
		if status != 503 {
			t.Errorf("期望 status=503，实际=%d", status)
		}
	})

	t.Run("resolver 返回错误", func(t *testing.T) {
		svc := &SchedulerService{}
		svc.SetLeaderAddrResolver(&mockLeaderAddrResolver{err: ErrMockDB})
		_, status, err := svc.ForwardToLeader(ctx, "GET", "/api/v1/tasks", nil)
		if err == nil {
			t.Fatal("期望返回错误")
		}
		if status != 503 {
			t.Errorf("期望 status=503，实际=%d", status)
		}
	})

	t.Run("leader 地址为空", func(t *testing.T) {
		svc := &SchedulerService{}
		svc.SetLeaderAddrResolver(&mockLeaderAddrResolver{leaderAddr: ""})
		_, status, err := svc.ForwardToLeader(ctx, "GET", "/api/v1/tasks", nil)
		if err == nil {
			t.Fatal("期望返回错误")
		}
		if status != 503 {
			t.Errorf("期望 status=503，实际=%d", status)
		}
	})
}

// ============ GetDomainName ============

func TestSchedulerService_GetDomainName(t *testing.T) {
	ctx := context.Background()

	t.Run("查询成功", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{"production"},
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		name := svc.GetDomainName(ctx, 1)
		if name != "production" {
			t.Errorf("期望 name=production，实际=%s", name)
		}
	})

	t.Run("DB 错误返回默认名称", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		name := svc.GetDomainName(ctx, 1)
		if name != "领域 1" {
			t.Errorf("期望 默认名称，实际=%s", name)
		}
	})

	t.Run("查询结果带错误返回默认名称", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := newSchedulerWithDB(db)

		name := svc.GetDomainName(ctx, 5)
		if name != "领域 5" {
			t.Errorf("期望 默认名称，实际=%s", name)
		}
	})

	t.Run("无行返回默认名称", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)

		name := svc.GetDomainName(ctx, 99)
		if name != "领域 99" {
			t.Errorf("期望 默认名称，实际=%s", name)
		}
	})

	t.Run("空名称返回默认名称", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{""},
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		name := svc.GetDomainName(ctx, 3)
		if name != "领域 3" {
			t.Errorf("期望 默认名称，实际=%s", name)
		}
	})
}

// ============ SendWebhookNotification ============

func TestSchedulerService_SendWebhookNotification(t *testing.T) {
	ctx := context.Background()

	t.Run("webhookSvc 为 nil 直接返回", func(t *testing.T) {
		svc := &SchedulerService{}
		// 不应 panic
		svc.SendWebhookNotification(ctx, 1, "exec-001", "success", "", "", 0)
	})

	t.Run("任务不存在直接返回", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)
		svc.webhookSvc = &WebhookService{}
		// 不应 panic
		svc.SendWebhookNotification(ctx, 999, "exec-001", "success", "", "", 0)
	})
}

// ============ SetWebhookService ============

func TestSchedulerService_SetWebhookService(t *testing.T) {
	svc := &SchedulerService{}
	wh := &WebhookService{}
	svc.SetWebhookService(wh)
	if svc.webhookSvc == nil {
		t.Error("期望 webhookSvc 非 nil")
	}
}

// ============ executeQuery ============

func TestSchedulerService_executeQuery(t *testing.T) {
	t.Run("带参数查询", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1)},
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		result, err := svc.executeQuery("SELECT id FROM t WHERE id = ?", []interface{}{int64(1)})
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if result.Err != nil {
			t.Fatalf("期望 result.Err 为 nil，实际: %v", result.Err)
		}
		if len(db.QueryStmts) != 1 {
			t.Errorf("期望 1 次查询调用，实际=%d", len(db.QueryStmts))
		}
	})

	t.Run("无参数查询", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{int64(1)},
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		result, err := svc.executeQuery("SELECT id FROM t", nil)
		if err != nil {
			t.Fatalf("期望无错误，实际: %v", err)
		}
		if result.Err != nil {
			t.Fatalf("期望 result.Err 为 nil，实际: %v", result.Err)
		}
		if len(db.QueryStmts) != 1 {
			t.Errorf("期望 1 次查询调用，实际=%d", len(db.QueryStmts))
		}
	})
}

// ============ getLastExecutionStatus ============

func TestSchedulerService_getLastExecutionStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("查询成功", func(t *testing.T) {
		qr := database.NewQueryResultWithRows([][]interface{}{
			{"success"},
		})
		db := &MockDB{QueryResult: qr}
		svc := newSchedulerWithDB(db)

		status := svc.getLastExecutionStatus(ctx, 1)
		if status != "success" {
			t.Errorf("期望 status=success，实际=%s", status)
		}
	})

	t.Run("无执行记录返回空", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithRows(nil)}
		svc := newSchedulerWithDB(db)

		status := svc.getLastExecutionStatus(ctx, 1)
		if status != "" {
			t.Errorf("期望空字符串，实际=%s", status)
		}
	})

	t.Run("DB 错误返回空", func(t *testing.T) {
		db := &MockDB{QueryError: ErrMockDB}
		svc := newSchedulerWithDB(db)

		status := svc.getLastExecutionStatus(ctx, 1)
		if status != "" {
			t.Errorf("期望空字符串，实际=%s", status)
		}
	})

	t.Run("查询结果带错误返回空", func(t *testing.T) {
		db := &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result err"))}
		svc := newSchedulerWithDB(db)

		status := svc.getLastExecutionStatus(ctx, 1)
		if status != "" {
			t.Errorf("期望空字符串，实际=%s", status)
		}
	})
}
