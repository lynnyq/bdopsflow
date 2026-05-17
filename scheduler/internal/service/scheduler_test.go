package service

import (
	"testing"
	"time"

	rqlite "github.com/rqlite/gorqlite"
)

func TestRowInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int64
	}{
		{"int64", int64(42), 42},
		{"float64", float64(42.5), 42},
		{"string", "42", 42},
		{"nil", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rowInt64(tt.input)
			if result != tt.expected {
				t.Errorf("rowInt64(%v) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRowString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"string", "hello", "hello"},
		{"int64", int64(42), "42"},
		{"float64", float64(3.14), "3.14"},
		{"nil", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rowString(tt.input)
			if result != tt.expected {
				t.Errorf("rowString(%v) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRowBool(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"int 1", int64(1), true},
		{"int 0", int64(0), false},
		{"float 1", float64(1.0), true},
		{"float 0", float64(0.0), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rowBool(tt.input)
			if result != tt.expected {
				t.Errorf("rowBool(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"whitespace", "   ", false},
		{"normal string", "hello", false},
		{"zero time", time.Time{}, true},
		{"non-zero time", time.Now(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isEmpty(tt.input)
			if result != tt.expected {
				t.Errorf("isEmpty(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseDateTime(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected time.Time
		isZero   bool
	}{
		{"valid datetime string", "2024-01-01 12:00:00", time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), false},
		{"time.Time", time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), false},
		{"invalid string", "invalid", time.Time{}, true},
		{"nil", nil, time.Time{}, true},
		{"empty string", "", time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDateTime(tt.input)
			if tt.isZero && !result.IsZero() {
				t.Errorf("expected zero time, got %v", result)
			}
			if !tt.isZero && !result.Equal(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNowUTC(t *testing.T) {
	result := nowUTC()
	if len(result) != len("2024-01-01 12:00:00") {
		t.Errorf("expected datetime string format, got %q", result)
	}
}

func TestScanTime(t *testing.T) {
	row := []interface{}{"2024-01-01 12:00:00"}
	result := scanTime(row, 0)
	if result.IsZero() {
		t.Error("expected non-zero time")
	}

	row2 := []interface{}{nil}
	result2 := scanTime(row2, 0)
	if !result2.IsZero() {
		t.Errorf("expected zero time for nil input, got %v", result2)
	}
}

func TestHandleDBError(t *testing.T) {
	err := handleDBError(nil, "test")
	if err != nil {
		t.Error("expected no error")
	}
}

func TestHandleWriteError(t *testing.T) {
	result := handleWriteError(rqlite.WriteResult{}, "test")
	if result != nil {
		t.Error("expected no error")
	}
}

func TestHandleQueryError(t *testing.T) {
	result := handleQueryError(rqlite.QueryResult{}, "test")
	if result != nil {
		t.Error("expected no error")
	}
}

func TestCalculateNextExecutionTime(t *testing.T) {
	tests := []struct {
		name        string
		cronExpr    string
		isEnabled   bool
		expectEmpty bool
	}{
		{
			name:        "valid 6-field cron enabled",
			cronExpr:    "0 0 12 * * *",
			isEnabled:   true,
			expectEmpty: false,
		},
		{
			name:        "valid 5-field cron enabled",
			cronExpr:    "0 12 * * *",
			isEnabled:   true,
			expectEmpty: false,
		},
		{
			name:        "disabled task",
			cronExpr:    "0 0 12 * * *",
			isEnabled:   false,
			expectEmpty: true,
		},
		{
			name:        "empty cron expression",
			cronExpr:    "",
			isEnabled:   true,
			expectEmpty: true,
		},
		{
			name:        "invalid cron expression",
			cronExpr:    "invalid cron",
			isEnabled:   true,
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateNextExecutionTime(tt.cronExpr, tt.isEnabled)

			if tt.expectEmpty && result != "" {
				t.Errorf("expected empty string, got %q", result)
			}
			if !tt.expectEmpty && result == "" {
				t.Error("expected non-empty result, got empty string")
			}

			if !tt.expectEmpty {
				_, err := time.Parse(time.RFC3339, result)
				if err != nil {
					t.Errorf("expected RFC3339 format, got %q: %v", result, err)
				}
			}
		})
	}
}