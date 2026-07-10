package logger

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// Logger is the global logger instance
	Logger *zap.Logger
	// Shipper is the global log shipper instance
	Shipper *LogShipper
)

// Config holds the logger configuration
type Config struct {
	Level       string
	Environment string
	ServiceName string
	Format      string // "json" or "console"
	LogFile     string // Path to log file for rotation
	Shipping    *ShippingConfig
}

// Initialize sets up the global logger
func Initialize(cfg *Config) error {
	// Set default level if not specified
	level := zapcore.InfoLevel
	if cfg.Level != "" {
		var err error
		level, err = zapcore.ParseLevel(cfg.Level)
		if err != nil {
			return err
		}
	}

	// Configure encoder
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if cfg.Format == "console" || (cfg.Format == "" && cfg.Environment == "development") {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Determine output
	var writeSyncer zapcore.WriteSyncer
	if cfg.LogFile != "" {
		// Use lumberjack for log rotation
		writeSyncer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.LogFile,
			MaxSize:    100, // MB
			MaxBackups: 3,
			MaxAge:     28, // days
			Compress:   true,
		})
	} else {
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	// Initialize log shipper if enabled
	if cfg.Shipping != nil && cfg.Shipping.Enabled {
		var err error
		Shipper, err = InitializeShipper(cfg.Shipping)
		if err != nil {
			return fmt.Errorf("failed to initialize log shipper: %w", err)
		}
	}

	// Create core
	core := zapcore.NewCore(
		encoder,
		writeSyncer,
		level,
	)

	// Create logger
	Logger = zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Fields(
			zap.String("service", cfg.ServiceName),
			zap.String("environment", cfg.Environment),
		),
	)

	return nil
}

// WithContext adds context values to the logger
func WithContext(ctx context.Context) *zap.Logger {
	logger := Logger
	if requestID := ctx.Value("request_id"); requestID != nil {
		logger = logger.With(zap.String("request_id", requestID.(string)))
	}
	return logger
}

// Sync flushes any buffered log entries
func Sync() error {
	if Shipper != nil {
		if err := Shipper.Flush(context.Background()); err != nil {
			return fmt.Errorf("failed to flush log shipper: %w", err)
		}
	}
	return Logger.Sync()
}

// LogRequest logs an HTTP request
func LogRequest(ctx context.Context, method, path string, status int, duration time.Duration) {
	WithContext(ctx).Info("http request",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", status),
		zap.Duration("duration", duration),
	)
}

// LogError logs an error with context
func LogError(ctx context.Context, msg string, err error, fields ...zap.Field) {
	WithContext(ctx).Error(msg,
		append(fields, zap.Error(err))...,
	)
}

// LogInfo logs an info message with context
func LogInfo(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Info(msg, fields...)
}

// LogDebug logs a debug message with context
func LogDebug(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Debug(msg, fields...)
}

// LogWarn logs a warning message
func LogWarn(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Warn(msg, fields...)
}
