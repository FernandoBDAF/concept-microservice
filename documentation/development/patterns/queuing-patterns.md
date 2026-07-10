# Queuing Patterns

> *Migrated and adapted from legacy_project/reference-materials/development/patterns/queuing-patterns.md*

## Overview

This document outlines the queuing patterns implemented in the API Service using direct RabbitMQ access via amqp091-go.

## Architecture Context

In the consolidated architecture, message publishing is done through **direct RabbitMQ access** using amqp091-go, not HTTP calls to a queue service.

```go
// Direct RabbitMQ publisher
import amqp "github.com/rabbitmq/amqp091-go"

type Publisher struct {
    conn    *amqp.Connection
    channel *amqp.Channel
}
```

## Message Queuing Patterns

### 1. Task Submission Pattern

Publishing tasks for background processing:

```go
type TaskPublisher struct {
    channel  *amqp.Channel
    exchange string
    logger   *zap.Logger
}

func (p *TaskPublisher) PublishTask(ctx context.Context, task *Task) error {
    body, err := json.Marshal(task)
    if err != nil {
        return fmt.Errorf("failed to marshal task: %w", err)
    }

    msg := amqp.Publishing{
        ContentType:  "application/json",
        DeliveryMode: amqp.Persistent,
        Timestamp:    time.Now(),
        MessageId:    task.ID,
        Body:         body,
    }

    err = p.channel.PublishWithContext(ctx,
        p.exchange,           // exchange
        task.Type,            // routing key
        false,                // mandatory
        false,                // immediate
        msg,
    )
    if err != nil {
        return fmt.Errorf("failed to publish task: %w", err)
    }

    p.logger.Info("Task published",
        zap.String("task_id", task.ID),
        zap.String("task_type", task.Type))

    return nil
}
```

### 2. Profile Task Types

Specific task publishing for profile operations:

```go
const (
    TaskTypeEmailVerification = "email.verification"
    TaskTypeImageProcessing   = "image.processing"
    TaskTypeProfileExport     = "profile.export"
)

type Task struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    ProfileID string                 `json:"profile_id"`
    Payload   map[string]interface{} `json:"payload"`
    CreatedAt time.Time              `json:"created_at"`
}

// Submit email verification task
func (s *ProfileService) SubmitEmailVerification(ctx context.Context, profileID, email string) error {
    task := &Task{
        ID:        uuid.New().String(),
        Type:      TaskTypeEmailVerification,
        ProfileID: profileID,
        Payload: map[string]interface{}{
            "email": email,
        },
        CreatedAt: time.Now(),
    }

    return s.publisher.PublishTask(ctx, task)
}

// Submit image processing task
func (s *ProfileService) SubmitImageProcessing(ctx context.Context, profileID, imageURL string) error {
    task := &Task{
        ID:        uuid.New().String(),
        Type:      TaskTypeImageProcessing,
        ProfileID: profileID,
        Payload: map[string]interface{}{
            "image_url": imageURL,
        },
        CreatedAt: time.Now(),
    }

    return s.publisher.PublishTask(ctx, task)
}
```

### 3. Publisher Setup

```go
func NewPublisher(url string) (*Publisher, error) {
    conn, err := amqp.Dial(url)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
    }

    channel, err := conn.Channel()
    if err != nil {
        conn.Close()
        return nil, fmt.Errorf("failed to open channel: %w", err)
    }

    // Declare exchange
    err = channel.ExchangeDeclare(
        "tasks",   // name
        "topic",   // type
        true,      // durable
        false,     // auto-deleted
        false,     // internal
        false,     // no-wait
        nil,       // arguments
    )
    if err != nil {
        channel.Close()
        conn.Close()
        return nil, fmt.Errorf("failed to declare exchange: %w", err)
    }

    return &Publisher{
        conn:    conn,
        channel: channel,
    }, nil
}
```

## Event Processing

### 1. Event Publishing Pattern

```go
type Event struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    Aggregate string                 `json:"aggregate"`
    Payload   map[string]interface{} `json:"payload"`
    Timestamp time.Time              `json:"timestamp"`
}

func (p *Publisher) PublishEvent(ctx context.Context, event *Event) error {
    body, err := json.Marshal(event)
    if err != nil {
        return err
    }

    return p.channel.PublishWithContext(ctx,
        "events",        // exchange
        event.Type,      // routing key
        false, false,
        amqp.Publishing{
            ContentType: "application/json",
            Body:        body,
        },
    )
}

// Usage
func (s *ProfileService) onProfileCreated(ctx context.Context, profile *Profile) error {
    return s.publisher.PublishEvent(ctx, &Event{
        ID:        uuid.New().String(),
        Type:      "profile.created",
        Aggregate: "profile",
        Payload: map[string]interface{}{
            "profile_id": profile.ID,
            "email":      profile.Email,
        },
        Timestamp: time.Now(),
    })
}
```

## Message Reliability

### 1. Publisher Confirms

```go
func NewReliablePublisher(url string) (*Publisher, error) {
    // ... connection setup ...

    // Enable publisher confirms
    if err := channel.Confirm(false); err != nil {
        return nil, fmt.Errorf("failed to enable confirms: %w", err)
    }

    return &Publisher{
        conn:     conn,
        channel:  channel,
        confirms: channel.NotifyPublish(make(chan amqp.Confirmation, 100)),
    }, nil
}

func (p *Publisher) PublishWithConfirm(ctx context.Context, msg amqp.Publishing) error {
    if err := p.channel.PublishWithContext(ctx, exchange, key, false, false, msg); err != nil {
        return err
    }

    // Wait for confirmation
    select {
    case confirm := <-p.confirms:
        if !confirm.Ack {
            return errors.New("message not acknowledged")
        }
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### 2. Connection Recovery

```go
func (p *Publisher) reconnect() error {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.conn != nil && !p.conn.IsClosed() {
        return nil
    }

    conn, err := amqp.Dial(p.url)
    if err != nil {
        return err
    }

    channel, err := conn.Channel()
    if err != nil {
        conn.Close()
        return err
    }

    p.conn = conn
    p.channel = channel

    return nil
}
```

## Best Practices

1. **Use persistent messages** - Set `DeliveryMode: amqp.Persistent`
2. **Enable publisher confirms** - For critical messages
3. **Implement connection recovery** - Handle reconnection gracefully
4. **Use appropriate routing** - Topic exchanges for flexibility
5. **Monitor queue depth** - Set up alerts for backlogs

## Cross-References

- [Long-Running Tasks](long-running-tasks.md)
- [Caching Patterns](caching-patterns.md)
- [Monitoring Patterns](monitoring-patterns.md)

## Notes

- Always use direct amqp091-go client, not HTTP
- Tasks are processed by external workers
- The API service only publishes, never consumes
