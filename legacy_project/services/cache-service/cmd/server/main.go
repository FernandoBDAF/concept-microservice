package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"cache-service/internal/config"
	"cache-service/internal/domain/services"
	"cache-service/internal/infrastructure/logging"
	"cache-service/internal/infrastructure/metrics"
	"cache-service/internal/infrastructure/redis"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Printf("Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := logging.NewLogger(&cfg.Logging)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Add service metadata to logger
	logger = logging.LoggerMiddleware(logger)

	logger.Info("Starting cache service",
		zap.String("version", "1.0.0"),
		zap.Int("http_port", cfg.Server.HTTPPort),
		zap.Int("grpc_port", cfg.Server.GRPCPort),
	)

	// Initialize metrics
	metricsCollector := metrics.NewMetrics()

	// Initialize Redis client
	redisClient, err := redis.NewClient(&cfg.Redis, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Redis client", zap.Error(err))
	}
	defer redisClient.Close()

	// Initialize circuit breaker with configuration (Task 2.4)
	redisClient.InitializeCircuitBreaker(&cfg.CircuitBr)

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	logger.Info("Successfully connected to Redis")

	// Initialize cache service
	cacheService := services.NewCacheService(
		redisClient,
		metricsCollector,
		logger,
		&cfg.Cache,
	)

	// Setup HTTP server
	httpServer := setupHTTPServer(cfg, cacheService, metricsCollector, logger)

	// Setup gRPC server (placeholder for now)
	grpcServer := setupGRPCServer(cfg, cacheService, logger)

	// Start servers
	httpErrChan := make(chan error, 1)
	grpcErrChan := make(chan error, 1)

	// Start HTTP server
	go func() {
		logger.Info("Starting HTTP server", zap.Int("port", cfg.Server.HTTPPort))
		httpErrChan <- httpServer.ListenAndServe()
	}()

	// Start gRPC server
	go func() {
		logger.Info("Starting gRPC server", zap.Int("port", cfg.Server.GRPCPort))
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
		if err != nil {
			grpcErrChan <- fmt.Errorf("failed to listen on gRPC port: %w", err)
			return
		}
		grpcErrChan <- grpcServer.Serve(lis)
	}()

	// Start metrics server if enabled
	if cfg.Metrics.Enabled {
		go func() {
			metricsServer := &http.Server{
				Addr:    fmt.Sprintf(":%d", cfg.Metrics.Port),
				Handler: promhttp.Handler(),
			}
			logger.Info("Starting metrics server", zap.Int("port", cfg.Metrics.Port))
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("Metrics server error", zap.Error(err))
			}
		}()
	}

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-httpErrChan:
		logger.Error("HTTP server error", zap.Error(err))
	case err := <-grpcErrChan:
		logger.Error("gRPC server error", zap.Error(err))
	case <-quit:
		logger.Info("Shutdown signal received")
	}

	// Graceful shutdown
	logger.Info("Shutting down servers...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", zap.Error(err))
	}

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	logger.Info("Cache service stopped")
}

// setupHTTPServer creates and configures the HTTP server
func setupHTTPServer(cfg *config.Config, cacheService *services.CacheService, metrics *metrics.Metrics, logger *zap.Logger) *http.Server {
	// Set Gin mode based on environment
	if cfg.Logging.Development {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
	router.Use(LoggingMiddleware(logger))
	router.Use(MetricsMiddleware(metrics))

	// Health check endpoints
	router.GET("/health", HealthCheckHandler(cacheService))
	router.GET("/ready", ReadinessCheckHandler(cacheService))

	// Cache API endpoints (basic implementation)
	v1 := router.Group("/api/v1")
	{
		v1.GET("/cache/:key", GetCacheHandler(cacheService))
		v1.POST("/cache/:key", SetCacheHandler(cacheService))
		v1.DELETE("/cache/:key", DeleteCacheHandler(cacheService))
		v1.GET("/cache/:key/exists", ExistsCacheHandler(cacheService))
		v1.GET("/cache/:key/ttl", GetTTLHandler(cacheService))
		v1.PUT("/cache/:key/ttl", SetTTLHandler(cacheService))

		// Batch operations
		v1.POST("/cache/batch/get", BatchGetHandler(cacheService))
		v1.POST("/cache/batch/set", BatchSetHandler(cacheService))
		v1.DELETE("/cache/batch", BatchDeleteHandler(cacheService))

		// Pattern-based operations (Task 2.2)
		v1.DELETE("/cache/pattern/:pattern", DeleteByPatternHandler(cacheService))

		// Statistics
		v1.GET("/stats", StatsHandler(cacheService))
		v1.GET("/status", StatusHandler(cacheService))
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	return server
}

// setupGRPCServer creates and configures the gRPC server
func setupGRPCServer(cfg *config.Config, cacheService *services.CacheService, logger *zap.Logger) *grpc.Server {
	// Create gRPC server with interceptors
	server := grpc.NewServer(
		grpc.UnaryInterceptor(GRPCLoggingInterceptor(logger)),
	)

	// TODO: Register gRPC service implementations
	// This would be implemented in Phase 1 completion

	return server
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		if path == "/ready" || path == "/health" {
			return
		}

		logger.Info("HTTP request",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", clientIP),
		)
	}
}

// MetricsMiddleware records HTTP metrics
func MetricsMiddleware(metrics *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		method := c.Request.Method

		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()

		metrics.RecordHTTPRequest(method, path, statusCode, duration)
	}
}

// GRPCLoggingInterceptor logs gRPC requests
func GRPCLoggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)

		if err != nil {
			logger.Error("gRPC request failed",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", duration),
				zap.Error(err),
			)
		} else {
			logger.Info("gRPC request",
				zap.String("method", info.FullMethod),
				zap.Duration("duration", duration),
			)
		}

		return resp, err
	}
}

// Placeholder handlers - will be implemented in subsequent phases
func HealthCheckHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := cacheService.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		})
	}
}

func ReadinessCheckHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := cacheService.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// Placeholder cache handlers - basic implementation for Phase 1
func GetCacheHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		value, err := cacheService.Get(ctx, key)
		if err != nil {
			if err == services.ErrKeyNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": string(value), "status": "success"})
	}
}

func SetCacheHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")

		// Read request body
		body, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}

		// Get TTL from query parameter (optional)
		ttl := time.Duration(0)
		if ttlStr := c.Query("ttl"); ttlStr != "" {
			if parsedTTL, err := time.ParseDuration(ttlStr); err == nil {
				ttl = parsedTTL
			}
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := cacheService.Set(ctx, key, body, ttl); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

func DeleteCacheHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := cacheService.Delete(ctx, key); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

func ExistsCacheHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		exists, err := cacheService.Exists(ctx, key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"exists": exists})
	}
}

func GetTTLHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		ttl, err := cacheService.GetTTL(ctx, key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ttl": ttl.String()})
	}
}

func SetTTLHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")

		var req struct {
			TTL string `json:"ttl"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		ttl, err := time.ParseDuration(req.TTL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid TTL format"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := cacheService.SetTTL(ctx, key, ttl); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

// Batch operation handlers - Task 2.1 Implementation
func BatchGetHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Keys []string `json:"keys" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if len(req.Keys) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "keys array cannot be empty"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		values, err := cacheService.MGet(ctx, req.Keys)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Convert values to base64 for JSON response
		response := gin.H{
			"values":  make(map[string]string),
			"missing": []string{},
		}

		for _, key := range req.Keys {
			if value, exists := values[key]; exists {
				response["values"].(map[string]string)[key] = string(value)
			} else {
				response["missing"] = append(response["missing"].([]string), key)
			}
		}

		c.JSON(http.StatusOK, response)
	}
}

func BatchSetHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Items []struct {
				Key   string `json:"key" binding:"required"`
				Value string `json:"value" binding:"required"`
				TTL   string `json:"ttl,omitempty"`
			} `json:"items" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if len(req.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "items array cannot be empty"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		// Convert to map format for MSet
		items := make(map[string][]byte)
		var ttl time.Duration

		for _, item := range req.Items {
			items[item.Key] = []byte(item.Value)

			// Parse TTL if provided
			if item.TTL != "" {
				parsedTTL, err := time.ParseDuration(item.TTL)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": fmt.Sprintf("invalid TTL format for key %s: %s", item.Key, item.TTL),
					})
					return
				}
				ttl = parsedTTL // Use the last TTL for all items
			}
		}

		if err := cacheService.MSet(ctx, items, ttl); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"count":  len(items),
		})
	}
}

func BatchDeleteHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Keys []string `json:"keys" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if len(req.Keys) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "keys array cannot be empty"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		if err := cacheService.MDelete(ctx, req.Keys); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"count":  len(req.Keys),
		})
	}
}

func DeleteByPatternHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		pattern := c.Param("pattern")
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		deletedCount, err := cacheService.DeleteByPattern(ctx, pattern)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"count":  deletedCount,
		})
	}
}

func StatsHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		stats, err := cacheService.GetStats(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, stats)
	}
}

func StatusHandler(cacheService *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service":   "cache-service",
			"version":   "1.0.0",
			"status":    "running",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}
