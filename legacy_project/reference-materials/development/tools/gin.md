# Gin Framework Usage Guide

## Overview

Gin is a high-performance HTTP web framework written in Go. In our microservices architecture, we use Gin for handling HTTP requests and responses, providing a robust and efficient way to build our REST APIs.

## Key Features Used

### 1. Router Setup

We use two main approaches for setting up Gin routers:

```go
// Approach 1: Using gin.Default()
router := gin.Default()
```

- Includes default logger middleware
- Includes recovery middleware
- Best for development and when you want standard logging

```go
// Approach 2: Using gin.New()
router := gin.New()
router.Use(gin.LoggerWithFormatter(customFormatter))
router.Use(gin.Recovery())
```

- Clean router without middleware
- Allows custom middleware configuration
- Better for production when you need custom logging

### 2. Middleware

Gin provides several built-in middleware options:

- `gin.Logger()`: Request logging
- `gin.Recovery()`: Panic recovery
- `gin.CORS()`: Cross-Origin Resource Sharing
- Custom middleware for authentication, rate limiting, etc.

### 3. Route Groups

We use route groups to organize our endpoints:

```go
v1 := router.Group("/api/v1")
{
    auth := v1.Group("/auth")
    {
        auth.POST("/token", authHandler.Authenticate)
        auth.POST("/validate", authHandler.ValidateToken)
    }
}
```

### 4. Request Handling

Gin provides several methods for handling requests:

```go
// JSON binding
var req AuthenticateRequest
if err := c.ShouldBindJSON(&req); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
}

// Path parameters
id := c.Param("id")

// Query parameters
query := c.Query("search")

// Headers
authHeader := c.GetHeader("Authorization")
```

### 5. Response Methods

Different ways to send responses:

```go
// JSON response
c.JSON(http.StatusOK, gin.H{
    "status": "success",
    "data": data,
})

// String response
c.String(http.StatusOK, "success")

// Status only
c.Status(http.StatusNoContent)
```

## Best Practices

1. **Router Setup**

   - Use `gin.Default()` for development and when standard logging is sufficient
   - Use `gin.New()` with custom middleware for production when you need specific logging formats

2. **Error Handling**

   - Always use proper error handling in middleware and handlers
   - Return appropriate HTTP status codes
   - Include meaningful error messages

3. **Middleware Order**

   - Recovery middleware should be first
   - Logging middleware should be early in the chain
   - Authentication middleware should be before protected routes

4. **Route Organization**

   - Use route groups for better organization
   - Group related endpoints together
   - Use consistent URL patterns

5. **Performance**
   - Use `gin.ReleaseMode` in production
   - Implement proper middleware for rate limiting
   - Use appropriate timeouts

## Common Issues and Solutions

1. **Missing Logs in Release Mode**

   - Problem: Logs not appearing in production
   - Solution: Use `gin.Default()` or explicitly add logger middleware

2. **Middleware Not Working**

   - Problem: Middleware not being applied
   - Solution: Check middleware order and ensure proper setup

3. **Request Binding Issues**
   - Problem: Request data not being bound correctly
   - Solution: Use proper struct tags and validation

## Examples from Our Project

### Profile API

```go
router := gin.Default()
// Routes and middleware setup
```

### Auth Service

```go
gin.SetMode(gin.ReleaseMode)
router := gin.Default()
// Routes and middleware setup
```

## References

- [Gin Official Documentation](https://gin-gonic.com/docs/)
- [Gin GitHub Repository](https://github.com/gin-gonic/gin)
