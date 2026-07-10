# Data Storage Patterns

> *Migrated and adapted from legacy_project/reference-materials/development/patterns/data-storage-patterns.md*

## Overview

This document outlines the data storage patterns implemented in the API Service using direct PostgreSQL access via sqlx.

## Architecture Context

In the consolidated architecture, data access is done through **direct PostgreSQL access** using sqlx, not HTTP calls to a storage service.

```go
// Direct PostgreSQL with sqlx
import "github.com/jmoiron/sqlx"

type ProfileRepository struct {
    db *sqlx.DB
}
```

## Primary Storage Strategies

### 1. Repository Pattern

```go
type ProfileRepository struct {
    db     *sqlx.DB
    logger *zap.Logger
}

func NewProfileRepository(db *sqlx.DB, logger *zap.Logger) *ProfileRepository {
    return &ProfileRepository{
        db:     db,
        logger: logger,
    }
}

// Get profile by ID
func (r *ProfileRepository) Get(ctx context.Context, id string) (*Profile, error) {
    var profile Profile
    query := `
        SELECT id, first_name, last_name, email, phone, 
               status, created_at, updated_at
        FROM profiles
        WHERE id = $1 AND deleted_at IS NULL
    `
    
    err := r.db.GetContext(ctx, &profile, query, id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, ErrProfileNotFound
        }
        return nil, fmt.Errorf("failed to get profile: %w", err)
    }
    
    return &profile, nil
}

// Create profile
func (r *ProfileRepository) Create(ctx context.Context, profile *Profile) error {
    query := `
        INSERT INTO profiles (id, first_name, last_name, email, phone, status, created_at, updated_at)
        VALUES (:id, :first_name, :last_name, :email, :phone, :status, :created_at, :updated_at)
    `
    
    _, err := r.db.NamedExecContext(ctx, query, profile)
    if err != nil {
        return fmt.Errorf("failed to create profile: %w", err)
    }
    
    return nil
}

// Update profile
func (r *ProfileRepository) Update(ctx context.Context, profile *Profile) error {
    query := `
        UPDATE profiles
        SET first_name = :first_name, last_name = :last_name, 
            email = :email, phone = :phone, status = :status, updated_at = :updated_at
        WHERE id = :id AND deleted_at IS NULL
    `
    
    result, err := r.db.NamedExecContext(ctx, query, profile)
    if err != nil {
        return fmt.Errorf("failed to update profile: %w", err)
    }
    
    rows, err := result.RowsAffected()
    if err != nil {
        return err
    }
    if rows == 0 {
        return ErrProfileNotFound
    }
    
    return nil
}
```

### 2. Batch Queries

```go
// Get multiple profiles by IDs
func (r *ProfileRepository) GetByIDs(ctx context.Context, ids []string) ([]*Profile, error) {
    if len(ids) == 0 {
        return nil, nil
    }

    query, args, err := sqlx.In(`
        SELECT id, first_name, last_name, email, phone, status, created_at, updated_at
        FROM profiles
        WHERE id IN (?) AND deleted_at IS NULL
    `, ids)
    if err != nil {
        return nil, err
    }

    query = r.db.Rebind(query)

    var profiles []*Profile
    err = r.db.SelectContext(ctx, &profiles, query, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to get profiles: %w", err)
    }

    return profiles, nil
}

// List profiles with pagination
func (r *ProfileRepository) List(ctx context.Context, opts ListOptions) ([]*Profile, error) {
    query := `
        SELECT id, first_name, last_name, email, phone, status, created_at, updated_at
        FROM profiles
        WHERE deleted_at IS NULL
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2
    `

    var profiles []*Profile
    err := r.db.SelectContext(ctx, &profiles, query, opts.Limit, opts.Offset)
    if err != nil {
        return nil, fmt.Errorf("failed to list profiles: %w", err)
    }

    return profiles, nil
}
```

## Data Consistency

### 1. Transactions

```go
func (r *ProfileRepository) WithTransaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }

    defer func() {
        if p := recover(); p != nil {
            tx.Rollback()
            panic(p)
        }
    }()

    if err := fn(tx); err != nil {
        if rbErr := tx.Rollback(); rbErr != nil {
            r.logger.Error("rollback failed", zap.Error(rbErr))
        }
        return err
    }

    return tx.Commit()
}

// Usage: Create profile with preferences
func (s *ProfileService) CreateProfileWithPreferences(ctx context.Context, profile *Profile, prefs *Preferences) error {
    return s.repository.WithTransaction(ctx, func(tx *sqlx.Tx) error {
        // Insert profile
        _, err := tx.NamedExecContext(ctx, `
            INSERT INTO profiles (id, first_name, last_name, email, status, created_at, updated_at)
            VALUES (:id, :first_name, :last_name, :email, :status, :created_at, :updated_at)
        `, profile)
        if err != nil {
            return err
        }

        // Insert preferences
        prefs.ProfileID = profile.ID
        _, err = tx.NamedExecContext(ctx, `
            INSERT INTO profile_preferences (profile_id, language, timezone, created_at, updated_at)
            VALUES (:profile_id, :language, :timezone, :created_at, :updated_at)
        `, prefs)
        return err
    })
}
```

### 2. Soft Deletes

```go
func (r *ProfileRepository) SoftDelete(ctx context.Context, id string) error {
    query := `
        UPDATE profiles
        SET deleted_at = NOW(), updated_at = NOW()
        WHERE id = $1 AND deleted_at IS NULL
    `

    result, err := r.db.ExecContext(ctx, query, id)
    if err != nil {
        return fmt.Errorf("failed to delete profile: %w", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return err
    }
    if rows == 0 {
        return ErrProfileNotFound
    }

    return nil
}
```

## Connection Pool Configuration

```go
func NewDB(cfg *Config) (*sqlx.DB, error) {
    dsn := fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
    )

    db, err := sqlx.Open("postgres", dsn)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Configure connection pool
    db.SetMaxOpenConns(cfg.MaxOpenConns)       // Default: 25
    db.SetMaxIdleConns(cfg.MaxIdleConns)       // Default: 10
    db.SetConnMaxLifetime(30 * time.Minute)
    db.SetConnMaxIdleTime(5 * time.Minute)

    // Test connection
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }

    return db, nil
}
```

## Best Practices

1. **Use named parameters** - Prevent SQL injection, improve readability
2. **Use transactions** - For multi-table operations
3. **Implement soft deletes** - For data recovery
4. **Configure connection pools** - Prevent connection exhaustion
5. **Use batch queries** - For bulk operations

## Cross-References

- [Caching Patterns](caching-patterns.md)
- [Database Best Practices](../best-practices/database-best-practices.md)
- [PostgreSQL Guide](../tools/postgresql.md)

## Notes

- Always use direct sqlx client, not HTTP
- Use prepared statements for repeated queries
- Monitor connection pool metrics
