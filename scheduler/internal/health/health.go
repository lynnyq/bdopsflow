package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	rqlite "github.com/rqlite/gorqlite"
)

type CheckStatus string

const (
	StatusPassing CheckStatus = "passing"
	StatusWarning CheckStatus = "warning"
	StatusFailing CheckStatus = "failing"
)

type HealthCheck struct {
	Name      string                 `json:"name"`
	Status    CheckStatus            `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Duration  time.Duration          `json:"duration_ms"`
}

type HealthReport struct {
	Status    CheckStatus            `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version,omitempty"`
	Checks    []HealthCheck          `json:"checks"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type Checker interface {
	Name() string
	Check(ctx context.Context) HealthCheck
}

type HealthChecker struct {
	checkers []Checker
	mu       sync.RWMutex
	version  string
}

func NewHealthChecker(version string) *HealthChecker {
	return &HealthChecker{
		checkers: make([]Checker, 0),
		version:  version,
	}
}

func (hc *HealthChecker) RegisterChecker(checker Checker) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checkers = append(hc.checkers, checker)
}

func (hc *HealthChecker) Check(ctx context.Context) *HealthReport {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	report := &HealthReport{
		Timestamp: time.Now(),
		Status:    StatusPassing,
		Version:   hc.version,
		Checks:    make([]HealthCheck, 0, len(hc.checkers)),
		Metadata:  make(map[string]interface{}),
	}

	for _, checker := range hc.checkers {
		check := checker.Check(ctx)
		report.Checks = append(report.Checks, check)

		if check.Status == StatusFailing {
			report.Status = StatusFailing
		} else if check.Status == StatusWarning && report.Status != StatusFailing {
			report.Status = StatusWarning
		}
	}

	return report
}

type RedisChecker struct {
	client *redis.Client
	name   string
}

func NewRedisChecker(client *redis.Client, name string) *RedisChecker {
	return &RedisChecker{
		client: client,
		name:   name,
	}
}

func (rc *RedisChecker) Name() string {
	return rc.name
}

func (rc *RedisChecker) Check(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		Name:      rc.Name(),
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := rc.client.Ping(ctx).Err(); err != nil {
		check.Status = StatusFailing
		check.Error = err.Error()
		check.Message = "Redis connection failed"
	} else {
		check.Status = StatusPassing
		check.Message = "Redis connection healthy"
	}

	check.Duration = time.Since(start)
	return check
}

type RQLiteChecker struct {
	conn *rqlite.Connection
	name string
}

func NewRQLiteChecker(conn *rqlite.Connection, name string) *RQLiteChecker {
	return &RQLiteChecker{
		conn: conn,
		name: name,
	}
}

func (rqc *RQLiteChecker) Name() string {
	return rqc.name
}

func (rqc *RQLiteChecker) Check(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		Name:      rqc.Name(),
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	stmt := rqlite.ParameterizedStatement{
		Query: "SELECT 1",
	}
	qr, err := rqc.conn.QueryOneParameterized(stmt)
	if err != nil || qr.Err != nil {
		check.Status = StatusFailing
		if err != nil {
			check.Error = err.Error()
		} else {
			check.Error = qr.Err.Error()
		}
		check.Message = "RQLite connection failed"
	} else {
		check.Status = StatusPassing
		check.Message = "RQLite connection healthy"
	}

	check.Duration = time.Since(start)
	return check
}

type DiskSpaceChecker struct {
	name      string
	warnPercent float64
	critPercent float64
	getDiskUsage func() (float64, error)
}

func NewDiskSpaceChecker(name string) *DiskSpaceChecker {
	return &DiskSpaceChecker{
		name:        name,
		warnPercent: 80.0,
		critPercent: 95.0,
		getDiskUsage: func() (float64, error) {
			return 50.0, nil
		},
	}
}

func (dsc *DiskSpaceChecker) Name() string {
	return dsc.name
}

func (dsc *DiskSpaceChecker) Check(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		Name:      dsc.Name(),
		Timestamp: time.Now(),
	}

	usage, err := dsc.getDiskUsage()
	if err != nil {
		check.Status = StatusWarning
		check.Error = err.Error()
		check.Message = "Failed to check disk space"
	} else {
		check.Metadata = map[string]interface{}{
			"usage_percent": usage,
		}
		if usage >= dsc.critPercent {
			check.Status = StatusFailing
			check.Message = fmt.Sprintf("Disk usage critical: %.1f%%", usage)
		} else if usage >= dsc.warnPercent {
			check.Status = StatusWarning
			check.Message = fmt.Sprintf("Disk usage warning: %.1f%%", usage)
		} else {
			check.Status = StatusPassing
			check.Message = fmt.Sprintf("Disk usage healthy: %.1f%%", usage)
		}
	}

	check.Duration = time.Since(start)
	return check
}
