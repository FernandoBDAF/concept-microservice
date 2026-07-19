package task

import (
	"context"
	"encoding/json"
	"fmt"
)

// Enqueuer stores a serialized envelope for asynchronous publication. It is
// satisfied by outbox.Store: every task publish goes tx{outbox.Add} and the
// relay does the actual broker publish with confirms (ADR-008.3). The old
// direct-publish path is gone — one publish path for all tasks.
type Enqueuer interface {
	Enqueue(ctx context.Context, routingKey string, envelope []byte) error
}

type Service struct {
	outbox Enqueuer
}

func NewService(outbox Enqueuer) *Service {
	return &Service{outbox: outbox}
}

// Submit builds the frozen-shape envelope (BuildEnvelope) and enqueues it in
// the transactional outbox. The returned ID is the envelope ID.
func (s *Service) Submit(ctx context.Context, routingKey, msgType string, payload interface{}, metadata map[string]string) (string, error) {
	msg, err := BuildEnvelope(ctx, routingKey, msgType, payload, metadata)
	if err != nil {
		return "", err
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal envelope: %w", err)
	}

	if err := s.outbox.Enqueue(ctx, routingKey, body); err != nil {
		return "", err
	}

	return msg.ID, nil
}
