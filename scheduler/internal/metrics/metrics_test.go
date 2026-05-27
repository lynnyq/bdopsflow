package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestNewMetricsCollector(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()
	
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	
	collector := NewMetricsCollector(client)
	assert.NotNil(t, collector)
	assert.Equal(t, client, collector.redis)
}

func TestMetricsCollector_RegisterCounter(t *testing.T) {
	collector := NewMetricsCollector(nil)
	counter := collector.RegisterCounter("test_counter")
	
	assert.NotNil(t, counter)
	assert.Equal(t, "test_counter", counter.Name)
	assert.Equal(t, int64(0), counter.Value)
}

func TestCounter_IncAndGet(t *testing.T) {
	collector := NewMetricsCollector(nil)
	counter := collector.RegisterCounter("test_inc")
	
	counter.Inc(1)
	assert.Equal(t, int64(1), counter.Get())
	
	counter.Inc(5)
	assert.Equal(t, int64(6), counter.Get())
}

func TestGauge_SetAndGet(t *testing.T) {
	collector := NewMetricsCollector(nil)
	gauge := collector.RegisterGauge("test_gauge")
	
	gauge.Set(42.5)
	assert.Equal(t, 42.5, gauge.Get())
	
	gauge.Set(100.0)
	assert.Equal(t, 100.0, gauge.Get())
}

func TestHistogram_Observe(t *testing.T) {
	collector := NewMetricsCollector(nil)
	histogram := collector.RegisterHistogram("test_hist")
	
	histogram.Observe(1.0)
	histogram.Observe(2.0)
	histogram.Observe(3.0)
	
	assert.Len(t, histogram.Values, 3)
}

func TestHistogram_GetStats(t *testing.T) {
	collector := NewMetricsCollector(nil)
	histogram := collector.RegisterHistogram("test_stats")
	
	histogram.Observe(1.0)
	histogram.Observe(2.0)
	histogram.Observe(3.0)
	
	count, sum, avg, min, max := histogram.GetStats()
	assert.Equal(t, 3, count)
	assert.InDelta(t, 6.0, sum, 0.001)
	assert.InDelta(t, 2.0, avg, 0.001)
	assert.InDelta(t, 1.0, min, 0.001)
	assert.InDelta(t, 3.0, max, 0.001)
}

func TestMetricsCollector_GetSnapshot(t *testing.T) {
	collector := NewMetricsCollector(nil)
	
	counter := collector.RegisterCounter("snap_counter")
	gauge := collector.RegisterGauge("snap_gauge")
	histogram := collector.RegisterHistogram("snap_hist")
	
	counter.Inc(10)
	gauge.Set(50.5)
	histogram.Observe(1.0)
	
	snapshot := collector.GetSnapshot()
	assert.NotNil(t, snapshot)
	assert.NotZero(t, snapshot.Timestamp)
	assert.Equal(t, int64(10), snapshot.Counters["snap_counter"])
	assert.Equal(t, 50.5, snapshot.Gauges["snap_gauge"])
	assert.NotNil(t, snapshot.Histograms["snap_hist"])
}

func TestMetricsCollector_RecordMetrics(t *testing.T) {
	collector := NewMetricsCollector(nil)
	
	collector.RecordTaskCreated()
	collector.RecordTaskCompleted()
	collector.RecordTaskFailed()
	collector.RecordTaskDuration(100 * time.Millisecond)
	collector.SetExecutorsOnline(3)
	collector.SetExecutorsOffline(1)
	collector.RecordWorkflowCreated()
	collector.SetWorkflowRunning(2)
	
	snapshot := collector.GetSnapshot()
	
	assert.Equal(t, int64(1), snapshot.Counters[MetricTasksCreated])
	assert.Equal(t, int64(1), snapshot.Counters[MetricTasksCompleted])
	assert.Equal(t, int64(1), snapshot.Counters[MetricTasksFailed])
	assert.Equal(t, int64(1), snapshot.Counters[MetricWorkflowCreated])
	assert.Equal(t, 3.0, snapshot.Gauges[MetricExecutorsOnline])
	assert.Equal(t, 1.0, snapshot.Gauges[MetricExecutorsOffline])
	assert.Equal(t, 2.0, snapshot.Gauges[MetricWorkflowRunning])
	assert.NotNil(t, snapshot.Histograms[MetricTaskDuration])
}

func TestMetricsCollector_SaveToRedis(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()
	
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	
	collector := NewMetricsCollector(client)
	collector.RecordTaskCreated()
	
	err = collector.SaveToRedis(context.Background())
	assert.NoError(t, err)
}
