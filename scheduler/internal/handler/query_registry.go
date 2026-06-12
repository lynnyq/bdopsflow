package handler

import (
	"context"
	"sync"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/datasource/driver"
)

type QueryStatus string

const (
	QueryStatusPending   QueryStatus = "pending"
	QueryStatusRunning   QueryStatus = "running"
	QueryStatusCompleted QueryStatus = "completed"
	QueryStatusFailed    QueryStatus = "failed"
	QueryStatusCancelled QueryStatus = "cancelled"
)

type RunningQuery struct {
	QueryID       string
	DatasourceID  int64
	Database      string
	SQL           string
	UserID        int64
	Status        QueryStatus
	Result        *driver.QueryResult
	Error         string
	ExecutionTime float64
	StartTime     time.Time
	FromCache     bool
	CancelFunc    context.CancelFunc
	CreatedAt     time.Time
}

// QueryObserver 查询状态变更观察者，用于 SSE 推送
type QueryObserver interface {
	OnQueryUpdate(queryID string, query *RunningQuery)
}

type QueryRegistry struct {
	mu        sync.RWMutex
	queries   map[string]*RunningQuery
	observers map[QueryObserver]struct{}
}

func NewQueryRegistry() *QueryRegistry {
	return &QueryRegistry{
		queries:   make(map[string]*RunningQuery),
		observers: make(map[QueryObserver]struct{}),
	}
}

func (r *QueryRegistry) Register(query *RunningQuery) {
	r.mu.Lock()
	defer r.mu.Unlock()
	query.CreatedAt = time.Now()
	r.queries[query.QueryID] = query
}

func (r *QueryRegistry) Get(queryID string) (*RunningQuery, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	q, ok := r.queries[queryID]
	return q, ok
}

func (r *QueryRegistry) UpdateResult(queryID string, result *driver.QueryResult, execTime float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if q, ok := r.queries[queryID]; ok {
		q.Status = QueryStatusCompleted
		q.Result = result
		q.ExecutionTime = execTime
		r.notifyObservers(queryID, q)
	}
}

func (r *QueryRegistry) UpdateError(queryID string, errMsg string, execTime float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if q, ok := r.queries[queryID]; ok {
		q.Status = QueryStatusFailed
		q.Error = errMsg
		q.ExecutionTime = execTime
		r.notifyObservers(queryID, q)
	}
}

func (r *QueryRegistry) Cancel(queryID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	q, ok := r.queries[queryID]
	if !ok {
		return false
	}
	if q.Status == QueryStatusPending || q.Status == QueryStatusRunning {
		q.Status = QueryStatusCancelled
		if q.CancelFunc != nil {
			q.CancelFunc()
		}
		r.notifyObservers(queryID, q)
		return true
	}
	return false
}

func (r *QueryRegistry) SetRunning(queryID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if q, ok := r.queries[queryID]; ok {
		q.Status = QueryStatusRunning
		q.StartTime = time.Now()
		r.notifyObservers(queryID, q)
	}
}

func (r *QueryRegistry) Cleanup(maxAge time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for id, q := range r.queries {
		if q.Status == QueryStatusCompleted || q.Status == QueryStatusFailed || q.Status == QueryStatusCancelled {
			if now.Sub(q.CreatedAt) > maxAge {
				delete(r.queries, id)
			}
		}
	}
}

func (r *QueryRegistry) StartCleanupLoop(interval, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			r.Cleanup(maxAge)
		}
	}()
}

// RegisterObserver 注册查询状态变更观察者
func (r *QueryRegistry) RegisterObserver(observer QueryObserver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observers[observer] = struct{}{}
}

// UnregisterObserver 注销查询状态变更观察者
func (r *QueryRegistry) UnregisterObserver(observer QueryObserver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.observers, observer)
}

// notifyObservers 通知所有观察者（调用方必须持有锁）
func (r *QueryRegistry) notifyObservers(queryID string, query *RunningQuery) {
	for observer := range r.observers {
		go observer.OnQueryUpdate(queryID, query)
	}
}
