package database

import (
	"context"
	"fmt"
	"time"

	"microservices/services/profile-storage/internal/config"
	"microservices/services/profile-storage/internal/logger"
	"microservices/services/profile-storage/internal/metrics"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// ConnectionManager handles database connection lifecycle and retries
type ConnectionManager struct {
	config  *config.Config
	db      *sqlx.DB
	metrics *metrics.PoolMetrics
	log     *zap.Logger
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(cfg *config.Config) *ConnectionManager {
	return &ConnectionManager{
		config:  cfg,
		metrics: metrics.NewPoolMetrics(),
		log:     logger.Get(),
	}
}

// Connect establishes a database connection with retry logic
func (cm *ConnectionManager) Connect(ctx context.Context) error {
	var err error
	maxRetries := 5
	retryInterval := time.Second

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			startTime := time.Now()
			cm.metrics.IncrementWaitingRequests()
			cm.db, err = sqlx.Connect("postgres", cm.config.GetDSN())
			cm.metrics.DecrementWaitingRequests()
			cm.metrics.RecordAcquisitionTime(time.Since(startTime))

			if err == nil {
				// Configure connection pool
				cm.db.SetMaxOpenConns(cm.config.MaxOpenConns)
				cm.db.SetMaxIdleConns(cm.config.MaxIdleConns)
				cm.db.SetConnMaxLifetime(cm.config.ConnMaxLifetime)
				cm.db.SetConnMaxIdleTime(cm.config.ConnMaxIdleTime)

				// Verify connection
				if err = cm.db.PingContext(ctx); err == nil {
					cm.metrics.IncrementSuccessfulRetries()
					cm.log.Info("Successfully connected to database",
						logger.Int("attempt", i+1),
						logger.String("host", cm.config.DBHost),
						logger.String("database", cm.config.DBName),
					)
					return nil
				}
			}

			cm.metrics.IncrementConnectionErrors()
			cm.metrics.IncrementRetryAttempts()
			cm.log.Warn("Failed to connect to database",
				logger.Int("attempt", i+1),
				logger.Int("max_attempts", maxRetries),
				logger.ErrorField(err),
				logger.String("host", cm.config.DBHost),
				logger.String("database", cm.config.DBName),
			)
			if i < maxRetries-1 {
				time.Sleep(retryInterval)
				retryInterval *= 2 // Exponential backoff
			}
		}
	}

	return fmt.Errorf("failed to connect to database after %d attempts: %v", maxRetries, err)
}

// GetDB returns the database connection
func (cm *ConnectionManager) GetDB() *sqlx.DB {
	return cm.db
}

// Close closes the database connection
func (cm *ConnectionManager) Close() error {
	if cm.db != nil {
		cm.log.Info("Closing database connection",
			logger.String("host", cm.config.DBHost),
			logger.String("database", cm.config.DBName),
		)
		return cm.db.Close()
	}
	return nil
}

// Ping verifies the database connection
func (cm *ConnectionManager) Ping(ctx context.Context) error {
	if cm.db == nil {
		cm.log.Error("Database connection not initialized",
			logger.String("host", cm.config.DBHost),
			logger.String("database", cm.config.DBName),
		)
		return fmt.Errorf("database connection not initialized")
	}
	return cm.db.PingContext(ctx)
}

// Reconnect attempts to reconnect to the database
func (cm *ConnectionManager) Reconnect(ctx context.Context) error {
	if cm.db != nil {
		cm.log.Info("Reconnecting to database",
			logger.String("host", cm.config.DBHost),
			logger.String("database", cm.config.DBName),
		)
		cm.db.Close()
	}
	return cm.Connect(ctx)
}

// GetMetrics returns the current pool metrics
func (cm *ConnectionManager) GetMetrics() map[string]interface{} {
	if cm.db != nil {
		stats := cm.db.Stats()
		cm.metrics.SetIdleConnections(int64(stats.Idle))
		cm.metrics.IncrementOpenConnections()
	}
	return cm.metrics.GetMetrics()
}
