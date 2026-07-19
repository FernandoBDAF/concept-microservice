package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	oteltrace "go.opentelemetry.io/otel/trace"
)

// Consumer connects to RabbitMQ, declares its topology idempotently, and
// consumes a single queue with automatic reconnect on connection/channel
// loss. Poison messages (unparseable or handler-rejected) are nack'd
// without requeue so they land on the dead-letter queue instead of being
// redelivered forever.
type Consumer struct {
	config *Config
	logger *Logger

	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel

	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

func NewConsumer(config *Config) (*Consumer, error) {
	logger, err := NewLogger(config.LogLevel)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		config: config,
		logger: logger,
		done:   make(chan struct{}),
	}, nil
}

// connect (re)establishes the connection/channel as needed and declares
// the full topology idempotently: main exchange, dead-letter exchange,
// main queue (with DLX args), dead-letter queue, and both bindings. All
// declarations must match the publisher's args exactly (see Config docs).
func (c *Consumer) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil || c.conn.IsClosed() {
		conn, err := amqp.DialConfig(c.config.URL, amqp.Config{
			Heartbeat: c.config.Heartbeat,
			Locale:    c.config.Locale,
		})
		if err != nil {
			return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
		}
		c.conn = conn
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrChannelFailed, err)
	}

	dlxName := c.config.Exchange + ".dlx"
	dlqName := c.config.Queue + ".dlq"

	// Main exchange (direct, durable) — matches the publisher's declaration.
	if err := ch.ExchangeDeclare(
		c.config.Exchange,
		"direct",
		c.config.Durable,
		c.config.AutoDelete,
		false, // internal
		c.config.NoWait,
		nil,
	); err != nil {
		_ = ch.Close()
		return fmt.Errorf("declare exchange %s: %w", c.config.Exchange, err)
	}

	// Dead-letter exchange.
	if err := ch.ExchangeDeclare(
		dlxName,
		"direct",
		c.config.Durable,
		c.config.AutoDelete,
		false,
		c.config.NoWait,
		nil,
	); err != nil {
		_ = ch.Close()
		return fmt.Errorf("declare dlx %s: %w", dlxName, err)
	}

	// Main queue: args must be byte-for-byte equivalent to the publisher's
	// ensureTopology() or RabbitMQ rejects the redeclare.
	queueArgs := amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": c.config.RoutingKey,
		"x-message-ttl":             int32(c.config.MessageTTL.Milliseconds()),
		"x-max-retries":             c.config.MaxRetries,
	}
	if _, err := ch.QueueDeclare(
		c.config.Queue,
		c.config.Durable,
		c.config.AutoDelete,
		c.config.Exclusive,
		c.config.NoWait,
		queueArgs,
	); err != nil {
		_ = ch.Close()
		return fmt.Errorf("declare queue %s: %w", c.config.Queue, err)
	}

	if err := ch.QueueBind(
		c.config.Queue,
		c.config.RoutingKey,
		c.config.Exchange,
		c.config.NoWait,
		nil,
	); err != nil {
		_ = ch.Close()
		return fmt.Errorf("bind queue %s: %w", c.config.Queue, err)
	}

	// Dead-letter queue.
	dlqArgs := amqp.Table{
		"x-message-ttl": int32(c.config.DeadLetterTTL.Milliseconds()),
	}
	if _, err := ch.QueueDeclare(
		dlqName,
		c.config.Durable,
		c.config.AutoDelete,
		c.config.Exclusive,
		c.config.NoWait,
		dlqArgs,
	); err != nil {
		_ = ch.Close()
		return fmt.Errorf("declare dlq %s: %w", dlqName, err)
	}

	if err := ch.QueueBind(
		dlqName,
		c.config.RoutingKey,
		dlxName,
		c.config.NoWait,
		nil,
	); err != nil {
		_ = ch.Close()
		return fmt.Errorf("bind dlq %s: %w", dlqName, err)
	}

	if err := ch.Qos(c.config.PrefetchCount, c.config.PrefetchSize, c.config.Global); err != nil {
		_ = ch.Close()
		return fmt.Errorf("set qos: %w", err)
	}

	if c.channel != nil {
		_ = c.channel.Close()
	}
	c.channel = ch

	return nil
}

// Start launches the consume loop in the background and returns
// immediately. It reconnects with backoff on connection/channel loss and
// stops cleanly when ctx is cancelled or Close is called.
func (c *Consumer) Start(ctx context.Context, handler MessageHandler) error {
	c.wg.Add(1)
	go c.run(ctx, handler)
	return nil
}

func (c *Consumer) run(ctx context.Context, handler MessageHandler) {
	defer c.wg.Done()

	backoff := c.config.RetryDelay
	if backoff <= 0 {
		backoff = DefaultRetryDelay
	}
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		default:
		}

		if err := c.connect(); err != nil {
			c.logger.Error("failed to connect to rabbitmq, retrying",
				zap.Error(err), zap.Duration("backoff", backoff))
			if !c.sleep(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}
		backoff = c.config.RetryDelay
		if backoff <= 0 {
			backoff = DefaultRetryDelay
		}

		c.mu.Lock()
		ch := c.channel
		c.mu.Unlock()

		deliveries, err := ch.Consume(
			c.config.Queue,
			"",    // consumer tag
			false, // auto-ack
			false, // exclusive
			false, // no-local
			false, // no-wait
			nil,   // args
		)
		if err != nil {
			c.logger.Error("failed to start consuming, retrying",
				zap.Error(err), zap.Duration("backoff", backoff))
			if !c.sleep(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}

		c.logger.Info("consuming", zap.String("queue", c.config.Queue))
		connectionLost := c.consumeLoop(ctx, deliveries, handler)
		if !connectionLost {
			// Stopped because of shutdown, not a connection/channel loss.
			return
		}

		c.logger.Warn("delivery channel closed unexpectedly, reconnecting",
			zap.String("queue", c.config.Queue))
		if !c.sleep(ctx, backoff) {
			return
		}
		backoff = nextBackoff(backoff, maxBackoff)
	}
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}

// sleep waits for d, returning false if shutdown was signaled meanwhile.
func (c *Consumer) sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	case <-c.done:
		return false
	}
}

// consumeLoop processes deliveries until the channel/connection is lost
// (returns true, so the caller reconnects) or shutdown is signaled
// (returns false). Each delivery is fully handled (acked or nack'd)
// before the next select iteration, so an in-flight message always
// finishes before Close() can tear down the channel.
func (c *Consumer) consumeLoop(ctx context.Context, deliveries <-chan amqp.Delivery, handler MessageHandler) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case <-c.done:
			return false
		case delivery, ok := <-deliveries:
			if !ok {
				return true
			}
			c.handleDelivery(delivery, handler)
		}
	}
}

func (c *Consumer) handleDelivery(delivery amqp.Delivery, handler MessageHandler) {
	startTime := time.Now()

	var msg Message
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		c.logger.Error("failed to unmarshal message; dropping to DLQ",
			zap.Error(err), zap.String("queue", c.config.Queue))
		incrementConsumeErrors(c.config.Queue, "unmarshal_error")
		if nackErr := delivery.Nack(false, false); nackErr != nil {
			c.logger.Error("failed to nack unparseable message", zap.Error(nackErr))
		}
		return
	}

	// Continue the trace started by the publisher: the parent span context
	// arrives in the AMQP headers (traceparent, injected from envelope
	// metadata). The consumer span is deliberately rooted in
	// context.Background(), not the shutdown context, so an in-flight
	// message is never aborted mid-processing by SIGTERM.
	ctx := extractTraceContext(context.Background(), delivery.Headers)
	ctx, span := otel.Tracer("operational-workers/queue").Start(ctx,
		"consume "+c.config.Queue,
		oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
		oteltrace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.operation", "process"),
			attribute.String("messaging.destination.name", c.config.Queue),
			attribute.String("messaging.message.id", msg.ID),
		),
	)
	defer span.End()

	if err := handler(ctx, &msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "message processing failed")
		// trace_id makes the EXP-31 triage pivot work: DLQ panel -> this
		// log line -> every other log line of the same trace. The span is
		// always valid here (extracted parent or a fresh root).
		c.logger.Error("failed to process message; dropping to DLQ",
			zap.Error(err),
			zap.String("queue", c.config.Queue),
			zap.String("message_id", msg.ID),
			zap.String("trace_id", span.SpanContext().TraceID().String()))
		incrementConsumeErrors(c.config.Queue, "handler_error")
		// No requeue: a message that fails processing is either poison or
		// will keep failing. Requeueing would spin it forever; the DLQ
		// (bound via x-dead-letter-exchange) is where it belongs.
		if nackErr := delivery.Nack(false, false); nackErr != nil {
			c.logger.Error("failed to nack failed message", zap.Error(nackErr))
		}
		return
	}

	if err := delivery.Ack(false); err != nil {
		c.logger.Error("failed to acknowledge message",
			zap.Error(err),
			zap.String("queue", c.config.Queue),
			zap.String("message_id", msg.ID))
		incrementConsumeErrors(c.config.Queue, "ack_error")
		return
	}

	incrementMessagesConsumed(c.config.Queue)
	observeProcessingTime(c.config.Queue, time.Since(startTime).Seconds())
}

// Close signals shutdown, waits for the in-flight delivery (if any) and the
// consume loop to finish, then closes the channel and connection.
func (c *Consumer) Close() error {
	c.closeOnce.Do(func() { close(c.done) })
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	var firstErr error
	if c.channel != nil {
		if err := c.channel.Close(); err != nil && err != amqp.ErrClosed {
			firstErr = err
		}
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil && err != amqp.ErrClosed && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
