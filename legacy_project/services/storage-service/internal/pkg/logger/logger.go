package logger

import (
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// log is the global logger instance
	log *zap.Logger
	// once ensures the logger is initialized only once
	once sync.Once
)

// Config holds the logger configuration
type Config struct {
	// Environment is the current environment (development/production)
	Environment string
	// Level is the minimum log level to output
	Level string
	// ServiceName is the name of the service for log identification
	ServiceName string
	// Rotation is the log rotation configuration
	Rotation RotationConfig
	// Aggregation is the log aggregation configuration
	Aggregation AggregationConfig
}

// DefaultConfig returns the default logger configuration
func DefaultConfig() Config {
	return Config{
		Environment: "development",
		Level:       "info",
		ServiceName: "storage-service",
		Rotation:    DefaultRotationConfig(),
	}
}

// Init initializes the global logger instance
func Init(cfg Config) error {
	var err error
	once.Do(func() {
		// Validate configuration
		if err = validateConfig(cfg); err != nil {
			return
		}

		// Set log level
		if err = setLogLevel(cfg.Level); err != nil {
			err = WrapError(err, "failed to set log level")
			return
		}

		// Add service name to all logs
		log = log.With(zap.String("service", cfg.ServiceName))

		// Set the global logger
		zap.ReplaceGlobals(log)
	})

	if err != nil {
		return WrapError(err, "failed to initialize logger")
	}
	return nil
}

// validateConfig validates the logger configuration
func validateConfig(config Config) error {
	// Validate environment
	if config.Environment != "development" && config.Environment != "production" {
		return WrapError(ErrInvalidConfig, "invalid environment: "+config.Environment)
	}

	// Validate log level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[config.Level] {
		return WrapError(ErrInvalidConfig, "invalid log level: "+config.Level)
	}

	// Validate service name
	if config.ServiceName == "" {
		return WrapError(ErrInvalidConfig, "service name is required")
	}

	return nil
}

// setLogLevel sets the log level for the global logger
func setLogLevel(level string) error {
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		return WrapError(ErrInvalidConfig, "invalid log level: "+level)
	}

	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// Create a new core with the specified level
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zapLevel,
	)

	// Create a new logger with the core
	log = zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return nil
}

// Get returns the global logger instance
func Get() *zap.Logger {
	if log == nil {
		// Initialize with default config if not already initialized
		if err := Init(DefaultConfig()); err != nil {
			// If initialization fails, create a basic logger
			config := zap.NewDevelopmentConfig()
			config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			log, _ = config.Build()
		}
	}
	return log
}

// Sync flushes any buffered log entries
func Sync() error {
	if log == nil {
		return ErrLoggerNotInitialized
	}
	return log.Sync()
}

// String creates a string field
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

// Int creates an integer field
func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

// Duration creates a duration field
func Duration(key string, value time.Duration) zap.Field {
	return zap.Duration(key, value)
}

// ErrorField creates an error field
func ErrorField(err error) zap.Field {
	return zap.Error(err)
}

// Bool creates a boolean field
func Bool(key string, value bool) zap.Field {
	return zap.Bool(key, value)
}

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

// Info logs an info message
func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

// Fatal logs a fatal message and then calls os.Exit(1)
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}

// With creates a child logger with the given fields
func With(fields ...zap.Field) *zap.Logger {
	return Get().With(fields...)
}
