package queue

import (
	"go.uber.org/zap"
)

type Logger struct {
	*zap.Logger
}

func NewLogger(level string) (*Logger, error) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(getLogLevel(level).Level())

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{logger}, nil
}

func (l *Logger) WithFields(fields ...zap.Field) *Logger {
	return &Logger{l.With(fields...)}
}

func getLogLevel(level string) zap.AtomicLevel {
	switch level {
	case "debug":
		return zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		return zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		return zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	}
}
