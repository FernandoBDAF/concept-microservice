# Microservices Task Processing Ecosystem: Profile → Queue → Worker Architecture

## Introduction

This document describes a comprehensive microservices task processing ecosystem built on Kubernetes with RabbitMQ as the message broker. The architecture consists of three core services working together: **profile-service** (orchestrator), **queue-service** (message broker), and **worker-service** (multi-worker processors). This design enables scalable, reliable task processing with independent deployment and scaling characteristics for different types of workloads.

The ecosystem supports three distinct task processing patterns:

- **Profile Processing**: User profile updates, data synchronization, and account management
- **Email Processing**: Notification delivery, welcome emails, and communication workflows
- **Image Processing**: Image resizing, format conversion, and media optimization

This architecture follows microservices best practices, clean architecture principles, and RabbitMQ messaging patterns to provide a robust foundation for enterprise-scale task processing.

## Architecture Overview

### High-Level Flow

```
Client Applications → Profile Service → Queue Service → RabbitMQ → Multi-Workers
     ↓ HTTP API          ↓ HTTP API        ↓ AMQP           ↓ AMQP
   Task Request      Queue Message    RabbitMQ Publish   Worker Consume
                                           ↓
                         ┌─────────────────┼─────────────────┐
                         ↓                 ↓                 ↓
                 profile.task         email.send      image.process
                         ↓                 ↓                 ↓
              Profile Processing   Email Processing   Image Processing
                         ↓                 ↓                 ↓
                Profile Worker      Email Worker       Image Worker
```

### Service Responsibilities

**Profile Service** (Entry Point & Orchestrator):

- Provides HTTP API for task submission
- Validates and routes tasks to appropriate workers
- Determines routing keys based on task types
- Maintains task records and status tracking
- Implements authentication and authorization

**Queue Service** (Message Broker & Publisher):

- Receives HTTP requests from profile-service
- Publishes messages to RabbitMQ with proper routing keys
- Implements publisher confirms for reliable delivery
- Provides message persistence and durability
- Supports multi-worker routing strategies

**Worker Service** (Multi-Worker Processors):

- Consumes messages from RabbitMQ queues
- Processes tasks based on worker specialization
- Implements independent scaling per worker type
- Provides health checks and metrics per worker
- Handles error recovery and retry mechanisms

## Service Implementations

### Profile Service Architecture

The profile-service serves as the primary entry point and orchestrator for the entire task processing ecosystem. It provides a clean HTTP API that abstracts the complexity of message routing and worker coordination.

#### Core Components

```go
// Profile Service - Entry Point & Orchestrator
type ProfileService struct {
    queueClient   QueueServiceInterface  // HTTP client to queue-service
    taskRepo      TaskRepository         // Task persistence
    validator     TaskValidator          // Request validation
    authService   AuthService           // Authentication/authorization
    metrics       MetricsCollector      // Prometheus metrics
    logger        *zap.Logger           // Structured logging
}

// Task request handling with routing key determination
func (s *ProfileService) SubmitTask(ctx context.Context, profileID string, req *TaskRequest) (*Task, error) {
    // 1. Validate request and authenticate user
    if err := s.validator.ValidateTaskRequest(req); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    // 2. Determine routing key based on task type
    routingKey := s.determineRoutingKey(req.Type)

    // 3. Create queue-compatible message
    queueMsg := &QueueMessage{
        ID:         generateUUID(),
        Type:       req.Type,
        Payload:    req.Payload, // json.RawMessage
        Timestamp:  time.Now(),
        Metadata:   req.Metadata,
        RoutingKey: routingKey,
    }

    // 4. Publish to queue-service via HTTP
    if err := s.queueClient.PublishMessage(ctx, queueMsg); err != nil {
        s.metrics.IncrementTaskSubmissionErrors(req.Type)
        return nil, fmt.Errorf("failed to publish message: %w", err)
    }

    // 5. Create and persist task record
    task := &Task{
        ID:        queueMsg.ID,
        ProfileID: profileID,
        Type:      req.Type,
        Status:    TaskStatusPending,
        CreatedAt: time.Now(),
    }

    if err := s.taskRepo.Create(ctx, task); err != nil {
        s.logger.Warn("Failed to persist task record", zap.Error(err))
        // Don't fail the request - message is already queued
    }

    s.metrics.IncrementTaskSubmissions(req.Type)
    s.logger.Info("Task submitted successfully",
        zap.String("task_id", task.ID),
        zap.String("type", req.Type),
        zap.String("routing_key", routingKey))

    return task, nil
}
```

#### Routing Key Determination

The profile-service implements intelligent routing key determination to ensure tasks reach the appropriate specialized workers:

```go
// Routing key mapping for multi-worker architecture
var RoutingKeyMap = map[string]string{
    "profile_update":     "profile.task",    // → Profile Worker
    "email_notification": "email.send",      // → Email Worker
    "image_processing":   "image.process",   // → Image Worker
    "user_deletion":      "profile.task",    // → Profile Worker
    "welcome_email":      "email.send",      // → Email Worker
    "avatar_resize":      "image.process",   // → Image Worker
}

func (s *ProfileService) determineRoutingKey(messageType string) string {
    if routingKey, exists := RoutingKeyMap[messageType]; exists {
        s.logger.Debug("Routing key determined",
            zap.String("message_type", messageType),
            zap.String("routing_key", routingKey))
        return routingKey
    }

    s.logger.Warn("Unknown message type, using default routing",
        zap.String("message_type", messageType))
    return "profile.task" // Default fallback to profile worker
}
```

#### API Endpoints

```go
// Task submission endpoint with comprehensive validation
func (h *TaskHandler) SubmitTask(c *gin.Context) {
    profileID := c.Param("profileId")

    var req TaskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "Invalid request format", "details": err.Error()})
        return
    }

    task, err := h.profileService.SubmitTask(c.Request.Context(), profileID, &req)
    if err != nil {
        h.logger.Error("Task submission failed", zap.Error(err))
        c.JSON(500, gin.H{"error": "Task submission failed", "details": err.Error()})
        return
    }

    c.JSON(201, gin.H{
        "task_id": task.ID,
        "status":  task.Status,
        "type":    task.Type,
        "message": "Task submitted successfully",
    })
}
```

### Queue Service Architecture

The queue-service acts as the message broker interface, receiving HTTP requests from the profile-service and publishing messages to RabbitMQ with proper routing and reliability guarantees.

#### Core Components

```go
// Queue Service - Message Broker & RabbitMQ Interface
type QueueService struct {
    rabbitmqPublisher RabbitMQPublisher  // Direct RabbitMQ integration
    httpServer        HTTPServer         // API for profile-service
    config           *Config            // Service configuration
    metrics          MetricsCollector   // Prometheus metrics
    logger           *zap.Logger        // Structured logging
}

// Enhanced message publishing with routing keys and publisher confirms
func (s *QueueService) PublishMessage(ctx context.Context, msg *Message) error {
    // 1. Validate message format
    if err := s.validateMessage(msg); err != nil {
        return fmt.Errorf("message validation failed: %w", err)
    }

    // 2. Determine exchange and routing configuration
    routingConfig := s.getRoutingConfig(msg.RoutingKey)

    // 3. Publish to RabbitMQ with publisher confirms
    publishCtx, cancel := context.WithTimeout(ctx, s.config.PublishTimeout)
    defer cancel()

    if err := s.rabbitmqPublisher.PublishWithConfirm(publishCtx, &PublishRequest{
        Exchange:    routingConfig.Exchange,
        RoutingKey:  msg.RoutingKey,
        Message:     msg,
        Persistent:  true,
        Mandatory:   false,
    }); err != nil {
        s.metrics.IncrementPublishErrors(msg.RoutingKey)
        return fmt.Errorf("failed to publish message: %w", err)
    }

    s.metrics.IncrementPublishedMessages(msg.RoutingKey)
    s.logger.Info("Message published successfully",
        zap.String("message_id", msg.ID),
        zap.String("type", msg.Type),
        zap.String("routing_key", msg.RoutingKey),
        zap.String("exchange", routingConfig.Exchange))

    return nil
}
```

#### RabbitMQ Integration with Best Practices

The queue-service implements RabbitMQ best practices including single exchange strategy, publisher confirms, and proper connection management:

```go
// RabbitMQ Publisher with best practices implementation
type RabbitMQPublisher struct {
    conn        *amqp.Connection
    channel     *amqp.Channel
    confirms    chan amqp.Confirmation
    config      *RabbitMQConfig
    logger      *zap.Logger
    mu          sync.RWMutex
}

func (p *RabbitMQPublisher) Initialize() error {
    // 1. Establish long-lived connection
    conn, err := amqp.Dial(p.config.URL)
    if err != nil {
        return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
    }
    p.conn = conn

    // 2. Create single channel for publishing
    ch, err := conn.Channel()
    if err != nil {
        return fmt.Errorf("failed to create channel: %w", err)
    }
    p.channel = ch

    // 3. Enable publisher confirms
    if err := ch.Confirm(false); err != nil {
        return fmt.Errorf("failed to enable publisher confirms: %w", err)
    }
    p.confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 1))

    // 4. Declare exchanges and queues
    if err := p.setupTopology(); err != nil {
        return fmt.Errorf("failed to setup topology: %w", err)
    }

    return nil
}

func (p *RabbitMQPublisher) setupTopology() error {
    // Single exchange strategy with routing keys
    exchanges := []ExchangeConfig{
        {Name: "tasks-exchange", Type: "direct", Durable: true},
    }

    for _, exchange := range exchanges {
        if err := p.channel.ExchangeDeclare(
            exchange.Name,
            exchange.Type,
            exchange.Durable,
            false, // autoDelete
            false, // internal
            false, // noWait
            nil,   // arguments
        ); err != nil {
            return fmt.Errorf("failed to declare exchange %s: %w", exchange.Name, err)
        }
    }

    // Declare queues and bindings for each worker type
    queueConfigs := []QueueConfig{
        {Name: "profile-processing", RoutingKey: "profile.task", Exchange: "tasks-exchange"},
        {Name: "email-processing", RoutingKey: "email.send", Exchange: "tasks-exchange"},
        {Name: "image-processing", RoutingKey: "image.process", Exchange: "tasks-exchange"},
    }

    for _, qConfig := range queueConfigs {
        // Declare durable queue
        _, err := p.channel.QueueDeclare(
            qConfig.Name,
            true,  // durable
            false, // autoDelete
            false, // exclusive
            false, // noWait
            amqp.Table{
                "x-message-ttl":             int32(24 * time.Hour / time.Millisecond),
                "x-dead-letter-exchange":    qConfig.Name + "-dlx",
                "x-dead-letter-routing-key": "failed",
            },
        )
        if err != nil {
            return fmt.Errorf("failed to declare queue %s: %w", qConfig.Name, err)
        }

        // Bind queue to exchange
        if err := p.channel.QueueBind(
            qConfig.Name,
            qConfig.RoutingKey,
            qConfig.Exchange,
            false, // noWait
            nil,   // arguments
        ); err != nil {
            return fmt.Errorf("failed to bind queue %s: %w", qConfig.Name, err)
        }
    }

    return nil
}
```

#### Message Format Standardization

The queue-service ensures message format compatibility across all services:

```go
// Standardized message format across all services
type Message struct {
    ID         string            `json:"id"`
    Type       string            `json:"type"`           // String, not enum
    Payload    json.RawMessage   `json:"payload"`        // RawMessage for flexibility
    Timestamp  time.Time         `json:"timestamp"`      // Proper time format
    Metadata   map[string]string `json:"metadata"`       // Consistent field name
    RoutingKey string            `json:"routing_key"`    // Required for routing
}

// Message validation ensuring compatibility
func (s *QueueService) validateMessage(msg *Message) error {
    if msg.ID == "" {
        return errors.New("message ID is required")
    }
    if msg.Type == "" {
        return errors.New("message type is required")
    }
    if msg.RoutingKey == "" {
        return errors.New("routing key is required")
    }
    if len(msg.Payload) == 0 {
        return errors.New("message payload is required")
    }

    // Validate routing key format
    if !isValidRoutingKey(msg.RoutingKey) {
        return fmt.Errorf("invalid routing key format: %s", msg.RoutingKey)
    }

    return nil
}
```

### Worker Service Multi-Worker Architecture

The worker-service implements a multi-worker architecture with shared foundation and specialized processing capabilities for different task types.

#### Shared Foundation Pattern

```go
// services/workers/common/base/worker.go
type BaseWorker struct {
    config      *WorkerConfig
    processor   processors.MessageProcessor
    consumer    *commonQueue.Consumer
    server      *HTTPServer
    metrics     MetricsCollector
    logger      *zap.Logger
    shutdown    chan os.Signal
    wg          sync.WaitGroup
}

func NewBaseWorker(config *WorkerConfig, processor processors.MessageProcessor) (*BaseWorker, error) {
    worker := &BaseWorker{
        config:    config,
        processor: processor,
        shutdown:  make(chan os.Signal, 1),
        logger:    zap.NewProduction(),
        metrics:   NewMetricsCollector(config.WorkerType),
    }

    // Initialize RabbitMQ consumer
    consumer, err := commonQueue.NewConsumer(&commonQueue.Config{
        URL:           config.RabbitMQURL,
        QueueName:     config.QueueName,
        ExchangeName:  config.ExchangeName,
        RoutingKey:    config.RoutingKey,
        PrefetchCount: config.PrefetchCount,
        AutoAck:       false, // Manual acknowledgment for reliability
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create consumer: %w", err)
    }
    worker.consumer = consumer

    // Initialize HTTP server for health checks
    server := NewHTTPServer(config.HTTPPort, worker.logger)
    server.RegisterHealthHandlers(worker.healthCheck, worker.readinessCheck)
    worker.server = server

    return worker, nil
}

func (w *BaseWorker) Run() error {
    signal.Notify(w.shutdown, os.Interrupt, syscall.SIGTERM)

    // Start HTTP server for health checks
    w.wg.Add(1)
    go func() {
        defer w.wg.Done()
        if err := w.server.Start(); err != nil && err != http.ErrServerClosed {
            w.logger.Error("HTTP server error", zap.Error(err))
        }
    }()

    // Start message consumption
    w.wg.Add(1)
    go func() {
        defer w.wg.Done()
        w.consumeMessages()
    }()

    // Wait for shutdown signal
    <-w.shutdown
    w.logger.Info("Shutdown signal received, starting graceful shutdown")

    // Graceful shutdown
    return w.gracefulShutdown()
}
```

#### Message Processing Pipeline

```go
func (w *BaseWorker) consumeMessages() {
    messages, err := w.consumer.Consume()
    if err != nil {
        w.logger.Error("Failed to start consuming", zap.Error(err))
        return
    }

    for msg := range messages {
        w.processMessage(msg)
    }
}

func (w *BaseWorker) processMessage(delivery amqp.Delivery) {
    startTime := time.Now()

    // Parse message
    var message commonQueue.Message
    if err := json.Unmarshal(delivery.Body, &message); err != nil {
        w.logger.Error("Failed to unmarshal message", zap.Error(err))
        w.metrics.IncrementProcessingErrors("unmarshal_error")
        delivery.Nack(false, false) // Send to DLQ
        return
    }

    // Validate message type matches worker capability
    if !w.processor.CanProcess(message.Type) {
        w.logger.Warn("Message type not supported by this worker",
            zap.String("message_type", message.Type),
            zap.String("worker_type", w.config.WorkerType))
        w.metrics.IncrementProcessingErrors("unsupported_type")
        delivery.Nack(false, false) // Send to DLQ
        return
    }

    // Process message with timeout
    ctx, cancel := context.WithTimeout(context.Background(), w.config.ProcessingTimeout)
    defer cancel()

    if err := w.processor.Process(ctx, &message); err != nil {
        w.logger.Error("Message processing failed",
            zap.String("message_id", message.ID),
            zap.String("message_type", message.Type),
            zap.Error(err))

        w.metrics.IncrementProcessingErrors("processing_failed")

        // Implement retry logic based on error type
        if w.shouldRetry(err, delivery.Headers) {
            delivery.Nack(false, true) // Requeue for retry
        } else {
            delivery.Nack(false, false) // Send to DLQ
        }
        return
    }

    // Acknowledge successful processing
    delivery.Ack(false)

    processingDuration := time.Since(startTime)
    w.metrics.RecordProcessingDuration(message.Type, processingDuration)
    w.metrics.IncrementProcessedMessages(message.Type)

    w.logger.Info("Message processed successfully",
        zap.String("message_id", message.ID),
        zap.String("message_type", message.Type),
        zap.Duration("processing_time", processingDuration))
}
```

#### Worker-Specific Implementations

**Email Worker Implementation:**

```go
// services/workers/email-worker/internal/processors/email_processor.go
type EmailProcessor struct {
    emailClient EmailServiceInterface
    templates   TemplateManager
    config      *EmailConfig
    logger      *zap.Logger
}

func (p *EmailProcessor) Process(ctx context.Context, msg *commonQueue.Message) error {
    var emailPayload EmailPayload
    if err := json.Unmarshal(msg.Payload, &emailPayload); err != nil {
        return fmt.Errorf("failed to unmarshal email payload: %w", err)
    }

    // Validate email payload
    if err := p.validateEmailPayload(&emailPayload); err != nil {
        return fmt.Errorf("email payload validation failed: %w", err)
    }

    // Render email template
    emailContent, err := p.templates.Render(emailPayload.Template, emailPayload.Variables)
    if err != nil {
        return fmt.Errorf("failed to render email template: %w", err)
    }

    // Send email (mock implementation for now)
    emailRequest := &EmailRequest{
        To:      emailPayload.To,
        Subject: emailPayload.Subject,
        Content: emailContent,
        From:    p.config.DefaultSender,
    }

    if err := p.emailClient.SendEmail(ctx, emailRequest); err != nil {
        return fmt.Errorf("failed to send email: %w", err)
    }

    p.logger.Info("Email sent successfully",
        zap.String("message_id", msg.ID),
        zap.String("recipient", emailPayload.To),
        zap.String("template", emailPayload.Template))

    return nil
}

func (p *EmailProcessor) CanProcess(messageType string) bool {
    supportedTypes := []string{
        "email_notification",
        "welcome_email",
        "password_reset",
        "account_verification",
    }

    for _, supportedType := range supportedTypes {
        if messageType == supportedType {
            return true
        }
    }
    return false
}
```

**Image Worker Implementation:**

```go
// services/workers/image-worker/internal/processors/image_processor.go
type ImageProcessor struct {
    pythonClient PythonServiceInterface
    storage      StorageInterface
    config       *ImageConfig
    logger       *zap.Logger
}

func (p *ImageProcessor) Process(ctx context.Context, msg *commonQueue.Message) error {
    var imagePayload ImagePayload
    if err := json.Unmarshal(msg.Payload, &imagePayload); err != nil {
        return fmt.Errorf("failed to unmarshal image payload: %w", err)
    }

    // Validate image payload
    if err := p.validateImagePayload(&imagePayload); err != nil {
        return fmt.Errorf("image payload validation failed: %w", err)
    }

    // Download source image
    sourceImage, err := p.storage.Download(ctx, imagePayload.ImageURL)
    if err != nil {
        return fmt.Errorf("failed to download source image: %w", err)
    }
    defer sourceImage.Close()

    // Process image using Python container (mock implementation)
    processedImage, err := p.pythonClient.ProcessImage(ctx, &ImageProcessingRequest{
        Image:      sourceImage,
        Operations: imagePayload.Operations,
        Dimensions: imagePayload.Dimensions,
        Format:     imagePayload.OutputFormat,
    })
    if err != nil {
        return fmt.Errorf("failed to process image: %w", err)
    }
    defer processedImage.Close()

    // Upload processed image
    outputURL, err := p.storage.Upload(ctx, processedImage, imagePayload.OutputPath)
    if err != nil {
        return fmt.Errorf("failed to upload processed image: %w", err)
    }

    p.logger.Info("Image processed successfully",
        zap.String("message_id", msg.ID),
        zap.String("source_url", imagePayload.ImageURL),
        zap.String("output_url", outputURL),
        zap.Strings("operations", imagePayload.Operations))

    return nil
}

func (p *ImageProcessor) CanProcess(messageType string) bool {
    supportedTypes := []string{
        "image_processing",
        "avatar_resize",
        "thumbnail_generation",
        "image_optimization",
    }

    for _, supportedType := range supportedTypes {
        if messageType == supportedType {
            return true
        }
    }
    return false
}
```

## Kubernetes Deployment Architecture

### RabbitMQ StatefulSet Deployment

The RabbitMQ deployment uses a StatefulSet for potential clustering and persistent storage:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: rabbitmq
  labels:
    app: rabbitmq
spec:
  clusterIP: None # Headless service for StatefulSet
  ports:
    - name: amqp
      port: 5672
      targetPort: 5672
    - name: management
      port: 15672
      targetPort: 15672
  selector:
    app: rabbitmq

---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: rabbitmq
spec:
  serviceName: rabbitmq
  replicas: 1 # Can be scaled for clustering
  selector:
    matchLabels:
      app: rabbitmq
  template:
    metadata:
      labels:
        app: rabbitmq
    spec:
      terminationGracePeriodSeconds: 30
      containers:
        - name: rabbitmq
          image: rabbitmq:3.11-management
          env:
            - name: RABBITMQ_DEFAULT_USER
              value: "admin"
            - name: RABBITMQ_DEFAULT_PASS
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: password
            - name: RABBITMQ_ERLANG_COOKIE
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: erlang-cookie
          ports:
            - name: amqp
              containerPort: 5672
            - name: management
              containerPort: 15672
          volumeMounts:
            - name: rabbitmq-data
              mountPath: /var/lib/rabbitmq
          livenessProbe:
            exec:
              command:
                - rabbitmq-diagnostics
                - ping
            initialDelaySeconds: 60
            periodSeconds: 30
            timeoutSeconds: 10
          readinessProbe:
            exec:
              command:
                - rabbitmq-diagnostics
                - check_port_connectivity
            initialDelaySeconds: 20
            periodSeconds: 10
            timeoutSeconds: 5
  volumeClaimTemplates:
    - metadata:
        name: rabbitmq-data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 10Gi
```

### Profile Service Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-service
  labels:
    app: profile-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: profile-service
  template:
    metadata:
      labels:
        app: profile-service
    spec:
      containers:
        - name: profile-service
          image: profile-service:latest
          ports:
            - name: http
              containerPort: 8080
          env:
            - name: QUEUE_SERVICE_URL
              value: "http://queue-service:8080"
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: profile-service-secret
                  key: database-url
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: profile-service-secret
                  key: jwt-secret
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10

---
apiVersion: v1
kind: Service
metadata:
  name: profile-service
spec:
  selector:
    app: profile-service
  ports:
    - name: http
      port: 8080
      targetPort: 8080
  type: ClusterIP
```

### Queue Service Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: queue-service
  labels:
    app: queue-service
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
            - name: http
              containerPort: 8080
          env:
            - name: RABBITMQ_URL
              value: "amqp://admin:$(RABBITMQ_PASSWORD)@rabbitmq:5672/"
            - name: RABBITMQ_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: password
            - name: PUBLISH_TIMEOUT
              value: "30s"
            - name: CONFIRM_TIMEOUT
              value: "10s"
          resources:
            requests:
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "200m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10

---
apiVersion: v1
kind: Service
metadata:
  name: queue-service
spec:
  selector:
    app: queue-service
  ports:
    - name: http
      port: 8080
      targetPort: 8080
  type: ClusterIP
```

### Multi-Worker Deployments

**Email Worker Deployment:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: email-worker
  labels:
    app: email-worker
    worker-type: email
spec:
  replicas: 5 # Higher replica count for burst processing
  selector:
    matchLabels:
      app: email-worker
  template:
    metadata:
      labels:
        app: email-worker
        worker-type: email
    spec:
      containers:
        - name: email-worker
          image: email-worker:latest
          ports:
            - name: http
              containerPort: 8080
          env:
            - name: RABBITMQ_URL
              value: "amqp://admin:$(RABBITMQ_PASSWORD)@rabbitmq:5672/"
            - name: RABBITMQ_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: password
            - name: QUEUE_NAME
              value: "email-processing"
            - name: ROUTING_KEY
              value: "email.send"
            - name: PREFETCH_COUNT
              value: "5" # Burst processing capability
            - name: PROCESSING_TIMEOUT
              value: "30s"
          resources:
            requests:
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "200m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: email-worker-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: email-worker
  minReplicas: 3
  maxReplicas: 20
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
```

**Image Worker Deployment:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: image-worker
  labels:
    app: image-worker
    worker-type: image
spec:
  replicas: 2 # Lower replica count, higher resources
  selector:
    matchLabels:
      app: image-worker
  template:
    metadata:
      labels:
        app: image-worker
        worker-type: image
    spec:
      containers:
        - name: image-worker
          image: image-worker:latest
          ports:
            - name: http
              containerPort: 8080
          env:
            - name: RABBITMQ_URL
              value: "amqp://admin:$(RABBITMQ_PASSWORD)@rabbitmq:5672/"
            - name: RABBITMQ_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: password
            - name: QUEUE_NAME
              value: "image-processing"
            - name: ROUTING_KEY
              value: "image.process"
            - name: PREFETCH_COUNT
              value: "1" # Resource-intensive processing
            - name: PROCESSING_TIMEOUT
              value: "300s" # 5 minutes for complex image operations
          resources:
            requests:
              memory: "512Mi"
              cpu: "500m"
            limits:
              memory: "2Gi"
              cpu: "1000m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 60
            periodSeconds: 60
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 20

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: image-worker-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: image-worker
  minReplicas: 1
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 60
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 70
```

## Message Flow and Processing Patterns

### Task Submission Flow

1. **Client Request**: Client applications submit tasks via HTTP API to profile-service
2. **Authentication & Validation**: Profile-service authenticates request and validates task payload
3. **Routing Key Determination**: Profile-service determines appropriate routing key based on task type
4. **Message Creation**: Profile-service creates standardized queue message with routing key
5. **HTTP Publishing**: Profile-service sends message to queue-service via HTTP API
6. **RabbitMQ Publishing**: Queue-service publishes message to RabbitMQ with publisher confirms
7. **Message Routing**: RabbitMQ routes message to appropriate worker queue based on routing key
8. **Worker Processing**: Specialized worker consumes and processes message
9. **Acknowledgment**: Worker acknowledges successful processing or sends to DLQ on failure

### Message Format Standardization

All services use a consistent message format for seamless integration:

```go
type Message struct {
    ID         string            `json:"id"`         // Unique message identifier
    Type       string            `json:"type"`       // Task type (profile_update, email_notification, etc.)
    Payload    json.RawMessage   `json:"payload"`    // Task-specific data as raw JSON
    Timestamp  time.Time         `json:"timestamp"`  // Message creation timestamp
    Metadata   map[string]string `json:"metadata"`   // Additional context and headers
    RoutingKey string            `json:"routing_key"` // RabbitMQ routing key for worker selection
}
```

### Task Type Examples

**Profile Update Task:**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "profile_update",
  "payload": {
    "user_id": "12345",
    "action": "update",
    "changes": {
      "email": "new@example.com",
      "name": "John Doe"
    }
  },
  "timestamp": "2023-12-01T10:30:00Z",
  "metadata": {
    "correlation_id": "req-123",
    "user_agent": "ProfileApp/1.0"
  },
  "routing_key": "profile.task"
}
```

**Email Notification Task:**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440001",
  "type": "email_notification",
  "payload": {
    "to": "user@example.com",
    "template": "welcome",
    "variables": {
      "user_name": "John Doe",
      "activation_link": "https://app.com/activate/token123"
    }
  },
  "timestamp": "2023-12-01T10:30:00Z",
  "metadata": {
    "correlation_id": "req-124",
    "priority": "high"
  },
  "routing_key": "email.send"
}
```

**Image Processing Task:**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440002",
  "type": "image_processing",
  "payload": {
    "image_url": "https://storage.com/images/avatar.jpg",
    "operations": ["resize", "compress"],
    "dimensions": {
      "width": 200,
      "height": 200
    },
    "output_format": "webp",
    "output_path": "thumbnails/avatar-200x200.webp"
  },
  "timestamp": "2023-12-01T10:30:00Z",
  "metadata": {
    "correlation_id": "req-125",
    "user_id": "12345"
  },
  "routing_key": "image.process"
}
```

## Monitoring and Observability

### Prometheus Metrics

Each service exposes comprehensive metrics for monitoring and alerting:

**Profile Service Metrics:**

```go
// Task submission metrics
profile_service_tasks_submitted_total{type="profile_update"} 1250
profile_service_tasks_submitted_total{type="email_notification"} 890
profile_service_tasks_submitted_total{type="image_processing"} 340

// API performance metrics
profile_service_http_request_duration_seconds{method="POST",path="/profiles/:id/tasks"} 0.045
profile_service_http_requests_total{method="POST",path="/profiles/:id/tasks",status="200"} 2480

// Queue service communication metrics
profile_service_queue_client_requests_total{status="success"} 2480
profile_service_queue_client_requests_total{status="error"} 12
profile_service_queue_client_duration_seconds 0.028
```

**Queue Service Metrics:**

```go
// Message publishing metrics
queue_service_messages_published_total{routing_key="profile.task"} 1250
queue_service_messages_published_total{routing_key="email.send"} 890
queue_service_messages_published_total{routing_key="image.process"} 340

// Publisher confirm metrics
queue_service_publisher_confirms_total{status="ack"} 2478
queue_service_publisher_confirms_total{status="nack"} 2
queue_service_publisher_confirm_duration_seconds 0.012

// RabbitMQ connection metrics
queue_service_rabbitmq_connection_status{status="connected"} 1
queue_service_rabbitmq_channel_status{status="open"} 1
```

**Worker Service Metrics:**

```go
// Message processing metrics
worker_service_messages_processed_total{worker_type="email",status="success"} 885
worker_service_messages_processed_total{worker_type="email",status="error"} 5
worker_service_message_processing_duration_seconds{worker_type="email"} 2.3

worker_service_messages_processed_total{worker_type="image",status="success"} 335
worker_service_messages_processed_total{worker_type="image",status="error"} 5
worker_service_message_processing_duration_seconds{worker_type="image"} 45.7

// Queue consumption metrics
worker_service_queue_messages_consumed_total{worker_type="email"} 890
worker_service_queue_messages_requeued_total{worker_type="email"} 3
worker_service_queue_messages_rejected_total{worker_type="email"} 2
```

### Health Checks and Readiness Probes

Each service implements comprehensive health checks:

**Profile Service Health Checks:**

```go
func (h *HealthHandler) HealthCheck(c *gin.Context) {
    health := &HealthStatus{
        Status:    "healthy",
        Timestamp: time.Now(),
        Checks: map[string]CheckResult{
            "database":     h.checkDatabase(),
            "queue_client": h.checkQueueService(),
            "memory":       h.checkMemoryUsage(),
        },
    }

    if !health.IsHealthy() {
        c.JSON(503, health)
        return
    }

    c.JSON(200, health)
}

func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
    readiness := &ReadinessStatus{
        Status:    "ready",
        Timestamp: time.Now(),
        Checks: map[string]CheckResult{
            "queue_service": h.checkQueueServiceConnectivity(),
            "database":      h.checkDatabaseConnectivity(),
        },
    }

    if !readiness.IsReady() {
        c.JSON(503, readiness)
        return
    }

    c.JSON(200, readiness)
}
```

### Distributed Tracing

The ecosystem supports distributed tracing for end-to-end request tracking:

```go
// Profile service - start trace
func (s *ProfileService) SubmitTask(ctx context.Context, profileID string, req *TaskRequest) (*Task, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "profile_service.submit_task")
    defer span.Finish()

    span.SetTag("profile_id", profileID)
    span.SetTag("task_type", req.Type)

    // Inject trace context into message metadata
    carrier := opentracing.TextMapCarrier{}
    opentracing.GlobalTracer().Inject(span.Context(), opentracing.TextMap, carrier)

    for key, value := range carrier {
        req.Metadata[key] = value
    }

    // Continue processing...
}

// Worker service - continue trace
func (w *BaseWorker) processMessage(delivery amqp.Delivery) {
    var message commonQueue.Message
    json.Unmarshal(delivery.Body, &message)

    // Extract trace context from message metadata
    carrier := opentracing.TextMapCarrier(message.Metadata)
    spanContext, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, carrier)

    var span opentracing.Span
    if err != nil {
        span = opentracing.StartSpan("worker.process_message")
    } else {
        span = opentracing.StartSpan("worker.process_message", opentracing.ChildOf(spanContext))
    }
    defer span.Finish()

    span.SetTag("worker_type", w.config.WorkerType)
    span.SetTag("message_id", message.ID)
    span.SetTag("message_type", message.Type)

    ctx := opentracing.ContextWithSpan(context.Background(), span)

    // Process message with trace context...
}
```

## Error Handling and Reliability

### Dead Letter Queue Configuration

The ecosystem implements comprehensive error handling with Dead Letter Queues:

```go
// Queue declaration with DLQ configuration
_, err := channel.QueueDeclare(
    queueName,
    true,  // durable
    false, // autoDelete
    false, // exclusive
    false, // noWait
    amqp.Table{
        "x-message-ttl":             int32(24 * time.Hour / time.Millisecond),
        "x-dead-letter-exchange":    queueName + "-dlx",
        "x-dead-letter-routing-key": "failed",
        "x-max-retries":             3,
    },
)

// Declare Dead Letter Exchange and Queue
channel.ExchangeDeclare(queueName+"-dlx", "direct", true, false, false, false, nil)
channel.QueueDeclare(queueName+"-dlq", true, false, false, false, nil)
channel.QueueBind(queueName+"-dlq", "failed", queueName+"-dlx", false, nil)
```

### Retry Logic and Circuit Breaker

Workers implement intelligent retry logic and circuit breaker patterns:

```go
func (w *BaseWorker) shouldRetry(err error, headers amqp.Table) bool {
    // Check retry count
    retryCount := int32(0)
    if count, exists := headers["x-retry-count"]; exists {
        if c, ok := count.(int32); ok {
            retryCount = c
        }
    }

    if retryCount >= w.config.MaxRetries {
        w.logger.Warn("Max retries exceeded", zap.Int32("retry_count", retryCount))
        return false
    }

    // Check error type for retry eligibility
    switch {
    case isTransientError(err):
        return true
    case isNetworkError(err):
        return true
    case isTimeoutError(err):
        return true
    default:
        return false
    }
}

func (w *BaseWorker) processMessageWithRetry(delivery amqp.Delivery) {
    retryCount := w.getRetryCount(delivery.Headers)

    if err := w.processor.Process(ctx, &message); err != nil {
        if w.shouldRetry(err, delivery.Headers) {
            // Add retry headers and requeue
            newHeaders := make(amqp.Table)
            for k, v := range delivery.Headers {
                newHeaders[k] = v
            }
            newHeaders["x-retry-count"] = retryCount + 1
            newHeaders["x-retry-timestamp"] = time.Now().Unix()

            // Exponential backoff delay
            delay := time.Duration(math.Pow(2, float64(retryCount))) * time.Second
            time.Sleep(delay)

            delivery.Nack(false, true) // Requeue
            return
        }

        delivery.Nack(false, false) // Send to DLQ
        return
    }

    delivery.Ack(false)
}
```

### Graceful Shutdown

All services implement graceful shutdown to prevent message loss:

```go
func (w *BaseWorker) gracefulShutdown() error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    w.logger.Info("Starting graceful shutdown")

    // Stop accepting new messages
    if err := w.consumer.Cancel(); err != nil {
        w.logger.Error("Failed to cancel consumer", zap.Error(err))
    }

    // Wait for current messages to complete processing
    done := make(chan struct{})
    go func() {
        w.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        w.logger.Info("All goroutines finished")
    case <-ctx.Done():
        w.logger.Warn("Graceful shutdown timeout, forcing exit")
    }

    // Close connections
    if err := w.consumer.Close(); err != nil {
        w.logger.Error("Failed to close consumer", zap.Error(err))
    }

    // Shutdown HTTP server
    if err := w.server.Shutdown(ctx); err != nil {
        w.logger.Error("Failed to shutdown HTTP server", zap.Error(err))
    }

    w.logger.Info("Graceful shutdown completed")
    return nil
}
```

## Performance Characteristics and Scaling

### Throughput Targets

The ecosystem is designed to handle enterprise-scale workloads:

**Profile Service:**

- API Response Time: < 50ms (95th percentile)
- Task Submission Rate: 1000+ tasks/second
- Concurrent Connections: 10,000+

**Queue Service:**

- Message Acceptance: < 100ms (95th percentile)
- Publisher Confirm: < 500ms (95th percentile)
- Message Throughput: 5000+ messages/second

**Worker Services:**

- Email Worker: 100+ messages/second with burst capability up to 500/second
- Image Worker: 10-20 messages/second for complex operations
- Profile Worker: 50+ messages/second for database operations

### Horizontal Scaling Strategies

**Profile Service Scaling:**

```yaml
# Stateless service - can scale horizontally without constraints
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: profile-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: profile-service
  minReplicas: 3
  maxReplicas: 50
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Pods
      pods:
        metric:
          name: http_requests_per_second
        target:
          type: AverageValue
          averageValue: "100"
```

**Queue Service Scaling:**

```yaml
# Can scale horizontally with shared RabbitMQ connection pool
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: queue-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: queue-service
  minReplicas: 2
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 60
    - type: Pods
      pods:
        metric:
          name: messages_published_per_second
        target:
          type: AverageValue
          averageValue: "200"
```

**Worker Scaling with KEDA:**

```yaml
# KEDA ScaledObject for queue-based scaling
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: email-worker-scaledobject
spec:
  scaleTargetRef:
    name: email-worker
  minReplicaCount: 3
  maxReplicaCount: 50
  triggers:
    - type: rabbitmq
      metadata:
        host: "amqp://admin:password@rabbitmq:5672/"
        queueName: "email-processing"
        queueLength: "10" # Scale up when queue has 10+ messages
        excludeUnacknowledged: "false"
```

### Resource Optimization

**Memory and CPU Allocation:**

```yaml
# Email Worker - Optimized for throughput
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "256Mi"
    cpu: "200m"

# Image Worker - Optimized for processing power
resources:
  requests:
    memory: "512Mi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "1000m"

# Profile Worker - Balanced for database operations
resources:
  requests:
    memory: "256Mi"
    cpu: "250m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

## Security Considerations

### Authentication and Authorization

**Profile Service Security:**

```go
func (h *AuthHandler) ValidateJWT(c *gin.Context) {
    token := c.GetHeader("Authorization")
    if token == "" {
        c.JSON(401, gin.H{"error": "Authorization header required"})
        c.Abort()
        return
    }

    claims, err := h.jwtService.ValidateToken(token)
    if err != nil {
        c.JSON(401, gin.H{"error": "Invalid token"})
        c.Abort()
        return
    }

    c.Set("user_id", claims.UserID)
    c.Set("user_roles", claims.Roles)
    c.Next()
}

func (h *TaskHandler) SubmitTask(c *gin.Context) {
    userID := c.GetString("user_id")
    profileID := c.Param("profileId")

    // Ensure user can only submit tasks for their own profile
    if userID != profileID {
        c.JSON(403, gin.H{"error": "Forbidden: Cannot submit tasks for other users"})
        return
    }

    // Continue with task submission...
}
```

### RabbitMQ Security

**TLS Configuration:**

```yaml
# RabbitMQ with TLS
env:
  - name: RABBITMQ_SSL_CERTFILE
    value: "/etc/ssl/certs/server-cert.pem"
  - name: RABBITMQ_SSL_KEYFILE
    value: "/etc/ssl/private/server-key.pem"
  - name: RABBITMQ_SSL_CACERTFILE
    value: "/etc/ssl/certs/ca-cert.pem"
  - name: RABBITMQ_SSL_VERIFY
    value: "verify_peer"
  - name: RABBITMQ_SSL_FAIL_IF_NO_PEER_CERT
    value: "true"
```

**User Permissions:**

```go
// Service-specific RabbitMQ users with limited permissions
const (
    QueueServiceUser = "queue-service-user"  // Can publish to exchanges
    WorkerServiceUser = "worker-service-user" // Can consume from queues
)

// RabbitMQ permission configuration
// queue-service-user: configure=^$ write=tasks-exchange read=^$
// worker-service-user: configure=^$ write=^$ read=.*-processing
```

### Network Security

**Network Policies:**

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: microservices-network-policy
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    # Allow ingress to profile-service from external
    - from: []
      ports:
        - protocol: TCP
          port: 8080
      podSelector:
        matchLabels:
          app: profile-service
    # Allow internal communication
    - from:
        - podSelector:
            matchLabels:
              app: profile-service
      to:
        - podSelector:
            matchLabels:
              app: queue-service
  egress:
    # Allow egress to RabbitMQ
    - to:
        - podSelector:
            matchLabels:
              app: rabbitmq
      ports:
        - protocol: TCP
          port: 5672
```

## Testing and Validation

### Integration Testing

**End-to-End Test Suite:**

```go
func TestCompleteTaskFlow(t *testing.T) {
    // Setup test environment
    testEnv := setupTestEnvironment(t)
    defer testEnv.Cleanup()

    // Test profile task flow
    t.Run("ProfileTaskFlow", func(t *testing.T) {
        taskReq := &TaskRequest{
            Type: "profile_update",
            Payload: json.RawMessage(`{"user_id": "123", "action": "update"}`),
            Metadata: map[string]string{"test": "true"},
        }

        // Submit task via profile-service
        resp, err := testEnv.ProfileClient.SubmitTask("123", taskReq)
        assert.NoError(t, err)
        assert.Equal(t, "pending", resp.Status)

        // Wait for processing
        time.Sleep(2 * time.Second)

        // Verify task was processed by profile worker
        processedTasks := testEnv.ProfileWorker.GetProcessedTasks()
        assert.Len(t, processedTasks, 1)
        assert.Equal(t, resp.TaskID, processedTasks[0].ID)
    })

    // Test email task flow
    t.Run("EmailTaskFlow", func(t *testing.T) {
        taskReq := &TaskRequest{
            Type: "email_notification",
            Payload: json.RawMessage(`{"to": "test@example.com", "template": "welcome"}`),
            Metadata: map[string]string{"test": "true"},
        }

        resp, err := testEnv.ProfileClient.SubmitTask("123", taskReq)
        assert.NoError(t, err)

        time.Sleep(2 * time.Second)

        processedEmails := testEnv.EmailWorker.GetProcessedEmails()
        assert.Len(t, processedEmails, 1)
        assert.Equal(t, "test@example.com", processedEmails[0].To)
    })
}
```

### Load Testing

**Performance Test Suite:**

```go
func TestSystemPerformance(t *testing.T) {
    testEnv := setupTestEnvironment(t)
    defer testEnv.Cleanup()

    // Test concurrent task submissions
    t.Run("ConcurrentTaskSubmissions", func(t *testing.T) {
        concurrency := 100
        tasksPerWorker := 50
        totalTasks := concurrency * tasksPerWorker

        var wg sync.WaitGroup
        results := make(chan error, totalTasks)

        startTime := time.Now()

        for i := 0; i < concurrency; i++ {
            wg.Add(1)
            go func(workerID int) {
                defer wg.Done()

                for j := 0; j < tasksPerWorker; j++ {
                    taskReq := &TaskRequest{
                        Type: "profile_update",
                        Payload: json.RawMessage(fmt.Sprintf(`{"worker_id": %d, "task_id": %d}`, workerID, j)),
                    }

                    _, err := testEnv.ProfileClient.SubmitTask("123", taskReq)
                    results <- err
                }
            }(i)
        }

        wg.Wait()
        close(results)

        duration := time.Since(startTime)

        // Check for errors
        errorCount := 0
        for err := range results {
            if err != nil {
                errorCount++
                t.Logf("Task submission error: %v", err)
            }
        }

        // Performance assertions
        throughput := float64(totalTasks) / duration.Seconds()
        t.Logf("Submitted %d tasks in %v (%.2f tasks/sec)", totalTasks, duration, throughput)

        assert.Less(t, errorCount, totalTasks/100, "Error rate should be less than 1%")
        assert.Greater(t, throughput, 500.0, "Throughput should be greater than 500 tasks/sec")
    })
}
```

## Deployment and Operations

### CI/CD Pipeline

**GitHub Actions Workflow:**

```yaml
name: Microservices CI/CD

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: 1.20

      - name: Run Tests
        run: |
          cd services/profile-service && go test ./...
          cd services/queue-service && go test ./...
          cd services/workers && go test ./...

      - name: Integration Tests
        run: |
          docker-compose -f docker-compose.test.yml up -d
          sleep 30
          go test ./tests/integration/...
          docker-compose -f docker-compose.test.yml down

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Build Images
        run: |
          docker build -t profile-service:${{ github.sha }} services/profile-service/
          docker build -t queue-service:${{ github.sha }} services/queue-service/
          docker build -t email-worker:${{ github.sha }} services/workers/email-worker/
          docker build -t image-worker:${{ github.sha }} services/workers/image-worker/

      - name: Push Images
        run: |
          echo ${{ secrets.REGISTRY_PASSWORD }} | docker login -u ${{ secrets.REGISTRY_USERNAME }} --password-stdin
          docker push profile-service:${{ github.sha }}
          docker push queue-service:${{ github.sha }}
          docker push email-worker:${{ github.sha }}
          docker push image-worker:${{ github.sha }}

  deploy:
    needs: build
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v3

      - name: Deploy to Kubernetes
        run: |
          kubectl set image deployment/profile-service profile-service=profile-service:${{ github.sha }}
          kubectl set image deployment/queue-service queue-service=queue-service:${{ github.sha }}
          kubectl set image deployment/email-worker email-worker=email-worker:${{ github.sha }}
          kubectl set image deployment/image-worker image-worker=image-worker:${{ github.sha }}

          kubectl rollout status deployment/profile-service
          kubectl rollout status deployment/queue-service
          kubectl rollout status deployment/email-worker
          kubectl rollout status deployment/image-worker
```

### Monitoring Setup

**Prometheus Configuration:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s

    scrape_configs:
      - job_name: 'profile-service'
        kubernetes_sd_configs:
          - role: pod
        relabel_configs:
          - source_labels: [__meta_kubernetes_pod_label_app]
            action: keep
            regex: profile-service
          - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
            action: keep
            regex: true
            
      - job_name: 'queue-service'
        kubernetes_sd_configs:
          - role: pod
        relabel_configs:
          - source_labels: [__meta_kubernetes_pod_label_app]
            action: keep
            regex: queue-service
            
      - job_name: 'workers'
        kubernetes_sd_configs:
          - role: pod
        relabel_configs:
          - source_labels: [__meta_kubernetes_pod_label_worker_type]
            action: keep
            regex: (email|image|profile)
```

### Alerting Rules

**PrometheusRule Configuration:**

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: microservices-alerts
spec:
  groups:
    - name: microservices.rules
      rules:
        - alert: HighErrorRate
          expr: |
            (
              rate(profile_service_http_requests_total{status=~"5.."}[5m]) /
              rate(profile_service_http_requests_total[5m])
            ) > 0.05
          for: 2m
          labels:
            severity: warning
          annotations:
            summary: "High error rate in profile-service"
            description: "Error rate is {{ $value | humanizePercentage }}"

        - alert: QueueBacklog
          expr: rabbitmq_queue_messages > 1000
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "High message backlog in {{ $labels.queue }}"
            description: "Queue {{ $labels.queue }} has {{ $value }} messages"

        - alert: WorkerDown
          expr: up{job=~".*worker.*"} == 0
          for: 1m
          labels:
            severity: critical
          annotations:
            summary: "Worker {{ $labels.instance }} is down"
            description: "Worker instance {{ $labels.instance }} has been down for more than 1 minute"
```

## Conclusion

This microservices task processing ecosystem provides a robust, scalable foundation for enterprise-scale task processing. The architecture separates concerns effectively:

- **Profile Service** handles API concerns and task orchestration
- **Queue Service** manages reliable message publishing and RabbitMQ integration
- **Worker Services** provide specialized processing with independent scaling

Key architectural benefits:

1. **Scalability**: Each service can scale independently based on load characteristics
2. **Reliability**: Publisher confirms, dead letter queues, and retry mechanisms ensure no message loss
3. **Maintainability**: Clean architecture and separation of concerns enable easy maintenance
4. **Observability**: Comprehensive metrics, logging, and tracing provide operational visibility
5. **Flexibility**: New worker types can be added without affecting existing services

The system follows RabbitMQ best practices, implements proper error handling and recovery, and provides the monitoring and operational capabilities needed for production deployment. This architecture can serve as a foundation for complex task processing workflows while maintaining performance, reliability, and operational excellence.
