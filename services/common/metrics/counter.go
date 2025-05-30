package metrics

import (
	"sync/atomic"
)

// counter implements the Counter interface
type counter struct {
	baseMetric
	value uint64
}

// NewCounter creates a new counter metric
func NewCounter(name, help string, labels []string) Counter {
	return &counter{
		baseMetric: baseMetric{
			name:   name,
			help:   help,
			labels: labels,
			mType:  CounterType,
		},
		value: 0,
	}
}

// Inc increments the counter by 1
func (c *counter) Inc() {
	atomic.AddUint64(&c.value, 1)
}

// Add increments the counter by the given value
func (c *counter) Add(value float64) {
	if value < 0 {
		return // Counters can only increase
	}
	atomic.AddUint64(&c.value, uint64(value))
}

// Get returns the current counter value
func (c *counter) Get() float64 {
	return float64(atomic.LoadUint64(&c.value))
}
