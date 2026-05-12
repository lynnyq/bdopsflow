package election

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type LeaderElection struct {
	client     *redis.Client
	leaderKey  string
	nodeID     string
	ttl        time.Duration
	isLeader   bool
	onAcquire  func()
	onRelease  func()
}

func NewLeaderElection(client *redis.Client, leaderKey, nodeID string, ttl time.Duration) *LeaderElection {
	return &LeaderElection{
		client:    client,
		leaderKey: leaderKey,
		nodeID:    nodeID,
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
	ok, err := e.client.SetNX(ctx, e.leaderKey, e.nodeID, e.ttl).Result()
	if err != nil {
		fmt.Printf("[Election] Failed to acquire leader: %v\n", err)
		return
	}

	if ok && !e.isLeader {
		e.isLeader = true
		fmt.Printf("[Election] Node %s became leader\n", e.nodeID)
		if e.onAcquire != nil {
			e.onAcquire()
		}
	}
}

func (e *LeaderElection) watch(ctx context.Context) {
	ticker := time.NewTicker(e.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if e.isLeader {
				if err := e.client.Expire(ctx, e.leaderKey, e.ttl).Err(); err != nil {
					fmt.Printf("[Election] Failed to extend leader TTL: %v\n", err)
				}
			} else {
				e.tryAcquire(ctx)
			}
		}
	}
}

func (e *LeaderElection) IsLeader() bool {
	return e.isLeader
}

func (e *LeaderElection) Stop(ctx context.Context) {
	if e.isLeader {
		e.client.Del(ctx, e.leaderKey)
		e.isLeader = false
		fmt.Printf("[Election] Node %s released leadership\n", e.nodeID)
		if e.onRelease != nil {
			e.onRelease()
		}
	}
}
