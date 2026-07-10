package metrics

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// timer implements the Timer interface
type timer struct {
	baseMetric
	startTime time.Time
	durations []time.Duration
	count     uint64
	sum       uint64 // Store sum in nanoseconds for atomic operations
	mu        sync.RWMutex
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
		durations: make([]time.Duration, 0),
	}
}

// Start starts the timer
func (t *timer) Start() Timer {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.startTime = time.Now()
	return t
}

// Stop stops the timer and records the duration
func (t *timer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.startTime.IsZero() {
		duration := time.Since(t.startTime)
		t.durations = append(t.durations, duration)
		atomic.AddUint64(&t.count, 1)
		atomic.AddUint64(&t.sum, uint64(duration))
		t.startTime = time.Time{} // Reset start time
	}
}

// Get returns the current timer statistics
func (t *timer) Get() map[string]float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.durations) == 0 {
		return map[string]float64{
			"count": 0,
			"sum":   0,
		}
	}

	// Calculate statistics
	count := float64(atomic.LoadUint64(&t.count))
	sum := float64(atomic.LoadUint64(&t.sum)) / float64(time.Second) // Convert to seconds

	stats := map[string]float64{
		"count":       count,
		"sum_seconds": sum,
		"avg_seconds": sum / count,
	}

	// Calculate percentiles
	durations := make([]float64, len(t.durations))
	for i, d := range t.durations {
		durations[i] = float64(d) / float64(time.Second)
	}

	// Sort durations for percentile calculation
	sort.Float64s(durations)

	// Calculate percentiles
	percentiles := []struct {
		name string
		p    float64
	}{
		{"p50", 0.5},
		{"p90", 0.9},
		{"p95", 0.95},
		{"p99", 0.99},
	}

	for _, p := range percentiles {
		index := int(float64(len(durations)) * p.p)
		if index >= 0 && index < len(durations) {
			stats[p.name+"_seconds"] = durations[index]
		}
	}

	return stats
}
