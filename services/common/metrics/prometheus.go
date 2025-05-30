package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
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
	metrics []prometheus.Collector
}

// NewRegistry creates a new metric registry
func NewRegistry() *Registry {
	return &Registry{
		metrics: make([]prometheus.Collector, 0),
	}
}

// RegisterCounter registers a Counter metric
func (r *Registry) RegisterCounter(metric Counter) {
	r.metrics = append(r.metrics, NewPrometheusCounter(metric))
}

// RegisterGauge registers a Gauge metric
func (r *Registry) RegisterGauge(metric Gauge) {
	r.metrics = append(r.metrics, NewPrometheusGauge(metric))
}

// RegisterHistogram registers a Histogram metric
func (r *Registry) RegisterHistogram(metric Histogram) {
	r.metrics = append(r.metrics, NewPrometheusHistogram(metric))
}

// RegisterTimer registers a Timer metric
func (r *Registry) RegisterTimer(metric Timer) {
	r.metrics = append(r.metrics, NewPrometheusTimer(metric))
}

// Collectors returns all registered collectors
func (r *Registry) Collectors() []prometheus.Collector {
	return r.metrics
}
