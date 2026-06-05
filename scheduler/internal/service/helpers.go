package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	rqlite "github.com/rqlite/gorqlite"
)

const (
	DateTimeFormat       = time.RFC3339Nano
	LegacyDateTimeFormat = "2006-01-02 15:04:05"
)

func nowUTC() string {
	return time.Now().Format(DateTimeFormat)
}

func parseDateTime(v interface{}) time.Time {
	if t, ok := v.(time.Time); ok {
		return t
	}
	if s, ok := v.(string); ok && s != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return parsed
		}
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			return parsed
		}
		if parsed, err := time.Parse(LegacyDateTimeFormat, s); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func parseTimeInLocalTimezone(timeStr string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, timeStr); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return parsed, nil
	}
	return time.Parse(LegacyDateTimeFormat, timeStr)
}

func scanNullTime(row []interface{}, idx int) rqlite.NullTime {
	v := row[idx]
	if !isEmpty(v) {
		t := parseDateTime(v)
		if !t.IsZero() {
			return rqlite.NullTime{Time: t, Valid: true}
		}
	}
	return rqlite.NullTime{}
}

func ScanNullTime(row []interface{}, idx int) rqlite.NullTime {
	return scanNullTime(row, idx)
}

func scanTime(row []interface{}, idx int) time.Time {
	v := row[idx]
	if !isEmpty(v) {
		return parseDateTime(v)
	}
	return time.Time{}
}

func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case time.Time:
		return val.IsZero()
	}
	return false
}

func handleDBError(err error, operation string) error {
	if err != nil {
		slog.Error(operation, "error", err)
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

func handleWriteError(result rqlite.WriteResult, operation string) error {
	if result.Err != nil {
		slog.Error(operation, "error", result.Err)
		return fmt.Errorf("%s: %w", operation, result.Err)
	}
	return nil
}

func handleQueryError(qr rqlite.QueryResult, operation string) error {
	if qr.Err != nil {
		slog.Error(operation, "error", qr.Err)
		return fmt.Errorf("%s: %w", operation, qr.Err)
	}
	return nil
}

func ConvertToLocalTime(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.Local)
}

func FormatTimeInLocal(t time.Time) string {
	return t.In(time.Local).Format(DateTimeFormat)
}

const (
	DBQueryTimeout   = 10 * time.Second
	DBWriteTimeout   = 15 * time.Second
	DBCleanupTimeout = 30 * time.Second
)

func queryCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), DBQueryTimeout)
}

func cleanupCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), DBCleanupTimeout)
}
