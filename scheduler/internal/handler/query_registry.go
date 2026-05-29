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
	QueryID        string
	DatasourceID   int64
	Database       string
	SQL            string
	UserID         int64
	Status         QueryStatus
	Result         *driver.QueryResult
	Error          string
	ExecutionTime  float64
	StartTime      time.Time
	FromCache      bool
	CancelFunc     context.CancelFunc
	CreatedAt      time.Time
}

type QueryRegistry struct {
	mu      sync.RWMutex
	queries map[string]*RunningQuery
}

func NewQueryRegistry() *QueryRegistry {
	return &QueryRegistry{
		queries: make(map[string]*RunningQuery),
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
	}
}

func (r *QueryRegistry) UpdateError(queryID string, errMsg string, execTime float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if q, ok := r.queries[queryID]; ok {
		q.Status = QueryStatusFailed
		q.Error = errMsg
		q.ExecutionTime = execTime
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
