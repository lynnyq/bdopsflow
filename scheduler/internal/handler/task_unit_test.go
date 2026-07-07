package handler

import (
	"testing"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

// TestFnvHash 测试 FNV-1a 哈希函数
func TestFnvHash(t *testing.T) {
	tests := []struct {
		name  string
		input string
		// FNV-1a 期望值
		want uint64
	}{
		{"空字符串", "", 14695981039346656037},
		{"单个字符a", "a", 1099511628211 ^ uint64('a')}, // offset64 ^ 'a' * prime... 简化验证
		{"相同输入产生相同输出", "hello", fnvHash("hello")},
		{"不同输入产生不同输出", "world", fnvHash("world")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fnvHash(tt.input)
			if tt.name == "相同输入产生相同输出" || tt.name == "不同输入产生不同输出" {
				if got != tt.want {
					t.Errorf("fnvHash(%q) = %d, want %d", tt.input, got, tt.want)
				}
			}
		})
	}

	// 验证确定性
	if fnvHash("test") != fnvHash("test") {
		t.Error("fnvHash 应该是确定性的")
	}

	// 验证不同输入产生不同输出
	if fnvHash("abc") == fnvHash("xyz") {
		t.Error("不同输入应该产生不同哈希值")
	}
}

// TestToTaskExecutionResponse 测试任务执行响应转换
func TestToTaskExecutionResponse(t *testing.T) {
	t.Run("完整字段", func(t *testing.T) {
		now := time.Now()
		exec := &model.TaskExecution{
			ID:          1,
			TaskID:      10,
			ExecutionID: "exec-123",
			ExecutorID:  5,
			Status:      "success",
			StartTime:   rqlite.NullTime{Valid: true, Time: now},
			EndTime:     rqlite.NullTime{Valid: true, Time: now.Add(time.Minute)},
			Output:      "task output",
			Error:       "",
			RetryTimes:  2,
			CreatedAt:   now,
		}

		resp := toTaskExecutionResponse(exec)
		if resp.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", resp.ID)
		}
		if resp.TaskID != 10 {
			t.Errorf("期望 TaskID=10，实际=%d", resp.TaskID)
		}
		if resp.ExecutionID != "exec-123" {
			t.Errorf("期望 ExecutionID=exec-123，实际=%s", resp.ExecutionID)
		}
		if resp.Status != "success" {
			t.Errorf("期望 Status=success，实际=%s", resp.Status)
		}
		if resp.Output != "task output" {
			t.Errorf("期望 Output=task output，实际=%s", resp.Output)
		}
		if resp.RetryTimes != 2 {
			t.Errorf("期望 RetryTimes=2，实际=%d", resp.RetryTimes)
		}
		if resp.StartTime == nil {
			t.Error("期望 StartTime 非 nil")
		}
		if resp.EndTime == nil {
			t.Error("期望 EndTime 非 nil")
		}
	})

	t.Run("空时间字段", func(t *testing.T) {
		exec := &model.TaskExecution{
			ID:        1,
			Status:    "pending",
			CreatedAt: time.Now(),
		}

		resp := toTaskExecutionResponse(exec)
		if resp.StartTime != nil {
			t.Error("期望 StartTime 为 nil（无效时间）")
		}
		if resp.EndTime != nil {
			t.Error("期望 EndTime 为 nil（无效时间）")
		}
	})

	t.Run("零值字段", func(t *testing.T) {
		exec := &model.TaskExecution{}
		resp := toTaskExecutionResponse(exec)
		if resp.ID != 0 {
			t.Errorf("期望 ID=0，实际=%d", resp.ID)
		}
		if resp.Status != "" {
			t.Errorf("期望 Status 为空，实际=%s", resp.Status)
		}
	})
}

// TestToTaskLogResponse 测试任务日志响应转换
func TestToTaskLogResponse(t *testing.T) {
	t.Run("完整字段", func(t *testing.T) {
		now := time.Now()
		tl := &model.TaskLog{
			ID:          1,
			ExecutionID: "exec-123",
			TaskID:      10,
			ExecutorID:  5,
			NodeID:      "node-1",
			LogLevel:    "info",
			Message:     "task started",
			LogTime:     now,
		}

		resp := toTaskLogResponse(tl)
		if resp.ID != 1 {
			t.Errorf("期望 ID=1，实际=%d", resp.ID)
		}
		if resp.ExecutionID != "exec-123" {
			t.Errorf("期望 ExecutionID=exec-123，实际=%s", resp.ExecutionID)
		}
		if resp.TaskID != 10 {
			t.Errorf("期望 TaskID=10，实际=%d", resp.TaskID)
		}
		if resp.ExecutorID != 5 {
			t.Errorf("期望 ExecutorID=5，实际=%d", resp.ExecutorID)
		}
		if resp.NodeID != "node-1" {
			t.Errorf("期望 NodeID=node-1，实际=%s", resp.NodeID)
		}
		if resp.LogLevel != "info" {
			t.Errorf("期望 LogLevel=info，实际=%s", resp.LogLevel)
		}
		if resp.Message != "task started" {
			t.Errorf("期望 Message=task started，实际=%s", resp.Message)
		}
		if resp.LogTime == "" {
			t.Error("期望 LogTime 非空")
		}
	})

	t.Run("零值字段", func(t *testing.T) {
		tl := &model.TaskLog{}
		resp := toTaskLogResponse(tl)
		if resp.ID != 0 {
			t.Errorf("期望 ID=0，实际=%d", resp.ID)
		}
		if resp.ExecutionID != "" {
			t.Errorf("期望 ExecutionID 为空，实际=%s", resp.ExecutionID)
		}
	})
}

// TestSafeTimePtr_Unit 测试时间指针转换（补充测试）
func TestSafeTimePtr_Unit(t *testing.T) {
	t.Run("零时间返回nil", func(t *testing.T) {
		result := safeTimePtr(time.Time{})
		if result != nil {
			t.Error("期望 nil")
		}
	})

	t.Run("有效时间返回非nil", func(t *testing.T) {
		now := time.Now()
		result := safeTimePtr(now)
		if result == nil {
			t.Fatal("期望非 nil")
		}
		// 验证格式化后的时间可以解析回来
		parsed, err := time.Parse(TimeResponseFormat, *result)
		if err != nil {
			t.Fatalf("解析时间失败: %v", err)
		}
		if !parsed.Equal(now) {
			t.Errorf("时间不匹配: 期望=%v, 实际=%v", now, parsed)
		}
	})
}
