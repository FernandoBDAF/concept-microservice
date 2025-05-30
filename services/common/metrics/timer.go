package metrics

import (
	"sync"
	"time"
)

// timer implements the Timer interface
type timer struct {
	baseMetric
	mu       sync.RWMutex
	start    time.Time
	duration time.Duration
	count    uint64
	sum      time.Duration
}

// NewTimer creates a new timer metric
func NewTimer(name, help string, labels []string) Timer {
	return &timer{
		baseMetric: baseMetric{
			name:   name,
			help:   help,
			labels: labels,
			mType:  TimerType,
		},
	}
}

// Start starts the timer
func (t *timer) Start() Timer {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.start = time.Now()
	return t
}

// Stop stops the timer and records the duration
func (t *timer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.start.IsZero() {
		t.duration = time.Since(t.start)
		t.sum += t.duration
		t.count++
		t.start = time.Time{} // Reset start time
	}
}

// Get returns the current timer statistics
func (t *timer) Get() map[string]float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := make(map[string]float64)
	stats["count"] = float64(t.count)
	stats["sum_seconds"] = t.sum.Seconds()

	if t.count > 0 {
		stats["avg_seconds"] = t.sum.Seconds() / float64(t.count)
	}

	return stats
}
