# Queue Service Revamp Implementation Plan

## Overview

This document outlines the detailed implementation plan for revamping the queue-service to use the new common queue package. The service will act as a publisher, sending messages to RabbitMQ for processing by the worker-service.

## Package Structure

```
services/queue-service/
├── cmd/
│   └── server/
│       └── main.go                 # Main entry point
├── internal/
│   ├── adapters/
│   │   └── queue/
│   │       ├── publisher.go        # Service-specific publisher
│   │       ├── config.go           # Service-specific config
│   │       └── middleware.go       # Publisher middleware
│   ├── domain/
│   │   ├── message.go             # Service-specific message model
│   │   └── types.go               # Domain types
│   ├── handlers/
│   │   └── http/
│   │       ├── queue.go           # HTTP handlers
│   │       └── health.go          # Health check handler
│   └── server/
│       └── server.go              # HTTP server setup
├── pkg/
│   └── middleware/
│       └── logging.go             # HTTP logging middleware
└── k8s/
    └── base/
        ├── deployment.yaml        # Updated deployment
        ├── service.yaml           # Service definition
        └── secrets.yaml           # RabbitMQ credentials
```

## Detailed Implementation

### 1. Service-Specific Message Model (internal/domain/message.go)

```go
package domain

import (
    "time"
    "github.com/yourusername/common/queue"
)

type ProfileMessage struct {
    queue.Message
    ProfileID  string    `json:"profile_id"`
    Action     string    `json:"action"`
    Data       any       `json:"data"`
    CreatedAt  time.Time `json:"created_at"`
}

func NewProfileMessage(profileID, action string, data any) (*ProfileMessage, error) {
    msg, err := queue.NewMessage(
        generateMessageID(),
        "profile.update",
        data,
    )
    if err != nil {
        return nil, err
    }

    return &ProfileMessage{
        Message:   *msg,
        ProfileID: profileID,
        Action:    action,
        Data:      data,
        CreatedAt: time.Now(),
    }, nil
}
```

### 2. Service-Specific Publisher (internal/adapters/queue/publisher.go)

```go
package queue

import (
    "context"
    "github.com/yourusername/common/queue"
    "github.com/yourusername/queue-service/internal/domain"
)

type Publisher struct {
    *queue.Publisher
    metrics *Metrics
}

type Metrics struct {
    // Add service-specific metrics
    publishLatency prometheus.Histogram
    publishErrors  prometheus.Counter
}

func NewPublisher(config *queue.Config) (*Publisher, error) {
    basePublisher, err := queue.NewPublisher(config)
    if err != nil {
        return nil, err
    }

    metrics := &Metrics{
        publishLatency: prometheus.NewHistogram(
            prometheus.HistogramOpts{
                Name: "queue_service_publish_latency_seconds",
                Help: "Time taken to publish messages",
            },
        ),
        publishErrors: prometheus.NewCounter(
            prometheus.CounterOpts{
                Name: "queue_service_publish_errors_total",
                Help: "Total number of publish errors",
            },
        ),
    }

    return &Publisher{
        Publisher: basePublisher,
        metrics:   metrics,
    }, nil
}

func (p *Publisher) PublishProfileMessage(ctx context.Context, msg *domain.ProfileMessage) error {
    timer := prometheus.NewTimer(p.metrics.publishLatency)
    defer timer.ObserveDuration()

    err := p.PublishMessage(ctx, &msg.Message)
    if err != nil {
        p.metrics.publishErrors.Inc()
        return err
    }

    return nil
}
```

### 3. HTTP Handlers (internal/handlers/http/queue.go)

```go
package http

import (
    "encoding/json"
    "net/http"
    "github.com/yourusername/queue-service/internal/domain"
    "github.com/yourusername/queue-service/internal/adapters/queue"
)

type QueueHandler struct {
    publisher *queue.Publisher
}

func NewQueueHandler(publisher *queue.Publisher) *QueueHandler {
    return &QueueHandler{
        publisher: publisher,
    }
}

func (h *QueueHandler) PublishProfile(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ProfileID string      `json:"profile_id"`
        Action    string      `json:"action"`
        Data      interface{} `json:"data"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    msg, err := domain.NewProfileMessage(req.ProfileID, req.Action, req.Data)
    if err != nil {
        http.Error(w, "Failed to create message", http.StatusInternalServerError)
        return
    }

    if err := h.publisher.PublishProfileMessage(r.Context(), msg); err != nil {
        http.Error(w, "Failed to publish message", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "message published",
        "id":     msg.ID,
    })
}
```

### 4. Server Setup (internal/server/server.go)

```go
package server

import (
    "net/http"
    "github.com/yourusername/queue-service/internal/handlers/http"
    "github.com/yourusername/queue-service/internal/adapters/queue"
    "github.com/yourusername/queue-service/pkg/middleware"
)

type Server struct {
    httpServer *http.Server
    publisher  *queue.Publisher
}

func NewServer(addr string, publisher *queue.Publisher) *Server {
    queueHandler := http.NewQueueHandler(publisher)

    mux := http.NewServeMux()
    mux.HandleFunc("/api/v1/queue/profile", queueHandler.PublishProfile)
    mux.HandleFunc("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))

    // Add middleware
    handler := middleware.Logging(mux)

    return &Server{
        httpServer: &http.Server{
            Addr:    addr,
            Handler: handler,
        },
        publisher: publisher,
    }
}

func (s *Server) Start() error {
    return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
    return s.httpServer.Shutdown(ctx)
}
```

### 5. Main Application (cmd/server/main.go)

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "github.com/yourusername/common/queue"
    "github.com/yourusername/queue-service/internal/server"
)

func main() {
    // Load configuration
    config := queue.NewConfig()
    config.URL = os.Getenv("RABBITMQ_URL")
    config.Exchange = os.Getenv("RABBITMQ_EXCHANGE")
    config.RoutingKey = os.Getenv("RABBITMQ_ROUTING_KEY")

    // Create publisher
    publisher, err := queue.NewPublisher(config)
    if err != nil {
        log.Fatal(err)
    }
    defer publisher.Close()

    // Create and start server
    srv := server.NewServer(":8080", publisher)
    go func() {
        if err := srv.Start(); err != nil {
            log.Fatal(err)
        }
    }()

    // Handle graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Fatal(err)
    }
}
```

## Implementation Steps

1. **Phase 1: Core Package Setup**

   - [ ] Create directory structure
   - [ ] Set up Go module
   - [ ] Add dependencies
   - [ ] Create basic configuration

2. **Phase 2: Domain Implementation**

   - [ ] Implement message model
   - [ ] Add domain types
   - [ ] Create message builders
   - [ ] Add validation

3. **Phase 3: Publisher Implementation**

   - [ ] Create service-specific publisher
   - [ ] Add metrics
   - [ ] Implement error handling
   - [ ] Add retry logic

4. **Phase 4: HTTP Layer**

   - [ ] Implement handlers
   - [ ] Add middleware
   - [ ] Set up routing
   - [ ] Add health checks

5. **Phase 5: Server Setup**

   - [ ] Create server package
   - [ ] Implement graceful shutdown
   - [ ] Add configuration
   - [ ] Set up logging

6. **Phase 6: Testing**
   - [ ] Write unit tests
   - [ ] Add integration tests
   - [ ] Implement error scenarios
   - [ ] Add performance tests

## Kubernetes Configuration

### Deployment (k8s/base/deployment.yaml)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: queue-service
spec:
  replicas: 2
  selector:
    matchLabels:
      app: queue-service
  template:
    metadata:
      labels:
        app: queue-service
    spec:
      containers:
        - name: queue-service
          image: queue-service:latest
          ports:
            - containerPort: 8080
          env:
            - name: RABBITMQ_URL
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-credentials
                  key: url
            - name: RABBITMQ_EXCHANGE
              value: "profile-tasks"
            - name: RABBITMQ_ROUTING_KEY
              value: "profile.task"
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 15
            periodSeconds: 20
```

### Service (k8s/base/service.yaml)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: queue-service
spec:
  selector:
    app: queue-service
  ports:
    - port: 80
      targetPort: 8080
  type: ClusterIP
```

## Next Steps

1. **Immediate Actions**

   - [ ] Set up the project structure
   - [ ] Implement core functionality
   - [ ] Write initial tests
   - [ ] Create documentation

2. **Future Improvements**
   - [ ] Add circuit breaker
   - [ ] Implement rate limiting
   - [ ] Add request tracing
   - [ ] Implement metrics dashboard
