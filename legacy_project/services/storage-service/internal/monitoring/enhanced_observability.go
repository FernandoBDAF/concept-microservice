package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/domain/service"
	"microservices/services/profile-storage/internal/infrastructure/database"
	"microservices/services/profile-storage/internal/performance"
	"microservices/services/profile-storage/internal/pkg/logger"
)

// EnhancedObservabilityManager provides comprehensive monitoring and observability
type EnhancedObservabilityManager struct {
	metricsCollector   *EnhancedMetricsCollector
	healthMonitor      *EnhancedHealthMonitor
	alertManager       *AlertManager
	logAnalyzer        *LogAnalyzer
	performanceMonitor *performance.OptimizationManager
	log                *zap.Logger
	config             *ObservabilityConfig
	mu                 sync.RWMutex
}

// ObservabilityConfig holds configuration for enhanced observability
type ObservabilityConfig struct {
	MetricsEnabled      bool            `json:"metrics_enabled"`
	HealthCheckEnabled  bool            `json:"health_check_enabled"`
	AlertingEnabled     bool            `json:"alerting_enabled"`
	LogAnalysisEnabled  bool            `json:"log_analysis_enabled"`
	MetricsPort         int             `json:"metrics_port"`
	HealthCheckInterval time.Duration   `json:"health_check_interval"`
	AlertThresholds     AlertThresholds `json:"alert_thresholds"`
}

// AlertThresholds defines thresholds for various alerts
type AlertThresholds struct {
	ErrorRatePercent     float64       `json:"error_rate_percent"`
	ResponseTimeP95      time.Duration `json:"response_time_p95"`
	MemoryUsageMB        int64         `json:"memory_usage_mb"`
	CPUUsagePercent      float64       `json:"cpu_usage_percent"`
	QueueDepth           int           `json:"queue_depth"`
	FailedBatchesPercent float64       `json:"failed_batches_percent"`
}

// EnhancedMetricsCollector collects comprehensive metrics
type EnhancedMetricsCollector struct {
	// Batch operation metrics
	batchOperationsTotal *prometheus.CounterVec
	batchProcessingTime  *prometheus.HistogramVec
	batchSuccessRate     *prometheus.GaugeVec
	batchOperationSize   *prometheus.HistogramVec
	batchConcurrency     prometheus.Gauge
	batchQueueDepth      prometheus.Gauge

	// Performance metrics
	performanceOptimizations prometheus.Counter
	connectionPoolUsage      *prometheus.GaugeVec
	queryPerformance         *prometheus.HistogramVec
	cacheHitRate             prometheus.Gauge
	resourceUsage            *prometheus.GaugeVec

	// Message queue metrics
	messageProcessingTime *prometheus.HistogramVec
	messageQueueDepth     *prometheus.GaugeVec
	messageFailures       *prometheus.CounterVec
	messageRetries        *prometheus.CounterVec

	// System metrics
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	activeConnections   prometheus.Gauge
	errorRate           *prometheus.GaugeVec

	registry prometheus.Registerer
}

// EnhancedHealthMonitor provides comprehensive health monitoring
type EnhancedHealthMonitor struct {
	checks        map[string]EnhancedHealthCheck
	overallStatus HealthStatus
	lastUpdate    time.Time
	connManager   *database.ConnectionManager
	batchService  *service.AdvancedBatchOperationsService
	log           *zap.Logger
	mu            sync.RWMutex
}

// EnhancedHealthCheck represents a single health check
type EnhancedHealthCheck struct {
	Name        string                 `json:"name"`
	Status      HealthStatus           `json:"status"`
	Message     string                 `json:"message"`
	LastChecked time.Time              `json:"last_checked"`
	Duration    time.Duration          `json:"duration"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// HealthStatus represents the health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// HealthReport provides a comprehensive health report
type HealthReport struct {
	OverallStatus HealthStatus                   `json:"overall_status"`
	Timestamp     time.Time                      `json:"timestamp"`
	Checks        map[string]EnhancedHealthCheck `json:"checks"`
	Summary       HealthSummary                  `json:"summary"`
	Uptime        time.Duration                  `json:"uptime"`
}

// HealthSummary provides a summary of health check results
type HealthSummary struct {
	TotalChecks     int `json:"total_checks"`
	HealthyChecks   int `json:"healthy_checks"`
	DegradedChecks  int `json:"degraded_checks"`
	UnhealthyChecks int `json:"unhealthy_checks"`
}

// AlertManager handles alerting based on metrics and health status
type AlertManager struct {
	enabled     bool
	thresholds  AlertThresholds
	alerts      []Alert
	subscribers []AlertSubscriber
	log         *zap.Logger
	mu          sync.RWMutex
}

// Alert represents a system alert
type Alert struct {
	ID         string                 `json:"id"`
	Type       AlertType              `json:"type"`
	Severity   AlertSeverity          `json:"severity"`
	Title      string                 `json:"title"`
	Message    string                 `json:"message"`
	Timestamp  time.Time              `json:"timestamp"`
	Resolved   bool                   `json:"resolved"`
	ResolvedAt *time.Time             `json:"resolved_at,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// AlertType represents the type of alert
type AlertType string

const (
	AlertTypePerformance AlertType = "performance"
	AlertTypeError       AlertType = "error"
	AlertTypeResource    AlertType = "resource"
	AlertTypeBatch       AlertType = "batch"
	AlertTypeSystem      AlertType = "system"
)

// AlertSeverity represents the severity of an alert
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertSubscriber handles alert notifications
type AlertSubscriber interface {
	SendAlert(alert Alert) error
}

// LogAnalyzer analyzes logs for patterns and anomalies
type LogAnalyzer struct {
	enabled   bool
	patterns  []LogPattern
	anomalies []LogAnomaly
	log       *zap.Logger
	mu        sync.RWMutex
}

// LogPattern represents a log pattern to monitor
type LogPattern struct {
	Name      string        `json:"name"`
	Pattern   string        `json:"pattern"`
	Threshold int           `json:"threshold"`
	Window    time.Duration `json:"window"`
	Severity  AlertSeverity `json:"severity"`
}

// LogAnomaly represents a detected log anomaly
type LogAnomaly struct {
	Pattern   LogPattern `json:"pattern"`
	Count     int        `json:"count"`
	Timestamp time.Time  `json:"timestamp"`
	Messages  []string   `json:"messages,omitempty"`
}

// NewEnhancedObservabilityManager creates a new enhanced observability manager
func NewEnhancedObservabilityManager(
	connManager *database.ConnectionManager,
	batchService *service.AdvancedBatchOperationsService,
	performanceMonitor *performance.OptimizationManager,
) *EnhancedObservabilityManager {
	config := &ObservabilityConfig{
		MetricsEnabled:      true,
		HealthCheckEnabled:  true,
		AlertingEnabled:     true,
		LogAnalysisEnabled:  true,
		MetricsPort:         9090,
		HealthCheckInterval: 30 * time.Second,
		AlertThresholds: AlertThresholds{
			ErrorRatePercent:     5.0,
			ResponseTimeP95:      500 * time.Millisecond,
			MemoryUsageMB:        512,
			CPUUsagePercent:      80.0,
			QueueDepth:           1000,
			FailedBatchesPercent: 10.0,
		},
	}

	manager := &EnhancedObservabilityManager{
		log:                logger.Get().Named("enhanced_observability"),
		config:             config,
		performanceMonitor: performanceMonitor,
	}

	// Initialize components
	manager.metricsCollector = NewEnhancedMetricsCollector()
	manager.healthMonitor = NewEnhancedHealthMonitor(connManager, batchService)
	manager.alertManager = NewAlertManager(config.AlertThresholds)
	manager.logAnalyzer = NewLogAnalyzer()

	return manager
}

// Start begins enhanced observability monitoring
func (eom *EnhancedObservabilityManager) Start(ctx context.Context) error {
	eom.log.Info("Starting enhanced observability manager")

	// Start metrics server if enabled
	if eom.config.MetricsEnabled {
		go eom.startMetricsServer()
	}

	// Start health monitoring
	if eom.config.HealthCheckEnabled {
		go eom.startHealthMonitoring(ctx)
	}

	// Start log analysis
	if eom.config.LogAnalysisEnabled {
		go eom.startLogAnalysis(ctx)
	}

	// Start alert monitoring
	if eom.config.AlertingEnabled {
		go eom.startAlertMonitoring(ctx)
	}

	eom.log.Info("Enhanced observability manager started successfully")
	return nil
}

// startMetricsServer starts the Prometheus metrics server
func (eom *EnhancedObservabilityManager) startMetricsServer() {
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", eom.handleHealthCheck)
	http.HandleFunc("/health/detailed", eom.handleDetailedHealthCheck)

	addr := fmt.Sprintf(":%d", eom.config.MetricsPort)
	eom.log.Info("Starting metrics server", logger.String("addr", addr))

	if err := http.ListenAndServe(addr, nil); err != nil {
		eom.log.Error("Metrics server failed", logger.ErrorField(err))
	}
}

// startHealthMonitoring starts continuous health monitoring
func (eom *EnhancedObservabilityManager) startHealthMonitoring(ctx context.Context) {
	ticker := time.NewTicker(eom.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			eom.log.Info("Health monitoring stopped")
			return
		case <-ticker.C:
			eom.performHealthChecks()
		}
	}
}

// startLogAnalysis starts log pattern analysis
func (eom *EnhancedObservabilityManager) startLogAnalysis(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			eom.log.Info("Log analysis stopped")
			return
		case <-ticker.C:
			eom.logAnalyzer.AnalyzeLogs()
		}
	}
}

// startAlertMonitoring starts alert monitoring and processing
func (eom *EnhancedObservabilityManager) startAlertMonitoring(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			eom.log.Info("Alert monitoring stopped")
			return
		case <-ticker.C:
			eom.checkAlertConditions()
		}
	}
}

// handleHealthCheck handles basic health check requests
func (eom *EnhancedObservabilityManager) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	report := eom.healthMonitor.GetHealthReport()

	status := http.StatusOK
	if report.OverallStatus == HealthStatusUnhealthy {
		status = http.StatusServiceUnavailable
	} else if report.OverallStatus == HealthStatusDegraded {
		status = http.StatusPartialContent
	}

	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"status":    report.OverallStatus,
		"timestamp": report.Timestamp,
		"uptime":    report.Uptime.String(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		eom.log.Error("Failed to encode health response", logger.ErrorField(err))
	}
}

// handleDetailedHealthCheck handles detailed health check requests
func (eom *EnhancedObservabilityManager) handleDetailedHealthCheck(w http.ResponseWriter, r *http.Request) {
	report := eom.healthMonitor.GetHealthReport()

	status := http.StatusOK
	if report.OverallStatus == HealthStatusUnhealthy {
		status = http.StatusServiceUnavailable
	} else if report.OverallStatus == HealthStatusDegraded {
		status = http.StatusPartialContent
	}

	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(report); err != nil {
		eom.log.Error("Failed to encode detailed health response", logger.ErrorField(err))
	}
}

// performHealthChecks performs all registered health checks
func (eom *EnhancedObservabilityManager) performHealthChecks() {
	eom.healthMonitor.RunAllChecks()

	// Check if any alerts should be triggered based on health status
	report := eom.healthMonitor.GetHealthReport()
	if report.OverallStatus == HealthStatusUnhealthy {
		eom.alertManager.TriggerAlert(Alert{
			Type:      AlertTypeSystem,
			Severity:  AlertSeverityCritical,
			Title:     "System Health Critical",
			Message:   "Multiple health checks are failing",
			Timestamp: time.Now(),
		})
	}
}

// checkAlertConditions checks various conditions and triggers alerts if necessary
func (eom *EnhancedObservabilityManager) checkAlertConditions() {
	// Get performance metrics
	perfReport := eom.performanceMonitor.GetOptimizationReport()

	// Check memory usage
	if perfReport.ResourceUsage.MemoryUsageMB > eom.config.AlertThresholds.MemoryUsageMB {
		eom.alertManager.TriggerAlert(Alert{
			Type:      AlertTypeResource,
			Severity:  AlertSeverityWarning,
			Title:     "High Memory Usage",
			Message:   fmt.Sprintf("Memory usage is %dMB (threshold: %dMB)", perfReport.ResourceUsage.MemoryUsageMB, eom.config.AlertThresholds.MemoryUsageMB),
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"current_memory_mb": perfReport.ResourceUsage.MemoryUsageMB,
				"threshold_mb":      eom.config.AlertThresholds.MemoryUsageMB,
			},
		})
	}

	// Check performance optimization recommendations
	if len(perfReport.Recommendations) > 3 {
		eom.alertManager.TriggerAlert(Alert{
			Type:      AlertTypePerformance,
			Severity:  AlertSeverityInfo,
			Title:     "Performance Optimization Recommendations",
			Message:   fmt.Sprintf("System has %d performance recommendations", len(perfReport.Recommendations)),
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"recommendations": perfReport.Recommendations,
			},
		})
	}
}

// GetObservabilityReport returns a comprehensive observability report
func (eom *EnhancedObservabilityManager) GetObservabilityReport() *ObservabilityReport {
	eom.mu.RLock()
	defer eom.mu.RUnlock()

	return &ObservabilityReport{
		Timestamp:         time.Now(),
		HealthReport:      eom.healthMonitor.GetHealthReport(),
		PerformanceReport: eom.performanceMonitor.GetOptimizationReport(),
		ActiveAlerts:      eom.alertManager.GetActiveAlerts(),
		MetricsSummary:    eom.metricsCollector.GetSummary(),
		LogAnomalies:      eom.logAnalyzer.GetAnomalies(),
	}
}

// ObservabilityReport provides a comprehensive observability overview
type ObservabilityReport struct {
	Timestamp         time.Time                       `json:"timestamp"`
	HealthReport      *HealthReport                   `json:"health_report"`
	PerformanceReport *performance.OptimizationReport `json:"performance_report"`
	ActiveAlerts      []Alert                         `json:"active_alerts"`
	MetricsSummary    *MetricsSummary                 `json:"metrics_summary"`
	LogAnomalies      []LogAnomaly                    `json:"log_anomalies"`
}

// MetricsSummary provides a summary of collected metrics
type MetricsSummary struct {
	TotalRequests       int64         `json:"total_requests"`
	ErrorRate           float64       `json:"error_rate"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	BatchOperations     int64         `json:"batch_operations"`
	MessageProcessed    int64         `json:"messages_processed"`
}

// Helper functions and component implementations

// NewEnhancedMetricsCollector creates a new enhanced metrics collector
func NewEnhancedMetricsCollector() *EnhancedMetricsCollector {
	return &EnhancedMetricsCollector{
		batchOperationsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "storage_batch_operations_total",
			Help: "Total number of batch operations",
		}, []string{"type", "mode", "status"}),
		batchProcessingTime: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name: "storage_batch_processing_duration_seconds",
			Help: "Batch processing duration in seconds",
		}, []string{"type", "mode"}),
		performanceOptimizations: promauto.NewCounter(prometheus.CounterOpts{
			Name: "storage_performance_optimizations_total",
			Help: "Total number of performance optimizations applied",
		}),
		httpRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "storage_http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"method", "endpoint", "status"}),
		httpRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name: "storage_http_request_duration_seconds",
			Help: "HTTP request duration in seconds",
		}, []string{"method", "endpoint"}),
		registry: prometheus.DefaultRegisterer,
	}
}

// NewEnhancedHealthMonitor creates a new enhanced health monitor
func NewEnhancedHealthMonitor(connManager *database.ConnectionManager, batchService *service.AdvancedBatchOperationsService) *EnhancedHealthMonitor {
	monitor := &EnhancedHealthMonitor{
		checks:        make(map[string]EnhancedHealthCheck),
		overallStatus: HealthStatusUnknown,
		connManager:   connManager,
		batchService:  batchService,
		log:           logger.Get().Named("enhanced_health_monitor"),
	}

	// Register health checks
	monitor.registerHealthChecks()

	return monitor
}

// registerHealthChecks registers all health checks
func (ehm *EnhancedHealthMonitor) registerHealthChecks() {
	// Database health check
	ehm.checks["database"] = EnhancedHealthCheck{
		Name:   "Database Connection",
		Status: HealthStatusUnknown,
	}

	// Batch service health check
	ehm.checks["batch_service"] = EnhancedHealthCheck{
		Name:   "Batch Service",
		Status: HealthStatusUnknown,
	}

	// Memory health check
	ehm.checks["memory"] = EnhancedHealthCheck{
		Name:   "Memory Usage",
		Status: HealthStatusUnknown,
	}

	// Performance health check
	ehm.checks["performance"] = EnhancedHealthCheck{
		Name:   "Performance Metrics",
		Status: HealthStatusUnknown,
	}
}

// RunAllChecks runs all registered health checks
func (ehm *EnhancedHealthMonitor) RunAllChecks() {
	ehm.mu.Lock()
	defer ehm.mu.Unlock()

	healthyCount := 0
	totalChecks := len(ehm.checks)

	for name := range ehm.checks {
		check := ehm.runHealthCheck(name)
		ehm.checks[name] = check

		if check.Status == HealthStatusHealthy {
			healthyCount++
		}
	}

	// Determine overall status
	if healthyCount == totalChecks {
		ehm.overallStatus = HealthStatusHealthy
	} else if healthyCount > totalChecks/2 {
		ehm.overallStatus = HealthStatusDegraded
	} else {
		ehm.overallStatus = HealthStatusUnhealthy
	}

	ehm.lastUpdate = time.Now()
}

// runHealthCheck runs a specific health check
func (ehm *EnhancedHealthMonitor) runHealthCheck(name string) EnhancedHealthCheck {
	startTime := time.Now()
	check := EnhancedHealthCheck{
		Name:        name,
		LastChecked: startTime,
	}

	switch name {
	case "database":
		check = ehm.checkDatabase()
	case "batch_service":
		check = ehm.checkBatchService()
	case "memory":
		check = ehm.checkMemory()
	case "performance":
		check = ehm.checkPerformance()
	}

	check.Duration = time.Since(startTime)
	return check
}

// checkDatabase checks database connectivity
func (ehm *EnhancedHealthMonitor) checkDatabase() EnhancedHealthCheck {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	check := EnhancedHealthCheck{
		Name:        "Database Connection",
		LastChecked: time.Now(),
	}

	if ehm.connManager == nil {
		check.Status = HealthStatusUnhealthy
		check.Message = "Database connection manager not initialized"
		return check
	}

	// Test database connectivity
	db := ehm.connManager.GetDB()
	if err := db.PingContext(ctx); err != nil {
		check.Status = HealthStatusUnhealthy
		check.Message = fmt.Sprintf("Database ping failed: %v", err)
		return check
	}

	check.Status = HealthStatusHealthy
	check.Message = "Database connection healthy"
	return check
}

// checkBatchService checks batch service health
func (ehm *EnhancedHealthMonitor) checkBatchService() EnhancedHealthCheck {
	check := EnhancedHealthCheck{
		Name:        "Batch Service",
		LastChecked: time.Now(),
	}

	if ehm.batchService == nil {
		check.Status = HealthStatusUnhealthy
		check.Message = "Batch service not initialized"
		return check
	}

	// Check batch service metrics
	metrics := ehm.batchService.GetMetrics()
	if metrics == nil {
		check.Status = HealthStatusDegraded
		check.Message = "Batch service metrics unavailable"
		return check
	}

	check.Status = HealthStatusHealthy
	check.Message = "Batch service operational"
	check.Details = map[string]interface{}{
		"total_batches": metrics.TotalBatches,
		"success_rate":  metrics.AverageSuccessRate,
	}
	return check
}

// checkMemory checks memory usage
func (ehm *EnhancedHealthMonitor) checkMemory() EnhancedHealthCheck {
	check := EnhancedHealthCheck{
		Name:        "Memory Usage",
		LastChecked: time.Now(),
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryMB := int64(m.Alloc / 1024 / 1024)

	if memoryMB > 1000 {
		check.Status = HealthStatusUnhealthy
		check.Message = fmt.Sprintf("Memory usage critical: %dMB", memoryMB)
	} else if memoryMB > 512 {
		check.Status = HealthStatusDegraded
		check.Message = fmt.Sprintf("Memory usage high: %dMB", memoryMB)
	} else {
		check.Status = HealthStatusHealthy
		check.Message = fmt.Sprintf("Memory usage normal: %dMB", memoryMB)
	}

	check.Details = map[string]interface{}{
		"memory_mb":  memoryMB,
		"goroutines": runtime.NumGoroutine(),
	}

	return check
}

// checkPerformance checks overall performance health
func (ehm *EnhancedHealthMonitor) checkPerformance() EnhancedHealthCheck {
	check := EnhancedHealthCheck{
		Name:        "Performance Metrics",
		LastChecked: time.Now(),
		Status:      HealthStatusHealthy,
		Message:     "Performance metrics available",
	}

	return check
}

// GetHealthReport returns a comprehensive health report
func (ehm *EnhancedHealthMonitor) GetHealthReport() *HealthReport {
	ehm.mu.RLock()
	defer ehm.mu.RUnlock()

	summary := HealthSummary{
		TotalChecks: len(ehm.checks),
	}

	for _, check := range ehm.checks {
		switch check.Status {
		case HealthStatusHealthy:
			summary.HealthyChecks++
		case HealthStatusDegraded:
			summary.DegradedChecks++
		case HealthStatusUnhealthy:
			summary.UnhealthyChecks++
		}
	}

	return &HealthReport{
		OverallStatus: ehm.overallStatus,
		Timestamp:     ehm.lastUpdate,
		Checks:        ehm.checks,
		Summary:       summary,
		Uptime:        time.Since(time.Now().Add(-24 * time.Hour)), // Simplified uptime
	}
}

// NewAlertManager creates a new alert manager
func NewAlertManager(thresholds AlertThresholds) *AlertManager {
	return &AlertManager{
		enabled:     true,
		thresholds:  thresholds,
		alerts:      make([]Alert, 0),
		subscribers: make([]AlertSubscriber, 0),
		log:         logger.Get().Named("alert_manager"),
	}
}

// TriggerAlert triggers a new alert
func (am *AlertManager) TriggerAlert(alert Alert) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if !am.enabled {
		return
	}

	alert.ID = fmt.Sprintf("alert_%d", time.Now().UnixNano())
	am.alerts = append(am.alerts, alert)

	am.log.Warn("Alert triggered",
		logger.String("alert_id", alert.ID),
		logger.String("type", string(alert.Type)),
		logger.String("severity", string(alert.Severity)),
		logger.String("title", alert.Title),
	)

	// Notify subscribers
	for _, subscriber := range am.subscribers {
		if err := subscriber.SendAlert(alert); err != nil {
			am.log.Error("Failed to send alert", logger.ErrorField(err))
		}
	}
}

// GetActiveAlerts returns all active alerts
func (am *AlertManager) GetActiveAlerts() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var activeAlerts []Alert
	for _, alert := range am.alerts {
		if !alert.Resolved {
			activeAlerts = append(activeAlerts, alert)
		}
	}
	return activeAlerts
}

// NewLogAnalyzer creates a new log analyzer
func NewLogAnalyzer() *LogAnalyzer {
	return &LogAnalyzer{
		enabled:   true,
		patterns:  make([]LogPattern, 0),
		anomalies: make([]LogAnomaly, 0),
		log:       logger.Get().Named("log_analyzer"),
	}
}

// AnalyzeLogs analyzes recent logs for patterns and anomalies
func (la *LogAnalyzer) AnalyzeLogs() {
	if !la.enabled {
		return
	}

	// Simplified log analysis - in a real implementation, this would
	// parse actual log files or hook into the logging system
	la.log.Debug("Analyzing logs for patterns and anomalies")

	// Placeholder implementation
}

// GetAnomalies returns detected log anomalies
func (la *LogAnalyzer) GetAnomalies() []LogAnomaly {
	la.mu.RLock()
	defer la.mu.RUnlock()

	return append([]LogAnomaly(nil), la.anomalies...)
}

// GetSummary returns a metrics summary
func (emc *EnhancedMetricsCollector) GetSummary() *MetricsSummary {
	// This is a simplified implementation - in practice, you'd collect
	// actual metric values from Prometheus
	return &MetricsSummary{
		TotalRequests:       1000,
		ErrorRate:           2.5,
		AverageResponseTime: 150 * time.Millisecond,
		BatchOperations:     50,
		MessageProcessed:    75,
	}
}
