package queue

import "errors"

var (
	// Connection Errors
	ErrConnectionFailed = errors.New("failed to connect to RabbitMQ")
	ErrChannelFailed    = errors.New("failed to open channel")
	ErrConnectionClosed = errors.New("connection closed")

	// Publisher Errors
	ErrPublishFailed  = errors.New("failed to publish message")
	ErrPublishTimeout = errors.New("publish confirmation timeout")
	ErrInvalidMessage = errors.New("invalid message format")

	// Consumer Errors
	ErrConsumeFailed = errors.New("failed to consume message")
	ErrAckFailed     = errors.New("failed to acknowledge message")
	ErrNackFailed    = errors.New("failed to negative acknowledge message")
	ErrHandlerFailed = errors.New("message handler failed")

	// ErrUnretryable marks a processing failure that must skip the retry
	// tiers and go straight to the DLQ — retrying cannot help (bad envelope,
	// failed validation/unmarshal). Wrap it so errors.Is matches, e.g.
	// errors.Join(queue.ErrUnretryable, validationErr). Everything not wrapping
	// it is treated as retryable (transient) by the consumer (ADR-008.1).
	ErrUnretryable = errors.New("unretryable message")

	// Configuration Errors
	ErrInvalidConfig = errors.New("invalid configuration")
)
