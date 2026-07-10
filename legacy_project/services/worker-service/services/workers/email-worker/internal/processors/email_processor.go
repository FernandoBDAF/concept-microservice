package processors

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fernandobarroso/common/queue"
	"github.com/fernandobarroso/workers/common/utils"
	"github.com/fernandobarroso/workers/email-worker/internal/domain"
)

// EmailProcessor handles email message processing
type EmailProcessor struct {
	metrics *utils.ProcessorMetrics
}

// NewEmailProcessor creates a new email processor
func NewEmailProcessor() *EmailProcessor {
	return &EmailProcessor{
		metrics: utils.NewProcessorMetrics("email"),
	}
}

// Process processes an email message
func (p *EmailProcessor) Process(ctx context.Context, msg *queue.Message) error {
	// Start metrics tracking
	timer := p.metrics.StartTimer()
	defer timer.ObserveDuration()
	p.metrics.RecordProcessingStart()

	// Convert to email message
	emailMsg, err := domain.NewEmailMessage(msg)
	if err != nil {
		p.metrics.RecordProcessingError()
		return fmt.Errorf("failed to parse email message: %w", err)
	}

	// Validate the message
	if err := p.Validate(msg); err != nil {
		p.metrics.RecordProcessingError()
		return err
	}

	// Process based on email type
	switch emailMsg.Payload.EmailType {
	case domain.EmailTypeWelcome:
		err = p.sendWelcomeEmail(ctx, emailMsg)
	case domain.EmailTypeNotification:
		err = p.sendNotificationEmail(ctx, emailMsg)
	case domain.EmailTypeAlert:
		err = p.sendAlertEmail(ctx, emailMsg)
	default:
		p.metrics.RecordProcessingError()
		return domain.ErrInvalidEmailType
	}

	if err != nil {
		p.metrics.RecordProcessingError()
		return err
	}

	p.metrics.RecordProcessingSuccess()
	return nil
}

// Validate validates the message
func (p *EmailProcessor) Validate(msg *queue.Message) error {
	emailMsg, err := domain.NewEmailMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to parse message for validation: %w", err)
	}
	return emailMsg.Validate()
}

// Type returns the processor type
func (p *EmailProcessor) Type() string {
	return "email"
}

// HandleError handles processing errors
func (p *EmailProcessor) HandleError(ctx context.Context, msg *queue.Message, err error) error {
	log.Printf("Email processing error: %v", err)

	// Could implement retry logic here
	// For now, just return the error which will cause message requeue
	return err
}

// sendWelcomeEmail simulates sending a welcome email
func (p *EmailProcessor) sendWelcomeEmail(ctx context.Context, msg *domain.EmailMessage) error {
	log.Printf("📧 Sending WELCOME email to %s (Priority: %s)",
		msg.Payload.Recipient, msg.Payload.Priority)

	// Simulate email service call with different delays based on priority
	delay := p.getProcessingDelay(msg.Payload.Priority)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		log.Printf("✅ Welcome email sent successfully to %s", msg.Payload.Recipient)
		return nil
	}
}

// sendNotificationEmail simulates sending a notification email
func (p *EmailProcessor) sendNotificationEmail(ctx context.Context, msg *domain.EmailMessage) error {
	log.Printf("📧 Sending NOTIFICATION email to %s (Priority: %s)",
		msg.Payload.Recipient, msg.Payload.Priority)

	// Simulate email service call
	delay := p.getProcessingDelay(msg.Payload.Priority)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		log.Printf("✅ Notification email sent successfully to %s", msg.Payload.Recipient)
		return nil
	}
}

// sendAlertEmail simulates sending an alert email
func (p *EmailProcessor) sendAlertEmail(ctx context.Context, msg *domain.EmailMessage) error {
	log.Printf("🚨 Sending ALERT email to %s (Priority: %s)",
		msg.Payload.Recipient, msg.Payload.Priority)

	// Alert emails are processed faster
	delay := p.getProcessingDelay(msg.Payload.Priority) / 2 // Alerts are faster

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		log.Printf("✅ Alert email sent successfully to %s", msg.Payload.Recipient)
		return nil
	}
}

// getProcessingDelay returns the processing delay based on priority
func (p *EmailProcessor) getProcessingDelay(priority domain.Priority) time.Duration {
	switch priority {
	case domain.PriorityHigh:
		return 2 * time.Second // High priority emails are faster
	case domain.PriorityNormal:
		return 5 * time.Second // Normal processing time
	case domain.PriorityLow:
		return 8 * time.Second // Low priority can take longer
	default:
		return 5 * time.Second
	}
}
