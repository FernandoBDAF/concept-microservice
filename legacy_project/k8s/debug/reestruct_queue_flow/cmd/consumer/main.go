package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fernandobarroso/reestruct_queue_flow/pkg/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewConsumer(url string) (*Consumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %v", err)
	}

	// Declare exchange
	err = ch.ExchangeDeclare(
		rabbitmq.ExchangeName,
		"direct", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %v", err)
	}

	// Declare queue
	_, err = ch.QueueDeclare(
		rabbitmq.QueueName,
		true,  // durable
		false, // auto-deleted
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %v", err)
	}

	// Bind queue to exchange
	err = ch.QueueBind(
		rabbitmq.QueueName,
		rabbitmq.RoutingKey,
		rabbitmq.ExchangeName,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to bind queue: %v", err)
	}

	// Set QoS
	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to set QoS: %v", err)
	}

	return &Consumer{
		conn:    conn,
		channel: ch,
	}, nil
}

func (c *Consumer) Start() error {
	msgs, err := c.channel.Consume(
		rabbitmq.QueueName,
		"",    // consumer tag
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %v", err)
	}

	log.Printf("Consumer started, waiting for messages on queue: %s", rabbitmq.QueueName)

	for msg := range msgs {
		log.Printf("Received message: %s", string(msg.Body))
		// Simulate some processing time
		time.Sleep(500 * time.Millisecond)
		msg.Ack(false)
	}

	return nil
}

func (c *Consumer) Close() {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

func main() {
	// Get RabbitMQ URL from environment
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://user:password@rabbitmq:5672/"
	}

	// Create consumer
	consumer, err := NewConsumer(rabbitURL)
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	defer consumer.Close()

	// Start consuming
	if err := consumer.Start(); err != nil {
		log.Fatalf("Failed to start consumer: %v", err)
	}
}
