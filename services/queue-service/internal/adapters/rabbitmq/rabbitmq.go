package rabbitmq

import (
	"fmt"
	"time"

	"github.com/FBDAF/microservices/services/queue-service/internal/domain/model"
	"github.com/streadway/amqp"
)

// RabbitMQ represents a RabbitMQ connection and channel
type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *Config
}

// Config holds the RabbitMQ configuration
type Config struct {
	Hosts            []string
	Username         string
	Password         string
	VHost            string
	PrefetchCount    int
	ReconnectTimeout time.Duration
	MaxRetries       int
}

// convertHeaders converts map[string]string to amqp.Table
func convertHeaders(headers map[string]string) amqp.Table {
	if headers == nil {
		return nil
	}

	table := make(amqp.Table)
	for k, v := range headers {
		table[k] = v
	}
	return table
}

// New creates a new RabbitMQ instance
func New(config *Config) (*RabbitMQ, error) {
	rmq := &RabbitMQ{
		config: config,
	}

	if err := rmq.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	return rmq, nil
}

// connect establishes a connection to RabbitMQ
func (r *RabbitMQ) connect() error {
	var err error
	var conn *amqp.Connection

	// Try to connect to each host in the cluster
	for _, host := range r.config.Hosts {
		url := fmt.Sprintf("amqp://%s:%s@%s/%s",
			r.config.Username,
			r.config.Password,
			host,
			r.config.VHost,
		)

		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
	}

	if err != nil {
		return fmt.Errorf("failed to connect to any RabbitMQ host: %w", err)
	}

	r.conn = conn

	// Create channel
	ch, err := r.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}

	// Set prefetch count
	if err := ch.Qos(
		r.config.PrefetchCount, // prefetch count
		0,                      // prefetch size
		false,                  // global
	); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	r.channel = ch
	return nil
}

// Publish publishes a message to a queue
func (r *RabbitMQ) Publish(queueName string, msg *model.Message) error {
	// Declare queue
	_, err := r.channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Convert message to bytes
	body, err := msg.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Convert headers to amqp.Table
	headers := convertHeaders(msg.Headers)

	// Publish message
	err = r.channel.Publish(
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			Headers:         headers,
			ContentType:     "application/json",
			ContentEncoding: "utf-8",
			Body:            body,
			DeliveryMode:    amqp.Persistent,
			Priority:        uint8(msg.Priority),
			Timestamp:       msg.Timestamp,
			MessageId:       msg.ID,
			CorrelationId:   msg.CorrelationID,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Consume starts consuming messages from a queue
func (r *RabbitMQ) Consume(queueName string, handler func(*model.Message) error) error {
	// Declare queue
	_, err := r.channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Start consuming
	msgs, err := r.channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	// Process messages
	go func() {
		for d := range msgs {
			msg := &model.Message{}
			if err := msg.UnmarshalJSON(d.Body); err != nil {
				// Log error and reject message
				d.Reject(false)
				continue
			}

			if err := handler(msg); err != nil {
				// Log error and reject message
				d.Reject(false)
				continue
			}

			// Acknowledge message
			d.Ack(false)
		}
	}()

	return nil
}

// Close closes the RabbitMQ connection
func (r *RabbitMQ) Close() error {
	if r.channel != nil {
		if err := r.channel.Close(); err != nil {
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}

	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}

	return nil
}
