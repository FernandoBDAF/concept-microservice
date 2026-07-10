# Queue Service Technical Context

## Production-Ready Architecture

The queue-service implements a production-ready architecture with RabbitMQ best practices and multi-worker support.

For complete implementation history, see `IMPLEMENTATION_HISTORY.md`.

---

## Internal Structure

### **Production Architecture Overview**

The upgraded queue-service follows clean architecture principles with **RabbitMQ best practices integration** and **multi-worker support**:

```
┌─────────────────────────────────────────────────────────────────┐
│                     Queue Service                               │
├─────────────────────────────────────────────────────────────────┤
│  HTTP Layer (Gin)                                               │
│  ├── /api/v1/queue/messages (Enhanced with routing keys)       │
│  ├── /api/v1/queue/status/{id} (Publisher confirm tracking)    │
│  ├── /health (RabbitMQ connection health)                      │
│  └── /metrics (Enhanced worker-type metrics)                   │
├─────────────────────────────────────────────────────────────────┤
│  Domain Layer                                                   │
│  ├── Message Model (Aligned with common queue package)         │
│  ├── Routing Logic (Routing key → Exchange mapping)            │
│  └── Publisher Service (Publisher confirms + retry logic)      │
├─────────────────────────────────────────────────────────────────┤
│  Infrastructure Layer                                           │
│  ├── RabbitMQ Publisher (Single exchange + routing keys)       │
│  ├── Connection Manager (Long-lived connection pattern)        │
│  └── Metrics Collector (Per-worker-type metrics)               │
└─────────────────────────────────────────────────────────────────┘
                                ↓
┌─────────────────────────────────────────────────────────────────┐
│                     RabbitMQ Broker                            │
├─────────────────────────────────────────────────────────────────┤
│  Exchange: tasks-exchange (profile.task)                       │
│  Exchange: email-tasks (email.send)                            │
│  Exchange: image-tasks (image.process)                         │
├─────────────────────────────────────────────────────────────────┤
│  Queue: profile-processing → Profile Worker                    │
│  Queue: email-processing → Email Worker                        │
│  Queue: image-processing → Image Worker                        │
└─────────────────────────────────────────────────────────────────┘
```

### **Core Components (Post-Upgrade)**

#### **1. Domain Layer** (`internal/domain/`)

##### **Enhanced Message Model** (`model/message.go`)

```go
// Aligned with common queue package format
type Message struct {
    ID        string            `json:"id"`
    Type      string            `json:"type"`           // String instead of enum
    Payload   json.RawMessage   `json:"payload"`        // RawMessage instead of interface{}
    Timestamp time.Time         `json:"timestamp"`
    Metadata  map[string]string `json:"metadata"`       // Renamed from Headers
    Priority  int32             `json:"priority"`
}

// Routing configuration
type RoutingConfig struct {
    RoutingKey string `json:"routing_key"`    // NEW: "profile.task", "email.send", "image.process"
    Exchange   string `json:"exchange"`       // Target exchange
    Queue      string `json:"queue"`          // Target queue
    TTL        time.Duration                  // Worker-specific TTL
    Prefetch   int                           // Worker-specific prefetch
}
```

##### **Enhanced Publisher Service** (`service/publisher.go`)

```go
type PublisherService struct {
    rabbitmq      RabbitMQPublisher
    routingConfig map[string]RoutingConfig  // Routing key → config mapping
    metrics       MetricsCollector
    confirmChan   <-chan amqp.Confirmation  // Publisher confirms
}

// Enhanced publishing with routing keys
func (s *PublisherService) PublishMessage(msg *Message, routingKey string) error {
    // 1. Validate routing key
    config, exists := s.routingConfig[routingKey]
    if !exists {
        return ErrInvalidRoutingKey
    }

    // 2. Publish to appropriate exchange
    err := s.rabbitmq.Publish(config.Exchange, routingKey, msg)
    if err != nil {
        s.metrics.RecordPublishError(routingKey, err)
        return err
    }

    // 3. Wait for publisher confirm
    select {
    case confirm := <-s.confirmChan:
        if confirm.Ack {
            s.metrics.RecordPublishSuccess(routingKey)
            return nil
        } else {
            s.metrics.RecordPublishNack(routingKey)
            return ErrPublishNack
        }
    case <-time.After(5 * time.Second):
        s.metrics.RecordPublishTimeout(routingKey)
        return ErrPublishTimeout
    }
}
```

##### **Routing Strategy** (`service/routing.go`)

```go
// Routing key to exchange mapping
var RoutingMap = map[string]RoutingConfig{
    "profile.task": {
        Exchange: "tasks-exchange",
        Queue:    "profile-processing",
        TTL:      24 * time.Hour,
        Prefetch: 1,
    },
    "email.send": {
        Exchange: "email-tasks",
        Queue:    "email-processing",
        TTL:      1 * time.Hour,
        Prefetch: 5,
    },
    "image.process": {
        Exchange: "image-tasks",
        Queue:    "image-processing",
        TTL:      6 * time.Hour,
        Prefetch: 1,
    },
}
```

#### **2. Infrastructure Layer** (`internal/adapters/`)

##### **Enhanced RabbitMQ Publisher** (`rabbitmq/publisher.go`)

```go
type Publisher struct {
    conn     *amqp.Connection    // Single long-lived connection
    channel  *amqp.Channel       // Single channel for publishing
    confirms <-chan amqp.Confirmation
    config   *Config
}

// Best practice connection pattern (from rabbit+go+kind.md)
func (p *Publisher) Connect() error {
    // 1. Single connection per service
    conn, err := amqp.Dial(p.config.URL)
    if err != nil {
        return err
    }
    p.conn = conn

    // 2. Single channel for publishing
    ch, err := conn.Channel()
    if err != nil {
        return err
    }
    p.channel = ch

    // 3. Enable publisher confirms
    if err := ch.Confirm(false); err != nil {
        return err
    }
    p.confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 1))

    return nil
}

// Simplified exchange and queue setup
func (p *Publisher) EnsureExchangeAndQueue(exchange, queue, routingKey string) error {
    // Declare exchange (idempotent)
    err := p.channel.ExchangeDeclare(
        exchange, "direct", true, false, false, false, nil,
    )
    if err != nil {
        return err
    }

    // Declare queue (idempotent)
    _, err = p.channel.QueueDeclare(
        queue, true, false, false, false, nil,
    )
    if err != nil {
        return err
    }

    // Bind queue to exchange
    return p.channel.QueueBind(
        queue, routingKey, exchange, false, nil,
    )
}

// Simplified publish method
func (p *Publisher) Publish(exchange, routingKey string, msg *Message) error {
    body, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    return p.channel.Publish(
        exchange,
        routingKey,
        false, // mandatory
        false, // immediate
        amqp.Publishing{
            ContentType:  "application/json",
            Body:         body,
            DeliveryMode: amqp.Persistent,
            MessageId:    msg.ID,
            Timestamp:    msg.Timestamp,
        },
    )
}
```

##### **Enhanced HTTP Handler** (`http/handler.go`)

```go
type Handler struct {
    publisherService *service.PublisherService
    routingValidator *RoutingValidator
    metrics         *MetricsCollector
}

// Enhanced message publishing endpoint
func (h *Handler) PublishMessage(c *gin.Context) {
    var req PublishRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid_request", "details": err.Error()})
        return
    }

    // Validate routing key
    if !h.routingValidator.IsValidRoutingKey(req.RoutingKey) {
        c.JSON(400, gin.H{
            "error": "invalid_routing_key",
            "message": fmt.Sprintf("Routing key '%s' is not supported", req.RoutingKey),
            "supported_keys": h.routingValidator.GetSupportedKeys(),
        })
        return
    }

    // Create message
    msg := &Message{
        ID:        uuid.New().String(),
        Type:      req.Type,
        Payload:   req.Payload,
        Timestamp: time.Now(),
        Metadata:  req.Metadata,
        Priority:  req.Priority,
    }

    // Publish with routing key
    if err := h.publisherService.PublishMessage(msg, req.RoutingKey); err != nil {
        h.metrics.RecordAPIError("publish", err)
        c.JSON(500, gin.H{"error": "publish_failed", "details": err.Error()})
        return
    }

    // Enhanced response
    c.JSON(202, gin.H{
        "message_id":        msg.ID,
        "status":           "accepted",
        "routing_key":      req.RoutingKey,
        "target_queue":     h.routingValidator.GetTargetQueue(req.RoutingKey),
        "exchange":         h.routingValidator.GetTargetExchange(req.RoutingKey),
        "timestamp":        msg.Timestamp,
        "publisher_confirm": "pending",
    })
}

// New API request format
type PublishRequest struct {
    Type       string            `json:"type" binding:"required"`
    RoutingKey string            `json:"routing_key" binding:"required"`  // NEW
    Payload    json.RawMessage   `json:"payload" binding:"required"`
    Metadata   map[string]string `json:"metadata"`                       // Renamed from Headers
    Priority   int32             `json:"priority"`
}
```

### **Design Patterns (Enhanced)**

#### **1. Clean Architecture (Maintained)**

- **Domain Independence**: Core business logic independent of infrastructure
- **Dependency Inversion**: Interfaces define contracts, implementations are injected
- **Single Responsibility**: Each layer has distinct responsibilities

#### **2. Publisher Pattern (Enhanced)**

```go
// Publisher interface with routing key support
type MessagePublisher interface {
    PublishMessage(msg *Message, routingKey string) error
    PublishBatch(messages []MessageWithRouting) error
    GetPublisherConfirms() <-chan PublishConfirm
}

// Routing strategy pattern
type RoutingStrategy interface {
    GetExchange(routingKey string) (string, error)
    GetQueue(routingKey string) (string, error)
    ValidateRoutingKey(routingKey string) error
    GetWorkerConfig(routingKey string) (*WorkerConfig, error)
}
```

#### **3. Connection Management Pattern (New)**

```go
// Best practice connection management
type ConnectionManager struct {
    conn      *amqp.Connection
    channels  map[string]*amqp.Channel  // Named channels for different purposes
    reconnect chan struct{}
    done      chan struct{}
}

func (cm *ConnectionManager) GetPublisherChannel() *amqp.Channel {
    return cm.channels["publisher"]
}

func (cm *ConnectionManager) MonitorConnection() {
    notifyClose := cm.conn.NotifyClose(make(chan *amqp.Error))
    for {
        select {
        case err := <-notifyClose:
            if err != nil {
                log.Printf("Connection lost: %v", err)
                cm.reconnect <- struct{}{}
            }
        case <-cm.done:
            return
        }
    }
}
```

#### **4. Publisher Confirms Pattern (New)**

```go
// Publisher confirms handling
type ConfirmHandler struct {
    confirms    <-chan amqp.Confirmation
    pending     map[uint64]PendingMessage  // Delivery tag → message mapping
    confirmChan chan PublishResult
}

func (ch *ConfirmHandler) HandleConfirms() {
    for confirm := range ch.confirms {
        if msg, exists := ch.pending[confirm.DeliveryTag]; exists {
            result := PublishResult{
                MessageID: msg.ID,
                Success:   confirm.Ack,
                Timestamp: time.Now(),
            }
            ch.confirmChan <- result
            delete(ch.pending, confirm.DeliveryTag)
        }
    }
}
```

### **Frameworks and Libraries (Updated)**

#### **1. Core Dependencies**

- **Gin**: HTTP framework (maintained)
- **RabbitMQ AMQP 0.9.1**: `github.com/rabbitmq/amqp091-go` (updated from streadway/amqp)
- **Prometheus**: Metrics collection (enhanced)
- **UUID**: Message identification (maintained)
- **Common Queue Package**: Shared message format and utilities (new)

#### **2. New Dependencies**

- **Routing Validation**: Custom routing key validation
- **Publisher Confirms**: Reliable delivery confirmation
- **Connection Monitoring**: Advanced connection health monitoring

## Implementation Details (Post-Upgrade)

### **Message Processing Flow (Enhanced)**

#### **1. Message Reception and Validation**

```
HTTP Request → Gin Handler → Request Validation → Routing Key Validation
     ↓              ↓              ↓                      ↓
JSON Binding → Message Creation → Routing Config → Exchange Selection
```

#### **2. Enhanced Message Publishing**

```
Message + Routing Key → Exchange Selection → Queue Binding → RabbitMQ Publish
         ↓                      ↓               ↓              ↓
Publisher Confirm Wait → Confirm Handler → Status Update → HTTP Response
```

#### **3. Multi-Worker Message Distribution**

```
                    Single Queue Service
                           ↓
                 ┌─────────┼─────────┐
                 ↓         ↓         ↓
         profile.task  email.send  image.process
                 ↓         ↓         ↓
        tasks-exchange email-tasks image-tasks
                 ↓         ↓         ↓
     profile-processing email-processing image-processing
                 ↓         ↓         ↓
      Profile Worker   Email Worker  Image Worker
```

### **RabbitMQ Integration (Aligned with Best Practices)**

#### **1. Connection Management (Simplified)**

```go
// Best practice: One connection per service
conn, err := amqp.Dial(config.URL)
if err != nil {
    return err
}

// Single channel for publishing
ch, err := conn.Channel()
if err != nil {
    return err
}

// Enable publisher confirms
ch.Confirm(false)
confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
```

#### **2. Exchange and Queue Strategy (Simplified)**

```go
// Single exchange declaration per worker type
exchanges := []string{"tasks-exchange", "email-tasks", "image-tasks"}
for _, exchange := range exchanges {
    err := ch.ExchangeDeclare(exchange, "direct", true, false, false, false, nil)
    if err != nil {
        return err
    }
}

// Queue binding with routing keys
bindings := map[string]struct{Exchange, Queue, RoutingKey string}{
    "profile": {"tasks-exchange", "profile-processing", "profile.task"},
    "email":   {"email-tasks", "email-processing", "email.send"},
    "image":   {"image-tasks", "image-processing", "image.process"},
}

for _, binding := range bindings {
    _, err := ch.QueueDeclare(binding.Queue, true, false, false, false, nil)
    if err != nil {
        return err
    }

    err = ch.QueueBind(binding.Queue, binding.RoutingKey, binding.Exchange, false, nil)
    if err != nil {
        return err
    }
}
```

#### **3. Message Publishing (Simplified)**

```go
// Direct publishing to exchange with routing key
err := ch.Publish(
    exchange,    // Exchange name
    routingKey,  // Routing key
    false,       // Mandatory
    false,       // Immediate
    amqp.Publishing{
        ContentType:  "application/json",
        Body:         messageBody,
        DeliveryMode: amqp.Persistent,
        MessageId:    messageID,
        Timestamp:    time.Now(),
    },
)
```

### **Enhanced HTTP API Implementation**

#### **1. Routing Key Support**

```go
// New endpoint with routing key support
router.POST("/api/v1/queue/messages", func(c *gin.Context) {
    var req struct {
        Type       string            `json:"type"`
        RoutingKey string            `json:"routing_key"`  // NEW
        Payload    json.RawMessage   `json:"payload"`
        Metadata   map[string]string `json:"metadata"`     // Renamed
    }

    // Validate routing key
    if !isValidRoutingKey(req.RoutingKey) {
        c.JSON(400, gin.H{"error": "invalid_routing_key"})
        return
    }

    // Publish with routing key
    err := publisher.Publish(req.RoutingKey, &req)
    if err != nil {
        c.JSON(500, gin.H{"error": "publish_failed"})
        return
    }

    c.JSON(202, gin.H{"status": "accepted", "routing_key": req.RoutingKey})
})
```

#### **2. Enhanced Status Tracking**

```go
// Enhanced status endpoint with publisher confirm info
router.GET("/api/v1/queue/status/:id", func(c *gin.Context) {
    messageID := c.Param("id")
    status := statusTracker.GetStatus(messageID)

    c.JSON(200, gin.H{
        "id":                messageID,
        "status":           status.Status,
        "routing_key":      status.RoutingKey,
        "target_queue":     status.TargetQueue,
        "publisher_confirm": status.PublisherConfirm,
        "timestamps":       status.Timestamps,
    })
})
```

### **Enhanced Metrics and Monitoring**

#### **1. Worker-Type Specific Metrics**

```go
// Enhanced metrics with routing key labels
type EnhancedMetrics struct {
    MessagesPublished    *prometheus.CounterVec   // Labels: routing_key, worker_type
    PublisherConfirms    *prometheus.CounterVec   // Labels: routing_key, status
    RoutingKeyDistribution *prometheus.CounterVec // Labels: routing_key
    PublishLatency       *prometheus.HistogramVec // Labels: routing_key
    ConnectionHealth     *prometheus.GaugeVec     // Labels: connection_type
    ExchangeHealth       *prometheus.GaugeVec     // Labels: exchange
}

// Record metrics with routing key context
func (m *EnhancedMetrics) RecordPublish(routingKey, workerType string) {
    m.MessagesPublished.WithLabelValues(routingKey, workerType).Inc()
    m.RoutingKeyDistribution.WithLabelValues(routingKey).Inc()
}
```

#### **2. Health Check Enhancement**

```go
// Enhanced health check with RabbitMQ connection status
func (h *Handler) HealthCheck(c *gin.Context) {
    health := gin.H{
        "status": "healthy",
        "checks": gin.H{
            "rabbitmq_connection": h.connectionManager.IsHealthy(),
            "rabbitmq_channel":    h.connectionManager.IsChannelHealthy(),
            "exchange_status":     h.exchangeManager.GetExchangeStatus(),
            "publisher_confirms":  h.publisherService.IsConfirmsEnabled(),
        },
        "timestamp": time.Now(),
    }

    c.JSON(200, health)
}
```

## Integration Patterns (Enhanced)

### **1. Multi-Worker Integration Pattern**

```
Profile Service → Queue Service (routing_key: profile.task) → Profile Worker
Notification Service → Queue Service (routing_key: email.send) → Email Worker
Media Service → Queue Service (routing_key: image.process) → Image Worker
```

### **2. Publisher Confirms Pattern**

```
Queue Service → RabbitMQ → Publisher Confirm → Status Update → Client Notification
```

### **3. Dead Letter Queue Pattern (Enhanced)**

```
Failed Message → Dead Letter Exchange → Worker-Specific DLQ → Manual Investigation
```

## Error Handling (Enhanced)

### **1. Routing Key Validation**

```go
type RoutingKeyError struct {
    Key           string   `json:"key"`
    SupportedKeys []string `json:"supported_keys"`
}

func (e *RoutingKeyError) Error() string {
    return fmt.Sprintf("invalid routing key '%s', supported: %v", e.Key, e.SupportedKeys)
}
```

### **2. Publisher Confirm Failures**

```go
type PublishConfirmError struct {
    MessageID    string        `json:"message_id"`
    RoutingKey   string        `json:"routing_key"`
    ConfirmNack  bool          `json:"confirm_nack"`
    Timeout      time.Duration `json:"timeout,omitempty"`
}
```

### **3. Connection Recovery**

```go
func (cm *ConnectionManager) HandleReconnection() {
    for {
        select {
        case <-cm.reconnect:
            log.Printf("Attempting to reconnect to RabbitMQ")
            for i := 0; i < maxRetries; i++ {
                if err := cm.Connect(); err == nil {
                    log.Printf("Successfully reconnected")
                    break
                }
                time.Sleep(reconnectDelay)
            }
        case <-cm.done:
            return
        }
    }
}
```

## Performance Considerations (Enhanced)

### **1. Connection Efficiency**

- **Single Connection**: One long-lived connection per service instance
- **Channel Reuse**: Single channel for publishing to reduce overhead
- **Connection Pooling**: Future enhancement for high-throughput scenarios

### **2. Publisher Confirms Optimization**

- **Batch Confirms**: Handle multiple confirms efficiently
- **Timeout Management**: Configurable confirm timeouts per routing key
- **Retry Logic**: Intelligent retry with exponential backoff

### **3. Worker-Specific Optimization**

```go
// Worker-specific configurations
var WorkerConfigs = map[string]WorkerConfig{
    "profile.task": {
        Prefetch:    1,    // Sequential processing
        TTL:         24 * time.Hour,
        MaxRetries:  3,
    },
    "email.send": {
        Prefetch:    5,    // Burst processing
        TTL:         1 * time.Hour,
        MaxRetries:  5,
    },
    "image.process": {
        Prefetch:    1,    // Resource intensive
        TTL:         6 * time.Hour,
        MaxRetries:  2,
    },
}
```

## Security Enhancements

### **1. Routing Key Validation**

- **Whitelist Approach**: Only predefined routing keys accepted
- **Schema Validation**: Payload validation per routing key
- **Rate Limiting**: Per-routing-key rate limits

### **2. Message Sanitization**

- **Input Validation**: Strict payload validation
- **Size Limits**: Maximum message size per worker type
- **Content Filtering**: Prevent malicious payload injection

## Future Architecture Evolution

### **1. Advanced Routing**

- **Topic Exchanges**: Support for wildcard routing patterns
- **Content-Based Routing**: Route based on message content
- **Multi-Tenancy**: Tenant-specific routing keys

### **2. Performance Enhancements**

- **Connection Pooling**: Multiple connections for high throughput
- **Batch Publishing**: Bulk message publishing
- **Streaming Integration**: Integration with streaming platforms

### **3. Observability**

- **Distributed Tracing**: End-to-end message tracing
- **Real-time Monitoring**: Live message flow visualization
- **Advanced Alerting**: Predictive alerting based on patterns

---

**Status**: 🎉 **PRODUCTION READY** - RabbitMQ best practices and multi-worker support fully implemented.
