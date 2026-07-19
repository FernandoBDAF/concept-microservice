package task

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TypeTaskResult is the envelope type workers and graphrag-service publish
// after ack-worthy processing (exchange task-results, rk task.result —
// graph-worker/shared/contracts/MESSAGE_FORMAT.md, ADR-008.3).
const TypeTaskResult = "task.result"

// ResultStatus values in a task.result payload.
const (
	ResultCompleted = "completed"
	ResultFailed    = "failed"
)

// ResultPayload is the payload of a task.result envelope.
type ResultPayload struct {
	TaskID     string `json:"task_id"`
	TaskType   string `json:"task_type"`
	Status     string `json:"status"` // "completed" | "failed"
	Error      string `json:"error,omitempty"`
	EnvelopeID string `json:"envelope_id"`
	DocumentID string `json:"document_id,omitempty"`
}

// DocumentStatusUpdater advances the document lifecycle from a task result
// (satisfied by document.Service.ApplyTaskResult).
type DocumentStatusUpdater interface {
	ApplyTaskResult(ctx context.Context, id uuid.UUID, status string, errorMsg *string) error
}

// ResultHandler consumes task.result envelopes: for document results it
// advances the document status processing→completed/failed; other results
// are logged only (nothing to advance server-side yet).
//
// Duplicate deliveries are tolerated BY DESIGN (the relay may re-publish
// after a crash between publish and mark-sent); ApplyTaskResult skips
// documents already in a terminal state, so the handler is idempotent.
type ResultHandler struct {
	docs DocumentStatusUpdater
	log  *zap.Logger
}

func NewResultHandler(docs DocumentStatusUpdater, log *zap.Logger) *ResultHandler {
	return &ResultHandler{docs: docs, log: log.Named("task_result_handler")}
}

// Handle processes one task.result envelope. A nil error means the message
// can be acked; an error means a transient failure worth redelivering.
// Malformed messages are NOT errors: task-results has no DLQ, so poison
// must be logged and acked, never requeued.
func (h *ResultHandler) Handle(ctx context.Context, msg *Message) error {
	if msg.Type != TypeTaskResult {
		h.log.Warn("unexpected envelope type on task-results queue, dropping",
			zap.String("type", msg.Type), zap.String("id", msg.ID))
		return nil
	}

	var res ResultPayload
	if err := json.Unmarshal(msg.Payload, &res); err != nil {
		h.log.Warn("malformed task.result payload, dropping",
			zap.Error(err), zap.String("id", msg.ID))
		return nil
	}

	if res.Status != ResultCompleted && res.Status != ResultFailed {
		h.log.Warn("task.result with unknown status, dropping",
			zap.String("status", res.Status), zap.String("task_id", res.TaskID))
		return nil
	}

	if res.DocumentID == "" {
		h.log.Info("task result received",
			zap.String("task_type", res.TaskType),
			zap.String("task_id", res.TaskID),
			zap.String("status", res.Status))
		return nil
	}

	docID, err := uuid.Parse(res.DocumentID)
	if err != nil {
		h.log.Warn("task.result with invalid document_id, dropping",
			zap.String("document_id", res.DocumentID), zap.String("task_id", res.TaskID))
		return nil
	}

	if h.docs == nil {
		h.log.Warn("document result received but document service is disabled, dropping",
			zap.String("document_id", res.DocumentID))
		return nil
	}

	var errMsg *string
	if res.Error != "" {
		errMsg = &res.Error
	}

	if err := h.docs.ApplyTaskResult(ctx, docID, res.Status, errMsg); err != nil {
		return fmt.Errorf("failed to apply task result for document %s: %w", docID, err)
	}

	h.log.Info("document status advanced from task result",
		zap.String("document_id", res.DocumentID),
		zap.String("status", res.Status),
		zap.String("task_id", res.TaskID),
		zap.String("envelope_id", res.EnvelopeID))
	return nil
}
