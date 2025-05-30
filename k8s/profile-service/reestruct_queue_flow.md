# Queue Flow Restructuring Plan

## Overview

This document outlines the plan to restructure the message queue flow between the queue-service (publisher) and worker-service (consumer) using RabbitMQ best practices. The goal is to create a more robust, maintainable, and scalable messaging system.

## Current Architecture

- Queue Service: Acts as a publisher, sending messages to RabbitMQ
- Worker Service: Acts as a consumer, processing messages from RabbitMQ
- RabbitMQ: Message broker handling the communication

## Proposed Changes

### 1. RabbitMQ Configuration

```yaml
# New RabbitMQ StatefulSet configuration
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: rabbitmq
spec:
  serviceName: rabbitmq
  replicas: 1 # Can be scaled later
  selector:
    matchLabels:
      app: rabbitmq
  template:
    metadata:
      labels:
        app: rabbitmq
    spec:
      containers:
        - name: rabbitmq
          image: rabbitmq:3.11-management
          env:
            - name: RABBITMQ_DEFAULT_USER
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-credentials
                  key: username
            - name: RABBITMQ_DEFAULT_PASS
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-credentials
                  key: password
            - name: RABBITMQ_ERLANG_COOKIE
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-credentials
                  key: cookie
          ports:
            - name: amqp
              containerPort: 5672
            - name: http
              containerPort: 15672
          readinessProbe:
            exec:
              command: ["rabbitmq-diagnostics", "check_port_connectivity"]
            initialDelaySeconds: 10
            periodSeconds: 10
          livenessProbe:
            exec:
              command: ["rabbitmq-diagnostics", "check_port_connectivity"]
            initialDelaySeconds: 30
            periodSeconds: 30
          volumeMounts:
            - name: data
              mountPath: /var/lib/rabbitmq
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: rabbitmq-data
```

### 2. Exchange and Queue Structure

```go
// New exchange and queue configuration
const (
    ExchangeName = "profile-tasks"
    QueueName    = "profile-processing"
    RoutingKey   = "profile.task"
)

// Exchange declaration
err = ch.ExchangeDeclare(
    ExchangeName,
    "direct",  // type
    true,      // durable
    false,     // auto-deleted
    false,     // internal
    false,     // no-wait
    nil,       // arguments
)

// Queue declaration
_, err = ch.QueueDeclare(
    QueueName,
    true,      // durable
    false,     // auto-deleted
    false,     // exclusive
    false,     // no-wait
    nil,       // arguments
)

// Queue binding
err = ch.QueueBind(
    QueueName,
    RoutingKey,
    ExchangeName,
    false,
    nil,
)
```

### 3. Queue Service (Publisher) Changes

1. **Connection Management**:

   - Implement connection pooling
   - Add reconnection logic
   - Use separate channels for publishing

2. **Message Publishing**:

   - Use persistent messages
   - Implement publisher confirms
   - Add proper error handling

3. **Code Structure**:

```go
// New publisher structure
type Publisher struct {
    conn    *amqp.Connection
    channel *amqp.Channel
    config  *Config
}

func NewPublisher(config *Config) (*Publisher, error) {
    // Initialize connection and channel
}

func (p *Publisher) PublishMessage(msg *Message) error {
    // Publish with confirmation
}

func (p *Publisher) Close() {
    // Cleanup resources
}
```

### 4. Worker Service (Consumer) Changes

1. **Connection Management**:

   - Implement connection pooling
   - Add reconnection logic
   - Use separate channels for consuming

2. **Message Processing**:

   - Implement manual acknowledgments
   - Set proper QoS
   - Add proper error handling

3. **Code Structure**:

```go
// New consumer structure
type Consumer struct {
    conn    *amqp.Connection
    channel *amqp.Channel
    config  *Config
}

func NewConsumer(config *Config) (*Consumer, error) {
    // Initialize connection and channel
}

func (c *Consumer) Start() error {
    // Start consuming with proper QoS
}

func (c *Consumer) ProcessMessage(msg *amqp.Delivery) error {
    // Process message with proper error handling
}

func (c *Consumer) Close() {
    // Cleanup resources
}
```

### 5. Kubernetes Configuration Updates

#### Queue Service Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: queue-service
spec:
  template:
    spec:
      containers:
        - name: queue-service
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
```

#### Worker Service Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker-service
spec:
  template:
    spec:
      containers:
        - name: worker-service
          env:
            - name: RABBITMQ_URL
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-credentials
                  key: url
            - name: RABBITMQ_QUEUE
              value: "profile-processing"
```

## Implementation Steps

1. **Phase 1: Infrastructure Setup**

   - [ ] Create RabbitMQ StatefulSet
   - [ ] Set up persistent storage
   - [ ] Configure secrets for credentials
   - [ ] Set up monitoring and logging

2. **Phase 2: Queue Service Updates**

   - [ ] Implement new connection management
   - [ ] Add exchange and queue declarations
   - [ ] Update message publishing logic
   - [ ] Add proper error handling

3. **Phase 3: Worker Service Updates**

   - [ ] Implement new connection management
   - [ ] Add queue consumption logic
   - [ ] Update message processing
   - [ ] Add proper error handling

4. **Phase 4: Testing and Validation**
   - [ ] Test connection resilience
   - [ ] Verify message persistence
   - [ ] Test error handling
   - [ ] Validate monitoring

## Monitoring and Maintenance

1. **Metrics to Track**:

   - Connection status
   - Message rates
   - Queue lengths
   - Error rates
   - Processing times

2. **Alerts to Set Up**:
   - Connection failures
   - High queue lengths
   - High error rates
   - Processing delays

## Future Improvements

1. **Scalability**:

   - Implement RabbitMQ clustering
   - Add multiple worker instances
   - Implement message partitioning

2. **Reliability**:

   - Add dead letter queues
   - Implement retry mechanisms
   - Add circuit breakers

3. **Security**:
   - Implement TLS
   - Add proper authentication
   - Set up network policies

## Rollout Strategy

1. **Preparation**:

   - Backup current configuration
   - Document current behavior
   - Set up monitoring

2. **Implementation**:

   - Deploy RabbitMQ changes
   - Update queue service
   - Update worker service
   - Verify each step

3. **Validation**:

   - Run integration tests
   - Monitor system behavior
   - Verify message flow
   - Check error handling

4. **Rollback Plan**:
   - Document rollback steps
   - Prepare rollback scripts
   - Test rollback procedure

## Test Implementation and Results

### Test Setup

We created a minimal test implementation in `k8s/debug/reestruct_queue_flow/` to validate our proposed changes. The test includes:

1. **Publisher (`publisher.go`)**:

   - Implements connection management
   - Uses direct exchange ("profile-tasks")
   - Publishes persistent messages
   - Includes proper error handling

2. **Consumer (`consumer.go`)**:

   - Implements connection management
   - Sets up exchange and queue bindings
   - Uses manual acknowledgments
   - Implements QoS for fair message distribution

3. **Infrastructure**:

   - RabbitMQ StatefulSet with proper configuration
   - Health checks and monitoring
   - Persistent storage setup

4. **Container Setup**:
   - Multi-stage Docker build
   - Combined publisher/consumer container
   - Proper startup sequence

### Test Results

#### Successes

1. **Message Flow**:

   - Messages are successfully published to the exchange
   - Consumer receives messages in order
   - Manual acknowledgments work as expected
   - No message loss during processing
   - Consistent 1-second intervals between messages
   - Perfect message delivery and processing sequence

2. **Connection Management**:

   - Stable connections between services
   - Proper channel management
   - Clean resource cleanup
   - Successful connection establishment on startup
   - Proper exchange and queue declarations

3. **Infrastructure**:
   - RabbitMQ StatefulSet provides stable service
   - Health checks help ensure service availability
   - Service discovery works as expected
   - Proper container startup sequence
   - Successful multi-container coordination

#### Test Execution Details

1. **Setup**:

   - Successfully built and loaded Docker image
   - RabbitMQ StatefulSet deployed and ready
   - Test job deployed successfully

2. **Message Flow Sequence**:

   ```
   2025/06/01 00:05:25 Consumer started, waiting for messages on queue: profile-processing
   2025/06/01 00:05:30 Published message: Test message 1
   2025/06/01 00:05:30 Received message: Test message 1
   2025/06/01 00:05:31 Published message: Test message 2
   2025/06/01 00:05:31 Received message: Test message 2
   2025/06/01 00:05:32 Published message: Test message 3
   2025/06/01 00:05:32 Received message: Test message 3
   2025/06/01 00:05:33 Published message: Test message 4
   2025/06/01 00:05:33 Received message: Test message 4
   2025/06/01 00:05:34 Published message: Test message 5
   2025/06/01 00:05:34 Received message: Test message 5
   2025/06/01 00:05:35 Publisher finished sending messages
   ```

3. **Performance Metrics**:
   - Message processing time: < 1 second per message
   - No message loss or duplication
   - Perfect message ordering
   - Consistent timing between messages

## Production Implementation Plan

### 1. Common Package Implementation

#### Directory Structure

```
services/common/
├── queue/
│   ├── constants.go      # Shared constants
│   ├── config.go         # Common configuration
│   ├── publisher.go      # Base publisher implementation
│   ├── consumer.go       # Base consumer implementation
│   ├── message.go        # Message domain model
│   └── errors.go         # Queue-specific errors
└── README.md            # Documentation
```

#### Implementation Steps

1. **Create Queue Constants**:

   ```go
   // queue/constants.go
   package queue

   const (
       ExchangeName = "profile-tasks"
       QueueName    = "profile-processing"
       RoutingKey   = "profile.task"
   )
   ```

2. **Create Common Configuration**:

   ```go
   // queue/config.go
   package queue

   type Config struct {
       URL           string
       Exchange      string
       Queue         string
       RoutingKey    string
       PrefetchCount int
       MaxRetries    int
       RetryDelay    time.Duration
   }

   func NewConfig() *Config {
       return &Config{
           Exchange:      ExchangeName,
           Queue:        QueueName,
           RoutingKey:   RoutingKey,
           PrefetchCount: 1,
           MaxRetries:   3,
           RetryDelay:   time.Second * 5,
       }
   }
   ```

3. **Create Base Publisher**:

   ```go
   // queue/publisher.go
   package queue

   type Publisher struct {
       conn    *amqp.Connection
       channel *amqp.Channel
       config  *Config
   }

   func NewPublisher(config *Config) (*Publisher, error) {
       // Initialize connection and channel
       // Declare exchange
       // Set up publisher confirms
   }

   func (p *Publisher) PublishMessage(msg *Message) error {
       // Publish with confirmation
       // Handle errors
       // Implement retry logic
   }
   ```

4. **Create Base Consumer**:

   ```go
   // queue/consumer.go
   package queue

   type Consumer struct {
       conn    *amqp.Connection
       channel *amqp.Channel
       config  *Config
   }

   func NewConsumer(config *Config) (*Consumer, error) {
       // Initialize connection and channel
       // Declare exchange and queue
       // Set up QoS
   }

   func (c *Consumer) Start(handler MessageHandler) error {
       // Start consuming
       // Handle messages
       // Implement error handling
   }
   ```

5. **Create Message Model**:

   ```go
   // queue/message.go
   package queue

   type Message struct {
       ID        string
       Type      string
       Payload   []byte
       Timestamp time.Time
   }

   type MessageHandler func(msg *Message) error
   ```

6. **Create Queue Errors**:

   ```go
   // queue/errors.go
   package queue

   import "errors"

   var (
       ErrConnectionFailed = errors.New("failed to connect to RabbitMQ")
       ErrChannelFailed    = errors.New("failed to open channel")
       ErrPublishFailed    = errors.New("failed to publish message")
       ErrConsumeFailed    = errors.New("failed to consume message")
   )
   ```

### 2. Queue Service (Publisher) Implementation

#### Directory Structure

```
services/queue-service/
├── cmd/
│   └── server/
│       └── main.go           # Main entry point
├── internal/
│   ├── adapters/
│   │   └── queue/
│   │       ├── publisher.go  # Publisher implementation
│   │       └── config.go     # Service-specific config
│   └── domain/
│       └── message.go        # Service-specific message model
└── k8s/
    └── base/
        ├── deployment.yaml   # Updated deployment
        └── secrets.yaml      # RabbitMQ credentials
```

#### Implementation Steps

1. **Create Service-Specific Publisher**:

   ```go
   // internal/adapters/queue/publisher.go
   package queue

   import (
       "github.com/yourusername/common/queue"
   )

   type Publisher struct {
       *queue.Publisher
       // Add service-specific fields
   }

   func NewPublisher(config *queue.Config) (*Publisher, error) {
       basePublisher, err := queue.NewPublisher(config)
       if err != nil {
           return nil, err
       }
       return &Publisher{Publisher: basePublisher}, nil
   }

   // Add service-specific methods
   ```

### 3. Worker Service (Consumer) Implementation

#### Directory Structure

```
services/worker-service/
├── cmd/
│   └── worker/
│       └── main.go           # Main entry point
├── internal/
│   ├── adapters/
│   │   └── queue/
│   │       ├── consumer.go   # Consumer implementation
│   │       └── config.go     # Service-specific config
│   └── domain/
│       └── processor.go      # Message processor
└── k8s/
    └── base/
        ├── deployment.yaml   # Updated deployment
        └── secrets.yaml      # RabbitMQ credentials
```

#### Implementation Steps

1. **Create Service-Specific Consumer**:

   ```go
   // internal/adapters/queue/consumer.go
   package queue

   import (
       "github.com/yourusername/common/queue"
   )

   type Consumer struct {
       *queue.Consumer
       // Add service-specific fields
   }

   func NewConsumer(config *queue.Config) (*Consumer, error) {
       baseConsumer, err := queue.NewConsumer(config)
       if err != nil {
           return nil, err
       }
       return &Consumer{Consumer: baseConsumer}, nil
   }

   // Add service-specific methods
   ```

### 4. Implementation Timeline

1. **Week 1: Common Package**

   - [ ] Create queue package structure
   - [ ] Implement base publisher
   - [ ] Implement base consumer
   - [ ] Add configuration and constants
   - [ ] Write tests and documentation

2. **Week 2: Queue Service Updates**

   - [ ] Create service-specific publisher
   - [ ] Update configuration
   - [ ] Implement message publishing
   - [ ] Add error handling
   - [ ] Update Kubernetes config

3. **Week 3: Worker Service Updates**

   - [ ] Create service-specific consumer
   - [ ] Update configuration
   - [ ] Implement message processing
   - [ ] Add error handling
   - [ ] Update Kubernetes config

4. **Week 4: Testing and Deployment**
   - [ ] Integration tests
   - [ ] Load testing
   - [ ] Deploy to staging
   - [ ] Set up monitoring
   - [ ] Document changes

### 5. Success Criteria

1. **Functionality**:

   - All messages are delivered successfully
   - No message loss or duplication
   - Proper error handling and recovery
   - Correct message ordering

2. **Performance**:

   - Message processing time < 1 second
   - No connection issues
   - Stable under load
   - Proper resource usage

3. **Reliability**:

   - Automatic reconnection
   - Message persistence
   - Proper error handling
   - Graceful shutdown

4. **Monitoring**:
   - Connection status
   - Message rates
   - Error rates
   - Processing times
