// Package example is the one reference processor shipped with the template.
// It implements processors.MessageProcessor with the canonical shape —
// parse+validate the payload, simulate the unit of work, return an error the
// consumer can classify (retryable vs unretryable). Replace it wholesale with
// your domain processor when adapting: keep Validate/Process/Type/HandleError,
// swap the body of doWork.
package example

import (
	"context"
	"fmt"
	"log"
	"time"

	"example.com/worker/internal/common/queue"
	"example.com/worker/internal/common/utils"
)

// Processor is the example MessageProcessor. It performs no real side effect —
// doWork simulates latency and logs — so the template runs end-to-end without
// external dependencies.
type Processor struct {
	metrics *utils.ProcessorMetrics

	// failFirstN is the FAIL_FIRST_N_ATTEMPTS test hook (EXP-40; also drives
	// EXP-81's deliberately-broken bootstrap): while a message's attempt count
	// is below this value the simulated work fails retryably, exercising the
	// 5s/30s/2m retry tiers without a fault injector; at attempt >= N it
	// succeeds ("dependency recovered"). 0 disables the hook.
	failFirstN int
}

// NewProcessor creates the example processor.
func NewProcessor() *Processor {
	return &Processor{
		metrics:    utils.NewProcessorMetrics("example"),
		failFirstN: utils.GetEnvIntOrDefault("FAIL_FIRST_N_ATTEMPTS", 0),
	}
}

// Process runs the business logic for one message.
func (p *Processor) Process(ctx context.Context, msg *queue.Message) error {
	timer := p.metrics.StartTimer()
	defer timer.ObserveDuration()
	p.metrics.RecordProcessingStart()

	task, err := NewTask(msg)
	if err != nil {
		p.metrics.RecordProcessingError()
		return fmt.Errorf("failed to parse task payload: %w", err)
	}

	if err := task.Validate(); err != nil {
		p.metrics.RecordProcessingError()
		return err
	}

	// FAIL_FIRST_N_ATTEMPTS test hook (inert unless set): simulate a flaky
	// downstream that fails the first N attempts, then recovers. The returned
	// error is a plain (retryable) error, so the consumer routes it through the
	// 5s/30s/2m retry tiers (ADR-008.1); msg.Attempt is the x-death count the
	// consumer threaded onto the message.
	if p.failFirstN > 0 && msg.Attempt < p.failFirstN {
		p.metrics.RecordProcessingError()
		return fmt.Errorf("simulated transient failure (FAIL_FIRST_N_ATTEMPTS=%d, attempt=%d)", p.failFirstN, msg.Attempt)
	}

	if err := p.doWork(ctx, task); err != nil {
		p.metrics.RecordProcessingError()
		return err
	}

	p.metrics.RecordProcessingSuccess()
	return nil
}

// Validate is the cheap pre-check the consumer runs before the idempotency
// guard; a failure here is treated as unretryable (routed straight to the DLQ).
func (p *Processor) Validate(msg *queue.Message) error {
	task, err := NewTask(msg)
	if err != nil {
		return fmt.Errorf("failed to parse message for validation: %w", err)
	}
	return task.Validate()
}

// Type returns the processor type used in per-worker metric names.
func (p *Processor) Type() string { return "example" }

// HandleError is the processor's error hook. Returning the error unchanged
// lets the consumer classify and route it (retry tier vs DLQ). Wrap the error
// with queue.ErrUnretryable here if a specific failure must skip the retry
// tiers.
func (p *Processor) HandleError(ctx context.Context, msg *queue.Message, err error) error {
	log.Printf("example processing error: %v", err)
	return err
}

// doWork simulates the unit of work. Replace the body with your real side
// effect; keep the ctx.Done() check so an in-flight message can observe
// shutdown cleanly.
func (p *Processor) doWork(ctx context.Context, task *Task) error {
	log.Printf("processing example.task target=%s", task.Payload.Target)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		log.Printf("example.task target=%s done", task.Payload.Target)
		return nil
	}
}
