package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQClient handles communication with RabbitMQ
type RabbitMQClient struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *RabbitMQConfig
}

// RabbitMQConfig holds the configuration for the RabbitMQ client
type RabbitMQConfig struct {
	URL            string
	QueueName      string
	PrefetchCount  int
	ReconnectDelay time.Duration
	MaxRetries     int
}

// QueueMessage represents a message from the queue
type QueueMessage struct {
	Type        string                 `json:"type"`
	ID          string                 `json:"id"`
	Data        map[string]interface{} `json:"data"`
	Timestamp   string                 `json:"timestamp"`
	Headers     map[string]interface{} `json:"headers"`
	deliveryTag uint64                 // Internal field for message acknowledgment
}

// NewRabbitMQClient creates a new RabbitMQ client instance
func NewRabbitMQClient(config *RabbitMQConfig) (*RabbitMQClient, error) {
	if config == nil {
		return nil, fmt.Errorf("rabbitmq config cannot be nil")
	}

	client := &RabbitMQClient{
		config: config,
	}

	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	return client, nil
}

// connect establishes a connection to RabbitMQ
func (c *RabbitMQClient) connect() error {
	var err error
	c.conn, err = amqp.Dial(c.config.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

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

	return nil
}

// setupQueue sets up the queue and its dead letter queue
func (c *RabbitMQClient) setupQueue() error {
	// Declare dead letter exchange
	err := c.channel.ExchangeDeclare(
		c.config.QueueName+".dlx", // name
		"direct",                  // type
		true,                      // durable
		false,                     // auto-deleted
		false,                     // internal
		false,                     // no-wait
		nil,                       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare dead letter exchange: %w", err)
	}

	// Declare dead letter queue
	_, err = c.channel.QueueDeclare(
		c.config.QueueName+".dlq", // name
		true,                      // durable
		false,                     // delete when unused
		false,                     // exclusive
		false,                     // no-wait
		amqp.Table{
			"x-message-ttl": int32(24 * time.Hour.Milliseconds()), // 24h TTL
		}, // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare dead letter queue: %w", err)
	}

	// Bind dead letter queue to exchange
	err = c.channel.QueueBind(
		c.config.QueueName+".dlq", // queue name
		c.config.QueueName,        // routing key
		c.config.QueueName+".dlx", // exchange
		false,                     // no-wait
		nil,                       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind dead letter queue: %w", err)
	}

	// Declare main queue with dead letter exchange
	_, err = c.channel.QueueDeclare(
		c.config.QueueName, // name
		true,               // durable
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    c.config.QueueName + ".dlx",
			"x-dead-letter-routing-key": c.config.QueueName,
			"x-message-ttl":             int32(24 * time.Hour.Milliseconds()), // 24h TTL
		}, // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	return nil
}

// ConsumeMessages starts consuming messages from RabbitMQ
func (c *RabbitMQClient) ConsumeMessages(ctx context.Context) (<-chan QueueMessage, error) {
	// Set up queue and dead letter queue
	if err := c.setupQueue(); err != nil {
		return nil, fmt.Errorf("failed to set up queue: %w", err)
	}

	msgs, err := c.channel.Consume(
		c.config.QueueName, // queue
		"",                 // consumer
		false,              // auto-ack
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register a consumer: %w", err)
	}

	processedMsgs := make(chan QueueMessage)

	go func() {
		defer close(processedMsgs)

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					// Channel closed, try to reconnect
					if err := c.reconnect(); err != nil {
						time.Sleep(c.config.ReconnectDelay)
						continue
					}
					msgs, err = c.channel.Consume(
						c.config.QueueName,
						"",
						false,
						false,
						false,
						false,
						nil,
					)
					if err != nil {
						time.Sleep(c.config.ReconnectDelay)
						continue
					}
					continue
				}

				var queueMsg QueueMessage
				if err := json.Unmarshal(msg.Body, &queueMsg); err != nil {
					msg.Nack(false, false)
					continue
				}

				// Store the delivery tag for acknowledgment
				queueMsg.deliveryTag = msg.DeliveryTag

				// Copy headers from AMQP message
				queueMsg.Headers = make(map[string]interface{})
				for k, v := range msg.Headers {
					queueMsg.Headers[k] = v
				}

				select {
				case <-ctx.Done():
					return
				case processedMsgs <- queueMsg:
				}
			}
		}
	}()

	return processedMsgs, nil
}

// AcknowledgeMessage acknowledges a processed message
func (c *RabbitMQClient) AcknowledgeMessage(ctx context.Context, msg QueueMessage) error {
	if msg.deliveryTag == 0 {
		return fmt.Errorf("invalid delivery tag")
	}
	return c.channel.Ack(msg.deliveryTag, false)
}

// RejectMessage rejects a message and optionally requeues it
func (c *RabbitMQClient) RejectMessage(ctx context.Context, msg QueueMessage, requeue bool) error {
	if msg.deliveryTag == 0 {
		return fmt.Errorf("invalid delivery tag")
	}
	return c.channel.Nack(msg.deliveryTag, false, requeue)
}

// reconnect attempts to reconnect to RabbitMQ
func (c *RabbitMQClient) reconnect() error {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}

	return c.connect()
}

// Close closes the RabbitMQ client
func (c *RabbitMQClient) Close() error {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
	return nil
}
