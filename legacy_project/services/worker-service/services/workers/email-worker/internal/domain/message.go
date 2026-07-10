package domain

import (
	"errors"
	"time"

	"github.com/fernandobarroso/common/queue"
)

var (
	ErrInvalidRecipient = errors.New("invalid recipient email")
	ErrInvalidEmailType = errors.New("invalid email type")
	ErrInvalidPriority  = errors.New("invalid priority")
	ErrInvalidData      = errors.New("invalid email data")
)

// EmailType represents the type of email to send
type EmailType string

const (
	EmailTypeWelcome      EmailType = "welcome"
	EmailTypeNotification EmailType = "notification"
	EmailTypeAlert        EmailType = "alert"
)

// Priority represents the email processing priority
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"
)

// EmailPayload contains the email-specific data
type EmailPayload struct {
	Recipient string         `json:"recipient"`
	EmailType EmailType      `json:"email_type"`
	Template  string         `json:"template"`
	Data      map[string]any `json:"data"`
	Priority  Priority       `json:"priority"`
}

// EmailMessage wraps the common queue message with email-specific payload
type EmailMessage struct {
	queue.Message
	Type      string       `json:"type"`
	Payload   EmailPayload `json:"payload"`
	CreatedAt time.Time    `json:"created_at"`
}

// NewEmailMessage creates a new email message from a queue message
func NewEmailMessage(msg *queue.Message) (*EmailMessage, error) {
	var emailMsg EmailMessage
	if err := msg.UnmarshalPayload(&emailMsg); err != nil {
		return nil, err
	}

	// Copy the queue message properties
	emailMsg.Message = *msg

	return &emailMsg, nil
}

// Validate validates the email message
func (m *EmailMessage) Validate() error {
	if m.Type != "email" {
		return errors.New("message type must be 'email'")
	}

	if m.Payload.Recipient == "" {
		return ErrInvalidRecipient
	}

	// Validate email type
	switch m.Payload.EmailType {
	case EmailTypeWelcome, EmailTypeNotification, EmailTypeAlert:
		// Valid email types
	default:
		return ErrInvalidEmailType
	}

	// Validate priority
	switch m.Payload.Priority {
	case PriorityHigh, PriorityNormal, PriorityLow:
		// Valid priorities
	default:
		return ErrInvalidPriority
	}

	if m.Payload.Data == nil {
		return ErrInvalidData
	}

	return nil
}

// GetPriorityScore returns a numeric score for priority ordering
func (m *EmailMessage) GetPriorityScore() int {
	switch m.Payload.Priority {
	case PriorityHigh:
		return 3
	case PriorityNormal:
		return 2
	case PriorityLow:
		return 1
	default:
		return 0
	}
}
