package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fernandobarroso/reestruct_queue_flow/pkg/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewPublisher(url string) (*Publisher, error) {
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

	return &Publisher{
		conn:    conn,
		channel: ch,
	}, nil
}

func (p *Publisher) PublishMessage(body string) error {
	return p.channel.Publish(
		rabbitmq.ExchangeName, // exchange
		rabbitmq.RoutingKey,   // routing key
		false,                 // mandatory
		false,                 // immediate
		amqp.Publishing{
			ContentType:  "text/plain",
			DeliveryMode: amqp.Persistent,
			Body:         []byte(body),
			Timestamp:    time.Now(),
		},
	)
}

func (p *Publisher) Close() {
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}

func main() {
	// Get RabbitMQ URL from environment
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://user:password@rabbitmq:5672/"
	}

	// Create publisher
	publisher, err := NewPublisher(rabbitURL)
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	// Publish test messages
	for i := 1; i <= 5; i++ {
		msg := fmt.Sprintf("Test message %d", i)
		err := publisher.PublishMessage(msg)
		if err != nil {
			log.Printf("Failed to publish message %d: %v", i, err)
			continue
		}
		log.Printf("Published message: %s", msg)
		time.Sleep(time.Second) // Wait a bit between messages
	}

	log.Println("Publisher finished sending messages")
}
