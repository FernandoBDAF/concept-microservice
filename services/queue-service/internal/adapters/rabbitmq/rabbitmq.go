package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/FBDAF/microservices/services/queue-service/internal/domain/model"
	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQ represents a RabbitMQ connection and channel following best practices
type RabbitMQ struct {
	conn           *amqp.Connection
	channel        *amqp.Channel
	config         *Config
	confirms       chan amqp.Confirmation
	confirmTracker map[uint64]chan bool
	trackerMu      sync.RWMutex
	connected      bool
	mu             sync.RWMutex
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
	MessageTTL       time.Duration
	ConfirmTimeout   time.Duration
}

// New creates a new RabbitMQ instance with best practices
func New(config *Config) (*RabbitMQ, error) {
	// Set default confirm timeout if not specified
	if config.ConfirmTimeout == 0 {
		config.ConfirmTimeout = 5 * time.Second
	}

	rmq := &RabbitMQ{
		config:         config,
		confirmTracker: make(map[uint64]chan bool),
	}

	if err := rmq.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	return rmq, nil
}

// connect establishes a connection to RabbitMQ following best practices
func (r *RabbitMQ) connect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

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

		log.Printf("Attempting to connect to RabbitMQ at %s", host)
		conn, err = amqp.Dial(url)
		if err == nil {
			log.Printf("Successfully connected to RabbitMQ at %s", host)
			break
		}
		log.Printf("Failed to connect to RabbitMQ at %s: %v", host, err)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to any RabbitMQ host: %w", err)
	}

	r.conn = conn

	// Create single channel for publishing (best practice)
	log.Printf("Creating RabbitMQ channel")
	ch, err := r.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}

	// Enable publisher confirms for reliability
	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("failed to enable publisher confirms: %w", err)
	}
	log.Printf("Publisher confirms enabled")

	// Set QoS
	if err := ch.Qos(r.config.PrefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	r.channel = ch
	r.connected = true

	// Setup publisher confirm handling
	r.confirms = make(chan amqp.Confirmation, 100)
	ch.NotifyPublish(r.confirms)

	// Start confirm handler
	go r.handleConfirms()

	// Simple connection monitoring (no complex goroutines)
	go r.monitorConnection()

	log.Printf("Successfully connected to RabbitMQ with publisher confirms")
	return nil
}

// handleConfirms processes publisher confirmations
func (r *RabbitMQ) handleConfirms() {
	for confirm := range r.confirms {
		r.trackerMu.RLock()
		if ch, exists := r.confirmTracker[confirm.DeliveryTag]; exists {
			ch <- confirm.Ack
			close(ch)
			delete(r.confirmTracker, confirm.DeliveryTag)
		}
		r.trackerMu.RUnlock()
	}
}

// monitorConnection monitors connection state with simple reconnection
func (r *RabbitMQ) monitorConnection() {
	closed := r.conn.NotifyClose(make(chan *amqp.Error))

	for err := range closed {
		if err != nil {
			log.Printf("RabbitMQ connection closed: %v", err)
			r.mu.Lock()
			r.connected = false
			r.mu.Unlock()

			// Simple reconnection logic
			for {
				log.Printf("Attempting to reconnect to RabbitMQ")
				if err := r.connect(); err == nil {
					log.Printf("Successfully reconnected to RabbitMQ")
					break
				}
				log.Printf("Failed to reconnect, retrying in %v", r.config.ReconnectTimeout)
				time.Sleep(r.config.ReconnectTimeout)
			}
		}
	}
}

// ensureTopology sets up exchanges and queues for routing key with worker-specific configuration
func (r *RabbitMQ) ensureTopology(routingKey string) error {
	config, exists := model.DefaultRoutingMap[routingKey]
	if !exists {
		// Use default configuration for unknown routing keys
		config = model.RoutingConfig{
			Exchange:      "tasks-exchange",
			Queue:         "default-processing",
			TTL:           24 * time.Hour,
			Prefetch:      1,
			Durable:       true,
			AutoDelete:    false,
			Exclusive:     false,
			NoWait:        false,
			DeadLetterTTL: 7 * 24 * time.Hour,
			MaxRetries:    3,
			Description:   "Default configuration for unknown routing keys",
		}
	}

	// Declare main exchange with worker-specific properties
	err := r.channel.ExchangeDeclare(
		config.Exchange,   // name
		"direct",          // type
		config.Durable,    // durable
		config.AutoDelete, // auto-deleted
		false,             // internal
		config.NoWait,     // no-wait
		nil,               // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", config.Exchange, err)
	}

	// Setup dead letter exchange with same durability as main exchange
	dlxName := config.Exchange + ".dlx"
	err = r.channel.ExchangeDeclare(
		dlxName,           // name
		"direct",          // type
		config.Durable,    // durable
		config.AutoDelete, // auto-deleted
		false,             // internal
		config.NoWait,     // no-wait
		nil,               // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare dead letter exchange %s: %w", dlxName, err)
	}

	// Declare main queue with worker-specific configuration and dead letter exchange
	queueArgs := amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": routingKey,
		"x-message-ttl":             int32(config.TTL.Milliseconds()),
		"x-max-retries":             config.MaxRetries,
	}

	_, err = r.channel.QueueDeclare(
		config.Queue,      // name
		config.Durable,    // durable
		config.AutoDelete, // delete when unused
		config.Exclusive,  // exclusive
		config.NoWait,     // no-wait
		queueArgs,         // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", config.Queue, err)
	}

	// Bind main queue to exchange with routing key
	err = r.channel.QueueBind(
		config.Queue,    // queue name
		routingKey,      // routing key
		config.Exchange, // exchange
		config.NoWait,   // no-wait
		nil,             // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue %s to exchange %s: %w", config.Queue, config.Exchange, err)
	}

	// Declare dead letter queue with worker-specific TTL
	dlqName := config.Queue + ".dlq"
	dlqArgs := amqp.Table{
		"x-message-ttl": int32(config.DeadLetterTTL.Milliseconds()),
	}

	_, err = r.channel.QueueDeclare(
		dlqName,           // name
		config.Durable,    // durable
		config.AutoDelete, // delete when unused
		config.Exclusive,  // exclusive
		config.NoWait,     // no-wait
		dlqArgs,           // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare dead letter queue %s: %w", dlqName, err)
	}

	// Bind dead letter queue to dead letter exchange
	err = r.channel.QueueBind(
		dlqName,       // queue name
		routingKey,    // routing key
		dlxName,       // exchange
		config.NoWait, // no-wait
		nil,           // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind dead letter queue %s: %w", dlqName, err)
	}

	log.Printf("Successfully configured topology for routing key %s: exchange=%s, queue=%s, TTL=%v, prefetch=%d",
		routingKey, config.Exchange, config.Queue, config.TTL, config.Prefetch)

	return nil
}

// PublishWithRoutingKey publishes a message with routing key support
func (r *RabbitMQ) PublishWithRoutingKey(routingKey string, msg *model.Message) error {
	r.mu.RLock()
	if !r.connected {
		r.mu.RUnlock()
		return fmt.Errorf("not connected to RabbitMQ")
	}
	r.mu.RUnlock()

	// Ensure topology is set up for this routing key
	if err := r.ensureTopology(routingKey); err != nil {
		return fmt.Errorf("failed to ensure topology: %w", err)
	}

	// Get exchange name for routing key
	config := model.DefaultRoutingMap[routingKey]
	if config.Exchange == "" {
		config.Exchange = "tasks-exchange" // default
	}

	// Marshal message to JSON
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Convert metadata to AMQP table
	headers := make(amqp.Table)
	for k, v := range msg.Metadata {
		headers[k] = v
	}

	// Get next delivery tag for confirmation tracking
	r.trackerMu.Lock()
	deliveryTag := r.channel.GetNextPublishSeqNo()
	confirmCh := make(chan bool, 1)
	r.confirmTracker[deliveryTag] = confirmCh
	r.trackerMu.Unlock()

	// Publish message
	err = r.channel.Publish(
		config.Exchange, // exchange
		routingKey,      // routing key
		true,            // mandatory
		false,           // immediate
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
		// Cleanup confirm tracker on publish error
		r.trackerMu.Lock()
		delete(r.confirmTracker, deliveryTag)
		r.trackerMu.Unlock()
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// Wait for publisher confirm with timeout
	ctx, cancel := context.WithTimeout(context.Background(), r.config.ConfirmTimeout)
	defer cancel()

	select {
	case ack := <-confirmCh:
		if !ack {
			return fmt.Errorf("message was not acknowledged by broker")
		}
		log.Printf("Message published successfully with routing key: %s", routingKey)
		return nil
	case <-ctx.Done():
		// Cleanup confirm tracker on timeout
		r.trackerMu.Lock()
		delete(r.confirmTracker, deliveryTag)
		r.trackerMu.Unlock()
		return fmt.Errorf("publisher confirm timeout after %v", r.config.ConfirmTimeout)
	}
}

// Publish publishes a message (backward compatibility)
func (r *RabbitMQ) Publish(queueName string, msg *model.Message) error {
	// Map queue name to routing key for backward compatibility
	routingKey := "profile.task" // default routing key

	// Try to infer routing key from message type or queue name
	switch msg.Type {
	case "profile_update", "profile_task":
		routingKey = "profile.task"
	case "email_send", "email_task":
		routingKey = "email.send"
	case "image_process", "image_task":
		routingKey = "image.process"
	}

	return r.PublishWithRoutingKey(routingKey, msg)
}

// Consume starts consuming messages from a queue (simplified)
func (r *RabbitMQ) Consume(queueName string, handler func(*model.Message) error) error {
	// For backward compatibility, use default routing key
	routingKey := "profile.task"

	if err := r.ensureTopology(routingKey); err != nil {
		return fmt.Errorf("failed to ensure topology: %w", err)
	}

	config := model.DefaultRoutingMap[routingKey]

	msgs, err := r.channel.Consume(
		config.Queue, // queue
		"",           // consumer
		false,        // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	go func() {
		for d := range msgs {
			msg := &model.Message{}
			if err := json.Unmarshal(d.Body, msg); err != nil {
				log.Printf("Failed to unmarshal message: %v", err)
				d.Reject(false)
				continue
			}

			if err := handler(msg); err != nil {
				log.Printf("Failed to handle message: %v", err)
				d.Reject(false)
				continue
			}

			d.Ack(false)
		}
	}()

	return nil
}

// Close closes the RabbitMQ connection
func (r *RabbitMQ) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.connected = false

	if r.confirms != nil {
		close(r.confirms)
	}

	if r.channel != nil {
		if err := r.channel.Close(); err != nil {
			log.Printf("Failed to close channel: %v", err)
		}
	}

	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}

	return nil
}
