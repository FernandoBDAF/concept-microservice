package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Publisher interface {
	PublishWithRoutingKey(routingKey string, msg *Message) error
}

type Service struct {
	publisher Publisher
}

func NewService(publisher Publisher) *Service {
	return &Service{publisher: publisher}
}

func (s *Service) Submit(ctx context.Context, routingKey, msgType string, payload interface{}, metadata map[string]string) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	msg := &Message{
		ID:            uuid.New().String(),
		Type:          msgType,
		Timestamp:     time.Now().UTC(),
		CorrelationID: uuid.New().String(),
		Payload:       body,
		Metadata:      metadata,
		Priority:      0,
	}

	if err := s.publisher.PublishWithRoutingKey(routingKey, msg); err != nil {
		return "", err
	}

	return msg.ID, nil
}

