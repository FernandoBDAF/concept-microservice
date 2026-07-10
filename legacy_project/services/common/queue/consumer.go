package queue

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *Config
	logger  *Logger
	done    chan struct{}
}

func NewConsumer(config *Config) (*Consumer, error) {
	logger, err := NewLogger(config.LogLevel)
	if err != nil {
		return nil, err
	}

	c := &Consumer{
		config: config,
		logger: logger,
		done:   make(chan struct{}),
	}

	if err := c.connect(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Consumer) connect() error {
	var err error
	c.conn, err = amqp.DialConfig(c.config.URL, amqp.Config{
		Heartbeat: c.config.Heartbeat,
		Locale:    c.config.Locale,
	})
	if err != nil {
		return ErrConnectionFailed
	}

	c.channel, err = c.conn.Channel()
	if err != nil {
		return ErrChannelFailed
	}

	// Declare exchange
	err = c.channel.ExchangeDeclare(
		c.config.Exchange,
		"direct", // type
		c.config.Durable,
		c.config.AutoDelete,
		false, // internal
		c.config.NoWait,
		nil, // arguments
	)
	if err != nil {
		return err
	}

	// Declare queue
	_, err = c.channel.QueueDeclare(
		c.config.Queue,
		c.config.Durable,
		c.config.AutoDelete,
		c.config.Exclusive,
		c.config.NoWait,
		nil, // arguments
	)
	if err != nil {
		return err
	}

	// Bind queue to exchange
	err = c.channel.QueueBind(
		c.config.Queue,
		c.config.RoutingKey,
		c.config.Exchange,
		c.config.NoWait,
		nil,
	)
	if err != nil {
		return err
	}

	// Set QoS
	err = c.channel.Qos(
		c.config.PrefetchCount,
		c.config.PrefetchSize,
		c.config.Global,
	)
	if err != nil {
		return err
	}

	return nil
}

func (c *Consumer) Start(ctx context.Context, handler MessageHandler) error {
	if c.conn == nil || c.conn.IsClosed() {
		if err := c.connect(); err != nil {
			return err
		}
	}

	deliveries, err := c.channel.Consume(
		c.config.Queue,
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return ErrConsumeFailed
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.done:
				return
			case delivery, ok := <-deliveries:
				if !ok {
					return
				}

				startTime := time.Now()

				// Parse message
				var msg Message
				if err := json.Unmarshal(delivery.Body, &msg); err != nil {
					c.logger.Error("failed to unmarshal message",
						zap.Error(err),
						zap.String("queue", c.config.Queue),
					)
					incrementConsumeErrors(c.config.Queue, "unmarshal_error")
					delivery.Nack(false, false)
					continue
				}

				// Process message
				err := handler(&msg)
				if err != nil {
					c.logger.Error("failed to process message",
						zap.Error(err),
						zap.String("queue", c.config.Queue),
						zap.String("message_id", msg.ID),
					)
					incrementConsumeErrors(c.config.Queue, "handler_error")
					delivery.Nack(false, true) // requeue
					continue
				}

				// Acknowledge message
				if err := delivery.Ack(false); err != nil {
					c.logger.Error("failed to acknowledge message",
						zap.Error(err),
						zap.String("queue", c.config.Queue),
						zap.String("message_id", msg.ID),
					)
					incrementConsumeErrors(c.config.Queue, "ack_error")
					continue
				}

				incrementMessagesConsumed(c.config.Queue)
				observeProcessingTime(c.config.Queue, time.Since(startTime).Seconds())
			}
		}
	}()

	return nil
}

func (c *Consumer) Close() error {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
	close(c.done)
	return nil
}
