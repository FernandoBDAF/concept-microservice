package queue

// task-results emission (ADR-008.3). After ack-worthy processing a worker
// publishes a task.result envelope to the shared `task-results` exchange
// (rk `task.result`); api-service consumes it to advance document status.
// Contract: graph-worker/shared/contracts/MESSAGE_FORMAT.md "Task Result".

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// TaskResultsExchange / TaskResultRoutingKey are broker-owned
	// (definitions.json); the exchange is shared across all workers.
	TaskResultsExchange  = "task-results"
	TaskResultRoutingKey = "task.result"
	taskResultType       = "task.result"

	statusCompleted = "completed"
	statusFailed    = "failed"
)

// outcome is the terminal disposition of a delivery. It maps 1:1 to both the
// work-message routing (ack / retry-republish / dlx-republish) and the
// task.result status (completed / none / failed), so deriving both from one
// classification keeps routing and results from diverging.
type outcome int

const (
	outcomeSuccess outcome = iota // ack + "completed" result
	outcomeRetry                  // republish to a retry tier + ack, NO result
	outcomeDLQ                    // republish to the DLX + ack + "failed" result
)

// classifyOutcome decides the disposition from the handler error and the
// message's retry attempt (x-death count): nil → success; ErrUnretryable → DLQ;
// a retryable error with a tier still available → retry; exhausted → DLQ.
func classifyOutcome(err error, attempt int) outcome {
	if err == nil {
		return outcomeSuccess
	}
	if errors.Is(err, ErrUnretryable) {
		return outcomeDLQ
	}
	if _, ok := NextRetryTier(attempt); ok {
		return outcomeRetry
	}
	return outcomeDLQ
}

// resultStatus maps an outcome to the task.result status to publish, or
// emit=false for a scheduled retry (not a terminal outcome — no result yet).
func (o outcome) resultStatus() (status string, emit bool) {
	switch o {
	case outcomeSuccess:
		return statusCompleted, true
	case outcomeDLQ:
		return statusFailed, true
	default: // outcomeRetry
		return "", false
	}
}

type taskResultPayload struct {
	TaskID     string `json:"task_id"`
	TaskType   string `json:"task_type"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	EnvelopeID string `json:"envelope_id"`
	DocumentID string `json:"document_id,omitempty"`
}

type taskResultEnvelope struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Timestamp string            `json:"timestamp"`
	Payload   taskResultPayload `json:"payload"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// buildTaskResult marshals a task.result envelope for a work message and
// outcome. The result gets a fresh uuid id; errMsg is included only for failed
// results; document_id only when the task payload carries one.
func buildTaskResult(workerType string, msg *Message, status, errMsg string) ([]byte, error) {
	payload := taskResultPayload{
		TaskID:     msg.ID,
		TaskType:   msg.Type,
		Status:     status,
		EnvelopeID: msg.ID,
		DocumentID: documentIDOf(msg),
	}
	if status == statusFailed {
		payload.Error = errMsg
	}

	source := "worker"
	if workerType != "" {
		source = workerType + "-worker"
	}

	return json.Marshal(taskResultEnvelope{
		ID:        uuid.NewString(),
		Type:      taskResultType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   payload,
		Metadata:  map[string]string{"source": source},
	})
}

// documentIDOf peeks the opaque task payload for a document_id, returning ""
// when absent — only document-processing tasks carry one.
func documentIDOf(msg *Message) string {
	if len(msg.Payload) == 0 {
		return ""
	}
	var p struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return ""
	}
	return p.DocumentID
}

// emitResult publishes a best-effort task.result on the confirm-mode channel.
// It MUST never influence the ack/retry routing of the work message: a build or
// publish failure is only logged and counted (`<type>_result_publish_errors_total`).
func (c *Consumer) emitResult(msg *Message, status, errMsg string) {
	body, err := buildTaskResult(c.config.WorkerType, msg, status, errMsg)
	if err != nil {
		c.logger.Error("failed to build task result (best-effort; routing unaffected)",
			zap.Error(err), zap.String("queue", c.config.Queue), zap.String("message_id", msg.ID))
		incrementResultPublishErrors(c.config.WorkerType)
		return
	}
	if err := c.publishConfirmed(TaskResultsExchange, TaskResultRoutingKey, nil, body); err != nil {
		c.logger.Error("failed to publish task result (best-effort; routing unaffected)",
			zap.Error(err), zap.String("queue", c.config.Queue),
			zap.String("message_id", msg.ID), zap.String("status", status))
		incrementResultPublishErrors(c.config.WorkerType)
		return
	}
	incrementResultsPublished(c.config.WorkerType, status)
}
