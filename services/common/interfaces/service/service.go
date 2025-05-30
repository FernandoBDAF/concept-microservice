package service

import (
	"context"
)

// Service defines the base service interface
type Service[T any] interface {
	// Create creates a new entity
	Create(ctx context.Context, entity *T) error

	// Get retrieves an entity by ID
	Get(ctx context.Context, id string) (*T, error)

	// List retrieves all entities
	List(ctx context.Context) ([]*T, error)

	// Update updates an existing entity
	Update(ctx context.Context, entity *T) error

	// Delete removes an entity by ID
	Delete(ctx context.Context, id string) error
}

// Validator defines the interface for entity validation
type Validator[T any] interface {
	// Validate validates an entity
	Validate(ctx context.Context, entity *T) error
}

// ValidatableService defines a service that supports validation
type ValidatableService[T any] interface {
	Service[T]
	Validator[T]
}

// QueryOptions represents options for querying entities
type QueryOptions struct {
	Limit  int
	Offset int
	Sort   []string
	Filter map[string]interface{}
}

// QueryableService defines a service that supports querying
type QueryableService[T any] interface {
	Service[T]

	// Query retrieves entities based on query options
	Query(ctx context.Context, options QueryOptions) ([]*T, error)

	// Count returns the total number of entities matching the query
	Count(ctx context.Context, options QueryOptions) (int64, error)
}

// TransactionalService defines a service that supports transactions
type TransactionalService[T any] interface {
	Service[T]

	// Begin starts a new transaction
	Begin(ctx context.Context) (Transaction, error)
}

// Transaction represents a service transaction
type Transaction interface {
	// Commit commits the transaction
	Commit() error

	// Rollback rolls back the transaction
	Rollback() error
}
