package main

import (
	"errors"
	"log"
	"time"

	"github.com/FBDAF/microservices/services/common/logging"
	"go.uber.org/zap"
)

func main() {
	// Create a new structured logger
	logger := logging.NewStructuredLogger()

	// Log with different levels
	logger.Debug("Debug message", zap.String("debug_info", "This is debug information"))

	logger.Info("User action", zap.String("action", "login"), zap.String("ip", "192.168.1.1"))

	logger.Warn("Resource usage high", zap.Int("cpu_usage", 85), zap.Int("mem_usage", 90))

	// Log an error
	err := errors.New("database connection failed")
	logger.Error("Operation failed", zap.String("error", err.Error()), zap.String("operation", "database_connect"))

	// Create a logger with rotation (placeholder, as actual implementation may differ)
	// rotationConfig := &logging.RotationConfig{...}
	// writer, err := logging.NewRotatingFileWriter("logs/app.log", rotationConfig)
	// if err != nil {
	// 	logger.Fatal("Failed to create rotating file writer", zap.String("error", err.Error()))
	// }
	// defer writer.Close()

	// Create a logger with aggregation (placeholder, as actual implementation may differ)
	// aggregationConfig := &logging.AggregationConfig{...}
	// aggregatingLogger := logging.NewAggregationLogger(logger, aggregationConfig)
	// defer aggregatingLogger.Sync()

	// Create a zap-based logger for metrics
	zapLogger, err := logging.New()
	if err != nil {
		log.Fatalf("Failed to create zap logger: %v", err)
	}
	metricsLogger := logging.NewMetricsLogger(zapLogger)
	// Log with metrics
	metricsLogger.Info("Application metrics", zap.String("metric_type", "performance"))

	// Simulate some work
	time.Sleep(time.Second)

	// Log completion
	metricsLogger.Info("Operation completed", zap.Int("duration_ms", 1000), zap.String("status", "success"))
}
