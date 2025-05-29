package logger

import (
	"context"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// AggregationConfig holds the configuration for log aggregation
type AggregationConfig struct {
	// Enabled determines if log aggregation is enabled
	Enabled bool
	// Endpoint is the log aggregation service endpoint
	Endpoint string
	// BatchSize is the number of logs to batch before sending
	BatchSize int
	// FlushInterval is the interval at which logs are flushed
	FlushInterval time.Duration
	// RetryCount is the number of times to retry failed sends
	RetryCount int
	// RetryDelay is the delay between retries
	RetryDelay time.Duration
	// Timeout is the timeout for sending logs
	Timeout time.Duration
}

// DefaultAggregationConfig returns the default log aggregation configuration
func DefaultAggregationConfig() AggregationConfig {
	return AggregationConfig{
		Enabled:       false,
		Endpoint:      "http://localhost:9200",
		BatchSize:     100,
		FlushInterval: 5 * time.Second,
		RetryCount:    3,
		RetryDelay:    time.Second,
		Timeout:       5 * time.Second,
	}
}

// Aggregator handles log aggregation
type Aggregator struct {
	config AggregationConfig
	logger *zap.Logger
	core   zapcore.Core
	ctx    context.Context
	cancel context.CancelFunc
}

// NewAggregator creates a new log aggregator
func NewAggregator(config AggregationConfig, logger *zap.Logger) (*Aggregator, error) {
	if !config.Enabled {
		return nil, nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create a new core for log aggregation
	core, err := createAggregationCore(config)
	if err != nil {
		cancel()
		return nil, WrapError(err, "failed to create aggregation core")
	}

	return &Aggregator{
		config: config,
		logger: logger,
		core:   core,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// createAggregationCore creates a new core for log aggregation
func createAggregationCore(config AggregationConfig) (zapcore.Core, error) {
	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// Create a new core with the specified level
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout), // Temporary output until we implement the actual aggregation
		zapcore.InfoLevel,
	)

	return core, nil
}

// Start starts the log aggregator
func (a *Aggregator) Start() error {
	if a == nil {
		return nil
	}

	// Start the aggregation process
	go a.aggregate()

	return nil
}

// Stop stops the log aggregator
func (a *Aggregator) Stop() error {
	if a == nil {
		return nil
	}

	// Cancel the context to stop the aggregation process
	a.cancel()

	return nil
}

// aggregate handles the log aggregation process
func (a *Aggregator) aggregate() {
	ticker := time.NewTicker(a.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if err := a.flush(); err != nil {
				a.logger.Error("failed to flush logs",
					ErrorField(err),
					String("endpoint", a.config.Endpoint),
				)
			}
		}
	}
}

// flush sends the batched logs to the aggregation service
func (a *Aggregator) flush() error {
	// TODO: Implement log shipping to the aggregation service
	// This will be implemented when we choose a specific log aggregation service
	return nil
}

// WithAggregation adds log aggregation to the logger
func WithAggregation(logger *zap.Logger, config AggregationConfig) (*zap.Logger, error) {
	aggregator, err := NewAggregator(config, logger)
	if err != nil {
		return nil, err
	}

	if aggregator == nil {
		return logger, nil
	}

	// Start the aggregator
	if err := aggregator.Start(); err != nil {
		return nil, err
	}

	// Create a new logger with the aggregation core
	return logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewTee(core, aggregator.core)
	})), nil
}
