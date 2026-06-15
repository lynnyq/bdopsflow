package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
)

const (
	// redisKeyPrefix 查询注册表在 Redis 中的 key 前缀
	redisKeyPrefix = "bdopsflow:query:"
	// redisResultKeyPrefix 查询结果在 Redis 中的 key 前缀
	redisResultKeyPrefix = "bdopsflow:query:result:"
	// redisChannel 查询状态变更的 Pub/Sub 频道
	redisChannel = "bdopsflow:query:updates"
	// redisCancelChannel 查询取消指令的 Pub/Sub 频道
	redisCancelChannel = "bdopsflow:query:cancel"
	// redisDefaultTTL 查询记录在 Redis 中的默认过期时间
	redisDefaultTTL = 35 * time.Minute
)

// redisQueryState 存储在 Redis 中的查询状态（不含 CancelFunc）
type redisQueryState struct {
	QueryID       string      `json:"query_id"`
	DatasourceID  int64       `json:"datasource_id"`
	Database      string      `json:"database"`
	SQL           string      `json:"sql"`
	UserID        int64       `json:"user_id"`
	Status        QueryStatus `json:"status"`
	Error         string      `json:"error,omitempty"`
	ExecutionTime float64     `json:"execution_time"`
	FromCache     bool        `json:"from_cache"`
	NodeID        string      `json:"node_id"`
	CreatedAt     int64       `json:"created_at"`
	StartTime     int64       `json:"start_time,omitempty"`
}

// redisCancelCommand 跨节点取消指令
type redisCancelCommand struct {
	QueryID string `json:"query_id"`
	FromNode string `json:"from_node"`
}

// DistributedQueryRegistry 基于 Redis 的分布式查询注册表
// 同时使用 Redis（跨节点共享状态和结果）和本地内存（CancelFunc、SSE observer）
type DistributedQueryRegistry struct {
	redisClient *redis.Client
	nodeID      string

	// 本地存储：仅本节点执行的查询的 CancelFunc
	mu           sync.RWMutex
	localQueries map[string]*RunningQuery // 本节点注册的查询（含 CancelFunc）
	observers    map[QueryObserver]struct{}

	// Redis Pub/Sub 订阅
	subOnce   sync.Once
	subCancel context.CancelFunc
}

// NewDistributedQueryRegistry 创建分布式查询注册表
func NewDistributedQueryRegistry(redisClient *redis.Client, nodeID string) *DistributedQueryRegistry {
	r := &DistributedQueryRegistry{
		redisClient:  redisClient,
		nodeID:       nodeID,
		localQueries: make(map[string]*RunningQuery),
		observers:    make(map[QueryObserver]struct{}),
	}
	return r
}

// startSubscriber 启动 Redis Pub/Sub 订阅，接收其他节点的查询状态变更通知和取消指令
func (r *DistributedQueryRegistry) startSubscriber() {
	r.subOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		r.subCancel = cancel

		go func() {
			// 同时订阅状态更新频道和取消指令频道
			sub := r.redisClient.Subscribe(ctx, redisChannel, redisCancelChannel)
			defer sub.Close()

			ch := sub.Channel()
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-ch:
					if !ok {
						return
					}
					switch msg.Channel {
					case redisCancelChannel:
						r.handleCancelCommand(msg)
					default:
						r.handleRedisMessage(msg)
					}
				}
			}
		}()

		slog.Info("distributed query registry subscriber started", "node_id", r.nodeID, "channels", []string{redisChannel, redisCancelChannel})
	})
}

// handleRedisMessage 处理 Redis Pub/Sub 消息
func (r *DistributedQueryRegistry) handleRedisMessage(msg *redis.Message) {
	var state redisQueryState
	if err := json.Unmarshal([]byte(msg.Payload), &state); err != nil {
		slog.Warn("failed to unmarshal query update message", "error", err)
		return
	}

	// 跳过本节点发出的消息（本节点的 observer 已在 UpdateResult/UpdateError 中直接通知）
	if state.NodeID == r.nodeID {
		return
	}

	// 构造 RunningQuery 通知本节点的 SSE observer
	q := &RunningQuery{
		QueryID:       state.QueryID,
		DatasourceID:  state.DatasourceID,
		Database:      state.Database,
		SQL:           state.SQL,
		UserID:        state.UserID,
		Status:        state.Status,
		Error:         state.Error,
		ExecutionTime: state.ExecutionTime,
		FromCache:     state.FromCache,
		CreatedAt:     time.Unix(state.CreatedAt, 0),
	}
	if state.StartTime > 0 {
		q.StartTime = time.Unix(state.StartTime, 0)
	}

	// 终态查询尝试从 Redis 获取 Result
	if state.Status == QueryStatusCompleted {
		result, err := r.getResultFromRedis(context.Background(), state.QueryID)
		if err == nil && result != nil {
			q.Result = result
		}
	}

	// 通知本节点的 SSE observer
	r.mu.RLock()
	observers := make([]QueryObserver, 0, len(r.observers))
	for obs := range r.observers {
		observers = append(observers, obs)
	}
	r.mu.RUnlock()

	for _, obs := range observers {
		go obs.OnQueryUpdate(state.QueryID, q)
	}
}

// Register 注册查询到本地内存和 Redis
func (r *DistributedQueryRegistry) Register(query *RunningQuery) {
	query.CreatedAt = time.Now()

	// 存储到本地内存（保留 CancelFunc）
	r.mu.Lock()
	r.localQueries[query.QueryID] = query
	r.mu.Unlock()

	// 存储到 Redis
	state := redisQueryState{
		QueryID:      query.QueryID,
		DatasourceID: query.DatasourceID,
		Database:     query.Database,
		SQL:          query.SQL,
		UserID:       query.UserID,
		Status:       query.Status,
		NodeID:       r.nodeID,
		CreatedAt:    query.CreatedAt.Unix(),
	}
	r.saveToRedis(context.Background(), &state)

	// 启动订阅（懒启动）
	r.startSubscriber()
}

// Get 从本地内存或 Redis 获取查询状态
func (r *DistributedQueryRegistry) Get(queryID string) (*RunningQuery, bool) {
	// 先查本地内存（本节点执行的查询，含 CancelFunc）
	r.mu.RLock()
	if q, ok := r.localQueries[queryID]; ok {
		r.mu.RUnlock()
		return q, true
	}
	r.mu.RUnlock()

	// 再查 Redis（其他节点执行的查询）
	state, err := r.getFromRedis(context.Background(), queryID)
	if err != nil {
		slog.Debug("failed to get query state from Redis", "query_id", queryID, "error", err)
		return nil, false
	}
	if state == nil {
		return nil, false
	}

	q := &RunningQuery{
		QueryID:       state.QueryID,
		DatasourceID:  state.DatasourceID,
		Database:      state.Database,
		SQL:           state.SQL,
		UserID:        state.UserID,
		Status:        state.Status,
		Error:         state.Error,
		ExecutionTime: state.ExecutionTime,
		FromCache:     state.FromCache,
		CreatedAt:     time.Unix(state.CreatedAt, 0),
	}
	if state.StartTime > 0 {
		q.StartTime = time.Unix(state.StartTime, 0)
	}

	// 终态查询尝试从 Redis 获取 Result
	if state.Status == QueryStatusCompleted {
		result, resultErr := r.getResultFromRedis(context.Background(), queryID)
		if resultErr == nil && result != nil {
			q.Result = result
		}
	}

	return q, true
}

// UpdateResult 更新查询结果
func (r *DistributedQueryRegistry) UpdateResult(queryID string, result *driver.QueryResult, execTime float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if q, ok := r.localQueries[queryID]; ok {
		q.Status = QueryStatusCompleted
		q.Result = result
		q.ExecutionTime = execTime

		// 更新 Redis 状态
		state := r.localToRedisState(q)
		r.saveToRedis(context.Background(), state)

		// 将查询结果存入 Redis（跨节点可获取）
		r.saveResultToRedis(context.Background(), queryID, result)

		// 发布状态变更通知
		r.publishUpdate(context.Background(), state)

		// 通知本节点 observer
		r.notifyObservers(queryID, q)
	}
}

// UpdateError 更新查询错误
func (r *DistributedQueryRegistry) UpdateError(queryID string, errMsg string, execTime float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if q, ok := r.localQueries[queryID]; ok {
		q.Status = QueryStatusFailed
		q.Error = errMsg
		q.ExecutionTime = execTime

		// 更新 Redis 状态
		state := r.localToRedisState(q)
		r.saveToRedis(context.Background(), state)

		// 发布状态变更通知
		r.publishUpdate(context.Background(), state)

		// 通知本节点 observer
		r.notifyObservers(queryID, q)
	}
}

// Cancel 取消查询（支持跨节点取消）
func (r *DistributedQueryRegistry) Cancel(queryID string) bool {
	r.mu.Lock()

	q, ok := r.localQueries[queryID]
	if ok && (q.Status == QueryStatusPending || q.Status == QueryStatusRunning) {
		q.Status = QueryStatusCancelled
		if q.CancelFunc != nil {
			q.CancelFunc()
		}

		// 更新 Redis 状态
		state := r.localToRedisState(q)
		r.saveToRedis(context.Background(), state)

		// 发布状态变更通知
		r.publishUpdate(context.Background(), state)

		// 通知本节点 observer
		r.notifyObservers(queryID, q)

		r.mu.Unlock()
		return true
	}
	r.mu.Unlock()

	// 查询不在本节点或已终态，检查 Redis 获取查询所属节点
	state, err := r.getFromRedis(context.Background(), queryID)
	if err != nil || state == nil {
		return false
	}

	// 查询已终态，无法取消
	if state.Status == QueryStatusCompleted || state.Status == QueryStatusFailed || state.Status == QueryStatusCancelled {
		return false
	}

	// 查询在其他节点执行，通过 Redis Pub/Sub 发送取消指令
	if state.NodeID != "" && state.NodeID != r.nodeID {
		cmd := redisCancelCommand{
			QueryID:   queryID,
			FromNode:  r.nodeID,
		}
		r.publishCancelCommand(context.Background(), &cmd)
		slog.Info("published cross-node cancel command", "query_id", queryID, "target_node", state.NodeID, "from_node", r.nodeID)
		return true
	}

	return false
}

// handleCancelCommand 处理跨节点取消指令
func (r *DistributedQueryRegistry) handleCancelCommand(msg *redis.Message) {
	var cmd redisCancelCommand
	if err := json.Unmarshal([]byte(msg.Payload), &cmd); err != nil {
		slog.Warn("failed to unmarshal cancel command", "error", err)
		return
	}

	slog.Info("received cross-node cancel command", "query_id", cmd.QueryID, "from_node", cmd.FromNode)

	r.mu.Lock()
	defer r.mu.Unlock()

	q, ok := r.localQueries[cmd.QueryID]
	if !ok {
		slog.Debug("cancel command: query not found on this node", "query_id", cmd.QueryID)
		return
	}

	if q.Status == QueryStatusPending || q.Status == QueryStatusRunning {
		q.Status = QueryStatusCancelled
		q.Error = "查询已被用户取消"
		if q.CancelFunc != nil {
			q.CancelFunc()
		}

		// 更新 Redis 状态
		state := r.localToRedisState(q)
		r.saveToRedis(context.Background(), state)

		// 发布状态变更通知（通知发起取消的节点和所有其他观察者）
		r.publishUpdate(context.Background(), state)

		// 通知本节点 observer
		r.notifyObservers(cmd.QueryID, q)

		slog.Info("cross-node cancel executed", "query_id", cmd.QueryID, "from_node", cmd.FromNode)
	}
}

// SetRunning 设置查询为运行中状态
func (r *DistributedQueryRegistry) SetRunning(queryID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if q, ok := r.localQueries[queryID]; ok {
		q.Status = QueryStatusRunning
		q.StartTime = time.Now()

		// 更新 Redis 状态
		state := r.localToRedisState(q)
		r.saveToRedis(context.Background(), state)

		// 发布状态变更通知
		r.publishUpdate(context.Background(), state)

		// 通知本节点 observer
		r.notifyObservers(queryID, q)
	}
}

// Cleanup 清理过期的查询记录
func (r *DistributedQueryRegistry) Cleanup(maxAge time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for id, q := range r.localQueries {
		if q.Status == QueryStatusCompleted || q.Status == QueryStatusFailed || q.Status == QueryStatusCancelled {
			if now.Sub(q.CreatedAt) > maxAge {
				delete(r.localQueries, id)
				// 删除 Redis 中的记录
				r.deleteFromRedis(context.Background(), id)
				r.deleteResultFromRedis(context.Background(), id)
			}
		}
	}
}

// StartCleanupLoop 启动定期清理循环
func (r *DistributedQueryRegistry) StartCleanupLoop(interval, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			r.Cleanup(maxAge)
		}
	}()
}

// RegisterObserver 注册查询状态变更观察者
func (r *DistributedQueryRegistry) RegisterObserver(observer QueryObserver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observers[observer] = struct{}{}

	// 确保 Redis 订阅已启动
	r.startSubscriber()
}

// UnregisterObserver 注销查询状态变更观察者
func (r *DistributedQueryRegistry) UnregisterObserver(observer QueryObserver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.observers, observer)
}

// Close 关闭注册表，停止 Redis 订阅
func (r *DistributedQueryRegistry) Close() {
	if r.subCancel != nil {
		r.subCancel()
	}
}

// notifyObservers 通知所有观察者（调用方必须持有锁）
func (r *DistributedQueryRegistry) notifyObservers(queryID string, query *RunningQuery) {
	for observer := range r.observers {
		go observer.OnQueryUpdate(queryID, query)
	}
}

// localToRedisState 将本地 RunningQuery 转换为 Redis 存储格式
func (r *DistributedQueryRegistry) localToRedisState(q *RunningQuery) *redisQueryState {
	state := &redisQueryState{
		QueryID:       q.QueryID,
		DatasourceID:  q.DatasourceID,
		Database:      q.Database,
		SQL:           q.SQL,
		UserID:        q.UserID,
		Status:        q.Status,
		Error:         q.Error,
		ExecutionTime: q.ExecutionTime,
		FromCache:     q.FromCache,
		NodeID:        r.nodeID,
		CreatedAt:     q.CreatedAt.Unix(),
	}
	if !q.StartTime.IsZero() {
		state.StartTime = q.StartTime.Unix()
	}
	return state
}

// saveToRedis 将查询状态保存到 Redis
func (r *DistributedQueryRegistry) saveToRedis(ctx context.Context, state *redisQueryState) {
	data, err := json.Marshal(state)
	if err != nil {
		slog.Error("failed to marshal query state for Redis", "query_id", state.QueryID, "error", err)
		return
	}

	key := redisKeyPrefix + state.QueryID
	if err := r.redisClient.Set(ctx, key, data, redisDefaultTTL).Err(); err != nil {
		slog.Error("failed to save query state to Redis", "query_id", state.QueryID, "error", err)
	}
}

// getFromRedis 从 Redis 获取查询状态
func (r *DistributedQueryRegistry) getFromRedis(ctx context.Context, queryID string) (*redisQueryState, error) {
	key := redisKeyPrefix + queryID
	data, err := r.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get query state from Redis: %w", err)
	}

	var state redisQueryState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query state from Redis: %w", err)
	}
	return &state, nil
}

// deleteFromRedis 从 Redis 删除查询状态
func (r *DistributedQueryRegistry) deleteFromRedis(ctx context.Context, queryID string) {
	key := redisKeyPrefix + queryID
	if err := r.redisClient.Del(ctx, key).Err(); err != nil {
		slog.Error("failed to delete query state from Redis", "query_id", queryID, "error", err)
	}
}

// saveResultToRedis 将查询结果保存到 Redis（独立 key，避免状态数据过大）
func (r *DistributedQueryRegistry) saveResultToRedis(ctx context.Context, queryID string, result *driver.QueryResult) {
	if result == nil {
		return
	}

	// 限制存入 Redis 的结果大小，避免大结果集占用过多内存
	const maxRowsForRedis = 10000
	resultToSave := result
	if result.RowCount > maxRowsForRedis {
		resultToSave = &driver.QueryResult{
			Columns:  result.Columns,
			Rows:     result.Rows[:maxRowsForRedis],
			RowCount: result.RowCount,
		}
	}

	data, err := json.Marshal(resultToSave)
	if err != nil {
		slog.Error("failed to marshal query result for Redis", "query_id", queryID, "error", err)
		return
	}

	key := redisResultKeyPrefix + queryID
	if err := r.redisClient.Set(ctx, key, data, redisDefaultTTL).Err(); err != nil {
		slog.Error("failed to save query result to Redis", "query_id", queryID, "error", err)
	}
}

// getResultFromRedis 从 Redis 获取查询结果
func (r *DistributedQueryRegistry) getResultFromRedis(ctx context.Context, queryID string) (*driver.QueryResult, error) {
	key := redisResultKeyPrefix + queryID
	data, err := r.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get query result from Redis: %w", err)
	}

	var result driver.QueryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query result from Redis: %w", err)
	}
	return &result, nil
}

// deleteResultFromRedis 从 Redis 删除查询结果
func (r *DistributedQueryRegistry) deleteResultFromRedis(ctx context.Context, queryID string) {
	key := redisResultKeyPrefix + queryID
	if err := r.redisClient.Del(ctx, key).Err(); err != nil {
		slog.Error("failed to delete query result from Redis", "query_id", queryID, "error", err)
	}
}

// publishUpdate 发布查询状态变更到 Redis Pub/Sub
func (r *DistributedQueryRegistry) publishUpdate(ctx context.Context, state *redisQueryState) {
	data, err := json.Marshal(state)
	if err != nil {
		slog.Error("failed to marshal query update for Redis Pub/Sub", "query_id", state.QueryID, "error", err)
		return
	}

	if err := r.redisClient.Publish(ctx, redisChannel, data).Err(); err != nil {
		slog.Error("failed to publish query update to Redis", "query_id", state.QueryID, "error", err)
	}
}

// publishCancelCommand 发布跨节点取消指令到 Redis Pub/Sub
func (r *DistributedQueryRegistry) publishCancelCommand(ctx context.Context, cmd *redisCancelCommand) {
	data, err := json.Marshal(cmd)
	if err != nil {
		slog.Error("failed to marshal cancel command for Redis Pub/Sub", "query_id", cmd.QueryID, "error", err)
		return
	}

	if err := r.redisClient.Publish(ctx, redisCancelChannel, data).Err(); err != nil {
		slog.Error("failed to publish cancel command to Redis", "query_id", cmd.QueryID, "error", err)
	}
}
