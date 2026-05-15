package service

import (
	"context"
	"testing"
	"time"
)

func TestTimeFilterParsing(t *testing.T) {
	tests := []struct {
		name           string
		inputTime      string
		expectedTime   string
		expectError    bool
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
	// 模拟前端发送的北京时间
	localTimeStr := "2024-01-01 08:00:00"
	
	// 解析时间字符串
	parsed, err := time.Parse(DateTimeFormat, localTimeStr)
	if err != nil {
		t.Fatalf("failed to parse time: %v", err)
	}
	
	t.Logf("=== 问题演示 ===")
	t.Logf("1. 前端发送时间: %s", localTimeStr)
	t.Logf("2. time.Parse() 解析结果: %v", parsed)
	t.Logf("3. 解析后的时区: %s", parsed.Location())
	t.Logf("4. 调用 .UTC() 后: %v", parsed.UTC())
	t.Logf("5. UTC 格式化: %s", parsed.UTC().Format(DateTimeFormat))
	
	// 问题：time.Parse() 不识别时区，默认当作 UTC
	// 所以 "2024-01-01 08:00:00" 被当作 UTC 08:00:00
	// 但数据库实际存储的是北京时间转换的 UTC 时间
	
	// 正确做法：需要指定原始时区
	t.Logf("\n=== 解决方案 ===")
	t.Logf("需要将前端时间（假设为北京时间 UTC+8）转换为 UTC")
	
	// 方案1：使用 LoadLocation 指定时区
	loc, _ := time.LoadLocation("Asia/Shanghai")
	parsedWithTimezone, _ := time.ParseInLocation(DateTimeFormat, localTimeStr, loc)
	t.Logf("方案1 - 使用 ParseInLocation(Asia/Shanghai):")
	t.Logf("  解析后: %v", parsedWithTimezone)
	t.Logf("  转换为UTC: %v", parsedWithTimezone.UTC())
	t.Logf("  UTC格式化: %s", parsedWithTimezone.UTC().Format(DateTimeFormat))
	
	// 方案2：手动添加时区偏移
	t.Logf("\n方案2 - 手动添加8小时偏移:")
	manualOffset := parsed.Add(-8 * time.Hour)
	t.Logf("  原始 + (-8小时): %s", manualOffset.Format(DateTimeFormat))
	
	// 验证：数据库中存储的时间应该是 UTC
	// 北京时间 2024-01-01 08:00:00 -> UTC 2024-01-01 00:00:00
	expectedUTC := "2024-01-01 00:00:00"
	actualUTC := parsedWithTimezone.UTC().Format(DateTimeFormat)
	
	t.Logf("\n=== 验证 ===")
	t.Logf("期望的 UTC 时间: %s", expectedUTC)
	t.Logf("实际转换的 UTC 时间: %s", actualUTC)
	
	if actualUTC != expectedUTC {
		t.Errorf("时区转换失败！期望 %s，实际 %s", expectedUTC, actualUTC)
	}
}

func TestDurationFilterParsing(t *testing.T) {
	tests := []struct {
		name          string
		inputMin      string
		inputMax      string
		expectedMin   int64
		expectedMax   int64
		expectError   bool
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.inputMin != "" {
				duration, err := parseFloatToInt64(tt.inputMin)
				if tt.expectError && err == nil {
					t.Errorf("expected error for input %q", tt.inputMin)
				}
				if !tt.expectError && err == nil && duration != tt.expectedMin {
					t.Errorf("expected %d, got %d", tt.expectedMin, duration)
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
	var result int64
	_, err := parseFloatParam(s, &result)
	return result, err
}

func parseFloatParam(s string, result *int64) (bool, error) {
	if s == "" {
		return false, nil
	}
	
	var f float64
	_, err := parseFloat(s, &f)
	if err != nil {
		return false, err
	}
	*result = int64(f)
	return true, nil
}

func parseFloat(s string, result *float64) (bool, error) {
	if s == "" {
		return false, nil
	}
	
	var f float64
	_, err := parseFloatValue(s, &f)
	if err != nil {
		return false, err
	}
	*result = f
	return true, nil
}

func parseFloatValue(s string, result *float64) (bool, error) {
	if s == "" {
		return false, nil
	}
	
	var f float64 = 0
	var decimalDivisor float64 = 1
	var afterDecimal bool = false
	
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' && i == 0 {
			continue
		}
		if c == '.' {
			afterDecimal = true
			continue
		}
		if c >= '0' && c <= '9' {
			digit := float64(c - '0')
			if afterDecimal {
				decimalDivisor *= 10
				f += digit / decimalDivisor
			} else {
				f = f*10 + digit
			}
		}
	}
	
	if s[0] == '-' {
		f = -f
	}
	
	*result = f
	return true, nil
}

func TestBuildWhereClause(t *testing.T) {
	filter := map[string]string{
		"id":             "123",
		"execution_id":   "exec-456",
		"task_name":      "test-task",
		"status":         "success",
		"start_time_from": "2024-01-01 00:00:00",
		"start_time_to":   "2024-12-31 23:59:59",
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
	
	parsed, err := time.Parse(DateTimeFormat, localTime)
	if err != nil {
		t.Fatalf("failed to parse time: %v", err)
	}
	
	utcTime := parsed.UTC()
	utcFormatted := utcTime.Format(DateTimeFormat)
	
	t.Logf("Local time: %s", localTime)
	t.Logf("UTC time: %s", utcFormatted)
	
	expectedUTC := "2024-01-01 00:00:00"
	if utcFormatted != expectedUTC {
		t.Errorf("expected UTC time %s, got %s", expectedUTC, utcFormatted)
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
		if t, err := time.Parse(DateTimeFormat, filter["start_time_from"]); err == nil {
			utcTime := t.UTC()
			whereClause += " AND te.start_time >= ?"
			args = append(args, utcTime.Format(DateTimeFormat))
		}
	}
	if filter["start_time_to"] != "" {
		if t, err := time.Parse(DateTimeFormat, filter["start_time_to"]); err == nil {
			utcTime := t.UTC()
			whereClause += " AND te.start_time <= ?"
			args = append(args, utcTime.Format(DateTimeFormat))
		}
	}
	if filter["end_time_from"] != "" {
		if t, err := time.Parse(DateTimeFormat, filter["end_time_from"]); err == nil {
			utcTime := t.UTC()
			whereClause += " AND te.end_time >= ?"
			args = append(args, utcTime.Format(DateTimeFormat))
		}
	}
	if filter["end_time_to"] != "" {
		if t, err := time.Parse(DateTimeFormat, filter["end_time_to"]); err == nil {
			utcTime := t.UTC()
			whereClause += " AND te.end_time <= ?"
			args = append(args, utcTime.Format(DateTimeFormat))
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
