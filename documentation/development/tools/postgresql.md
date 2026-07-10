# PostgreSQL Usage Guide

> *Migrated from legacy_project/reference-materials/development/tools/postgresql.md*

## Overview

PostgreSQL is our primary relational database, providing robust data storage and querying capabilities. This guide covers our PostgreSQL implementation, best practices, and common patterns using direct database access with sqlx.

## Key Features Used

### 1. Connection Management with sqlx

We use sqlx for type-safe database access:

```go
// PostgreSQL configuration
type PostgresConfig struct {
    Host     string
    Port     int
    User     string
    Password string
    Database string
    SSLMode  string
    MaxConns int
}

func NewPostgresClient(cfg *PostgresConfig) (*sqlx.DB, error) {
    dsn := fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
    )

    db, err := sqlx.Open("postgres", dsn)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Configure connection pool
    db.SetMaxOpenConns(cfg.MaxConns)
    db.SetMaxIdleConns(cfg.MaxConns / 2)
    db.SetConnMaxLifetime(time.Hour)

    // Test connection
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }

    return db, nil
}
```

### 2. Repository Pattern with sqlx

```go
// Profile repository with sqlx
type ProfileRepository struct {
    db *sqlx.DB
}

func NewProfileRepository(db *sqlx.DB) *ProfileRepository {
    return &ProfileRepository{db: db}
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

// Get profiles by email
func (r *ProfileRepository) GetByEmail(ctx context.Context, email string) (*Profile, error) {
    var profile Profile
    err := r.db.GetContext(ctx, &profile, `
        SELECT id, first_name, last_name, email, phone, status, created_at, updated_at
        FROM profiles
        WHERE email = $1 AND deleted_at IS NULL
    `, email)
    if err != nil {
        return nil, fmt.Errorf("failed to get profile by email: %w", err)
    }
    return &profile, nil
}

// Batch query with IN clause
func (r *ProfileRepository) GetByIDs(ctx context.Context, ids []string) ([]*Profile, error) {
    var profiles []*Profile
    query, args, err := sqlx.In(`
        SELECT id, first_name, last_name, email, phone, status, created_at, updated_at
        FROM profiles
        WHERE id IN (?) AND deleted_at IS NULL
    `, ids)
    if err != nil {
        return nil, err
    }
    
    query = r.db.Rebind(query)
    err = r.db.SelectContext(ctx, &profiles, query, args...)
    return profiles, err
}
```

### 3. Transaction Management

We implement proper transaction handling:

```go
// Transaction wrapper
func (r *ProfileRepository) WithTransaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
    tx, err := r.db.BeginTxx(ctx, &sql.TxOptions{
        Isolation: sql.LevelReadCommitted,
        ReadOnly:  false,
    })
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
            return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
        }
        return err
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    return nil
}

// Transaction usage
func (r *ProfileRepository) CreateWithPreferences(ctx context.Context, profile *Profile, prefs *Preferences) error {
    return r.WithTransaction(ctx, func(tx *sqlx.Tx) error {
        // Insert profile
        _, err := tx.NamedExecContext(ctx, `
            INSERT INTO profiles (id, first_name, last_name, email)
            VALUES (:id, :first_name, :last_name, :email)
        `, profile)
        if err != nil {
            return fmt.Errorf("failed to create profile: %w", err)
        }

        // Insert preferences
        _, err = tx.NamedExecContext(ctx, `
            INSERT INTO profile_preferences (profile_id, language, timezone)
            VALUES (:profile_id, :language, :timezone)
        `, prefs)
        if err != nil {
            return fmt.Errorf("failed to create preferences: %w", err)
        }

        return nil
    })
}
```

### 4. Data Modeling

We follow best practices for data modeling:

```sql
-- Table creation with constraints
CREATE TABLE profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    phone VARCHAR(20),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes
CREATE INDEX idx_profiles_email ON profiles(email);
CREATE INDEX idx_profiles_status ON profiles(status);
CREATE INDEX idx_profiles_created_at ON profiles(created_at);

-- Triggers
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_profiles_updated_at
    BEFORE UPDATE ON profiles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

## Best Practices

1. **Connection Management**

   - Use connection pooling
   - Set appropriate timeouts
   - Handle connection errors
   - Monitor connection health

2. **Query Optimization**

   - Use sqlx for type safety
   - Create appropriate indexes
   - Monitor query performance
   - Use EXPLAIN ANALYZE

3. **Transaction Management**

   - Use appropriate isolation levels
   - Handle deadlocks
   - Implement retry logic
   - Monitor transaction performance

4. **Data Modeling**

   - Use appropriate data types
   - Implement constraints
   - Create indexes
   - Use triggers when needed

## Common Issues and Solutions

1. **Connection Issues**

   - Problem: Connection leaks
   - Solution: Use connection pooling, implement proper cleanup

2. **Performance Issues**

   - Problem: Slow queries
   - Solution: Optimize queries, create indexes, use sqlx properly

3. **Deadlock Issues**
   - Problem: Transaction deadlocks
   - Solution: Implement retry logic, use appropriate isolation levels

## Cross-References

- [Database Best Practices](../best-practices/database-best-practices.md)
- [Redis Guide](redis.md)

## References

- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [sqlx Documentation](https://github.com/jmoiron/sqlx)
- [PostgreSQL Performance Tuning](https://www.postgresql.org/docs/current/performance-tips.html)
