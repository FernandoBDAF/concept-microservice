# PostgreSQL Usage Guide

## Overview

PostgreSQL is our primary relational database, providing robust data storage and querying capabilities. This guide covers our PostgreSQL implementation, best practices, and common patterns.

## Key Features Used

### 1. Connection Management

We use a connection pool for efficient database connections:

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

func NewPostgresClient(cfg *PostgresConfig) (*sql.DB, error) {
    dsn := fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
    )

    db, err := sql.Open("postgres", dsn)
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

### 2. Query Optimization

We implement various query optimization techniques:

```go
// Prepared statements
type ProfileRepository struct {
    db *sql.DB
    stmts struct {
        getProfile    *sql.Stmt
        createProfile *sql.Stmt
        updateProfile *sql.Stmt
        deleteProfile *sql.Stmt
    }
}

func NewProfileRepository(db *sql.DB) (*ProfileRepository, error) {
    repo := &ProfileRepository{db: db}

    // Prepare statements
    var err error
    repo.stmts.getProfile, err = db.Prepare(`
        SELECT id, name, email, created_at, updated_at
        FROM profiles
        WHERE id = $1
    `)
    if err != nil {
        return nil, fmt.Errorf("failed to prepare get profile statement: %w", err)
    }

    repo.stmts.createProfile, err = db.Prepare(`
        INSERT INTO profiles (id, name, email, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
    `)
    if err != nil {
        return nil, fmt.Errorf("failed to prepare create profile statement: %w", err)
    }

    // ... prepare other statements

    return repo, nil
}

// Index usage
func (r *ProfileRepository) GetProfileByEmail(ctx context.Context, email string) (*Profile, error) {
    var profile Profile
    err := r.db.QueryRowContext(ctx, `
        SELECT id, name, email, created_at, updated_at
        FROM profiles
        WHERE email = $1
    `, email).Scan(
        &profile.ID,
        &profile.Name,
        &profile.Email,
        &profile.CreatedAt,
        &profile.UpdatedAt,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to get profile by email: %w", err)
    }

    return &profile, nil
}
```

### 3. Transaction Management

We implement proper transaction handling:

```go
// Transaction wrapper
func (r *ProfileRepository) WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
    tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
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
func (r *ProfileRepository) TransferPoints(ctx context.Context, fromID, toID string, points int) error {
    return r.WithTransaction(ctx, func(tx *sql.Tx) error {
        // Deduct points from source
        result, err := tx.ExecContext(ctx, `
            UPDATE profiles
            SET points = points - $1
            WHERE id = $2 AND points >= $1
        `, points, fromID)
        if err != nil {
            return fmt.Errorf("failed to deduct points: %w", err)
        }

        affected, err := result.RowsAffected()
        if err != nil {
            return fmt.Errorf("failed to get affected rows: %w", err)
        }
        if affected == 0 {
            return fmt.Errorf("insufficient points")
        }

        // Add points to destination
        _, err = tx.ExecContext(ctx, `
            UPDATE profiles
            SET points = points + $1
            WHERE id = $2
        `, points, toID)
        if err != nil {
            return fmt.Errorf("failed to add points: %w", err)
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
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    points INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT points_non_negative CHECK (points >= 0)
);

-- Indexes
CREATE INDEX idx_profiles_email ON profiles(email);
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

   - Use prepared statements
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
   - Solution: Optimize queries, create indexes, use prepared statements

3. **Deadlock Issues**
   - Problem: Transaction deadlocks
   - Solution: Implement retry logic, use appropriate isolation levels

## Examples from Our Project

### Repository Implementation

```go
type ProfileRepository struct {
    db *sql.DB
}

func (r *ProfileRepository) GetProfile(ctx context.Context, id string) (*Profile, error) {
    var profile Profile
    err := r.db.QueryRowContext(ctx, `
        SELECT id, name, email, points, created_at, updated_at
        FROM profiles
        WHERE id = $1
    `, id).Scan(
        &profile.ID,
        &profile.Name,
        &profile.Email,
        &profile.Points,
        &profile.CreatedAt,
        &profile.UpdatedAt,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to get profile: %w", err)
    }

    return &profile, nil
}

func (r *ProfileRepository) CreateProfile(ctx context.Context, profile *Profile) error {
    return r.db.QueryRowContext(ctx, `
        INSERT INTO profiles (name, email)
        VALUES ($1, $2)
        RETURNING id, created_at, updated_at
    `, profile.Name, profile.Email).Scan(
        &profile.ID,
        &profile.CreatedAt,
        &profile.UpdatedAt,
    )
}
```

### Migration Management

```go
type Migration struct {
    ID        int
    Name      string
    AppliedAt time.Time
}

func (r *ProfileRepository) RunMigrations(ctx context.Context) error {
    // Create migrations table if not exists
    _, err := r.db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS migrations (
            id SERIAL PRIMARY KEY,
            name VARCHAR(255) NOT NULL UNIQUE,
            applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
        )
    `)
    if err != nil {
        return fmt.Errorf("failed to create migrations table: %w", err)
    }

    // Run migrations
    migrations := []string{
        `CREATE TABLE IF NOT EXISTS profiles (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            name VARCHAR(255) NOT NULL,
            email VARCHAR(255) NOT NULL UNIQUE,
            points INTEGER NOT NULL DEFAULT 0,
            created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
            CONSTRAINT points_non_negative CHECK (points >= 0)
        )`,
        `CREATE INDEX IF NOT EXISTS idx_profiles_email ON profiles(email)`,
        // ... more migrations
    }

    for i, migration := range migrations {
        name := fmt.Sprintf("migration_%d", i+1)

        // Check if migration already applied
        var count int
        err := r.db.QueryRowContext(ctx, `
            SELECT COUNT(*)
            FROM migrations
            WHERE name = $1
        `, name).Scan(&count)
        if err != nil {
            return fmt.Errorf("failed to check migration: %w", err)
        }
        if count > 0 {
            continue
        }

        // Run migration
        tx, err := r.db.BeginTx(ctx, nil)
        if err != nil {
            return fmt.Errorf("failed to begin transaction: %w", err)
        }

        if _, err := tx.ExecContext(ctx, migration); err != nil {
            tx.Rollback()
            return fmt.Errorf("failed to run migration: %w", err)
        }

        if _, err := tx.ExecContext(ctx, `
            INSERT INTO migrations (name)
            VALUES ($1)
        `, name); err != nil {
            tx.Rollback()
            return fmt.Errorf("failed to record migration: %w", err)
        }

        if err := tx.Commit(); err != nil {
            return fmt.Errorf("failed to commit migration: %w", err)
        }
    }

    return nil
}
```

## References

- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [PostgreSQL Performance Tuning](https://www.postgresql.org/docs/current/performance-tips.html)
- [PostgreSQL Indexes](https://www.postgresql.org/docs/current/indexes.html)
- [PostgreSQL Transactions](https://www.postgresql.org/docs/current/tutorial-transactions.html)
