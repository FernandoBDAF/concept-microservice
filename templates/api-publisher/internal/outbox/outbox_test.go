package outbox

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

// newMockStore wraps sqlmock with the "postgres" bind type so Rebind
// produces the exact $n placeholders production uses.
func newMockStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewStore(sqlx.NewDb(db, "postgres")), mock
}

func pendingColumns() []string {
	return []string{"id", "routing_key", "envelope", "created_at", "sent_at", "attempts"}
}

func TestStore_Add_InsertsWithinCallerTx(t *testing.T) {
	store, mock := newMockStore(t)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO outbox (routing_key, envelope) VALUES ($1, $2)`)).
		WithArgs("document.process", []byte(`{"id":"e1"}`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	tx, err := store.DB().Beginx()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := store.Add(context.Background(), tx, "document.process", []byte(`{"id":"e1"}`)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestStore_Enqueue_IsOneShortTransaction(t *testing.T) {
	store, mock := newMockStore(t)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO outbox (routing_key, envelope) VALUES ($1, $2)`)).
		WithArgs("email.send", []byte(`{"id":"e2"}`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := store.Enqueue(context.Background(), "email.send", []byte(`{"id":"e2"}`)); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestStore_Enqueue_RollsBackOnInsertError(t *testing.T) {
	store, mock := newMockStore(t)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO outbox`)).
		WillReturnError(errors.New("constraint violation"))
	mock.ExpectRollback()

	if err := store.Enqueue(context.Background(), "email.send", []byte(`{}`)); err == nil {
		t.Fatalf("expected error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestStore_PendingBatch_SQLShape pins the correctness-critical clause: the
// batch is selected FOR UPDATE SKIP LOCKED (multi-replica safe), oldest
// first, only unsent rows, bounded.
func TestStore_PendingBatch_SQLShape(t *testing.T) {
	store, mock := newMockStore(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, routing_key, envelope, created_at, sent_at, attempts\s+FROM outbox\s+WHERE sent_at IS NULL\s+ORDER BY id\s+LIMIT \$1\s+FOR UPDATE SKIP LOCKED`).
		WithArgs(50).
		WillReturnRows(sqlmock.NewRows(pendingColumns()).
			AddRow(int64(7), "profile.task", []byte(`{"id":"a"}`), time.Now(), nil, 0).
			AddRow(int64(8), "email.send", []byte(`{"id":"b"}`), time.Now(), nil, 2))

	tx, err := store.DB().Beginx()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	rows, err := store.PendingBatch(context.Background(), tx, 50)
	if err != nil {
		t.Fatalf("PendingBatch: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].ID != 7 || rows[0].RoutingKey != "profile.task" {
		t.Errorf("row 0 mismatch: %+v", rows[0])
	}
	if rows[1].Attempts != 2 {
		t.Errorf("expected attempts to round-trip, got %+v", rows[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestStore_MarkSentAndMarkFailed_SQLShape(t *testing.T) {
	store, mock := newMockStore(t)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE outbox SET sent_at = now() WHERE id IN ($1, $2)`)).
		WithArgs(int64(1), int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE outbox SET attempts = attempts + 1 WHERE id IN ($1)`)).
		WithArgs(int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	tx, err := store.DB().Beginx()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := store.MarkSent(context.Background(), tx, []int64{1, 2}); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	if err := store.MarkFailed(context.Background(), tx, []int64{3}); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	// Empty id slices must be no-ops (no SQL executed).
	if err := store.MarkSent(context.Background(), tx, nil); err != nil {
		t.Fatalf("MarkSent(nil): %v", err)
	}
	if err := store.MarkFailed(context.Background(), tx, nil); err != nil {
		t.Fatalf("MarkFailed(nil): %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

type fakePublisher struct {
	published []string // routing keys in publish order
	failOn    map[string]error
}

func (f *fakePublisher) PublishRaw(ctx context.Context, routingKey string, body []byte) error {
	if err, ok := f.failOn[routingKey]; ok {
		return err
	}
	f.published = append(f.published, routingKey)
	return nil
}

// TestRelayOnce_BatchTxSelectPublishMark exercises the decided batch shape:
// one transaction spanning select + publish + mark, with failures bumping
// attempts while successes are stamped sent — and the tx committing either
// way (a failed row stays pending for the next cycle).
func TestRelayOnce_BatchTxSelectPublishMark(t *testing.T) {
	store, mock := newMockStore(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`FOR UPDATE SKIP LOCKED`).
		WithArgs(DefaultBatchSize).
		WillReturnRows(sqlmock.NewRows(pendingColumns()).
			AddRow(int64(1), "profile.task", []byte(`{"id":"ok"}`), time.Now(), nil, 0).
			AddRow(int64(2), "image.process", []byte(`{"id":"boom"}`), time.Now(), nil, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE outbox SET sent_at = now() WHERE id IN ($1)`)).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE outbox SET attempts = attempts + 1 WHERE id IN ($1)`)).
		WithArgs(int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	pub := &fakePublisher{failOn: map[string]error{"image.process": errors.New("broker nack")}}
	n, err := relayOnce(context.Background(), store, pub, DefaultBatchSize)
	if err != nil {
		t.Fatalf("relayOnce: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 rows handled, got %d", n)
	}
	if len(pub.published) != 1 || pub.published[0] != "profile.task" {
		t.Errorf("expected exactly the healthy row published, got %v", pub.published)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRelayOnce_EmptyBatchCommitsAndPublishesNothing(t *testing.T) {
	store, mock := newMockStore(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`FOR UPDATE SKIP LOCKED`).
		WithArgs(DefaultBatchSize).
		WillReturnRows(sqlmock.NewRows(pendingColumns()))
	mock.ExpectCommit()

	pub := &fakePublisher{}
	n, err := relayOnce(context.Background(), store, pub, DefaultBatchSize)
	if err != nil {
		t.Fatalf("relayOnce: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 rows, got %d", n)
	}
	if len(pub.published) != 0 {
		t.Errorf("expected no publishes, got %v", pub.published)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestStore_PendingCount(t *testing.T) {
	store, mock := newMockStore(t)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM outbox WHERE sent_at IS NULL`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))

	n, err := store.PendingCount(context.Background())
	if err != nil {
		t.Fatalf("PendingCount: %v", err)
	}
	if n != 4 {
		t.Errorf("expected 4 pending, got %d", n)
	}
}
