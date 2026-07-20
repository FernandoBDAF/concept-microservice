package document

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, doc *Document) error
	// CreateWithTask commits the document row AND its document.process
	// outbox envelope in ONE transaction (ADR-008.3), advancing the
	// document to processing as the task is outboxed. On success
	// doc.Status is StatusProcessing.
	CreateWithTask(ctx context.Context, doc *Document, routingKey string, envelope []byte) error
	GetByID(ctx context.Context, id uuid.UUID) (*Document, error)
	GetByProfileID(ctx context.Context, profileID uuid.UUID, limit, offset int) ([]*Document, error)
	CountByProfileID(ctx context.Context, profileID uuid.UUID) (int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status DocumentStatus, errorMsg *string) error
	UpdateProcessingStarted(ctx context.Context, id uuid.UUID) error
	UpdateProcessingCompleted(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}
