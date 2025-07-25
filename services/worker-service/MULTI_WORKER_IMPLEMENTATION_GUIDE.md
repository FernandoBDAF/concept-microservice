# Multi-Worker Implementation Guide

## Overview

This guide provides a step-by-step approach to implementing different worker types that can be scaled independently while sharing common infrastructure and patterns. The strategy balances code reuse with deployment flexibility.

## Architecture Decision: Monorepo with Independent Deployments

### Recommended Approach: **Shared Foundation, Independent Deployments**

```
services/
├── worker-service/                    # Foundation service (current)
├── workers/                          # New worker implementations
│   ├── common/                       # Shared worker components
│   │   ├── base/                     # Base worker implementation
│   │   ├── processors/               # Common processor interfaces
│   │   └── utils/                    # Shared utilities
│   ├── profile-worker/               # Profile-specific worker
│   │   ├── cmd/main.go
│   │   ├── internal/processors/
│   │   ├── Dockerfile
│   │   └── k8s/
│   ├── notification-worker/          # Notification worker
│   │   ├── cmd/main.go
│   │   ├── internal/processors/
│   │   ├── Dockerfile
│   │   └── k8s/
│   ├── analytics-worker/             # Analytics worker
│   │   ├── cmd/main.go
│   │   ├── internal/processors/
│   │   ├── Dockerfile
│   │   └── k8s/
│   └── email-worker/                 # Email processing worker
│       ├── cmd/main.go
│       ├── internal/processors/
│       ├── Dockerfile
│       └── k8s/
└── common/                           # Shared across all services
    └── queue/                        # Common queue package
```

### Why This Approach?

✅ **Advantages:**

- **Code Reuse**: Share common worker patterns and infrastructure
- **Independent Scaling**: Each worker has its own Dockerfile and K8s manifests
- **Specialized Logic**: Each worker can have domain-specific processing
- **Deployment Flexibility**: Deploy, scale, and update workers independently
- **Maintenance**: Centralized common patterns, distributed specific logic
- **Resource Optimization**: Scale workers based on their specific load patterns

⚠️ **Considerations:**

- **Complexity**: More services to manage
- **Coordination**: Ensure common components stay compatible
- **Testing**: Need integration tests across worker types

## Step-by-Step Implementation

### Phase 1: Create Worker Foundation Structure

#### Step 1.1: Create Workers Directory Structure

```bash
# Create the new workers directory structure
mkdir -p services/workers/common/{base,processors,utils}
mkdir -p services/workers/{profile-worker,notification-worker,analytics-worker,email-worker}

# For each worker, create standard structure
for worker in profile-worker notification-worker analytics-worker email-worker; do
    mkdir -p services/workers/$worker/{cmd,internal/{processors,adapters,domain},k8s}
    touch services/workers/$worker/cmd/main.go
    touch services/workers/$worker/Dockerfile
    touch services/workers/$worker/go.mod
done
```

#### Step 1.2: Create Common Worker Base

**File: `services/workers/common/base/worker.go`**

```go
package base

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    commonQueue "github.com/fernandobarroso/common/queue"
    "github.com/fernandobarroso/workers/common/processors"
)

// WorkerConfig holds configuration for any worker
type WorkerConfig struct {
    WorkerType    string
    QueueName     string
    ExchangeName  string
    RoutingKey    string
    PrefetchCount int
    HTTPPort      string
}

// BaseWorker provides common worker functionality
type BaseWorker struct {
    config    *WorkerConfig
    processor processors.MessageProcessor
    consumer  *commonQueue.Consumer
    server    *HTTPServer
}

// NewBaseWorker creates a new base worker
func NewBaseWorker(config *WorkerConfig, processor processors.MessageProcessor) (*BaseWorker, error) {
    // Initialize queue configuration
    queueConfig := commonQueue.NewConfig()
    queueConfig.Queue = config.QueueName
    queueConfig.Exchange = config.ExchangeName
    queueConfig.RoutingKey = config.RoutingKey
    queueConfig.PrefetchCount = config.PrefetchCount

    // Build RabbitMQ URL from environment
    rabbitUser := os.Getenv("RABBITMQ_USER")
    rabbitPassword := os.Getenv("RABBITMQ_PASSWORD")
    rabbitHost := os.Getenv("RABBITMQ_HOST")
    rabbitPort := os.Getenv("RABBITMQ_PORT")
    queueConfig.URL = fmt.Sprintf("amqp://%s:%s@%s:%s/",
        rabbitUser, rabbitPassword, rabbitHost, rabbitPort)

    // Create consumer
    consumer, err := commonQueue.NewConsumer(queueConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create consumer: %w", err)
    }

    // Create HTTP server for health checks
    server := NewHTTPServer(config.HTTPPort)

    return &BaseWorker{
        config:    config,
        processor: processor,
        consumer:  consumer,
        server:    server,
    }, nil
}

// Start starts the worker
func (w *BaseWorker) Start(ctx context.Context) error {
    log.Printf("Starting %s worker", w.config.WorkerType)

    // Start HTTP server
    go func() {
        w.server.SetReady(true)
        if err := w.server.Start(ctx); err != nil {
            log.Printf("HTTP server error: %v", err)
        }
    }()

    // Message handler
    handler := func(msg *commonQueue.Message) error {
        return w.processor.Process(ctx, msg)
    }

    // Start consumer
    return w.consumer.Start(ctx, handler)
}

// Shutdown gracefully shuts down the worker
func (w *BaseWorker) Shutdown(ctx context.Context) error {
    log.Printf("Shutting down %s worker", w.config.WorkerType)

    w.server.SetReady(false)

    // Create shutdown context with timeout
    shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    // Shutdown HTTP server
    if err := w.server.Shutdown(shutdownCtx); err != nil {
        return err
    }

    // Close consumer
    return w.consumer.Close()
}

// Run runs the worker with signal handling
func (w *BaseWorker) Run() error {
    // Create context that listens for interrupt signals
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    // Start worker
    go func() {
        if err := w.Start(ctx); err != nil {
            log.Fatalf("Worker start error: %v", err)
        }
    }()

    // Wait for interrupt signal
    <-ctx.Done()

    // Shutdown worker
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    return w.Shutdown(shutdownCtx)
}
```

#### Step 1.3: Create Common Processor Interface

**File: `services/workers/common/processors/interface.go`**

```go
package processors

import (
    "context"
    commonQueue "github.com/fernandobarroso/common/queue"
)

// MessageProcessor defines the interface for processing messages
type MessageProcessor interface {
    // Process processes a message
    Process(ctx context.Context, msg *commonQueue.Message) error

    // Type returns the processor type for logging/metrics
    Type() string

    // Validate validates a message before processing
    Validate(msg *commonQueue.Message) error

    // HandleError handles processing errors
    HandleError(ctx context.Context, msg *commonQueue.Message, err error) error
}

// ProcessorMetrics defines metrics interface for processors
type ProcessorMetrics interface {
    IncProcessed(processorType string)
    IncErrors(processorType string)
    ObserveProcessingTime(processorType string, duration float64)
}

// BaseProcessor provides common processor functionality
type BaseProcessor struct {
    processorType string
    metrics       ProcessorMetrics
}

// NewBaseProcessor creates a new base processor
func NewBaseProcessor(processorType string, metrics ProcessorMetrics) *BaseProcessor {
    return &BaseProcessor{
        processorType: processorType,
        metrics:       metrics,
    }
}

// Type returns the processor type
func (p *BaseProcessor) Type() string {
    return p.processorType
}

// HandleError provides default error handling
func (p *BaseProcessor) HandleError(ctx context.Context, msg *commonQueue.Message, err error) error {
    p.metrics.IncErrors(p.processorType)
    return err // Return error to trigger message requeue
}
```

### Phase 2: Implement Specific Workers

#### Step 2.1: Profile Worker Implementation

**File: `services/workers/profile-worker/cmd/main.go`**

```go
package main

import (
    "log"
    "os"

    "github.com/fernandobarroso/workers/common/base"
    "github.com/fernandobarroso/workers/profile-worker/internal/processors"
)

func main() {
    // Configuration
    config := &base.WorkerConfig{
        WorkerType:    "profile",
        QueueName:     getEnv("QUEUE_NAME", "profile-processing"),
        ExchangeName:  getEnv("EXCHANGE_NAME", "profile-tasks"),
        RoutingKey:    getEnv("ROUTING_KEY", "profile.task"),
        PrefetchCount: getEnvInt("PREFETCH_COUNT", 1),
        HTTPPort:      getEnv("HTTP_PORT", "8080"),
    }

    // Create processor
    processor := processors.NewProfileProcessor()

    // Create worker
    worker, err := base.NewBaseWorker(config, processor)
    if err != nil {
        log.Fatalf("Failed to create worker: %v", err)
    }

    // Run worker
    if err := worker.Run(); err != nil {
        log.Fatalf("Worker error: %v", err)
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
    // Implementation for integer environment variables
    // ... (simplified for brevity)
    return defaultValue
}
```

**File: `services/workers/profile-worker/internal/processors/profile_processor.go`**

```go
package processors

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    commonQueue "github.com/fernandobarroso/common/queue"
    "github.com/fernandobarroso/workers/common/processors"
)

// ProfileMessage represents a profile processing message
type ProfileMessage struct {
    ProfileID string                 `json:"profile_id"`
    Action    string                 `json:"action"`
    Data      map[string]interface{} `json:"data"`
    Timestamp time.Time              `json:"timestamp"`
}

// ProfileProcessor handles profile-specific message processing
type ProfileProcessor struct {
    *processors.BaseProcessor
}

// NewProfileProcessor creates a new profile processor
func NewProfileProcessor() *ProfileProcessor {
    // Create metrics (implementation details omitted for brevity)
    metrics := &DefaultMetrics{} // Implement ProcessorMetrics interface

    return &ProfileProcessor{
        BaseProcessor: processors.NewBaseProcessor("profile", metrics),
    }
}

// Process processes a profile message
func (p *ProfileProcessor) Process(ctx context.Context, msg *commonQueue.Message) error {
    start := time.Now()
    defer func() {
        duration := time.Since(start).Seconds()
        p.BaseProcessor.ObserveProcessingTime(p.Type(), duration)
    }()

    // Validate message
    if err := p.Validate(msg); err != nil {
        return p.HandleError(ctx, msg, fmt.Errorf("validation failed: %w", err))
    }

    // Parse profile message
    var profileMsg ProfileMessage
    if err := json.Unmarshal(msg.Payload, &profileMsg); err != nil {
        return p.HandleError(ctx, msg, fmt.Errorf("failed to unmarshal: %w", err))
    }

    // Process based on action
    switch profileMsg.Action {
    case "update":
        return p.handleUpdate(ctx, &profileMsg)
    case "delete":
        return p.handleDelete(ctx, &profileMsg)
    default:
        return p.HandleError(ctx, msg, fmt.Errorf("unknown action: %s", profileMsg.Action))
    }
}

// Validate validates the message structure
func (p *ProfileProcessor) Validate(msg *commonQueue.Message) error {
    if msg.Type != "profile" {
        return fmt.Errorf("invalid message type: %s", msg.Type)
    }
    if len(msg.Payload) == 0 {
        return fmt.Errorf("empty payload")
    }
    return nil
}

func (p *ProfileProcessor) handleUpdate(ctx context.Context, msg *ProfileMessage) error {
    // Implement profile update logic
    // For now, simulate processing
    time.Sleep(2 * time.Second)

    p.BaseProcessor.IncProcessed(p.Type())
    return nil
}

func (p *ProfileProcessor) handleDelete(ctx context.Context, msg *ProfileMessage) error {
    // Implement profile delete logic
    // For now, simulate processing
    time.Sleep(1 * time.Second)

    p.BaseProcessor.IncProcessed(p.Type())
    return nil
}
```

#### Step 2.2: Notification Worker Implementation

**File: `services/workers/notification-worker/cmd/main.go`**

```go
package main

import (
    "log"
    "os"

    "github.com/fernandobarroso/workers/common/base"
    "github.com/fernandobarroso/workers/notification-worker/internal/processors"
)

func main() {
    config := &base.WorkerConfig{
        WorkerType:    "notification",
        QueueName:     getEnv("QUEUE_NAME", "notification-processing"),
        ExchangeName:  getEnv("EXCHANGE_NAME", "notification-tasks"),
        RoutingKey:    getEnv("ROUTING_KEY", "notification.send"),
        PrefetchCount: getEnvInt("PREFETCH_COUNT", 5), // Higher throughput for notifications
        HTTPPort:      getEnv("HTTP_PORT", "8080"),
    }

    processor := processors.NewNotificationProcessor()
    worker, err := base.NewBaseWorker(config, processor)
    if err != nil {
        log.Fatalf("Failed to create worker: %v", err)
    }

    if err := worker.Run(); err != nil {
        log.Fatalf("Worker error: %v", err)
    }
}

// getEnv and getEnvInt implementations...
```

**File: `services/workers/notification-worker/internal/processors/notification_processor.go`**

```go
package processors

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    commonQueue "github.com/fernandobarroso/common/queue"
    "github.com/fernandobarroso/workers/common/processors"
)

// NotificationMessage represents a notification message
type NotificationMessage struct {
    UserID      string            `json:"user_id"`
    Type        string            `json:"type"` // email, sms, push
    Template    string            `json:"template"`
    Data        map[string]string `json:"data"`
    Priority    string            `json:"priority"` // high, normal, low
    ScheduledAt *time.Time        `json:"scheduled_at,omitempty"`
}

// NotificationProcessor handles notification processing
type NotificationProcessor struct {
    *processors.BaseProcessor
}

// NewNotificationProcessor creates a new notification processor
func NewNotificationProcessor() *NotificationProcessor {
    metrics := &DefaultMetrics{}
    return &NotificationProcessor{
        BaseProcessor: processors.NewBaseProcessor("notification", metrics),
    }
}

// Process processes a notification message
func (p *NotificationProcessor) Process(ctx context.Context, msg *commonQueue.Message) error {
    start := time.Now()
    defer func() {
        duration := time.Since(start).Seconds()
        p.BaseProcessor.ObserveProcessingTime(p.Type(), duration)
    }()

    if err := p.Validate(msg); err != nil {
        return p.HandleError(ctx, msg, err)
    }

    var notificationMsg NotificationMessage
    if err := json.Unmarshal(msg.Payload, &notificationMsg); err != nil {
        return p.HandleError(ctx, msg, fmt.Errorf("failed to unmarshal: %w", err))
    }

    // Check if scheduled
    if notificationMsg.ScheduledAt != nil && notificationMsg.ScheduledAt.After(time.Now()) {
        // Requeue for later (or implement scheduling mechanism)
        return fmt.Errorf("message scheduled for later: %v", notificationMsg.ScheduledAt)
    }

    // Process based on type
    switch notificationMsg.Type {
    case "email":
        return p.sendEmail(ctx, &notificationMsg)
    case "sms":
        return p.sendSMS(ctx, &notificationMsg)
    case "push":
        return p.sendPush(ctx, &notificationMsg)
    default:
        return p.HandleError(ctx, msg, fmt.Errorf("unknown notification type: %s", notificationMsg.Type))
    }
}

// Validate validates notification message
func (p *NotificationProcessor) Validate(msg *commonQueue.Message) error {
    if msg.Type != "notification" {
        return fmt.Errorf("invalid message type: %s", msg.Type)
    }
    return nil
}

func (p *NotificationProcessor) sendEmail(ctx context.Context, msg *NotificationMessage) error {
    // Implement email sending logic
    // Simulate processing time based on priority
    switch msg.Priority {
    case "high":
        time.Sleep(100 * time.Millisecond)
    case "normal":
        time.Sleep(500 * time.Millisecond)
    case "low":
        time.Sleep(1 * time.Second)
    }

    p.BaseProcessor.IncProcessed(p.Type())
    return nil
}

func (p *NotificationProcessor) sendSMS(ctx context.Context, msg *NotificationMessage) error {
    // Implement SMS sending logic
    time.Sleep(200 * time.Millisecond)
    p.BaseProcessor.IncProcessed(p.Type())
    return nil
}

func (p *NotificationProcessor) sendPush(ctx context.Context, msg *NotificationMessage) error {
    // Implement push notification logic
    time.Sleep(50 * time.Millisecond)
    p.BaseProcessor.IncProcessed(p.Type())
    return nil
}
```

### Phase 3: Independent Dockerfiles

#### Step 3.1: Profile Worker Dockerfile

**File: `services/workers/profile-worker/Dockerfile`**

```dockerfile
# Build stage
FROM golang:1.24.0-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /workspace

# Copy the entire workspace for shared dependencies
COPY . .

# Set working directory to the profile worker
WORKDIR /workspace/workers/profile-worker

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o profile-worker ./cmd/main.go

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /workspace/workers/profile-worker/profile-worker .

# Set ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Set environment variables
ENV TZ=UTC

# Expose health check port
EXPOSE 8080

# Run the application
CMD ["./profile-worker"]
```

#### Step 3.2: Notification Worker Dockerfile

**File: `services/workers/notification-worker/Dockerfile`**

```dockerfile
# Build stage
FROM golang:1.24.0-alpine AS builder

RUN apk add --no-cache git
WORKDIR /workspace

# Copy workspace
COPY . .

# Build notification worker
WORKDIR /workspace/workers/notification-worker
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o notification-worker ./cmd/main.go

# Final stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -g '' appuser

WORKDIR /app
COPY --from=builder /workspace/workers/notification-worker/notification-worker .
RUN chown -R appuser:appuser /app

USER appuser
ENV TZ=UTC
EXPOSE 8080

CMD ["./notification-worker"]
```

### Phase 4: Kubernetes Manifests for Independent Scaling

#### Step 4.1: Profile Worker Kubernetes Manifests

**File: `services/workers/profile-worker/k8s/deployment.yaml`**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-worker
  labels:
    app: profile-worker
    component: worker
    worker-type: profile
spec:
  replicas: 2 # Start with 2 replicas
  selector:
    matchLabels:
      app: profile-worker
  template:
    metadata:
      labels:
        app: profile-worker
        component: worker
        worker-type: profile
    spec:
      containers:
        - name: profile-worker
          image: profile-worker:latest
          env:
            - name: RABBITMQ_HOST
              value: "rabbitmq"
            - name: RABBITMQ_PORT
              value: "5672"
            - name: RABBITMQ_USER
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-credentials
                  key: username
            - name: RABBITMQ_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-credentials
                  key: password
            - name: QUEUE_NAME
              value: "profile-processing"
            - name: EXCHANGE_NAME
              value: "profile-tasks"
            - name: ROUTING_KEY
              value: "profile.task"
            - name: PREFETCH_COUNT
              value: "1"
            - name: LOG_LEVEL
              value: "info"
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: profile-worker
  labels:
    app: profile-worker
spec:
  selector:
    app: profile-worker
  ports:
    - name: http
      port: 8080
      targetPort: 8080
  type: ClusterIP
```

#### Step 4.2: Notification Worker with Higher Scaling

**File: `services/workers/notification-worker/k8s/deployment.yaml`**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: notification-worker
  labels:
    app: notification-worker
    component: worker
    worker-type: notification
spec:
  replicas: 5 # Higher replicas for notification throughput
  selector:
    matchLabels:
      app: notification-worker
  template:
    metadata:
      labels:
        app: notification-worker
        component: worker
        worker-type: notification
    spec:
      containers:
        - name: notification-worker
          image: notification-worker:latest
          env:
            - name: RABBITMQ_HOST
              value: "rabbitmq"
            - name: RABBITMQ_PORT
              value: "5672"
            - name: RABBITMQ_USER
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-credentials
                  key: username
            - name: RABBITMQ_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-credentials
                  key: password
            - name: QUEUE_NAME
              value: "notification-processing"
            - name: EXCHANGE_NAME
              value: "notification-tasks"
            - name: ROUTING_KEY
              value: "notification.send"
            - name: PREFETCH_COUNT
              value: "5" # Higher prefetch for notifications
            - name: LOG_LEVEL
              value: "info"
          resources:
            requests:
              cpu: 50m # Lower CPU for notifications
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 256Mi
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: notification-worker
  labels:
    app: notification-worker
spec:
  selector:
    app: notification-worker
  ports:
    - name: http
      port: 8080
      targetPort: 8080
  type: ClusterIP
```

### Phase 5: Horizontal Pod Autoscaler (HPA) Configuration

#### Step 5.1: Profile Worker HPA

**File: `services/workers/profile-worker/k8s/hpa.yaml`**

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: profile-worker-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: profile-worker
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300 # 5 minutes
      policies:
        - type: Percent
          value: 50
          periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 60 # 1 minute
      policies:
        - type: Percent
          value: 100
          periodSeconds: 60
```

#### Step 5.2: Notification Worker HPA (More Aggressive Scaling)

**File: `services/workers/notification-worker/k8s/hpa.yaml`**

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: notification-worker-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: notification-worker
  minReplicas: 3
  maxReplicas: 20 # Higher max for notification bursts
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 60 # Lower threshold for faster scaling
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 70
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 180 # Faster scale down
      policies:
        - type: Percent
          value: 50
          periodSeconds: 30
    scaleUp:
      stabilizationWindowSeconds: 30 # Very fast scale up
      policies:
        - type: Percent
          value: 200
          periodSeconds: 30
```

## Scaling Considerations and Extra Configurations

### 1. Queue-Based Scaling with KEDA

For more sophisticated scaling based on queue depth, use KEDA (Kubernetes Event-driven Autoscaling):

**File: `services/workers/profile-worker/k8s/keda-scaler.yaml`**

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: profile-worker-scaler
spec:
  scaleTargetRef:
    name: profile-worker
  minReplicaCount: 2
  maxReplicaCount: 15
  triggers:
    - type: rabbitmq
      metadata:
        host: "amqp://user:password@rabbitmq:5672/"
        queueName: "profile-processing"
        queueLength: "10" # Scale up when queue has >10 messages
        excludeUnacknowledged: "true"
```

### 2. Resource Quotas and Limits

**File: `services/workers/k8s/resource-quota.yaml`**

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: workers-quota
spec:
  hard:
    requests.cpu: "4" # Total CPU requests for all workers
    requests.memory: "8Gi" # Total memory requests
    limits.cpu: "8" # Total CPU limits
    limits.memory: "16Gi" # Total memory limits
    pods: "50" # Maximum pods for workers
---
apiVersion: v1
kind: LimitRange
metadata:
  name: workers-limits
spec:
  limits:
    - type: Container
      default:
        cpu: "200m"
        memory: "256Mi"
      defaultRequest:
        cpu: "50m"
        memory: "64Mi"
      max:
        cpu: "1"
        memory: "1Gi"
      min:
        cpu: "10m"
        memory: "32Mi"
```

### 3. Pod Disruption Budgets

**File: `services/workers/profile-worker/k8s/pdb.yaml`**

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: profile-worker-pdb
spec:
  minAvailable: 1 # Always keep at least 1 pod running
  selector:
    matchLabels:
      app: profile-worker
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: notification-worker-pdb
spec:
  minAvailable: 2 # Keep at least 2 notification workers
  selector:
    matchLabels:
      app: notification-worker
```

### 4. Monitoring and Alerting

**File: `services/workers/k8s/servicemonitor.yaml`**

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: workers-metrics
spec:
  selector:
    matchLabels:
      component: worker
  endpoints:
    - port: http
      path: /metrics
      interval: 30s
```

**File: `services/workers/k8s/alerts.yaml`**

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: worker-alerts
spec:
  groups:
    - name: worker.rules
      rules:
        - alert: WorkerHighErrorRate
          expr: rate(worker_processing_errors_total[5m]) > 0.1
          for: 2m
          labels:
            severity: warning
          annotations:
            summary: "High error rate in {{ $labels.worker_type }} worker"

        - alert: WorkerQueueBacklog
          expr: rabbitmq_queue_messages > 100
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "Queue backlog in {{ $labels.queue }}"

        - alert: WorkerDown
          expr: up{job="workers"} == 0
          for: 1m
          labels:
            severity: critical
          annotations:
            summary: "Worker {{ $labels.instance }} is down"
```

## Deployment and Management Commands

### Building and Deploying Workers

```bash
# Build all worker images
for worker in profile-worker notification-worker analytics-worker email-worker; do
    docker build -t $worker:latest -f services/workers/$worker/Dockerfile .
    kind load docker-image $worker:latest
done

# Deploy workers
kubectl apply -f services/workers/profile-worker/k8s/
kubectl apply -f services/workers/notification-worker/k8s/

# Scale specific worker
kubectl scale deployment profile-worker --replicas=5

# Check worker status
kubectl get pods -l component=worker
kubectl get hpa

# View worker logs
kubectl logs -l app=profile-worker -f
```

### Monitoring Commands

```bash
# Check queue depths
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_queues

# Check worker metrics
kubectl port-forward svc/profile-worker 8080:8080
curl http://localhost:8080/metrics

# Check scaling events
kubectl describe hpa profile-worker-hpa
```

## Best Practices and Recommendations

### 1. **Resource Planning**

- **Profile Workers**: CPU-intensive, moderate memory
- **Notification Workers**: I/O-intensive, low CPU, burst scaling
- **Analytics Workers**: Memory-intensive, batch processing
- **Email Workers**: I/O-intensive, rate-limited scaling

### 2. **Queue Strategy**

- **Separate Queues**: Each worker type has its own queue
- **Priority Queues**: Use RabbitMQ priority queues for urgent messages
- **Dead Letter Queues**: Configure DLQs for failed messages
- **TTL**: Set appropriate message TTL based on business requirements

### 3. **Scaling Strategy**

- **Conservative for Critical Workers**: Profile workers scale conservatively
- **Aggressive for High-Volume**: Notification workers scale aggressively
- **Queue-Depth Based**: Use KEDA for queue-depth-based scaling
- **Resource Limits**: Set appropriate resource limits to prevent resource starvation

### 4. **Monitoring Strategy**

- **Per-Worker Metrics**: Each worker exposes its own metrics
- **Queue Metrics**: Monitor queue depth and processing rates
- **Error Tracking**: Track error rates and types per worker
- **Performance Metrics**: Monitor processing times and throughput

### 5. **Deployment Strategy**

- **Rolling Updates**: Use rolling updates for zero-downtime deployments
- **Blue-Green**: Consider blue-green deployments for critical workers
- **Canary**: Use canary deployments for testing new worker versions
- **Pod Disruption Budgets**: Ensure minimum availability during updates

This comprehensive guide provides a complete strategy for implementing multiple, independently scalable workers while maintaining code reuse and operational efficiency.
