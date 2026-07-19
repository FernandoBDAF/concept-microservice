package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/fernandobarroso/microservices/api-service/internal/config"
)

// NewClient creates a PostgreSQL connection pool. The database/sql driver
// is wrapped with otelsql so every query produces a span (no DB metrics,
// spans only). With the no-op tracer provider (tracing disabled) the
// wrapper is effectively free, so it is applied unconditionally.
func NewClient(cfg config.PostgresConfig) (*sqlx.DB, error) {
	sqlDB, err := otelsql.Open("postgres", cfg.DSN,
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			// Ping and connection-reset spans are noise; queries are what
			// we want to see in traces.
			OmitConnResetSession: true,
			OmitConnPrepare:      true,
			OmitRows:             true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres: %w", err)
	}

	db := sqlx.NewDb(sqlDB, "postgres")

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
