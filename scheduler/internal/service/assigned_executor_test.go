package service

import (
	"testing"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

// 测试 Task 结构体是否包含 AssignedExecutorID 字段
func TestTaskStructHasAssignedExecutorID(t *testing.T) {
	var task model.Task
	// 只要这个编译通过，就说明字段存在
	_ = task.AssignedExecutorID
}

// 测试当我们创建 Task 结构体时，AssignedExecutorID 字段是否可以正常设置和获取
func TestTaskAssignedExecutorIDField(t *testing.T) {
	task := model.Task{
		AssignedExecutorID: 123,
	}

	if task.AssignedExecutorID != 123 {
		t.Errorf("期望 AssignedExecutorID 为 123，实际为 %d", task.AssignedExecutorID)
	}

	task.AssignedExecutorID = 456
	if task.AssignedExecutorID != 456 {
		t.Errorf("期望 AssignedExecutorID 为 456，实际为 %d", task.AssignedExecutorID)
	}
}
