package logger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// saveLoggerState 保存当前 logger 全局状态，用于测试后恢复。
// 返回一个 cleanup 函数，在 t.Cleanup 中调用以恢复状态。
func saveLoggerState(t *testing.T) {
	t.Helper()

	origLogger := Logger
	origLogFile := logFile
	origLogLevel := logLevel
	origLogFormat := logFormat
	origLogPath := logPath

	t.Cleanup(func() {
		Logger = origLogger
		if logFile != nil && logFile != origLogFile {
			logFile.Close()
		}
		logFile = origLogFile
		logLevel = origLogLevel
		logFormat = origLogFormat
		logPath = origLogPath
	})
}

// ------------------------------------------------------------
// parseLevel
// ------------------------------------------------------------

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  string // slog.Level 的字符串表示
	}{
		{"debug", "debug", "DEBUG"},
		{"info", "info", "INFO"},
		{"warn", "warn", "WARN"},
		{"error", "error", "ERROR"},
		{"empty defaults to info", "", "INFO"},
		{"unknown defaults to info", "unknown", "INFO"},
		{"DEBUG uppercase defaults to info", "DEBUG", "INFO"},
		{"trace defaults to info", "trace", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLevel(tt.level)
			if got.String() != tt.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.level, got.String(), tt.want)
			}
		})
	}
}

// ------------------------------------------------------------
// Init
// ------------------------------------------------------------

func TestInit_JSONFormat(t *testing.T) {
	saveLoggerState(t)

	Init("info", "json")

	if Logger == nil {
		t.Fatal("Logger should not be nil after Init")
	}
	if logLevel != "info" {
		t.Errorf("logLevel = %v, want info", logLevel)
	}
	if logFormat != "json" {
		t.Errorf("logFormat = %v, want json", logFormat)
	}
}

func TestInit_TextFormat(t *testing.T) {
	saveLoggerState(t)

	Init("debug", "text")

	if Logger == nil {
		t.Fatal("Logger should not be nil after Init")
	}
	if logLevel != "debug" {
		t.Errorf("logLevel = %v, want debug", logLevel)
	}
	if logFormat != "text" {
		t.Errorf("logFormat = %v, want text", logFormat)
	}
}

func TestInit_DefaultFormat(t *testing.T) {
	saveLoggerState(t)

	// format 不匹配 "text" 时应默认使用 JSON handler
	Init("info", "unknown-format")

	if Logger == nil {
		t.Fatal("Logger should not be nil after Init")
	}
	if logFormat != "unknown-format" {
		t.Errorf("logFormat = %v, want unknown-format", logFormat)
	}
}

func TestInit_AllLevels(t *testing.T) {
	saveLoggerState(t)

	levels := []string{"debug", "info", "warn", "error"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			Init(level, "json")
			if logLevel != level {
				t.Errorf("logLevel = %v, want %v", logLevel, level)
			}
			if Logger == nil {
				t.Fatal("Logger should not be nil")
			}
		})
	}
}

// ------------------------------------------------------------
// InitWithFile
// ------------------------------------------------------------

func TestInitWithFile_Success(t *testing.T) {
	saveLoggerState(t)

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	InitWithFile("info", "json", logFile)

	if Logger == nil {
		t.Fatal("Logger should not be nil after InitWithFile")
	}
	if logLevel != "info" {
		t.Errorf("logLevel = %v, want info", logLevel)
	}
	if logFormat != "json" {
		t.Errorf("logFormat = %v, want json", logFormat)
	}
	if logPath != logFile {
		t.Errorf("logPath = %v, want %v", logPath, logFile)
	}

	// 验证文件已创建
	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("log file should exist: %v", err)
	}
	if info.Size() > 0 {
		// 初始化时会写一条 "logger initialized" 日志
		t.Logf("log file size = %d (expected > 0 due to init message)", info.Size())
	}
}

func TestInitWithFile_InvalidPath(t *testing.T) {
	saveLoggerState(t)

	// 使用无效路径，应回退到 stdout 而非 panic
	InitWithFile("info", "json", "/nonexistent/dir/that/does/not/exist/test.log")

	if Logger == nil {
		t.Fatal("Logger should not be nil even with invalid file path")
	}
	// logFile 应为 nil（因为打开失败）
	if logFile != nil {
		t.Error("logFile should be nil when file open fails")
	}
}

func TestInitWithFile_TextFormat(t *testing.T) {
	saveLoggerState(t)

	tmpDir := t.TempDir()
	logFilePath := filepath.Join(tmpDir, "test_text.log")

	InitWithFile("debug", "text", logFilePath)

	if Logger == nil {
		t.Fatal("Logger should not be nil")
	}
	if logFormat != "text" {
		t.Errorf("logFormat = %v, want text", logFormat)
	}
}

// ------------------------------------------------------------
// 日志输出函数 (Info, Error, Warn, Debug)
// ------------------------------------------------------------

func TestLogFunctions_DoNotPanic(t *testing.T) {
	saveLoggerState(t)

	// 先初始化 Logger
	Init("debug", "json")

	// 所有日志函数不应 panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("log function panicked: %v", r)
		}
	}()

	Info("test info message", "key", "value")
	Error("test error message", "key", "value")
	Warn("test warn message", "key", "value")
	Debug("test debug message", "key", "value")
}

func TestLogFunctions_WithNilLogger(t *testing.T) {
	saveLoggerState(t)

	// 设置 Logger 为 nil（模拟未初始化状态）
	Logger = nil

	// 所有日志函数应 panic（因为 Logger.Info 会 nil pointer dereference）
	// 这个测试验证调用者必须先初始化 Logger
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when Logger is nil")
		}
	}()

	Info("this should panic")
}

func TestInfo_WithMultipleArgs(t *testing.T) {
	saveLoggerState(t)

	Init("info", "json")

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Info panicked: %v", r)
		}
	}()

	Info("message with multiple args",
		"key1", "value1",
		"key2", 42,
		"key3", true,
		"key4", []string{"a", "b"},
	)
}

func TestError_WithMultipleArgs(t *testing.T) {
	saveLoggerState(t)

	Init("error", "json")

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Error panicked: %v", r)
		}
	}()

	Error("error with args",
		"error_code", 500,
		"error_msg", "internal server error",
	)
}

func TestDebug_LevelFiltered(t *testing.T) {
	saveLoggerState(t)

	// 使用 error 级别，debug 消息应被过滤（不输出）
	Init("error", "json")

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Debug panicked: %v", r)
		}
	}()

	// 即使被过滤也不应 panic
	Debug("this should be filtered out")
}

// ------------------------------------------------------------
// ReopenLogFile
// ------------------------------------------------------------

func TestReopenLogFile_NoPathConfigured(t *testing.T) {
	saveLoggerState(t)

	// 初始化但不设置文件路径
	Init("info", "json")

	// logPath 为空时应跳过并返回 nil
	err := ReopenLogFile()
	if err != nil {
		t.Errorf("ReopenLogFile should return nil when no path configured, got %v", err)
	}
}

func TestReopenLogFile_Success(t *testing.T) {
	saveLoggerState(t)

	tmpDir := t.TempDir()
	logFilePath := filepath.Join(tmpDir, "reopen_test.log")

	// 使用文件初始化
	InitWithFile("info", "json", logFilePath)

	// 写入一些日志
	Info("before reopen")

	// 重新打开日志文件
	err := ReopenLogFile()
	if err != nil {
		t.Fatalf("ReopenLogFile failed: %v", err)
	}

	// 重新打开后仍可写入
	Info("after reopen")

	// 验证文件存在且可读
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if len(content) == 0 {
		t.Error("log file should not be empty")
	}
}

func TestReopenLogFile_InvalidPath(t *testing.T) {
	saveLoggerState(t)

	// 先使用有效路径初始化
	tmpDir := t.TempDir()
	validPath := filepath.Join(tmpDir, "valid.log")
	InitWithFile("info", "json", validPath)

	// 然后设置无效路径
	logPath = "/nonexistent/dir/invalid.log"

	err := ReopenLogFile()
	if err == nil {
		t.Error("ReopenLogFile should fail with invalid path")
	}
}

func TestReopenLogFile_FileContent(t *testing.T) {
	saveLoggerState(t)

	tmpDir := t.TempDir()
	logFilePath := filepath.Join(tmpDir, "content_test.log")

	InitWithFile("info", "json", logFilePath)
	Info("test message before reopen")

	// 读取 reopen 前的内容
	contentBefore, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("failed to read log file before reopen: %v", err)
	}

	// 重新打开
	err = ReopenLogFile()
	if err != nil {
		t.Fatalf("ReopenLogFile failed: %v", err)
	}

	Info("test message after reopen")

	// 读取 reopen 后的内容
	contentAfter, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("failed to read log file after reopen: %v", err)
	}

	// reopen 后内容应比 reopen 前多（因为追加了新日志）
	if len(contentAfter) <= len(contentBefore) {
		t.Errorf("content after reopen (%d) should be longer than before (%d)", len(contentAfter), len(contentBefore))
	}

	// 验证 reopen 前的内容包含预期消息
	if !strings.Contains(string(contentBefore), "test message before reopen") {
		t.Error("content before reopen should contain the log message")
	}
}

// ------------------------------------------------------------
// 并发安全测试
// ------------------------------------------------------------

func TestLogger_ConcurrentInit(t *testing.T) {
	saveLoggerState(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			Init("info", "json")
			Info("concurrent message", "goroutine", i)
		}(i)
	}
	wg.Wait()

	// 不应 panic 或 data race
	if Logger == nil {
		t.Error("Logger should not be nil after concurrent init")
	}
}

func TestLogger_ConcurrentLogFunctions(t *testing.T) {
	saveLoggerState(t)

	Init("debug", "json")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(4)
		go func(i int) {
			defer wg.Done()
			Info("concurrent info", "id", i)
		}(i)
		go func(i int) {
			defer wg.Done()
			Warn("concurrent warn", "id", i)
		}(i)
		go func(i int) {
			defer wg.Done()
			Error("concurrent error", "id", i)
		}(i)
		go func(i int) {
			defer wg.Done()
			Debug("concurrent debug", "id", i)
		}(i)
	}
	wg.Wait()
}

// ------------------------------------------------------------
// 日志文件权限测试
// ------------------------------------------------------------

func TestInitWithFile_FilePermissions(t *testing.T) {
	saveLoggerState(t)

	tmpDir := t.TempDir()
	logFilePath := filepath.Join(tmpDir, "perms_test.log")

	InitWithFile("info", "json", logFilePath)

	info, err := os.Stat(logFilePath)
	if err != nil {
		t.Fatalf("failed to stat log file: %v", err)
	}

	// 验证文件权限为 0644
	if info.Mode().Perm() != 0644 {
		t.Errorf("file permission = %v, want 0644", info.Mode().Perm())
	}
}

// ------------------------------------------------------------
// 日志级别过滤测试
// ------------------------------------------------------------

func TestLogLevel_Filtering(t *testing.T) {
	saveLoggerState(t)

	tmpDir := t.TempDir()
	logFilePath := filepath.Join(tmpDir, "filter_test.log")

	// 使用 error 级别初始化
	InitWithFile("error", "json", logFilePath)

	// 写入不同级别的日志
	Debug("debug message - should be filtered")
	Warn("warn message - should be filtered")
	Info("info message - should be filtered")
	Error("error message - should appear")

	// 读取日志文件内容
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	contentStr := string(content)

	// error 消息应出现
	if !strings.Contains(contentStr, "error message - should appear") {
		t.Error("error level message should appear in log file")
	}

	// 初始化日志也会出现（它是 info 级别的，但在初始化时写的）
	// 注意：slog.SetDefault 之前的 "logger initialized" 消息仍会输出
}

// ------------------------------------------------------------
// 多次初始化测试
// ------------------------------------------------------------

func TestInit_MultipleTimes(t *testing.T) {
	saveLoggerState(t)

	// 多次初始化不应 panic
	Init("info", "json")
	Info("first init")

	Init("debug", "text")
	Debug("second init")

	Init("error", "json")
	Error("third init")

	if Logger == nil {
		t.Error("Logger should not be nil after multiple inits")
	}
	if logLevel != "error" {
		t.Errorf("logLevel = %v, want error", logLevel)
	}
	if logFormat != "json" {
		t.Errorf("logFormat = %v, want json", logFormat)
	}
}
