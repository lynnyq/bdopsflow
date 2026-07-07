package handler

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
)

// setupTestRedis 创建测试用的 miniredis 和 Redis 客户端
func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to create miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return mr, client
}

func TestDistributedQueryRegistry_RegisterAndGet(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	registry := NewDistributedQueryRegistry(client, "node-1")

	query := &RunningQuery{
		QueryID:      "q_test_001",
		DatasourceID: 1,
		Database:     "test_db",
		SQL:          "SELECT 1",
		UserID:       1,
		Status:       QueryStatusPending,
		CancelFunc:   func() {},
	}

	registry.Register(query)

	// 本节点应该能获取到查询
	got, ok := registry.Get("q_test_001")
	if !ok {
		t.Fatal("query should exist in registry")
	}
	if got.QueryID != "q_test_001" {
		t.Errorf("QueryID = %v, want q_test_001", got.QueryID)
	}
	if got.Status != QueryStatusPending {
		t.Errorf("Status = %v, want pending", got.Status)
	}
}

func TestDistributedQueryRegistry_CrossNodeGet(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	// 模拟 Node A 注册查询
	nodeA := NewDistributedQueryRegistry(client, "node-A")
	query := &RunningQuery{
		QueryID:      "q_cross_node",
		DatasourceID: 1,
		Database:     "test_db",
		SQL:          "SELECT * FROM users",
		UserID:       1,
		Status:       QueryStatusPending,
		CancelFunc:   func() {},
	}
	nodeA.Register(query)

	// 模拟 Node B 获取查询（跨节点）
	nodeB := NewDistributedQueryRegistry(client, "node-B")
	got, ok := nodeB.Get("q_cross_node")
	if !ok {
		t.Fatal("query should be accessible from another node via Redis")
	}
	if got.QueryID != "q_cross_node" {
		t.Errorf("QueryID = %v, want q_cross_node", got.QueryID)
	}
	if got.Status != QueryStatusPending {
		t.Errorf("Status = %v, want pending", got.Status)
	}
}

func TestDistributedQueryRegistry_CrossNodeGetResult(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	// Node A 执行查询并更新结果
	nodeA := NewDistributedQueryRegistry(client, "node-A")
	query := &RunningQuery{
		QueryID:      "q_result_cross",
		DatasourceID: 1,
		Database:     "test_db",
		SQL:          "SELECT 1",
		UserID:       1,
		Status:       QueryStatusPending,
		CancelFunc:   func() {},
	}
	nodeA.Register(query)

	result := &driver.QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]interface{}{{int64(1), "test"}},
		RowCount: 1,
	}
	nodeA.UpdateResult("q_result_cross", result, 0.5)

	// Node B 获取已完成查询的结果
	nodeB := NewDistributedQueryRegistry(client, "node-B")
	got, ok := nodeB.Get("q_result_cross")
	if !ok {
		t.Fatal("query should be accessible from another node")
	}
	if got.Status != QueryStatusCompleted {
		t.Errorf("Status = %v, want completed", got.Status)
	}
	if got.Result == nil {
		t.Fatal("Result should not be nil for completed query from another node")
	}
	if len(got.Result.Columns) != 2 {
		t.Errorf("Columns length = %d, want 2", len(got.Result.Columns))
	}
	if got.Result.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", got.Result.RowCount)
	}
}

func TestDistributedQueryRegistry_CrossNodeGetError(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	// Node A 执行查询失败
	nodeA := NewDistributedQueryRegistry(client, "node-A")
	query := &RunningQuery{
		QueryID:      "q_error_cross",
		DatasourceID: 1,
		Database:     "test_db",
		SQL:          "SELECT * FROM nonexistent",
		UserID:       1,
		Status:       QueryStatusPending,
		CancelFunc:   func() {},
	}
	nodeA.Register(query)
	nodeA.UpdateError("q_error_cross", "table not found", 0.1)

	// Node B 获取失败查询的信息
	nodeB := NewDistributedQueryRegistry(client, "node-B")
	got, ok := nodeB.Get("q_error_cross")
	if !ok {
		t.Fatal("query should be accessible from another node")
	}
	if got.Status != QueryStatusFailed {
		t.Errorf("Status = %v, want failed", got.Status)
	}
	if got.Error != "table not found" {
		t.Errorf("Error = %v, want 'table not found'", got.Error)
	}
}

func TestDistributedQueryRegistry_CancelLocalOnly(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	// Node A 注册查询
	nodeA := NewDistributedQueryRegistry(client, "node-A")
	var cancelled atomic.Bool
	query := &RunningQuery{
		QueryID:    "q_cancel_local",
		Status:     QueryStatusRunning,
		CancelFunc: func() { cancelled.Store(true) },
	}
	nodeA.Register(query)

	// Node A 可以取消自己的查询
	ok := nodeA.Cancel("q_cancel_local")
	if !ok {
		t.Fatal("should be able to cancel local query")
	}
	if !cancelled.Load() {
		t.Error("CancelFunc should have been called")
	}

	// 验证 Redis 中的状态已更新
	got, _ := nodeA.Get("q_cancel_local")
	if got.Status != QueryStatusCancelled {
		t.Errorf("Status = %v, want cancelled", got.Status)
	}
}

func TestDistributedQueryRegistry_CancelCrossNode(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	// Node A 注册查询
	nodeA := NewDistributedQueryRegistry(client, "node-A")
	var cancelled atomic.Bool
	query := &RunningQuery{
		QueryID:    "q_cancel_remote",
		Status:     QueryStatusRunning,
		CancelFunc: func() { cancelled.Store(true) },
	}
	nodeA.Register(query)

	// Node B 注册观察者（触发订阅启动）
	nodeB := NewDistributedQueryRegistry(client, "node-B")
	ch := make(chan *RunningQuery, 10)
	observer := &testObserver{ch: ch}
	nodeB.RegisterObserver(observer)

	// 等待 Redis Pub/Sub 订阅就绪
	time.Sleep(100 * time.Millisecond)

	// Node B 发起跨节点取消
	ok := nodeB.Cancel("q_cancel_remote")
	if !ok {
		t.Error("cross-node cancel should return true (cancel command published)")
	}

	// 等待跨节点取消指令执行
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cancelled.Load() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 验证 Node A 的查询已被取消
	if !cancelled.Load() {
		t.Error("CancelFunc on Node A should have been called via cross-node cancel")
	}

	// 验证 Redis 中的状态已更新为 cancelled
	got, _ := nodeB.Get("q_cancel_remote")
	if got.Status != QueryStatusCancelled {
		t.Errorf("Status = %v, want cancelled after cross-node cancel", got.Status)
	}
}

func TestDistributedQueryRegistry_SetRunning(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	registry := NewDistributedQueryRegistry(client, "node-1")
	query := &RunningQuery{
		QueryID:    "q_running",
		Status:     QueryStatusPending,
		CancelFunc: func() {},
	}
	registry.Register(query)

	registry.SetRunning("q_running")

	got, ok := registry.Get("q_running")
	if !ok {
		t.Fatal("query should exist")
	}
	if got.Status != QueryStatusRunning {
		t.Errorf("Status = %v, want running", got.Status)
	}
}

func TestDistributedQueryRegistry_Cleanup(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	registry := NewDistributedQueryRegistry(client, "node-1")

	// 注册一个已完成的查询
	query := &RunningQuery{
		QueryID:    "q_cleanup",
		Status:     QueryStatusCompleted,
		Result:     &driver.QueryResult{Columns: []string{"id"}, Rows: [][]interface{}{{1}}, RowCount: 1},
		CancelFunc: func() {},
	}
	registry.Register(query)

	// 手动设置 CreatedAt 为 31 分钟前
	registry.mu.Lock()
	q := registry.localQueries["q_cleanup"]
	q.CreatedAt = time.Now().Add(-31 * time.Minute)
	registry.mu.Unlock()

	// 清理超过 30 分钟的已完成查询
	registry.Cleanup(30 * time.Minute)

	_, ok := registry.Get("q_cleanup")
	if ok {
		t.Error("expired query should be cleaned up")
	}
}

func TestDistributedQueryRegistry_Observer(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	registry := NewDistributedQueryRegistry(client, "node-1")

	query := &RunningQuery{
		QueryID:    "q_observer",
		Status:     QueryStatusPending,
		CancelFunc: func() {},
	}
	registry.Register(query)

	// 注册观察者
	ch := make(chan *RunningQuery, 10)
	observer := &testObserver{ch: ch}
	registry.RegisterObserver(observer)

	// 更新结果应触发观察者
	result := &driver.QueryResult{
		Columns:  []string{"id"},
		Rows:     [][]interface{}{{1}},
		RowCount: 1,
	}
	registry.UpdateResult("q_observer", result, 0.1)

	// 等待观察者通知
	select {
	case q := <-ch:
		if q.QueryID != "q_observer" {
			t.Errorf("QueryID = %v, want q_observer", q.QueryID)
		}
		if q.Status != QueryStatusCompleted {
			t.Errorf("Status = %v, want completed", q.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("observer should have been notified")
	}

	registry.UnregisterObserver(observer)
}

func TestDistributedQueryRegistry_CrossNodeObserver(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	// Node A 注册查询
	nodeA := NewDistributedQueryRegistry(client, "node-A")
	query := &RunningQuery{
		QueryID:    "q_cross_observer",
		Status:     QueryStatusPending,
		CancelFunc: func() {},
	}
	nodeA.Register(query)

	// Node B 注册观察者（触发 Redis 订阅启动）
	nodeB := NewDistributedQueryRegistry(client, "node-B")
	ch := make(chan *RunningQuery, 10)
	observer := &testObserver{ch: ch}
	nodeB.RegisterObserver(observer)

	// 等待 Redis Pub/Sub 订阅就绪
	time.Sleep(100 * time.Millisecond)

	// Node A 更新结果
	result := &driver.QueryResult{
		Columns:  []string{"id"},
		Rows:     [][]interface{}{{1}},
		RowCount: 1,
	}
	nodeA.UpdateResult("q_cross_observer", result, 0.1)

	// Node B 的观察者应该通过 Redis Pub/Sub 收到通知
	select {
	case q := <-ch:
		if q.QueryID != "q_cross_observer" {
			t.Errorf("QueryID = %v, want q_cross_observer", q.QueryID)
		}
		if q.Status != QueryStatusCompleted {
			t.Errorf("Status = %v, want completed", q.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Node B observer should have been notified via Redis Pub/Sub")
	}
}

func TestDistributedQueryRegistry_QueryNotFound(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	registry := NewDistributedQueryRegistry(client, "node-1")

	_, ok := registry.Get("nonexistent_query")
	if ok {
		t.Error("nonexistent query should not be found")
	}
}

func TestDistributedQueryRegistry_RedisStateSerialization(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	registry := NewDistributedQueryRegistry(client, "node-1")

	query := &RunningQuery{
		QueryID:       "q_serial",
		DatasourceID:  42,
		Database:      "my_db",
		SQL:           "SELECT * FROM test",
		UserID:        7,
		Status:        QueryStatusPending,
		ExecutionTime: 0,
		FromCache:     false,
		CancelFunc:    func() {},
	}
	registry.Register(query)

	// 直接验证 Redis 中的数据
	key := redisKeyPrefix + "q_serial"
	data, err := client.Get(context.Background(), key).Bytes()
	if err != nil {
		t.Fatalf("failed to get query state from Redis: %v", err)
	}

	var state redisQueryState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("failed to unmarshal query state: %v", err)
	}

	if state.QueryID != "q_serial" {
		t.Errorf("QueryID = %v, want q_serial", state.QueryID)
	}
	if state.DatasourceID != 42 {
		t.Errorf("DatasourceID = %v, want 42", state.DatasourceID)
	}
	if state.Database != "my_db" {
		t.Errorf("Database = %v, want my_db", state.Database)
	}
	if state.NodeID != "node-1" {
		t.Errorf("NodeID = %v, want node-1", state.NodeID)
	}
	if state.Status != QueryStatusPending {
		t.Errorf("Status = %v, want pending", state.Status)
	}
}

// testObserver 测试用观察者
type testObserver struct {
	ch chan *RunningQuery
}

func (o *testObserver) OnQueryUpdate(queryID string, query *RunningQuery) {
	select {
	case o.ch <- query:
	default:
	}
}
