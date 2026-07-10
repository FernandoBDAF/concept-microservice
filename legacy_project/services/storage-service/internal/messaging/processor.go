package messaging

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

var (
	ErrInvalidMessage    = errors.New("invalid message format")
	ErrUnknownRoutingKey = errors.New("unknown routing key")
	ErrProcessingTimeout = errors.New("message processing timeout")
	ErrHandlerNotFound   = errors.New("no handler found for message type")
)

// MessageHandler defines the interface for handling specific message types
type MessageHandler interface {
	Handle(ctx context.Context, msg *Message) (*MessageResponse, error)
	CanHandle(routingKey string) bool
	GetSupportedRoutingKeys() []string
}

// MessageProcessor processes incoming messages and routes them to appropriate handlers
type MessageProcessor struct {
	handlers       map[string]MessageHandler
	log            *zap.Logger
	processTimeout time.Duration
	maxRetries     int
	retryBackoff   time.Duration
}

// NewMessageProcessor creates a new message processor
func NewMessageProcessor() *MessageProcessor {
	return &MessageProcessor{
		handlers:       make(map[string]MessageHandler),
		log:            zap.L().Named("message_processor"),
		processTimeout: 30 * time.Second,
		maxRetries:     3,
		retryBackoff:   1 * time.Second,
	}
}

// RegisterHandler registers a message handler for specific routing keys
func (p *MessageProcessor) RegisterHandler(handler MessageHandler) error {
	supportedKeys := handler.GetSupportedRoutingKeys()
	if len(supportedKeys) == 0 {
		return fmt.Errorf("handler must support at least one routing key")
	}

	for _, key := range supportedKeys {
		if existingHandler, exists := p.handlers[key]; exists {
			p.log.Warn("Overwriting existing handler for routing key",
				zap.String("routing_key", key),
				zap.String("existing_handler", fmt.Sprintf("%T", existingHandler)),
				zap.String("new_handler", fmt.Sprintf("%T", handler)))
		}
		p.handlers[key] = handler
	}

	p.log.Info("Registered message handler",
		zap.String("handler", fmt.Sprintf("%T", handler)),
		zap.Strings("routing_keys", supportedKeys))

	return nil
}

// ProcessMessage processes a single message by routing it to the appropriate handler
func (p *MessageProcessor) ProcessMessage(ctx context.Context, msg *Message) (*MessageResponse, error) {
	startTime := time.Now()

	// Validate message
	if err := p.validateMessage(msg); err != nil {
		p.log.Error("Invalid message received",
			zap.String("message_id", msg.ID),
			zap.Error(err))
		return p.createErrorResponse(msg.ID, err, startTime), err
	}

	// Check if message is expired
	if msg.IsExpired(24 * time.Hour) {
		err := fmt.Errorf("message expired: %s", msg.ID)
		p.log.Warn("Expired message received",
			zap.String("message_id", msg.ID),
			zap.Time("timestamp", msg.Timestamp))
		return p.createErrorResponse(msg.ID, err, startTime), err
	}

	// Find appropriate handler
	handler, exists := p.handlers[msg.RoutingKey]
	if !exists {
		err := fmt.Errorf("%w: %s", ErrUnknownRoutingKey, msg.RoutingKey)
		p.log.Error("No handler found for routing key",
			zap.String("message_id", msg.ID),
			zap.String("routing_key", msg.RoutingKey))
		return p.createErrorResponse(msg.ID, err, startTime), err
	}

	// Create context with timeout
	processCtx, cancel := context.WithTimeout(ctx, p.processTimeout)
	defer cancel()

	// Log processing start
	p.log.Info("Processing message",
		zap.String("message_id", msg.ID),
		zap.String("routing_key", msg.RoutingKey),
		zap.String("type", msg.Type),
		zap.Int("retry_count", msg.RetryCount))

	// Process message with handler
	response, err := handler.Handle(processCtx, msg)
	if err != nil {
		p.log.Error("Message processing failed",
			zap.String("message_id", msg.ID),
			zap.String("routing_key", msg.RoutingKey),
			zap.Error(err),
			zap.Duration("processing_time", time.Since(startTime)))

		if response == nil {
			response = p.createErrorResponse(msg.ID, err, startTime)
		}
		return response, err
	}

	// Log successful processing
	processingTime := time.Since(startTime)
	p.log.Info("Message processed successfully",
		zap.String("message_id", msg.ID),
		zap.String("routing_key", msg.RoutingKey),
		zap.Duration("processing_time", processingTime))

	// Ensure response has required fields
	if response.MessageID == "" {
		response.MessageID = msg.ID
	}
	if response.ProcessedAt.IsZero() {
		response.ProcessedAt = time.Now()
	}
	if response.ProcessingTime == 0 {
		response.ProcessingTime = processingTime
	}

	return response, nil
}

// GetSupportedRoutingKeys returns all supported routing keys
func (p *MessageProcessor) GetSupportedRoutingKeys() []string {
	keys := make([]string, 0, len(p.handlers))
	for key := range p.handlers {
		keys = append(keys, key)
	}
	return keys
}

// GetHandlerCount returns the number of registered handlers
func (p *MessageProcessor) GetHandlerCount() int {
	return len(p.handlers)
}

// validateMessage validates the message format and required fields
func (p *MessageProcessor) validateMessage(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("%w: message is nil", ErrInvalidMessage)
	}

	if msg.ID == "" {
		return fmt.Errorf("%w: missing message ID", ErrInvalidMessage)
	}

	if msg.Type == "" {
		return fmt.Errorf("%w: missing message type", ErrInvalidMessage)
	}

	if msg.RoutingKey == "" {
		return fmt.Errorf("%w: missing routing key", ErrInvalidMessage)
	}

	if len(msg.Payload) == 0 {
		return fmt.Errorf("%w: empty payload", ErrInvalidMessage)
	}

	if msg.Timestamp.IsZero() {
		return fmt.Errorf("%w: missing timestamp", ErrInvalidMessage)
	}

	return nil
}

// createErrorResponse creates a standardized error response
func (p *MessageProcessor) createErrorResponse(messageID string, err error, startTime time.Time) *MessageResponse {
	return &MessageResponse{
		MessageID:      messageID,
		Success:        false,
		Error:          err.Error(),
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}
}

// ProcessorStats holds statistics about the message processor
type ProcessorStats struct {
	RegisteredHandlers int               `json:"registered_handlers"`
	SupportedKeys      []string          `json:"supported_keys"`
	ProcessTimeout     time.Duration     `json:"process_timeout"`
	MaxRetries         int               `json:"max_retries"`
	RetryBackoff       time.Duration     `json:"retry_backoff"`
	HandlerDetails     map[string]string `json:"handler_details"`
}

// GetStats returns processor statistics
func (p *MessageProcessor) GetStats() *ProcessorStats {
	handlerDetails := make(map[string]string)
	for key, handler := range p.handlers {
		handlerDetails[key] = fmt.Sprintf("%T", handler)
	}

	return &ProcessorStats{
		RegisteredHandlers: len(p.handlers),
		SupportedKeys:      p.GetSupportedRoutingKeys(),
		ProcessTimeout:     p.processTimeout,
		MaxRetries:         p.maxRetries,
		RetryBackoff:       p.retryBackoff,
		HandlerDetails:     handlerDetails,
	}
}
