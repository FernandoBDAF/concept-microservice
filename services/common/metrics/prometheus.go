package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// prometheusCounter implements prometheus.Collector for Counter metrics
type prometheusCounter struct {
	metric Counter
	desc   *prometheus.Desc
}

// NewPrometheusCounter creates a new Prometheus collector for Counter metrics
func NewPrometheusCounter(metric Counter) prometheus.Collector {
	return &prometheusCounter{
		metric: metric,
		desc: prometheus.NewDesc(
			metric.Name(),
			metric.Help(),
			metric.Labels(),
			nil,
		),
	}
}

// Describe implements prometheus.Collector
func (c *prometheusCounter) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

// Collect implements prometheus.Collector
func (c *prometheusCounter) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		c.desc,
		prometheus.CounterValue,
		c.metric.Get(),
	)
}

// prometheusGauge implements prometheus.Collector for Gauge metrics
type prometheusGauge struct {
	metric Gauge
	desc   *prometheus.Desc
}

// NewPrometheusGauge creates a new Prometheus collector for Gauge metrics
func NewPrometheusGauge(metric Gauge) prometheus.Collector {
	return &prometheusGauge{
		metric: metric,
		desc: prometheus.NewDesc(
			metric.Name(),
			metric.Help(),
			metric.Labels(),
			nil,
		),
	}
}

// Describe implements prometheus.Collector
func (g *prometheusGauge) Describe(ch chan<- *prometheus.Desc) {
	ch <- g.desc
}

// Collect implements prometheus.Collector
func (g *prometheusGauge) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		g.desc,
		prometheus.GaugeValue,
		g.metric.Get(),
	)
}

// prometheusHistogram implements prometheus.Collector for Histogram metrics
type prometheusHistogram struct {
	metric Histogram
	desc   *prometheus.Desc
}

// NewPrometheusHistogram creates a new Prometheus collector for Histogram metrics
func NewPrometheusHistogram(metric Histogram) prometheus.Collector {
	return &prometheusHistogram{
		metric: metric,
		desc: prometheus.NewDesc(
			metric.Name(),
			metric.Help(),
			metric.Labels(),
			nil,
		),
	}
}

// Describe implements prometheus.Collector
func (h *prometheusHistogram) Describe(ch chan<- *prometheus.Desc) {
	ch <- h.desc
}

// Collect implements prometheus.Collector
func (h *prometheusHistogram) Collect(ch chan<- prometheus.Metric) {
	stats := h.metric.Get()

	// Send count metric
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			h.metric.Name()+"_count",
			h.metric.Help()+" (count)",
			h.metric.Labels(),
			nil,
		),
		prometheus.CounterValue,
		stats["count"],
	)

	// Send sum metric
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			h.metric.Name()+"_sum",
			h.metric.Help()+" (sum)",
			h.metric.Labels(),
			nil,
		),
		prometheus.CounterValue,
		stats["sum"],
	)

	// Send percentile metrics
	for _, p := range []struct {
		name string
		key  string
	}{
		{"p50", "p50"},
		{"p90", "p90"},
		{"p95", "p95"},
		{"p99", "p99"},
	} {
		if value, ok := stats[p.key]; ok {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(
					h.metric.Name()+"_"+p.name,
					h.metric.Help()+" ("+p.name+")",
					h.metric.Labels(),
					nil,
				),
				prometheus.GaugeValue,
				value,
			)
		}
	}
}

// prometheusTimer implements prometheus.Collector for Timer metrics
type prometheusTimer struct {
	metric Timer
	desc   *prometheus.Desc
}

// NewPrometheusTimer creates a new Prometheus collector for Timer metrics
func NewPrometheusTimer(metric Timer) prometheus.Collector {
	return &prometheusTimer{
		metric: metric,
		desc: prometheus.NewDesc(
			metric.Name(),
			metric.Help(),
			metric.Labels(),
			nil,
		),
	}
}

// Describe implements prometheus.Collector
func (t *prometheusTimer) Describe(ch chan<- *prometheus.Desc) {
	ch <- t.desc
}

// Collect implements prometheus.Collector
func (t *prometheusTimer) Collect(ch chan<- prometheus.Metric) {
	stats := t.metric.Get()

	// Send count metric
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			t.metric.Name()+"_count",
			t.metric.Help()+" (count)",
			t.metric.Labels(),
			nil,
		),
		prometheus.CounterValue,
		stats["count"],
	)

	// Send sum metric
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			t.metric.Name()+"_sum",
			t.metric.Help()+" (sum)",
			t.metric.Labels(),
			nil,
		),
		prometheus.CounterValue,
		stats["sum_seconds"],
	)

	// Send average metric
	if avg, ok := stats["avg_seconds"]; ok {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(
				t.metric.Name()+"_avg",
				t.metric.Help()+" (average)",
				t.metric.Labels(),
				nil,
			),
			prometheus.GaugeValue,
			avg,
		)
	}
}

// Registry manages metric registration and collection
type Registry struct {
	Registry *prometheus.Registry
	metrics  []prometheus.Collector
	mu       sync.RWMutex
}

// NewRegistry creates a new metric registry
func NewRegistry() *Registry {
	return &Registry{
		Registry: prometheus.NewRegistry(),
		metrics:  make([]prometheus.Collector, 0),
	}
}

// RegisterCounter registers a Counter metric
func (r *Registry) RegisterCounter(metric Counter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	collector := NewPrometheusCounter(metric)
	r.metrics = append(r.metrics, collector)
	r.Registry.MustRegister(collector)
}

// RegisterGauge registers a Gauge metric
func (r *Registry) RegisterGauge(metric Gauge) {
	r.mu.Lock()
	defer r.mu.Unlock()
	collector := NewPrometheusGauge(metric)
	r.metrics = append(r.metrics, collector)
	r.Registry.MustRegister(collector)
}

// RegisterHistogram registers a Histogram metric
func (r *Registry) RegisterHistogram(metric Histogram) {
	r.mu.Lock()
	defer r.mu.Unlock()
	collector := NewPrometheusHistogram(metric)
	r.metrics = append(r.metrics, collector)
	r.Registry.MustRegister(collector)
}

// RegisterTimer registers a Timer metric
func (r *Registry) RegisterTimer(metric Timer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	collector := NewPrometheusTimer(metric)
	r.metrics = append(r.metrics, collector)
	r.Registry.MustRegister(collector)
}

// Collectors returns all registered collectors
func (r *Registry) Collectors() []prometheus.Collector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.metrics
}

// Gather implements prometheus.Gatherer
func (r *Registry) Gather() ([]*dto.MetricFamily, error) {
	return r.Registry.Gather()
}

// Global registry instance
var DefaultRegistry = NewRegistry()

// prometheusCounterAdapter adapts a prometheus.Counter to our Counter interface
type prometheusCounterAdapter struct {
	counter prometheus.Counter
}

// NewPrometheusCounterAdapter creates a new adapter for prometheus.Counter
func NewPrometheusCounterAdapter(counter prometheus.Counter) Counter {
	return &prometheusCounterAdapter{counter: counter}
}

// Type implements Metric
func (a *prometheusCounterAdapter) Type() MetricType {
	return CounterType
}

// Name implements Metric
func (a *prometheusCounterAdapter) Name() string {
	return "prometheus_counter"
}

// Help implements Metric
func (a *prometheusCounterAdapter) Help() string {
	return "Prometheus counter adapter"
}

// Labels implements Metric
func (a *prometheusCounterAdapter) Labels() []string {
	return nil
}

// Inc implements Counter
func (a *prometheusCounterAdapter) Inc() {
	a.counter.Inc()
}

// Add implements Counter
func (a *prometheusCounterAdapter) Add(value float64) {
	a.counter.Add(value)
}

// Get implements Counter
func (a *prometheusCounterAdapter) Get() float64 {
	return 0 // Prometheus counters don't support getting the current value
}

// prometheusGaugeAdapter adapts a prometheus.Gauge to our Gauge interface
type prometheusGaugeAdapter struct {
	gauge prometheus.Gauge
}

// NewPrometheusGaugeAdapter creates a new adapter for prometheus.Gauge
func NewPrometheusGaugeAdapter(gauge prometheus.Gauge) Gauge {
	return &prometheusGaugeAdapter{gauge: gauge}
}

// Type implements Metric
func (a *prometheusGaugeAdapter) Type() MetricType {
	return GaugeType
}

// Name implements Metric
func (a *prometheusGaugeAdapter) Name() string {
	return "prometheus_gauge"
}

// Help implements Metric
func (a *prometheusGaugeAdapter) Help() string {
	return "Prometheus gauge adapter"
}

// Labels implements Metric
func (a *prometheusGaugeAdapter) Labels() []string {
	return nil
}

// Set implements Gauge
func (a *prometheusGaugeAdapter) Set(value float64) {
	a.gauge.Set(value)
}

// Inc implements Gauge
func (a *prometheusGaugeAdapter) Inc() {
	a.gauge.Inc()
}

// Dec implements Gauge
func (a *prometheusGaugeAdapter) Dec() {
	a.gauge.Dec()
}

// Add implements Gauge
func (a *prometheusGaugeAdapter) Add(value float64) {
	a.gauge.Add(value)
}

// Sub implements Gauge
func (a *prometheusGaugeAdapter) Sub(value float64) {
	a.gauge.Sub(value)
}

// Get implements Gauge
func (a *prometheusGaugeAdapter) Get() float64 {
	return 0 // Prometheus gauges don't support getting the current value
}

// prometheusHistogramAdapter adapts a prometheus.Histogram to our Histogram interface
type prometheusHistogramAdapter struct {
	histogram prometheus.Histogram
}

// NewPrometheusHistogramAdapter creates a new adapter for prometheus.Histogram
func NewPrometheusHistogramAdapter(histogram prometheus.Histogram) Histogram {
	return &prometheusHistogramAdapter{histogram: histogram}
}

// Type implements Metric
func (a *prometheusHistogramAdapter) Type() MetricType {
	return HistogramType
}

// Name implements Metric
func (a *prometheusHistogramAdapter) Name() string {
	return "prometheus_histogram"
}

// Help implements Metric
func (a *prometheusHistogramAdapter) Help() string {
	return "Prometheus histogram adapter"
}

// Labels implements Metric
func (a *prometheusHistogramAdapter) Labels() []string {
	return nil
}

// Observe implements Histogram
func (a *prometheusHistogramAdapter) Observe(value float64) {
	a.histogram.Observe(value)
}

// Get implements Histogram
func (a *prometheusHistogramAdapter) Get() map[string]float64 {
	return map[string]float64{
		"count": 0, // Prometheus histograms don't support getting current values
		"sum":   0,
	}
}

// prometheusCounterVecAdapter adapts a prometheus.CounterVec to our Counter interface
type prometheusCounterVecAdapter struct {
	counterVec *prometheus.CounterVec
	labels     []string
}

// NewPrometheusCounterVecAdapter creates a new adapter for prometheus.CounterVec
func NewPrometheusCounterVecAdapter(counterVec *prometheus.CounterVec, labels []string) Counter {
	return &prometheusCounterVecAdapter{
		counterVec: counterVec,
		labels:     labels,
	}
}

// Type implements Metric
func (a *prometheusCounterVecAdapter) Type() MetricType {
	return CounterType
}

// Name implements Metric
func (a *prometheusCounterVecAdapter) Name() string {
	return "prometheus_counter_vec"
}

// Help implements Metric
func (a *prometheusCounterVecAdapter) Help() string {
	return "Prometheus counter vector adapter"
}

// Labels implements Metric
func (a *prometheusCounterVecAdapter) Labels() []string {
	return a.labels
}

// Inc implements Counter
func (a *prometheusCounterVecAdapter) Inc() {
	a.counterVec.WithLabelValues(a.labels...).Inc()
}

// Add implements Counter
func (a *prometheusCounterVecAdapter) Add(value float64) {
	a.counterVec.WithLabelValues(a.labels...).Add(value)
}

// Get implements Counter
func (a *prometheusCounterVecAdapter) Get() float64 {
	return 0 // Prometheus counters don't support getting the current value
}

// prometheusHistogramVecAdapter adapts a prometheus.HistogramVec to our Histogram interface
type prometheusHistogramVecAdapter struct {
	histogramVec *prometheus.HistogramVec
	labels       []string
}

// NewPrometheusHistogramVecAdapter creates a new adapter for prometheus.HistogramVec
func NewPrometheusHistogramVecAdapter(histogramVec *prometheus.HistogramVec, labels []string) Histogram {
	return &prometheusHistogramVecAdapter{
		histogramVec: histogramVec,
		labels:       labels,
	}
}

// Type implements Metric
func (a *prometheusHistogramVecAdapter) Type() MetricType {
	return HistogramType
}

// Name implements Metric
func (a *prometheusHistogramVecAdapter) Name() string {
	return "prometheus_histogram_vec"
}

// Help implements Metric
func (a *prometheusHistogramVecAdapter) Help() string {
	return "Prometheus histogram vector adapter"
}

// Labels implements Metric
func (a *prometheusHistogramVecAdapter) Labels() []string {
	return a.labels
}

// Observe implements Histogram
func (a *prometheusHistogramVecAdapter) Observe(value float64) {
	a.histogramVec.WithLabelValues(a.labels...).Observe(value)
}

// Get implements Histogram
func (a *prometheusHistogramVecAdapter) Get() map[string]float64 {
	return map[string]float64{
		"count": 0, // Prometheus histograms don't support getting current values
		"sum":   0,
	}
}

// prometheusGaugeVecAdapter adapts prometheus.GaugeVec to the Gauge interface
type prometheusGaugeVecAdapter struct {
	baseMetric
	gauge *prometheus.GaugeVec
}

// NewPrometheusGaugeVecAdapter creates a new prometheusGaugeVecAdapter
func NewPrometheusGaugeVecAdapter(gauge *prometheus.GaugeVec, labels []string) Gauge {
	return &prometheusGaugeVecAdapter{
		baseMetric: baseMetric{
			name:   "gauge_vec",           // Prometheus doesn't expose the name directly
			help:   "Gauge vector metric", // Prometheus doesn't expose the help text directly
			labels: labels,
			mType:  GaugeType,
		},
		gauge: gauge,
	}
}

// Set sets the gauge to the given value
func (a *prometheusGaugeVecAdapter) Set(value float64) {
	a.gauge.WithLabelValues().Set(value)
}

// Inc increments the gauge by 1
func (a *prometheusGaugeVecAdapter) Inc() {
	a.gauge.WithLabelValues().Inc()
}

// Dec decrements the gauge by 1
func (a *prometheusGaugeVecAdapter) Dec() {
	a.gauge.WithLabelValues().Dec()
}

// Add adds the given value to the gauge
func (a *prometheusGaugeVecAdapter) Add(value float64) {
	a.gauge.WithLabelValues().Add(value)
}

// Sub subtracts the given value from the gauge
func (a *prometheusGaugeVecAdapter) Sub(value float64) {
	a.gauge.WithLabelValues().Sub(value)
}

// Get returns the current gauge value
func (a *prometheusGaugeVecAdapter) Get() float64 {
	// Prometheus doesn't provide a way to get the current value
	// This is a limitation of the Prometheus client library
	return 0
}
