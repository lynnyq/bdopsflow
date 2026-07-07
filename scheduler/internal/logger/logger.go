package logger

import (
	"io"
	"log"
	"log/slog"
	"os"
	"sync"
)

var (
	// Logger 全局 logger 指针，并发访问由 loggerMu 保护。
	// 外部代码可读取此字段用于诊断，调用日志函数请使用 Info/Error/Warn/Debug。
	Logger     *slog.Logger
	loggerMu   sync.RWMutex
	logFile    *os.File
	logFileMu  sync.Mutex
	logLevel   string
	logFormat  string
	logPath    string
	initMu     sync.Mutex
)

// getLogger 安全地获取当前 logger。
func getLogger() *slog.Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return Logger
}

// setLogger 安全地设置当前 logger。
func setLogger(l *slog.Logger) {
	loggerMu.Lock()
	Logger = l
	loggerMu.Unlock()
}

func Init(level string, format string) {
	initMu.Lock()
	defer initMu.Unlock()
	logLevel = level
	logFormat = format
	initializeLogger(level, format, "")
}

func InitWithFile(level string, format string, filePath string) {
	initMu.Lock()
	defer initMu.Unlock()
	logLevel = level
	logFormat = format
	logPath = filePath
	initializeLogger(level, format, filePath)
}

func initializeLogger(level string, format string, filePath string) {
	slogLevel := parseLevel(level)
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: slogLevel}

	var writer io.Writer
	if filePath != "" {
		var err error
		logFileMu.Lock()
		logFile, err = os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		logFileMu.Unlock()
		if err != nil {
			slog.Warn("failed to open log file, falling back to stdout", "error", err, "file", filePath)
			writer = os.Stdout
		} else {
			writer = logFile
		}
	} else {
		writer = os.Stdout
	}

	switch format {
	case "text":
		handler = slog.NewTextHandler(writer, opts)
	default:
		handler = slog.NewJSONHandler(writer, opts)
	}

	newLogger := slog.New(handler)
	setLogger(newLogger)
	slog.SetDefault(newLogger)
	log.SetOutput(io.Discard)

	slog.Info("logger initialized", "level", level, "format", format, "log_file", filePath)
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Info(msg string, args ...any) {
	getLogger().Info(msg, args...)
}

func Error(msg string, args ...any) {
	getLogger().Error(msg, args...)
}

func Warn(msg string, args ...any) {
	getLogger().Warn(msg, args...)
}

func Debug(msg string, args ...any) {
	getLogger().Debug(msg, args...)
}

func ReopenLogFile() error {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	if logPath == "" {
		slog.Info("log file path not configured, skipping reopen")
		return nil
	}

	if logFile != nil {
		slog.Info("closing existing log file")
		if err := logFile.Close(); err != nil {
			return err
		}
		logFile = nil
	}

	var err error
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	slogLevel := parseLevel(logLevel)
	opts := &slog.HandlerOptions{Level: slogLevel}

	var handler slog.Handler
	switch logFormat {
	case "text":
		handler = slog.NewTextHandler(logFile, opts)
	default:
		handler = slog.NewJSONHandler(logFile, opts)
	}

	newLogger := slog.New(handler)
	setLogger(newLogger)
	slog.SetDefault(newLogger)
	log.SetOutput(io.Discard)

	slog.Info("log file reopened successfully", "log_file", logPath)
	return nil
}
