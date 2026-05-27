package election

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestNewLeaderElection(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	election := NewLeaderElection(client, "test-leader", "node-1", "127.0.0.1:8080", 10*time.Second)
	if election == nil {
		t.Fatal("NewLeaderElection returned nil")
	}
	if election.client != client {
		t.Error("expected client to be set")
	}
	if election.leaderKey != "test-leader" {
		t.Errorf("expected leaderKey 'test-leader', got %q", election.leaderKey)
	}
	if election.nodeID != "node-1" {
		t.Errorf("expected nodeID 'node-1', got %q", election.nodeID)
	}
	if election.httpAddr != "127.0.0.1:8080" {
		t.Errorf("expected httpAddr '127.0.0.1:8080', got %q", election.httpAddr)
	}
	if election.ttl != 10*time.Second {
		t.Errorf("expected ttl 10s, got %v", election.ttl)
	}
}

func TestIsLeader(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	election := NewLeaderElection(client, "test-is-leader", "node-1", "", 10*time.Second)
	
	if election.IsLeader() {
		t.Error("expected not to be leader initially")
	}
}

func TestOnAcquire(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	election := NewLeaderElection(client, "test-on-acquire", "node-1", "", 10*time.Second)
	
	acquired := false
	election.OnAcquire(func() {
		acquired = true
	})
	
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	election.tryAcquire(ctx)
	
	if !acquired {
		t.Error("expected onAcquire to be called")
	}
	
	if !election.IsLeader() {
		t.Error("expected to be leader after acquiring")
	}
}

func TestOnRelease(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	election := NewLeaderElection(client, "test-on-release", "node-1", "", 10*time.Second)
	
	released := false
	election.OnRelease(func() {
		released = true
	})
	
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	election.tryAcquire(ctx)
	
	if !election.IsLeader() {
		t.Skip("skip test if failed to acquire leadership")
	}
	
	election.Stop(ctx)
	
	if !released {
		t.Error("expected onRelease to be called")
	}
	
	if election.IsLeader() {
		t.Error("expected not to be leader after releasing")
	}
}

func TestStop_NotLeader(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	election := NewLeaderElection(client, "test-stop-not-leader", "node-1", "", 10*time.Second)
	
	ctx := context.Background()
	election.Stop(ctx)
	
	if election.IsLeader() {
		t.Error("expected not to be leader")
	}
}

func TestTryAcquire_Failure(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6380",
	})
	defer client.Close()

	election := NewLeaderElection(client, "test-acquire-fail", "node-1", "", 10*time.Second)
	
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	election.tryAcquire(ctx)
	
	if election.IsLeader() {
		t.Error("expected not to be leader when Redis is unavailable")
	}
}