// Package postgres wires the connection pool the outbox needs. It is
// intentionally thin — the outbox owns all SQL; this only opens and tunes
// the pool.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // database/sql driver

	"example.com/api-publisher/internal/config"
)

// NewClient opens a PostgreSQL connection pool and verifies it with a ping.
func NewClient(cfg config.PostgresConfig) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return db, nil
}
