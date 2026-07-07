package driver

import (
	"context"
	"testing"
	"time"
)

// TestTrinoDriver_ConnectLazy_then_UseDatabase_SinglePart 测试 Trino UseDatabase 单部分数据库名
// 覆盖 trino.go:242-250 的 config.Database 分支
func TestTrinoDriver_ConnectLazy_then_UseDatabase_SinglePart(t *testing.T) {
	d := &TrinoDriver{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := d.Connect(ctx, DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "test",
		Database: "default",
	})

	if err != nil {
		t.Skipf("Trino Connect 失败: %v", err)
	}
	defer d.Close()

	// UseDatabase 单部分名称，config.Database 已设置
	err = d.UseDatabase(context.Background(), "myschema")
	_ = err
}

// TestTrinoDriver_ConnectLazy_then_UseDatabase_NoConfig 测试 Trino UseDatabase 单部分名且无 config.Database
// 覆盖 trino.go:251-255 的 else 分支
func TestTrinoDriver_ConnectLazy_then_UseDatabase_NoConfig(t *testing.T) {
	d := &TrinoDriver{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := d.Connect(ctx, DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "test",
	})

	if err != nil {
		t.Skipf("Trino Connect 失败: %v", err)
	}
	defer d.Close()

	// UseDatabase 单部分名称，config.Database 未设置
	err = d.UseDatabase(context.Background(), "myschema")
	_ = err
}

// TestTrinoDriver_ConnectLazy_then_TryQueryWithDB 测试 Trino TryQueryWithDB
func TestTrinoDriver_ConnectLazy_then_TryQueryWithDB(t *testing.T) {
	d := &TrinoDriver{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := d.Connect(ctx, DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "test",
		Database: "default",
	})

	if err != nil {
		t.Skipf("Trino Connect 失败: %v", err)
	}
	defer d.Close()

	_, err = d.TryQueryWithDB(context.Background(), "SELECT 1", "catalog.schema")
	_ = err
}

// TestTrinoDriver_ConnectLazy_then_Close 测试 Trino Close 有 db 时
// 覆盖 trino.go:62 Close 的有 db 分支
func TestTrinoDriver_ConnectLazy_then_Close(t *testing.T) {
	d := &TrinoDriver{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := d.Connect(ctx, DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "test",
		Database: "default",
	})

	if err != nil {
		t.Skipf("Trino Connect 失败: %v", err)
	}

	if err := d.Close(); err != nil {
		t.Errorf("Close 不应返回错误: %v", err)
	}
}

// TestTrinoDriver_Connect_WithPassword 测试 Trino Connect 带密码
// 覆盖 trino.go:272-277 buildDSN 的 password 分支
func TestTrinoDriver_Connect_WithPassword(t *testing.T) {
	d := &TrinoDriver{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = d.Connect(ctx, DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "test",
		Password: "secret",
		Database: "default",
	})
	defer d.Close()
}

// TestTrinoDriver_Connect_WithSSL 测试 Trino Connect 带 SSL 配置
// 覆盖 trino.go:263-265 buildDSN 的 SSL 分支
func TestTrinoDriver_Connect_WithSSL(t *testing.T) {
	d := &TrinoDriver{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = d.Connect(ctx, DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "test",
		Database: "default",
		Config:   map[string]interface{}{"ssl": true},
	})
	defer d.Close()
}

// TestTrinoDriver_Connect_WithLDAP 测试 Trino Connect 带 LDAP 认证
// 覆盖 trino.go:288-289 buildDSN 的 LDAP 分支
func TestTrinoDriver_Connect_WithLDAP(t *testing.T) {
	d := &TrinoDriver{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = d.Connect(ctx, DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "test",
		Password: "secret",
		Database: "default",
		AuthType: "ldap",
	})
	defer d.Close()
}

// TestTrinoDriver_Connect_DefaultPort 测试 Trino Connect 默认端口
// 覆盖 trino.go:27-29 的默认端口分支
func TestTrinoDriver_Connect_DefaultPort(t *testing.T) {
	d := &TrinoDriver{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = d.Connect(ctx, DatasourceConfig{
		Host:     "127.0.0.1",
		Port:     0,
		Username: "test",
		Database: "default",
	})
	defer d.Close()
}
