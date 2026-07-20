package email

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
	"github.com/fernandobarroso/microservices/operational-workers/internal/common/utils"
)

// EmailProcessor handles email message processing. No real SMTP/email
// service is wired up (per mission scope) — Process simulates the send
// and logs coherently so behavior is observable end-to-end.
type EmailProcessor struct {
	metrics *utils.ProcessorMetrics

	// failFirstN is the FAIL_FIRST_N_ATTEMPTS test hook (A3/EXP-40): while a
	// message's attempt count is below this value the simulated send fails
	// (retryably), letting the retry tiers be exercised without Chaos Mesh; at
	// attempt >= N it succeeds ("dependency recovered"). 0 disables the hook.
	failFirstN int
}

// NewEmailProcessor creates a new email processor
func NewEmailProcessor() *EmailProcessor {
	return &EmailProcessor{
		metrics:    utils.NewProcessorMetrics("email"),
		failFirstN: utils.GetEnvIntOrDefault("FAIL_FIRST_N_ATTEMPTS", 0),
	}
}

// Process processes an email message
func (p *EmailProcessor) Process(ctx context.Context, msg *queue.Message) error {
	timer := p.metrics.StartTimer()
	defer timer.ObserveDuration()
	p.metrics.RecordProcessingStart()

	emailMsg, err := NewEmailMessage(msg)
	if err != nil {
		p.metrics.RecordProcessingError()
		return fmt.Errorf("failed to parse email message: %w", err)
	}

	if err := emailMsg.Validate(); err != nil {
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

	if err := p.sendEmail(ctx, emailMsg); err != nil {
		p.metrics.RecordProcessingError()
		return err
	}

	p.metrics.RecordProcessingSuccess()
	return nil
}

// Validate validates the message
func (p *EmailProcessor) Validate(msg *queue.Message) error {
	emailMsg, err := NewEmailMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to parse message for validation: %w", err)
	}
	return emailMsg.Validate()
}

// Type returns the processor type
func (p *EmailProcessor) Type() string {
	return "email"
}

// HandleError handles processing errors. Returning a non-nil error here
// tells the consumer to nack the message WITHOUT requeue, so it lands on
// the email-processing.dlq instead of being redelivered forever.
func (p *EmailProcessor) HandleError(ctx context.Context, msg *queue.Message, err error) error {
	log.Printf("email processing error: %v", err)
	return err
}

// sendEmail simulates dispatching the email via the appropriate template
// flow. Unrecognized email_type values still get a generic simulated send
// (forward compatible) rather than failing.
func (p *EmailProcessor) sendEmail(ctx context.Context, msg *EmailMessage) error {
	delay := 150 * time.Millisecond

	switch EmailType(msg.Payload.EmailType) {
	case EmailTypeWelcome:
		log.Printf("sending WELCOME email to %s (template=%s)", msg.Payload.Recipient, msg.Payload.TemplateID)
	case EmailTypeNotification:
		log.Printf("sending NOTIFICATION email to %s (subject=%q)", msg.Payload.Recipient, msg.Payload.Subject)
	case EmailTypeAlert:
		log.Printf("sending ALERT email to %s (subject=%q)", msg.Payload.Recipient, msg.Payload.Subject)
		delay = delay / 2 // alerts go out faster
	default:
		log.Printf("sending email (type=%s) to %s", msg.Payload.EmailType, msg.Payload.Recipient)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		log.Printf("email sent successfully to %s", msg.Payload.Recipient)
		return nil
	}
}
