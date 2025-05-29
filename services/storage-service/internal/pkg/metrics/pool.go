package metrics

import (
	"sync/atomic"
	"time"
)

// PoolMetrics tracks connection pool statistics
type PoolMetrics struct {
	// Connection counts
	openConnections   int64
	idleConnections   int64
	waitingRequests   int64
	connectionErrors  int64
	retryAttempts     int64
	successfulRetries int64

	// Timing metrics
	lastAcquisitionTime time.Time
	acquisitionLatency  time.Duration
}

// NewPoolMetrics creates a new pool metrics instance
func NewPoolMetrics() *PoolMetrics {
	return &PoolMetrics{
		lastAcquisitionTime: time.Now(),
	}
}

// IncrementOpenConnections increments the open connections counter
func (pm *PoolMetrics) IncrementOpenConnections() {
	atomic.AddInt64(&pm.openConnections, 1)
}

// DecrementOpenConnections decrements the open connections counter
func (pm *PoolMetrics) DecrementOpenConnections() {
	atomic.AddInt64(&pm.openConnections, -1)
}

// SetIdleConnections sets the idle connections count
func (pm *PoolMetrics) SetIdleConnections(count int64) {
	atomic.StoreInt64(&pm.idleConnections, count)
}

// IncrementWaitingRequests increments the waiting requests counter
func (pm *PoolMetrics) IncrementWaitingRequests() {
	atomic.AddInt64(&pm.waitingRequests, 1)
}

// DecrementWaitingRequests decrements the waiting requests counter
func (pm *PoolMetrics) DecrementWaitingRequests() {
	atomic.AddInt64(&pm.waitingRequests, -1)
}

// IncrementConnectionErrors increments the connection errors counter
func (pm *PoolMetrics) IncrementConnectionErrors() {
	atomic.AddInt64(&pm.connectionErrors, 1)
}

// IncrementRetryAttempts increments the retry attempts counter
func (pm *PoolMetrics) IncrementRetryAttempts() {
	atomic.AddInt64(&pm.retryAttempts, 1)
}

// IncrementSuccessfulRetries increments the successful retries counter
func (pm *PoolMetrics) IncrementSuccessfulRetries() {
	atomic.AddInt64(&pm.successfulRetries, 1)
}

// RecordAcquisitionTime records the time taken to acquire a connection
func (pm *PoolMetrics) RecordAcquisitionTime(duration time.Duration) {
	pm.acquisitionLatency = duration
	pm.lastAcquisitionTime = time.Now()
}

// GetMetrics returns the current pool metrics
func (pm *PoolMetrics) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"open_connections":    atomic.LoadInt64(&pm.openConnections),
		"idle_connections":    atomic.LoadInt64(&pm.idleConnections),
		"waiting_requests":    atomic.LoadInt64(&pm.waitingRequests),
		"connection_errors":   atomic.LoadInt64(&pm.connectionErrors),
		"retry_attempts":      atomic.LoadInt64(&pm.retryAttempts),
		"successful_retries":  atomic.LoadInt64(&pm.successfulRetries),
		"acquisition_latency": pm.acquisitionLatency,
		"last_acquisition":    pm.lastAcquisitionTime,
		"pool_saturation":     float64(atomic.LoadInt64(&pm.openConnections)) / float64(atomic.LoadInt64(&pm.idleConnections)),
		"retry_success_rate":  float64(atomic.LoadInt64(&pm.successfulRetries)) / float64(atomic.LoadInt64(&pm.retryAttempts)),
	}
}
