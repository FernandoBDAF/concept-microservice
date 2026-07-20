package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher publishes envelopes to a single exchange with publisher confirms.
// The confirm listener is registered once per channel (not per publish) and
// PublishMessage is safe for concurrent use: publishes and their confirmations
// are serialised under mu so a confirmation is never mismatched to the wrong
// publish.
type Publisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *Config
	logger  *Logger
	done    chan struct{}

	mu       sync.Mutex
	confirms chan amqp.Confirmation
}

func NewPublisher(config *Config) (*Publisher, error) {
	logger, err := NewLogger(config.LogLevel)
	if err != nil {
		return nil, err
	}

	p := &Publisher{
		config: config,
		logger: logger,
		done:   make(chan struct{}),
	}

	if err := p.connect(); err != nil {
		return nil, err
	}

	return p, nil
}

// connect must be called with p.mu held (or before the publisher is shared).
func (p *Publisher) connect() error {
	var err error
	p.conn, err = amqp.DialConfig(p.config.URL, amqp.Config{
		Heartbeat: p.config.Heartbeat,
		Locale:    p.config.Locale,
	})
	if err != nil {
		return ErrConnectionFailed
	}

	p.channel, err = p.conn.Channel()
	if err != nil {
		return ErrChannelFailed
	}

	// Topology is broker-owned (ADR-008.4); verify passively, never declare.
	if err := p.channel.ExchangeDeclarePassive(
		p.config.Exchange,
		"direct",
		p.config.Durable,
		p.config.AutoDelete,
		false, // internal
		p.config.NoWait,
		nil,
	); err != nil {
		return fmt.Errorf("verify exchange %q (is definitions.json loaded?): %w", p.config.Exchange, err)
	}

	// Enable publisher confirms and register the confirm listener ONCE for this
	// channel. Re-registering per publish (the previous bug) leaks channels and
	// fans each confirmation out to stale listeners.
	if err := p.channel.Confirm(false); err != nil {
		return err
	}
	p.confirms = p.channel.NotifyPublish(make(chan amqp.Confirmation, 1))

	return nil
}

func (p *Publisher) PublishMessage(ctx context.Context, msg *Message) error {
	body, err := msg.MarshalJSON()
	if err != nil {
		return ErrInvalidMessage
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil || p.conn.IsClosed() {
		if err := p.connect(); err != nil {
			return err
		}
	}

	if err := p.channel.PublishWithContext(ctx,
		p.config.Exchange,
		p.config.RoutingKey,
		p.config.Mandatory,
		p.config.Immediate,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	); err != nil {
		incrementPublishErrors(p.config.Exchange, p.config.RoutingKey, err.Error())
		return ErrPublishFailed
	}

	select {
	case confirm, ok := <-p.confirms:
		if !ok {
			incrementPublishErrors(p.config.Exchange, p.config.RoutingKey, "confirm channel closed")
			return ErrPublishFailed
		}
		if !confirm.Ack {
			incrementPublishErrors(p.config.Exchange, p.config.RoutingKey, "publish not acknowledged")
			return ErrPublishFailed
		}
	case <-time.After(5 * time.Second):
		incrementPublishErrors(p.config.Exchange, p.config.RoutingKey, "publish confirmation timeout")
		return ErrPublishTimeout
	}

	incrementMessagesPublished(p.config.Exchange, p.config.RoutingKey)
	return nil
}

func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
	close(p.done)
	return nil
}
