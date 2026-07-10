package queue

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *Config
	logger  *Logger
	done    chan struct{}
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

	// Declare exchange
	err = p.channel.ExchangeDeclare(
		p.config.Exchange,
		"direct", // type
		p.config.Durable,
		p.config.AutoDelete,
		false, // internal
		p.config.NoWait,
		nil, // arguments
	)
	if err != nil {
		return err
	}

	// Enable publisher confirms
	err = p.channel.Confirm(false)
	if err != nil {
		return err
	}

	return nil
}

func (p *Publisher) PublishMessage(ctx context.Context, msg *Message) error {
	if p.conn == nil || p.conn.IsClosed() {
		if err := p.connect(); err != nil {
			return err
		}
	}

	body, err := msg.MarshalJSON()
	if err != nil {
		return ErrInvalidMessage
	}

	// Publish with confirmation
	err = p.channel.PublishWithContext(ctx,
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
	)
	if err != nil {
		incrementPublishErrors(p.config.Exchange, p.config.RoutingKey, err.Error())
		return ErrPublishFailed
	}

	// Wait for confirmation
	select {
	case confirm := <-p.channel.NotifyPublish(make(chan amqp.Confirmation, 1)):
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
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
	close(p.done)
	return nil
}
