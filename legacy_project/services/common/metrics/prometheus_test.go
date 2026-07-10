package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPrometheusCounter(t *testing.T) {
	counter := NewCounter("test_counter", "Test counter", []string{"test"})
	collector := NewPrometheusCounter(counter)

	// Test Describe
	descCh := make(chan *prometheus.Desc, 1)
	collector.Describe(descCh)
	desc := <-descCh
	if desc == nil {
		t.Error("Expected non-nil description")
	}

	// Test Collect
	counter.Inc()
	counter.Add(5)

	metricCh := make(chan prometheus.Metric, 1)
	collector.Collect(metricCh)
	metric := <-metricCh
	if metric == nil {
		t.Error("Expected non-nil metric")
	}
}

func TestPrometheusGauge(t *testing.T) {
	gauge := NewGauge("test_gauge", "Test gauge", []string{"test"})
	collector := NewPrometheusGauge(gauge)

	// Test Describe
	descCh := make(chan *prometheus.Desc, 1)
	collector.Describe(descCh)
	desc := <-descCh
	if desc == nil {
		t.Error("Expected non-nil description")
	}

	// Test Collect
	gauge.Set(10)
	gauge.Inc()

	metricCh := make(chan prometheus.Metric, 1)
	collector.Collect(metricCh)
	metric := <-metricCh
	if metric == nil {
		t.Error("Expected non-nil metric")
	}
}

func TestPrometheusHistogram(t *testing.T) {
	buckets := []float64{1, 5, 10, 50, 100}
	histogram := NewHistogram("test_histogram", "Test histogram", []string{"test"}, buckets)
	collector := NewPrometheusHistogram(histogram)

	// Test Describe
	descCh := make(chan *prometheus.Desc, 1)
	collector.Describe(descCh)
	desc := <-descCh
	if desc == nil {
		t.Error("Expected non-nil description")
	}

	// Test Collect
	histogram.Observe(3)
	histogram.Observe(7)
	histogram.Observe(25)
	histogram.Observe(75)

	metricCh := make(chan prometheus.Metric, 6) // count, sum, p50, p90, p95, p99
	collector.Collect(metricCh)

	// Verify we got all expected metrics
	metrics := make([]prometheus.Metric, 0)
	for i := 0; i < 6; i++ {
		metric := <-metricCh
		if metric == nil {
			t.Errorf("Expected non-nil metric at index %d", i)
		}
		metrics = append(metrics, metric)
	}
}

func TestPrometheusTimer(t *testing.T) {
	timer := NewTimer("test_timer", "Test timer", []string{"test"})
	collector := NewPrometheusTimer(timer)

	// Test Describe
	descCh := make(chan *prometheus.Desc, 1)
	collector.Describe(descCh)
	desc := <-descCh
	if desc == nil {
		t.Error("Expected non-nil description")
	}

	// Test Collect
	timer.Start()
	timer.Stop()

	metricCh := make(chan prometheus.Metric, 3) // count, sum, avg
	collector.Collect(metricCh)

	// Verify we got all expected metrics
	metrics := make([]prometheus.Metric, 0)
	for i := 0; i < 3; i++ {
		metric := <-metricCh
		if metric == nil {
			t.Errorf("Expected non-nil metric at index %d", i)
		}
		metrics = append(metrics, metric)
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	// Register metrics
	counter := NewCounter("test_counter", "Test counter", []string{"test"})
	gauge := NewGauge("test_gauge", "Test gauge", []string{"test"})
	buckets := []float64{1, 5, 10, 50, 100}
	histogram := NewHistogram("test_histogram", "Test histogram", []string{"test"}, buckets)
	timer := NewTimer("test_timer", "Test timer", []string{"test"})

	registry.RegisterCounter(counter)
	registry.RegisterGauge(gauge)
	registry.RegisterHistogram(histogram)
	registry.RegisterTimer(timer)

	// Verify collectors
	collectors := registry.Collectors()
	if len(collectors) != 4 {
		t.Errorf("Expected 4 collectors, got %d", len(collectors))
	}
}
