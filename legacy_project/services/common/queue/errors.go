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

	// Configuration Errors
	ErrInvalidConfig = errors.New("invalid configuration")
)
