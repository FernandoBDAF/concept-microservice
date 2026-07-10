package metrics

import (
	"sync/atomic"
	"time"
)

var (
	userOperationCount   int64
	userOperationErrors  int64
	userOperationLatency int64
)

// RecordUserOperation records metrics for a user operation
func RecordUserOperation(operation string, duration time.Duration) {
	atomic.AddInt64(&userOperationCount, 1)
	atomic.AddInt64(&userOperationLatency, int64(duration))
}

// RecordUserError records a user operation error
func RecordUserError(operation string) {
	atomic.AddInt64(&userOperationErrors, 1)
}

// GetUserMetrics returns the current user operation metrics
func GetUserMetrics() map[string]int64 {
	return map[string]int64{
		"operation_count":   atomic.LoadInt64(&userOperationCount),
		"operation_errors":  atomic.LoadInt64(&userOperationErrors),
		"operation_latency": atomic.LoadInt64(&userOperationLatency),
	}
}

// ResetUserMetrics resets all user operation metrics
func ResetUserMetrics() {
	atomic.StoreInt64(&userOperationCount, 0)
	atomic.StoreInt64(&userOperationErrors, 0)
	atomic.StoreInt64(&userOperationLatency, 0)
}
