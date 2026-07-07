package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
	sysconfig "github.com/lynnyq/bdopsflow/scheduler/internal/system_config"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

// Manager 数据源连接池管理器
// 负责管理所有数据源的连接池、健康检查、熔断器和配置热更新。
// 线程安全，支持并发访问。
type Manager struct {
	pools      map[int64]driver.Driver // 数据源 ID -> 驱动实例
	poolMu     sync.RWMutex
	crypto     *Crypto                 // 密码加解密
	sysConfig  *sysconfig.Service      // 系统配置服务（运行时动态配置，支持热更新）
	closed     bool
	closeMu    sync.Mutex
	lastCheck  map[int64]time.Time     // 数据源 ID -> 上次健康检查时间
	checkMu    sync.Mutex
	connecting map[int64]struct{}      // 正在连接中的数据源（防重复连接）
	connectMu  sync.Mutex

	// 熔断器映射，每个数据源独立熔断器
	circuitBreakers map[int64]*CircuitBreaker
	cbMu            sync.RWMutex
}

// NewManager 创建数据源连接池管理器
// crypto: 密码加解密服务，可为 nil（不加密场景）
// sysConfig: 系统配置服务，读取运行时动态配置（支持热更新）
func NewManager(crypto *Crypto, sysConfig *sysconfig.Service) *Manager {
	return &Manager{
		pools:           make(map[int64]driver.Driver),
		crypto:          crypto,
		sysConfig:       sysConfig,
		lastCheck:       make(map[int64]time.Time),
		connecting:      make(map[int64]struct{}),
		circuitBreakers: make(map[int64]*CircuitBreaker),
	}
}

// GetDriver 获取指定数据源的驱动实例
// 该方法实现了连接池复用、健康检查、熔断器保护等功能。
// 如果数据源已禁用，返回 ErrDatasourceDisabled
// 如果熔断器开启，返回 ErrDatasourceCircuitOpen
// 如果连接不存在或不健康，会自动创建新连接
func (m *Manager) GetDriver(ctx context.Context, ds *model.Datasource) (driver.Driver, error) {
	if !ds.IsEnabled {
		return nil, ErrDatasourceDisabled
	}

	// 检查熔断器状态
	cb := m.getCircuitBreaker(ds.ID)
	if !cb.AllowRequest() {
		slog.Warn("datasource circuit breaker is open, rejecting request",
			"datasource_id", ds.ID,
			"type", ds.Type,
			"name", ds.Name,
			"failure_count", cb.GetFailureCount())
		return nil, ErrDatasourceCircuitOpen
	}

	m.poolMu.RLock()
	d, ok := m.pools[ds.ID]
	m.poolMu.RUnlock()

	if ok {
		if hc, ok := d.(driver.UnhealthyChecker); ok && hc.IsUnhealthy() {
			slog.Info("datasource marked as unhealthy, reconnecting", "datasource_id", ds.ID, "type", ds.Type, "name", ds.Name)
			d.Close()
			m.poolMu.Lock()
			delete(m.pools, ds.ID)
			m.poolMu.Unlock()
			m.checkMu.Lock()
			delete(m.lastCheck, ds.ID)
			m.checkMu.Unlock()
		} else {
			m.checkMu.Lock()
			last := m.lastCheck[ds.ID]
			m.checkMu.Unlock()

			checkInterval := 30 * time.Second
			if time.Since(last) < checkInterval {
				cb.RecordSuccess() // 记录成功
				return d, nil
			}

			// 非阻塞健康检查：使用独立的短超时 context，避免 Ping 阻塞在连接池获取上
			// 当连接池被慢查询占满时，Ping 的 acquire 也会阻塞，导致 GetDriver 卡死
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*time.Second)
			pingErr := d.Ping(pingCtx)
			pingCancel()

			if pingErr == nil {
				m.checkMu.Lock()
				m.lastCheck[ds.ID] = time.Now()
				m.checkMu.Unlock()
				cb.RecordSuccess() // 记录成功
				return d, nil
			}

			// 区分"连接池暂时繁忙"和"连接真正断开"
			// 连接池繁忙时不应重建连接，直接返回现有驱动即可
			if isPoolBusyError(pingErr) {
				slog.Debug("datasource connection pool busy, skipping health check", "datasource_id", ds.ID, "type", ds.Type, "name", ds.Name)
				m.checkMu.Lock()
				m.lastCheck[ds.ID] = time.Now()
				m.checkMu.Unlock()
				return d, nil
			}

			slog.Info("datasource connection stale, reconnecting", "datasource_id", ds.ID, "type", ds.Type, "name", ds.Name, "error", pingErr)
			d.Close()
			m.poolMu.Lock()
			delete(m.pools, ds.ID)
			m.poolMu.Unlock()
			m.checkMu.Lock()
			delete(m.lastCheck, ds.ID)
			m.checkMu.Unlock()
		}
	}

	slog.Debug("creating new datasource connection", "datasource_id", ds.ID, "type", ds.Type, "host", ds.Host, "port", ds.Port)

	m.connectMu.Lock()
	if _, ok := m.connecting[ds.ID]; ok {
		m.connectMu.Unlock()
		return nil, fmt.Errorf("datasource %d is connecting, please retry later", ds.ID)
	}
	m.connecting[ds.ID] = struct{}{}
	m.connectMu.Unlock()

	drv, err := m.connect(ctx, ds)

	m.connectMu.Lock()
	delete(m.connecting, ds.ID)
	m.connectMu.Unlock()

	if err != nil {
		cb.RecordFailure() // 记录失败
		return nil, err
	}

	cb.RecordSuccess() // 连接成功
	return drv, nil
}

func (m *Manager) connect(ctx context.Context, ds *model.Datasource) (driver.Driver, error) {
	drv, err := driver.GetDriver(ds.Type)
	if err != nil {
		return nil, ErrDatasourceTypeNotSupport
	}

	password := ds.Password
	if m.crypto != nil && password != "" && ds.ID > 0 {
		decrypted, err := m.crypto.Decrypt(password)
		if err != nil {
			slog.Warn("failed to decrypt password, trying raw", "datasource_id", ds.ID, "error", err)
		} else {
			password = decrypted
		}
	}

	dsConfig := driver.DatasourceConfig{
		Type:               ds.Type,
		Host:               ds.Host,
		Port:               ds.Port,
		Path:               ds.Path,
		Database:           ds.Database,
		Username:           ds.Username,
		Password:           password,
		AuthType:           ds.AuthType,
		ConnectionMode:     ds.ConnectionMode,
		ZookeeperQuorum:    ds.ZkHosts,
		ZookeeperNamespace: ds.ZkPath,
		RqliteHosts:        ds.RqliteHosts,
	}

	if ds.Config != "" {
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(ds.Config), &cfg); err == nil {
			dsConfig.Config = cfg
		}
	}

	testTimeout := 0
	if m.sysConfig != nil {
		testTimeout = m.sysConfig.GetInt("datasource.test_timeout")
	}
	if testTimeout <= 0 {
		testTimeout = 30
	}
	connectCtx, cancel := context.WithTimeout(ctx, time.Duration(testTimeout)*time.Second)
	defer cancel()

	if err := drv.Connect(connectCtx, dsConfig); err != nil {
		slog.Error("failed to connect datasource", "datasource_id", ds.ID, "type", ds.Type, "host", ds.Host, "port", ds.Port, "error", err)
		return nil, fmt.Errorf("%w: %v", ErrDatasourceConnFailed, err)
	}

	// 连接成功后，立即应用系统配置的连接池参数
	if updater, ok := drv.(driver.PoolConfigUpdater); ok {
		poolCfg := m.getPoolConfigFromSystemSettings()
		updater.UpdatePoolConfig(poolCfg)
		slog.Debug("applied pool config after connect", "datasource_id", ds.ID, "max_open", poolCfg.MaxOpen, "min_idle", poolCfg.MinIdle)
	}

	m.poolMu.Lock()
	m.pools[ds.ID] = drv
	m.poolMu.Unlock()

	m.checkMu.Lock()
	m.lastCheck[ds.ID] = time.Now()
	m.checkMu.Unlock()

	slog.Info("datasource connected successfully", "datasource_id", ds.ID, "type", ds.Type, "name", ds.Name, "host", ds.Host)
	return drv, nil
}

func (m *Manager) TestConnection(ctx context.Context, ds *model.Datasource) error {
	drv, err := driver.GetDriver(ds.Type)
	if err != nil {
		return ErrDatasourceTypeNotSupport
	}

	password := ds.Password
	if m.crypto != nil && password != "" && ds.ID > 0 {
		decrypted, err := m.crypto.Decrypt(password)
		if err != nil {
			slog.Warn("failed to decrypt password for test", "datasource_id", ds.ID, "error", err)
		} else {
			password = decrypted
		}
	}

	dsConfig := driver.DatasourceConfig{
		Type:               ds.Type,
		Host:               ds.Host,
		Port:               ds.Port,
		Path:               ds.Path,
		Database:           ds.Database,
		Username:           ds.Username,
		Password:           password,
		AuthType:           ds.AuthType,
		ConnectionMode:     ds.ConnectionMode,
		ZookeeperQuorum:    ds.ZkHosts,
		ZookeeperNamespace: ds.ZkPath,
		RqliteHosts:        ds.RqliteHosts,
	}

	if ds.Config != "" {
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(ds.Config), &cfg); err == nil {
			dsConfig.Config = cfg
		}
	}

	testTimeout := 0
	if m.sysConfig != nil {
		testTimeout = m.sysConfig.GetInt("datasource.test_timeout")
	}
	if testTimeout <= 0 {
		testTimeout = 10
	}
	connectCtx, cancel := context.WithTimeout(ctx, time.Duration(testTimeout)*time.Second)
	defer cancel()

	if err := drv.Connect(connectCtx, dsConfig); err != nil {
		slog.Error("failed to connect datasource for test", "datasource_id", ds.ID, "type", ds.Type, "host", ds.Host, "error", err)
		return fmt.Errorf("%w: %v", ErrDatasourceConnFailed, err)
	}
	defer drv.Close()

	slog.Debug("datasource test connection succeeded", "datasource_id", ds.ID, "type", ds.Type, "name", ds.Name)
	return drv.TestConnection(connectCtx)
}

func (m *Manager) RemoveDatasource(dsID int64) {
	m.poolMu.Lock()
	defer m.poolMu.Unlock()
	if d, ok := m.pools[dsID]; ok {
		d.Close()
		delete(m.pools, dsID)
		slog.Info("datasource connection removed from pool", "datasource_id", dsID)
	}

	// 同时清理熔断器
	m.cbMu.Lock()
	delete(m.circuitBreakers, dsID)
	m.cbMu.Unlock()
}

func (m *Manager) Close() {
	m.closeMu.Lock()
	defer m.closeMu.Unlock()
	if m.closed {
		return
	}
	m.closed = true

	m.poolMu.Lock()
	defer m.poolMu.Unlock()
	for id, d := range m.pools {
		if err := d.Close(); err != nil {
			slog.Warn("failed to close datasource connection", "id", id, "error", err)
		}
		delete(m.pools, id)
	}
}

// OnConfigChanged 实现 ConfigObserver 接口，配置变更时动态更新连接池
func (m *Manager) OnConfigChanged(key, value string) {
	switch key {
	case "datasource.connection_max_open", "datasource.connection_max_idle", "datasource.connection_max_lifetime":
		m.updatePoolConfigFromSystemSettings()
	}
}

// updatePoolConfigFromSystemSettings 从系统设置更新所有驱动的连接池配置
func (m *Manager) updatePoolConfigFromSystemSettings() {
	cfg := m.getPoolConfigFromSystemSettings()

	m.poolMu.RLock()
	defer m.poolMu.RUnlock()

	for id, drv := range m.pools {
		if updater, ok := drv.(driver.PoolConfigUpdater); ok {
			updater.UpdatePoolConfig(cfg)
			slog.Debug("updated pool config for datasource", "datasource_id", id, "max_open", cfg.MaxOpen, "min_idle", cfg.MinIdle, "max_lifetime", cfg.MaxLifetime)
		}
	}
}

// getPoolConfigFromSystemSettings 从系统设置构建连接池配置
func (m *Manager) getPoolConfigFromSystemSettings() driver.PoolConfig {
	cfg := driver.DefaultPoolConfig()

	if m.sysConfig != nil {
		if maxOpen := m.sysConfig.GetInt("datasource.connection_max_open"); maxOpen > 0 {
			cfg.MaxOpen = maxOpen
		}
		if minIdle := m.sysConfig.GetInt("datasource.connection_max_idle"); minIdle > 0 {
			cfg.MinIdle = minIdle
		}
		if maxLifetime := m.sysConfig.GetInt("datasource.connection_max_lifetime"); maxLifetime > 0 {
			cfg.MaxLifetime = time.Duration(maxLifetime) * time.Second
		}
	}

	return cfg
}

func (m *Manager) ActiveConnections() map[int64]string {
	m.poolMu.RLock()
	defer m.poolMu.RUnlock()
	result := make(map[int64]string)
	for id := range m.pools {
		result[id] = "active"
	}
	return result
}

// getCircuitBreaker 获取或创建熔断器
func (m *Manager) getCircuitBreaker(dsID int64) *CircuitBreaker {
	m.cbMu.RLock()
	cb, ok := m.circuitBreakers[dsID]
	m.cbMu.RUnlock()

	if ok {
		return cb
	}

	m.cbMu.Lock()
	defer m.cbMu.Unlock()

	// 双重检查
	if cb, ok := m.circuitBreakers[dsID]; ok {
		return cb
	}

	cb = NewCircuitBreaker(dsID)
	m.circuitBreakers[dsID] = cb
	return cb
}

// GetCircuitBreakerState 获取熔断器状态（用于监控）
func (m *Manager) GetCircuitBreakerState(dsID int64) (CircuitState, int) {
	m.cbMu.RLock()
	cb, ok := m.circuitBreakers[dsID]
	m.cbMu.RUnlock()

	if !ok {
		return CircuitStateClosed, 0
	}

	return cb.GetState(), cb.GetFailureCount()
}

// StartHealthCheck 启动定期健康检查
func (m *Manager) StartHealthCheck(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			m.checkAllConnections()
		}
	}()
	slog.Info("datasource health check started", "interval", interval)
}

// checkAllConnections 检查所有连接的健康状态
func (m *Manager) checkAllConnections() {
	m.poolMu.RLock()
	dsIDs := make([]int64, 0, len(m.pools))
	for id := range m.pools {
		dsIDs = append(dsIDs, id)
	}
	m.poolMu.RUnlock()

	for _, dsID := range dsIDs {
		m.poolMu.RLock()
		drv, ok := m.pools[dsID]
		m.poolMu.RUnlock()

		if !ok {
			continue
		}

		cb := m.getCircuitBreaker(dsID)

		// 后台健康检查：使用 TestConnection 做真正的连接验证（执行 SELECT 1）
		// Ping 仅做轻量级统计检查，无法检测 Hive 重启后的死连接
		// TestConnection 在后台运行，允许短时间阻塞，不影响前端请求
		// 当连接池满时跳过，避免长时间等待连接释放
		var err error
		if pu, ok := drv.(driver.PoolConfigUpdater); ok {
			_, _, inUse, maxOpen := pu.GetPoolStats()
			if inUse >= maxOpen {
				// 连接池满，跳过本次检查，下次再验证
				cb.RecordSuccess()
				continue
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = drv.TestConnection(ctx)
		cancel()

		if err != nil {
			slog.Warn("datasource health check failed",
				"datasource_id", dsID,
				"error", err)
			cb.RecordFailure()

			// 连接验证失败，标记为不健康，下次 GetDriver 会自动重建连接
			if hc, ok := drv.(driver.UnhealthyChecker); ok && !hc.IsUnhealthy() {
				hc.MarkUnhealthy()
				slog.Info("datasource marked unhealthy by health check, will reconnect on next request",
					"datasource_id", dsID)
			}
		} else {
			cb.RecordSuccess()
		}
	}
}

// isPoolBusyError 判断错误是否为连接池暂时繁忙（非连接断开）
// 连接池繁忙时不应触发重建连接，只需等待连接释放即可
func isPoolBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "pool fully occupied")
}
