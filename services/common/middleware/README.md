# Middleware Package

A collection of common HTTP middleware components for Go microservices using the Gin framework.

## Features

- Request logging middleware
- Panic recovery middleware
- CORS middleware
- Authentication middleware
- Response writer with status code tracking

## Installation

```bash
go get github.com/FBDAF/microservices/services/common/middleware
```

## Usage

### Logging Middleware

```go
import (
    "github.com/FBDAF/microservices/services/common/middleware"
    "github.com/gin-gonic/gin"
)

// Create a logger that implements the LogRequest interface
type Logger struct{}

func (l *Logger) LogRequest(r *http.Request, statusCode int, duration time.Duration) {
    // Log the request details
}

// Use the logging middleware
router := gin.Default()
logger := &Logger{}
router.Use(middleware.LoggingMiddleware(logger))
```

### Recovery Middleware

```go
// Add panic recovery middleware
router := gin.Default()
router.Use(middleware.RecoveryMiddleware())
```

### CORS Middleware

```go
// Add CORS middleware
router := gin.Default()
router.Use(middleware.CORSMiddleware())
```

### Authentication Middleware

```go
// Create an authentication function
authFunc := func(c *gin.Context) error {
    // Validate the request
    return nil
}

// Add authentication middleware
router := gin.Default()
router.Use(middleware.AuthMiddleware(authFunc))
```

### Response Writer

```go
// The ResponseWriter is used internally by the middleware
// You can access the status code through the gin.Context
func handler(c *gin.Context) {
    // Your handler code
    c.Status(http.StatusOK) // This will be captured by the ResponseWriter
}
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
