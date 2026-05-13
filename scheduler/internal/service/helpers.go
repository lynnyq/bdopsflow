package service

import (
	"fmt"
	"log/slog"
	"time"

	rqlite "github.com/rqlite/gorqlite"
)

const (
	DateTimeFormat = "2006-01-02 15:04:05"
)

func nowUTC() string {
	return time.Now().UTC().Format(DateTimeFormat)
}

func parseDateTime(v interface{}) time.Time {
	if t, ok := v.(time.Time); ok {
		return t.UTC()
	}
	if s, ok := v.(string); ok && s != "" {
		parsed, err := time.Parse(DateTimeFormat, s)
		if err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
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
