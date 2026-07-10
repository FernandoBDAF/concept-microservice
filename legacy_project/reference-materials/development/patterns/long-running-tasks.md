# Long-Running Tasks Pattern

## Overview

This document outlines the patterns and best practices for handling long-running tasks in worker services, with specific focus on image generation and email processing tasks.

## Task Categories

### 1. Image Generation Tasks

#### Characteristics

- Long processing time (30s - 5min)
- High resource usage
- External service dependency
- Result storage required

#### Implementation Considerations

- Progress tracking
- Resource management
- Result caching
- Error recovery

### 2. Email Processing Tasks

#### Characteristics

- Moderate processing time (1-10s)
- External service dependency
- Retry requirements
- Status tracking

#### Implementation Considerations

- Rate limiting
- Retry strategies
- Status updates
- Error handling

## Implementation Patterns

### 1. Task State Management

```go
type TaskState struct {
    ID          string
    Status      TaskStatus
    Progress    float64
    StartTime   time.Time
    EndTime     time.Time
    Error       error
    Result      interface{}
}

type TaskStatus string

const (
    TaskStatusPending   TaskStatus = "PENDING"
    TaskStatusRunning   TaskStatus = "RUNNING"
    TaskStatusComplete  TaskStatus = "COMPLETE"
    TaskStatusFailed    TaskStatus = "FAILED"
    TaskStatusCancelled TaskStatus = "CANCELLED"
)
```

### 2. Progress Tracking

```go
type ProgressTracker struct {
    taskID     string
    totalSteps int
    currentStep int
    status     TaskStatus
    startTime  time.Time
    updates    chan ProgressUpdate
}

func (pt *ProgressTracker) UpdateProgress(step int, message string) {
    pt.currentStep = step
    progress := float64(step) / float64(pt.totalSteps)

    pt.updates <- ProgressUpdate{
        TaskID:    pt.taskID,
        Progress:  progress,
        Message:   message,
        Timestamp: time.Now(),
    }
}
```

### 3. Resource Management

```go
type ResourceManager struct {
    maxConcurrentTasks int
    currentTasks      int
    resourcePool      chan struct{}
    metrics          MetricsClient
}

func (rm *ResourceManager) AcquireResource() error {
    select {
    case rm.resourcePool <- struct{}{}:
        rm.currentTasks++
        rm.metrics.RecordResourceAcquisition()
        return nil
    case <-time.After(5 * time.Second):
        return ErrResourceUnavailable
    }
}

func (rm *ResourceManager) ReleaseResource() {
    <-rm.resourcePool
    rm.currentTasks--
    rm.metrics.RecordResourceRelease()
}
```

### 4. Result Handling

```go
type ResultHandler struct {
    storage    StorageClient
    cache      CacheClient
    metrics    MetricsClient
}

func (rh *ResultHandler) StoreResult(ctx context.Context, task Task, result interface{}) error {
    // Store in persistent storage
    if err := rh.storage.Store(ctx, task.ID, result); err != nil {
        return fmt.Errorf("failed to store result: %w", err)
    }

    // Cache for quick access
    if err := rh.cache.Set(ctx, task.ID, result, 24*time.Hour); err != nil {
        rh.metrics.RecordCacheError()
    }

    return nil
}
```

## Best Practices

### 1. Task Management

- Implement task timeouts
- Use task prioritization
- Implement task cancellation
- Handle task dependencies

### 2. Resource Management

- Implement resource limits
- Use resource pooling
- Monitor resource usage
- Implement backpressure

### 3. Error Handling

- Implement retry strategies
- Use circuit breakers
- Handle partial failures
- Implement recovery procedures

### 4. Monitoring

- Track task progress
- Monitor resource usage
- Track error rates
- Implement alerting

### 5. Performance

- Implement caching
- Use batch processing
- Optimize resource usage
- Monitor performance metrics

## Implementation Guidelines

### 1. Image Generation Tasks

```go
type ImageGenerationTask struct {
    TaskID      string
    ImageType   string
    Parameters  map[string]interface{}
    MaxRetries  int
    Timeout     time.Duration
}

func (t *ImageGenerationTask) Process(ctx context.Context) error {
    // Initialize progress tracker
    tracker := NewProgressTracker(t.TaskID, 5)

    // Acquire resources
    if err := resourceManager.AcquireResource(); err != nil {
        return err
    }
    defer resourceManager.ReleaseResource()

    // Process task
    result, err := t.generateImage(ctx, tracker)
    if err != nil {
        return t.handleError(ctx, err)
    }

    // Store result
    return resultHandler.StoreResult(ctx, t, result)
}
```

### 2. Email Processing Tasks

```go
type EmailProcessingTask struct {
    TaskID      string
    EmailType   string
    Recipient   string
    Content     interface{}
    MaxRetries  int
}

func (t *EmailProcessingTask) Process(ctx context.Context) error {
    // Initialize retry mechanism
    retry := NewRetryMechanism(t.MaxRetries)

    // Process with retries
    return retry.Execute(func() error {
        return t.sendEmail(ctx)
    })
}
```

## Cross-References

- [Worker Service Patterns](worker-service-patterns.md)
- [Queuing Patterns](queuing-patterns.md)
- [Monitoring Patterns](monitoring-patterns.md)
- [Error Handling Patterns](error-handling-patterns.md)

## Notes

- Keep patterns up to date
- Document implementation details
- Track pattern evolution
- Maintain cross-references
