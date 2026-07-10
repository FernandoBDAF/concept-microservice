package performance

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/pkg/logger"
)

// OptimizationManager handles performance optimization across the storage service
type OptimizationManager struct {
	db                   *sqlx.DB
	connectionPool       *ConnectionPoolOptimizer
	queryOptimizer       *QueryOptimizer
	resourceMonitor      *ResourceMonitor
	performanceCollector *PerformanceCollector
	log                  *zap.Logger
	config               *OptimizationConfig
	mu                   sync.RWMutex
}

// OptimizationConfig holds configuration for performance optimization
type OptimizationConfig struct {
	// Connection Pool Settings
	MaxOpenConnections    int           `json:"max_open_connections"`
	MaxIdleConnections    int           `json:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `json:"connection_max_lifetime"`
	ConnectionMaxIdleTime time.Duration `json:"connection_max_idle_time"`

	// Query Optimization Settings
	EnableQueryCache   bool          `json:"enable_query_cache"`
	QueryCacheSize     int           `json:"query_cache_size"`
	QueryCacheTTL      time.Duration `json:"query_cache_ttl"`
	SlowQueryThreshold time.Duration `json:"slow_query_threshold"`

	// Resource Monitoring Settings
	MonitoringInterval  time.Duration `json:"monitoring_interval"`
	MemoryThresholdMB   int64         `json:"memory_threshold_mb"`
	CPUThresholdPercent float64       `json:"cpu_threshold_percent"`

	// Auto-tuning Settings
	EnableAutoTuning     bool          `json:"enable_auto_tuning"`
	AutoTuningInterval   time.Duration `json:"auto_tuning_interval"`
	PerformanceTargetP95 time.Duration `json:"performance_target_p95"`
}

// ConnectionPoolOptimizer optimizes database connection pool settings
type ConnectionPoolOptimizer struct {
	db               *sqlx.DB
	config           *OptimizationConfig
	metrics          *ConnectionPoolMetrics
	log              *zap.Logger
	lastOptimization time.Time
	mu               sync.RWMutex
}

// ConnectionPoolMetrics tracks connection pool performance
type ConnectionPoolMetrics struct {
	OpenConnections      int           `json:"open_connections"`
	IdleConnections      int           `json:"idle_connections"`
	InUseConnections     int           `json:"in_use_connections"`
	WaitCount            int64         `json:"wait_count"`
	WaitDuration         time.Duration `json:"wait_duration"`
	MaxIdleClosed        int64         `json:"max_idle_closed"`
	MaxIdleTimeClosed    int64         `json:"max_idle_time_closed"`
	MaxLifetimeClosed    int64         `json:"max_lifetime_closed"`
	ConnectionCreateTime time.Duration `json:"avg_connection_create_time"`
	LastOptimizedAt      time.Time     `json:"last_optimized_at"`
}

// QueryOptimizer provides query optimization and caching
type QueryOptimizer struct {
	cache       *QueryCache
	config      *OptimizationConfig
	slowQueries map[string]*SlowQueryInfo
	queryStats  map[string]*QueryStats
	log         *zap.Logger
	mu          sync.RWMutex
}

// QueryCache implements a simple in-memory query result cache
type QueryCache struct {
	entries   map[string]*CacheEntry
	maxSize   int
	ttl       time.Duration
	hitCount  int64
	missCount int64
	mu        sync.RWMutex
}

// CacheEntry represents a cached query result
type CacheEntry struct {
	Key            string      `json:"key"`
	Result         interface{} `json:"result"`
	CreatedAt      time.Time   `json:"created_at"`
	AccessCount    int64       `json:"access_count"`
	LastAccessedAt time.Time   `json:"last_accessed_at"`
}

// SlowQueryInfo tracks information about slow queries
type SlowQueryInfo struct {
	Query           string        `json:"query"`
	Count           int64         `json:"count"`
	TotalDuration   time.Duration `json:"total_duration"`
	AverageDuration time.Duration `json:"average_duration"`
	MaxDuration     time.Duration `json:"max_duration"`
	FirstSeenAt     time.Time     `json:"first_seen_at"`
	LastSeenAt      time.Time     `json:"last_seen_at"`
}

// QueryStats tracks general query statistics
type QueryStats struct {
	Query           string        `json:"query"`
	ExecutionCount  int64         `json:"execution_count"`
	TotalDuration   time.Duration `json:"total_duration"`
	AverageDuration time.Duration `json:"average_duration"`
	CacheHits       int64         `json:"cache_hits"`
	CacheMisses     int64         `json:"cache_misses"`
}

// ResourceMonitor monitors system resource usage
type ResourceMonitor struct {
	config  *OptimizationConfig
	metrics *ResourceMetrics
	alerts  []ResourceAlert
	log     *zap.Logger
	stopCh  chan struct{}
	mu      sync.RWMutex
}

// ResourceMetrics tracks system resource usage
type ResourceMetrics struct {
	MemoryUsageMB      int64         `json:"memory_usage_mb"`
	MemoryUsagePercent float64       `json:"memory_usage_percent"`
	CPUUsagePercent    float64       `json:"cpu_usage_percent"`
	GoroutineCount     int           `json:"goroutine_count"`
	GCPauseTime        time.Duration `json:"gc_pause_time"`
	HeapObjects        uint64        `json:"heap_objects"`
	HeapSizeBytes      uint64        `json:"heap_size_bytes"`
	LastUpdated        time.Time     `json:"last_updated"`
}

// ResourceAlert represents a resource usage alert
type ResourceAlert struct {
	Type         string    `json:"type"`
	Message      string    `json:"message"`
	Severity     string    `json:"severity"` // "warning", "critical"
	Threshold    float64   `json:"threshold"`
	CurrentValue float64   `json:"current_value"`
	Timestamp    time.Time `json:"timestamp"`
}

// PerformanceCollector collects and aggregates performance metrics
type PerformanceCollector struct {
	samples []PerformanceSample
	config  *OptimizationConfig
	log     *zap.Logger
	mu      sync.RWMutex
}

// PerformanceSample represents a performance measurement sample
type PerformanceSample struct {
	Timestamp     time.Time       `json:"timestamp"`
	Operation     string          `json:"operation"`
	Duration      time.Duration   `json:"duration"`
	Success       bool            `json:"success"`
	ResourceUsage ResourceMetrics `json:"resource_usage"`
}

// NewOptimizationManager creates a new performance optimization manager
func NewOptimizationManager(db *sqlx.DB) *OptimizationManager {
	config := &OptimizationConfig{
		MaxOpenConnections:    25,
		MaxIdleConnections:    10,
		ConnectionMaxLifetime: 5 * time.Minute,
		ConnectionMaxIdleTime: 2 * time.Minute,
		EnableQueryCache:      true,
		QueryCacheSize:        1000,
		QueryCacheTTL:         5 * time.Minute,
		SlowQueryThreshold:    1 * time.Second,
		MonitoringInterval:    30 * time.Second,
		MemoryThresholdMB:     512,
		CPUThresholdPercent:   80.0,
		EnableAutoTuning:      true,
		AutoTuningInterval:    5 * time.Minute,
		PerformanceTargetP95:  500 * time.Millisecond,
	}

	manager := &OptimizationManager{
		db:     db,
		config: config,
		log:    logger.Get().Named("performance_optimization"),
		performanceCollector: &PerformanceCollector{
			samples: make([]PerformanceSample, 0, 1000),
			config:  config,
			log:     logger.Get().Named("performance_collector"),
		},
	}

	// Initialize components
	manager.connectionPool = NewConnectionPoolOptimizer(db, config)
	manager.queryOptimizer = NewQueryOptimizer(config)
	manager.resourceMonitor = NewResourceMonitor(config)

	return manager
}

// Start begins performance optimization and monitoring
func (om *OptimizationManager) Start(ctx context.Context) error {
	om.log.Info("Starting performance optimization manager")

	// Optimize connection pool
	if err := om.connectionPool.OptimizeConnectionPool(); err != nil {
		om.log.Error("Failed to optimize connection pool", logger.ErrorField(err))
		return err
	}

	// Start resource monitoring
	go om.resourceMonitor.StartMonitoring(ctx)

	// Start auto-tuning if enabled
	if om.config.EnableAutoTuning {
		go om.startAutoTuning(ctx)
	}

	om.log.Info("Performance optimization manager started successfully")
	return nil
}

// startAutoTuning runs periodic auto-tuning of performance parameters
func (om *OptimizationManager) startAutoTuning(ctx context.Context) {
	ticker := time.NewTicker(om.config.AutoTuningInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			om.log.Info("Auto-tuning stopped")
			return
		case <-ticker.C:
			om.performAutoTuning()
		}
	}
}

// performAutoTuning analyzes performance and adjusts parameters
func (om *OptimizationManager) performAutoTuning() {
	om.log.Debug("Performing auto-tuning")

	// Get current performance metrics
	resourceMetrics := om.resourceMonitor.GetMetrics()
	poolMetrics := om.connectionPool.GetMetrics()

	// Analyze and adjust connection pool
	if om.shouldAdjustConnectionPool(resourceMetrics, poolMetrics) {
		if err := om.connectionPool.AutoTuneConnectionPool(resourceMetrics); err != nil {
			om.log.Error("Failed to auto-tune connection pool", logger.ErrorField(err))
		} else {
			om.log.Info("Auto-tuned connection pool settings")
		}
	}

	// Analyze and adjust query cache
	if om.shouldAdjustQueryCache() {
		om.queryOptimizer.AutoTuneCache()
		om.log.Info("Auto-tuned query cache settings")
	}

	om.log.Debug("Auto-tuning completed")
}

// shouldAdjustConnectionPool determines if connection pool needs adjustment
func (om *OptimizationManager) shouldAdjustConnectionPool(resourceMetrics *ResourceMetrics, poolMetrics *ConnectionPoolMetrics) bool {
	// Adjust if memory usage is high and we have many open connections
	if resourceMetrics.MemoryUsageMB > om.config.MemoryThresholdMB && poolMetrics.OpenConnections > 15 {
		return true
	}

	// Adjust if we're frequently waiting for connections
	if poolMetrics.WaitCount > 100 && poolMetrics.WaitDuration > 100*time.Millisecond {
		return true
	}

	return false
}

// shouldAdjustQueryCache determines if query cache needs adjustment
func (om *OptimizationManager) shouldAdjustQueryCache() bool {
	hitRate := om.queryOptimizer.GetCacheHitRate()

	// Adjust if hit rate is too low (< 70%) or too high (> 95%, might indicate stale data)
	return hitRate < 0.70 || hitRate > 0.95
}

// RecordPerformanceSample records a performance measurement
func (om *OptimizationManager) RecordPerformanceSample(operation string, duration time.Duration, success bool) {
	om.performanceCollector.RecordSample(operation, duration, success, om.resourceMonitor.GetMetrics())
}

// GetOptimizationReport returns a comprehensive optimization report
func (om *OptimizationManager) GetOptimizationReport() *OptimizationReport {
	om.mu.RLock()
	defer om.mu.RUnlock()

	return &OptimizationReport{
		Timestamp:        time.Now(),
		ConnectionPool:   om.connectionPool.GetMetrics(),
		ResourceUsage:    om.resourceMonitor.GetMetrics(),
		QueryStats:       om.queryOptimizer.GetQueryStats(),
		CacheStats:       om.queryOptimizer.GetCacheStats(),
		Recommendations:  om.generateRecommendations(),
		PerformanceTrend: om.performanceCollector.GetTrend(),
	}
}

// OptimizationReport contains comprehensive optimization information
type OptimizationReport struct {
	Timestamp        time.Time              `json:"timestamp"`
	ConnectionPool   *ConnectionPoolMetrics `json:"connection_pool"`
	ResourceUsage    *ResourceMetrics       `json:"resource_usage"`
	QueryStats       map[string]*QueryStats `json:"query_stats"`
	CacheStats       *CacheStats            `json:"cache_stats"`
	Recommendations  []string               `json:"recommendations"`
	PerformanceTrend string                 `json:"performance_trend"`
}

// CacheStats provides query cache statistics
type CacheStats struct {
	HitCount         int64   `json:"hit_count"`
	MissCount        int64   `json:"miss_count"`
	HitRate          float64 `json:"hit_rate"`
	EntryCount       int     `json:"entry_count"`
	MaxSize          int     `json:"max_size"`
	MemoryUsageBytes int64   `json:"memory_usage_bytes"`
}

// generateRecommendations generates optimization recommendations
func (om *OptimizationManager) generateRecommendations() []string {
	var recommendations []string

	// Connection pool recommendations
	poolMetrics := om.connectionPool.GetMetrics()
	if poolMetrics.WaitCount > 50 {
		recommendations = append(recommendations, "Consider increasing max open connections - frequent connection waits detected")
	}

	if poolMetrics.MaxIdleClosed > 100 {
		recommendations = append(recommendations, "Consider reducing max idle connections - many idle connections are being closed")
	}

	// Resource usage recommendations
	resourceMetrics := om.resourceMonitor.GetMetrics()
	if resourceMetrics.MemoryUsageMB > om.config.MemoryThresholdMB {
		recommendations = append(recommendations, "Memory usage is high - consider optimizing queries or increasing memory limits")
	}

	if resourceMetrics.CPUUsagePercent > om.config.CPUThresholdPercent {
		recommendations = append(recommendations, "CPU usage is high - consider adding more processing capacity or optimizing algorithms")
	}

	// Query cache recommendations
	hitRate := om.queryOptimizer.GetCacheHitRate()
	if hitRate < 0.5 {
		recommendations = append(recommendations, "Query cache hit rate is low - consider increasing cache size or TTL")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Performance is within acceptable ranges - no immediate optimizations needed")
	}

	return recommendations
}

// Helper function implementations for the components would go here
// NewConnectionPoolOptimizer, NewQueryOptimizer, NewResourceMonitor, etc.

// Simplified implementations for the individual components:

// NewConnectionPoolOptimizer creates a new connection pool optimizer
func NewConnectionPoolOptimizer(db *sqlx.DB, config *OptimizationConfig) *ConnectionPoolOptimizer {
	return &ConnectionPoolOptimizer{
		db:      db,
		config:  config,
		metrics: &ConnectionPoolMetrics{},
		log:     logger.Get().Named("connection_pool_optimizer"),
	}
}

// OptimizeConnectionPool optimizes database connection pool settings
func (cpo *ConnectionPoolOptimizer) OptimizeConnectionPool() error {
	cpo.log.Info("Optimizing connection pool settings",
		logger.Int("max_open_connections", cpo.config.MaxOpenConnections),
		logger.Int("max_idle_connections", cpo.config.MaxIdleConnections),
	)

	cpo.db.SetMaxOpenConns(cpo.config.MaxOpenConnections)
	cpo.db.SetMaxIdleConns(cpo.config.MaxIdleConnections)
	cpo.db.SetConnMaxLifetime(cpo.config.ConnectionMaxLifetime)
	cpo.db.SetConnMaxIdleTime(cpo.config.ConnectionMaxIdleTime)

	cpo.lastOptimization = time.Now()
	return nil
}

// AutoTuneConnectionPool automatically adjusts connection pool based on metrics
func (cpo *ConnectionPoolOptimizer) AutoTuneConnectionPool(resourceMetrics *ResourceMetrics) error {
	cpo.mu.Lock()
	defer cpo.mu.Unlock()

	// Simple auto-tuning logic
	currentStats := cpo.db.Stats()

	if resourceMetrics.MemoryUsageMB > 400 && currentStats.OpenConnections > 15 {
		// Reduce connections if memory usage is high
		newMax := cpo.config.MaxOpenConnections - 5
		if newMax < 5 {
			newMax = 5
		}
		cpo.config.MaxOpenConnections = newMax
		cpo.db.SetMaxOpenConns(newMax)
		cpo.log.Info("Reduced max open connections due to high memory usage", logger.Int("new_max", newMax))
	} else if currentStats.WaitCount > 10 && cpo.config.MaxOpenConnections < 50 {
		// Increase connections if we're waiting frequently
		newMax := cpo.config.MaxOpenConnections + 5
		cpo.config.MaxOpenConnections = newMax
		cpo.db.SetMaxOpenConns(newMax)
		cpo.log.Info("Increased max open connections due to frequent waits", logger.Int("new_max", newMax))
	}

	return nil
}

// GetMetrics returns current connection pool metrics
func (cpo *ConnectionPoolOptimizer) GetMetrics() *ConnectionPoolMetrics {
	stats := cpo.db.Stats()
	return &ConnectionPoolMetrics{
		OpenConnections:   stats.OpenConnections,
		IdleConnections:   stats.Idle,
		InUseConnections:  stats.InUse,
		WaitCount:         stats.WaitCount,
		WaitDuration:      stats.WaitDuration,
		MaxIdleClosed:     stats.MaxIdleClosed,
		MaxIdleTimeClosed: stats.MaxIdleTimeClosed,
		MaxLifetimeClosed: stats.MaxLifetimeClosed,
		LastOptimizedAt:   cpo.lastOptimization,
	}
}

// NewQueryOptimizer creates a new query optimizer
func NewQueryOptimizer(config *OptimizationConfig) *QueryOptimizer {
	return &QueryOptimizer{
		cache: &QueryCache{
			entries: make(map[string]*CacheEntry),
			maxSize: config.QueryCacheSize,
			ttl:     config.QueryCacheTTL,
		},
		config:      config,
		slowQueries: make(map[string]*SlowQueryInfo),
		queryStats:  make(map[string]*QueryStats),
		log:         logger.Get().Named("query_optimizer"),
	}
}

// GetCacheHitRate returns the cache hit rate
func (qo *QueryOptimizer) GetCacheHitRate() float64 {
	qo.mu.RLock()
	defer qo.mu.RUnlock()

	if qo.cache.hitCount+qo.cache.missCount == 0 {
		return 0.0
	}
	return float64(qo.cache.hitCount) / float64(qo.cache.hitCount+qo.cache.missCount)
}

// AutoTuneCache automatically adjusts cache settings based on performance
func (qo *QueryOptimizer) AutoTuneCache() {
	// Simple auto-tuning logic for cache
	hitRate := qo.GetCacheHitRate()

	if hitRate < 0.5 && qo.cache.maxSize < 2000 {
		qo.cache.maxSize = qo.cache.maxSize + 200
		qo.log.Info("Increased cache size due to low hit rate", logger.Int("new_size", qo.cache.maxSize))
	} else if hitRate > 0.95 && qo.cache.maxSize > 500 {
		qo.cache.maxSize = qo.cache.maxSize - 100
		qo.log.Info("Decreased cache size due to very high hit rate", logger.Int("new_size", qo.cache.maxSize))
	}
}

// GetQueryStats returns query statistics
func (qo *QueryOptimizer) GetQueryStats() map[string]*QueryStats {
	qo.mu.RLock()
	defer qo.mu.RUnlock()

	// Return a copy to avoid concurrent access issues
	stats := make(map[string]*QueryStats)
	for k, v := range qo.queryStats {
		stats[k] = v
	}
	return stats
}

// GetCacheStats returns cache statistics
func (qo *QueryOptimizer) GetCacheStats() *CacheStats {
	qo.mu.RLock()
	defer qo.mu.RUnlock()

	return &CacheStats{
		HitCount:   qo.cache.hitCount,
		MissCount:  qo.cache.missCount,
		HitRate:    qo.GetCacheHitRate(),
		EntryCount: len(qo.cache.entries),
		MaxSize:    qo.cache.maxSize,
	}
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(config *OptimizationConfig) *ResourceMonitor {
	return &ResourceMonitor{
		config:  config,
		metrics: &ResourceMetrics{},
		log:     logger.Get().Named("resource_monitor"),
		stopCh:  make(chan struct{}),
	}
}

// StartMonitoring begins resource monitoring
func (rm *ResourceMonitor) StartMonitoring(ctx context.Context) {
	ticker := time.NewTicker(rm.config.MonitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			rm.log.Info("Resource monitoring stopped")
			return
		case <-rm.stopCh:
			return
		case <-ticker.C:
			rm.updateMetrics()
		}
	}
}

// updateMetrics updates current resource metrics
func (rm *ResourceMonitor) updateMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.metrics = &ResourceMetrics{
		MemoryUsageMB:  int64(memStats.Alloc / 1024 / 1024),
		GoroutineCount: runtime.NumGoroutine(),
		GCPauseTime:    time.Duration(memStats.PauseNs[(memStats.NumGC+255)%256]),
		HeapObjects:    memStats.HeapObjects,
		HeapSizeBytes:  memStats.HeapSys,
		LastUpdated:    time.Now(),
	}

	// Check for alerts
	rm.checkResourceAlerts()
}

// checkResourceAlerts checks if any resource thresholds are exceeded
func (rm *ResourceMonitor) checkResourceAlerts() {
	if rm.metrics.MemoryUsageMB > rm.config.MemoryThresholdMB {
		alert := ResourceAlert{
			Type:         "memory",
			Message:      "Memory usage exceeded threshold",
			Severity:     "warning",
			Threshold:    float64(rm.config.MemoryThresholdMB),
			CurrentValue: float64(rm.metrics.MemoryUsageMB),
			Timestamp:    time.Now(),
		}
		rm.alerts = append(rm.alerts, alert)
		rm.log.Warn("Memory usage alert",
			logger.String("message", alert.Message),
			logger.String("current_mb", fmt.Sprintf("%.0f", alert.CurrentValue)),
		)
	}
}

// GetMetrics returns current resource metrics
func (rm *ResourceMonitor) GetMetrics() *ResourceMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Return a copy
	metrics := *rm.metrics
	return &metrics
}

// RecordSample records a performance sample
func (pc *PerformanceCollector) RecordSample(operation string, duration time.Duration, success bool, resourceMetrics *ResourceMetrics) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	sample := PerformanceSample{
		Timestamp:     time.Now(),
		Operation:     operation,
		Duration:      duration,
		Success:       success,
		ResourceUsage: *resourceMetrics,
	}

	pc.samples = append(pc.samples, sample)

	// Keep only the last 1000 samples
	if len(pc.samples) > 1000 {
		pc.samples = pc.samples[1:]
	}
}

// GetTrend returns a simple performance trend analysis
func (pc *PerformanceCollector) GetTrend() string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if len(pc.samples) < 10 {
		return "insufficient_data"
	}

	// Simple trend analysis - compare recent vs older samples
	recentSamples := pc.samples[len(pc.samples)-10:]
	var recentAvg time.Duration
	for _, sample := range recentSamples {
		recentAvg += sample.Duration
	}
	recentAvg = recentAvg / time.Duration(len(recentSamples))

	olderSamples := pc.samples[:10]
	var olderAvg time.Duration
	for _, sample := range olderSamples {
		olderAvg += sample.Duration
	}
	olderAvg = olderAvg / time.Duration(len(olderSamples))

	if recentAvg > olderAvg*110/100 {
		return "degrading"
	} else if recentAvg < olderAvg*90/100 {
		return "improving"
	}
	return "stable"
}
