# Worker Service Patterns

## Overview

This document outlines the patterns and best practices for implementing worker services in our microservices architecture, specifically focusing on asynchronous task processing. Worker services are crucial components that handle background tasks, long-running operations, and asynchronous processing in a distributed system.

## Core Patterns

### 1. Message Processing

#### Queue Integration

- **RabbitMQ Connection Management**

  ```go
  type QueueConnection struct {
      conn    *amqp.Connection
      channel *amqp.Channel
      config  QueueConfig
      logger  Logger
  }

  func NewQueueConnection(config QueueConfig) (*QueueConnection, error) {
      conn, err := amqp.Dial(config.URL)
      if err != nil {
          return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
      }

      ch, err := conn.Channel()
      if err != nil {
          return nil, fmt.Errorf("failed to open channel: %w", err)
      }

      return &QueueConnection{
          conn:    conn,
          channel: ch,
          config:  config,
      }, nil
  }
  ```

- **Message Acknowledgment Patterns**

  ```go
  type MessageAcknowledger struct {
      channel *amqp.Channel
      logger  Logger
  }

  func (ma *MessageAcknowledger) Acknowledge(msg amqp.Delivery) error {
      if err := msg.Ack(false); err != nil {
          ma.logger.Error("failed to acknowledge message",
              "error", err,
              "messageId", msg.MessageId,
          )
          return err
      }
      return nil
  }

  func (ma *MessageAcknowledger) Reject(msg amqp.Delivery, requeue bool) error {
      if err := msg.Reject(requeue); err != nil {
          ma.logger.Error("failed to reject message",
              "error", err,
              "messageId", msg.MessageId,
              "requeue", requeue,
          )
          return err
      }
      return nil
  }
  ```

- **Dead Letter Queue Handling**

  ```go
  type DeadLetterHandler struct {
      channel *amqp.Channel
      config  DeadLetterConfig
      logger  Logger
  }

  func (dlh *DeadLetterHandler) SetupDeadLetterQueue() error {
      // Declare dead letter exchange
      err := dlh.channel.ExchangeDeclare(
          dlh.config.ExchangeName,
          "direct",
          true,
          false,
          false,
          false,
          nil,
      )
      if err != nil {
          return fmt.Errorf("failed to declare dead letter exchange: %w", err)
      }

      // Declare dead letter queue
      _, err = dlh.channel.QueueDeclare(
          dlh.config.QueueName,
          true,
          false,
          false,
          false,
          amqp.Table{
              "x-dead-letter-exchange":    dlh.config.ExchangeName,
              "x-dead-letter-routing-key": dlh.config.RoutingKey,
          },
      )
      if err != nil {
          return fmt.Errorf("failed to declare dead letter queue: %w", err)
      }

      return nil
  }
  ```

#### Message Handling

- **Message Validation**

  ```go
  type MessageValidator struct {
      schema    *jsonschema.Schema
      logger    Logger
  }

  func (mv *MessageValidator) ValidateMessage(msg []byte) error {
      result, err := mv.schema.Validate(bytes.NewReader(msg))
      if err != nil {
          return fmt.Errorf("failed to validate message: %w", err)
      }

      if !result.Valid() {
          mv.logger.Error("message validation failed",
              "errors", result.Errors(),
          )
          return ErrInvalidMessage
      }

      return nil
  }
  ```

- **Message Transformation**

  ```go
  type MessageTransformer struct {
      transformers map[string]MessageTransformFunc
      logger       Logger
  }

  func (mt *MessageTransformer) TransformMessage(msg Message) (Message, error) {
      transform, exists := mt.transformers[msg.Type]
      if !exists {
          return msg, nil
      }

      transformed, err := transform(msg)
      if err != nil {
          mt.logger.Error("failed to transform message",
              "error", err,
              "messageType", msg.Type,
          )
          return Message{}, err
      }

      return transformed, nil
  }
  ```

### 2. Task Processing

#### Long-Running Tasks

- **Task State Management**

  ```go
  type TaskStateManager struct {
      storage StorageClient
      logger  Logger
  }

  func (tsm *TaskStateManager) UpdateTaskState(ctx context.Context, taskID string, state TaskState) error {
      update := TaskStateUpdate{
          TaskID:    taskID,
          State:     state,
          UpdatedAt: time.Now(),
      }

      if err := tsm.storage.UpdateTaskState(ctx, update); err != nil {
          tsm.logger.Error("failed to update task state",
              "error", err,
              "taskID", taskID,
              "state", state,
          )
          return err
      }

      return nil
  }
  ```

- **Progress Tracking**

  ```go
  type ProgressTracker struct {
      storage StorageClient
      logger  Logger
  }

  func (pt *ProgressTracker) UpdateProgress(ctx context.Context, taskID string, progress float64) error {
      update := ProgressUpdate{
          TaskID:    taskID,
          Progress:  progress,
          UpdatedAt: time.Now(),
      }

      if err := pt.storage.UpdateProgress(ctx, update); err != nil {
          pt.logger.Error("failed to update progress",
              "error", err,
              "taskID", taskID,
              "progress", progress,
          )
          return err
      }

      return nil
  }
  ```

#### Task Prioritization

- **Priority Queue Implementation**

  ```go
  type PriorityQueue struct {
      queues    map[int]*Queue
      priorities []int
      mu        sync.RWMutex
  }

  func (pq *PriorityQueue) Push(task Task) error {
      pq.mu.Lock()
      defer pq.mu.Unlock()

      queue, exists := pq.queues[task.Priority]
      if !exists {
          queue = NewQueue()
          pq.queues[task.Priority] = queue
          pq.priorities = append(pq.priorities, task.Priority)
          sort.Ints(pq.priorities)
      }

      return queue.Push(task)
  }

  func (pq *PriorityQueue) Pop() (Task, error) {
      pq.mu.Lock()
      defer pq.mu.Unlock()

      for _, priority := range pq.priorities {
          queue := pq.queues[priority]
          if task, err := queue.Pop(); err == nil {
              return task, nil
          }
      }

      return Task{}, ErrQueueEmpty
  }
  ```

### 3. Error Handling

#### Recovery Mechanisms

- **Circuit Breaker Implementation**

  ```go
  type CircuitBreaker struct {
      failures     int
      threshold    int
      resetTimeout time.Duration
      lastFailure  time.Time
      mu           sync.RWMutex
  }

  func (cb *CircuitBreaker) Execute(fn func() error) error {
      if cb.isOpen() {
          return ErrCircuitOpen
      }

      err := fn()
      if err != nil {
          cb.recordFailure()
          return err
      }

      cb.reset()
      return nil
  }

  func (cb *CircuitBreaker) isOpen() bool {
      cb.mu.RLock()
      defer cb.mu.RUnlock()

      if cb.failures >= cb.threshold {
          if time.Since(cb.lastFailure) > cb.resetTimeout {
              cb.reset()
              return false
          }
          return true
      }
      return false
  }
  ```

- **Retry Mechanism**

  ```go
  type RetryMechanism struct {
      maxRetries  int
      backoff     BackoffStrategy
      logger      Logger
  }

  func (rm *RetryMechanism) Execute(fn func() error) error {
      var lastErr error
      for attempt := 0; attempt <= rm.maxRetries; attempt++ {
          err := fn()
          if err == nil {
              return nil
          }

          lastErr = err
          if attempt < rm.maxRetries {
              delay := rm.backoff.Next(attempt)
              rm.logger.Info("retrying operation",
                  "attempt", attempt+1,
                  "maxRetries", rm.maxRetries,
                  "delay", delay,
                  "error", err,
              )
              time.Sleep(delay)
          }
      }

      return fmt.Errorf("operation failed after %d retries: %w", rm.maxRetries, lastErr)
  }
  ```

### 4. Monitoring and Metrics

#### Performance Metrics

- **Metrics Collection**

  ```go
  type MetricsCollector struct {
      prometheusClient PrometheusClient
      logger          Logger
  }

  func (mc *MetricsCollector) RecordTaskMetrics(task Task, duration time.Duration, err error) {
      mc.prometheusClient.RecordProcessingTime(task.Type, duration)
      mc.prometheusClient.RecordQueueLength(task.Queue)

      if err != nil {
          mc.prometheusClient.RecordError(task.Type, err)
      }

      mc.logger.Info("recorded task metrics",
          "taskType", task.Type,
          "duration", duration,
          "error", err,
      )
  }
  ```

#### Health Checks

- **Health Check Implementation**

  ```go
  type HealthChecker struct {
      checks    []HealthCheck
      logger    Logger
  }

  func (hc *HealthChecker) CheckHealth() HealthStatus {
      status := HealthStatus{
          Status: "healthy",
          Checks: make(map[string]CheckResult),
      }

      for _, check := range hc.checks {
          result := check.Execute()
          status.Checks[check.Name] = result

          if !result.Healthy {
              status.Status = "unhealthy"
              hc.logger.Error("health check failed",
                  "check", check.Name,
                  "error", result.Error,
              )
          }
      }

      return status
  }
  ```

## Implementation Guidelines

### 1. Service Structure

```go
type WorkerService struct {
    queueClient    QueueClient
    taskProcessor  TaskProcessor
    metricsClient  MetricsClient
    config         Config
    logger         Logger
    healthChecker  HealthChecker
    circuitBreaker CircuitBreaker
}

func NewWorkerService(config Config) (*WorkerService, error) {
    queueClient, err := NewQueueClient(config.Queue)
    if err != nil {
        return nil, fmt.Errorf("failed to create queue client: %w", err)
    }

    metricsClient, err := NewMetricsClient(config.Metrics)
    if err != nil {
        return nil, fmt.Errorf("failed to create metrics client: %w", err)
    }

    return &WorkerService{
        queueClient:    queueClient,
        taskProcessor:  NewTaskProcessor(config.Processing),
        metricsClient:  metricsClient,
        config:         config,
        logger:         NewLogger(config.Logging),
        healthChecker:  NewHealthChecker(config.Health),
        circuitBreaker: NewCircuitBreaker(config.CircuitBreaker),
    }, nil
}
```

### 2. Configuration Management

```yaml
worker:
  queue:
    host: rabbitmq
    port: 5672
    username: worker
    password: ${QUEUE_PASSWORD}
    vhost: /workers
    prefetch: 10
    reconnect:
      maxAttempts: 5
      initialDelay: 1s
      maxDelay: 30s

  processing:
    maxRetries: 3
    retryDelay: 5s
    timeout: 30s
    batchSize: 10
    concurrency: 5
    priorityLevels: 3

  monitoring:
    metricsPort: 9090
    healthCheckInterval: 30s
    logLevel: info
    alertThresholds:
      errorRate: 0.01
      processingTime: 100ms
      queueLength: 1000

  circuitBreaker:
    threshold: 5
    resetTimeout: 30s
    halfOpenTimeout: 5s
```

### 3. Error Handling

```go
func (w *WorkerService) processMessage(msg Message) error {
    ctx := context.Background()
    startTime := time.Now()

    // Validate message
    if err := w.validateMessage(msg); err != nil {
        w.metricsClient.RecordError("validation", err)
        return w.handleValidationError(ctx, msg, err)
    }

    // Process task with circuit breaker
    err := w.circuitBreaker.Execute(func() error {
        return w.taskProcessor.ProcessTask(ctx, msg.Task)
    })

    if err != nil {
        w.metricsClient.RecordError("processing", err)
        return w.handleProcessingError(ctx, msg, err)
    }

    // Record metrics
    duration := time.Since(startTime)
    w.metricsClient.RecordProcessingTime(msg.Task.Type, duration)

    // Acknowledge message
    return w.acknowledgeMessage(msg)
}
```

### 4. Monitoring Implementation

```go
func (w *WorkerService) recordMetrics(task Task, duration time.Duration, err error) {
    w.metricsClient.RecordProcessingTime(task.Type, duration)
    w.metricsClient.RecordQueueLength(w.queueClient.GetQueueName())

    if err != nil {
        w.metricsClient.RecordError(task.Type, err)
        w.logger.Error("task processing failed",
            "taskType", task.Type,
            "taskID", task.ID,
            "error", err,
            "duration", duration,
        )
    } else {
        w.logger.Info("task processed successfully",
            "taskType", task.Type,
            "taskID", task.ID,
            "duration", duration,
        )
    }
}
```

## Best Practices

### 1. Message Processing

- **Idempotency**

  - Use message IDs for deduplication
  - Implement idempotent operations
  - Handle duplicate messages gracefully
  - Track processed message IDs

- **Message Correlation**

  - Use correlation IDs
  - Track message relationships
  - Implement message tracing
  - Handle message chains

- **Dead Letter Queues**

  - Configure DLQ for failed messages
  - Implement retry policies
  - Handle poison messages
  - Monitor DLQ size

- **Message Ordering**
  - Use message sequencing
  - Handle out-of-order messages
  - Implement ordering guarantees
  - Track message dependencies

### 2. Resource Management

- **Graceful Shutdown**

  - Handle in-flight messages
  - Complete current tasks
  - Close connections properly
  - Save state if needed

- **Connection Pooling**

  - Implement connection limits
  - Handle connection failures
  - Monitor pool health
  - Implement backoff

- **Backpressure**

  - Implement rate limiting
  - Handle queue backpressure
  - Monitor resource usage
  - Scale based on load

- **Resource Monitoring**
  - Track memory usage
  - Monitor CPU usage
  - Track connection counts
  - Monitor queue sizes

### 3. Error Handling

- **Circuit Breakers**

  - Implement failure thresholds
  - Handle partial failures
  - Implement fallbacks
  - Monitor circuit state

- **Retry Strategies**

  - Use exponential backoff
  - Implement jitter
  - Handle retry limits
  - Track retry attempts

- **Error Logging**

  - Log detailed errors
  - Include context
  - Track error patterns
  - Monitor error rates

- **Error Reporting**
  - Send error notifications
  - Track error trends
  - Implement error aggregation
  - Monitor error impact

### 4. Monitoring

- **Processing Metrics**

  - Track processing time
  - Monitor success rates
  - Track error rates
  - Monitor queue lengths

- **Queue Health**

  - Monitor queue size
  - Track message age
  - Monitor consumer lag
  - Track queue performance

- **Resource Usage**

  - Monitor CPU usage
  - Track memory usage
  - Monitor network I/O
  - Track disk usage

- **Alerting**
  - Set up error alerts
  - Monitor performance
  - Track resource usage
  - Alert on thresholds

### 5. Security

- **Queue Security**

  - Use TLS for connections
  - Implement authentication
  - Use secure credentials
  - Monitor access

- **Message Security**

  - Encrypt sensitive data
  - Validate messages
  - Sanitize inputs
  - Track message flow

- **Access Control**

  - Implement RBAC
  - Monitor access
  - Track changes
  - Audit operations

- **Security Monitoring**
  - Track security events
  - Monitor access patterns
  - Alert on anomalies
  - Audit security logs

## Cross-References

- [Queuing Patterns](queuing-patterns.md)
- [Monitoring Patterns](monitoring-patterns.md)
- [Security Patterns](security-patterns.md)
- [Data Storage Patterns](data-storage-patterns.md)
- [Long-Running Tasks](long-running-tasks.md)

## Notes

- Keep patterns up to date
- Document implementation details
- Track pattern evolution
- Maintain cross-references
- Update examples regularly
- Validate patterns in production
- Monitor pattern effectiveness
- Gather feedback from teams
