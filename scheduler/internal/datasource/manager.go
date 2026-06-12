package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

type Manager struct {
	pools      map[int64]driver.Driver
	poolMu     sync.RWMutex
	crypto     *Crypto
	config     *ConfigService
	closed     bool
	closeMu    sync.Mutex
	lastCheck  map[int64]time.Time
	checkMu    sync.Mutex
	connecting map[int64]struct{}
	connectMu  sync.Mutex
}

func NewManager(crypto *Crypto, config *ConfigService) *Manager {
	return &Manager{
		pools:      make(map[int64]driver.Driver),
		crypto:     crypto,
		config:     config,
		lastCheck:  make(map[int64]time.Time),
		connecting: make(map[int64]struct{}),
	}
}

func (m *Manager) GetDriver(ctx context.Context, ds *model.Datasource) (driver.Driver, error) {
	if !ds.IsEnabled {
		return nil, ErrDatasourceDisabled
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
				return d, nil
			}

			pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
			defer pingCancel()
			if err := d.Ping(pingCtx); err == nil {
				m.checkMu.Lock()
				m.lastCheck[ds.ID] = time.Now()
				m.checkMu.Unlock()
				return d, nil
			}
			slog.Info("datasource connection stale, reconnecting", "datasource_id", ds.ID, "type", ds.Type, "name", ds.Name)
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

	return drv, err
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

	testTimeout := m.config.GetInt("datasource.test_timeout")
	if testTimeout <= 0 {
		testTimeout = 30
	}
	connectCtx, cancel := context.WithTimeout(ctx, time.Duration(testTimeout)*time.Second)
	defer cancel()

	if err := drv.Connect(connectCtx, dsConfig); err != nil {
		slog.Error("failed to connect datasource", "datasource_id", ds.ID, "type", ds.Type, "host", ds.Host, "port", ds.Port, "error", err)
		return nil, fmt.Errorf("%w: %v", ErrDatasourceConnFailed, err)
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

	testTimeout := m.config.GetInt("datasource.test_timeout")
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

	if maxOpen := m.config.GetInt("datasource.connection_max_open"); maxOpen > 0 {
		cfg.MaxOpen = maxOpen
	}
	if minIdle := m.config.GetInt("datasource.connection_max_idle"); minIdle > 0 {
		cfg.MinIdle = minIdle
	}
	if maxLifetime := m.config.GetInt("datasource.connection_max_lifetime"); maxLifetime > 0 {
		cfg.MaxLifetime = time.Duration(maxLifetime) * time.Second
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
