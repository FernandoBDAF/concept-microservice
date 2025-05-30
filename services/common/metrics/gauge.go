package metrics

import (
	"sync/atomic"
)

// gauge implements the Gauge interface
type gauge struct {
	baseMetric
	value uint64
}

// NewGauge creates a new gauge metric
func NewGauge(name, help string, labels []string) Gauge {
	return &gauge{
		baseMetric: baseMetric{
			name:   name,
			help:   help,
			labels: labels,
			mType:  GaugeType,
		},
		value: 0,
	}
}

// Set sets the gauge to the given value
func (g *gauge) Set(value float64) {
	atomic.StoreUint64(&g.value, uint64(value))
}

// Inc increments the gauge by 1
func (g *gauge) Inc() {
	atomic.AddUint64(&g.value, 1)
}

// Dec decrements the gauge by 1
func (g *gauge) Dec() {
	atomic.AddUint64(&g.value, ^uint64(0))
}

// Add adds the given value to the gauge
func (g *gauge) Add(value float64) {
	atomic.AddUint64(&g.value, uint64(value))
}

// Sub subtracts the given value from the gauge
func (g *gauge) Sub(value float64) {
	atomic.AddUint64(&g.value, ^uint64(uint64(value)-1))
}

// Get returns the current gauge value
func (g *gauge) Get() float64 {
	return float64(atomic.LoadUint64(&g.value))
}
