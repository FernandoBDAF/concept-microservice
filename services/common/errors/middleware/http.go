package middleware

import (
	"github.com/FBDAF/microservices/services/common/errors/formatters"
	"github.com/FBDAF/microservices/services/common/errors/types"
	"github.com/gin-gonic/gin"
)

// ErrorHandler is a middleware that handles errors in HTTP requests
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			formatters.WriteError(c.Writer, err)
			return
		}
	}
}

// Recovery is a middleware that recovers from panics
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Convert panic to error
				var panicErr error
				switch v := err.(type) {
				case error:
					panicErr = v
				default:
					panicErr = types.New(types.ErrCodeInternal, "Internal server error")
				}

				// Add trace ID if available
				if traceID := c.GetHeader("X-Trace-ID"); traceID != "" {
					if customErr, ok := panicErr.(types.Error); ok {
						panicErr = customErr.WithTraceID(traceID)
					}
				}

				// Write error response
				formatters.WriteError(c.Writer, panicErr)
				c.Abort()
			}
		}()

		c.Next()
	}
}

// NotFoundHandler handles 404 errors
func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := types.New(types.ErrCodeNotFound, "Resource not found")
		formatters.WriteError(c.Writer, err)
		c.Abort()
	}
}

// MethodNotAllowedHandler handles 405 errors
func MethodNotAllowedHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := types.New(types.ErrCodeForbidden, "Method not allowed")
		formatters.WriteError(c.Writer, err)
		c.Abort()
	}
}
