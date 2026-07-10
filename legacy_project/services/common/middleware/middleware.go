package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ResponseWriter is a wrapper around http.ResponseWriter that captures the status code
type ResponseWriter struct {
	gin.ResponseWriter
	statusCode int
}

// NewResponseWriter creates a new ResponseWriter
func NewResponseWriter(w gin.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{w, http.StatusOK}
}

// WriteHeader captures the status code
func (rw *ResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(logger interface {
	LogRequest(*http.Request, int, time.Duration)
}) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		rw := NewResponseWriter(c.Writer)
		c.Writer = rw
		c.Next()
		duration := time.Since(start)
		logger.LogRequest(c.Request, rw.statusCode, duration)
	}
}

// RecoveryMiddleware recovers from panics
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Internal Server Error",
				})
			}
		}()
		c.Next()
	}
}

// CORSMiddleware handles CORS headers
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// AuthMiddleware handles authentication
func AuthMiddleware(authFunc func(*gin.Context) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := authFunc(c); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized",
			})
			return
		}
		c.Next()
	}
}
