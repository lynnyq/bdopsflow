package datasource

import (
	"testing"
)

func TestDsRowInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int64
	}{
		{"nil returns 0", nil, 0},
		{"int64 value", int64(123), 123},
		{"int64 zero", int64(0), 0},
		{"int64 negative", int64(-42), -42},
		{"int64 max", int64(9223372036854775807), int64(9223372036854775807)},
		{"int value", int(456), 456},
		{"int zero", int(0), 0},
		{"int negative", int(-10), -10},
		{"float64 value", float64(789.0), 789},
		{"float64 zero", float64(0.0), 0},
		{"float64 truncated", float64(123.7), 123},
		{"string numeric", "42", 42},
		{"string zero", "0", 0},
		{"string negative", "-5", -5},
		{"string with leading spaces", " 99", 99},
		{"string non-numeric", "abc", 0},
		{"string empty", "", 0},
		{"string float format", "3.14", 3},
		{"bool true", true, 0},
		{"bool false", false, 0},
		{"uint value", uint(100), 0},
		{"[]byte value", []byte("50"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dsRowInt64(tt.input)
			if got != tt.expected {
				t.Errorf("dsRowInt64(%v) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDsRowFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
	}{
		{"nil returns 0", nil, 0.0},
		{"float64 value", float64(123.456), 123.456},
		{"float64 zero", float64(0.0), 0.0},
		{"float64 negative", float64(-3.14), -3.14},
		{"float64 large", float64(1e10), 1e10},
		{"float64 small", float64(0.001), 0.001},
		{"int64 value", int64(789), 789.0},
		{"int64 zero", int64(0), 0.0},
		{"int64 negative", int64(-42), -42.0},
		{"int value", int(100), 100.0},
		{"int zero", int(0), 0.0},
		{"int negative", int(-7), -7.0},
		{"string numeric", "3.14", 3.14},
		{"string integer", "42", 42.0},
		{"string zero", "0", 0.0},
		{"string negative", "-2.5", -2.5},
		{"string non-numeric", "abc", 0.0},
		{"string empty", "", 0.0},
		{"string with leading spaces", " 1.5", 1.5},
		{"bool true", true, 0.0},
		{"bool false", false, 0.0},
		{"uint value", uint(50), 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dsRowFloat64(tt.input)
			if got != tt.expected {
				t.Errorf("dsRowFloat64(%v) = %f, want %f", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDsRowString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"nil returns empty", nil, ""},
		{"string value", "hello", "hello"},
		{"string empty", "", ""},
		{"string with spaces", "hello world", "hello world"},
		{"int64 value", int64(123), "123"},
		{"int64 zero", int64(0), "0"},
		{"int64 negative", int64(-42), "-42"},
		{"int value", int(456), "456"},
		{"float64 value", float64(3.14), "3.14"},
		{"float64 zero", float64(0), "0"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"uint value", uint(100), "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dsRowString(tt.input)
			if got != tt.expected {
				t.Errorf("dsRowString(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDsRowBool(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"nil returns false", nil, false},
		{"bool true", true, true},
		{"bool false", false, false},
		{"int64 non-zero", int64(1), true},
		{"int64 zero", int64(0), false},
		{"int64 large positive", int64(999), true},
		{"int64 negative", int64(-1), true},
		{"float64 non-zero", float64(1.0), true},
		{"float64 zero", float64(0.0), false},
		{"float64 small positive", float64(0.001), true},
		{"float64 negative", float64(-0.5), true},
		{"string value", "true", false},
		{"int value", int(1), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dsRowBool(tt.input)
			if got != tt.expected {
				t.Errorf("dsRowBool(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDsRowInt64_Float64Precision(t *testing.T) {
	largeFloat := float64(1<<53 + 1)
	result := dsRowInt64(largeFloat)
	if result != int64(largeFloat) {
		t.Errorf("dsRowInt64(float64 large) = %d, want %d", result, int64(largeFloat))
	}
}

func TestDsRowFloat64_Int64Large(t *testing.T) {
	largeInt := int64(1<<53 - 1)
	result := dsRowFloat64(largeInt)
	if result != float64(largeInt) {
		t.Errorf("dsRowFloat64(int64 large) = %f, want %f", result, float64(largeInt))
	}
}

func TestDsRowString_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"unicode", "你好世界", "你好世界"},
		{"newline", "line1\nline2", "line1\nline2"},
		{"tab", "col1\tcol2", "col1\tcol2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dsRowString(tt.input)
			if got != tt.expected {
				t.Errorf("dsRowString(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDsRowInt64_StringScientificNotation(t *testing.T) {
	result := dsRowInt64("1e5")
	if result != 1 {
		t.Errorf("dsRowInt64('1e5') = %d, want 1 (Sscanf %%d parses '1' and stops at 'e')", result)
	}
}

func TestDsRowFloat64_StringScientificNotation(t *testing.T) {
	result := dsRowFloat64("1e5")
	if result != 100000.0 {
		t.Errorf("dsRowFloat64('1e5') = %f, want 100000.0", result)
	}
}

func TestNewDatasourceService(t *testing.T) {
	svc := NewDatasourceService(nil, nil, nil)
	if svc == nil {
		t.Fatal("NewDatasourceService returned nil")
	}
	if svc.db != nil {
		t.Error("expected db to be nil")
	}
	if svc.crypto != nil {
		t.Error("expected crypto to be nil")
	}
	if svc.manager != nil {
		t.Error("expected manager to be nil")
	}
}

func TestNewDatasourceService_WithComponents(t *testing.T) {
	mgr := NewManager(nil, nil)
	svc := NewDatasourceService(nil, nil, mgr)
	if svc == nil {
		t.Fatal("NewDatasourceService returned nil")
	}
	if svc.manager != mgr {
		t.Error("expected manager to be set")
	}
}

func TestGetDatasourceDomainID_NilDB(t *testing.T) {
	svc := NewDatasourceService(nil, nil, nil)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling GetDatasourceDomainID with nil db, but did not panic")
		}
	}()
	_, _ = svc.GetDatasourceDomainID(1)
}

func TestCheckDatasourcePermission_NilDB(t *testing.T) {
	svc := NewDatasourceService(nil, nil, nil)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when calling CheckDatasourcePermission with nil db, but did not panic")
		}
	}()
	_, _ = svc.CheckDatasourcePermission(1, 1, "query")
}

func TestDsRowHelpers_Consistency(t *testing.T) {
	t.Run("dsRowInt64 and dsRowFloat64 agree on integers", func(t *testing.T) {
		vals := []interface{}{int64(42), int(42), float64(42.0), "42"}
		for _, v := range vals {
			intResult := dsRowInt64(v)
			floatResult := dsRowFloat64(v)
			if float64(intResult) != floatResult {
				t.Errorf("dsRowInt64(%v)=%d but dsRowFloat64(%v)=%f, inconsistent", v, intResult, v, floatResult)
			}
		}
	})

	t.Run("dsRowString round-trip for int64", func(t *testing.T) {
		original := int64(98765)
		str := dsRowString(original)
		parsed := dsRowInt64(str)
		if parsed != original {
			t.Errorf("round-trip: dsRowInt64(dsRowString(%d)) = %d", original, parsed)
		}
	})

	t.Run("dsRowString round-trip for float64", func(t *testing.T) {
		original := float64(3.14)
		str := dsRowString(original)
		parsed := dsRowFloat64(str)
		if parsed != original {
			t.Errorf("round-trip: dsRowFloat64(dsRowString(%f)) = %f", original, parsed)
		}
	})

	t.Run("nil returns zero values consistently", func(t *testing.T) {
		if dsRowInt64(nil) != 0 {
			t.Error("dsRowInt64(nil) should be 0")
		}
		if dsRowFloat64(nil) != 0 {
			t.Error("dsRowFloat64(nil) should be 0")
		}
		if dsRowString(nil) != "" {
			t.Error("dsRowString(nil) should be empty string")
		}
		if dsRowBool(nil) != false {
			t.Error("dsRowBool(nil) should be false")
		}
	})
}

func TestDsRowBool_Int64Boundary(t *testing.T) {
	if dsRowBool(int64(1)) != true {
		t.Error("dsRowBool(int64(1)) should be true")
	}
	if dsRowBool(int64(0)) != false {
		t.Error("dsRowBool(int64(0)) should be false")
	}
	if dsRowBool(int64(-1)) != true {
		t.Error("dsRowBool(int64(-1)) should be true (non-zero)")
	}
}

func TestDsRowBool_Float64Boundary(t *testing.T) {
	if dsRowBool(float64(0.5)) != true {
		t.Error("dsRowBool(float64(0.5)) should be true (non-zero)")
	}
	if dsRowBool(float64(0.0)) != false {
		t.Error("dsRowBool(float64(0.0)) should be false")
	}
	if dsRowBool(float64(-0.1)) != true {
		t.Error("dsRowBool(float64(-0.1)) should be true (non-zero)")
	}
}

func TestDsRowInt64_StringPartialParse(t *testing.T) {
	result := dsRowInt64("42abc")
	if result != 42 {
		t.Errorf("dsRowInt64('42abc') = %d, want 42 (Sscanf partial parse)", result)
	}
}

func TestDsRowFloat64_StringPartialParse(t *testing.T) {
	result := dsRowFloat64("3.14xyz")
	if result != 3.14 {
		t.Errorf("dsRowFloat64('3.14xyz') = %f, want 3.14 (Sscanf partial parse)", result)
	}
}
