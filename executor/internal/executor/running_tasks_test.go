package executor

import (
	"testing"
)

func TestTaskExecutor_GetRunningExecutionIds(t *testing.T) {
	executor := NewTaskExecutor("test-executor", nil)

	ids := executor.GetRunningExecutionIds()
	if len(ids) != 0 {
		t.Errorf("expected 0 running tasks initially, got %d", len(ids))
	}
}

func TestTaskExecutor_RunningTaskTracking(t *testing.T) {
	executor := NewTaskExecutor("test-executor", nil)

	executionId1 := "exec-001"
	executionId2 := "exec-002"
	executionId3 := "exec-003"

	executor.addRunningTask(executionId1)
	if executor.getRunningCount() != 1 {
		t.Errorf("expected 1 running task, got %d", executor.getRunningCount())
	}

	executor.addRunningTask(executionId2)
	if executor.getRunningCount() != 2 {
		t.Errorf("expected 2 running tasks, got %d", executor.getRunningCount())
	}

	executor.addRunningTask(executionId3)
	if executor.getRunningCount() != 3 {
		t.Errorf("expected 3 running tasks, got %d", executor.getRunningCount())
	}

	ids := executor.GetRunningExecutionIds()
	if len(ids) != 3 {
		t.Errorf("expected 3 execution IDs, got %d", len(ids))
	}

	found := make(map[string]bool)
	for _, id := range ids {
		found[id] = true
	}

	if !found[executionId1] || !found[executionId2] || !found[executionId3] {
		t.Errorf("missing execution IDs, found: %v", found)
	}

	executor.removeRunningTask(executionId2)
	if executor.getRunningCount() != 2 {
		t.Errorf("expected 2 running tasks after removal, got %d", executor.getRunningCount())
	}

	ids = executor.GetRunningExecutionIds()
	if len(ids) != 2 {
		t.Errorf("expected 2 execution IDs after removal, got %d", len(ids))
	}

	found = make(map[string]bool)
	for _, id := range ids {
		found[id] = true
	}

	if found[executionId2] {
		t.Errorf("execution ID %s should have been removed", executionId2)
	}

	if !found[executionId1] || !found[executionId3] {
		t.Errorf("execution IDs should contain %s and %s", executionId1, executionId3)
	}
}

func TestTaskExecutor_GetRunningTasks(t *testing.T) {
	executor := NewTaskExecutor("test-executor", nil)

	count := executor.GetRunningTasks()
	if count != 0 {
		t.Errorf("expected 0 running tasks initially, got %d", count)
	}
}
