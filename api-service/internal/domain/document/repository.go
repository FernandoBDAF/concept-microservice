package document

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, doc *Document) error
	GetByID(ctx context.Context, id uuid.UUID) (*Document, error)
	GetByProfileID(ctx context.Context, profileID uuid.UUID, limit, offset int) ([]*Document, error)
	CountByProfileID(ctx context.Context, profileID uuid.UUID) (int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status DocumentStatus, errorMsg *string) error
	UpdateProcessingStarted(ctx context.Context, id uuid.UUID) error
	UpdateProcessingCompleted(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}
