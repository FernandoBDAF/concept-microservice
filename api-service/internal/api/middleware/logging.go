package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// LoggingMiddleware logs request details
func LoggingMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		}

		// Correlate logs with traces: otelgin runs before this middleware,
		// so a recording span (tracing enabled) is in the request context.
		if span := trace.SpanFromContext(c.Request.Context()); span.IsRecording() {
			fields = append(fields, zap.String("trace_id", span.SpanContext().TraceID().String()))
		}

		logger.Info("request", fields...)
	}
}
