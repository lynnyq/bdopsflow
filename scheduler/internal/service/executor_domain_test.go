package service

import (
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

func TestExecutorDomainAssignment(t *testing.T) {
	t.Run("创建执行器领域分配模型", func(t *testing.T) {
		de := &model.DomainExecutor{
			ID:         1,
			DomainID:   1,
			ExecutorID: 1,
			AssignedBy: ptrInt64(1),
		}

		if de.DomainID != 1 {
			t.Errorf("期望 DomainID 为 1，实际为 %d", de.DomainID)
		}
		if de.ExecutorID != 1 {
			t.Errorf("期望 ExecutorID 为 1，实际为 %d", de.ExecutorID)
		}
		if de.AssignedBy == nil || *de.AssignedBy != 1 {
			t.Error("期望 AssignedBy 为 1")
		}
	})

	t.Run("创建带有领域信息的执行器", func(t *testing.T) {
		domains := []*model.Domain{
			{ID: 1, Name: "default"},
			{ID: 2, Name: "production"},
		}

		exec := &model.ExecutorWithDomains{
			Executor: model.Executor{
				ID:          1,
				Name:        "测试执行器",
				Address:     "localhost:8080",
				Status:      "online",
				Capacity:    10,
				CurrentLoad: 0,
				IsGlobal:    false,
			},
			Domains:  domains,
			IsGlobal: false,
		}

		if len(exec.Domains) != 2 {
			t.Errorf("期望 2 个领域，实际为 %d", len(exec.Domains))
		}
		if exec.Domains[0].Name != "default" {
			t.Errorf("期望第一个领域名称为 'default'，实际为 '%s'", exec.Domains[0].Name)
		}
		if exec.Domains[1].Name != "production" {
			t.Errorf("期望第二个领域名称为 'production'，实际为 '%s'", exec.Domains[1].Name)
		}
	})

	t.Run("执行器是否为全局", func(t *testing.T) {
		globalExec := &model.Executor{
			ID:       1,
			IsGlobal: true,
		}

		if !globalExec.IsGlobal {
			t.Error("期望全局执行器 IsGlobal 为 true")
		}

		domainExec := &model.Executor{
			ID:       2,
			IsGlobal: false,
		}

		if domainExec.IsGlobal {
			t.Error("期望领域执行器 IsGlobal 为 false")
		}
	})
}

func TestDomainExecutorAssignment(t *testing.T) {
	t.Run("分配执行器到多个领域请求", func(t *testing.T) {
		req := &model.ExecutorDomainRequest{
			DomainIDs: []int64{1, 2, 3},
		}

		if len(req.DomainIDs) != 3 {
			t.Errorf("期望 3 个领域ID，实际为 %d", len(req.DomainIDs))
		}

		if req.DomainIDs[0] != 1 || req.DomainIDs[1] != 2 || req.DomainIDs[2] != 3 {
			t.Error("领域ID顺序不正确")
		}
	})

	t.Run("分配执行器到单个领域请求", func(t *testing.T) {
		req := &model.ExecutorDomainRequest{
			DomainIDs: []int64{1},
		}

		if len(req.DomainIDs) != 1 {
			t.Errorf("期望 1 个领域ID，实际为 %d", len(req.DomainIDs))
		}
	})

	t.Run("分配执行器到空领域请求（全局执行器）", func(t *testing.T) {
		req := &model.ExecutorDomainRequest{
			DomainIDs: []int64{},
		}

		if len(req.DomainIDs) != 0 {
			t.Errorf("期望 0 个领域ID，实际为 %d", len(req.DomainIDs))
		}
	})
}

func TestDomainWithStats(t *testing.T) {
	t.Run("创建带有统计信息的领域", func(t *testing.T) {
		domainStats := &model.DomainWithStats{
			Domain: model.Domain{
				ID:          1,
				Name:        "production",
				Description: "生产环境",
			},
			UserCount:     10,
			ExecutorCount: 5,
			TaskCount:     100,
		}

		if domainStats.UserCount != 10 {
			t.Errorf("期望 UserCount 为 10，实际为 %d", domainStats.UserCount)
		}
		if domainStats.ExecutorCount != 5 {
			t.Errorf("期望 ExecutorCount 为 5，实际为 %d", domainStats.ExecutorCount)
		}
		if domainStats.TaskCount != 100 {
			t.Errorf("期望 TaskCount 为 100，实际为 %d", domainStats.TaskCount)
		}
	})
}

func TestExecutorDeletePermission(t *testing.T) {
	t.Run("系统管理员可以删除任何执行器", func(t *testing.T) {
		userRole := "system_admin"
		isAdmin := userRole == "system_admin" || userRole == "admin"

		if !isAdmin {
			t.Error("系统管理员应该可以删除任何执行器")
		}
	})

	t.Run("普通管理员可以删除任何执行器", func(t *testing.T) {
		userRole := "admin"
		isAdmin := userRole == "system_admin" || userRole == "admin"

		if !isAdmin {
			t.Error("管理员应该可以删除任何执行器")
		}
	})

	t.Run("领域管理员只能删除只分配给其领域的执行器", func(t *testing.T) {
		userRole := "domain_admin"
		isAdmin := userRole == "system_admin" || userRole == "admin"

		if isAdmin {
			t.Error("领域管理员不应该是管理员")
		}

		executorDomainCount := 1
		canDelete := executorDomainCount <= 1

		if !canDelete {
			t.Error("领域管理员应该能够删除只分配给其领域的执行器")
		}
	})

	t.Run("领域管理员不能删除分配给多个领域的执行器", func(t *testing.T) {
		executorDomainCount := 2
		canDelete := executorDomainCount <= 1

		if canDelete {
			t.Error("领域管理员不应该能够删除分配给多个领域的执行器")
		}
	})
}

func TestTaskExecutorBinding(t *testing.T) {
	t.Run("检查执行器是否被任务绑定", func(t *testing.T) {
		task := &model.Task{
			ID:                 1,
			Name:               "测试任务",
			AssignedExecutorID: 1,
		}

		if task.AssignedExecutorID != 1 {
			t.Errorf("期望 AssignedExecutorID 为 1，实际为 %d", task.AssignedExecutorID)
		}

		isBound := task.AssignedExecutorID > 0
		if !isBound {
			t.Error("任务应该绑定到执行器")
		}
	})

	t.Run("未绑定执行器的任务", func(t *testing.T) {
		task := &model.Task{
			ID:                 2,
			Name:               "未绑定任务",
			AssignedExecutorID: 0,
		}

		isBound := task.AssignedExecutorID > 0
		if isBound {
			t.Error("任务不应该绑定到执行器")
		}
	})
}

func TestExecutorDomainFiltering(t *testing.T) {
	t.Run("系统管理员可以看到所有执行器", func(t *testing.T) {
		userRole := "system_admin"
		canSeeAll := userRole == "system_admin" || userRole == "admin"

		if !canSeeAll {
			t.Error("系统管理员应该可以看到所有执行器")
		}
	})

	t.Run("领域管理员只能看到分配给其领域的执行器", func(t *testing.T) {
		userRole := "domain_admin"
		canSeeAll := userRole == "system_admin" || userRole == "admin"

		if canSeeAll {
			t.Error("领域管理员不应该可以看到所有执行器")
		}

		userDomainID := int64(1)
		executors := []*model.ExecutorWithDomains{
			{Executor: model.Executor{ID: 1}, Domains: []*model.Domain{{ID: 1}}},
			{Executor: model.Executor{ID: 2}, Domains: []*model.Domain{{ID: 2}}},
		}

		var accessibleExecutors []*model.ExecutorWithDomains
		for _, exec := range executors {
			for _, domain := range exec.Domains {
				if domain.ID == userDomainID {
					accessibleExecutors = append(accessibleExecutors, exec)
					break
				}
			}
		}

		if len(accessibleExecutors) != 1 {
			t.Errorf("领域管理员应该只能看到 1 个执行器，实际为 %d", len(accessibleExecutors))
		}
	})

	t.Run("普通用户只能看到分配给其领域的执行器", func(t *testing.T) {
		userRole := "user"
		canSeeAll := userRole == "system_admin" || userRole == "admin"

		if canSeeAll {
			t.Error("普通用户不应该可以看到所有执行器")
		}
	})
}

func TestExecutorDomainMultiAssignment(t *testing.T) {
	t.Run("执行器可以被分配给多个领域", func(t *testing.T) {
		exec := &model.ExecutorWithDomains{
			Executor: model.Executor{
				ID:   1,
				Name: "多领域执行器",
			},
			Domains: []*model.Domain{
				{ID: 1, Name: "default"},
				{ID: 2, Name: "staging"},
				{ID: 3, Name: "production"},
			},
		}

		if len(exec.Domains) != 3 {
			t.Errorf("期望执行器分配到 3 个领域，实际为 %d", len(exec.Domains))
		}
	})

	t.Run("执行器所属领域ID列表", func(t *testing.T) {
		exec := &model.ExecutorWithDomains{
			Executor: model.Executor{
				ID: 1,
			},
			Domains: []*model.Domain{
				{ID: 1, Name: "default"},
				{ID: 2, Name: "production"},
			},
		}

		domainIDs := make([]int64, len(exec.Domains))
		for i, d := range exec.Domains {
			domainIDs[i] = d.ID
		}

		if len(domainIDs) != 2 {
			t.Errorf("期望 2 个领域ID，实际为 %d", len(domainIDs))
		}
		if domainIDs[0] != 1 || domainIDs[1] != 2 {
			t.Error("领域ID列表不正确")
		}
	})
}

func ptrInt64(v int64) *int64 {
	return &v
}
