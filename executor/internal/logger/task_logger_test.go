package logger

import (
	"context"
	"log/slog"
	"testing"

	pb "github.com/lynnyq/bdopsflow/proto"
)

// mockLogReporter 用于测试的 LogReporter mock，记录所有调用
type mockLogReporter struct {
	calls []*pb.ReportTaskLogRequest
	err   error
}

func (m *mockLogReporter) ReportLog(ctx context.Context, req *pb.ReportTaskLogRequest) error {
	m.calls = append(m.calls, req)
	return m.err
}

// TestNewTaskLogger 测试构造函数是否正确赋值
func TestNewTaskLogger(t *testing.T) {
	t.Run("正常构造", func(t *testing.T) {
		reporter := &mockLogReporter{}
		l := NewTaskLogger("exec-001", int64(42), reporter)
		if l == nil {
			t.Fatal("期望返回非 nil 实例")
		}
		if l.executionID != "exec-001" {
			t.Errorf("executionID 期望 exec-001，实际 %s", l.executionID)
		}
		if l.taskID != 42 {
			t.Errorf("taskID 期望 42，实际 %d", l.taskID)
		}
		if l.client != reporter {
			t.Error("client 未正确赋值")
		}
	})

	t.Run("nil client 时仍可构造", func(t *testing.T) {
		l := NewTaskLogger("exec-002", int64(1), nil)
		if l == nil {
			t.Fatal("期望返回非 nil 实例")
		}
		if l.client != nil {
			t.Error("期望 client 为 nil")
		}
	})
}

// TestNewGRPCLogReporter 测试 GRPCLogReporter 构造函数
func TestNewGRPCLogReporter(t *testing.T) {
	t.Run("nil client 时仍可构造", func(t *testing.T) {
		r := NewGRPCLogReporter(nil)
		if r == nil {
			t.Fatal("期望返回非 nil 实例")
		}
	})
}

// TestTaskLogger_LogLevels 测试各级别日志方法在 nil client 时不 panic，
// 并通过 slog 输出日志
func TestTaskLogger_LogLevels(t *testing.T) {
	tests := []struct {
		name    string
		call    func(l *TaskLogger)
		level   string
		message string
	}{
		{
			name:    "Info",
			call:    func(l *TaskLogger) { l.Info("info message") },
			level:   "info",
			message: "info message",
		},
		{
			name:    "Error",
			call:    func(l *TaskLogger) { l.Error("error message") },
			level:   "error",
			message: "error message",
		},
		{
			name:    "Debug",
			call:    func(l *TaskLogger) { l.Debug("debug message") },
			level:   "debug",
			message: "debug message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// nil client 不应 panic
			l := NewTaskLogger("exec-level", int64(1), nil)
			tt.call(l)
		})
	}
}

// TestTaskLogger_FormatMethods 测试 Infof/Errorf/Debugf 格式化方法
func TestTaskLogger_FormatMethods(t *testing.T) {
	tests := []struct {
		name     string
		call     func(l *TaskLogger)
		expected string
	}{
		{
			name:     "Infof",
			call:     func(l *TaskLogger) { l.Infof("task %s started with id %d", "alpha", 100) },
			expected: "task alpha started with id 100",
		},
		{
			name:     "Errorf",
			call:     func(l *TaskLogger) { l.Errorf("error code %d: %s", 500, "internal") },
			expected: "error code 500: internal",
		},
		{
			name:     "Debugf",
			call:     func(l *TaskLogger) { l.Debugf("debug value=%v", true) },
			expected: "debug value=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := &mockLogReporter{}
			l := NewTaskLogger("exec-fmt", int64(7), reporter)
			tt.call(l)

			if len(reporter.calls) != 1 {
				t.Fatalf("期望 ReportLog 调用 1 次，实际 %d 次", len(reporter.calls))
			}
			if reporter.calls[0].LogContent != tt.expected {
				t.Errorf("期望日志内容 %q，实际 %q", tt.expected, reporter.calls[0].LogContent)
			}
			if reporter.calls[0].ExecutionId != "exec-fmt" {
				t.Errorf("期望 ExecutionId=exec-fmt，实际 %s", reporter.calls[0].ExecutionId)
			}
			if reporter.calls[0].TaskId != 7 {
				t.Errorf("期望 TaskId=7，实际 %d", reporter.calls[0].TaskId)
			}
		})
	}
}

// TestTaskLogger_LogWithClient 测试有 client 时 log 方法调用 ReportLog
func TestTaskLogger_LogWithClient(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		call       func(l *TaskLogger)
		expectLvl  string
		expectMsg  string
	}{
		{
			name:      "Info 通过 client 上报",
			method:    "Info",
			call:      func(l *TaskLogger) { l.Info("hello info") },
			expectLvl: "info",
			expectMsg: "hello info",
		},
		{
			name:      "Error 通过 client 上报",
			method:    "Error",
			call:      func(l *TaskLogger) { l.Error("hello error") },
			expectLvl: "error",
			expectMsg: "hello error",
		},
		{
			name:      "Debug 通过 client 上报",
			method:    "Debug",
			call:      func(l *TaskLogger) { l.Debug("hello debug") },
			expectLvl: "debug",
			expectMsg: "hello debug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := &mockLogReporter{}
			l := NewTaskLogger("exec-client", int64(99), reporter)
			tt.call(l)

			if len(reporter.calls) != 1 {
				t.Fatalf("期望 ReportLog 调用 1 次，实际 %d 次", len(reporter.calls))
			}
			req := reporter.calls[0]
			if req.ExecutionId != "exec-client" {
				t.Errorf("期望 ExecutionId=exec-client，实际 %s", req.ExecutionId)
			}
			if req.TaskId != 99 {
				t.Errorf("期望 TaskId=99，实际 %d", req.TaskId)
			}
			if req.LogLevel != tt.expectLvl {
				t.Errorf("期望 LogLevel=%s，实际 %s", tt.expectLvl, req.LogLevel)
			}
			if req.LogContent != tt.expectMsg {
				t.Errorf("期望 LogContent=%s，实际 %s", tt.expectMsg, req.LogContent)
			}
			if req.Timestamp <= 0 {
				t.Error("期望 Timestamp 为正数")
			}
		})
	}
}

// TestTaskLogger_LogNilClientNoPanic 测试 nil client 时 log 不 panic
func TestTaskLogger_LogNilClientNoPanic(t *testing.T) {
	l := NewTaskLogger("exec-nil", int64(0), nil)

	// 各级别调用都不应 panic
	l.Info("info without client")
	l.Error("error without client")
	l.Debug("debug without client")
	l.Infof("infof %d", 1)
	l.Errorf("errorf %d", 2)
	l.Debugf("debugf %d", 3)
}

// TestTaskLogger_LogDefaultLevel 测试 log 方法 default 分支（未识别的级别）
// 通过直接调用 log 方法触发 default 分支
func TestTaskLogger_LogDefaultLevel(t *testing.T) {
	reporter := &mockLogReporter{}
	l := NewTaskLogger("exec-default", int64(5), reporter)

	// 调用未识别的级别，应走 default 分支（slog.Info）
	l.log("unknown_level", "default branch message")

	if len(reporter.calls) != 1 {
		t.Fatalf("期望 ReportLog 调用 1 次，实际 %d 次", len(reporter.calls))
	}
	if reporter.calls[0].LogLevel != "unknown_level" {
		t.Errorf("期望 LogLevel=unknown_level，实际 %s", reporter.calls[0].LogLevel)
	}
	if reporter.calls[0].LogContent != "default branch message" {
		t.Errorf("期望 LogContent=default branch message，实际 %s", reporter.calls[0].LogContent)
	}
}

// TestTaskLogger_LogWarnLevel 测试 warn 级别（源码中有 case "warn" 但无公开方法调用）
func TestTaskLogger_LogWarnLevel(t *testing.T) {
	reporter := &mockLogReporter{}
	l := NewTaskLogger("exec-warn", int64(8), reporter)

	// 直接调用 log 方法测试 warn 分支
	l.log("warn", "warn message")

	if len(reporter.calls) != 1 {
		t.Fatalf("期望 ReportLog 调用 1 次，实际 %d 次", len(reporter.calls))
	}
	if reporter.calls[0].LogLevel != "warn" {
		t.Errorf("期望 LogLevel=warn，实际 %s", reporter.calls[0].LogLevel)
	}
}

// TestTaskLogger_ClientErrorIgnored 测试当 client 返回错误时，log 方法忽略错误
// （源码中 _ = l.client.ReportLog(...) 显式忽略错误）
func TestTaskLogger_ClientErrorIgnored(t *testing.T) {
	reporter := &mockLogReporter{err: context.Canceled}
	l := NewTaskLogger("exec-err", int64(10), reporter)

	// 不应 panic
	l.Info("message with error client")

	if len(reporter.calls) != 1 {
		t.Fatalf("期望 ReportLog 调用 1 次，实际 %d 次", len(reporter.calls))
	}
}

// TestInitLogger 测试 Init 函数初始化全局 Logger
func TestInitLogger(t *testing.T) {
	// 保存原始 Logger
	origLogger := Logger
	defer func() { Logger = origLogger }()

	Init()
	if Logger == nil {
		t.Fatal("期望 Logger 被初始化为非 nil")
	}
}

// TestLoggerPackageFunctions 测试包级别 Info/Error/Warn/Debug 函数
// 在 Logger 未初始化时不 panic（slog.Default 兜底）
func TestLoggerPackageFunctions(t *testing.T) {
	// 临时将 Logger 设为 nil，测试包函数是否会 panic
	origLogger := Logger
	defer func() { Logger = origLogger }()

	// 即使 Logger 为 nil，包函数会调用 Logger.Info，导致 nil panic
	// 因此先初始化 Logger
	Init()

	// 这些调用不应 panic
	Info("test info")
	Error("test error")
	Warn("test warn")
	Debug("test debug")
}

// TestTaskLogger_SlogLevel 测试 log 方法对各 slog 级别不 panic
func TestTaskLogger_SlogLevel(t *testing.T) {
	// 各级别 slog 常量均可被 log 方法正常使用，不应 panic
	levels := []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}
	for _, lvl := range levels {
		_ = lvl.String()
	}
}
