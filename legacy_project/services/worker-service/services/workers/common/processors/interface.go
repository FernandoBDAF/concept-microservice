package processors

import (
	"context"

	"github.com/fernandobarroso/common/queue"
)

// MessageProcessor defines the interface for processing messages
// This is a generalized version of the domain.Processor interface
type MessageProcessor interface {
	// Process handles the business logic for a message
	Process(ctx context.Context, msg *queue.Message) error

	// Validate ensures the message is valid before processing
	Validate(msg *queue.Message) error

	// Type returns the processor type for identification
	Type() string

	// HandleError provides custom error handling logic
	HandleError(ctx context.Context, msg *queue.Message, err error) error
}

// ProcessingResult represents the result of message processing
type ProcessingResult struct {
	Success bool
	Error   error
	Metrics map[string]float64
}
