// Package metrics is a deliberately tiny, dependency-free Prometheus
// exposition (text format 0.0.4) implementation — just enough for the host
// contract's /metrics clause (HOST_CONTRACT.md §1.2). Hand-rolled on purpose:
// the guest must prove the contract can be met with zero external deps.
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
)

// Counter is a monotonically increasing uint64.
type Counter struct {
	v atomic.Uint64
}

// Inc adds 1.
func (c *Counter) Inc() { c.v.Add(1) }

// Add adds n.
func (c *Counter) Add(n uint64) { c.v.Add(n) }

// Value returns the current count.
func (c *Counter) Value() uint64 { return c.v.Load() }

// GaugeFunc is evaluated at scrape time.
type GaugeFunc func() float64

// Histogram is a fixed-bucket cumulative histogram.
type Histogram struct {
	mu      sync.Mutex
	bounds  []float64 // upper bounds, ascending; +Inf implicit
	counts  []uint64  // per-bound (non-cumulative) counts; len(bounds)+1
	sum     float64
	total   uint64
}

// NewHistogram returns a histogram with the given ascending upper bounds.
func NewHistogram(bounds []float64) *Histogram {
	b := append([]float64(nil), bounds...)
	sort.Float64s(b)
	return &Histogram{bounds: b, counts: make([]uint64, len(b)+1)}
}

// Observe records a single observation.
func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	i := sort.SearchFloat64s(h.bounds, v) // first bound >= v
	h.counts[i]++
	h.sum += v
	h.total++
}

type metric struct {
	name  string
	help  string
	typ   string
	write func(w io.Writer, name string)
}

// Registry renders registered metrics in registration order.
type Registry struct {
	mu      sync.Mutex
	metrics []metric
}

func NewRegistry() *Registry { return &Registry{} }

// Counter registers and returns a new counter. Prometheus convention: name
// ends in _total.
func (r *Registry) Counter(name, help string) *Counter {
	c := &Counter{}
	r.add(metric{name, help, "counter", func(w io.Writer, n string) {
		fmt.Fprintf(w, "%s %d\n", n, c.Value())
	}})
	return c
}

// GaugeFunc registers a gauge computed at scrape time.
func (r *Registry) GaugeFunc(name, help string, f GaugeFunc) {
	r.add(metric{name, help, "gauge", func(w io.Writer, n string) {
		fmt.Fprintf(w, "%s %s\n", n, formatFloat(f()))
	}})
}

// Histogram registers and returns a new histogram with the given bounds.
func (r *Registry) Histogram(name, help string, bounds []float64) *Histogram {
	h := NewHistogram(bounds)
	r.add(metric{name, help, "histogram", func(w io.Writer, n string) {
		h.mu.Lock()
		defer h.mu.Unlock()
		var cum uint64
		for i, b := range h.bounds {
			cum += h.counts[i]
			fmt.Fprintf(w, "%s_bucket{le=%q} %d\n", n, formatFloat(b), cum)
		}
		cum += h.counts[len(h.bounds)]
		fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n", n, cum)
		fmt.Fprintf(w, "%s_sum %s\n", n, formatFloat(h.sum))
		fmt.Fprintf(w, "%s_count %d\n", n, h.total)
	}})
	return h
}

func (r *Registry) add(m metric) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = append(r.metrics, m)
}

// Write renders the exposition text format.
func (r *Registry) Write(w io.Writer) {
	r.mu.Lock()
	ms := append([]metric(nil), r.metrics...)
	r.mu.Unlock()
	for _, m := range ms {
		fmt.Fprintf(w, "# HELP %s %s\n", m.name, m.help)
		fmt.Fprintf(w, "# TYPE %s %s\n", m.name, m.typ)
		m.write(w, m.name)
	}
}

// Handler serves the registry at /metrics.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		r.Write(w)
	})
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
