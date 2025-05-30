package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/logger"
)

// RequestIDKey is the context key for the request ID
type RequestIDKey string

const (
	// RequestIDHeader is the HTTP header name for the request ID
	RequestIDHeader = "X-Request-ID"
	// RequestIDContextKey is the context key for the request ID
	RequestIDContextKey RequestIDKey = "request_id"
)

// LoggingMiddleware creates a middleware that logs HTTP requests and responses
func LoggingMiddleware(next http.Handler) http.Handler {
	log := logger.Get()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Get request ID from context
		requestID := r.Context().Value(RequestIDContextKey)
		if requestID == nil {
			requestID = "unknown"
		}

		// Create a custom response writer to capture the status code
		rw := newResponseWriter(w)

		// Log request
		log.Info("HTTP request started",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.String("remote_addr", r.RemoteAddr),
			logger.String("user_agent", r.UserAgent()),
			logger.String("request_id", requestID.(string)),
		)

		// Process request
		next.ServeHTTP(rw, r)

		// Log response
		duration := time.Since(startTime)
		log.Info("HTTP request completed",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.Int("status_code", rw.statusCode),
			logger.Duration("duration", duration),
			logger.String("request_id", requestID.(string)),
		)
	})
}

// responseWriter is a custom response writer that captures the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// newResponseWriter creates a new response writer
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

// WriteHeader captures the status code before writing it
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// RecoveryMiddleware creates a middleware that recovers from panics and logs them
func RecoveryMiddleware(next http.Handler) http.Handler {
	log := logger.Get()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("HTTP request panic recovered",
					zap.Any("panic", err),
					logger.String("method", r.Method),
					logger.String("path", r.URL.Path),
					logger.String("remote_addr", r.RemoteAddr),
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// RequestIDMiddleware creates a middleware that adds a request ID to the context
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get request ID from header or generate new one
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Add request ID to response header
		w.Header().Set(RequestIDHeader, requestID)

		// Create new context with request ID
		ctx := r.Context()
		ctx = context.WithValue(ctx, RequestIDContextKey, requestID)

		// Process request with new context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TimeoutMiddleware creates a middleware that adds a timeout to the request context
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create context with timeout
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			// Process request with timeout context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
