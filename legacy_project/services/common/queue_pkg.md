# Common Queue Package Implementation Plan

## Overview

This document outlines the detailed implementation plan for the common queue package that will be used by both the queue-service (publisher) and worker-service (consumer). The package will provide a robust, reusable implementation of RabbitMQ functionality.

## Package Structure

```
services/common/queue/
├── constants.go      # Exchange, queue, and routing key constants
├── config.go         # Configuration types and defaults
├── publisher.go      # Base publisher implementation
├── consumer.go       # Base consumer implementation
├── message.go        # Message domain model
├── errors.go         # Queue-specific errors
├── metrics.go        # Prometheus metrics
├── logger.go         # Structured logging
└── README.md         # Package documentation
```

## Detailed Implementation

### 1. Constants (constants.go)

```go
package queue

const (
    // Exchange and Queue Names
    ExchangeName = "profile-tasks"
    QueueName    = "profile-processing"
    RoutingKey   = "profile.task"

    // Default Values
    DefaultPrefetchCount = 1
    DefaultMaxRetries   = 3
    DefaultRetryDelay   = 5 * time.Second

    // Connection Settings
    DefaultHeartbeat = 10 * time.Second
    DefaultLocale    = "en_US"
)
```

### 2. Configuration (config.go)

```go
package queue

import "time"

type Config struct {
    // Connection Settings
    URL           string
    Heartbeat     time.Duration
    Locale        string

    // Exchange and Queue Settings
    Exchange      string
    Queue         string
    RoutingKey    string
    Durable       bool
    AutoDelete    bool
    Exclusive     bool
    NoWait        bool

    // Consumer Settings
    PrefetchCount int
    PrefetchSize  int
    Global        bool

    // Publisher Settings
    Mandatory     bool
    Immediate     bool

    // Retry Settings
    MaxRetries    int
    RetryDelay    time.Duration

    // Logging
    LogLevel      string
}

func NewConfig() *Config {
    return &Config{
        Exchange:      ExchangeName,
        Queue:        QueueName,
        RoutingKey:   RoutingKey,
        Durable:      true,
        AutoDelete:   false,
        Exclusive:    false,
        NoWait:       false,
        PrefetchCount: DefaultPrefetchCount,
        MaxRetries:   DefaultMaxRetries,
        RetryDelay:   DefaultRetryDelay,
        Heartbeat:    DefaultHeartbeat,
        Locale:       DefaultLocale,
        LogLevel:     "info",
    }
}
```

### 3. Message Model (message.go)

```go
package queue

import (
    "time"
    "encoding/json"
)

type Message struct {
    ID        string          `json:"id"`
    Type      string          `json:"type"`
    Payload   json.RawMessage `json:"payload"`
    Timestamp time.Time       `json:"timestamp"`
    Metadata  map[string]string `json:"metadata,omitempty"`
}

type MessageHandler func(msg *Message) error

func NewMessage(id, msgType string, payload interface{}) (*Message, error) {
    payloadBytes, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }

    return &Message{
        ID:        id,
        Type:      msgType,
        Payload:   payloadBytes,
        Timestamp: time.Now(),
        Metadata:  make(map[string]string),
    }, nil
}
```

### 4. Errors (errors.go)

```go
package queue

import "errors"

var (
    // Connection Errors
    ErrConnectionFailed = errors.New("failed to connect to RabbitMQ")
    ErrChannelFailed    = errors.New("failed to open channel")
    ErrConnectionClosed = errors.New("connection closed")

    // Publisher Errors
    ErrPublishFailed    = errors.New("failed to publish message")
    ErrPublishTimeout   = errors.New("publish confirmation timeout")
    ErrInvalidMessage   = errors.New("invalid message format")

    // Consumer Errors
    ErrConsumeFailed    = errors.New("failed to consume message")
    ErrAckFailed       = errors.New("failed to acknowledge message")
    ErrNackFailed      = errors.New("failed to negative acknowledge message")
    ErrHandlerFailed    = errors.New("message handler failed")

    // Configuration Errors
    ErrInvalidConfig   = errors.New("invalid configuration")
)
```

### 5. Metrics (metrics.go)

```go
package queue

import (
    "github.com/prometheus/client_golang/prometheus"
)

var (
    // Publisher Metrics
    messagesPublished = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "queue_messages_published_total",
            Help: "Total number of messages published",
        },
        []string{"exchange", "routing_key"},
    )

    publishErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "queue_publish_errors_total",
            Help: "Total number of publish errors",
        },
        []string{"exchange", "routing_key", "error"},
    )

    // Consumer Metrics
    messagesConsumed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "queue_messages_consumed_total",
            Help: "Total number of messages consumed",
        },
        []string{"queue"},
    )

    consumeErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "queue_consume_errors_total",
            Help: "Total number of consume errors",
        },
        []string{"queue", "error"},
    )

    processingTime = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "queue_message_processing_seconds",
            Help: "Time taken to process messages",
        },
        []string{"queue"},
    )
)

func init() {
    prometheus.MustRegister(
        messagesPublished,
        publishErrors,
        messagesConsumed,
        consumeErrors,
        processingTime,
    )
}
```

### 6. Logger (logger.go)

```go
package queue

import (
    "go.uber.org/zap"
)

type Logger struct {
    *zap.Logger
}

func NewLogger(level string) (*Logger, error) {
    config := zap.NewProductionConfig()
    config.Level = zap.NewAtomicLevelAt(getLogLevel(level))

    logger, err := config.Build()
    if err != nil {
        return nil, err
    }

    return &Logger{logger}, nil
}

func (l *Logger) WithFields(fields ...zap.Field) *Logger {
    return &Logger{l.With(fields...)}
}
```

### 7. Publisher Implementation (publisher.go)

```go
package queue

import (
    "context"
    "time"
    amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
    conn    *amqp.Connection
    channel *amqp.Channel
    config  *Config
    logger  *Logger
    done    chan struct{}
}

func NewPublisher(config *Config) (*Publisher, error) {
    logger, err := NewLogger(config.LogLevel)
    if err != nil {
        return nil, err
    }

    p := &Publisher{
        config: config,
        logger: logger,
        done:   make(chan struct{}),
    }

    if err := p.connect(); err != nil {
        return nil, err
    }

    return p, nil
}

func (p *Publisher) PublishMessage(ctx context.Context, msg *Message) error {
    // Implementation details
}

func (p *Publisher) Close() error {
    // Implementation details
}
```

### 8. Consumer Implementation (consumer.go)

```go
package queue

import (
    "context"
    amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer struct {
    conn    *amqp.Connection
    channel *amqp.Channel
    config  *Config
    logger  *Logger
    done    chan struct{}
}

func NewConsumer(config *Config) (*Consumer, error) {
    logger, err := NewLogger(config.LogLevel)
    if err != nil {
        return nil, err
    }

    c := &Consumer{
        config: config,
        logger: logger,
        done:   make(chan struct{}),
    }

    if err := c.connect(); err != nil {
        return nil, err
    }

    return c, nil
}

func (c *Consumer) Start(ctx context.Context, handler MessageHandler) error {
    // Implementation details
}

func (c *Consumer) Close() error {
    // Implementation details
}
```

## Implementation Steps

1. **Phase 1: Core Package Setup**

   - [ ] Create package structure
   - [ ] Implement constants and configuration
   - [ ] Create message model
   - [ ] Define error types
   - [ ] Set up logging

2. **Phase 2: Publisher Implementation**

   - [ ] Implement connection management
   - [ ] Add exchange declaration
   - [ ] Implement message publishing
   - [ ] Add publisher confirms
   - [ ] Implement retry logic

3. **Phase 3: Consumer Implementation**

   - [ ] Implement connection management
   - [ ] Add queue declaration
   - [ ] Implement message consumption
   - [ ] Add manual acknowledgments
   - [ ] Implement QoS settings

4. **Phase 4: Metrics and Monitoring**

   - [ ] Add Prometheus metrics
   - [ ] Implement structured logging
   - [ ] Add health checks
   - [ ] Create monitoring dashboards

5. **Phase 5: Testing**
   - [ ] Write unit tests
   - [ ] Add integration tests
   - [ ] Implement error scenario tests
   - [ ] Add performance tests

## Usage Examples

### Publisher Example

```go
config := queue.NewConfig()
config.URL = "amqp://user:pass@localhost:5672/"

publisher, err := queue.NewPublisher(config)
if err != nil {
    log.Fatal(err)
}
defer publisher.Close()

msg, err := queue.NewMessage("123", "profile.update", map[string]string{
    "user_id": "456",
    "action":  "update",
})
if err != nil {
    log.Fatal(err)
}

err = publisher.PublishMessage(context.Background(), msg)
if err != nil {
    log.Fatal(err)
}
```

### Consumer Example

```go
config := queue.NewConfig()
config.URL = "amqp://user:pass@localhost:5672/"

consumer, err := queue.NewConsumer(config)
if err != nil {
    log.Fatal(err)
}
defer consumer.Close()

handler := func(msg *queue.Message) error {
    // Process message
    return nil
}

err = consumer.Start(context.Background(), handler)
if err != nil {
    log.Fatal(err)
}
```

## Next Steps

1. **Immediate Actions**

   - [ ] Set up the package structure
   - [ ] Implement core functionality
   - [ ] Write initial tests
   - [ ] Create documentation

2. **Future Improvements**
   - [ ] Add circuit breaker
   - [ ] Implement dead letter queues
   - [ ] Add message tracing
   - [ ] Implement message validation
