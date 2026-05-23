package datasource

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"testing"
)

func TestDefaultConfigValues(t *testing.T) {
	expectedKeys := []string{
		"datasource.default_limit",
		"datasource.max_export_rows",
		"datasource.cache_ttl",
		"datasource.cache_max_size",
		"datasource.query_timeout",
		"datasource.max_concurrent_per_user",
		"datasource.max_concurrent_global",
		"datasource.allow_write_sql",
		"datasource.history_retention_days",
		"datasource.connection_max_idle",
		"datasource.connection_max_open",
		"datasource.connection_max_lifetime",
		"datasource.max_sql_length",
		"datasource.max_cell_size",
		"datasource.health_check_interval",
		"datasource.test_timeout",
	}

	for _, key := range expectedKeys {
		v, ok := defaultConfigValues[key]
		if !ok {
			t.Errorf("defaultConfigValues missing key: %s", key)
		} else if v == "" {
			t.Errorf("defaultConfigValues[%s] is empty", key)
		}
	}

	if len(defaultConfigValues) != len(expectedKeys) {
		t.Errorf("defaultConfigValues has %d keys, expected %d", len(defaultConfigValues), len(expectedKeys))
	}
}

func newTestConfigService() *ConfigService {
	cache := make(map[string]string)
	for k, v := range defaultConfigValues {
		cache[k] = v
	}
	return &ConfigService{
		db:    nil,
		cache: cache,
	}
}

func TestConfigService_GetInt(t *testing.T) {
	svc := newTestConfigService()

	tests := []struct {
		key      string
		expected int
	}{
		{"datasource.default_limit", 1000},
		{"datasource.max_export_rows", 1000},
		{"datasource.cache_ttl", 300},
		{"datasource.cache_max_size", 100},
		{"datasource.query_timeout", 60},
		{"datasource.max_concurrent_per_user", 5},
		{"datasource.max_concurrent_global", 50},
		{"datasource.history_retention_days", 30},
		{"datasource.connection_max_idle", 5},
		{"datasource.connection_max_open", 10},
		{"datasource.connection_max_lifetime", 1800},
		{"datasource.max_sql_length", 65536},
		{"datasource.max_cell_size", 65536},
		{"datasource.health_check_interval", 300},
		{"datasource.test_timeout", 10},
		{"datasource.nonexistent_key", 0},
	}

	for _, tt := range tests {
		got := svc.GetInt(tt.key)
		if got != tt.expected {
			t.Errorf("GetInt(%q) = %d, want %d", tt.key, got, tt.expected)
		}
	}
}

func TestConfigService_GetBool(t *testing.T) {
	svc := newTestConfigService()

	if svc.GetBool("datasource.allow_write_sql") {
		t.Errorf("GetBool(%q) = true, want false (default value is 'false')", "datasource.allow_write_sql")
	}

	boolTests := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"yes", false},
		{"", false},
		{"TRUE", false},
		{"True", false},
	}

	for _, tt := range boolTests {
		svc.cache["test_bool_key"] = tt.value
		got := svc.GetBool("test_bool_key")
		if got != tt.expected {
			t.Errorf("GetBool with value %q = %v, want %v", tt.value, got, tt.expected)
		}
	}
}

func TestConfigService_Get(t *testing.T) {
	svc := newTestConfigService()

	got := svc.Get("datasource.cache_ttl")
	if got != "300" {
		t.Errorf("Get(datasource.cache_ttl) = %q, want %q", got, "300")
	}

	svc.cache["datasource.cache_ttl"] = "600"
	got = svc.Get("datasource.cache_ttl")
	if got != "600" {
		t.Errorf("Get(datasource.cache_ttl) after override = %q, want %q", got, "600")
	}

	delete(svc.cache, "datasource.cache_ttl")
	got = svc.Get("datasource.cache_ttl")
	if got != "300" {
		t.Errorf("Get(datasource.cache_ttl) fallback to default = %q, want %q", got, "300")
	}

	got = svc.Get("nonexistent_key")
	if got != "" {
		t.Errorf("Get(nonexistent_key) = %q, want empty string", got)
	}

	emptySvc := &ConfigService{
		db:    nil,
		cache: map[string]string{"custom_key": "custom_value"},
	}
	got = emptySvc.Get("custom_key")
	if got != "custom_value" {
		t.Errorf("Get(custom_key) = %q, want %q", got, "custom_value")
	}

	got = emptySvc.Get("datasource.cache_ttl")
	if got != "300" {
		t.Errorf("Get(datasource.cache_ttl) from defaultConfigValues with empty cache = %q, want %q", got, "300")
	}
}

func TestConfigService_GetAll(t *testing.T) {
	svc := newTestConfigService()

	all := svc.GetAll()
	if len(all) != len(defaultConfigValues) {
		t.Errorf("GetAll() returned %d keys, expected %d", len(all), len(defaultConfigValues))
	}

	for k, v := range defaultConfigValues {
		if all[k] != v {
			t.Errorf("GetAll()[%q] = %q, want %q", k, all[k], v)
		}
	}

	all["datasource.cache_ttl"] = "modified"
	got := svc.Get("datasource.cache_ttl")
	if got == "modified" {
		t.Errorf("GetAll() returned map should be a copy, modifying it should not affect cache")
	}
}

func TestConfigService_CacheDynamicUpdate(t *testing.T) {
	svc := newTestConfigService()

	tests := []struct {
		key    string
		newVal string
	}{
		{"datasource.default_limit", "500"},
		{"datasource.query_timeout", "120"},
		{"datasource.max_concurrent_per_user", "10"},
		{"datasource.max_concurrent_global", "100"},
		{"datasource.cache_ttl", "600"},
		{"datasource.cache_max_size", "200"},
		{"datasource.max_export_rows", "5000"},
		{"datasource.connection_max_idle", "10"},
		{"datasource.connection_max_open", "20"},
		{"datasource.connection_max_lifetime", "3600"},
		{"datasource.max_sql_length", "131072"},
		{"datasource.max_cell_size", "131072"},
		{"datasource.health_check_interval", "600"},
		{"datasource.test_timeout", "30"},
		{"datasource.history_retention_days", "90"},
	}

	for _, tt := range tests {
		svc.mu.Lock()
		svc.cache[tt.key] = tt.newVal
		svc.mu.Unlock()

		got := svc.Get(tt.key)
		if got != tt.newVal {
			t.Errorf("after cache update Get(%q) = %q, want %q", tt.key, got, tt.newVal)
		}

		gotInt := svc.GetInt(tt.key)
		expectedInt, _ := strconv.Atoi(tt.newVal)
		if gotInt != expectedInt {
			t.Errorf("after cache update GetInt(%q) = %d, want %d", tt.key, gotInt, expectedInt)
		}
	}
}

func TestConfigService_BoolDynamicUpdate(t *testing.T) {
	svc := newTestConfigService()

	if svc.GetBool("datasource.allow_write_sql") {
		t.Errorf("allow_write_sql should be false by default")
	}

	svc.mu.Lock()
	svc.cache["datasource.allow_write_sql"] = "true"
	svc.mu.Unlock()

	if !svc.GetBool("datasource.allow_write_sql") {
		t.Errorf("allow_write_sql should be true after update")
	}

	svc.mu.Lock()
	svc.cache["datasource.allow_write_sql"] = "false"
	svc.mu.Unlock()

	if svc.GetBool("datasource.allow_write_sql") {
		t.Errorf("allow_write_sql should be false after reverting")
	}
}

func TestConfigService_ConcurrentAccess(t *testing.T) {
	svc := newTestConfigService()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "datasource.cache_ttl"
			val := fmt.Sprintf("%d", 100+idx)

			svc.mu.Lock()
			svc.cache[key] = val
			svc.mu.Unlock()

			got := svc.Get(key)
			if got == "" {
				errors <- fmt.Errorf("goroutine %d: Get(%q) returned empty", idx, key)
			}

			_ = svc.GetInt(key)
			_ = svc.GetBool("datasource.allow_write_sql")
			_ = svc.GetAll()
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent access error: %v", err)
	}
}

func TestConfigService_GetAllWithMeta(t *testing.T) {
	svc := newTestConfigService()

	meta := svc.GetAllWithMeta()

	if len(meta) != len(configMetaList) {
		t.Errorf("GetAllWithMeta() returned %d items, expected %d", len(meta), len(configMetaList))
	}

	for i, m := range meta {
		if m.Key == "" {
			t.Errorf("meta[%d].Key is empty", i)
		}
		if m.Label == "" {
			t.Errorf("meta[%d].Label is empty for key %s", i, m.Key)
		}
		if m.Description == "" {
			t.Errorf("meta[%d].Description is empty for key %s", i, m.Key)
		}
		if m.Type == "" {
			t.Errorf("meta[%d].Type is empty for key %s", i, m.Key)
		}
		if m.DefaultValue == "" {
			t.Errorf("meta[%d].DefaultValue is empty for key %s", i, m.Key)
		}
		if m.Group == "" {
			t.Errorf("meta[%d].Group is empty for key %s", i, m.Key)
		}
		if m.Value == "" {
			t.Errorf("meta[%d].Value is empty for key %s", i, m.Key)
		}

		if m.Type == "number" {
			if m.MinValue == nil {
				t.Errorf("meta[%d].MinValue is nil for number type key %s", i, m.Key)
			}
			if m.MaxValue == nil {
				t.Errorf("meta[%d].MaxValue is nil for number type key %s", i, m.Key)
			}
		}
	}
}

func TestConfigService_GetAllWithMeta_ValuesMatchCache(t *testing.T) {
	svc := newTestConfigService()

	svc.mu.Lock()
	svc.cache["datasource.query_timeout"] = "120"
	svc.cache["datasource.default_limit"] = "500"
	svc.mu.Unlock()

	meta := svc.GetAllWithMeta()

	for _, m := range meta {
		if m.Key == "datasource.query_timeout" {
			if m.Value != "120" {
				t.Errorf("GetAllWithMeta() value for query_timeout = %q, want %q", m.Value, "120")
			}
			if m.DefaultValue != "60" {
				t.Errorf("GetAllWithMeta() default for query_timeout = %q, want %q", m.DefaultValue, "60")
			}
		}
		if m.Key == "datasource.default_limit" {
			if m.Value != "500" {
				t.Errorf("GetAllWithMeta() value for default_limit = %q, want %q", m.Value, "500")
			}
			if m.DefaultValue != "1000" {
				t.Errorf("GetAllWithMeta() default for default_limit = %q, want %q", m.DefaultValue, "1000")
			}
		}
		if m.Key == "datasource.cache_ttl" {
			if m.Value != "300" {
				t.Errorf("GetAllWithMeta() value for cache_ttl (unchanged) = %q, want %q", m.Value, "300")
			}
		}
	}
}

func TestConfigService_GetAllWithMeta_DefaultFallback(t *testing.T) {
	svc := &ConfigService{
		db:    nil,
		cache: map[string]string{},
	}

	meta := svc.GetAllWithMeta()

	for _, m := range meta {
		if m.Value != m.DefaultValue {
			t.Errorf("GetAllWithMeta() with empty cache: value for %s = %q, should default to %q", m.Key, m.Value, m.DefaultValue)
		}
	}
}

func TestConfigMetaList_ConsistencyWithDefaults(t *testing.T) {
	metaKeys := make(map[string]bool)
	for _, m := range configMetaList {
		if metaKeys[m.Key] {
			t.Errorf("duplicate key in configMetaList: %s", m.Key)
		}
		metaKeys[m.Key] = true

		defaultVal, ok := defaultConfigValues[m.Key]
		if !ok {
			t.Errorf("configMetaList has key %s not in defaultConfigValues", m.Key)
			continue
		}
		if m.DefaultValue != defaultVal {
			t.Errorf("configMetaList[%s].DefaultValue = %q, defaultConfigValues = %q", m.Key, m.DefaultValue, defaultVal)
		}
	}

	for key := range defaultConfigValues {
		if !metaKeys[key] {
			t.Errorf("defaultConfigValues has key %s not in configMetaList", key)
		}
	}
}

func TestConfigMetaList_DefaultValuesInRange(t *testing.T) {
	for _, m := range configMetaList {
		if m.Type != "number" {
			continue
		}

		defaultInt, err := strconv.Atoi(m.DefaultValue)
		if err != nil {
			t.Errorf("configMetaList[%s].DefaultValue %q is not a valid integer", m.Key, m.DefaultValue)
			continue
		}

		if m.MinValue != nil && defaultInt < *m.MinValue {
			t.Errorf("configMetaList[%s].DefaultValue %d < MinValue %d", m.Key, defaultInt, *m.MinValue)
		}
		if m.MaxValue != nil && defaultInt > *m.MaxValue {
			t.Errorf("configMetaList[%s].DefaultValue %d > MaxValue %d", m.Key, defaultInt, *m.MaxValue)
		}
	}
}

func TestConfigMetaList_JSONSerialization(t *testing.T) {
	svc := newTestConfigService()
	meta := svc.GetAllWithMeta()

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal GetAllWithMeta result: %v", err)
	}

	var unmarshaled []ConfigMeta
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal GetAllWithMeta result: %v", err)
	}

	if len(unmarshaled) != len(meta) {
		t.Errorf("round-trip: got %d items, expected %d", len(unmarshaled), len(meta))
	}

	for i, m := range unmarshaled {
		if m.Key != meta[i].Key {
			t.Errorf("round-trip meta[%d].Key = %q, want %q", i, m.Key, meta[i].Key)
		}
		if m.Value != meta[i].Value {
			t.Errorf("round-trip meta[%d].Value = %q, want %q", i, m.Value, meta[i].Value)
		}
		if m.DefaultValue != meta[i].DefaultValue {
			t.Errorf("round-trip meta[%d].DefaultValue = %q, want %q", i, m.DefaultValue, meta[i].DefaultValue)
		}
		if m.Type != meta[i].Type {
			t.Errorf("round-trip meta[%d].Type = %q, want %q", i, m.Type, meta[i].Type)
		}
		if m.Group != meta[i].Group {
			t.Errorf("round-trip meta[%d].Group = %q, want %q", i, m.Group, meta[i].Group)
		}
	}
}

func TestConfigMetaList_JSONFieldNames(t *testing.T) {
	svc := newTestConfigService()
	meta := svc.GetAllWithMeta()

	data, err := json.Marshal(meta[0])
	if err != nil {
		t.Fatalf("failed to marshal ConfigMeta: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal ConfigMeta to map: %v", err)
	}

	expectedFields := []string{"key", "label", "description", "type", "default_value", "value", "group"}
	for _, field := range expectedFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("ConfigMeta JSON missing field: %s", field)
		}
	}

	if _, ok := raw["min_value"]; !ok {
		t.Errorf("ConfigMeta JSON missing optional field: min_value (for number type)")
	}
	if _, ok := raw["max_value"]; !ok {
		t.Errorf("ConfigMeta JSON missing optional field: max_value (for number type)")
	}
	if _, ok := raw["unit"]; !ok {
		t.Errorf("ConfigMeta JSON missing optional field: unit")
	}
}

func TestConfigMetaList_AllTypesValid(t *testing.T) {
	validTypes := map[string]bool{"number": true, "boolean": true}

	for _, m := range configMetaList {
		if !validTypes[m.Type] {
			t.Errorf("configMetaList[%s] has invalid type: %q", m.Key, m.Type)
		}
	}
}

func TestConfigMetaList_BooleanNoRange(t *testing.T) {
	for _, m := range configMetaList {
		if m.Type == "boolean" {
			if m.MinValue != nil {
				t.Errorf("configMetaList[%s] is boolean but has MinValue", m.Key)
			}
			if m.MaxValue != nil {
				t.Errorf("configMetaList[%s] is boolean but has MaxValue", m.Key)
			}
			if m.Unit != "" {
				t.Errorf("configMetaList[%s] is boolean but has Unit %q", m.Key, m.Unit)
			}
		}
	}
}

func buildTestValidators() map[string]func(string) error {
	return map[string]func(string) error{
		"datasource.cache_ttl": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return fmt.Errorf("must be non-negative integer")
			}
			return nil
		},
		"datasource.query_timeout": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be positive integer")
			}
			return nil
		},
		"datasource.max_concurrent_per_user": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be positive integer")
			}
			return nil
		},
		"datasource.max_concurrent_global": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be positive integer")
			}
			return nil
		},
		"datasource.default_limit": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be positive integer")
			}
			return nil
		},
		"datasource.max_export_rows": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be positive integer")
			}
			return nil
		},
		"datasource.allow_write_sql": func(v string) error {
			if v != "true" && v != "false" {
				return fmt.Errorf("must be true or false")
			}
			return nil
		},
	}
}

func TestConfigService_Validators(t *testing.T) {
	validators := buildTestValidators()

	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{"cache_ttl valid", "datasource.cache_ttl", "300", false},
		{"cache_ttl zero", "datasource.cache_ttl", "0", false},
		{"cache_ttl negative", "datasource.cache_ttl", "-1", true},
		{"cache_ttl non-integer", "datasource.cache_ttl", "abc", true},

		{"query_timeout valid", "datasource.query_timeout", "60", false},
		{"query_timeout zero", "datasource.query_timeout", "0", true},
		{"query_timeout negative", "datasource.query_timeout", "-1", true},
		{"query_timeout non-integer", "datasource.query_timeout", "abc", true},

		{"max_concurrent_per_user valid", "datasource.max_concurrent_per_user", "5", false},
		{"max_concurrent_per_user zero", "datasource.max_concurrent_per_user", "0", true},
		{"max_concurrent_per_user negative", "datasource.max_concurrent_per_user", "-1", true},

		{"max_concurrent_global valid", "datasource.max_concurrent_global", "50", false},
		{"max_concurrent_global zero", "datasource.max_concurrent_global", "0", true},
		{"max_concurrent_global negative", "datasource.max_concurrent_global", "-1", true},

		{"default_limit valid", "datasource.default_limit", "1000", false},
		{"default_limit zero", "datasource.default_limit", "0", true},
		{"default_limit negative", "datasource.default_limit", "-1", true},

		{"max_export_rows valid", "datasource.max_export_rows", "1000", false},
		{"max_export_rows zero", "datasource.max_export_rows", "0", true},
		{"max_export_rows negative", "datasource.max_export_rows", "-1", true},

		{"allow_write_sql true", "datasource.allow_write_sql", "true", false},
		{"allow_write_sql false", "datasource.allow_write_sql", "false", false},
		{"allow_write_sql invalid", "datasource.allow_write_sql", "yes", true},
		{"allow_write_sql 1", "datasource.allow_write_sql", "1", true},
		{"allow_write_sql 0", "datasource.allow_write_sql", "0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, ok := validators[tt.key]
			if !ok {
				t.Fatalf("no validator for key: %s", tt.key)
			}

			err := validator(tt.value)
			if tt.wantErr && err == nil {
				t.Errorf("validator(%q, %q) expected error, got nil", tt.key, tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validator(%q, %q) unexpected error: %v", tt.key, tt.value, err)
			}
		})
	}
}

func TestConfigService_Set_ValidationError(t *testing.T) {
	svc := &ConfigService{
		db:    nil,
		cache: map[string]string{"datasource.cache_ttl": "300"},
	}

	err := svc.Set(nil, "datasource.cache_ttl", "-1", 1)
	if err == nil {
		t.Errorf("Set with invalid value expected validation error, got nil")
	}
}

func TestConfigService_Set_NoValidator(t *testing.T) {
	svc := &ConfigService{
		db:    nil,
		cache: map[string]string{"datasource.cache_max_size": "100"},
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Set with valid value and no validator should panic on nil db, but did not")
		}
	}()

	_ = svc.Set(nil, "datasource.cache_max_size", "200", 1)
}

func TestConfigService_AllNumericDefaultsMatchGetInt(t *testing.T) {
	svc := newTestConfigService()

	for _, m := range configMetaList {
		if m.Type != "number" {
			continue
		}

		expectedInt, err := strconv.Atoi(m.DefaultValue)
		if err != nil {
			t.Errorf("DefaultValue for %s is not a valid integer: %q", m.Key, m.DefaultValue)
			continue
		}

		got := svc.GetInt(m.Key)
		if got != expectedInt {
			t.Errorf("GetInt(%q) = %d, want %d (from DefaultValue)", m.Key, got, expectedInt)
		}
	}
}

func TestConfigService_AllMetaGroups(t *testing.T) {
	expectedGroups := map[string]bool{
		"查询":  true,
		"并发":  true,
		"安全":  true,
		"缓存":  true,
		"连接池": true,
		"其他":  true,
	}

	foundGroups := make(map[string]bool)
	for _, m := range configMetaList {
		foundGroups[m.Group] = true
		if !expectedGroups[m.Group] {
			t.Errorf("unexpected group in configMetaList: %q", m.Group)
		}
	}

	for g := range expectedGroups {
		if !foundGroups[g] {
			t.Errorf("expected group %q not found in configMetaList", g)
		}
	}
}
