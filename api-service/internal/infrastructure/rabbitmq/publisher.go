package rabbitmq

import (
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/fernandobarroso/microservices/api-service/internal/domain/task"
)

// Publisher publishes task messages to RabbitMQ
type Publisher struct {
	client *Client
}

func NewPublisher(client *Client) *Publisher {
	return &Publisher{client: client}
}

func (p *Publisher) PublishWithRoutingKey(routingKey string, msg *task.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	headers := amqp.Table{}
	for k, v := range msg.Metadata {
		headers[k] = v
	}

	priority := uint8(0)
	if msg.Priority > 0 && msg.Priority <= 9 {
		priority = uint8(msg.Priority)
	}

	ts := msg.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	return p.client.PublishWithRoutingKey(
		routingKey,
		body,
		headers,
		msg.ID,
		priority,
		ts,
		msg.CorrelationID,
		"application/json",
	)
}

