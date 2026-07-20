package document

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// mockRepo records lifecycle calls for ApplyTaskResult tests.
type mockRepo struct {
	docs map[uuid.UUID]*Document

	getErr           error
	completedCalls   int
	failedCalls      int
	lastFailedErrMsg *string
	statusCalls      int
}

func newMockRepo(docs ...*Document) *mockRepo {
	m := &mockRepo{docs: map[uuid.UUID]*Document{}}
	for _, d := range docs {
		m.docs[d.ID] = d
	}
	return m
}

func (m *mockRepo) Create(ctx context.Context, doc *Document) error { return nil }
func (m *mockRepo) CreateWithTask(ctx context.Context, doc *Document, routingKey string, envelope []byte) error {
	return nil
}
func (m *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (*Document, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.docs[id], nil
}
func (m *mockRepo) GetByProfileID(ctx context.Context, profileID uuid.UUID, limit, offset int) ([]*Document, error) {
	return nil, nil
}
func (m *mockRepo) CountByProfileID(ctx context.Context, profileID uuid.UUID) (int, error) {
	return 0, nil
}
func (m *mockRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status DocumentStatus, errorMsg *string) error {
	m.statusCalls++
	if status == StatusFailed {
		m.failedCalls++
		m.lastFailedErrMsg = errorMsg
	}
	if d, ok := m.docs[id]; ok {
		d.Status = status
		d.ErrorMessage = errorMsg
	}
	return nil
}
func (m *mockRepo) UpdateProcessingStarted(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockRepo) UpdateProcessingCompleted(ctx context.Context, id uuid.UUID) error {
	m.completedCalls++
	if d, ok := m.docs[id]; ok {
		d.Status = StatusCompleted
	}
	return nil
}
func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error { return nil }

func newTestService(repo Repository) *Service {
	return &Service{repo: repo, logger: zap.NewNop()}
}

func processingDoc() *Document {
	return &Document{ID: uuid.New(), Status: StatusProcessing}
}

func TestApplyTaskResult_CompletedAdvancesProcessingDocument(t *testing.T) {
	doc := processingDoc()
	repo := newMockRepo(doc)
	svc := newTestService(repo)

	if err := svc.ApplyTaskResult(context.Background(), doc.ID, "completed", nil); err != nil {
		t.Fatalf("ApplyTaskResult returned error: %v", err)
	}
	if repo.completedCalls != 1 {
		t.Fatalf("expected UpdateProcessingCompleted once, got %d", repo.completedCalls)
	}
	if doc.Status != StatusCompleted {
		t.Errorf("expected document completed, got %s", doc.Status)
	}
}

func TestApplyTaskResult_FailedSetsErrorMessage(t *testing.T) {
	doc := processingDoc()
	repo := newMockRepo(doc)
	svc := newTestService(repo)

	msg := "worker exploded"
	if err := svc.ApplyTaskResult(context.Background(), doc.ID, "failed", &msg); err != nil {
		t.Fatalf("ApplyTaskResult returned error: %v", err)
	}
	if repo.failedCalls != 1 {
		t.Fatalf("expected one failed-status update, got %d", repo.failedCalls)
	}
	if repo.lastFailedErrMsg == nil || *repo.lastFailedErrMsg != msg {
		t.Errorf("expected error message forwarded, got %v", repo.lastFailedErrMsg)
	}
	if doc.Status != StatusFailed {
		t.Errorf("expected document failed, got %s", doc.Status)
	}
}

func TestApplyTaskResult_PendingDocumentStillAdvances(t *testing.T) {
	// The lifecycle is pending→processing→completed/failed; a result for a
	// doc still marked pending (e.g. relay raced ahead of a status read)
	// must not wedge the document.
	doc := &Document{ID: uuid.New(), Status: StatusPending}
	repo := newMockRepo(doc)
	svc := newTestService(repo)

	if err := svc.ApplyTaskResult(context.Background(), doc.ID, "completed", nil); err != nil {
		t.Fatalf("ApplyTaskResult returned error: %v", err)
	}
	if doc.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", doc.Status)
	}
}

func TestApplyTaskResult_TerminalDocumentIsIdempotentSkip(t *testing.T) {
	// Duplicate task.result deliveries are tolerated BY DESIGN (relay may
	// re-publish); a second result must not flip a terminal status.
	doc := &Document{ID: uuid.New(), Status: StatusCompleted}
	repo := newMockRepo(doc)
	svc := newTestService(repo)

	msg := "late duplicate failure"
	if err := svc.ApplyTaskResult(context.Background(), doc.ID, "failed", &msg); err != nil {
		t.Fatalf("ApplyTaskResult returned error: %v", err)
	}
	if repo.statusCalls != 0 || repo.completedCalls != 0 {
		t.Errorf("expected no updates on terminal document")
	}
	if doc.Status != StatusCompleted {
		t.Errorf("terminal status must not change, got %s", doc.Status)
	}
}

func TestApplyTaskResult_UnknownDocumentIsDropped(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	// nil error → the consumer acks instead of redelivering forever.
	if err := svc.ApplyTaskResult(context.Background(), uuid.New(), "completed", nil); err != nil {
		t.Fatalf("expected unknown document to be dropped, got %v", err)
	}
	if repo.completedCalls != 0 || repo.statusCalls != 0 {
		t.Errorf("expected no updates for unknown document")
	}
}

func TestApplyTaskResult_TransientRepoErrorPropagates(t *testing.T) {
	repo := newMockRepo()
	repo.getErr = errors.New("db down")
	svc := newTestService(repo)

	if err := svc.ApplyTaskResult(context.Background(), uuid.New(), "completed", nil); err == nil {
		t.Fatalf("expected transient repo error to propagate for redelivery")
	}
}
