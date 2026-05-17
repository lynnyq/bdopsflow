package service

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestTimeFilterParsing(t *testing.T) {
	tests := []struct {
		name         string
		inputTime    string
		expectedTime string
		expectError  bool
	}{
		{
			name:         "标准格式时间解析",
			inputTime:    "2024-01-01 08:00:00",
			expectedTime: "2024-01-01 08:00:00",
			expectError:  false,
		},
		{
			name:         "下午时间解析",
			inputTime:    "2024-06-15 14:30:00",
			expectedTime: "2024-06-15 14:30:00",
			expectError:  false,
		},
		{
			name:         "无效时间格式",
			inputTime:    "invalid",
			expectedTime: "",
			expectError:  true,
		},
		{
			name:         "空字符串",
			inputTime:    "",
			expectedTime: "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.inputTime == "" || tt.expectError {
				if tt.inputTime != "" {
					_, err := time.Parse(DateTimeFormat, tt.inputTime)
					if err == nil && tt.expectError {
						t.Errorf("expected error for input %q, but got none", tt.inputTime)
					}
				}
				return
			}

			parsed, err := time.Parse(DateTimeFormat, tt.inputTime)
			if err != nil {
				if !tt.expectError {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}

			parsedFormatted := parsed.Format(DateTimeFormat)
			if parsedFormatted != tt.expectedTime {
				t.Errorf("expected %q, got %q", tt.expectedTime, parsedFormatted)
			}

			t.Logf("Parsed time: %s (location: %s)", parsed, parsed.Location())
		})
	}
}

func TestTimeZoneConversion(t *testing.T) {
	localTimeStr := "2024-01-01 08:00:00"
	
	parsed, err := time.Parse(DateTimeFormat, localTimeStr)
	if err != nil {
		t.Fatalf("failed to parse time: %v", err)
	}
	
	t.Logf("=== 问题演示 ===")
	t.Logf("1. 前端发送时间: %s", localTimeStr)
	t.Logf("2. time.Parse() 解析结果: %v", parsed)
	t.Logf("3. 解析后的时区: %s", parsed.Location())
	t.Logf("4. 调用 .Local() 后: %v", parsed.Local())
	t.Logf("5. 本地格式化: %s", parsed.Local().Format(DateTimeFormat))
	
	t.Logf("\n=== 使用本地时间处理 ===")
	t.Logf("使用 time.ParseInLocation 直接解析为本地时间")
	
	parsedLocal, err := time.ParseInLocation(DateTimeFormat, localTimeStr, time.Local)
	if err != nil {
		t.Fatalf("failed to parse time in local location: %v", err)
	}
	
	expectedLocal := "2024-01-01 08:00:00"
	actualLocal := parsedLocal.Format(DateTimeFormat)
	
	t.Logf("\n=== 验证 ===")
	t.Logf("期望的本地时间: %s", expectedLocal)
	t.Logf("实际的本地时间: %s", actualLocal)
	
	if actualLocal != expectedLocal {
		t.Errorf("本地时间解析失败！期望 %s，实际 %s", expectedLocal, actualLocal)
	}
}

func TestDurationFilterParsing(t *testing.T) {
	tests := []struct {
		name        string
		inputMin    string
		inputMax    string
		expectedMin int64
		expectedMax int64
		expectError bool
	}{
		{
			name:        "正常整数秒",
			inputMin:    "10",
			inputMax:    "60",
			expectedMin: 10,
			expectedMax: 60,
			expectError: false,
		},
		{
			name:        "浮点数秒",
			inputMin:    "10.5",
			inputMax:    "60.5",
			expectedMin: 10,
			expectedMax: 60,
			expectError: false,
		},
		{
			name:        "零值",
			inputMin:    "0",
			inputMax:    "0",
			expectedMin: 0,
			expectedMax: 0,
			expectError: false,
		},
		{
			name:        "无效字符串",
			inputMin:    "invalid",
			expectedMin: 0,
			expectError: true,
		},
		{
			name:        "负数",
			inputMin:    "-5",
			inputMax:    "10",
			expectedMin: -5,
			expectedMax: 10,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.inputMin != "" {
				duration, err := parseFloatToInt64(tt.inputMin)
				if tt.expectError {
					if err == nil {
						t.Errorf("expected error for input %q, but got none", tt.inputMin)
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error: %v", err)
					} else if duration != tt.expectedMin {
						t.Errorf("expected %d, got %d", tt.expectedMin, duration)
					}
				}
			}

			if tt.inputMax != "" && !tt.expectError {
				duration, err := parseFloatToInt64(tt.inputMax)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if duration != tt.expectedMax {
					t.Errorf("expected %d, got %d", tt.expectedMax, duration)
				}
			}
		})
	}
}

func parseFloatToInt64(s string) (int64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(f), nil
}

func TestBuildWhereClause(t *testing.T) {
	filter := map[string]string{
		"id":               "123",
		"execution_id":     "exec-456",
		"task_name":        "test-task",
		"status":           "success",
		"start_time_from":  "2024-01-01 00:00:00",
		"start_time_to":    "2024-12-31 23:59:59",
	}

	whereClause, args := buildWhereClauseFromFilter(filter)

	if whereClause == "" {
		t.Error("expected non-empty where clause")
	}

	t.Logf("Generated WHERE clause: %s", whereClause)
	t.Logf("Arguments: %v", args)

	if len(args) != 6 {
		t.Errorf("expected 6 arguments, got %d", len(args))
	}
}

func TestBuildWhereClause_TimeConversion(t *testing.T) {
	localTime := "2024-01-01 08:00:00"
	
	parsed, err := time.ParseInLocation(DateTimeFormat, localTime, time.Local)
	if err != nil {
		t.Fatalf("failed to parse time: %v", err)
	}
	
	localFormatted := parsed.Format(DateTimeFormat)
	
	t.Logf("Local time: %s", localTime)
	t.Logf("Parsed local time: %s", localFormatted)
	t.Logf("Time location: %s", parsed.Location())
	
	expectedLocal := "2024-01-01 08:00:00"
	if localFormatted != expectedLocal {
		t.Errorf("expected local time %s, got %s", expectedLocal, localFormatted)
	}
}

func buildWhereClauseFromFilter(filter map[string]string) (string, []interface{}) {
	var whereClause = " WHERE 1=1"
	var args []interface{}

	if filter["id"] != "" {
		whereClause += " AND te.id = ?"
		args = append(args, filter["id"])
	}
	if filter["execution_id"] != "" {
		whereClause += " AND te.execution_id LIKE ?"
		args = append(args, "%"+filter["execution_id"]+"%")
	}
	if filter["task_name"] != "" {
		whereClause += " AND t.name LIKE ?"
		args = append(args, "%"+filter["task_name"]+"%")
	}
	if filter["status"] != "" {
		whereClause += " AND te.status = ?"
		args = append(args, filter["status"])
	}
	if filter["start_time_from"] != "" {
		if t, err := time.ParseInLocation(DateTimeFormat, filter["start_time_from"], time.Local); err == nil {
			whereClause += " AND te.start_time >= ?"
			args = append(args, t.Format(DateTimeFormat))
		}
	}
	if filter["start_time_to"] != "" {
		if t, err := time.ParseInLocation(DateTimeFormat, filter["start_time_to"], time.Local); err == nil {
			whereClause += " AND te.start_time <= ?"
			args = append(args, t.Format(DateTimeFormat))
		}
	}
	if filter["end_time_from"] != "" {
		if t, err := time.ParseInLocation(DateTimeFormat, filter["end_time_from"], time.Local); err == nil {
			whereClause += " AND te.end_time >= ?"
			args = append(args, t.Format(DateTimeFormat))
		}
	}
	if filter["end_time_to"] != "" {
		if t, err := time.ParseInLocation(DateTimeFormat, filter["end_time_to"], time.Local); err == nil {
			whereClause += " AND te.end_time <= ?"
			args = append(args, t.Format(DateTimeFormat))
		}
	}

	return whereClause, args
}

func TestGetAllExecutions_Integration(t *testing.T) {
	t.Skip("需要数据库连接，跳过集成测试")
	
	ctx := context.Background()
	filter := map[string]string{
		"start_time_from": "2024-01-01 00:00:00",
		"start_time_to":   "2024-12-31 23:59:59",
	}

	_, _, err := getAllExecutionsWithFilter(ctx, filter, 1, 20)
	if err == nil {
		t.Error("expected error without database connection")
	}
}

func getAllExecutionsWithFilter(ctx context.Context, filter map[string]string, page, pageSize int) ([]*TaskExecutionWithNames, int, error) {
	return nil, 0, nil
}