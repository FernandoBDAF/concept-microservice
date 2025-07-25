# Integration Challenges with @common Packages

This document outlines common challenges, examples, and solutions encountered when integrating the @common packages into a service.

## 1. Middleware Integration

### Challenge

Integrating middleware (e.g., logging, recovery, CORS) requires the middleware to accept a logger that implements a specific interface (e.g., `LogRequest`). If your logger does not implement this interface, you'll encounter lint errors.

### Example

In our `simple-service`, the `middleware.LoggingMiddleware` expects a logger that implements:

```go
interface {
    LogRequest(*http.Request, int, time.Duration)
}
```

Our logger (`*logging.Logger`) did not implement this method, causing a lint error.

### Solution

Create an adapter that implements the required interface and delegates to your logger:

```go
type GinLoggerAdapter struct {
    logger *logging.Logger
}

func (g *GinLoggerAdapter) LogRequest(r *http.Request, statusCode int, duration time.Duration) {
    g.logger.Info("HTTP request",
        zap.String("method", r.Method),
        zap.String("path", r.URL.Path),
        zap.String("ip", r.RemoteAddr),
        zap.Int("status", statusCode),
        zap.Duration("duration", duration),
    )
}
```

Then use the adapter with the middleware:

```go
ginLogger := &GinLoggerAdapter{logger: service.logger}
r.Use(middleware.LoggingMiddleware(ginLogger))
```

## 2. Dependency Management

### Challenge

When integrating multiple @common packages, you may encounter issues with missing dependencies or incorrect module paths, leading to lint errors or build failures.

### Example

In our `simple-service`, we initially had lint errors because the `go.mod` file did not include the required dependencies for `gin` and the local `middleware` package.

### Solution

Ensure your `go.mod` file includes all necessary dependencies and uses the correct module paths. For local packages, use `replace` directives to point to the local directory:

```go
replace github.com/FBDAF/microservices/services/common/middleware => ../common/middleware
```

Then run `go mod tidy` to resolve and fetch all dependencies.

## 3. Configuration Management

### Challenge

Managing configuration across different environments (e.g., development, production) can be complex, especially when using a shared configuration package.

### Example

In our `simple-service`, we use the `config` package to load configuration from a `config.yaml` file. If the file is missing or misconfigured, the service will fail to start.

### Solution

Ensure your configuration files are present and correctly formatted. Use environment-specific configuration files (e.g., `config.dev.yaml`, `config.prod.yaml`) and load the appropriate file based on the environment.

## 4. Error Handling

### Challenge

Integrating a shared error handling package requires consistent error wrapping and propagation across your service.

### Example

In our `simple-service`, we use the `errors` package to wrap errors with additional context. If errors are not wrapped correctly, debugging becomes difficult.

### Solution

Always wrap errors with context using the `errors.Wrap` function:

```go
if err != nil {
    return nil, errors.Wrap(err, "failed to load configuration")
}
```

## 5. Logging and Metrics

### Challenge

Integrating logging and metrics packages requires consistent usage across your service to ensure all events are captured and monitored.

### Example

In our `simple-service`, we log requests and responses using the `logging` package. If logging is inconsistent, you may miss critical events.

### Solution

Use structured logging consistently across your service. Include relevant context (e.g., request ID, user ID) in log messages to facilitate debugging and monitoring.

## Conclusion

Integrating @common packages into a service can be challenging, but with careful planning and consistent practices, these challenges can be overcome. Always ensure your code adheres to the interfaces and patterns defined by the shared packages, and use adapters or wrappers when necessary to bridge gaps between your service and the shared components.
