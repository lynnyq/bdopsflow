package health

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestNewHealthChecker(t *testing.T) {
	checker := NewHealthChecker("1.0.0")
	assert.NotNil(t, checker)
	assert.Equal(t, "1.0.0", checker.version)
	assert.Empty(t, checker.checkers)
}

func TestHealthChecker_RegisterChecker(t *testing.T) {
	checker := NewHealthChecker("1.0.0")
	mockChecker := &mockChecker{name: "test"}
	
	checker.RegisterChecker(mockChecker)
	assert.Len(t, checker.checkers, 1)
}

func TestHealthChecker_Check(t *testing.T) {
	checker := NewHealthChecker("1.0.0")
	
	passingChecker := &mockChecker{name: "passing", status: StatusPassing}
	warningChecker := &mockChecker{name: "warning", status: StatusWarning}
	failingChecker := &mockChecker{name: "failing", status: StatusFailing}
	
	checker.RegisterChecker(passingChecker)
	report := checker.Check(context.Background())
	assert.Equal(t, StatusPassing, report.Status)
	
	checker.RegisterChecker(warningChecker)
	report = checker.Check(context.Background())
	assert.Equal(t, StatusWarning, report.Status)
	
	checker.RegisterChecker(failingChecker)
	report = checker.Check(context.Background())
	assert.Equal(t, StatusFailing, report.Status)
	
	assert.Len(t, report.Checks, 3)
}

func TestRedisChecker(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()
	
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	
	checker := NewRedisChecker(client, "redis_test")
	assert.Equal(t, "redis_test", checker.Name())
	
	check := checker.Check(context.Background())
	assert.Equal(t, StatusPassing, check.Status)
	assert.Equal(t, "Redis connection healthy", check.Message)
}

func TestHealthReport_Version(t *testing.T) {
	checker := NewHealthChecker("2.0.0")
	report := checker.Check(context.Background())
	assert.Equal(t, "2.0.0", report.Version)
}

func TestHealthReport_Timestamp(t *testing.T) {
	checker := NewHealthChecker("1.0.0")
	before := time.Now()
	report := checker.Check(context.Background())
	after := time.Now()
	
	assert.True(t, report.Timestamp.After(before) || report.Timestamp.Equal(before))
	assert.True(t, report.Timestamp.Before(after) || report.Timestamp.Equal(after))
}

func TestDiskSpaceChecker(t *testing.T) {
	checker := NewDiskSpaceChecker("disk_test")
	assert.Equal(t, "disk_test", checker.Name())
	
	check := checker.Check(context.Background())
	assert.Equal(t, StatusPassing, check.Status)
	assert.Contains(t, check.Message, "Disk usage healthy")
	assert.NotNil(t, check.Metadata)
	assert.Equal(t, 50.0, check.Metadata["usage_percent"])
}

type mockChecker struct {
	name   string
	status CheckStatus
}

func (mc *mockChecker) Name() string {
	return mc.name
}

func (mc *mockChecker) Check(ctx context.Context) HealthCheck {
	return HealthCheck{
		Name:      mc.name,
		Status:    mc.status,
		Timestamp: time.Now(),
	}
}
