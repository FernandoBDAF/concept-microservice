package profile

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
	"github.com/fernandobarroso/microservices/operational-workers/internal/common/utils"
)

// Processor handles profile task processing. No real profile store is
// wired up (per mission scope) — Process simulates the task and logs
// coherently so behavior is observable end-to-end.
type Processor struct {
	metrics *utils.ProcessorMetrics
}

// NewProcessor creates a new profile processor
func NewProcessor() *Processor {
	return &Processor{
		metrics: utils.NewProcessorMetrics("profile"),
	}
}

// Process processes a profile task message
func (p *Processor) Process(ctx context.Context, msg *queue.Message) error {
	timer := p.metrics.StartTimer()
	defer timer.ObserveDuration()
	p.metrics.RecordProcessingStart()

	profileMsg, err := NewProfileMessage(msg)
	if err != nil {
		p.metrics.RecordProcessingError()
		return fmt.Errorf("failed to parse profile message: %w", err)
	}

	if err := profileMsg.Validate(); err != nil {
		p.metrics.RecordProcessingError()
		return err
	}

	if err := p.runTask(ctx, profileMsg); err != nil {
		p.metrics.RecordProcessingError()
		return err
	}

	p.metrics.RecordProcessingSuccess()
	return nil
}

// Validate validates the message
func (p *Processor) Validate(msg *queue.Message) error {
	profileMsg, err := NewProfileMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to parse message for validation: %w", err)
	}
	return profileMsg.Validate()
}

// Type returns the processor type
func (p *Processor) Type() string {
	return "profile"
}

// HandleError handles processing errors. Returning a non-nil error here
// tells the consumer to nack the message WITHOUT requeue, so it lands on
// the profile-processing.dlq instead of being redelivered forever.
func (p *Processor) HandleError(ctx context.Context, msg *queue.Message, err error) error {
	log.Printf("profile processing error: %v", err)
	return err
}

// profileRow is the row a profile.task writes. Modeling the write as an
// upsert keyed on the profile id is what makes this processor the
// naturally-idempotent example (ADR-008.2): replaying the same task converges
// to the same row instead of accumulating side effects, so a duplicate
// delivery is harmless even if the Redis guard misses it. Contrast the email
// processor, whose "send" is not idempotent and leans on the guard alone.
type profileRow struct {
	ID     string
	Status string
	Source string
}

// runTask derives the target row from the task, then upserts it. Unrecognized
// task_type values still map to a generic processed state (forward compatible)
// rather than failing.
func (p *Processor) runTask(ctx context.Context, msg *ProfileMessage) error {
	var status string
	var delay time.Duration
	switch TaskType(msg.Payload.TaskType) {
	case TaskTypeSync:
		status, delay = "synced", 200*time.Millisecond
	case TaskTypeValidate:
		status, delay = "validated", 100*time.Millisecond
	case TaskTypeEnrich:
		status, delay = "enriched", 300*time.Millisecond
	default:
		log.Printf("unrecognized profile task_type %q for profile %s; handling generically", msg.Payload.TaskType, msg.Payload.ProfileID)
		status, delay = "processed", 100*time.Millisecond
	}

	return p.upsert(ctx, profileRow{
		ID:     msg.Payload.ProfileID,
		Status: status,
		Source: sourceOf(msg),
	}, delay)
}

// upsert simulates the idempotent write. The shape is the point:
//
//	INSERT INTO profiles (id, status, source, updated_at)
//	VALUES ($1, $2, $3, now())
//	ON CONFLICT (id) DO UPDATE
//	  SET status = EXCLUDED.status, source = EXCLUDED.source, updated_at = now();
//
// No profile store is wired up (per mission scope) — the log line stands in
// for the row write — but structuring it as last-writer-wins on a fixed
// primary key means replays are convergent, not additive.
func (p *Processor) upsert(ctx context.Context, row profileRow, delay time.Duration) error {
	log.Printf("upserting profile %s (status=%s, source=%s)", row.ID, row.Status, row.Source)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		log.Printf("profile %s upserted (status=%s)", row.ID, row.Status)
		return nil
	}
}

// sourceOf extracts an optional origin marker from the task payload.
func sourceOf(msg *ProfileMessage) string {
	if msg.Payload.Data != nil {
		if s, ok := msg.Payload.Data["source"].(string); ok && s != "" {
			return s
		}
	}
	return "unknown"
}
