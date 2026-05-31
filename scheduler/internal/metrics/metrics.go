package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type MetricsCollector struct {
	redis          *redis.Client
	metrics        map[string]*Counter
	gauges         map[string]*Gauge
	histograms     map[string]*Histogram
	mu             sync.RWMutex
	collectionInterval time.Duration
}

type Counter struct {
	Name  string
	Value int64
	mu    sync.Mutex
}

type Gauge struct {
	Name  string
	Value float64
	mu    sync.Mutex
}

type Histogram struct {
	Name   string
	Values []float64
	mu     sync.Mutex
}

type MetricsSnapshot struct {
	Timestamp   int64                  `json:"timestamp"`
	Counters    map[string]int64       `json:"counters"`
	Gauges      map[string]float64     `json:"gauges"`
	Histograms  map[string]interface{} `json:"histograms"`
}

var (
	globalCollector *MetricsCollector
	once            sync.Once
)

func NewMetricsCollector(redisClient *redis.Client) *MetricsCollector {
	once.Do(func() {
		globalCollector = &MetricsCollector{
			redis:              redisClient,
			metrics:            make(map[string]*Counter),
			gauges:             make(map[string]*Gauge),
			histograms:         make(map[string]*Histogram),
			collectionInterval: 60 * time.Second,
		}
	})
	return globalCollector
}

func GetGlobalCollector() *MetricsCollector {
	return globalCollector
}

func (m *MetricsCollector) RegisterCounter(name string) *Counter {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.metrics[name]; !exists {
		m.metrics[name] = &Counter{Name: name}
	}
	return m.metrics[name]
}

func (m *MetricsCollector) RegisterGauge(name string) *Gauge {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.gauges[name]; !exists {
		m.gauges[name] = &Gauge{Name: name}
	}
	return m.gauges[name]
}

func (m *MetricsCollector) RegisterHistogram(name string) *Histogram {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.histograms[name]; !exists {
		m.histograms[name] = &Histogram{Name: name}
	}
	return m.histograms[name]
}

func (c *Counter) Inc(delta int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Value += delta
}

func (c *Counter) Get() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Value
}

func (g *Gauge) Set(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Value = value
}

func (g *Gauge) Get() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.Value
}

func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Values = append(h.Values, value)
	if len(h.Values) > 1000 {
		h.Values = h.Values[1:]
	}
}

func (h *Histogram) GetStats() (count int, sum float64, avg float64, min float64, max float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	count = len(h.Values)
	if count == 0 {
		return
	}
	
	sum = 0
	min = h.Values[0]
	max = h.Values[0]
	
	for _, v := range h.Values {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	avg = sum / float64(count)
	
	return
}

func (m *MetricsCollector) GetSnapshot() *MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := &MetricsSnapshot{
		Timestamp:  time.Now().Unix(),
		Counters:   make(map[string]int64),
		Gauges:     make(map[string]float64),
		Histograms: make(map[string]interface{}),
	}

	for name, counter := range m.metrics {
		snapshot.Counters[name] = counter.Get()
	}

	for name, gauge := range m.gauges {
		snapshot.Gauges[name] = gauge.Get()
	}

	for name, histogram := range m.histograms {
		count, sum, avg, min, max := histogram.GetStats()
		snapshot.Histograms[name] = map[string]interface{}{
			"count": count,
			"sum":   sum,
			"avg":   avg,
			"min":   min,
			"max":   max,
		}
	}

	return snapshot
}

func (m *MetricsCollector) SaveToRedis(ctx context.Context) error {
	snapshot := m.GetSnapshot()
	
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	key := fmt.Sprintf("bdopsflow:metrics:%d", snapshot.Timestamp)
	if err := m.redis.Set(ctx, key, data, 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to save metrics to redis: %w", err)
	}

	return nil
}

func (m *MetricsCollector) StartBackgroundCollection(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(m.collectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := m.SaveToRedis(ctx); err != nil {
				slog.Error("Failed to save metrics", "error", err)
				}
			}
		}
	}()
}

const (
	MetricTasksCreated       = "bdopsflow:bdopsflow_tasks:created"
	MetricTasksCompleted    = "bdopsflow:bdopsflow_tasks:completed"
	MetricTasksFailed       = "bdopsflow:bdopsflow_tasks:failed"
	MetricTasksRunning      = "bdopsflow:bdopsflow_tasks:running"
	MetricExecutorsOnline   = "bdopsflow:bdopsflow_executors:online"
	MetricExecutorsOffline  = "bdopsflow:bdopsflow_executors:offline"
	MetricTaskDuration      = "bdopsflow:task:duration_seconds"
)

func (m *MetricsCollector) RecordTaskCreated() {
	m.RegisterCounter(MetricTasksCreated).Inc(1)
}

func (m *MetricsCollector) RecordTaskCompleted() {
	m.RegisterCounter(MetricTasksCompleted).Inc(1)
}

func (m *MetricsCollector) RecordTaskFailed() {
	m.RegisterCounter(MetricTasksFailed).Inc(1)
}

func (m *MetricsCollector) RecordTaskDuration(duration time.Duration) {
	m.RegisterHistogram(MetricTaskDuration).Observe(duration.Seconds())
}

func (m *MetricsCollector) SetExecutorsOnline(count int64) {
	m.RegisterGauge(MetricExecutorsOnline).Set(float64(count))
}

func (m *MetricsCollector) SetExecutorsOffline(count int64) {
	m.RegisterGauge(MetricExecutorsOffline).Set(float64(count))
}
