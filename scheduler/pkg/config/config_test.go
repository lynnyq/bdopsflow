package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	cfg, err := New()
	if err != nil {
		t.Errorf("New() error = %v", err)
	}
	if cfg == nil {
		t.Error("New() returned nil")
	}
}

func TestNew_WithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("database:\n  host: localhost\n  port: 5432"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp config file: %v", err)
	}

	cfg, err := New(Options{ConfigFile: configFile})
	if err != nil {
		t.Errorf("New() error = %v", err)
	}

	if cfg.GetString("database.host", "") != "localhost" {
		t.Errorf("expected database.host=localhost")
	}

	if cfg.GetInt("database.port", 0) != 5432 {
		t.Errorf("expected database.port=5432")
	}
}

func TestNew_WithDefaults(t *testing.T) {
	cfg, err := New(Options{
		Defaults: map[string]string{
			"default_key": "default_value",
			"another_key": "another_value",
		},
	})
	if err != nil {
		t.Errorf("New() error = %v", err)
	}

	if cfg.GetString("default_key", "") != "default_value" {
		t.Errorf("expected default_key=default_value")
	}

	if cfg.GetString("another_key", "") != "another_value" {
		t.Errorf("expected another_key=another_value")
	}
}

func TestGetString(t *testing.T) {
	cfg := &Config{
		values: map[string]interface{}{
			"string_key": "hello",
			"int_key":    42,
			"float_key":  3.14,
		},
	}

	tests := []struct {
		name       string
		key        string
		defaultVal string
		expected   string
	}{
		{"existing string", "string_key", "", "hello"},
		{"int to string", "int_key", "", "42"},
		{"float to string", "float_key", "", "3.14"},
		{"missing key", "missing", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.GetString(tt.key, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("GetString(%q, %q) = %q, expected %q", tt.key, tt.defaultVal, result, tt.expected)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	cfg := &Config{
		values: map[string]interface{}{
			"int_key":     42,
			"int64_key":   int64(100),
			"float_key":   3.99,
			"string_key":  "123",
			"bad_string":  "not_a_number",
		},
	}

	tests := []struct {
		name       string
		key        string
		defaultVal int
		expected   int
	}{
		{"int", "int_key", 0, 42},
		{"int64", "int64_key", 0, 100},
		{"float", "float_key", 0, 3},
		{"string", "string_key", 0, 123},
		{"bad string", "bad_string", 0, 0},
		{"missing", "missing", 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.GetInt(tt.key, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("GetInt(%q, %d) = %d, expected %d", tt.key, tt.defaultVal, result, tt.expected)
			}
		})
	}
}

func TestGetInt32(t *testing.T) {
	cfg := &Config{
		values: map[string]interface{}{
			"int_key":    42,
			"int64_key":  int64(100),
			"float_key":  3.99,
			"string_key": "123",
		},
	}

	result := cfg.GetInt32("int_key", 0)
	if result != 42 {
		t.Errorf("GetInt32(int_key) = %d, expected 42", result)
	}

	result = cfg.GetInt32("missing", 10)
	if result != 10 {
		t.Errorf("GetInt32(missing) = %d, expected 10", result)
	}
}

func TestGetInt64(t *testing.T) {
	cfg := &Config{
		values: map[string]interface{}{
			"int_key":    42,
			"int64_key":  int64(100),
			"float_key":  3.99,
			"string_key": "123",
		},
	}

	result := cfg.GetInt64("int64_key", 0)
	if result != 100 {
		t.Errorf("GetInt64(int64_key) = %d, expected 100", result)
	}

	result = cfg.GetInt64("missing", 10)
	if result != 10 {
		t.Errorf("GetInt64(missing) = %d, expected 10", result)
	}
}

func TestGetBool(t *testing.T) {
	cfg := &Config{
		values: map[string]interface{}{
			"bool_true":   true,
			"bool_false":  false,
			"string_true": "true",
			"string_1":    "1",
			"string_yes":  "yes",
			"string_no":   "no",
			"string_0":    "0",
			"string_f":    "false",
		},
	}

	tests := []struct {
		name       string
		key        string
		defaultVal bool
		expected   bool
	}{
		{"bool true", "bool_true", false, true},
		{"bool false", "bool_false", true, false},
		{"string true", "string_true", false, true},
		{"string 1", "string_1", false, true},
		{"string yes", "string_yes", false, true},
		{"string no", "string_no", true, false},
		{"string 0", "string_0", true, false},
		{"string false", "string_f", true, false},
		{"missing", "missing", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.GetBool(tt.key, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("GetBool(%q, %v) = %v, expected %v", tt.key, tt.defaultVal, result, tt.expected)
			}
		})
	}
}

func TestGetFloat(t *testing.T) {
	cfg := &Config{
		values: map[string]interface{}{
			"float_key":  3.14,
			"int_key":    42,
			"int64_key":  int64(100),
			"string_key": "3.14159",
		},
	}

	result := cfg.GetFloat("float_key", 0)
	if result != 3.14 {
		t.Errorf("GetFloat(float_key) = %f, expected 3.14", result)
	}

	result = cfg.GetFloat("int_key", 0)
	if result != 42.0 {
		t.Errorf("GetFloat(int_key) = %f, expected 42.0", result)
	}

	result = cfg.GetFloat("missing", 2.5)
	if result != 2.5 {
		t.Errorf("GetFloat(missing) = %f, expected 2.5", result)
	}
}

func TestGetStringMap(t *testing.T) {
	cfg := &Config{
		values: map[string]interface{}{
			"nested": map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			"not_map": "string",
		},
	}

	result := cfg.GetStringMap("nested")
	if result == nil {
		t.Error("GetStringMap(nested) returned nil")
	} else if result["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", result["key1"])
	}

	result = cfg.GetStringMap("not_map")
	if result != nil {
		t.Errorf("GetStringMap(not_map) should return nil, got %v", result)
	}

	result = cfg.GetStringMap("missing")
	if result != nil {
		t.Errorf("GetStringMap(missing) should return nil, got %v", result)
	}
}

func TestGetValue_WithEnv(t *testing.T) {
	os.Setenv("TEST_KEY", "env_value")
	defer os.Unsetenv("TEST_KEY")

	cfg := &Config{
		values: map[string]interface{}{},
	}

	result := cfg.getValue("test.key")
	if result != "env_value" {
		t.Errorf("getValue(test.key) = %v, expected env_value", result)
	}
}

func TestGetStringMap_WithEnv(t *testing.T) {
	cfg := &Config{
		values: map[string]interface{}{
			"existing": "value",
		},
	}

	result := cfg.getValue("existing")
	if result != "value" {
		t.Errorf("getValue(existing) = %v, expected value", result)
	}
}

func TestToEnvKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"database.host", "DATABASE_HOST"},
		{"redis.port", "REDIS_PORT"},
		{"single", "SINGLE"},
		{"nested.path.to.key", "NESTED_PATH_TO_KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toEnvKey(tt.input)
			if result != tt.expected {
				t.Errorf("toEnvKey(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSplitKey(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a.b.c", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"a..b", []string{"a", "b"}},
		{".a.b", []string{"a", "b"}},
		{"a.b.", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitKey(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("splitKey(%q) = %v, expected %v", tt.input, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("splitKey(%q)[%d] = %q, expected %q", tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestToUpperSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "HELLO"},
		{"helloWorld", "HELLO_WORLD"},
		{"HelloWorld", "HELLO_WORLD"},
		{"HTTPServer", "H_T_T_P_SERVER"},
		{"myHTTPService", "MY_H_T_T_P_SERVICE"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toUpperSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("toUpperSnakeCase(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToUpper(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "HELLO"},
		{"Hello", "HELLO"},
		{"123", "123"},
		{"aBc123", "ABC123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toUpper(tt.input)
			if result != tt.expected {
				t.Errorf("toUpper(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFlattenMap(t *testing.T) {
	input := map[string]interface{}{
		"a": "value1",
		"b": map[string]interface{}{
			"c": "value2",
			"d": map[string]interface{}{
				"e": "value3",
			},
		},
	}

	result := flattenMap("", input)

	expected := map[string]interface{}{
		"a":       "value1",
		"b.c":     "value2",
		"b.d.e":   "value3",
	}

	for k, v := range expected {
		if result[k] != v {
			t.Errorf("flattenMap()[%q] = %v, expected %v", k, result[k], v)
		}
	}
}

func TestConfigFile(t *testing.T) {
	cfg := &Config{
		configFile: "/etc/config.yaml",
	}

	result := cfg.ConfigFile()
	if result != "/etc/config.yaml" {
		t.Errorf("ConfigFile() = %q, expected /etc/config.yaml", result)
	}
}

func TestGetStringSlice(t *testing.T) {
	cfg := &Config{
		values: map[string]interface{}{
			"slice_key": []interface{}{"a", "b", "c"},
			"mixed_slice": []interface{}{1, "2", 3.14},
			"string_comma_separated": "http://node1:4001,http://node2:4001,http://node3:4001",
			"string_with_spaces": "  a  ,  b  ,  c  ",
		},
	}

	tests := []struct {
		name       string
		key        string
		defaultVal []string
		expected   []string
	}{
		{"slice", "slice_key", []string{"default"}, []string{"a", "b", "c"}},
		{"mixed slice", "mixed_slice", []string{"default"}, []string{"1", "2", "3.14"}},
		{"comma separated string", "string_comma_separated", []string{"default"}, []string{"http://node1:4001", "http://node2:4001", "http://node3:4001"}},
		{"string with spaces", "string_with_spaces", []string{"default"}, []string{"a", "b", "c"}},
		{"missing key", "missing", []string{"default"}, []string{"default"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.GetStringSlice(tt.key, tt.defaultVal)
			if len(result) != len(tt.expected) {
				t.Errorf("GetStringSlice(%q) = %v, expected %v", tt.key, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("GetStringSlice(%q)[%d] = %q, expected %q", tt.key, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestSplitCommaSeparated(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b, c", []string{"a", "b", "c"}},
		{"", []string{}},
		{"single", []string{"single"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitCommaSeparated(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("splitCommaSeparated(%q) = %v, expected %v", tt.input, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("splitCommaSeparated(%q)[%d] = %q, expected %q", tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  hello  ", "hello"},
		{"\thello\n", "hello"},
		{"hello", "hello"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := trimSpace(tt.input)
			if result != tt.expected {
				t.Errorf("trimSpace(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}