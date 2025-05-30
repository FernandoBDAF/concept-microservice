package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/FBDAF/microservices/services/queue-service/internal/adapters/http"
	"github.com/FBDAF/microservices/services/queue-service/internal/domain/model"
)

// QueueService implements the queue operations
type QueueService struct {
	messageStore map[string]*http.MessageStatus
	mu           sync.RWMutex
}

// NewQueueService creates a new queue service
func NewQueueService() *QueueService {
	return &QueueService{
		messageStore: make(map[string]*http.MessageStatus),
	}
}

// PublishMessage publishes a message to the queue
func (s *QueueService) PublishMessage(msg *model.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store message status
	s.messageStore[msg.ID] = &http.MessageStatus{
		ID:        msg.ID,
		Status:    "accepted",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return nil
}

// GetMessageStatus retrieves the status of a message
func (s *QueueService) GetMessageStatus(messageID string) (*http.MessageStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status, exists := s.messageStore[messageID]
	if !exists {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}

	return status, nil
}

// UpdateMessageStatus updates the status of a message
func (s *QueueService) UpdateMessageStatus(messageID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	msgStatus, exists := s.messageStore[messageID]
	if !exists {
		return fmt.Errorf("message not found: %s", messageID)
	}

	msgStatus.Status = status
	msgStatus.Timestamp = time.Now().UTC().Format(time.RFC3339)

	return nil
}
