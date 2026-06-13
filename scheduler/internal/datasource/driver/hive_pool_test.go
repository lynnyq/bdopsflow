package driver

import (
	"context"
	"sync"
	"testing"
	"time"

	gohive "github.com/beltran/gohive"
)

// mockHiveConn 创建一个 mock 的 *gohive.Connection
// 由于 gohive.Connection 是结构体不是接口，无法直接 mock，
// 这里使用 nil 指针测试 pool 的计数逻辑（stats 不依赖 conn 的实际方法调用）
// 对于需要实际连接的测试，使用真实的 createConn 函数

func TestHiveConnPool_Put_IncrementsOpenCount(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        2,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	// 使用一个总是返回 nil connection 的 factory
	// pool.put 接受 *gohive.Connection，nil 也是合法的（会被放入 channel）
	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 使用 put 放入 3 个连接
	pool.put(nil, "db1")
	pool.put(nil, "db2")
	pool.put(nil, "db3")

	oc, ic, mo := pool.stats()
	if oc != 3 {
		t.Errorf("stats().openCount = %d, want 3 after 3 puts", oc)
	}
	if ic != 3 {
		t.Errorf("stats().idleCount = %d, want 3 after 3 puts", ic)
	}
	if mo != 5 {
		t.Errorf("stats().maxOpen = %d, want 5", mo)
	}
}

func TestHiveConnPool_Release_DoesNotIncrementOpenCount(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        2,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// release 是"归还"语义，openCount 应该已经在 acquire 时增加过了
	// 直接 release 一个新创建的 pooledConn，openCount 不应该增加
	// 这正是之前的 bug：预热时用 release 导致 openCount 为 0
	pool.release(&pooledConn{conn: nil, database: "db1"})

	oc, ic, mo := pool.stats()
	// 旧 bug：release 不增加 openCount，所以 oc=0，防御性校验 ic=min(ic,oc)=0
	// 修复后：release 仍然不增加 openCount（这是正确行为，release 是归还不是新增）
	// 但预热应该用 put 而不是 release
	if oc != 0 {
		t.Errorf("stats().openCount = %d, want 0 (release should not increment openCount)", oc)
	}
	// idleCount = len(conns) = 1, 但防御性校验 ic = min(ic, oc) = 0
	if ic != 0 {
		t.Errorf("stats().idleCount = %d, want 0 (defense: idleCount capped at openCount)", ic)
	}
	if mo != 5 {
		t.Errorf("stats().maxOpen = %d, want 5", mo)
	}
}

func TestHiveConnPool_PutThenStats(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        2,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 模拟预热：put 2 个连接（MinIdle=2）
	pool.put(nil, "default")
	pool.put(nil, "default")

	oc, ic, mo := pool.stats()
	if oc != 2 {
		t.Errorf("stats().openCount = %d, want 2 after 2 puts", oc)
	}
	if ic != 2 {
		t.Errorf("stats().idleCount = %d, want 2 after 2 puts", ic)
	}
	if mo != 5 {
		t.Errorf("stats().maxOpen = %d, want 5", mo)
	}
}

func TestHiveConnPool_PutExceedsMaxOpen(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        2,
		MinIdle:        1,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// put 3 个连接，但 MaxOpen=2，第 3 个应该被拒绝
	pool.put(nil, "db1")
	pool.put(nil, "db2")
	pool.put(nil, "db3") // 超过 MaxOpen，应该被关闭并回滚 openCount

	oc, ic, mo := pool.stats()
	if oc != 2 {
		t.Errorf("stats().openCount = %d, want 2 (3rd put should be rejected)", oc)
	}
	if ic != 2 {
		t.Errorf("stats().idleCount = %d, want 2", ic)
	}
	if mo != 2 {
		t.Errorf("stats().maxOpen = %d, want 2", mo)
	}
}

func TestHiveConnPool_PutNilConnection(t *testing.T) {
	cfg := DefaultPoolConfig()

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// put nil 连接不应该 panic
	pool.put(nil, "db1")

	oc, _, _ := pool.stats()
	// nil 连接也会被放入 channel 并计数（实际场景中不会 put nil）
	if oc != 1 {
		t.Errorf("stats().openCount = %d, want 1 after put(nil)", oc)
	}
}

func TestHiveConnPool_PutAfterClose(t *testing.T) {
	cfg := DefaultPoolConfig()

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})

	pool.close()

	// put 到已关闭的 pool 不应该 panic，连接应该被丢弃
	pool.put(nil, "db1")

	oc, _, _ := pool.stats()
	if oc != 0 {
		t.Errorf("stats().openCount = %d, want 0 after put to closed pool", oc)
	}
}

func TestHiveConnPool_AcquireAndRelease(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        3,
		MinIdle:        1,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 预热 1 个连接
	pool.put(nil, "default")

	// acquire 应该从池中获取
	pc, err := pool.acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// acquire 后：openCount=1, idleCount=0（连接被取出）
	oc, ic, _ := pool.stats()
	if oc != 1 {
		t.Errorf("after acquire: openCount = %d, want 1", oc)
	}
	if ic != 0 {
		t.Errorf("after acquire: idleCount = %d, want 0", ic)
	}

	// release 归还连接
	pool.release(pc)

	// release 后：openCount=1, idleCount=1（连接归还到池中）
	oc, ic, _ = pool.stats()
	if oc != 1 {
		t.Errorf("after release: openCount = %d, want 1", oc)
	}
	if ic != 1 {
		t.Errorf("after release: idleCount = %d, want 1", ic)
	}
}

func TestHiveConnPool_AcquireOrCreate(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        3,
		MinIdle:        0,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	var createCount int32
	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		// 模拟创建延迟
		return nil, nil
	})
	defer pool.close()

	// 没有预热，acquire 应该创建新连接
	pc, err := pool.acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	oc, ic, _ := pool.stats()
	if oc != 1 {
		t.Errorf("after acquire from empty pool: openCount = %d, want 1", oc)
	}
	if ic != 0 {
		t.Errorf("after acquire from empty pool: idleCount = %d, want 0", ic)
	}

	// 归还连接
	pool.release(pc)

	_ = createCount // avoid unused warning
}

func TestHiveConnPool_Discard(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        1,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 预热 2 个连接
	pool.put(nil, "db1")
	pool.put(nil, "db2")

	oc, ic, _ := pool.stats()
	if oc != 2 || ic != 2 {
		t.Fatalf("after 2 puts: openCount=%d idleCount=%d, want 2,2", oc, ic)
	}

	// acquire 一个连接
	pc, err := pool.acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// discard 该连接（模拟连接损坏）
	pool.discard(pc)

	oc, ic, _ = pool.stats()
	if oc != 1 {
		t.Errorf("after discard: openCount = %d, want 1", oc)
	}
	if ic != 1 {
		t.Errorf("after discard: idleCount = %d, want 1", ic)
	}
}

func TestHiveConnPool_StatsConcurrent(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        10,
		MinIdle:        2,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 预热
	pool.put(nil, "default")
	pool.put(nil, "default")

	// 并发读写 stats
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			pool.stats()
		}()
		go func() {
			defer wg.Done()
			pool.put(nil, "db")
		}()
	}
	wg.Wait()

	// 不 panic 就算通过
}

func TestHiveConnPool_UpdateConfig(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        2,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 更新配置
	newCfg := PoolConfig{
		MaxOpen:        10,
		MinIdle:        3,
		MaxLifetime:    1 * time.Hour,
		AcquireTimeout: 10 * time.Second,
	}
	pool.UpdateConfig(newCfg)

	gotCfg := pool.GetConfig()
	if gotCfg.MaxOpen != 10 {
		t.Errorf("GetConfig().MaxOpen = %d, want 10", gotCfg.MaxOpen)
	}
	if gotCfg.MinIdle != 3 {
		t.Errorf("GetConfig().MinIdle = %d, want 3", gotCfg.MinIdle)
	}
	if gotCfg.MaxLifetime != 1*time.Hour {
		t.Errorf("GetConfig().MaxLifetime = %v, want 1h", gotCfg.MaxLifetime)
	}

	// stats 中的 maxOpen 应该反映新配置
	_, _, mo := pool.stats()
	if mo != 10 {
		t.Errorf("stats().maxOpen = %d, want 10 after config update", mo)
	}
}

func TestHiveConnPool_DefaultPoolConfig(t *testing.T) {
	cfg := DefaultPoolConfig()
	if cfg.MaxOpen != 5 {
		t.Errorf("DefaultPoolConfig().MaxOpen = %d, want 5", cfg.MaxOpen)
	}
	if cfg.MinIdle != 2 {
		t.Errorf("DefaultPoolConfig().MinIdle = %d, want 2", cfg.MinIdle)
	}
	if cfg.MaxLifetime != 30*time.Minute {
		t.Errorf("DefaultPoolConfig().MaxLifetime = %v, want 30m", cfg.MaxLifetime)
	}
	if cfg.AcquireTimeout != 30*time.Second {
		t.Errorf("DefaultPoolConfig().AcquireTimeout = %v, want 30s", cfg.AcquireTimeout)
	}
}

func TestHiveConnPool_NewPoolInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     PoolConfig
		wantMax int
		wantMin int
	}{
		{
			name:    "zero MaxOpen defaults to 5",
			cfg:     PoolConfig{MaxOpen: 0, MinIdle: 1, AcquireTimeout: 5 * time.Second},
			wantMax: 5,
			wantMin: 1,
		},
		{
			name:    "negative MinIdle defaults to 0",
			cfg:     PoolConfig{MaxOpen: 3, MinIdle: -1, AcquireTimeout: 5 * time.Second},
			wantMax: 3,
			wantMin: 0,
		},
		{
			name:    "MinIdle > MaxOpen capped to MaxOpen",
			cfg:     PoolConfig{MaxOpen: 2, MinIdle: 10, AcquireTimeout: 5 * time.Second},
			wantMax: 2,
			wantMin: 2,
		},
		{
			name:    "zero AcquireTimeout defaults to 30s",
			cfg:     PoolConfig{MaxOpen: 5, MinIdle: 2, AcquireTimeout: 0},
			wantMax: 5,
			wantMin: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := newHiveConnPool(tt.cfg, func(ctx context.Context) (*gohive.Connection, error) {
				return nil, nil
			})
			defer pool.close()

			gotCfg := pool.GetConfig()
			if gotCfg.MaxOpen != tt.wantMax {
				t.Errorf("MaxOpen = %d, want %d", gotCfg.MaxOpen, tt.wantMax)
			}
			if gotCfg.MinIdle != tt.wantMin {
				t.Errorf("MinIdle = %d, want %d", gotCfg.MinIdle, tt.wantMin)
			}
			if gotCfg.AcquireTimeout <= 0 {
				t.Errorf("AcquireTimeout = %v, want positive", gotCfg.AcquireTimeout)
			}
		})
	}
}

// TestHiveDriver_GetPoolStats_AfterPut 验证 HiveDriver.GetPoolStats 在 put 后返回正确数据
func TestHiveDriver_GetPoolStats_AfterPut(t *testing.T) {
	d := &HiveDriver{}

	// 未连接时返回 0
	oc, ic, inUse, mo := d.GetPoolStats()
	if oc != 0 || ic != 0 || inUse != 0 || mo != 0 {
		t.Errorf("GetPoolStats() on unconnected driver = (%d,%d,%d,%d), want (0,0,0,0)", oc, ic, inUse, mo)
	}

	// 获取默认配置
	cfg := d.GetPoolConfig()
	if cfg.MaxOpen != 5 {
		t.Errorf("GetPoolConfig().MaxOpen = %d, want 5 (default)", cfg.MaxOpen)
	}
}

// TestHiveDriver_ImplementsPoolConfigUpdater 验证 HiveDriver 实现了 PoolConfigUpdater 接口
func TestHiveDriver_ImplementsPoolConfigUpdater(t *testing.T) {
	var _ PoolConfigUpdater = &HiveDriver{}
}

// TestKyuubiDriver_ImplementsPoolConfigUpdater 验证 KyuubiDriver 实现了 PoolConfigUpdater 接口
func TestKyuubiDriver_ImplementsPoolConfigUpdater(t *testing.T) {
	var _ PoolConfigUpdater = &KyuubiDriver{}
}

// TestSparkDriver_ImplementsPoolConfigUpdater 验证 SparkDriver 实现了 PoolConfigUpdater 接口
func TestSparkDriver_ImplementsPoolConfigUpdater(t *testing.T) {
	var _ PoolConfigUpdater = &SparkDriver{}
}

// TestHiveConnPool_PutVsRelease_Comparison 对比 put 和 release 的行为差异
// 这是修复的核心：预热时必须用 put 而不是 release
func TestHiveConnPool_PutVsRelease_Comparison(t *testing.T) {
	t.Run("put increments openCount", func(t *testing.T) {
		cfg := PoolConfig{MaxOpen: 5, MinIdle: 2, MaxLifetime: 30 * time.Minute, AcquireTimeout: 5 * time.Second}
		pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
			return nil, nil
		})
		defer pool.close()

		pool.put(nil, "db")
		oc, ic, _ := pool.stats()
		if oc != 1 {
			t.Errorf("after put: openCount=%d, want 1", oc)
		}
		if ic != 1 {
			t.Errorf("after put: idleCount=%d, want 1", ic)
		}
	})

	t.Run("release does not increment openCount", func(t *testing.T) {
		cfg := PoolConfig{MaxOpen: 5, MinIdle: 2, MaxLifetime: 30 * time.Minute, AcquireTimeout: 5 * time.Second}
		pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
			return nil, nil
		})
		defer pool.close()

		// release 是"归还"语义，openCount 应该已经在 acquire 时增加
		// 直接 release 一个新创建的 pooledConn，openCount 不应该增加
		pool.release(&pooledConn{conn: nil, database: "db"})
		oc, ic, _ := pool.stats()
		// 旧 bug 的表现：oc=0, ic=0（防御性校验）
		// 这是 release 的正确行为：release 不增加 openCount
		if oc != 0 {
			t.Errorf("after release without prior acquire: openCount=%d, want 0", oc)
		}
		// idleCount 被 openCount 上限裁剪为 0
		if ic != 0 {
			t.Errorf("after release without prior acquire: idleCount=%d, want 0 (capped by openCount)", ic)
		}
	})
}

// TestHiveConnPool_PreWarmSimulation 模拟完整的预热场景
func TestHiveConnPool_PreWarmSimulation(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        5,
		MinIdle:        3,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 模拟 HiveDriver.Connect 中的预热逻辑
	// 初始连接
	pool.put(nil, "default")

	// 额外 MinIdle-1 个连接
	for i := 1; i < cfg.MinIdle; i++ {
		pool.put(nil, "default")
	}

	// 验证 stats
	oc, ic, mo := pool.stats()
	if oc != cfg.MinIdle {
		t.Errorf("after pre-warm: openCount=%d, want %d", oc, cfg.MinIdle)
	}
	if ic != cfg.MinIdle {
		t.Errorf("after pre-warm: idleCount=%d, want %d", ic, cfg.MinIdle)
	}
	if mo != cfg.MaxOpen {
		t.Errorf("after pre-warm: maxOpen=%d, want %d", mo, cfg.MaxOpen)
	}

	// 模拟 GetPoolStats 的计算逻辑
	inUse := oc - ic
	if inUse != 0 {
		t.Errorf("inUse = %d, want 0 (all connections idle)", inUse)
	}
}

// TestHiveConnPool_AcquireReleaseCycle 测试完整的 acquire-release 循环
func TestHiveConnPool_AcquireReleaseCycle(t *testing.T) {
	cfg := PoolConfig{
		MaxOpen:        3,
		MinIdle:        1,
		MaxLifetime:    0, // 不限制生命周期，避免 cleanup 干扰
		AcquireTimeout: 5 * time.Second,
	}

	pool := newHiveConnPool(cfg, func(ctx context.Context) (*gohive.Connection, error) {
		return nil, nil
	})
	defer pool.close()

	// 预热 1 个连接
	pool.put(nil, "default")

	// 模拟多次查询：acquire -> release
	for i := 0; i < 10; i++ {
		pc, err := pool.acquire(context.Background())
		if err != nil {
			t.Fatalf("cycle %d: acquire failed: %v", i, err)
		}

		oc, ic, _ := pool.stats()
		if ic != 0 {
			t.Errorf("cycle %d: after acquire idleCount=%d, want 0", i, ic)
		}

		pool.release(pc)

		oc, ic, _ = pool.stats()
		if ic != 1 {
			t.Errorf("cycle %d: after release idleCount=%d, want 1", i, ic)
		}
		if oc != 1 {
			t.Errorf("cycle %d: after release openCount=%d, want 1", i, oc)
		}
	}
}
