package queue

import (
	"context"
	"encoding/json"
	"errors"
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

// Consumer connects to RabbitMQ, verifies the broker-owned topology
// (definitions.json, ADR-008.4) passively, and consumes a single queue with
// automatic reconnect on connection/channel loss. On a handler error the
// message is republished to the next retry wait-queue (ADR-008.1) or, when
// retries are exhausted or the error is unretryable, to the DLX — always
// followed by an ACK. The consumer never nack-requeues.
type Consumer struct {
	config *Config
	logger *Logger

	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel

	// Dedicated confirm-mode channel for retry/DLX republishing, kept separate
	// from the consume channel so publisher confirms and consumer acks never
	// interleave. pubConfirms is registered once per channel (in connect).
	pubChannel  *amqp.Channel
	pubConfirms chan amqp.Confirmation

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

// connect (re)establishes the connection/channels as needed and verifies the
// broker-owned topology passively (ADR-008.4): the exchanges and queues this
// consumer depends on must already exist, authored from definitions.json.
// Services never declare topology. A missing entity is a fatal
// misconfiguration (definitions.json not loaded) that no reconnect can fix, so
// it crashes with a pointed message; a transient connection loss returns an
// error and lets the reconnect loop retry.
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

	if err := c.verifyTopology(ch); err != nil {
		_ = ch.Close()
		var amqpErr *amqp.Error
		if errors.As(err, &amqpErr) && amqpErr.Code == amqp.NotFound {
			// The broker is reachable but the entity is absent: fail loud.
			c.logger.Fatal("broker topology missing — is definitions.json loaded?",
				zap.Error(err),
				zap.String("queue", c.config.Queue),
				zap.String("exchange", c.config.Exchange))
			// unreachable: Fatal exits the process.
		}
		// Not a missing-entity error (e.g. the channel dropped mid-verify):
		// let the reconnect loop handle it.
		return fmt.Errorf("verify topology: %w", err)
	}

	if err := ch.Qos(c.config.PrefetchCount, c.config.PrefetchSize, c.config.Global); err != nil {
		_ = ch.Close()
		return fmt.Errorf("set qos: %w", err)
	}

	// Dedicated confirm-mode channel for retry/DLX republishing. The confirm
	// listener is registered exactly once here (per channel), not per publish.
	pubCh, err := c.conn.Channel()
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("%w: %v", ErrChannelFailed, err)
	}
	if err := pubCh.Confirm(false); err != nil {
		_ = ch.Close()
		_ = pubCh.Close()
		return fmt.Errorf("enable publisher confirms: %w", err)
	}
	pubConfirms := pubCh.NotifyPublish(make(chan amqp.Confirmation, 1))

	if c.channel != nil {
		_ = c.channel.Close()
	}
	if c.pubChannel != nil {
		_ = c.pubChannel.Close()
	}
	c.channel = ch
	c.pubChannel = pubCh
	c.pubConfirms = pubConfirms

	return nil
}

// verifyTopology passively declares (existence-checks) every exchange and
// queue this consumer publishes to or consumes from. A passive declare against
// a missing entity raises a 404 NOT_FOUND channel exception, which connect()
// turns into a pointed crash. Bindings cannot be checked passively and are
// trusted to definitions.json.
func (c *Consumer) verifyTopology(ch *amqp.Channel) error {
	exchanges := []string{
		c.config.Exchange,                // main work exchange
		RetryExchange(c.config.Exchange), // <exchange>.retry
		DLXExchange(c.config.Exchange),   // <exchange>.dlx
		TaskResultsExchange,              // shared task-results (ADR-008.3)
	}
	for _, ex := range exchanges {
		if err := ch.ExchangeDeclarePassive(ex, "direct", c.config.Durable, c.config.AutoDelete, false, false, nil); err != nil {
			return fmt.Errorf("exchange %q: %w", ex, err)
		}
	}

	queues := []string{c.config.Queue, c.config.Queue + ".dlq"}
	for _, tier := range RetryTiers {
		queues = append(queues, c.config.Queue+".retry."+tier)
	}
	for _, q := range queues {
		if _, err := ch.QueueDeclarePassive(q, c.config.Durable, c.config.AutoDelete, c.config.Exclusive, false, nil); err != nil {
			return fmt.Errorf("queue %q: %w", q, err)
		}
	}
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
		// An unparseable envelope is unretryable poison: route it explicitly
		// to the DLQ (via the DLX) and ack. Never nack-requeue.
		c.logger.Error("failed to unmarshal message; routing to DLQ",
			zap.Error(err), zap.String("queue", c.config.Queue))
		incrementConsumeErrors(c.config.Queue, "unmarshal_error")
		c.deadLetterAndAck(delivery)
		return
	}

	// Attempt = how many retry cycles this envelope already went through
	// (x-death count for our retry wait-queues). Threaded onto the message so
	// BaseWorker can scope the idempotency key per attempt (retries must not be
	// deduped) and processors can drive attempt-aware test hooks off it.
	msg.Attempt = DeathCount(delivery.Headers, c.config.Queue)

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
			attribute.Int("messaging.rabbitmq.delivery.attempt", msg.Attempt),
		),
	)
	defer span.End()

	if err := handler(ctx, &msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "message processing failed")
		incrementConsumeErrors(c.config.Queue, "handler_error")
		// trace_id on the failure-path logs preserves the EXP-31 triage
		// pivot (DLQ panel -> log line -> every log line of the same trace)
		// across v4's retry/DLX routing. The span is always valid here
		// (extracted parent or a fresh root).
		c.routeFailure(delivery, &msg, err, span.SpanContext().TraceID().String())
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
	// Ack-worthy processing done → publish the terminal "completed" result
	// (ADR-008.3). A deduped delivery also lands here (handler returned nil):
	// re-emitting "completed" is correct (the task IS complete) and idempotent
	// at api-service, which dedupes results on envelope_id.
	c.emitResult(&msg, statusCompleted, "")
}

// routeFailure implements the ADR-008.1 handler-error path. A retryable error
// with a tier still available is republished to the next retry wait-queue (body
// unchanged, headers copied so x-death survives) then acked, with NO task.result
// (not a terminal outcome). Unretryable errors or exhausted tiers go to the DLQ
// via the DLX, are acked, and emit a "failed" task.result. It never
// nack-requeues on the normal path; only a publish-infra failure requeues (see
// requeueOnPublishFailure).
func (c *Consumer) routeFailure(delivery amqp.Delivery, msg *Message, err error, traceID string) {
	if classifyOutcome(err, msg.Attempt) == outcomeRetry {
		tier, _ := NextRetryTier(msg.Attempt)
		c.logger.Warn("retryable error; scheduling retry",
			zap.Error(err), zap.String("queue", c.config.Queue),
			zap.String("message_id", msg.ID), zap.Int("attempt", msg.Attempt), zap.String("tier", tier),
			zap.String("trace_id", traceID))

		exchange := RetryExchange(c.config.Exchange)
		rk := RetryRoutingKey(c.config.RoutingKey, tier)
		if perr := c.publishConfirmed(exchange, rk, copyHeaders(delivery.Headers), delivery.Body); perr != nil {
			c.requeueOnPublishFailure(delivery, "retry", perr)
			return
		}
		incrementRetries(c.config.WorkerType, tier)
		if aerr := delivery.Ack(false); aerr != nil {
			c.logger.Error("failed to ack after scheduling retry",
				zap.Error(aerr), zap.String("queue", c.config.Queue), zap.String("message_id", msg.ID))
			incrementConsumeErrors(c.config.Queue, "ack_error")
		}
		return
	}

	// Terminal: unretryable or exhausted → DLQ.
	if errors.Is(err, ErrUnretryable) {
		c.logger.Warn("unretryable error; routing to DLQ",
			zap.Error(err), zap.String("queue", c.config.Queue), zap.String("message_id", msg.ID),
			zap.String("trace_id", traceID))
	} else {
		c.logger.Warn("retries exhausted; routing to DLQ",
			zap.Error(err), zap.String("queue", c.config.Queue),
			zap.String("message_id", msg.ID), zap.Int("attempt", msg.Attempt),
			zap.String("trace_id", traceID))
	}
	if c.deadLetterAndAck(delivery) {
		// Only a genuine terminal dead-letter yields a "failed" result; a
		// requeued publish-infra failure will be retried, so no result yet.
		c.emitResult(msg, statusFailed, err.Error())
	}
}

// deadLetterAndAck publishes the delivery to the DLX (poison path) and acks,
// returning true when the message was terminally dead-lettered. On a publish
// failure it requeues (see requeueOnPublishFailure) and returns false — the
// message is not terminally failed, so the caller emits no task.result.
func (c *Consumer) deadLetterAndAck(delivery amqp.Delivery) bool {
	exchange := DLXExchange(c.config.Exchange)
	if err := c.publishConfirmed(exchange, c.config.RoutingKey, copyHeaders(delivery.Headers), delivery.Body); err != nil {
		c.requeueOnPublishFailure(delivery, "dlx", err)
		return false
	}
	incrementDLQ(c.config.WorkerType)
	if err := delivery.Ack(false); err != nil {
		c.logger.Error("failed to ack after dead-lettering",
			zap.Error(err), zap.String("queue", c.config.Queue))
		incrementConsumeErrors(c.config.Queue, "ack_error")
	}
	return true
}

// requeueOnPublishFailure handles a failed retry/DLX republish. A publish-infra
// failure is NOT poison, so we Nack WITH requeue: this preserves at-least-once
// delivery, the cadence is paced by the confirm timeout, and on broker recovery
// the redelivery re-enters routeFailure and routes correctly. We deliberately
// do NOT Nack(requeue=false) here — that would dead-letter via the queue's
// x-dead-letter-routing-key (for email-processing that is email.expired, which
// would masquerade as staleness expiry; other queues would DLQ a message that
// still had retry tiers left).
func (c *Consumer) requeueOnPublishFailure(delivery amqp.Delivery, kind string, cause error) {
	c.logger.Error("failed to publish "+kind+"; nacking WITH requeue (publish-infra failure, not poison)",
		zap.Error(cause), zap.String("queue", c.config.Queue))
	incrementConsumeErrors(c.config.Queue, kind+"_publish_error")
	if nerr := delivery.Nack(false, true); nerr != nil {
		c.logger.Error("fallback requeue nack failed", zap.Error(nerr))
	}
}

// publishConfirmed publishes body persistently on the dedicated confirm-mode
// channel and blocks for the broker confirmation. It is called only from the
// (single-goroutine) consume loop, so publishes and their confirmations stay
// strictly ordered on pubConfirms.
func (c *Consumer) publishConfirmed(exchange, routingKey string, headers amqp.Table, body []byte) error {
	c.mu.Lock()
	pubCh := c.pubChannel
	confirms := c.pubConfirms
	c.mu.Unlock()
	if pubCh == nil {
		return ErrChannelFailed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pubCh.PublishWithContext(ctx, exchange, routingKey,
		false, // mandatory: broker-owned bindings verified at connect
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Headers:      headers,
			Body:         body,
		},
	); err != nil {
		return fmt.Errorf("%w: %v", ErrPublishFailed, err)
	}

	select {
	case confirm, ok := <-confirms:
		if !ok {
			return ErrConnectionClosed
		}
		if !confirm.Ack {
			return ErrPublishFailed
		}
		return nil
	case <-time.After(5 * time.Second):
		return ErrPublishTimeout
	}
}

// copyHeaders clones a delivery's headers so the x-death chain (and any trace
// headers) survives a republish without aliasing the original table.
func copyHeaders(h amqp.Table) amqp.Table {
	out := make(amqp.Table, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out
}

// Close signals shutdown, waits for the in-flight delivery (if any) and the
// consume loop to finish, then closes the channel and connection.
func (c *Consumer) Close() error {
	c.closeOnce.Do(func() { close(c.done) })
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	var firstErr error
	if c.pubChannel != nil {
		if err := c.pubChannel.Close(); err != nil && err != amqp.ErrClosed {
			firstErr = err
		}
	}
	if c.channel != nil {
		if err := c.channel.Close(); err != nil && err != amqp.ErrClosed && firstErr == nil {
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
