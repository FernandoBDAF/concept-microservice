# Database Best Practices

> *Migrated from legacy_project/reference-materials/development/database-best-practices.md*

## Overview

This document outlines the best practices for database design, optimization, and management in our microservices architecture. It covers database patterns, connection management, query optimization, and data consistency strategies.

## Database Design

### 1. Schema Design

```sql
-- Profile table with proper indexing
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

-- Indexes for common queries
CREATE INDEX idx_profiles_email ON profiles(email);
CREATE INDEX idx_profiles_status ON profiles(status);
CREATE INDEX idx_profiles_created_at ON profiles(created_at);

-- Preferences table with foreign key
CREATE TABLE profile_preferences (
    profile_id UUID PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
    language VARCHAR(10) NOT NULL DEFAULT 'en',
    timezone VARCHAR(50) NOT NULL DEFAULT 'UTC',
    notification_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### 2. Data Types and Constraints

```sql
-- Using appropriate data types
CREATE TABLE profile_activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    activity_type VARCHAR(50) NOT NULL,
    activity_data JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT valid_activity_type CHECK (activity_type IN ('login', 'update', 'delete'))
);

-- Using enums for fixed values
CREATE TYPE profile_status AS ENUM ('active', 'inactive', 'suspended', 'deleted');
ALTER TABLE profiles ALTER COLUMN status TYPE profile_status USING status::profile_status;
```

## Connection Management

### 1. Connection Pool Configuration

```go
// Database configuration
type DBConfig struct {
    Host            string
    Port            int
    User            string
    Password        string
    Database        string
    MaxConnections  int
    MinConnections  int
    MaxIdleTime     time.Duration
    MaxLifetime     time.Duration
}

// Connection pool setup with sqlx
func NewDBPool(config DBConfig) (*sqlx.DB, error) {
    dsn := fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
        config.Host, config.Port, config.User, config.Password, config.Database,
    )

    db, err := sqlx.Open("postgres", dsn)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Configure connection pool
    db.SetMaxOpenConns(config.MaxConnections)
    db.SetMaxIdleConns(config.MinConnections)
    db.SetConnMaxIdleTime(config.MaxIdleTime)
    db.SetConnMaxLifetime(config.MaxLifetime)

    return db, nil
}
```

### 2. Connection Health Checks

```go
// Health check implementation
func (db *Database) HealthCheck(ctx context.Context) error {
    var result int
    err := db.pool.QueryRowContext(ctx, "SELECT 1").Scan(&result)
    if err != nil {
        return fmt.Errorf("database health check failed: %w", err)
    }
    return nil
}

// Periodic health check
func (db *Database) StartHealthCheck(interval time.Duration) {
    ticker := time.NewTicker(interval)
    go func() {
        for range ticker.C {
            if err := db.HealthCheck(context.Background()); err != nil {
                log.Printf("Database health check failed: %v", err)
            }
        }
    }()
}
```

## Query Optimization

### 1. Using sqlx for Type-Safe Queries

```go
// Profile repository with sqlx
type ProfileRepository struct {
    db *sqlx.DB
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

## Data Consistency

### 1. Transactions

```go
// Transaction wrapper
func (db *Database) WithTransaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
    tx, err := db.pool.BeginTxx(ctx, nil)
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
func (s *Service) UpdateProfileWithPreferences(ctx context.Context, profile *Profile, prefs *Preferences) error {
    return s.db.WithTransaction(ctx, func(tx *sqlx.Tx) error {
        if err := s.updateProfile(ctx, tx, profile); err != nil {
            return err
        }
        return s.updatePreferences(ctx, tx, profile.ID, prefs)
    })
}
```

### 2. Data Validation

```go
// Input validation
func validateProfile(profile *Profile) error {
    if profile.FirstName == "" {
        return errors.New("first name is required")
    }
    if profile.LastName == "" {
        return errors.New("last name is required")
    }
    if profile.Email == "" {
        return errors.New("email is required")
    }
    if !isValidEmail(profile.Email) {
        return errors.New("invalid email format")
    }
    return nil
}
```

## Best Practices

1. **Schema Design**

   - Use appropriate data types
   - Implement proper indexing
   - Define constraints
   - Use foreign keys

2. **Connection Management**

   - Configure connection pools
   - Implement health checks
   - Handle connection errors
   - Monitor pool metrics

3. **Query Optimization**

   - Use prepared statements
   - Implement query caching
   - Optimize indexes
   - Monitor query performance

4. **Data Consistency**
   - Use transactions
   - Implement validation
   - Handle concurrent access
   - Maintain referential integrity

## Common Issues and Solutions

1. **Connection Leaks**

   - Problem: Unclosed connections
   - Solution: Use connection pooling and proper cleanup

2. **Slow Queries**

   - Problem: Unoptimized queries
   - Solution: Use proper indexing and query optimization

3. **Data Inconsistency**
   - Problem: Race conditions
   - Solution: Use transactions and proper locking

## Cross-References

- [Error Handling Best Practices](error-handling-best-practices.md)
- [Logging Best Practices](logging-best-practices.md)
- [PostgreSQL Guide](../tools/postgresql.md)

## References

- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [sqlx Documentation](https://github.com/jmoiron/sqlx)
- [Database Design Best Practices](https://www.postgresql.org/docs/current/ddl.html)
- [Query Optimization](https://www.postgresql.org/docs/current/performance-tips.html)
