package rabbitmq

import (
	"fmt"
	"log"
	"time"

	"github.com/FBDAF/microservices/services/queue-service/internal/domain/model"
	amqp "github.com/rabbitmq/amqp091-go"
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
	MessageTTL       time.Duration
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

	// Create channel
	log.Printf("Creating RabbitMQ channel")
	ch, err := r.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}
	log.Printf("Successfully created RabbitMQ channel")

	// Set prefetch count
	if err := ch.Qos(
		r.config.PrefetchCount, // prefetch count
		0,                      // prefetch size
		false,                  // global
	); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	r.channel = ch

	// Monitor connection state
	go func() {
		notifyClose := r.conn.NotifyClose(make(chan *amqp.Error))
		for {
			select {
			case err := <-notifyClose:
				if err != nil {
					log.Printf("RabbitMQ connection closed: %v", err)
					// Connection closed, try to reconnect
					for {
						log.Printf("Attempting to reconnect to RabbitMQ")
						if err := r.connect(); err == nil {
							log.Printf("Successfully reconnected to RabbitMQ")
							break
						}
						log.Printf("Failed to reconnect to RabbitMQ: %v", err)
						time.Sleep(r.config.ReconnectTimeout)
					}
				}
			}
		}
	}()

	// Monitor channel state
	go func() {
		notifyClose := r.channel.NotifyClose(make(chan *amqp.Error))
		for {
			select {
			case err := <-notifyClose:
				if err != nil {
					log.Printf("RabbitMQ channel closed: %v", err)
					// Channel closed, try to recreate
					for {
						log.Printf("Attempting to recreate RabbitMQ channel")
						if err := r.connect(); err == nil {
							log.Printf("Successfully recreated RabbitMQ channel")
							break
						}
						log.Printf("Failed to recreate RabbitMQ channel: %v", err)
						time.Sleep(r.config.ReconnectTimeout)
					}
				}
			}
		}
	}()

	return nil
}

// ensureChannelOpen ensures the channel is open and reconnects if necessary
func (r *RabbitMQ) ensureChannelOpen() error {
	if r.channel == nil {
		if err := r.connect(); err != nil {
			return fmt.Errorf("failed to reconnect: %w", err)
		}
	}

	// Check if channel is closed
	select {
	case <-r.channel.NotifyClose(make(chan *amqp.Error)):
		// Channel is closed, try to reconnect
		if err := r.connect(); err != nil {
			return fmt.Errorf("failed to reconnect: %w", err)
		}
	default:
		// Channel is open
	}

	return nil
}

// setupDeadLetterQueue sets up the dead letter queue and exchange
func (r *RabbitMQ) setupDeadLetterQueue(queueName string) error {
	// Declare dead letter exchange
	err := r.channel.ExchangeDeclare(
		queueName+".dlx", // name
		"direct",         // type
		true,             // durable
		false,            // auto-deleted
		false,            // internal
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare dead letter exchange: %w", err)
	}

	// Declare dead letter queue
	_, err = r.channel.QueueDeclare(
		queueName+".dlq", // name
		true,             // durable
		false,            // delete when unused
		false,            // exclusive
		false,            // no-wait
		amqp.Table{
			"x-message-ttl": int32(r.config.MessageTTL.Milliseconds()),
		}, // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare dead letter queue: %w", err)
	}

	// Bind dead letter queue to exchange
	err = r.channel.QueueBind(
		queueName+".dlq", // queue name
		queueName,        // routing key
		queueName+".dlx", // exchange
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind dead letter queue: %w", err)
	}

	return nil
}

// ensureQueueSetup ensures the queue and exchange are set up
func (r *RabbitMQ) ensureQueueSetup(queueName string) error {
	// Ensure channel is open
	if err := r.ensureChannelOpen(); err != nil {
		return fmt.Errorf("failed to ensure channel is open: %w", err)
	}

	// Set up dead letter queue
	if err := r.setupDeadLetterQueue(queueName); err != nil {
		return fmt.Errorf("failed to set up dead letter queue: %w", err)
	}

	// Declare main exchange
	err := r.channel.ExchangeDeclare(
		queueName+".exchange", // name
		"direct",              // type
		true,                  // durable
		false,                 // auto-deleted
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare queue with dead letter exchange
	_, err = r.channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    queueName + ".dlx",
			"x-dead-letter-routing-key": queueName,
			"x-message-ttl":             int32(r.config.MessageTTL.Milliseconds()),
		}, // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange
	err = r.channel.QueueBind(
		queueName,             // queue name
		queueName,             // routing key
		queueName+".exchange", // exchange
		false,                 // no-wait
		nil,                   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue to exchange: %w", err)
	}

	return nil
}

// Publish publishes a message to a queue
func (r *RabbitMQ) Publish(queueName string, msg *model.Message) error {
	log.Printf("Starting to publish message to queue: %s", queueName)

	// Ensure queue and exchange are set up
	if err := r.ensureQueueSetup(queueName); err != nil {
		log.Printf("Failed to ensure queue setup: %v", err)
		return fmt.Errorf("failed to ensure queue setup: %w", err)
	}
	log.Printf("Queue setup ensured for: %s", queueName)

	// Ensure channel is open
	if err := r.ensureChannelOpen(); err != nil {
		log.Printf("Failed to ensure channel is open: %v", err)
		return fmt.Errorf("failed to ensure channel is open: %w", err)
	}
	log.Printf("Channel is open and ready")

	// Convert message to bytes
	body, err := msg.MarshalJSON()
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	log.Printf("Message marshaled successfully, size: %d bytes", len(body))

	// Convert headers to amqp.Table
	headers := convertHeaders(msg.Headers)
	log.Printf("Message headers: %v", headers)

	// Convert expiration to milliseconds string
	expiration := fmt.Sprintf("%d", r.config.MessageTTL.Milliseconds())
	log.Printf("Message expiration: %s", expiration)

	// Publish message
	exchangeName := queueName + ".exchange"
	log.Printf("Attempting to publish message to exchange: %s with routing key: %s", exchangeName, queueName)

	// Verify exchange exists
	err = r.channel.ExchangeDeclarePassive(
		exchangeName, // name
		"direct",     // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Printf("Exchange %s does not exist or is not accessible: %v", exchangeName, err)
		return fmt.Errorf("exchange %s does not exist or is not accessible: %w", exchangeName, err)
	}
	log.Printf("Exchange %s exists and is accessible", exchangeName)

	err = r.channel.Publish(
		exchangeName, // exchange
		queueName,    // routing key
		true,         // mandatory - make sure message is routable
		false,        // immediate
		amqp.Publishing{
			Headers:         headers,
			ContentType:     "application/json",
			ContentEncoding: "utf-8",
			Body:            body,
			DeliveryMode:    amqp.Persistent, // Make message persistent
			Priority:        uint8(msg.Priority),
			Timestamp:       msg.Timestamp,
			MessageId:       msg.ID,
			CorrelationId:   msg.CorrelationID,
			Expiration:      expiration,
		},
	)
	if err != nil {
		log.Printf("Failed to publish message: %v", err)
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// Check for returned messages (when mandatory=true)
	returns := r.channel.NotifyReturn(make(chan amqp.Return))
	select {
	case ret := <-returns:
		log.Printf("Message was returned: %v", ret)
		return fmt.Errorf("message was returned: %v", ret)
	default:
		log.Printf("Message published successfully")
	}

	return nil
}

// Consume starts consuming messages from a queue
func (r *RabbitMQ) Consume(queueName string, handler func(*model.Message) error) error {
	// Ensure queue and exchange are set up
	if err := r.ensureQueueSetup(queueName); err != nil {
		return fmt.Errorf("failed to ensure queue setup: %w", err)
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
				// Log error and reject message to dead letter queue
				d.Reject(false)
				continue
			}

			if err := handler(msg); err != nil {
				// Log error and reject message to dead letter queue
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
