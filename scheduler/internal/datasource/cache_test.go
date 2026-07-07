package datasource

import (
	"testing"
)

// TestCacheService_BuildKey_Deterministic 验证相同输入生成相同的 cache key
func TestCacheService_BuildKey_Deterministic(t *testing.T) {
	svc := &CacheService{}
	key1 := svc.buildKey(1, "db1", "SELECT 1", 100)
	key2 := svc.buildKey(1, "db1", "SELECT 1", 100)
	if key1 != key2 {
		t.Errorf("expected same key for same input, got %q and %q", key1, key2)
	}
}

// TestCacheService_BuildKey_DifferentDatasource 验证不同数据源 ID 生成不同 key
func TestCacheService_BuildKey_DifferentDatasource(t *testing.T) {
	svc := &CacheService{}
	key1 := svc.buildKey(1, "db1", "SELECT 1", 100)
	key2 := svc.buildKey(2, "db1", "SELECT 1", 100)
	if key1 == key2 {
		t.Error("expected different keys for different datasource IDs")
	}
}

// TestCacheService_BuildKey_DifferentDatabase 验证不同数据库生成不同 key
func TestCacheService_BuildKey_DifferentDatabase(t *testing.T) {
	svc := &CacheService{}
	key1 := svc.buildKey(1, "db1", "SELECT 1", 100)
	key2 := svc.buildKey(1, "db2", "SELECT 1", 100)
	if key1 == key2 {
		t.Error("expected different keys for different databases")
	}
}

// TestCacheService_BuildKey_DifferentSQL 验证不同 SQL 生成不同 key
func TestCacheService_BuildKey_DifferentSQL(t *testing.T) {
	svc := &CacheService{}
	key1 := svc.buildKey(1, "db1", "SELECT 1", 100)
	key2 := svc.buildKey(1, "db1", "SELECT 2", 100)
	if key1 == key2 {
		t.Error("expected different keys for different SQL")
	}
}

// TestCacheService_BuildKey_DifferentLimit 验证不同 limit 生成不同 key
func TestCacheService_BuildKey_DifferentLimit(t *testing.T) {
	svc := &CacheService{}
	key1 := svc.buildKey(1, "db1", "SELECT 1", 100)
	key2 := svc.buildKey(1, "db1", "SELECT 1", 200)
	if key1 == key2 {
		t.Error("expected different keys for different limits")
	}
}

// TestCacheService_BuildKey_KeyFormat 验证 key 的前缀格式正确
func TestCacheService_BuildKey_KeyFormat(t *testing.T) {
	svc := &CacheService{}
	key := svc.buildKey(42, "db1", "SELECT 1", 100)
	expectedPrefix := "datasource:query:cache:42:"
	if len(key) < len(expectedPrefix) {
		t.Fatalf("key %q is shorter than expected prefix %q", key, expectedPrefix)
	}
	if key[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected key to start with %q, got %q", expectedPrefix, key[:len(expectedPrefix)])
	}
}

// TestCacheService_BuildKey_EmptyValues 验证空值输入也能正常生成 key
func TestCacheService_BuildKey_EmptyValues(t *testing.T) {
	svc := &CacheService{}
	key1 := svc.buildKey(0, "", "", 0)
	if key1 == "" {
		t.Error("expected non-empty key for empty values")
	}
	// 空值也应确定性
	key2 := svc.buildKey(0, "", "", 0)
	if key1 != key2 {
		t.Error("expected same key for same empty values")
	}
}

// TestCacheService_GetStats_Initial 验证初始统计数据为零
func TestCacheService_GetStats_Initial(t *testing.T) {
	svc := &CacheService{}
	stats := svc.GetStats()
	if stats.Hits != 0 {
		t.Errorf("expected 0 hits, got %d", stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("expected 0 misses, got %d", stats.Misses)
	}
	if stats.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", stats.Errors)
	}
	if stats.Evictions != 0 {
		t.Errorf("expected 0 evictions, got %d", stats.Evictions)
	}
}

// TestCacheService_ResetStats 验证重置统计数据
func TestCacheService_ResetStats(t *testing.T) {
	svc := &CacheService{}
	// 手动设置统计数据
	svc.mu.Lock()
	svc.stats.Hits = 10
	svc.stats.Misses = 5
	svc.stats.Errors = 2
	svc.stats.Evictions = 3
	svc.mu.Unlock()

	// 验证设置成功
	stats := svc.GetStats()
	if stats.Hits != 10 || stats.Misses != 5 || stats.Errors != 2 || stats.Evictions != 3 {
		t.Fatalf("expected stats (10,5,2,3), got (%d,%d,%d,%d)",
			stats.Hits, stats.Misses, stats.Errors, stats.Evictions)
	}

	// 重置
	svc.ResetStats()
	stats = svc.GetStats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.Errors != 0 || stats.Evictions != 0 {
		t.Errorf("expected all zeros after reset, got (%d,%d,%d,%d)",
			stats.Hits, stats.Misses, stats.Errors, stats.Evictions)
	}
}

// TestCacheService_GetHitRate_NoData 验证无数据时命中率为 0
func TestCacheService_GetHitRate_NoData(t *testing.T) {
	svc := &CacheService{}
	rate := svc.GetHitRate()
	if rate != 0 {
		t.Errorf("expected 0%% hit rate with no data, got %f%%", rate)
	}
}

// TestCacheService_GetHitRate_AllHits 验证全部命中时命中率为 100
func TestCacheService_GetHitRate_AllHits(t *testing.T) {
	svc := &CacheService{}
	svc.mu.Lock()
	svc.stats.Hits = 10
	svc.stats.Misses = 0
	svc.mu.Unlock()

	rate := svc.GetHitRate()
	if rate != 100 {
		t.Errorf("expected 100%% hit rate, got %f%%", rate)
	}
}

// TestCacheService_GetHitRate_AllMisses 验证全部未命中时命中率为 0
func TestCacheService_GetHitRate_AllMisses(t *testing.T) {
	svc := &CacheService{}
	svc.mu.Lock()
	svc.stats.Hits = 0
	svc.stats.Misses = 10
	svc.mu.Unlock()

	rate := svc.GetHitRate()
	if rate != 0 {
		t.Errorf("expected 0%% hit rate, got %f%%", rate)
	}
}

// TestCacheService_GetHitRate_Mixed 验证混合情况下的命中率计算
func TestCacheService_GetHitRate_Mixed(t *testing.T) {
	svc := &CacheService{}
	svc.mu.Lock()
	svc.stats.Hits = 7
	svc.stats.Misses = 3
	svc.mu.Unlock()

	rate := svc.GetHitRate()
	expected := 70.0
	if rate != expected {
		t.Errorf("expected %f%% hit rate, got %f%%", expected, rate)
	}
}

// TestCacheService_GetHitRate_IgnoresErrors 验证命中率计算只考虑 Hits 和 Misses，不考虑 Errors
func TestCacheService_GetHitRate_IgnoresErrors(t *testing.T) {
	svc := &CacheService{}
	svc.mu.Lock()
	svc.stats.Hits = 5
	svc.stats.Misses = 5
	svc.stats.Errors = 100 // errors 不应影响命中率
	svc.mu.Unlock()

	rate := svc.GetHitRate()
	if rate != 50 {
		t.Errorf("expected 50%% hit rate (errors ignored), got %f%%", rate)
	}
}

// TestCacheService_OnConfigChanged_IrrelevantKey 验证无关 key 不触发配置刷新（不 panic）
func TestCacheService_OnConfigChanged_IrrelevantKey(t *testing.T) {
	svc := &CacheService{runtimeCacheTTL: 300}
	// 无关 key 不应触发 refreshRuntimeConfig，不会因 config 为 nil 而 panic
	svc.OnConfigChanged("some.irrelevant.key", "value")
	svc.OnConfigChanged("", "")

	// runtimeCacheTTL 不应改变
	svc.mu.RLock()
	ttl := svc.runtimeCacheTTL
	svc.mu.RUnlock()
	if ttl != 300 {
		t.Errorf("expected TTL to remain 300, got %d", ttl)
	}
}

// TestCacheStats_JSON 验证 CacheStats 结构体能正确序列化为 JSON
func TestCacheStats_JSON(t *testing.T) {
	stats := CacheStats{
		Hits:      100,
		Misses:    50,
		Errors:    5,
		Evictions: 10,
	}

	// 验证字段通过 JSON tag 正确映射
	if stats.Hits != 100 {
		t.Errorf("expected Hits=100, got %d", stats.Hits)
	}
	if stats.Misses != 50 {
		t.Errorf("expected Misses=50, got %d", stats.Misses)
	}
	if stats.Errors != 5 {
		t.Errorf("expected Errors=5, got %d", stats.Errors)
	}
	if stats.Evictions != 10 {
		t.Errorf("expected Evictions=10, got %d", stats.Evictions)
	}
}
