package driver

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	gohive "github.com/beltran/gohive"
)

// PoolConfig 连接池配置，可通过系统设置动态调整
type PoolConfig struct {
	MaxOpen       int           // 最大连接数
	MinIdle       int           // 最小空闲连接数（常驻连接）
	MaxLifetime   time.Duration // 连接最大生命周期，0 表示不限制
	AcquireTimeout time.Duration // 获取连接超时时间
}

// DefaultPoolConfig 返回默认连接池配置
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpen:        5,
		MinIdle:        2,
		MaxLifetime:    30 * time.Minute,
		AcquireTimeout: 30 * time.Second,
	}
}

// hiveConnPool 管理 Hive 连接池，支持多用户并发查询。
// 每个查询从池中获取独立连接，设置 database context 后执行，完成后归还。
// 参考 Superset 的 SQLAlchemy 连接池设计。
type hiveConnPool struct {
	conns       chan *pooledConn
	config      atomic.Value // 存储 PoolConfig
	openCount   atomic.Int32
	createConn  func(ctx context.Context) (*gohive.Connection, error)
	mu          sync.Mutex
	closed      bool
	closeMu     sync.Mutex
	createTime  sync.Map // conn指针 -> 创建时间，用于 MaxLifetime 检查
	cleanupOnce sync.Once
	stopCleanup chan struct{}
}

type pooledConn struct {
	conn     *gohive.Connection
	database string // 当前连接的 database context
}

func newHiveConnPool(cfg PoolConfig, createFn func(ctx context.Context) (*gohive.Connection, error)) *hiveConnPool {
	if cfg.MaxOpen <= 0 {
		cfg.MaxOpen = 5
	}
	if cfg.MinIdle < 0 {
		cfg.MinIdle = 0
	}
	if cfg.MinIdle > cfg.MaxOpen {
		cfg.MinIdle = cfg.MaxOpen
	}
	if cfg.AcquireTimeout <= 0 {
		cfg.AcquireTimeout = 30 * time.Second
	}

	p := &hiveConnPool{
		conns:       make(chan *pooledConn, cfg.MaxOpen),
		createConn:  createFn,
		stopCleanup: make(chan struct{}),
	}
	p.config.Store(cfg)

	// 启动后台清理协程
	p.cleanupOnce.Do(func() {
		go p.cleanupLoop()
	})

	return p
}

// UpdateConfig 动态更新连接池配置
func (p *hiveConnPool) UpdateConfig(cfg PoolConfig) {
	if cfg.MaxOpen <= 0 {
		cfg.MaxOpen = 5
	}
	if cfg.MinIdle < 0 {
		cfg.MinIdle = 0
	}
	if cfg.AcquireTimeout <= 0 {
		cfg.AcquireTimeout = 30 * time.Second
	}

	oldCfg := p.config.Load().(PoolConfig)
	p.config.Store(cfg)

	if oldCfg.MaxOpen != cfg.MaxOpen {
		slog.Info("hive pool max_open updated", "old", oldCfg.MaxOpen, "new", cfg.MaxOpen)
	}
	if oldCfg.MinIdle != cfg.MinIdle {
		slog.Info("hive pool min_idle updated", "old", oldCfg.MinIdle, "new", cfg.MinIdle)
	}
}

// GetConfig 获取当前连接池配置
func (p *hiveConnPool) GetConfig() PoolConfig {
	return p.config.Load().(PoolConfig)
}

// acquire 从池中获取连接，如果池为空且未达到上限则创建新连接。
// 如果池为空且已达上限，则阻塞等待直到有连接归还或 context 取消。
func (p *hiveConnPool) acquire(ctx context.Context) (*pooledConn, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	cfg := p.config.Load().(PoolConfig)

	// 优先从池中获取
	select {
	case pc := <-p.conns:
		// 检查连接是否超过最大生命周期
		if cfg.MaxLifetime > 0 {
			if createTime, ok := p.createTime.Load(pc.conn); ok {
				if time.Since(createTime.(time.Time)) > cfg.MaxLifetime {
					slog.Debug("hive pooled connection exceeded max lifetime, discarding", "max_lifetime", cfg.MaxLifetime)
					p.discard(pc)
					// 继续尝试获取新连接
					return p.acquireOrCreate(ctx, cfg)
				}
			}
		}
		return pc, nil
	default:
	}

	return p.acquireOrCreate(ctx, cfg)
}

func (p *hiveConnPool) acquireOrCreate(ctx context.Context, cfg PoolConfig) (*pooledConn, error) {
	// 池为空，尝试创建新连接
	currentOpen := p.openCount.Load()
	if currentOpen < int32(cfg.MaxOpen) {
		if p.openCount.CompareAndSwap(currentOpen, currentOpen+1) {
			conn, err := p.createConn(ctx)
			if err != nil {
				p.openCount.Add(-1)
				return nil, err
			}
			p.createTime.Store(conn, time.Now())
			return &pooledConn{conn: conn}, nil
		}
	}

	// 已达上限，等待连接归还
	acquireCtx, cancel := context.WithTimeout(ctx, cfg.AcquireTimeout)
	defer cancel()

	select {
	case pc := <-p.conns:
		// 检查连接是否超过最大生命周期
		if cfg.MaxLifetime > 0 {
			if createTime, ok := p.createTime.Load(pc.conn); ok {
				if time.Since(createTime.(time.Time)) > cfg.MaxLifetime {
					p.discard(pc)
					// 递归尝试获取新连接
					return p.acquireOrCreate(ctx, cfg)
				}
			}
		}
		return pc, nil
	case <-acquireCtx.Done():
		return nil, acquireCtx.Err()
	}
}

// release 将连接归还到池中。
func (p *hiveConnPool) release(pc *pooledConn) {
	if pc == nil || pc.conn == nil {
		return
	}

	p.closeMu.Lock()
	defer p.closeMu.Unlock()
	if p.closed {
		pc.conn.Close()
		p.openCount.Add(-1)
		p.createTime.Delete(pc.conn)
		return
	}

	// 检查连接是否超过最大生命周期
	cfg := p.config.Load().(PoolConfig)
	if cfg.MaxLifetime > 0 {
		if createTime, ok := p.createTime.Load(pc.conn); ok {
			if time.Since(createTime.(time.Time)) > cfg.MaxLifetime {
				slog.Debug("hive pooled connection exceeded max lifetime on release, discarding")
				pc.conn.Close()
				p.openCount.Add(-1)
				p.createTime.Delete(pc.conn)
				return
			}
		}
	}

	select {
	case p.conns <- pc:
	default:
		// 池已满，关闭多余连接
		pc.conn.Close()
		p.openCount.Add(-1)
		p.createTime.Delete(pc.conn)
	}
}

// discard 丢弃损坏的连接。
func (p *hiveConnPool) discard(pc *pooledConn) {
	if pc == nil {
		return
	}
	if pc.conn != nil {
		pc.conn.Close()
		p.createTime.Delete(pc.conn)
	}
	p.openCount.Add(-1)
}

// close 关闭池中所有连接。
func (p *hiveConnPool) close() {
	p.closeMu.Lock()
	p.closed = true
	p.closeMu.Unlock()

	// 停止清理协程
	close(p.stopCleanup)

	// 排空池中连接
	for {
		select {
		case pc := <-p.conns:
			if pc.conn != nil {
				pc.conn.Close()
				p.createTime.Delete(pc.conn)
			}
			p.openCount.Add(-1)
		default:
			return
		}
	}
}

// stats 返回连接池统计信息（用于调试和监控）。
func (p *hiveConnPool) stats() (openCount int, idleCount int, maxOpen int) {
	cfg := p.config.Load().(PoolConfig)
	return int(p.openCount.Load()), len(p.conns), cfg.MaxOpen
}

// cleanupLoop 后台清理协程，定期：
// 1. 回收超过 MaxLifetime 的空闲连接
// 2. 确保 MinIdle 个常驻连接
func (p *hiveConnPool) cleanupLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.doCleanup()
		case <-p.stopCleanup:
			return
		}
	}
}

func (p *hiveConnPool) doCleanup() {
	p.closeMu.Lock()
	if p.closed {
		p.closeMu.Unlock()
		return
	}
	p.closeMu.Unlock()

	cfg := p.config.Load().(PoolConfig)

	// 1. 回收超过 MaxLifetime 的空闲连接
	var validConns []*pooledConn
	for {
		select {
		case pc := <-p.conns:
			if cfg.MaxLifetime > 0 {
				if createTime, ok := p.createTime.Load(pc.conn); ok {
					if time.Since(createTime.(time.Time)) > cfg.MaxLifetime {
						slog.Debug("hive cleanup: discarding expired connection", "max_lifetime", cfg.MaxLifetime)
						pc.conn.Close()
						p.openCount.Add(-1)
						p.createTime.Delete(pc.conn)
						continue
					}
				}
			}
			validConns = append(validConns, pc)
		default:
			goto requeue
		}
	}

requeue:
	// 将有效连接放回池中
	for _, pc := range validConns {
		select {
		case p.conns <- pc:
		default:
			pc.conn.Close()
			p.openCount.Add(-1)
			p.createTime.Delete(pc.conn)
		}
	}

	// 2. 确保 MinIdle 个常驻连接
	currentIdle := len(p.conns)
	currentOpen := int(p.openCount.Load())

	if currentIdle < cfg.MinIdle && currentOpen < cfg.MaxOpen {
		need := cfg.MinIdle - currentIdle
		if need > cfg.MaxOpen-currentOpen {
			need = cfg.MaxOpen - currentOpen
		}
		for i := 0; i < need; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			conn, err := p.createConn(ctx)
			cancel()
			if err != nil {
				slog.Warn("hive cleanup: failed to create min_idle connection", "error", err)
				continue
			}
			p.createTime.Store(conn, time.Now())
			pc := &pooledConn{conn: conn}
			select {
			case p.conns <- pc:
				p.openCount.Add(1)
			default:
				conn.Close()
			}
		}
	}

	open, idle, maxOpen := p.stats()
	slog.Debug("hive pool cleanup done", "open", open, "idle", idle, "max_open", maxOpen, "min_idle", cfg.MinIdle)
}

// ensureDatabase 确保连接处于指定的 database context。
// 如果连接已经在目标 database，则跳过 USE 语句。
func (pc *pooledConn) ensureDatabase(ctx context.Context, database string) error {
	if database == "" || pc.database == database {
		return nil
	}

	cursor := pc.conn.Cursor()
	cursor.Exec(context.Background(), "USE "+escapeHiveIdentifier(database))
	if cursor.Err != nil {
		err := cursor.Err
		cursor.Close()
		return extractGohiveError(err, "hive switch database error")
	}
	cursor.Close()
	pc.database = database
	return nil
}

// setQueryTimeout 在连接上设置服务端查询超时，作为 Go context 超时的双重保障。
// timeoutSQL 格式如 "SET hive.execution.engine.query.timeout=60"。
// 使用 context.Background() 执行 SET 语句，避免用户 context 已取消导致设置失败。
// 如果 SET 语句执行失败，仅记录警告，不阻断查询（尽力而为的保障）。
func (pc *pooledConn) setQueryTimeout(ctx context.Context, timeoutSQL string) {
	if timeoutSQL == "" {
		return
	}

	cursor := pc.conn.Cursor()
	cursor.Exec(context.Background(), timeoutSQL)
	if cursor.Err != nil {
		slog.Warn("failed to set server-side query timeout, continuing without it", "sql", timeoutSQL, "error", cursor.Err)
		cursor.Close()
		return
	}
	cursor.Close()

	deadline, ok := ctx.Deadline()
	if ok {
		slog.Debug("server-side query timeout set", "sql", timeoutSQL, "context_deadline", time.Until(deadline).Round(time.Second))
	}
}

// extractQueryTimeout 从 context 的 deadline 中提取超时秒数，并加上 buffer 秒的缓冲。
// 如果 context 没有 deadline，返回 0（表示不设置服务端超时，仅依赖 Go context）。
// 返回的超时值（秒）和对应的 SET SQL 语句。
func extractQueryTimeout(ctx context.Context, setPrefix string, buffer time.Duration) (int, string) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0, ""
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0, ""
	}
	// 加上 buffer 避免竞态：服务端超时应略晚于 Go context 超时
	timeoutSec := int((remaining + buffer).Seconds())
	if timeoutSec <= 0 {
		return 0, ""
	}
	return timeoutSec, fmt.Sprintf("%s%d", setPrefix, timeoutSec)
}

// ping 检查连接是否存活。
func (pc *pooledConn) ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cursor := pc.conn.Cursor()
	cursor.Exec(pingCtx, "SELECT 1")
	if cursor.Err != nil {
		err := cursor.Err
		cursor.Close()
		return err
	}
	cursor.Close()
	return nil
}

// acquireWithTimeout 带超时的获取连接，并验证连接可用性。
func (p *hiveConnPool) acquireWithTimeout(ctx context.Context, timeout time.Duration) (*pooledConn, error) {
	acquireCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pc, err := p.acquire(acquireCtx)
	if err != nil {
		return nil, err
	}

	// 验证连接可用性
	if err := pc.ping(ctx); err != nil {
		slog.Warn("hive pooled connection ping failed, discarding", "error", err)
		p.discard(pc)
		// 尝试获取新连接
		pc, err = p.acquire(ctx)
		if err != nil {
			return nil, err
		}
	}

	return pc, nil
}
