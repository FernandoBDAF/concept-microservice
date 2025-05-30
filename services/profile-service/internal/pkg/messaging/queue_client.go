package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// QueueClient handles communication with the queue service
type QueueClient struct {
	client  *http.Client
	baseURL string
	config  *QueueConfig
}

// QueueConfig holds the configuration for the queue client
type QueueConfig struct {
	URL       string
	Timeout   time.Duration
	Retries   int
	QueueName string
}

// QueueMessage represents a message to be sent to the queue
type QueueMessage struct {
	ID            string            `json:"id"`
	Type          string            `json:"type" validate:"required,oneof=profile_update cache_invalidation background_job"`
	Timestamp     string            `json:"timestamp"`
	CorrelationID string            `json:"correlation_id"`
	Payload       interface{}       `json:"payload"`
	Priority      int32             `json:"priority" validate:"min=0,max=9"`
	Headers       map[string]string `json:"headers"`
}

// NewQueueClient creates a new queue client instance
func NewQueueClient(config *QueueConfig) (*QueueClient, error) {
	if config == nil {
		return nil, fmt.Errorf("queue config cannot be nil")
	}

	client := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &QueueClient{
		client:  client,
		baseURL: config.URL,
		config:  config,
	}, nil
}

// PublishMessage sends a message to the queue service
func (c *QueueClient) PublishMessage(ctx context.Context, msg *QueueMessage) error {
	logger.LogInfo(ctx, "Preparing to publish message",
		zap.String("message_id", msg.ID),
		zap.String("message_type", msg.Type))

	// Set default values if not provided
	if msg.ID == "" {
		msg.ID = uuid.New().String()
		logger.LogInfo(ctx, "Generated new message ID",
			zap.String("message_id", msg.ID))
	}
	if msg.Timestamp == "" {
		msg.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if msg.Priority == 0 {
		msg.Priority = 1 // Default priority
	}
	if msg.Headers == nil {
		msg.Headers = make(map[string]string)
	}
	if msg.CorrelationID == "" {
		msg.CorrelationID = uuid.New().String()
		logger.LogInfo(ctx, "Generated new correlation ID",
			zap.String("correlation_id", msg.CorrelationID))
	}

	body, err := json.Marshal(msg)
	if err != nil {
		logger.LogError(ctx, "Failed to marshal message", err,
			zap.String("message_id", msg.ID))
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	logger.LogInfo(ctx, "Sending message to queue service",
		zap.String("message_id", msg.ID),
		zap.String("message_type", msg.Type),
		zap.String("url", c.baseURL+"/api/v1/queue/messages"))

	var lastErr error
	for attempt := 0; attempt < c.config.Retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/queue/messages", bytes.NewBuffer(body))
		if err != nil {
			logger.LogError(ctx, "Failed to create request", err,
				zap.String("message_id", msg.ID),
				zap.Int("attempt", attempt+1))
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: failed to send request: %w", attempt+1, err)
			logger.LogError(ctx, "Failed to send request", err,
				zap.String("message_id", msg.ID),
				zap.Int("attempt", attempt+1))
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logger.LogInfo(ctx, "Successfully published message",
				zap.String("message_id", msg.ID),
				zap.String("message_type", msg.Type),
				zap.Int("status_code", resp.StatusCode))
			return nil
		}

		body, _ := io.ReadAll(resp.Body)
		lastErr = fmt.Errorf("attempt %d: unexpected status code %d: %s", attempt+1, resp.StatusCode, string(body))
		logger.LogError(ctx, "Unexpected status code", lastErr,
			zap.String("message_id", msg.ID),
			zap.Int("attempt", attempt+1),
			zap.Int("status_code", resp.StatusCode),
			zap.String("response_body", string(body)))
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}

	logger.LogError(ctx, "All retry attempts failed", lastErr,
		zap.String("message_id", msg.ID),
		zap.String("message_type", msg.Type))
	return fmt.Errorf("all retry attempts failed: %w", lastErr)
}

// Close closes the queue client
func (c *QueueClient) Close() error {
	// No cleanup needed for HTTP client
	return nil
}

// GetQueueServiceURL returns the queue service URL
func (c *QueueClient) GetQueueServiceURL() string {
	return c.baseURL
}
