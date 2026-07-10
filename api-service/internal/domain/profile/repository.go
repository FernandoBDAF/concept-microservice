package profile

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, profile *Profile) error
	GetByID(ctx context.Context, id uuid.UUID) (*Profile, error)
	GetByEmail(ctx context.Context, email string) (*Profile, error)
	Update(ctx context.Context, profile *Profile) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, page, pageSize int) ([]*Profile, error)
}

