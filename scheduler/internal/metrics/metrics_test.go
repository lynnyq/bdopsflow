package metrics

import (
	"testing"
	"time"
)

func TestCounter_Inc(t *testing.T) {
	counter := &Counter{Name: "test", Value: 0}

	counter.Inc(1)
	if counter.Get() != 1 {
		t.Errorf("Expected 1, got %d", counter.Get())
	}

	counter.Inc(5)
	if counter.Get() != 6 {
		t.Errorf("Expected 6, got %d", counter.Get())
	}

	counter.Inc(-3)
	if counter.Get() != 3 {
		t.Errorf("Expected 3, got %d", counter.Get())
	}
}

func TestGauge_Set(t *testing.T) {
	gauge := &Gauge{Name: "test", Value: 0}

	gauge.Set(10.5)
	if gauge.Get() != 10.5 {
		t.Errorf("Expected 10.5, got %f", gauge.Get())
	}

	gauge.Set(0)
	if gauge.Get() != 0 {
		t.Errorf("Expected 0, got %f", gauge.Get())
	}
}

func TestHistogram_Observe(t *testing.T) {
	histogram := &Histogram{Name: "test", Values: []float64{}}

	histogram.Observe(10.0)
	histogram.Observe(20.0)
	histogram.Observe(30.0)

	count, sum, avg, min, max := histogram.GetStats()

	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
	if sum != 60.0 {
		t.Errorf("Expected sum 60.0, got %f", sum)
	}
	if avg != 20.0 {
		t.Errorf("Expected avg 20.0, got %f", avg)
	}
	if min != 10.0 {
		t.Errorf("Expected min 10.0, got %f", min)
	}
	if max != 30.0 {
		t.Errorf("Expected max 30.0, got %f", max)
	}
}

func TestHistogram_Empty(t *testing.T) {
	histogram := &Histogram{Name: "test", Values: []float64{}}

	count, sum, avg, min, max := histogram.GetStats()

	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}
	if sum != 0 {
		t.Errorf("Expected sum 0, got %f", sum)
	}
	if avg != 0 {
		t.Errorf("Expected avg 0, got %f", avg)
	}
	if min != 0 {
		t.Errorf("Expected min 0, got %f", min)
	}
	if max != 0 {
		t.Errorf("Expected max 0, got %f", max)
	}
}

func TestMetricsCollector_RegisterCounter(t *testing.T) {
	collector := &MetricsCollector{
		metrics: make(map[string]*Counter),
	}

	counter := collector.RegisterCounter("test_counter")
	counter.Inc(10)

	if counter.Value != 10 {
		t.Errorf("Expected counter value 10, got %d", counter.Value)
	}

	sameCounter := collector.RegisterCounter("test_counter")
	if sameCounter != counter {
		t.Error("Should return same counter instance")
	}
}

func TestMetricsCollector_RegisterGauge(t *testing.T) {
	collector := &MetricsCollector{
		gauges: make(map[string]*Gauge),
	}

	gauge := collector.RegisterGauge("test_gauge")
	gauge.Set(25.5)

	if gauge.Value != 25.5 {
		t.Errorf("Expected gauge value 25.5, got %f", gauge.Value)
	}
}

func TestMetricsCollector_RegisterHistogram(t *testing.T) {
	collector := &MetricsCollector{
		histograms: make(map[string]*Histogram),
	}

	histogram := collector.RegisterHistogram("test_histogram")
	histogram.Observe(100.0)

	if len(histogram.Values) != 1 {
		t.Errorf("Expected 1 value, got %d", len(histogram.Values))
	}
}

func TestMetricsCollector_GetSnapshot(t *testing.T) {
	collector := &MetricsCollector{
		metrics:    make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}

	counter := collector.RegisterCounter("requests")
	counter.Inc(100)

	gauge := collector.RegisterGauge("cpu_usage")
	gauge.Set(75.5)

	histogram := collector.RegisterHistogram("latency")
	histogram.Observe(0.5)
	histogram.Observe(1.0)
	histogram.Observe(1.5)

	snapshot := collector.GetSnapshot()

	if snapshot.Counters["requests"] != 100 {
		t.Errorf("Expected counter 'requests' to be 100, got %d", snapshot.Counters["requests"])
	}

	if snapshot.Gauges["cpu_usage"] != 75.5 {
		t.Errorf("Expected gauge 'cpu_usage' to be 75.5, got %f", snapshot.Gauges["cpu_usage"])
	}

	latencyStats, ok := snapshot.Histograms["latency"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected latency histogram to be map")
	}
	if latencyStats["count"].(int) != 3 {
		t.Errorf("Expected latency count to be 3, got %v", latencyStats["count"])
	}
}

func TestMetricsCollector_MetricsConstants(t *testing.T) {
	if MetricTasksCreated != "bdopsflow:bdopsflow_tasks:created" {
		t.Errorf("Unexpected MetricTasksCreated value")
	}
	if MetricTasksCompleted != "bdopsflow:bdopsflow_tasks:completed" {
		t.Errorf("Unexpected MetricTasksCompleted value")
	}
	if MetricExecutorsOnline != "bdopsflow:bdopsflow_executors:online" {
		t.Errorf("Unexpected MetricExecutorsOnline value")
	}
}

func TestMetricsCollector_RecordTaskCreated(t *testing.T) {
	collector := &MetricsCollector{
		metrics:    make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}

	collector.RecordTaskCreated()
	collector.RecordTaskCreated()
	collector.RecordTaskCreated()

	if collector.metrics[MetricTasksCreated].Get() != 3 {
		t.Errorf("Expected 3 task creations, got %d", collector.metrics[MetricTasksCreated].Get())
	}
}

func TestMetricsCollector_RecordTaskDuration(t *testing.T) {
	collector := &MetricsCollector{
		metrics:    make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}

	collector.RecordTaskDuration(100 * time.Millisecond)
	collector.RecordTaskDuration(200 * time.Millisecond)

	histogram := collector.histograms[MetricTaskDuration]
	count, _, _, _, _ := histogram.GetStats()

	if count != 2 {
		t.Errorf("Expected 2 observations, got %d", count)
	}
}

func TestMetricsCollector_SetExecutorsOnline(t *testing.T) {
	collector := &MetricsCollector{
		metrics:    make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}

	collector.SetExecutorsOnline(5)

	if collector.gauges[MetricExecutorsOnline].Get() != 5 {
		t.Errorf("Expected 5 online bdopsflow_executors, got %f", collector.gauges[MetricExecutorsOnline].Get())
	}
}

func TestMetricsSnapshot_Structure(t *testing.T) {
	snapshot := &MetricsSnapshot{
		Timestamp:  time.Now().Unix(),
		Counters:   make(map[string]int64),
		Gauges:     make(map[string]float64),
		Histograms: make(map[string]interface{}),
	}

	snapshot.Counters["test"] = 42
	snapshot.Gauges["test_gauge"] = 3.14
	snapshot.Histograms["test_hist"] = map[string]interface{}{
		"count": 1,
		"sum":   10.0,
	}

	if snapshot.Counters["test"] != 42 {
		t.Errorf("Expected counter value 42, got %d", snapshot.Counters["test"])
	}
}
