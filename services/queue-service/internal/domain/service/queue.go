package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/FBDAF/microservices/services/queue-service/internal/adapters/rabbitmq"
	"github.com/FBDAF/microservices/services/queue-service/internal/domain/model"
	"github.com/FBDAF/microservices/services/queue-service/pkg/queue"
)

// QueueService implements the queue operations with routing key support
type QueueService struct {
	messageStore map[string]*model.MessageStatus
	mu           sync.RWMutex
	metrics      *queue.Metrics
	rabbitmq     *rabbitmq.RabbitMQ
	queueName    string
}

// NewQueueService creates a new queue service
func NewQueueService(rmq *rabbitmq.RabbitMQ, queueName string) *QueueService {
	return &QueueService{
		messageStore: make(map[string]*model.MessageStatus),
		metrics:      queue.DefaultMetrics,
		rabbitmq:     rmq,
		queueName:    queueName,
	}
}

// PublishWithRoutingKey publishes a message with specific routing key
func (s *QueueService) PublishWithRoutingKey(routingKey string, msg *model.Message) error {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store message status
	s.messageStore[msg.ID] = &model.MessageStatus{
		ID:        msg.ID,
		Status:    "accepted",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Publish message to RabbitMQ with routing key
	if err := s.rabbitmq.PublishWithRoutingKey(routingKey, msg); err != nil {
		// Update status to failed on error
		s.messageStore[msg.ID].Status = "failed"
		s.messageStore[msg.ID].Timestamp = time.Now().UTC().Format(time.RFC3339)
		s.metrics.ErrorsTotal.Inc()
		return fmt.Errorf("failed to publish message to RabbitMQ: %w", err)
	}

	// Update status to published
	s.messageStore[msg.ID].Status = "published"
	s.messageStore[msg.ID].Timestamp = time.Now().UTC().Format(time.RFC3339)

	// Record metrics
	s.metrics.MessagesTotal.Inc()
	s.metrics.ProcessingDuration.Observe(time.Since(start).Seconds())
	s.metrics.Size.Inc()

	return nil
}

// PublishMessage publishes a message (backward compatibility with routing key inference)
func (s *QueueService) PublishMessage(msg *model.Message) error {
	// Infer routing key from message type for backward compatibility
	routingKey := "profile.task" // default

	switch msg.Type {
	case "profile_update", "profile_task":
		routingKey = "profile.task"
	case "email_send", "email_task":
		routingKey = "email.send"
	case "image_process", "image_task":
		routingKey = "image.process"
	default:
		// For unknown message types, check if there's a routing key in metadata
		if rk, exists := msg.Metadata["routing_key"]; exists {
			routingKey = rk
		}
	}

	return s.PublishWithRoutingKey(routingKey, msg)
}

// GetMessageStatus retrieves the status of a message
func (s *QueueService) GetMessageStatus(messageID string) (*model.MessageStatus, error) {
	start := time.Now()
	s.mu.RLock()
	defer s.mu.RUnlock()

	status, exists := s.messageStore[messageID]
	if !exists {
		s.metrics.ErrorsTotal.Inc()
		return nil, fmt.Errorf("message not found: %s", messageID)
	}

	s.metrics.Latency.Observe(time.Since(start).Seconds())

	return status, nil
}

// UpdateMessageStatus updates the status of a message
func (s *QueueService) UpdateMessageStatus(messageID string, status string) error {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	msgStatus, exists := s.messageStore[messageID]
	if !exists {
		s.metrics.ErrorsTotal.Inc()
		return fmt.Errorf("message not found: %s", messageID)
	}

	msgStatus.Status = status
	msgStatus.Timestamp = time.Now().UTC().Format(time.RFC3339)

	s.metrics.Latency.Observe(time.Since(start).Seconds())

	return nil
}

// StartConsuming starts consuming messages from RabbitMQ
func (s *QueueService) StartConsuming() error {
	return s.rabbitmq.Consume(s.queueName, func(msg *model.Message) error {
		// Update message status
		if err := s.UpdateMessageStatus(msg.ID, "processing"); err != nil {
			return fmt.Errorf("failed to update message status: %w", err)
		}

		// Process message
		// TODO: Implement message processing logic

		// Update message status
		if err := s.UpdateMessageStatus(msg.ID, "completed"); err != nil {
			return fmt.Errorf("failed to update message status: %w", err)
		}

		return nil
	})
}

// GetSupportedRoutingKeys returns list of supported routing keys
func (s *QueueService) GetSupportedRoutingKeys() []string {
	keys := make([]string, 0, len(model.DefaultRoutingMap))
	for key := range model.DefaultRoutingMap {
		keys = append(keys, key)
	}
	return keys
}
