package metrics

import (
	"sync"
	"time"
)

// StorageMetrics tracks metrics for storage operations
type StorageMetrics struct {
	mu sync.RWMutex

	// Operation counts
	GetProfileCount    int64
	GetProfilesCount   int64
	CreateProfileCount int64
	UpdateProfileCount int64
	DeleteProfileCount int64
	ErrorCount         int64

	// Latency measurements
	GetProfileLatency    time.Duration
	GetProfilesLatency   time.Duration
	CreateProfileLatency time.Duration
	UpdateProfileLatency time.Duration
	DeleteProfileLatency time.Duration

	// Last operation timestamps
	LastGetProfile    time.Time
	LastGetProfiles   time.Time
	LastCreateProfile time.Time
	LastUpdateProfile time.Time
	LastDeleteProfile time.Time
	LastError         time.Time
}

var (
	metrics = &StorageMetrics{}
)

// GetMetrics returns the current metrics
func GetMetrics() *StorageMetrics {
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()
	return metrics
}

// RecordGetProfile records a GetProfile operation
func RecordGetProfile(duration time.Duration) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.GetProfileCount++
	metrics.GetProfileLatency = duration
	metrics.LastGetProfile = time.Now()
}

// RecordGetProfiles records a GetProfiles operation
func RecordGetProfiles(duration time.Duration) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.GetProfilesCount++
	metrics.GetProfilesLatency = duration
	metrics.LastGetProfiles = time.Now()
}

// RecordCreateProfile records a CreateProfile operation
func RecordCreateProfile(duration time.Duration) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.CreateProfileCount++
	metrics.CreateProfileLatency = duration
	metrics.LastCreateProfile = time.Now()
}

// RecordUpdateProfile records an UpdateProfile operation
func RecordUpdateProfile(duration time.Duration) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.UpdateProfileCount++
	metrics.UpdateProfileLatency = duration
	metrics.LastUpdateProfile = time.Now()
}

// RecordDeleteProfile records a DeleteProfile operation
func RecordDeleteProfile(duration time.Duration) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.DeleteProfileCount++
	metrics.DeleteProfileLatency = duration
	metrics.LastDeleteProfile = time.Now()
}

// RecordError records an error occurrence
func RecordError() {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.ErrorCount++
	metrics.LastError = time.Now()
}

// Reset resets all metrics to zero
func Reset() {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics = &StorageMetrics{}
}
