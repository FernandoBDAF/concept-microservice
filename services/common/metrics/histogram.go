package metrics

import (
	"math"
	"sort"
	"sync"
)

// histogram implements the Histogram interface
type histogram struct {
	baseMetric
	mu      sync.RWMutex
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
}

// NewHistogram creates a new histogram metric with the given buckets
func NewHistogram(name, help string, labels []string, buckets []float64) Histogram {
	// Sort buckets to ensure they are in ascending order
	sortedBuckets := make([]float64, len(buckets))
	copy(sortedBuckets, buckets)
	sort.Float64s(sortedBuckets)

	return &histogram{
		baseMetric: baseMetric{
			name:   name,
			help:   help,
			labels: labels,
			mType:  HistogramType,
		},
		buckets: sortedBuckets,
		counts:  make([]uint64, len(sortedBuckets)),
	}
}

// Observe records a value in the histogram
func (h *histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sum += value
	h.count++

	// Find the appropriate bucket for the value
	for i, bucket := range h.buckets {
		if value <= bucket {
			h.counts[i]++
		}
	}
}

// Get returns the current histogram statistics
func (h *histogram) Get() map[string]float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := make(map[string]float64)
	stats["sum"] = h.sum
	stats["count"] = float64(h.count)

	if h.count > 0 {
		stats["avg"] = h.sum / float64(h.count)
	}

	// Calculate percentiles if we have data
	if h.count > 0 {
		values := make([]float64, 0, h.count)
		for i, bucket := range h.buckets {
			for j := uint64(0); j < h.counts[i]; j++ {
				values = append(values, bucket)
			}
		}
		sort.Float64s(values)

		stats["p50"] = percentile(values, 50)
		stats["p90"] = percentile(values, 90)
		stats["p95"] = percentile(values, 95)
		stats["p99"] = percentile(values, 99)
	}

	return stats
}

// percentile calculates the given percentile from the sorted values
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	index := int(math.Ceil(float64(len(values))*p/100)) - 1
	if index < 0 {
		index = 0
	}
	return values[index]
}
