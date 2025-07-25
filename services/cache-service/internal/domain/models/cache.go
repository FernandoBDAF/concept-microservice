package models

import (
	"time"
)

// CacheItem represents a single cache entry
type CacheItem struct {
	Key       string        `json:"key"`
	Value     []byte        `json:"value"`
	TTL       time.Duration `json:"ttl"`
	CreatedAt time.Time     `json:"created_at"`
	ExpiresAt *time.Time    `json:"expires_at,omitempty"`
}

// CacheStats represents cache statistics
type CacheStats struct {
	Hits        int64     `json:"hits"`
	Misses      int64     `json:"misses"`
	Evictions   int64     `json:"evictions"`
	TotalKeys   int64     `json:"total_keys"`
	UsedMemory  int64     `json:"used_memory"`
	HitRatio    float64   `json:"hit_ratio"`
	LastUpdated time.Time `json:"last_updated"`
}

// BatchItem represents an item in a batch operation
type BatchItem struct {
	Key   string        `json:"key"`
	Value []byte        `json:"value"`
	TTL   time.Duration `json:"ttl"`
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}

// Profile represents user profile data for caching
type Profile struct {
	ID        string                 `json:"id"`
	Email     string                 `json:"email"`
	Username  string                 `json:"username"`
	FullName  string                 `json:"full_name"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// TaskStatus represents task status information
type TaskStatus struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	Progress    float64                `json:"progress"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// QueueMetrics represents queue performance metrics
type QueueMetrics struct {
	QueueName       string        `json:"queue_name"`
	TotalJobs       int64         `json:"total_jobs"`
	PendingJobs     int64         `json:"pending_jobs"`
	ProcessingJobs  int64         `json:"processing_jobs"`
	CompletedJobs   int64         `json:"completed_jobs"`
	FailedJobs      int64         `json:"failed_jobs"`
	AverageWaitTime time.Duration `json:"average_wait_time"`
	LastUpdated     time.Time     `json:"last_updated"`
}

// WorkerStatus represents worker status information
type WorkerStatus struct {
	WorkerType     string    `json:"worker_type"`
	ActiveWorkers  int       `json:"active_workers"`
	IdleWorkers    int       `json:"idle_workers"`
	TotalWorkers   int       `json:"total_workers"`
	ProcessingJobs int       `json:"processing_jobs"`
	CompletedJobs  int64     `json:"completed_jobs"`
	FailedJobs     int64     `json:"failed_jobs"`
	LastHeartbeat  time.Time `json:"last_heartbeat"`
}

// Session represents user session data
type Session struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	DeviceID  string                 `json:"device_id,omitempty"`
	IPAddress string                 `json:"ip_address"`
	UserAgent string                 `json:"user_agent,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	LastUsed  time.Time              `json:"last_used"`
	ExpiresAt time.Time              `json:"expires_at"`
}

// HealthStatus represents service health status
type HealthStatus struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]bool   `json:"checks"`
	Details   map[string]string `json:"details,omitempty"`
}

// CircuitBreakerStatus represents circuit breaker status
type CircuitBreakerStatus struct {
	Name        string    `json:"name"`
	State       string    `json:"state"`
	Failures    uint32    `json:"failures"`
	Requests    uint32    `json:"requests"`
	LastFailure time.Time `json:"last_failure,omitempty"`
	NextRetry   time.Time `json:"next_retry,omitempty"`
}
