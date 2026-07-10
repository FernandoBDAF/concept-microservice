package domain

import (
	"errors"
	"time"

	"github.com/fernandobarroso/common/queue"
)

var (
	ErrInvalidImageURL       = errors.New("invalid image URL")
	ErrInvalidProcessingType = errors.New("invalid processing type")
	ErrInvalidPriority       = errors.New("invalid priority")
	ErrInvalidParameters     = errors.New("invalid processing parameters")
	ErrInvalidTimeout        = errors.New("invalid timeout value")
)

// ProcessingType represents the type of image processing to perform
type ProcessingType string

const (
	ProcessingTypeResize  ProcessingType = "resize"
	ProcessingTypeFilter  ProcessingType = "filter"
	ProcessingTypeAnalyze ProcessingType = "analyze"
)

// Priority represents the image processing priority
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"
)

// ImagePayload contains the image processing specific data
type ImagePayload struct {
	ImageURL       string                 `json:"image_url"`
	ProcessingType ProcessingType         `json:"processing_type"`
	Parameters     map[string]interface{} `json:"parameters"`
	CallbackURL    string                 `json:"callback_url"`
	TimeoutSeconds int                    `json:"timeout_seconds"`
	Priority       Priority               `json:"priority"`
}

// ImageMessage represents the full message structure expected
type ImageMessage struct {
	Type      string       `json:"type"`
	Payload   ImagePayload `json:"payload"`
	CreatedAt time.Time    `json:"created_at"`

	// Embed the queue message for access to ID, Timestamp, etc.
	queue.Message
}

// NewImageMessage creates a new image message from a queue message
func NewImageMessage(msg *queue.Message) (*ImageMessage, error) {
	var imageMsg ImageMessage

	// Unmarshal the entire payload to get the structured message
	if err := msg.UnmarshalPayload(&imageMsg); err != nil {
		return nil, err
	}

	// Copy the queue message properties
	imageMsg.Message = *msg

	return &imageMsg, nil
}

// Validate validates the image message
func (m *ImageMessage) Validate() error {
	if m.Type != "image" {
		return errors.New("message type must be 'image'")
	}

	if m.Payload.ImageURL == "" {
		return ErrInvalidImageURL
	}

	// Validate processing type
	switch m.Payload.ProcessingType {
	case ProcessingTypeResize, ProcessingTypeFilter, ProcessingTypeAnalyze:
		// Valid processing types
	default:
		return ErrInvalidProcessingType
	}

	// Validate priority
	switch m.Payload.Priority {
	case PriorityHigh, PriorityNormal, PriorityLow:
		// Valid priorities
	default:
		return ErrInvalidPriority
	}

	if m.Payload.Parameters == nil {
		return ErrInvalidParameters
	}

	// Validate timeout
	if m.Payload.TimeoutSeconds <= 0 || m.Payload.TimeoutSeconds > 600 {
		return ErrInvalidTimeout
	}

	return nil
}

// GetPriorityScore returns a numeric score for priority ordering
func (m *ImageMessage) GetPriorityScore() int {
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

// GetExpectedProcessingTime returns expected processing duration based on type and priority
func (m *ImageMessage) GetExpectedProcessingTime() time.Duration {
	baseDuration := m.getBaseProcessingTime()

	// Adjust based on priority
	switch m.Payload.Priority {
	case PriorityHigh:
		return baseDuration * 3 / 4 // 25% faster for high priority
	case PriorityLow:
		return baseDuration * 5 / 4 // 25% slower for low priority
	default:
		return baseDuration
	}
}

// getBaseProcessingTime returns base processing time for each operation type
func (m *ImageMessage) getBaseProcessingTime() time.Duration {
	switch m.Payload.ProcessingType {
	case ProcessingTypeResize:
		return 10 * time.Second // Resize is relatively fast
	case ProcessingTypeFilter:
		return 15 * time.Second // Filtering takes more time
	case ProcessingTypeAnalyze:
		return 25 * time.Second // Analysis is most resource intensive
	default:
		return 15 * time.Second
	}
}
