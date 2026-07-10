# Jaeger Usage Guide

## Overview

Jaeger is an open-source distributed tracing system that helps monitor and troubleshoot microservices-based distributed systems. In our architecture, we use Jaeger to track requests across services, measure performance, and debug issues.

## Key Features Used

### 1. Service Configuration

We configure Jaeger in our services:

```go
// Jaeger configuration
func initJaeger(serviceName string) (opentracing.Tracer, io.Closer, error) {
    cfg := &config.Configuration{
        ServiceName: serviceName,
        Sampler: &config.SamplerConfig{
            Type:  "const",
            Param: 1,
        },
        Reporter: &config.ReporterConfig{
            LogSpans:           true,
            LocalAgentHostPort: "jaeger-agent:6831",
        },
    }
    return cfg.NewTracer()
}
```

### 2. Tracing Implementation

We implement tracing in our services:

```go
// Example of tracing in a service
func (s *ProfileService) GetProfile(ctx context.Context, id string) (*Profile, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "GetProfile")
    defer span.Finish()

    // Add tags to span
    span.SetTag("profile.id", id)

    // Check cache
    if profile, err := s.cache.Get(ctx, id); err == nil {
        span.SetTag("cache.hit", true)
        return profile, nil
    }
    span.SetTag("cache.hit", false)

    // Get from repository
    profile, err := s.repository.Get(ctx, id)
    if err != nil {
        span.SetTag("error", true)
        span.SetTag("error.message", err.Error())
        return nil, err
    }

    return profile, nil
}
```

### 3. Cross-Service Tracing

For tracing across service boundaries:

```go
// Client-side tracing
func (c *ProfileClient) GetProfile(ctx context.Context, id string) (*Profile, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "GetProfile")
    defer span.Finish()

    // Inject trace context into request
    req, err := http.NewRequest("GET", fmt.Sprintf("/profiles/%s", id), nil)
    if err != nil {
        return nil, err
    }

    if err := opentracing.GlobalTracer().Inject(
        span.Context(),
        opentracing.HTTPHeaders,
        opentracing.HTTPHeadersCarrier(req.Header),
    ); err != nil {
        return nil, err
    }

    // Make request
    resp, err := c.client.Do(req)
    if err != nil {
        span.SetTag("error", true)
        return nil, err
    }
    defer resp.Body.Close()

    // Process response
    var profile Profile
    if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
        span.SetTag("error", true)
        return nil, err
    }

    return &profile, nil
}
```

## Best Practices

1. **Span Management**

   - Use meaningful span names
   - Add relevant tags
   - Keep spans focused
   - Proper error handling

2. **Sampling**

   - Configure appropriate sampling rates
   - Use adaptive sampling
   - Monitor sampling impact
   - Adjust based on volume

3. **Performance**

   - Minimize span overhead
   - Use async reporting
   - Configure buffer sizes
   - Monitor memory usage

4. **Security**
   - Sanitize sensitive data
   - Use secure transport
   - Implement access control
   - Regular audits

## Common Issues and Solutions

1. **High Memory Usage**

   - Problem: Jaeger using too much memory
   - Solution: Adjust sampling rates and buffer sizes

2. **Missing Traces**

   - Problem: Traces not appearing in UI
   - Solution: Check sampling configuration and network connectivity

3. **Performance Impact**
   - Problem: Tracing affecting service performance
   - Solution: Use async reporting and optimize span creation

## Examples from Our Project

### Profile Service Tracing

```go
// Profile service tracing setup
func NewProfileService(repo Repository, cache Cache, logger Logger) (*ProfileService, error) {
    tracer, closer, err := initJaeger("profile-service")
    if err != nil {
        return nil, fmt.Errorf("failed to initialize tracer: %w", err)
    }
    opentracing.SetGlobalTracer(tracer)

    return &ProfileService{
        repository: repo,
        cache:      cache,
        logger:     logger,
        tracer:     tracer,
        closer:     closer,
    }, nil
}

// Tracing middleware
func TracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        spanCtx, _ := opentracing.GlobalTracer().Extract(
            opentracing.HTTPHeaders,
            opentracing.HTTPHeadersCarrier(r.Header),
        )
        span := opentracing.StartSpan(
            r.URL.Path,
            ext.RPCServerOption(spanCtx),
        )
        defer span.Finish()

        ctx := opentracing.ContextWithSpan(r.Context(), span)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Storage Service Tracing

```go
// Storage service tracing setup
func NewStorageService(repo Repository, logger Logger) (*StorageService, error) {
    tracer, closer, err := initJaeger("storage-service")
    if err != nil {
        return nil, fmt.Errorf("failed to initialize tracer: %w", err)
    }
    opentracing.SetGlobalTracer(tracer)

    return &StorageService{
        repository: repo,
        logger:     logger,
        tracer:     tracer,
        closer:     closer,
    }, nil
}

// Tracing in storage operations
func (s *StorageService) StoreFile(ctx context.Context, file *File) error {
    span, ctx := opentracing.StartSpanFromContext(ctx, "StoreFile")
    defer span.Finish()

    span.SetTag("file.name", file.Name)
    span.SetTag("file.size", file.Size)

    err := s.repository.Store(ctx, file)
    if err != nil {
        span.SetTag("error", true)
        span.SetTag("error.message", err.Error())
        return err
    }

    return nil
}
```

## References

- [Jaeger Official Documentation](https://www.jaegertracing.io/docs/)
- [OpenTracing Documentation](https://opentracing.io/docs/)
- [Jaeger Best Practices](https://www.jaegertracing.io/docs/1.21/best-practices/)
