package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"example.com/api-publisher/internal/config"
	"example.com/api-publisher/internal/task"
)

// Client manages a RabbitMQ connection and channel.
//
// Topology ownership: the broker loads the full topology (exchanges, queues,
// bindings, DLQ/TTL args) from its own provisioning — e.g. a definitions.json
// loaded at boot. This client never declares anything; it only verifies
// passively at (re)connect that the resources in task.DefaultRoutingMap
// exist, and crashes with a pointed message when they don't.
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

	// Passive verification before anything publishes: the broker must already
	// hold the topology (definitions.json or equivalent).
	if err := verifyTopology(conn); err != nil {
		_ = conn.Close()
		return err
	}

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

// verifyTopology passively checks every exchange and queue in the routing
// map. A failed passive declare closes its channel, so each entry gets a
// throwaway channel; any failure aborts with the pointed crash message —
// the process must not start against a topology-less broker.
func verifyTopology(conn *amqp.Connection) error {
	for routingKey, rc := range task.DefaultRoutingMap {
		ch, err := conn.Channel()
		if err != nil {
			return fmt.Errorf("failed to open verification channel: %w", err)
		}

		if err := ch.ExchangeDeclarePassive(rc.Exchange, "direct", true, false, false, false, nil); err != nil {
			return topologyMissingError("exchange", rc.Exchange, routingKey, err)
		}
		if _, err := ch.QueueDeclarePassive(rc.Queue, true, false, false, false, nil); err != nil {
			return topologyMissingError("queue", rc.Queue, routingKey, err)
		}

		_ = ch.Close()
	}
	return nil
}

func topologyMissingError(kind, name, routingKey string, err error) error {
	return fmt.Errorf(
		"broker topology missing — is definitions.json loaded? (%s %q for routing key %q not found: %v)",
		kind, name, routingKey, err,
	)
}

func (c *Client) handleConfirms() {
	for confirm := range c.confirms {
		// Write lock: this deletes from the tracker (the old code deleted
		// under RLock — a data race).
		c.trackerMu.Lock()
		if ch, exists := c.confirmTracker[confirm.DeliveryTag]; exists {
			ch <- confirm.Ack
			close(ch)
			delete(c.confirmTracker, confirm.DeliveryTag)
		}
		c.trackerMu.Unlock()
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
				} else {
					log.Printf("RabbitMQ reconnect failed: %v", err)
				}
				time.Sleep(c.config.ReconnectTimeout)
			}
		}
	}
}

// Publish sends a raw, pre-serialized envelope with publisher confirms. No
// topology is declared: the routing key maps to its exchange via
// task.DefaultRoutingMap, and unknown keys fail fast — there is no fallback.
// headers and messageID restore broker-level metadata (e.g. W3C trace
// propagation headers so consumers can continue the trace); both may be empty.
func (c *Client) Publish(ctx context.Context, routingKey string, body []byte, headers amqp.Table, messageID string) error {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected to rabbitmq")
	}
	c.mu.RUnlock()

	rc, ok := task.DefaultRoutingMap[routingKey]
	if !ok {
		return fmt.Errorf("unknown routing key %q: not in the contract routing map", routingKey)
	}

	c.trackerMu.Lock()
	deliveryTag := c.channel.GetNextPublishSeqNo()
	confirmCh := make(chan bool, 1)
	c.confirmTracker[deliveryTag] = confirmCh
	c.trackerMu.Unlock()

	if err := c.channel.PublishWithContext(
		ctx,
		rc.Exchange,
		routingKey,
		true,
		false,
		amqp.Publishing{
			Headers:         headers,
			ContentType:     "application/json",
			ContentEncoding: "utf-8",
			Body:            body,
			DeliveryMode:    amqp.Persistent,
			Timestamp:       time.Now().UTC(),
			MessageId:       messageID,
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
	case <-ctx.Done():
		c.trackerMu.Lock()
		delete(c.confirmTracker, deliveryTag)
		c.trackerMu.Unlock()
		return ctx.Err()
	case <-time.After(timeout):
		c.trackerMu.Lock()
		delete(c.confirmTracker, deliveryTag)
		c.trackerMu.Unlock()
		return fmt.Errorf("publisher confirm timeout after %v", timeout)
	}
}

// Channel opens a fresh channel on the current connection (used by
// consumers, which manage their own channel lifecycle).
func (c *Client) Channel() (*amqp.Channel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.connected || c.conn == nil {
		return nil, fmt.Errorf("not connected to rabbitmq")
	}
	return c.conn.Channel()
}

// PrefetchCount exposes the configured consumer prefetch.
func (c *Client) PrefetchCount() int {
	return c.config.PrefetchCount
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false

	// Do NOT manually close(c.confirms) here: amqp091-go already closes every
	// channel registered via NotifyPublish (which includes c.confirms) as part
	// of Channel.shutdown(), triggered below by channel.Close()/conn.Close().
	// Closing it ourselves first would race the library and cause a
	// "close of closed channel" panic during shutdown.
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
