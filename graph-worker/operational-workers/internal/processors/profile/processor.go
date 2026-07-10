package profile

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
	"github.com/fernandobarroso/microservices/operational-workers/internal/common/utils"
)

// Processor handles profile task processing
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

	if err := p.Validate(msg); err != nil {
		p.metrics.RecordProcessingError()
		return err
	}

	switch profileMsg.Payload.TaskType {
	case TaskTypeSync:
		err = p.handleSync(ctx, profileMsg)
	case TaskTypeValidate:
		err = p.handleValidate(ctx, profileMsg)
	case TaskTypeEnrich:
		err = p.handleEnrich(ctx, profileMsg)
	default:
		p.metrics.RecordProcessingError()
		return ErrInvalidTaskType
	}

	if err != nil {
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

// HandleError handles processing errors
func (p *Processor) HandleError(ctx context.Context, msg *queue.Message, err error) error {
	log.Printf("Profile processing error: %v", err)
	return err
}

func (p *Processor) handleSync(ctx context.Context, msg *ProfileMessage) error {
	log.Printf("Syncing profile %s", msg.Payload.ProfileID)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(200 * time.Millisecond):
		log.Printf("Profile synced: %s", msg.Payload.ProfileID)
		return nil
	}
}

func (p *Processor) handleValidate(ctx context.Context, msg *ProfileMessage) error {
	log.Printf("Validating profile %s", msg.Payload.ProfileID)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		log.Printf("Profile validated: %s", msg.Payload.ProfileID)
		return nil
	}
}

func (p *Processor) handleEnrich(ctx context.Context, msg *ProfileMessage) error {
	log.Printf("Enriching profile %s", msg.Payload.ProfileID)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(300 * time.Millisecond):
		log.Printf("Profile enriched: %s", msg.Payload.ProfileID)
		return nil
	}
}
