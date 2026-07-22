// Package outbox implements the transactional outbox: domain writes and
// their task events commit in ONE database transaction; a relay goroutine
// publishes pending rows with broker confirms and marks them sent. This
// closes the crash window where the DB commits but the publish is lost
// (lab experiment EXP-42 — authored; live run pending).
//
// Duplicate publishes are tolerated BY DESIGN: a crash between publish and
// mark-sent re-publishes the row, and consumer-side idempotency (a SETNX on
// the envelope id — see the worker-go template) dedupes, which is why the
// envelope is stored whole.
package outbox

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	// DefaultInterval is the idle poll interval; while batches come back
	// full the relay loops immediately.
	DefaultInterval = 250 * time.Millisecond
	// DefaultBatchSize bounds one relay transaction.
	DefaultBatchSize = 100
)

var (
	outboxPending = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "api_outbox_pending",
		Help: "Outbox rows not yet published (sent_at IS NULL)",
	})
	outboxPublished = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "api_outbox_published_total",
		Help: "Outbox rows published with broker confirm and marked sent",
	})
	outboxPublishErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "api_outbox_publish_errors_total",
		Help: "Outbox publish attempts that failed (attempts bumped, row retried)",
	})
)

func init() {
	prometheus.MustRegister(outboxPending, outboxPublished, outboxPublishErrors)
}

// Row is one to-be-published task event. Envelope fields are stored
// pre-serialized (payload JSON + routing key) so the relay never needs
// domain knowledge.
type Row struct {
	ID         int64      `db:"id"`
	RoutingKey string     `db:"routing_key"`
	Envelope   []byte     `db:"envelope"` // full task.Message JSON, built at write time
	CreatedAt  time.Time  `db:"created_at"`
	SentAt     *time.Time `db:"sent_at"`
	Attempts   int        `db:"attempts"`
}

// Store persists events inside the caller's transaction — the whole point:
// callers pass the SAME tx that writes the domain row.
type Store struct{ db *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }

// DB exposes the underlying pool for callers that open the combined
// domain-write + outbox transaction themselves.
func (s *Store) DB() *sqlx.DB { return s.db }

// Add inserts a pending row within tx.
func (s *Store) Add(ctx context.Context, tx *sqlx.Tx, routingKey string, envelope []byte) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO outbox (routing_key, envelope) VALUES ($1, $2)`,
		routingKey, envelope,
	)
	return err
}

// Enqueue is the no-domain-write path (task endpoints): a short transaction
// holding only the outbox insert, so every publish flows through one path.
// It satisfies task.Enqueuer.
func (s *Store) Enqueue(ctx context.Context, routingKey string, envelope []byte) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	if err := s.Add(ctx, tx, routingKey, envelope); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// PendingBatch fetches unsent rows FOR UPDATE SKIP LOCKED within tx (relay
// is safe to run on every replica). The tx must stay open across
// publish + mark: the row locks are the mutual exclusion.
func (s *Store) PendingBatch(ctx context.Context, tx *sqlx.Tx, limit int) ([]Row, error) {
	var rows []Row
	err := tx.SelectContext(ctx, &rows,
		`SELECT id, routing_key, envelope, created_at, sent_at, attempts
		   FROM outbox
		  WHERE sent_at IS NULL
		  ORDER BY id
		  LIMIT $1
		    FOR UPDATE SKIP LOCKED`,
		limit,
	)
	return rows, err
}

// MarkSent stamps sent_at; MarkFailed bumps attempts (the relay backs off
// via its poll interval and rows are never deleted by the relay — alerting
// keys off attempts and the pending gauge).
func (s *Store) MarkSent(ctx context.Context, tx *sqlx.Tx, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`UPDATE outbox SET sent_at = now() WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, tx.Rebind(query), args...)
	return err
}

func (s *Store) MarkFailed(ctx context.Context, tx *sqlx.Tx, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`UPDATE outbox SET attempts = attempts + 1 WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, tx.Rebind(query), args...)
	return err
}

// PendingCount backs the api_outbox_pending gauge (cheap via the partial
// index idx_outbox_pending).
func (s *Store) PendingCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM outbox WHERE sent_at IS NULL`)
	return n, err
}

// Publisher is satisfied by the rabbitmq publisher (confirm-mode).
type Publisher interface {
	PublishRaw(ctx context.Context, routingKey string, body []byte) error
}

// Relay drains the outbox until ctx is done: poll → one batch tx
// (PendingBatch FOR UPDATE SKIP LOCKED → publish each with confirms →
// MarkSent/MarkFailed → commit). Runs from cmd/server/main.go as a
// goroutine; idle interval ~250ms, immediate loop while batches come back
// full. Returns nil on context cancellation (graceful shutdown).
func Relay(ctx context.Context, store *Store, pub Publisher, interval time.Duration) error {
	log := zap.L().Named("outbox_relay")
	if interval <= 0 {
		interval = DefaultInterval
	}
	log.Info("outbox relay started", zap.Duration("interval", interval))

	for {
		n, err := relayOnce(ctx, store, pub, DefaultBatchSize)
		if err != nil && ctx.Err() == nil {
			log.Warn("outbox relay batch failed", zap.Error(err))
		}

		if pending, err := store.PendingCount(ctx); err == nil {
			outboxPending.Set(float64(pending))
		}

		if n == DefaultBatchSize && ctx.Err() == nil {
			continue // backlog: keep draining without the idle wait
		}

		select {
		case <-ctx.Done():
			log.Info("outbox relay stopped")
			return nil
		case <-time.After(interval):
		}
	}
}

// relayOnce is one batch transaction: select + publish + mark. Publish
// failures bump attempts and leave the row pending for the next cycle;
// a crash anywhere here re-publishes at most one batch (dedupe downstream).
func relayOnce(ctx context.Context, store *Store, pub Publisher, limit int) (int, error) {
	tx, err := store.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }() // no-op after commit

	rows, err := store.PendingBatch(ctx, tx, limit)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, tx.Commit()
	}

	var sent, failed []int64
	for _, row := range rows {
		if err := pub.PublishRaw(ctx, row.RoutingKey, row.Envelope); err != nil {
			zap.L().Named("outbox_relay").Warn("publish failed",
				zap.Int64("outbox_id", row.ID),
				zap.String("routing_key", row.RoutingKey),
				zap.Int("attempts", row.Attempts+1),
				zap.Error(err))
			failed = append(failed, row.ID)
			outboxPublishErrors.Inc()
			continue
		}
		sent = append(sent, row.ID)
	}

	if err := store.MarkSent(ctx, tx, sent); err != nil {
		return 0, err
	}
	if err := store.MarkFailed(ctx, tx, failed); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	outboxPublished.Add(float64(len(sent)))
	return len(rows), nil
}
