package logger

import (
	"io"
	"log"
	"log/slog"
	"os"
)

var Logger *slog.Logger

func Init(level string, format string) {
	slogLevel := parseLevel(level)
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: slogLevel}
	switch format {
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	Logger = slog.New(handler)
	slog.SetDefault(Logger)
	log.SetOutput(io.Discard)
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
	Logger.Info(msg, args...)
}

func Error(msg string, args ...any) {
	Logger.Error(msg, args...)
}

func Warn(msg string, args ...any) {
	Logger.Warn(msg, args...)
}

func Debug(msg string, args ...any) {
	Logger.Debug(msg, args...)
}