package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// ConsumerConfig holds configuration for the RabbitMQ consumer
type ConsumerConfig struct {
	ConnectionURL   string
	QueueName       string
	ExchangeName    string
	RoutingKey      string
	ConsumerTag     string
	PrefetchCount   int
	ReconnectDelay  time.Duration
	ProcessTimeout  time.Duration
	DLQEnabled      bool
	DLQExchangeName string
	DLQQueueName    string
	MaxRetries      int
}

// Consumer represents a RabbitMQ consumer
type Consumer struct {
	config    *ConsumerConfig
	processor *MessageProcessor
	conn      *amqp.Connection
	channel   *amqp.Channel
	delivery  <-chan amqp.Delivery
	done      chan bool
	log       *zap.Logger
	mu        sync.RWMutex
	connected bool
	reconnect chan bool
}

// NewConsumer creates a new RabbitMQ consumer
func NewConsumer(config *ConsumerConfig, processor *MessageProcessor) *Consumer {
	return &Consumer{
		config:    config,
		processor: processor,
		done:      make(chan bool),
		log:       zap.L().Named("rabbitmq_consumer"),
		reconnect: make(chan bool, 1),
	}
}

// Start starts the consumer and begins processing messages
func (c *Consumer) Start(ctx context.Context) error {
	c.log.Info("Starting RabbitMQ consumer",
		zap.String("queue", c.config.QueueName),
		zap.String("exchange", c.config.ExchangeName),
		zap.String("routing_key", c.config.RoutingKey))

	// Initial connection
	if err := c.connect(); err != nil {
		return fmt.Errorf("failed to establish initial connection: %w", err)
	}

	// Start message processing loop
	go c.processMessages(ctx)

	// Start reconnection handler
	go c.handleReconnection(ctx)

	return nil
}

// Stop stops the consumer gracefully
func (c *Consumer) Stop() error {
	c.log.Info("Stopping RabbitMQ consumer")

	select {
	case c.done <- true:
	default:
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			c.log.Error("Error closing channel", zap.Error(err))
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.log.Error("Error closing connection", zap.Error(err))
		}
	}

	c.connected = false
	c.log.Info("RabbitMQ consumer stopped")
	return nil
}

// IsConnected returns the connection status
func (c *Consumer) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.conn != nil && !c.conn.IsClosed()
}

// connect establishes connection to RabbitMQ
func (c *Consumer) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error

	// Close existing connections
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}

	// Establish connection
	c.conn, err = amqp.Dial(c.config.ConnectionURL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create channel
	c.channel, err = c.conn.Channel()
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	// Set prefetch count
	if err := c.channel.Qos(c.config.PrefetchCount, 0, false); err != nil {
		c.channel.Close()
		c.conn.Close()
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Declare exchange
	if err := c.channel.ExchangeDeclare(
		c.config.ExchangeName,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	); err != nil {
		c.channel.Close()
		c.conn.Close()
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare main queue
	queue, err := c.channel.QueueDeclare(
		c.config.QueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		c.getQueueArgs(),
	)
	if err != nil {
		c.channel.Close()
		c.conn.Close()
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange
	if err := c.channel.QueueBind(
		queue.Name,
		c.config.RoutingKey,
		c.config.ExchangeName,
		false,
		nil,
	); err != nil {
		c.channel.Close()
		c.conn.Close()
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	// Setup DLQ if enabled
	if c.config.DLQEnabled {
		if err := c.setupDLQ(); err != nil {
			c.channel.Close()
			c.conn.Close()
			return fmt.Errorf("failed to setup DLQ: %w", err)
		}
	}

	// Start consuming
	c.delivery, err = c.channel.Consume(
		queue.Name,
		c.config.ConsumerTag,
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		c.channel.Close()
		c.conn.Close()
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	c.connected = true

	// Monitor connection
	go c.monitorConnection()

	c.log.Info("Connected to RabbitMQ",
		zap.String("queue", c.config.QueueName),
		zap.String("exchange", c.config.ExchangeName))

	return nil
}

// processMessages processes incoming messages from RabbitMQ
func (c *Consumer) processMessages(ctx context.Context) {
	for {
		select {
		case delivery, ok := <-c.delivery:
			if !ok {
				c.log.Warn("Delivery channel closed")
				c.triggerReconnect()
				return
			}
			c.handleDelivery(ctx, delivery)

		case <-c.done:
			c.log.Info("Message processing stopped")
			return

		case <-ctx.Done():
			c.log.Info("Context cancelled, stopping message processing")
			return
		}
	}
}

// handleDelivery handles a single message delivery
func (c *Consumer) handleDelivery(ctx context.Context, delivery amqp.Delivery) {
	startTime := time.Now()

	// Parse message
	var msg Message
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		c.log.Error("Failed to unmarshal message",
			zap.Error(err),
			zap.String("body", string(delivery.Body)))
		c.nackMessage(delivery, false) // Don't requeue invalid messages
		return
	}

	// Create processing context with timeout
	processCtx, cancel := context.WithTimeout(ctx, c.config.ProcessTimeout)
	defer cancel()

	// Process message
	_, err := c.processor.ProcessMessage(processCtx, &msg)
	processingTime := time.Since(startTime)

	if err != nil {
		c.log.Error("Message processing failed",
			zap.String("message_id", msg.ID),
			zap.String("routing_key", msg.RoutingKey),
			zap.Error(err),
			zap.Duration("processing_time", processingTime))

		// Handle retry logic
		if msg.ShouldRetry() {
			c.log.Info("Requeuing message for retry",
				zap.String("message_id", msg.ID),
				zap.Int("retry_count", msg.RetryCount))
			msg.IncrementRetry()
			c.nackMessage(delivery, true) // Requeue for retry
		} else {
			c.log.Error("Max retries exceeded, sending to DLQ",
				zap.String("message_id", msg.ID))
			c.sendToDLQ(delivery, &msg, err)
			c.ackMessage(delivery)
		}
		return
	}

	// Successfully processed
	c.log.Info("Message processed successfully",
		zap.String("message_id", msg.ID),
		zap.String("routing_key", msg.RoutingKey),
		zap.Duration("processing_time", processingTime))

	c.ackMessage(delivery)
}

// ackMessage acknowledges a message
func (c *Consumer) ackMessage(delivery amqp.Delivery) {
	if err := delivery.Ack(false); err != nil {
		c.log.Error("Failed to acknowledge message", zap.Error(err))
	}
}

// nackMessage negatively acknowledges a message
func (c *Consumer) nackMessage(delivery amqp.Delivery, requeue bool) {
	if err := delivery.Nack(false, requeue); err != nil {
		c.log.Error("Failed to nack message", zap.Error(err))
	}
}

// sendToDLQ sends a message to the Dead Letter Queue
func (c *Consumer) sendToDLQ(delivery amqp.Delivery, msg *Message, processingErr error) {
	if !c.config.DLQEnabled {
		return
	}

	// Create DLQ message with error information
	dlqMessage := map[string]interface{}{
		"original_message":     msg,
		"error":                processingErr.Error(),
		"failed_at":            time.Now(),
		"original_routing_key": delivery.RoutingKey,
		"retry_count":          msg.RetryCount,
	}

	dlqBody, err := json.Marshal(dlqMessage)
	if err != nil {
		c.log.Error("Failed to marshal DLQ message", zap.Error(err))
		return
	}

	c.mu.RLock()
	channel := c.channel
	c.mu.RUnlock()

	if channel == nil {
		c.log.Error("No channel available for DLQ")
		return
	}

	if err := channel.Publish(
		c.config.DLQExchangeName,
		c.config.DLQQueueName,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        dlqBody,
			Timestamp:   time.Now(),
		},
	); err != nil {
		c.log.Error("Failed to publish to DLQ", zap.Error(err))
	} else {
		c.log.Info("Message sent to DLQ",
			zap.String("message_id", msg.ID),
			zap.String("dlq_queue", c.config.DLQQueueName))
	}
}

// setupDLQ sets up the Dead Letter Queue
func (c *Consumer) setupDLQ() error {
	// Declare DLQ exchange
	if err := c.channel.ExchangeDeclare(
		c.config.DLQExchangeName,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	); err != nil {
		return fmt.Errorf("failed to declare DLQ exchange: %w", err)
	}

	// Declare DLQ queue
	_, err := c.channel.QueueDeclare(
		c.config.DLQQueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLQ queue: %w", err)
	}

	// Bind DLQ queue to exchange
	if err := c.channel.QueueBind(
		c.config.DLQQueueName,
		c.config.DLQQueueName,
		c.config.DLQExchangeName,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind DLQ queue: %w", err)
	}

	return nil
}

// getQueueArgs returns arguments for queue declaration
func (c *Consumer) getQueueArgs() amqp.Table {
	args := amqp.Table{}

	if c.config.DLQEnabled {
		args["x-dead-letter-exchange"] = c.config.DLQExchangeName
		args["x-dead-letter-routing-key"] = c.config.DLQQueueName
	}

	return args
}

// monitorConnection monitors the connection and triggers reconnection if needed
func (c *Consumer) monitorConnection() {
	connectionClosed := c.conn.NotifyClose(make(chan *amqp.Error))
	channelClosed := c.channel.NotifyClose(make(chan *amqp.Error))

	select {
	case err := <-connectionClosed:
		c.log.Error("Connection closed", zap.Error(err))
		c.connected = false
		c.triggerReconnect()

	case err := <-channelClosed:
		c.log.Error("Channel closed", zap.Error(err))
		c.connected = false
		c.triggerReconnect()
	}
}

// triggerReconnect triggers a reconnection attempt
func (c *Consumer) triggerReconnect() {
	select {
	case c.reconnect <- true:
	default:
	}
}

// handleReconnection handles reconnection attempts
func (c *Consumer) handleReconnection(ctx context.Context) {
	for {
		select {
		case <-c.reconnect:
			c.log.Info("Attempting to reconnect to RabbitMQ")
			for {
				if err := c.connect(); err != nil {
					c.log.Error("Reconnection failed, retrying",
						zap.Error(err),
						zap.Duration("delay", c.config.ReconnectDelay))
					time.Sleep(c.config.ReconnectDelay)
					continue
				}
				c.log.Info("Successfully reconnected to RabbitMQ")
				go c.processMessages(ctx)
				break
			}

		case <-c.done:
			return

		case <-ctx.Done():
			return
		}
	}
}

// GetStats returns consumer statistics
func (c *Consumer) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"connected":      c.connected,
		"queue_name":     c.config.QueueName,
		"exchange_name":  c.config.ExchangeName,
		"routing_key":    c.config.RoutingKey,
		"prefetch_count": c.config.PrefetchCount,
		"dlq_enabled":    c.config.DLQEnabled,
		"connection_url": c.maskConnectionURL(),
	}
}

// maskConnectionURL masks sensitive information in connection URL
func (c *Consumer) maskConnectionURL() string {
	// Simple masking - in production, use proper URL parsing
	return "amqp://***:***@***:***/**"
}
