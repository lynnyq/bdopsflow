package executor

import (
	"testing"

	pb "github.com/lynnyq/bdopsflow/proto"
)

func TestTaskExecutor_GetRunningExecutionIds(t *testing.T) {
	executor := NewTaskExecutor(nil)

	ids := executor.GetRunningExecutionIds()
	if len(ids) != 0 {
		t.Errorf("expected 0 running tasks initially, got %d", len(ids))
	}
}

func TestTaskExecutor_RunningTaskTracking(t *testing.T) {
	executor := NewTaskExecutor(nil)

	executionId1 := "exec-001"
	executionId2 := "exec-002"
	executionId3 := "exec-003"

	task1 := &pb.Task{
		TaskId:      1,
		ExecutionId: executionId1,
	}
	task2 := &pb.Task{
		TaskId:      2,
		ExecutionId: executionId2,
	}
	task3 := &pb.Task{
		TaskId:      3,
		ExecutionId: executionId3,
	}

	executor.addRunningTask(executionId1, task1)
	if executor.getRunningCount() != 1 {
		t.Errorf("expected 1 running task, got %d", executor.getRunningCount())
	}

	executor.addRunningTask(executionId2, task2)
	if executor.getRunningCount() != 2 {
		t.Errorf("expected 2 running tasks, got %d", executor.getRunningCount())
	}

	executor.addRunningTask(executionId3, task3)
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
	executor := NewTaskExecutor(nil)

	count := executor.GetRunningTasks()
	if count != 0 {
		t.Errorf("expected 0 running tasks initially, got %d", count)
	}
}

func TestTaskExecutor_GetRunningTaskStates(t *testing.T) {
	executor := NewTaskExecutor(nil)

	states := executor.GetRunningTaskStates()
	if len(states) != 0 {
		t.Errorf("expected 0 running task states initially, got %d", len(states))
	}

	executionId := "exec-001"
	task := &pb.Task{
		TaskId:      1,
		ExecutionId: executionId,
	}

	executor.addRunningTask(executionId, task)
	states = executor.GetRunningTaskStates()
	if len(states) != 1 {
		t.Errorf("expected 1 running task state, got %d", len(states))
	}
	if states[0].ExecutionId != executionId {
		t.Errorf("expected execution ID %s, got %s", executionId, states[0].ExecutionId)
	}
	if states[0].TaskId != 1 {
		t.Errorf("expected task ID 1, got %d", states[0].TaskId)
	}
	if states[0].Status != "running" {
		t.Errorf("expected status 'running', got %s", states[0].Status)
	}
}
