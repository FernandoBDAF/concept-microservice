# Operational Workers - Implementation Plan

**Project:** operational-workers  
**Language:** Go  
**Status:** 📋 Architecture planned  
**Session Focus:** Create Go workers for email, image, and profile task processing

---

## 1. Overview

The operational-workers project contains Go-based task processors:
- **Email Worker** - Sends emails (welcome, notifications, etc.)
- **Image Worker** - Processes images (resize, compress, etc.)
- **Profile Worker** - Handles profile-related background tasks

### Why Go
- Lightweight containers (~50MB vs ~2GB for Python)
- Fast startup (~2s vs ~60s)
- Efficient resource usage
- Consistent with api-service
- Existing code in legacy worker-service

### Architecture
All workers share a common foundation but run as separate deployments for independent scaling.

---

## 2. Source References

| Component | Source Location | Notes |
|-----------|-----------------|-------|
| Common Foundation | `legacy_project/services/worker-service/services/workers/common/` | Base worker, utilities |
| Queue Package | `legacy_project/services/common/queue/` | RabbitMQ consumer |
| Email Worker | `legacy_project/services/worker-service/services/workers/email-worker/` | Complete implementation |
| Image Worker | `legacy_project/services/worker-service/services/workers/image-worker/` | Complete implementation |

---

## 3. Implementation Tasks

### Phase 1: Project Setup (Day 1)

#### Task 1.1: Create Directory Structure

```bash
mkdir -p graph-worker/operational-workers/{cmd/{email-worker,image-worker,profile-worker},internal/{common/{base,processors,queue,metrics},processors/{email,image,profile},domain},deployments/kubernetes/{email-worker,image-worker,profile-worker}}
```

**Final structure:**
```
operational-workers/
├── cmd/
│   ├── email-worker/
│   │   └── main.go
│   ├── image-worker/
│   │   └── main.go
│   └── profile-worker/
│       └── main.go
├── internal/
│   ├── common/
│   │   ├── base/
│   │   │   ├── worker.go
│   │   │   └── http_server.go
│   │   ├── processors/
│   │   │   └── processor.go
│   │   ├── queue/
│   │   │   ├── connection.go
│   │   │   ├── consumer.go
│   │   │   └── message.go
│   │   ├── metrics/
│   │   │   └── metrics.go
│   │   └── utils/
│   │       └── logger.go
│   ├── processors/
│   │   ├── email/
│   │   │   ├── processor.go
│   │   │   └── message.go
│   │   ├── image/
│   │   │   ├── processor.go
│   │   │   └── message.go
│   │   └── profile/
│   │       ├── processor.go
│   │       └── message.go
│   └── domain/
│       └── worker.go
├── deployments/
│   └── kubernetes/
│       ├── email-worker/
│       │   ├── deployment.yaml
│       │   ├── service.yaml
│       │   └── configmap.yaml
│       ├── image-worker/
│       │   └── ...
│       └── profile-worker/
│           └── ...
├── go.mod
├── go.sum
├── Dockerfile.email
├── Dockerfile.image
├── Dockerfile.profile
├── Makefile
├── IMPLEMENTATION_PLAN.md
└── README.md
```

#### Task 1.2: Initialize Go Module

```bash
cd graph-worker/operational-workers
go mod init github.com/fernandobarroso/microservices/operational-workers
```

**File:** `go.mod`

```go
module github.com/fernandobarroso/microservices/operational-workers

go 1.22

require (
    github.com/gin-gonic/gin v1.10.1
    github.com/rabbitmq/amqp091-go v1.10.0
    github.com/prometheus/client_golang v1.22.0
    go.uber.org/zap v1.27.0
    github.com/google/uuid v1.6.0
    github.com/spf13/viper v1.19.0
)
```

---

### Phase 2: Copy Common Foundation (Day 2)

#### Task 2.1: Copy Worker Common Code

**Source:** `legacy_project/services/worker-service/services/workers/common/`

```bash
# From repository root
cp -r legacy_project/services/worker-service/services/workers/common/base/* \
  graph-worker/operational-workers/internal/common/base/

cp -r legacy_project/services/worker-service/services/workers/common/processors/* \
  graph-worker/operational-workers/internal/common/processors/

cp -r legacy_project/services/worker-service/services/workers/common/utils/* \
  graph-worker/operational-workers/internal/common/utils/
```

#### Task 2.2: Copy Queue Package (Self-Contained)

**Source:** `legacy_project/services/common/queue/`

```bash
cp -r legacy_project/services/common/queue/* \
  graph-worker/operational-workers/internal/common/queue/
```

#### Task 2.3: Update Import Paths

Update all Go files to use new import paths:

```go
// OLD imports
import "github.com/fernandobarroso/common/queue"

// NEW imports
import "github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
```

**Files to update:**
- `internal/common/base/worker.go`
- `internal/common/base/http_server.go`
- `internal/common/processors/processor.go`
- All `main.go` files

---

### Phase 3: Email Worker (Day 3)

#### Task 3.1: Copy Email Worker

```bash
cp legacy_project/services/worker-service/services/workers/email-worker/cmd/main.go \
  graph-worker/operational-workers/cmd/email-worker/

cp legacy_project/services/worker-service/services/workers/email-worker/internal/processors/email_processor.go \
  graph-worker/operational-workers/internal/processors/email/processor.go

cp legacy_project/services/worker-service/services/workers/email-worker/internal/domain/message.go \
  graph-worker/operational-workers/internal/processors/email/message.go
```

#### Task 3.2: Update Email Worker Imports

**File:** `cmd/email-worker/main.go`

```go
package main

import (
    "os"
    "os/signal"
    "syscall"

    "github.com/fernandobarroso/microservices/operational-workers/internal/common/base"
    "github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
    "github.com/fernandobarroso/microservices/operational-workers/internal/processors/email"

    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    // Load configuration
    config := loadConfig()

    // Create processor
    processor := email.NewProcessor(config)

    // Create worker
    worker := base.NewWorker(
        "email-worker",
        config.RabbitMQ,
        processor,
        logger,
    )

    // Start worker
    go worker.Start()

    // Wait for shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Shutting down email worker")
    worker.Stop()
}

func loadConfig() *Config {
    return &Config{
        RabbitMQ: queue.Config{
            Host:     getEnv("RABBITMQ_HOST", "rabbitmq"),
            Port:     getEnvInt("RABBITMQ_PORT", 5672),
            User:     getEnv("RABBITMQ_USER", "guest"),
            Password: os.Getenv("RABBITMQ_PASSWORD"),
            Queue:    "email-processing",
        },
        HealthPort: getEnvInt("HEALTH_PORT", 8080),
    }
}
```

#### Task 3.3: Email Processor

**File:** `internal/processors/email/processor.go`

```go
package email

import (
    "context"
    "encoding/json"
    "time"

    "github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
    "go.uber.org/zap"
)

type Processor struct {
    logger *zap.Logger
    // Add SMTP client or email service client
}

func NewProcessor(config interface{}) *Processor {
    return &Processor{
        logger: zap.L().Named("email-processor"),
    }
}

func (p *Processor) Process(ctx context.Context, msg *queue.Message) error {
    var emailMsg EmailMessage
    if err := json.Unmarshal(msg.Body, &emailMsg); err != nil {
        p.logger.Error("Failed to unmarshal message", zap.Error(err))
        return err
    }

    p.logger.Info("Processing email",
        zap.String("type", emailMsg.EmailType),
        zap.String("recipient", emailMsg.Recipient),
    )

    // TODO: Implement actual email sending
    // For now, simulate processing
    time.Sleep(100 * time.Millisecond)

    p.logger.Info("Email sent successfully",
        zap.String("recipient", emailMsg.Recipient),
    )

    return nil
}

func (p *Processor) Type() string {
    return "email"
}
```

#### Task 3.4: Email Message Model

**File:** `internal/processors/email/message.go`

```go
package email

type EmailMessage struct {
    ID          string            `json:"id"`
    Type        string            `json:"type"`
    Timestamp   string            `json:"timestamp"`
    Payload     EmailPayload      `json:"payload"`
    Metadata    map[string]string `json:"metadata"`
}

type EmailPayload struct {
    EmailType   string            `json:"email_type"`
    Recipient   string            `json:"recipient"`
    Subject     string            `json:"subject,omitempty"`
    TemplateID  string            `json:"template_id,omitempty"`
    Variables   map[string]string `json:"variables,omitempty"`
}
```

---

### Phase 4: Image Worker (Day 4)

#### Task 4.1: Copy Image Worker

```bash
cp legacy_project/services/worker-service/services/workers/image-worker/cmd/main.go \
  graph-worker/operational-workers/cmd/image-worker/

cp legacy_project/services/worker-service/services/workers/image-worker/internal/processors/image_processor.go \
  graph-worker/operational-workers/internal/processors/image/processor.go

cp legacy_project/services/worker-service/services/workers/image-worker/internal/domain/message.go \
  graph-worker/operational-workers/internal/processors/image/message.go
```

#### Task 4.2: Image Processor

**File:** `internal/processors/image/processor.go`

```go
package image

import (
    "context"
    "encoding/json"
    "time"

    "github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
    "go.uber.org/zap"
)

type Processor struct {
    logger *zap.Logger
}

func NewProcessor(config interface{}) *Processor {
    return &Processor{
        logger: zap.L().Named("image-processor"),
    }
}

func (p *Processor) Process(ctx context.Context, msg *queue.Message) error {
    var imageMsg ImageMessage
    if err := json.Unmarshal(msg.Body, &imageMsg); err != nil {
        p.logger.Error("Failed to unmarshal message", zap.Error(err))
        return err
    }

    p.logger.Info("Processing image",
        zap.String("operation", imageMsg.Payload.Operation),
        zap.String("source", imageMsg.Payload.SourceURL),
    )

    // TODO: Implement actual image processing
    // For now, simulate processing
    time.Sleep(500 * time.Millisecond)

    p.logger.Info("Image processed successfully")

    return nil
}

func (p *Processor) Type() string {
    return "image"
}
```

#### Task 4.3: Image Message Model

**File:** `internal/processors/image/message.go`

```go
package image

type ImageMessage struct {
    ID          string            `json:"id"`
    Type        string            `json:"type"`
    Timestamp   string            `json:"timestamp"`
    Payload     ImagePayload      `json:"payload"`
    Metadata    map[string]string `json:"metadata"`
}

type ImagePayload struct {
    Operation   string `json:"operation"`  // resize, compress, convert
    SourceURL   string `json:"source_url"`
    TargetPath  string `json:"target_path"`
    Width       int    `json:"width,omitempty"`
    Height      int    `json:"height,omitempty"`
    Quality     int    `json:"quality,omitempty"`
    Format      string `json:"format,omitempty"`
}
```

---

### Phase 5: Profile Worker (Day 5)

**NEW Worker Type** - Create from scratch based on email/image pattern.

#### Task 5.1: Create Profile Worker Entry Point

**File:** `cmd/profile-worker/main.go`

```go
package main

import (
    "os"
    "os/signal"
    "syscall"

    "github.com/fernandobarroso/microservices/operational-workers/internal/common/base"
    "github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
    "github.com/fernandobarroso/microservices/operational-workers/internal/processors/profile"

    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    config := loadConfig()

    processor := profile.NewProcessor(config)

    worker := base.NewWorker(
        "profile-worker",
        config.RabbitMQ,
        processor,
        logger,
    )

    go worker.Start()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Shutting down profile worker")
    worker.Stop()
}

func loadConfig() *Config {
    return &Config{
        RabbitMQ: queue.Config{
            Host:     getEnv("RABBITMQ_HOST", "rabbitmq"),
            Port:     getEnvInt("RABBITMQ_PORT", 5672),
            User:     getEnv("RABBITMQ_USER", "guest"),
            Password: os.Getenv("RABBITMQ_PASSWORD"),
            Queue:    "profile-processing",
        },
        HealthPort: getEnvInt("HEALTH_PORT", 8080),
    }
}
```

#### Task 5.2: Create Profile Processor

**File:** `internal/processors/profile/processor.go`

```go
package profile

import (
    "context"
    "encoding/json"
    "time"

    "github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
    "go.uber.org/zap"
)

type Processor struct {
    logger *zap.Logger
}

func NewProcessor(config interface{}) *Processor {
    return &Processor{
        logger: zap.L().Named("profile-processor"),
    }
}

func (p *Processor) Process(ctx context.Context, msg *queue.Message) error {
    var profileMsg ProfileMessage
    if err := json.Unmarshal(msg.Body, &profileMsg); err != nil {
        p.logger.Error("Failed to unmarshal message", zap.Error(err))
        return err
    }

    p.logger.Info("Processing profile task",
        zap.String("task_type", profileMsg.Payload.TaskType),
        zap.String("profile_id", profileMsg.Payload.ProfileID),
    )

    // Route to appropriate handler based on task type
    switch profileMsg.Payload.TaskType {
    case "sync":
        return p.handleSync(ctx, &profileMsg)
    case "validate":
        return p.handleValidate(ctx, &profileMsg)
    case "enrich":
        return p.handleEnrich(ctx, &profileMsg)
    default:
        p.logger.Warn("Unknown task type", zap.String("type", profileMsg.Payload.TaskType))
    }

    return nil
}

func (p *Processor) handleSync(ctx context.Context, msg *ProfileMessage) error {
    // TODO: Implement profile sync logic
    time.Sleep(200 * time.Millisecond)
    p.logger.Info("Profile synced", zap.String("profile_id", msg.Payload.ProfileID))
    return nil
}

func (p *Processor) handleValidate(ctx context.Context, msg *ProfileMessage) error {
    // TODO: Implement profile validation logic
    time.Sleep(100 * time.Millisecond)
    p.logger.Info("Profile validated", zap.String("profile_id", msg.Payload.ProfileID))
    return nil
}

func (p *Processor) handleEnrich(ctx context.Context, msg *ProfileMessage) error {
    // TODO: Implement profile enrichment logic
    time.Sleep(300 * time.Millisecond)
    p.logger.Info("Profile enriched", zap.String("profile_id", msg.Payload.ProfileID))
    return nil
}

func (p *Processor) Type() string {
    return "profile"
}
```

#### Task 5.3: Create Profile Message Model

**File:** `internal/processors/profile/message.go`

```go
package profile

type ProfileMessage struct {
    ID          string            `json:"id"`
    Type        string            `json:"type"`
    Timestamp   string            `json:"timestamp"`
    Payload     ProfilePayload    `json:"payload"`
    Metadata    map[string]string `json:"metadata"`
}

type ProfilePayload struct {
    TaskType    string            `json:"task_type"`  // sync, validate, enrich
    ProfileID   string            `json:"profile_id"`
    UserID      string            `json:"user_id,omitempty"`
    Data        map[string]string `json:"data,omitempty"`
}
```

---

### Phase 6: Dockerfiles (Day 6)

#### Task 6.1: Email Worker Dockerfile

**File:** `Dockerfile.email`

```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o email-worker ./cmd/email-worker

# Production image
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/email-worker .

EXPOSE 8080

CMD ["./email-worker"]
```

#### Task 6.2: Image Worker Dockerfile

**File:** `Dockerfile.image`

```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o image-worker ./cmd/image-worker

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/image-worker .

EXPOSE 8080

CMD ["./image-worker"]
```

#### Task 6.3: Profile Worker Dockerfile

**File:** `Dockerfile.profile`

```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o profile-worker ./cmd/profile-worker

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/profile-worker .

EXPOSE 8080

CMD ["./profile-worker"]
```

---

### Phase 7: Kubernetes Deployment (Day 7)

#### Task 7.1: Email Worker Deployment

**File:** `deployments/kubernetes/email-worker/deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: email-worker
  labels:
    app: email-worker
spec:
  replicas: 2
  selector:
    matchLabels:
      app: email-worker
  template:
    metadata:
      labels:
        app: email-worker
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
    spec:
      containers:
        - name: email-worker
          image: email-worker:latest
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 8080
              name: http
          env:
            - name: RABBITMQ_HOST
              value: "rabbitmq"
            - name: RABBITMQ_PORT
              value: "5672"
            - name: RABBITMQ_USER
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: RABBITMQ_USER
            - name: RABBITMQ_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: RABBITMQ_PASSWORD
            - name: QUEUE_NAME
              value: "email-processing"
          resources:
            requests:
              cpu: 100m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
```

---

## 4. Makefile

**File:** `Makefile`

```makefile
.PHONY: all build test clean docker

all: build

build:
	go build -o bin/email-worker ./cmd/email-worker
	go build -o bin/image-worker ./cmd/image-worker
	go build -o bin/profile-worker ./cmd/profile-worker

test:
	go test -v ./...

clean:
	rm -rf bin/

docker:
	docker build -f Dockerfile.email -t email-worker:latest .
	docker build -f Dockerfile.image -t image-worker:latest .
	docker build -f Dockerfile.profile -t profile-worker:latest .

docker-email:
	docker build -f Dockerfile.email -t email-worker:latest .

docker-image:
	docker build -f Dockerfile.image -t image-worker:latest .

docker-profile:
	docker build -f Dockerfile.profile -t profile-worker:latest .

mod:
	go mod tidy
```

---

## 5. Environment Variables

### Common Variables (All Workers)

| Variable | Default | Description |
|----------|---------|-------------|
| `RABBITMQ_HOST` | rabbitmq | RabbitMQ host |
| `RABBITMQ_PORT` | 5672 | RabbitMQ port |
| `RABBITMQ_USER` | guest | RabbitMQ user |
| `RABBITMQ_PASSWORD` | (required) | RabbitMQ password |
| `HEALTH_PORT` | 8080 | Health check port |
| `LOG_LEVEL` | info | Log level |

### Worker-Specific Variables

| Worker | Variable | Default | Description |
|--------|----------|---------|-------------|
| email-worker | `QUEUE_NAME` | email-processing | RabbitMQ queue |
| image-worker | `QUEUE_NAME` | image-processing | RabbitMQ queue |
| profile-worker | `QUEUE_NAME` | profile-processing | RabbitMQ queue |

---

## 6. File Checklist

### Files to Copy
- [ ] `legacy_project/services/worker-service/services/workers/common/base/*` → `internal/common/base/`
- [ ] `legacy_project/services/worker-service/services/workers/common/processors/*` → `internal/common/processors/`
- [ ] `legacy_project/services/worker-service/services/workers/common/utils/*` → `internal/common/utils/`
- [ ] `legacy_project/services/common/queue/*` → `internal/common/queue/`
- [ ] `legacy_project/services/worker-service/services/workers/email-worker/` → `cmd/email-worker/`, `internal/processors/email/`
- [ ] `legacy_project/services/worker-service/services/workers/image-worker/` → `cmd/image-worker/`, `internal/processors/image/`

### Files to Create
- [ ] `go.mod`
- [ ] `Makefile`
- [ ] `cmd/profile-worker/main.go`
- [ ] `internal/processors/profile/processor.go`
- [ ] `internal/processors/profile/message.go`
- [ ] `Dockerfile.email`
- [ ] `Dockerfile.image`
- [ ] `Dockerfile.profile`
- [ ] `deployments/kubernetes/email-worker/*.yaml`
- [ ] `deployments/kubernetes/image-worker/*.yaml`
- [ ] `deployments/kubernetes/profile-worker/*.yaml`
- [ ] `README.md`

### Files to Modify
- [ ] All copied Go files - update import paths

---

## 7. Testing Checklist

### Unit Tests
- [ ] Processor message parsing
- [ ] Worker lifecycle

### Integration Tests
- [ ] Worker connects to RabbitMQ
- [ ] Worker consumes and acknowledges messages
- [ ] Worker handles errors and requeues

### Manual Tests
```bash
# Build and run locally
make build
./bin/email-worker

# Send test message via RabbitMQ Management UI
# Verify worker processes message

# Test Docker image
make docker-email
docker run -e RABBITMQ_HOST=host.docker.internal email-worker:latest
```

---

## 8. Dependencies on Other Components

| Component | Dependency | Notes |
|-----------|------------|-------|
| RabbitMQ | Required | Must be deployed first |
| api-service | Upstream | Publishes task messages |

---

## 9. Success Criteria

- [ ] All three workers build successfully
- [ ] Workers start and connect to RabbitMQ
- [ ] Health checks return 200
- [ ] Messages consumed from respective queues
- [ ] Processing completes without errors
- [ ] Docker images under 50MB each
- [ ] Startup time under 5 seconds

---

*Document Version: 1.0*  
*Created: January 2026*  
*Estimated Effort: 5-7 days*
