package rabbitmq

import (
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/fernandobarroso/microservices/api-service/internal/config"
	"github.com/fernandobarroso/microservices/api-service/internal/domain/task"
)

// Client manages a RabbitMQ connection and channel
type Client struct {
	conn           *amqp.Connection
	channel        *amqp.Channel
	config         config.RabbitMQConfig
	confirms       chan amqp.Confirmation
	confirmTracker map[uint64]chan bool
	trackerMu      sync.RWMutex
	connected      bool
	mu             sync.RWMutex
}

func NewClient(cfg config.RabbitMQConfig) (*Client, error) {
	client := &Client{
		config:         cfg,
		confirmTracker: make(map[uint64]chan bool),
	}

	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	return client, nil
}

func (c *Client) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	var conn *amqp.Connection

	for _, host := range c.config.Hosts {
		url := fmt.Sprintf("amqp://%s:%s@%s/%s",
			c.config.Username,
			c.config.Password,
			host,
			c.config.VHost,
		)
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to RabbitMQ host %s: %v", host, err)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to any RabbitMQ host: %w", err)
	}

	c.conn = conn

	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("failed to enable publisher confirms: %w", err)
	}

	if err := ch.Qos(c.config.PrefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	c.channel = ch
	c.connected = true

	c.confirms = make(chan amqp.Confirmation, 100)
	ch.NotifyPublish(c.confirms)

	go c.handleConfirms()
	go c.monitorConnection()

	return nil
}

func (c *Client) handleConfirms() {
	for confirm := range c.confirms {
		c.trackerMu.RLock()
		if ch, exists := c.confirmTracker[confirm.DeliveryTag]; exists {
			ch <- confirm.Ack
			close(ch)
			delete(c.confirmTracker, confirm.DeliveryTag)
		}
		c.trackerMu.RUnlock()
	}
}

func (c *Client) monitorConnection() {
	closed := c.conn.NotifyClose(make(chan *amqp.Error))
	for err := range closed {
		if err != nil {
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			for {
				if err := c.connect(); err == nil {
					break
				}
				time.Sleep(c.config.ReconnectTimeout)
			}
		}
	}
}

func (c *Client) ensureTopology(routingKey string) error {
	config, exists := task.DefaultRoutingMap[routingKey]
	if !exists {
		config = task.RoutingConfig{
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

	if err := c.channel.ExchangeDeclare(
		config.Exchange,
		"direct",
		config.Durable,
		config.AutoDelete,
		false,
		config.NoWait,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", config.Exchange, err)
	}

	dlxName := config.Exchange + ".dlx"
	if err := c.channel.ExchangeDeclare(
		dlxName,
		"direct",
		config.Durable,
		config.AutoDelete,
		false,
		config.NoWait,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare dead letter exchange %s: %w", dlxName, err)
	}

	queueArgs := amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": routingKey,
		"x-message-ttl":             int32(config.TTL.Milliseconds()),
		"x-max-retries":             config.MaxRetries,
	}

	if _, err := c.channel.QueueDeclare(
		config.Queue,
		config.Durable,
		config.AutoDelete,
		config.Exclusive,
		config.NoWait,
		queueArgs,
	); err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", config.Queue, err)
	}

	if err := c.channel.QueueBind(
		config.Queue,
		routingKey,
		config.Exchange,
		config.NoWait,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind queue %s to exchange %s: %w", config.Queue, config.Exchange, err)
	}

	dlqName := config.Queue + ".dlq"
	dlqArgs := amqp.Table{
		"x-message-ttl": int32(config.DeadLetterTTL.Milliseconds()),
	}
	if _, err := c.channel.QueueDeclare(
		dlqName,
		config.Durable,
		config.AutoDelete,
		config.Exclusive,
		config.NoWait,
		dlqArgs,
	); err != nil {
		return fmt.Errorf("failed to declare dead letter queue %s: %w", dlqName, err)
	}

	if err := c.channel.QueueBind(
		dlqName,
		routingKey,
		dlxName,
		config.NoWait,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind dead letter queue %s: %w", dlqName, err)
	}

	return nil
}

func (c *Client) PublishWithRoutingKey(routingKey string, body []byte, headers amqp.Table, msgID string, priority uint8, timestamp time.Time, correlationID string, contentType string) error {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected to rabbitmq")
	}
	c.mu.RUnlock()

	if err := c.ensureTopology(routingKey); err != nil {
		return fmt.Errorf("failed to ensure topology: %w", err)
	}

	config := task.DefaultRoutingMap[routingKey]
	if config.Exchange == "" {
		config.Exchange = "tasks-exchange"
	}

	c.trackerMu.Lock()
	deliveryTag := c.channel.GetNextPublishSeqNo()
	confirmCh := make(chan bool, 1)
	c.confirmTracker[deliveryTag] = confirmCh
	c.trackerMu.Unlock()

	if err := c.channel.Publish(
		config.Exchange,
		routingKey,
		true,
		false,
		amqp.Publishing{
			Headers:         headers,
			ContentType:     contentType,
			ContentEncoding: "utf-8",
			Body:            body,
			DeliveryMode:    amqp.Persistent,
			Priority:        priority,
			Timestamp:       timestamp,
			MessageId:       msgID,
			CorrelationId:   correlationID,
		},
	); err != nil {
		c.trackerMu.Lock()
		delete(c.confirmTracker, deliveryTag)
		c.trackerMu.Unlock()
		return fmt.Errorf("failed to publish message: %w", err)
	}

	timeout := c.config.ConfirmTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	select {
	case ack := <-confirmCh:
		if !ack {
			return fmt.Errorf("message was not acknowledged by broker")
		}
		return nil
	case <-time.After(timeout):
		c.trackerMu.Lock()
		delete(c.confirmTracker, deliveryTag)
		c.trackerMu.Unlock()
		return fmt.Errorf("publisher confirm timeout after %v", timeout)
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	if c.confirms != nil {
		close(c.confirms)
	}
	if c.channel != nil {
		_ = c.channel.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

