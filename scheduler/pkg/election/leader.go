package election

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type LeaderElection struct {
	client    *redis.Client
	leaderKey string
	nodeID    string
	httpAddr  string
	ttl       time.Duration
	isLeader  bool
	mu        sync.RWMutex
	onAcquire func()
	onRelease func()
}

func NewLeaderElection(client *redis.Client, leaderKey, nodeID, httpAddr string, ttl time.Duration) *LeaderElection {
	return &LeaderElection{
		client:    client,
		leaderKey: leaderKey,
		nodeID:    nodeID,
		httpAddr:  httpAddr,
		ttl:       ttl,
	}
}

func (e *LeaderElection) OnAcquire(f func()) {
	e.onAcquire = f
}

func (e *LeaderElection) OnRelease(f func()) {
	e.onRelease = f
}

func (e *LeaderElection) Start(ctx context.Context) {
	e.tryAcquire(ctx)
	go e.watch(ctx)
}

func (e *LeaderElection) tryAcquire(ctx context.Context) {
	value := e.nodeID
	if e.httpAddr != "" {
		value = e.nodeID + "|" + e.httpAddr
	}
	ok, err := e.client.SetNX(ctx, e.leaderKey, value, e.ttl).Result()
	if err != nil {
		slog.Error("[Election] Failed to acquire leader", "error", err, "node_id", e.nodeID)
		return
	}

	if ok {
		e.mu.Lock()
		wasLeader := e.isLeader
		if !wasLeader {
			e.isLeader = true
			slog.Info("[Election] Node became leader", "node_id", e.nodeID)
		}
		onAcquire := e.onAcquire
		e.mu.Unlock()

		if !wasLeader && onAcquire != nil {
			onAcquire()
		}
	}
}

func (e *LeaderElection) watch(ctx context.Context) {
	ticker := time.NewTicker(e.ttl / 2)
	defer ticker.Stop()

	consecutiveFailures := 0
	maxFailures := 3

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.mu.RLock()
			currentIsLeader := e.isLeader
			e.mu.RUnlock()

			if currentIsLeader {
				currentHolder, err := e.client.Get(ctx, e.leaderKey).Result()
				if err == redis.Nil {
					slog.Warn("[Election] Leader key lost, attempting to re-acquire", "node_id", e.nodeID)
					e.stepDown()
					e.tryAcquire(ctx)
					consecutiveFailures = 0
				} else if err != nil {
					slog.Error("[Election] Failed to check leader key", "error", err, "node_id", e.nodeID)
					consecutiveFailures++
					if consecutiveFailures >= maxFailures {
						slog.Error("[Election] Too many consecutive failures, stepping down", "failures", consecutiveFailures, "node_id", e.nodeID)
						e.stepDown()
						consecutiveFailures = 0
					}
				} else if currentHolder == e.nodeID || strings.HasPrefix(currentHolder, e.nodeID+"|") {
					if err := e.client.Expire(ctx, e.leaderKey, e.ttl).Err(); err != nil {
						slog.Error("[Election] Failed to extend leader TTL", "error", err, "node_id", e.nodeID)
						consecutiveFailures++
						if consecutiveFailures >= maxFailures {
							slog.Error("[Election] Too many consecutive TTL extension failures, stepping down", "failures", consecutiveFailures, "node_id", e.nodeID)
							e.stepDown()
							consecutiveFailures = 0
						}
					} else {
						consecutiveFailures = 0
					}
				} else {
					slog.Warn("[Election] Leader key held by another node, stepping down", "current_holder", currentHolder, "node_id", e.nodeID)
					e.stepDown()
					consecutiveFailures = 0
				}
			} else {
				e.tryAcquire(ctx)
				consecutiveFailures = 0
			}
		}
	}
}

func (e *LeaderElection) stepDown() {
	e.mu.Lock()
	wasLeader := e.isLeader
	if wasLeader {
		e.isLeader = false
		slog.Info("[Election] Node stepped down from leader", "node_id", e.nodeID)

		currentHolder, err := e.client.Get(context.Background(), e.leaderKey).Result()
		if err == nil && currentHolder == e.nodeID {
			deleted, delErr := e.client.Del(context.Background(), e.leaderKey).Result()
			if delErr != nil {
				slog.Warn("[Election] Failed to delete leader key on stepdown", "error", delErr, "node_id", e.nodeID)
			} else {
				slog.Info("[Election] Deleted leader key on stepdown", "deleted", deleted, "node_id", e.nodeID)
			}
		}
	}
	onRelease := e.onRelease
	e.mu.Unlock()

	if wasLeader && onRelease != nil {
		onRelease()
	}
}

func (e *LeaderElection) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.isLeader
}

func (e *LeaderElection) Stop(ctx context.Context) {
	e.stepDown()
}

func (e *LeaderElection) GetLeaderHTTPAddr(ctx context.Context) (string, error) {
	value, err := e.client.Get(ctx, e.leaderKey).Result()
	if err != nil {
		return "", err
	}

	sepIdx := strings.Index(value, "|")
	if sepIdx < 0 {
		return "", nil
	}

	return value[sepIdx+1:], nil
}

// GetLeaderInfo 返回当前 leader 的节点 ID 和 HTTP 地址
func (e *LeaderElection) GetLeaderInfo(ctx context.Context) (nodeID string, httpAddr string, err error) {
	value, err := e.client.Get(ctx, e.leaderKey).Result()
	if err != nil {
		return "", "", err
	}

	sepIdx := strings.Index(value, "|")
	if sepIdx < 0 {
		return value, "", nil
	}

	return value[:sepIdx], value[sepIdx+1:], nil
}

// GetNodeID 返回当前节点的 ID
func (e *LeaderElection) GetNodeID() string {
	return e.nodeID
}
