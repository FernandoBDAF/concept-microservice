package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/fernandobarroso/microservices/api-service/internal/config"
)

// New creates a configured Zap logger
func New(cfg config.LoggingConfig) (*zap.Logger, error) {
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	if cfg.Development {
		encoderCfg = zap.NewDevelopmentEncoderConfig()
	}

	var encoder zapcore.Encoder
	if cfg.Format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	}

	core := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level)
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return logger, nil
}

// String is a helper for structured logging
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

// ErrorField is a helper for structured errors
func ErrorField(err error) zap.Field {
	return zap.Error(err)
}

