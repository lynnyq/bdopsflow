package driver

import (
	"fmt"
	"strings"
	"testing"

	gohive "github.com/beltran/gohive"
	"github.com/pkg/errors"
)

// TestApplyLimitToSQL 测试在 SQL 语句末尾添加 LIMIT 的逻辑
func TestApplyLimitToSQL(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		limit    int
		dsType   string
		expected string
	}{
		// limit <= 0 时不添加
		{"limit<=0 不添加", "SELECT * FROM t", 0, "mysql", "SELECT * FROM t"},
		{"limit负数 不添加", "SELECT * FROM t", -1, "mysql", "SELECT * FROM t"},

		// 无 LIMIT 子句，添加系统限制
		{"mysql 添加 LIMIT", "SELECT * FROM t", 100, "mysql", "SELECT * FROM t LIMIT 100"},
		{"sqlite 添加 LIMIT", "SELECT * FROM t", 50, "sqlite", "SELECT * FROM t LIMIT 50"},
		{"rqlite 添加 LIMIT", "SELECT * FROM t", 50, "rqlite", "SELECT * FROM t LIMIT 50"},
		{"starrocks 添加 LIMIT", "SELECT * FROM t", 200, "starrocks", "SELECT * FROM t LIMIT 200"},
		{"doris 添加 LIMIT", "SELECT * FROM t", 200, "doris", "SELECT * FROM t LIMIT 200"},
		{"trino 添加 LIMIT", "SELECT * FROM t", 200, "trino", "SELECT * FROM t LIMIT 200"},
		{"hive 添加 LIMIT", "SELECT * FROM t", 500, "hive", "SELECT * FROM t LIMIT 500"},
		{"kyuubi 添加 LIMIT", "SELECT * FROM t", 500, "kyuubi", "SELECT * FROM t LIMIT 500"},
		{"spark 添加 LIMIT", "SELECT * FROM t", 500, "spark", "SELECT * FROM t LIMIT 500"},
		{"未知类型默认添加 LIMIT", "SELECT * FROM t", 100, "unknown", "SELECT * FROM t LIMIT 100"},

		// 非 SELECT/WITH 语句不添加 LIMIT
		{"SHOW 不添加 LIMIT", "SHOW TABLES", 100, "mysql", "SHOW TABLES"},
		{"DESCRIBE 不添加 LIMIT", "DESCRIBE t", 100, "mysql", "DESCRIBE t"},
		{"EXPLAIN 不添加 LIMIT", "EXPLAIN SELECT * FROM t", 100, "mysql", "EXPLAIN SELECT * FROM t"},
		{"INSERT 不添加 LIMIT", "INSERT INTO t VALUES(1)", 100, "mysql", "INSERT INTO t VALUES(1)"},

		// WITH (CTE) 语句添加 LIMIT
		{"WITH 添加 LIMIT", "WITH cte AS (SELECT 1) SELECT * FROM cte", 100, "mysql", "WITH cte AS (SELECT 1) SELECT * FROM cte LIMIT 100"},

		// 已有 LIMIT，用户 LIMIT 大于系统限制，替换为系统限制
		{"用户LIMIT大于系统限制 替换", "SELECT * FROM t LIMIT 2000", 1000, "mysql", "SELECT * FROM t LIMIT 1000"},
		{"用户LIMIT大于系统限制 hive替换", "SELECT * FROM t LIMIT 2000", 1000, "hive", "SELECT * FROM t LIMIT 1000"},

		// 已有 LIMIT，用户 LIMIT 小于等于系统限制，保持原样
		{"用户LIMIT小于系统限制 保持", "SELECT * FROM t LIMIT 50", 1000, "mysql", "SELECT * FROM t LIMIT 50"},
		{"用户LIMIT等于系统限制 保持", "SELECT * FROM t LIMIT 1000", 1000, "mysql", "SELECT * FROM t LIMIT 1000"},

		// 带 OFFSET 的情况：用户 LIMIT 大于系统限制时，替换并去除 OFFSET（兼容 Hive/Spark）
		{"带OFFSET 替换去除OFFSET", "SELECT * FROM t LIMIT 2000 OFFSET 100", 1000, "hive", "SELECT * FROM t LIMIT 1000"},
		{"带逗号OFFSET 替换去除OFFSET", "SELECT * FROM t LIMIT 2000, 100", 1000, "hive", "SELECT * FROM t LIMIT 1000"},

		// 带 OFFSET 但用户 LIMIT 小于系统限制，保持原样
		{"带OFFSET 用户LIMIT小 保持", "SELECT * FROM t LIMIT 50 OFFSET 100", 1000, "mysql", "SELECT * FROM t LIMIT 50 OFFSET 100"},

		// 末尾分号被规范化
		{"末尾分号规范化", "SELECT * FROM t;", 100, "mysql", "SELECT * FROM t LIMIT 100"},
		{"hive 末尾分号取最后语句", "SELECT 1; SELECT * FROM t", 100, "hive", "SELECT * FROM t LIMIT 100"},

		// 多行格式 SQL：SELECT 后为换行符，必须正确添加 LIMIT
		{"多行SQL SELECT后换行 添加LIMIT", "SELECT\n  *\nFROM\n  `t`", 20, "mysql", "SELECT\n  *\nFROM\n  `t` LIMIT 20"},
		{"多行SQL SELECT后制表符 添加LIMIT", "SELECT\t*\tFROM\t`t`", 20, "mysql", "SELECT\t*\tFROM\t`t` LIMIT 20"},
		{"多行SQL WITH后换行 添加LIMIT", "WITH cte AS\n  (SELECT 1)\nSELECT * FROM cte", 100, "mysql", "WITH cte AS\n  (SELECT 1)\nSELECT * FROM cte LIMIT 100"},
		// 多行 SQL 中 LIMIT 独占一行，应正确识别并替换为较小值
		// replaceLimitValue 正则会将 \nLIMIT 替换为 LIMIT（空白符被 \s+ 消耗后用空格替代）
		{"多行SQL LIMIT独占一行 替换", "SELECT *\nFROM t\nLIMIT 2000", 1000, "mysql", "SELECT *\nFROM t LIMIT 1000"},
		{"多行SQL LIMIT独占一行 保持", "SELECT *\nFROM t\nLIMIT 50", 1000, "mysql", "SELECT *\nFROM t\nLIMIT 50"},
		// 带反引号表名的多行 SQL（用户实际场景）
		{"多行SQL 带反引号表名", "SELECT\n  *\nFROM\n  `bdopsflow_audit_logs`", 20, "mysql", "SELECT\n  *\nFROM\n  `bdopsflow_audit_logs` LIMIT 20"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyLimitToSQL(tt.sql, tt.limit, tt.dsType)
			if got != tt.expected {
				t.Errorf("ApplyLimitToSQL(%q, %d, %q) = %q, want %q", tt.sql, tt.limit, tt.dsType, got, tt.expected)
			}
		})
	}
}

// TestExtractUserLimit 测试从 SQL 中提取用户指定的 LIMIT 值
func TestExtractUserLimit(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected int
	}{
		{"简单 LIMIT", "SELECT * FROM t LIMIT 100", 100},
		{"LIMIT 小写", "select * from t limit 50", 50},
		{"LIMIT 大小写混合", "Select * From t LiMiT 200", 200},
		{"带 OFFSET", "SELECT * FROM t LIMIT 100 OFFSET 200", 100},
		{"带逗号", "SELECT * FROM t LIMIT 100,200", 100},
		{"无 LIMIT", "SELECT * FROM t", 0},
		{"LIMIT 0", "SELECT * FROM t LIMIT 0", 0},
		{"多语句返回第一个 LIMIT", "SELECT 1 LIMIT 10; SELECT 2 LIMIT 20", 10},
		{"空字符串", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUserLimit(tt.sql)
			if got != tt.expected {
				t.Errorf("extractUserLimit(%q) = %d, want %d", tt.sql, got, tt.expected)
			}
		})
	}
}

// TestReplaceLimitValue 测试替换 SQL 中的 LIMIT 值
func TestReplaceLimitValue(t *testing.T) {
	tests := []struct {
		name            string
		sql             string
		oldLimit        int
		newLimit        int
		wantContains    string
		wantNotContains string
	}{
		{
			name:            "简单替换",
			sql:             "SELECT * FROM t LIMIT 2000",
			oldLimit:        2000,
			newLimit:        1000,
			wantContains:    "LIMIT 1000",
			wantNotContains: "LIMIT 2000",
		},
		{
			name:            "带 OFFSET 替换并去除 OFFSET",
			sql:             "SELECT * FROM t LIMIT 2000 OFFSET 100",
			oldLimit:        2000,
			newLimit:        1000,
			wantContains:    "LIMIT 1000",
			wantNotContains: "OFFSET",
		},
		{
			name:            "带逗号格式替换",
			sql:             "SELECT * FROM t LIMIT 2000, 100",
			oldLimit:        2000,
			newLimit:        1000,
			wantContains:    "LIMIT 1000",
			wantNotContains: "2000",
		},
		{
			name:            "小写 limit 替换",
			sql:             "select * from t limit 2000",
			oldLimit:        2000,
			newLimit:        500,
			wantContains:    "LIMIT 500",
			wantNotContains: "2000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceLimitValue(tt.sql, tt.oldLimit, tt.newLimit)
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("replaceLimitValue(%q, %d, %d) = %q, want to contain %q", tt.sql, tt.oldLimit, tt.newLimit, got, tt.wantContains)
			}
			if tt.wantNotContains != "" && strings.Contains(got, tt.wantNotContains) {
				t.Errorf("replaceLimitValue(%q, %d, %d) = %q, should NOT contain %q", tt.sql, tt.oldLimit, tt.newLimit, got, tt.wantNotContains)
			}
		})
	}
}

// TestIsConnectionErrorBase 测试 base.go 中的 isConnectionError 连接错误判断
// 注意：errors.go 中也有一个导出的 IsConnectionError，由 errors_test.go 中的 TestIsConnectionError 测试
func TestIsConnectionErrorBase(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil 错误", nil, false},
		{"broken pipe", fmt.Errorf("write tcp: broken pipe"), true},
		{"connection reset", fmt.Errorf("read tcp: connection reset by peer"), true},
		{"connection refused", fmt.Errorf("dial tcp: connection refused"), true},
		{"network unreachable", fmt.Errorf("network is unreachable"), true},
		{"i/o timeout", fmt.Errorf("i/o timeout"), true},
		{"dial tcp", fmt.Errorf("dial tcp 127.0.0.1:3306: connect: connection refused"), true},
		{"no such host", fmt.Errorf("dial tcp: lookup example.com: no such host"), true},
		{"ttransport", fmt.Errorf("ttransport error"), true},
		{"transport error", fmt.Errorf("transport error occurred"), true},
		{"eof", fmt.Errorf("EOF: unexpected eof"), true},
		{"普通错误", fmt.Errorf("syntax error near 'FROM'"), false},
		{"空错误", fmt.Errorf(""), false},
		{"权限错误", fmt.Errorf("access denied for user"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isConnectionError(tt.err)
			if got != tt.expected {
				t.Errorf("isConnectionError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

// TestExtractGohiveError 测试从 gohive 错误中提取信息
func TestExtractGohiveError(t *testing.T) {
	t.Run("nil 错误返回 nil", func(t *testing.T) {
		got := extractGohiveError(nil, "wrap")
		if got != nil {
			t.Errorf("extractGohiveError(nil, ...) = %v, want nil", got)
		}
	})

	t.Run("HiveError 带 ErrorCode 和 Message", func(t *testing.T) {
		// HiveError 的 error 字段是未导出的嵌入式接口，无法从包外设置
		// 但 errors.As 仍然能识别 HiveError 类型
		hiveErr := gohive.HiveError{
			Message:   "Table 'test' does not exist",
			ErrorCode: 10001,
		}
		got := extractGohiveError(hiveErr, "查询失败")
		if got == nil {
			t.Fatal("expected error, got nil")
		}
		msg := got.Error()
		if !strings.Contains(msg, "查询失败") {
			t.Errorf("error should contain wrap message, got: %s", msg)
		}
		if !strings.Contains(msg, "Table 'test' does not exist") {
			t.Errorf("error should contain hive error message, got: %s", msg)
		}
		if !strings.Contains(msg, "10001") {
			t.Errorf("error should contain error code, got: %s", msg)
		}
	})

	t.Run("HiveError ErrorCode 为 0", func(t *testing.T) {
		hiveErr := gohive.HiveError{
			Message:   "some hive error",
			ErrorCode: 0,
		}
		got := extractGohiveError(hiveErr, "执行错误")
		if got == nil {
			t.Fatal("expected error, got nil")
		}
		msg := got.Error()
		if !strings.Contains(msg, "执行错误") {
			t.Errorf("error should contain wrap message, got: %s", msg)
		}
		if !strings.Contains(msg, "some hive error") {
			t.Errorf("error should contain hive message, got: %s", msg)
		}
		// ErrorCode 为 0 时不输出 errorCode
		if strings.Contains(msg, "errorCode") {
			t.Errorf("error should not contain errorCode when 0, got: %s", msg)
		}
	})

	t.Run("普通 error 包装", func(t *testing.T) {
		err := fmt.Errorf("some random error")
		got := extractGohiveError(err, "操作失败")
		if got == nil {
			t.Fatal("expected error, got nil")
		}
		msg := got.Error()
		if !strings.Contains(msg, "操作失败") {
			t.Errorf("error should contain wrap message, got: %s", msg)
		}
		if !strings.Contains(msg, "some random error") {
			t.Errorf("error should contain original message, got: %s", msg)
		}
	})

	t.Run("operation in state 特殊错误信息", func(t *testing.T) {
		err := fmt.Errorf("operation in state without task status or error message")
		got := extractGohiveError(err, "查询执行")
		if got == nil {
			t.Fatal("expected error, got nil")
		}
		msg := got.Error()
		if !strings.Contains(msg, "Hive未返回详细错误信息") {
			t.Errorf("error should contain friendly hive error message, got: %s", msg)
		}
	})

	t.Run("包装过的 error 不含 HiveError", func(t *testing.T) {
		err := errors.Wrap(fmt.Errorf("plain error"), "outer wrap")
		got := extractGohiveError(err, "查询失败")
		if got == nil {
			t.Fatal("expected error, got nil")
		}
		msg := got.Error()
		if !strings.Contains(msg, "查询失败") {
			t.Errorf("error should contain wrap message, got: %s", msg)
		}
		if !strings.Contains(msg, "plain error") {
			t.Errorf("error should contain original message, got: %s", msg)
		}
	})
}

// TestIsSupported 测试驱动类型是否被支持
func TestIsSupported(t *testing.T) {
	tests := []struct {
		dsType   string
		expected bool
	}{
		{"mysql", true},
		{"sqlite", true},
		{"hive", true},
		{"kyuubi", true},
		{"spark", true},
		{"trino", true},
		{"starrocks", true},
		{"doris", true},
		{"rqlite", true},
		{"postgres", false},
		{"clickhouse", false},
		{"", false},
		{"MYSQL", false}, // 区分大小写
		{"mysql ", false},
	}

	for _, tt := range tests {
		t.Run(tt.dsType, func(t *testing.T) {
			got := IsSupported(tt.dsType)
			if got != tt.expected {
				t.Errorf("IsSupported(%q) = %v, want %v", tt.dsType, got, tt.expected)
			}
		})
	}
}

// TestGetDriver_ErrorCase 测试获取不支持的驱动时返回错误
func TestGetDriver_ErrorCase(t *testing.T) {
	_, err := GetDriver("nonexistent")
	if err == nil {
		t.Error("GetDriver with unsupported type should return error")
	}
	if !strings.Contains(err.Error(), "unsupported datasource type") {
		t.Errorf("error should mention 'unsupported datasource type', got: %v", err)
	}
}

// TestSupportedTypes_Count 验证已注册驱动数量
func TestSupportedTypes_Count(t *testing.T) {
	types := SupportedTypes()
	if len(types) < 9 {
		t.Errorf("SupportedTypes() returned %d types, want at least 9", len(types))
	}
}

// TestRegisterDriver_Custom 注册并获取自定义驱动
func TestRegisterDriver_Custom(t *testing.T) {
	// 注意：由于全局注册表，这里注册一个测试驱动并验证
	RegisterDriver("test-custom-driver", NewMySQLDriver)
	// 测试结束后清理注册表，避免污染其他测试（如 TestSupportedTypes）
	defer UnregisterDriver("test-custom-driver")

	if !IsSupported("test-custom-driver") {
		t.Error("test-custom-driver should be supported after registration")
	}

	d, err := GetDriver("test-custom-driver")
	if err != nil {
		t.Fatalf("GetDriver(test-custom-driver) failed: %v", err)
	}
	if d == nil {
		t.Error("GetDriver(test-custom-driver) returned nil driver")
	}
}

// === IsRetryableError 测试 ===
// IsRetryableError 对非 DatasourceError 委托给 isConnectionError（仅识别连接错误关键字），
// 对 DatasourceError 则使用其 Retryable 字段。
// 注意："too many connections" 不在 isConnectionError 的关键字列表中，因此不可重试。

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"connection refused - retryable", fmt.Errorf("connection refused"), true},
		{"broken pipe - retryable", fmt.Errorf("broken pipe"), true},
		{"i/o timeout - retryable", fmt.Errorf("i/o timeout"), true},
		{"network unreachable - retryable", fmt.Errorf("network is unreachable"), true},
		// "too many connections" 不被 isConnectionError 识别为连接错误，因此不可重试
		{"too many connections - not retryable by plain error", fmt.Errorf("too many connections"), false},
		{"ttransport - retryable", fmt.Errorf("ttransport error"), true},
		{"syntax error - not retryable", fmt.Errorf("syntax error near SELECT"), false},
		{"access denied - not retryable", fmt.Errorf("access denied for user"), false},
		{"permission denied - not retryable", fmt.Errorf("permission denied"), false},
		{"empty error", fmt.Errorf(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryableError(tt.err)
			if got != tt.want {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsRetryableError_WithDatasourceError(t *testing.T) {
	// 测试 DatasourceError 的可重试判断
	retryableErr := &DatasourceError{
		Err:      fmt.Errorf("connection lost"),
		Category: ErrCategoryConnection,
		Retryable: true,
	}
	if !IsRetryableError(retryableErr) {
		t.Error("retryable DatasourceError should be retryable")
	}

	nonRetryableErr := &DatasourceError{
		Err:      fmt.Errorf("syntax error"),
		Category: ErrCategoryQuery,
		Retryable: false,
	}
	if IsRetryableError(nonRetryableErr) {
		t.Error("non-retryable DatasourceError should not be retryable")
	}
}

// === DatasourceError.Error 格式测试 ===
// 基础场景由 errors_test.go 中的 TestDatasourceError_Error 覆盖，
// 这里补充带/不带 DatasourceType 的格式化路径。

func TestDatasourceError_ErrorFormat(t *testing.T) {
	tests := []struct {
		name     string
		err      *DatasourceError
		contains string
	}{
		{
			name: "with datasource type",
			err: &DatasourceError{
				Err:            fmt.Errorf("connection failed"),
				Category:       ErrCategoryConnection,
				DatasourceType: "mysql",
			},
			contains: "[mysql]",
		},
		{
			name: "without datasource type",
			err: &DatasourceError{
				Err:      fmt.Errorf("query failed"),
				Category: ErrCategoryQuery,
			},
			contains: "query error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			if !strings.Contains(msg, tt.contains) {
				t.Errorf("Error() = %q, want to contain %q", msg, tt.contains)
			}
		})
	}
}

// === DatasourceError.Unwrap 测试 ===

func TestDatasourceError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner error")
	dsErr := &DatasourceError{
		Err:      inner,
		Category: ErrCategoryConnection,
	}

	unwrapped := dsErr.Unwrap()
	if unwrapped != inner {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, inner)
	}

	// 验证 errors.Is 能通过 Unwrap 链找到内部错误
	if !errorsIs(dsErr, inner) {
		t.Error("errors.Is should find inner error through Unwrap chain")
	}
}

// errorsIs 是 errors.Is 的简单包装（避免直接导入标准库 errors 与 github.com/pkg/errors 冲突）
func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		type unwrapper interface {
			Unwrap() error
		}
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// === ClassifyError 综合测试 ===
// 基础场景由 errors_test.go 中的 TestClassifyError 覆盖，
// 这里补充更多错误类别和边界场景。

func TestClassifyError_Comprehensive(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		dsType         string
		wantCategory   string
		wantRetryable  bool
	}{
		{"nil error", nil, "mysql", "", false},
		{"connection refused", fmt.Errorf("connection refused"), "mysql", ErrCategoryConnection, true},
		{"broken pipe", fmt.Errorf("broken pipe"), "hive", ErrCategoryConnection, true},
		{"connection reset", fmt.Errorf("connection reset by peer"), "trino", ErrCategoryConnection, true},
		{"network unreachable", fmt.Errorf("network is unreachable"), "mysql", ErrCategoryConnection, true},
		{"no such host", fmt.Errorf("dial tcp: no such host"), "mysql", ErrCategoryConnection, true},
		{"dial tcp", fmt.Errorf("dial tcp 127.0.0.1:3306"), "mysql", ErrCategoryConnection, true},
		{"eof", fmt.Errorf("unexpected eof"), "mysql", ErrCategoryConnection, true},
		{"timeout", fmt.Errorf("query timeout"), "mysql", ErrCategoryTimeout, true},
		{"i/o timeout", fmt.Errorf("i/o timeout"), "mysql", ErrCategoryTimeout, true},
		{"deadline exceeded", fmt.Errorf("deadline exceeded"), "mysql", ErrCategoryTimeout, true},
		{"authentication", fmt.Errorf("authentication failed"), "mysql", ErrCategoryAuthentication, false},
		{"access denied", fmt.Errorf("access denied for user"), "mysql", ErrCategoryAuthentication, false},
		{"unauthorized", fmt.Errorf("unauthorized access"), "mysql", ErrCategoryAuthentication, false},
		{"permission denied", fmt.Errorf("permission denied"), "mysql", ErrCategoryPermission, false},
		{"forbidden", fmt.Errorf("forbidden operation"), "mysql", ErrCategoryPermission, false},
		{"too many connections", fmt.Errorf("too many connections"), "mysql", ErrCategoryResource, true},
		{"connection pool", fmt.Errorf("connection pool exhausted"), "mysql", ErrCategoryResource, true},
		{"ttransport", fmt.Errorf("ttransport error"), "hive", ErrCategoryConnection, true},
		{"transport error", fmt.Errorf("transport error"), "hive", ErrCategoryConnection, true},
		{"query error (default)", fmt.Errorf("syntax error near SELECT"), "mysql", ErrCategoryQuery, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.err, tt.dsType)
			if tt.err == nil {
				if got != nil {
					t.Errorf("ClassifyError(nil, ...) = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil DatasourceError")
			}
			if got.Category != tt.wantCategory {
				t.Errorf("Category = %q, want %q", got.Category, tt.wantCategory)
			}
			if got.Retryable != tt.wantRetryable {
				t.Errorf("Retryable = %v, want %v", got.Retryable, tt.wantRetryable)
			}
			if got.DatasourceType != tt.dsType {
				t.Errorf("DatasourceType = %q, want %q", got.DatasourceType, tt.dsType)
			}
		})
	}
}

// === escapeMySQLIdentifier 边界场景测试 ===
// 基础场景由 driver_test.go 中的 TestEscapeMySQLIdentifier 覆盖，
// 这里补充空字符串、多反引号、Unicode 等边界场景。

func TestEscapeMySQLIdentifier_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"name with multiple backticks", "u`s`e`r", "u``s``e``r"},
		{"empty string", "", ""},
		{"name with spaces", "my table", "my table"},
		{"name with special chars", "user-name", "user-name"},
		{"unicode name", "用户表", "用户表"},
		{"only backticks", "```", "``````"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeMySQLIdentifier(tt.input)
			if got != tt.want {
				t.Errorf("escapeMySQLIdentifier(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// === buildDSN 测试（MySQL/Doris/StarRocks） ===

func TestMySQLDriver_buildDSN(t *testing.T) {
	tests := []struct {
		name   string
		config DatasourceConfig
		want   string
	}{
		{
			name: "standard config",
			config: DatasourceConfig{
				Host:     "localhost",
				Port:     3306,
				Username: "root",
				Password: "pass",
				Database: "testdb",
			},
			want: "root:pass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=true&loc=Local",
		},
		{
			name: "default port when zero",
			config: DatasourceConfig{
				Host:     "localhost",
				Port:     0,
				Username: "user",
				Password: "pass",
				Database: "db",
			},
			want: "user:pass@tcp(localhost:3306)/db?charset=utf8mb4&parseTime=true&loc=Local",
		},
		{
			name: "with SSL",
			config: DatasourceConfig{
				Host:     "localhost",
				Port:     3306,
				Username: "root",
				Password: "pass",
				Database: "testdb",
				Config:   map[string]interface{}{"ssl": true},
			},
			want: "root:pass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=true&loc=Local&tls=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &MySQLDriver{config: tt.config}
			got := d.buildDSN()
			if got != tt.want {
				t.Errorf("buildDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDorisDriver_buildDSN(t *testing.T) {
	tests := []struct {
		name   string
		config DatasourceConfig
		want   string
	}{
		{
			name: "standard config with default port",
			config: DatasourceConfig{
				Host:     "localhost",
				Port:     0,
				Username: "root",
				Password: "pass",
				Database: "testdb",
			},
			want: "root:pass@tcp(localhost:9030)/testdb?charset=utf8mb4&parseTime=true&loc=Local",
		},
		{
			name: "custom port",
			config: DatasourceConfig{
				Host:     "doris.example.com",
				Port:     9031,
				Username: "admin",
				Password: "secret",
				Database: "analytics",
			},
			want: "admin:secret@tcp(doris.example.com:9031)/analytics?charset=utf8mb4&parseTime=true&loc=Local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DorisDriver{config: tt.config}
			got := d.buildDSN()
			if got != tt.want {
				t.Errorf("buildDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

// === DatasourceConfig 测试 ===

func TestDatasourceConfig_Fields(t *testing.T) {
	cfg := DatasourceConfig{
		Host:     "localhost",
		Port:     3306,
		Username: "root",
		Password: "password",
		Database: "testdb",
		Type:     "mysql",
	}

	if cfg.Host != "localhost" {
		t.Errorf("Host = %q, want 'localhost'", cfg.Host)
	}
	if cfg.Port != 3306 {
		t.Errorf("Port = %d, want 3306", cfg.Port)
	}
	if cfg.Username != "root" {
		t.Errorf("Username = %q, want 'root'", cfg.Username)
	}
	if cfg.Database != "testdb" {
		t.Errorf("Database = %q, want 'testdb'", cfg.Database)
	}
	if cfg.Type != "mysql" {
		t.Errorf("Type = %q, want 'mysql'", cfg.Type)
	}
}

// === PoolConfig 测试 ===

func TestDefaultPoolConfig(t *testing.T) {
	cfg := DefaultPoolConfig()

	if cfg.MaxOpen <= 0 {
		t.Errorf("MaxOpen should be positive, got %d", cfg.MaxOpen)
	}
	if cfg.MinIdle < 0 {
		t.Errorf("MinIdle should not be negative, got %d", cfg.MinIdle)
	}
	if cfg.MaxLifetime <= 0 {
		t.Errorf("MaxLifetime should be positive, got %v", cfg.MaxLifetime)
	}
	if cfg.AcquireTimeout <= 0 {
		t.Errorf("AcquireTimeout should be positive, got %v", cfg.AcquireTimeout)
	}
}

// === normalizeSQL / ExtractLastStatement / NormalizeSQLForType 补充测试 ===
// normalizeSQL 只移除一个尾部分号，不递归移除多个分号。

func TestNormalizeSQL_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		// ";;;" → TrimSpace → ";;;" → TrimSuffix ";" → ";;" → TrimSpace → ";;"
		{"only semicolons removes one", ";;;", ";;"},
		// "   " → TrimSpace → "" → TrimSuffix ";" → "" → TrimSpace → ""
		{"only whitespace", "   ", ""},
		// ";;SELECT 1" → 无尾部分号可移除
		{"leading semicolons kept", ";;SELECT 1", ";;SELECT 1"},
		// "SELECT 1;;;" → 移除一个尾部分号 → "SELECT 1;;"
		{"trailing semicolons removes one", "SELECT 1;;;", "SELECT 1;;"},
		// "SELECT 1; SELECT 2;" → 移除一个尾部分号
		{"multiple statements removes trailing", "SELECT 1; SELECT 2;", "SELECT 1; SELECT 2"},
		// "  SELECT 1;  " → TrimSpace → "SELECT 1;" → TrimSuffix ";" → "SELECT 1"
		{"with surrounding whitespace", "  SELECT 1;  ", "SELECT 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSQL(tt.input)
			if got != tt.want {
				t.Errorf("normalizeSQL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractLastStatement_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"single statement", "SELECT 1", "SELECT 1"},
		{"multiple statements", "SELECT 1; SELECT 2", "SELECT 2"},
		{"trailing semicolon", "SELECT 1;", "SELECT 1"},
		{"only semicolons", ";;;", ""},
		{"leading semicolons", ";;SELECT 1", "SELECT 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractLastStatement(tt.input)
			if got != tt.want {
				t.Errorf("ExtractLastStatement(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeSQLForType_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		sql    string
		dsType string
		want   string
	}{
		{"empty sql mysql", "", "mysql", ""},
		{"simple select mysql", "SELECT 1", "mysql", "SELECT 1"},
		{"trailing semicolon mysql", "SELECT 1;", "mysql", "SELECT 1"},
		{"multiple statements hive", "SELECT 1; SELECT 2", "hive", "SELECT 2"},
		{"multiple statements mysql", "SELECT 1; SELECT 2", "mysql", "SELECT 1; SELECT 2"},
		{"empty sql hive", "", "hive", ""},
		{"trailing semicolon hive", "SELECT 1;", "hive", "SELECT 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeSQLForType(tt.sql, tt.dsType)
			if got != tt.want {
				t.Errorf("NormalizeSQLForType(%q, %q) = %q, want %q", tt.sql, tt.dsType, got, tt.want)
			}
		})
	}
}

// === truncateSQL 边界场景测试 ===
// 基础场景由 driver_test.go 中的 TestTruncateSQL 覆盖，
// 这里补充零长度、Unicode 等边界场景。

func TestTruncateSQL_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		sql    string
		maxLen int
		want   string
	}{
		{"empty string", "", 100, ""},
		{"exact length", "SELECT 1", 9, "SELECT 1"},
		{"truncated", "SELECT * FROM users WHERE id = 1", 10, "SELECT * F..."},
		{"max len zero", "SELECT 1", 0, "..."},
		{"max len one", "SELECT 1", 1, "S..."},
		// truncateSQL 按字节截断（非 rune），中文字符占 3 字节，maxLen 需对齐到字符边界
		{"unicode truncated", "你好世界测试", 6, "你好..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateSQL(tt.sql, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateSQL(%q, %d) = %q, want %q", tt.sql, tt.maxLen, got, tt.want)
			}
		})
	}
}
