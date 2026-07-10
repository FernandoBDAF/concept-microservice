package metrics

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
)

// DefaultHistogramBuckets are the default histogram buckets
var DefaultHistogramBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

// histogram implements the Histogram interface
type histogram struct {
	baseMetric
	values  []float64
	buckets []float64
	count   uint64
	sum     uint64 // Store sum as uint64 for atomic operations
	mu      sync.RWMutex
}

// NewHistogram creates a new histogram metric
func NewHistogram(name, help string, labels []string, buckets []float64) Histogram {
	if buckets == nil {
		buckets = DefaultHistogramBuckets
	}
	// Ensure buckets are sorted
	sort.Float64s(buckets)
	return &histogram{
		baseMetric: baseMetric{
			name:   name,
			help:   help,
			labels: labels,
			mType:  HistogramType,
		},
		values:  make([]float64, 0),
		buckets: buckets,
	}
}

// Observe records a value in the histogram
func (h *histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.values = append(h.values, value)
	atomic.AddUint64(&h.count, 1)
	atomic.AddUint64(&h.sum, uint64(value))
}

// Get returns the current histogram statistics
func (h *histogram) Get() map[string]float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.values) == 0 {
		return map[string]float64{
			"count": 0,
			"sum":   0,
		}
	}

	// Calculate percentiles
	sort.Float64s(h.values)
	count := float64(len(h.values))
	sum := float64(atomic.LoadUint64(&h.sum))

	stats := map[string]float64{
		"count": count,
		"sum":   sum,
		"min":   h.values[0],
		"max":   h.values[len(h.values)-1],
		"avg":   sum / count,
	}

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
		index := int(math.Ceil(p.p*count)) - 1
		if index >= 0 && index < len(h.values) {
			stats[p.name] = h.values[index]
		}
	}

	// Calculate bucket counts
	for _, bucket := range h.buckets {
		count := 0
		for _, v := range h.values {
			if v <= bucket {
				count++
			}
		}
		stats[formatBucketKey(bucket)] = float64(count)
	}

	return stats
}

// formatBucketKey formats a bucket value as a string key
func formatBucketKey(bucket float64) string {
	return "bucket_" + formatFloat(bucket)
}

// formatFloat formats a float value as a string
func formatFloat(f float64) string {
	if f == math.Inf(1) {
		return "inf"
	}
	if f == math.Inf(-1) {
		return "-inf"
	}
	return formatFloat64(f)
}

// formatFloat64 formats a float64 value as a string
func formatFloat64(f float64) string {
	if f == 0 {
		return "0"
	}
	if f < 0.001 || f >= 1000 {
		return formatScientific(f)
	}
	return formatDecimal(f)
}

// formatScientific formats a float64 value in scientific notation
func formatScientific(f float64) string {
	return formatFloat64(f)
}

// formatDecimal formats a float64 value in decimal notation
func formatDecimal(f float64) string {
	return formatFloat64(f)
}
