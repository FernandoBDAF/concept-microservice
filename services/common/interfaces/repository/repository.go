package repository

import (
	"context"
)

// Repository defines the base repository interface
type Repository[T any] interface {
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

	// Exists checks if an entity exists
	Exists(ctx context.Context, id string) (bool, error)
}

// QueryOptions represents options for querying entities
type QueryOptions struct {
	Limit  int
	Offset int
	Sort   []string
	Filter map[string]interface{}
}

// QueryableRepository defines a repository that supports querying
type QueryableRepository[T any] interface {
	Repository[T]

	// Query retrieves entities based on query options
	Query(ctx context.Context, options QueryOptions) ([]*T, error)

	// Count returns the total number of entities matching the query
	Count(ctx context.Context, options QueryOptions) (int64, error)
}

// TransactionalRepository defines a repository that supports transactions
type TransactionalRepository[T any] interface {
	Repository[T]

	// Begin starts a new transaction
	Begin(ctx context.Context) (Transaction, error)
}

// Transaction represents a database transaction
type Transaction interface {
	// Commit commits the transaction
	Commit() error

	// Rollback rolls back the transaction
	Rollback() error
}
