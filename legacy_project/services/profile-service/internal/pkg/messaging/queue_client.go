package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ✅ NEW: Error types for enhanced error handling
var (
	ErrQueueServiceUnavailable = errors.New("queue service is unavailable")
	ErrCircuitBreakerOpen      = errors.New("circuit breaker is open")
	ErrRequestTimeout          = errors.New("request timeout exceeded")
	ErrInvalidRoutingKey       = errors.New("invalid routing key")
	ErrPayloadTooLarge         = errors.New("payload size exceeds limit")
	ErrRateLimited             = errors.New("rate limited by queue service")
)

// ✅ NEW: Circuit breaker states
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// ✅ NEW: Circuit breaker implementation
type CircuitBreaker struct {
	mu                    sync.RWMutex
	state                 CircuitBreakerState
	failureCount          int
	failureThreshold      int
	recoveryTimeout       time.Duration
	maxConcurrentRequests int
	lastFailureTime       time.Time
	currentRequests       int
}

// ✅ NEW: Enhanced error with context and retry information
type QueueError struct {
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	RoutingKey string    `json:"routing_key,omitempty"`
	TaskType   string    `json:"task_type,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Retryable  bool      `json:"retryable"`
	Attempt    int       `json:"attempt,omitempty"`
	Err        error     `json:"-"`
}

func (e *QueueError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *QueueError) Unwrap() error {
	return e.Err
}

// ✅ NEW: Retry policy configuration
type RetryPolicy struct {
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// QueueClient handles communication with the queue service
type QueueClient struct {
	client         *http.Client
	baseURL        string
	config         *QueueConfig
	circuitBreaker *CircuitBreaker
	retryPolicy    RetryPolicy
	metrics        *QueueMetrics
}

// ✅ NEW: Queue metrics for monitoring
type QueueMetrics struct {
	mu                  sync.RWMutex
	totalRequests       int64
	successfulRequests  int64
	failedRequests      int64
	circuitBreakerOpens int64
	averageResponseTime time.Duration
	lastRequestTime     time.Time
}

// QueueConfig holds the configuration for the queue client
type QueueConfig struct {
	URL                   string
	Timeout               time.Duration
	Retries               int
	MaxRequestSize        int64
	CircuitBreakerEnabled bool
	FailureThreshold      int
	RecoveryTimeout       time.Duration
	MaxConcurrentRequests int
}

// QueueMessage represents a message to be sent to the queue - Updated for queue-service compatibility
type QueueMessage struct {
	ID         string            `json:"id"`
	Type       string            `json:"type" validate:"required,oneof=profile_update email_notification image_processing"`
	Payload    json.RawMessage   `json:"payload"`     // ✅ Changed from interface{} to json.RawMessage
	Timestamp  time.Time         `json:"timestamp"`   // ✅ Changed from string to time.Time
	Metadata   map[string]string `json:"metadata"`    // ✅ Changed from Headers to Metadata
	RoutingKey string            `json:"routing_key"` // ✅ NEW - Required for multi-worker routing
}

// NewQueueClient creates a new queue client instance
func NewQueueClient(config *QueueConfig) (*QueueClient, error) {
	if config == nil {
		return nil, fmt.Errorf("queue config cannot be nil")
	}

	// ✅ Enhanced HTTP client with timeouts and connection pooling
	client := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   false,
		},
	}

	// ✅ Initialize circuit breaker if enabled
	var circuitBreaker *CircuitBreaker
	if config.CircuitBreakerEnabled {
		circuitBreaker = &CircuitBreaker{
			state:                 CircuitBreakerClosed,
			failureThreshold:      config.FailureThreshold,
			recoveryTimeout:       config.RecoveryTimeout,
			maxConcurrentRequests: config.MaxConcurrentRequests,
		}
	}

	// ✅ Default retry policy
	retryPolicy := RetryPolicy{
		MaxRetries:    config.Retries,
		InitialDelay:  1 * time.Second,
		MaxDelay:      60 * time.Second,
		BackoffFactor: 2.0,
	}

	return &QueueClient{
		client:         client,
		baseURL:        config.URL,
		config:         config,
		circuitBreaker: circuitBreaker,
		retryPolicy:    retryPolicy,
		metrics:        &QueueMetrics{},
	}, nil
}

// ✅ ENHANCED: PublishMessage with circuit breaker and advanced retry logic
func (c *QueueClient) PublishMessage(ctx context.Context, msg *QueueMessage) error {
	startTime := time.Now()
	defer func() {
		c.updateMetrics(time.Since(startTime))
	}()

	// Validate message
	if err := c.validateMessage(msg); err != nil {
		return &QueueError{
			Code:       "VALIDATION_ERROR",
			Message:    "Message validation failed",
			RoutingKey: msg.RoutingKey,
			TaskType:   msg.Type,
			Timestamp:  time.Now(),
			Retryable:  false,
			Err:        err,
		}
	}

	// Check circuit breaker
	if c.circuitBreaker != nil {
		if err := c.checkCircuitBreaker(ctx); err != nil {
			return &QueueError{
				Code:       "CIRCUIT_BREAKER_OPEN",
				Message:    "Circuit breaker is open",
				RoutingKey: msg.RoutingKey,
				TaskType:   msg.Type,
				Timestamp:  time.Now(),
				Retryable:  true,
				Err:        err,
			}
		}
	}

	logger.LogInfo(ctx, "Enhanced message publishing initiated",
		zap.String("message_id", msg.ID),
		zap.String("message_type", msg.Type),
		zap.String("routing_key", msg.RoutingKey),
		zap.Bool("circuit_breaker_enabled", c.circuitBreaker != nil))

	// Set default values if not provided
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now().UTC()
	}
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]string)
	}

	// Add tracing metadata
	msg.Metadata["client"] = "profile-service"
	msg.Metadata["attempt_time"] = time.Now().Format(time.RFC3339)

	// Execute with retry logic
	return c.executeWithRetry(ctx, msg)
}

// ✅ NEW: Execute request with advanced retry logic
func (c *QueueClient) executeWithRetry(ctx context.Context, msg *QueueMessage) error {
	var lastErr error

	for attempt := 0; attempt <= c.retryPolicy.MaxRetries; attempt++ {
		// Update metadata with attempt info
		msg.Metadata["attempt"] = fmt.Sprintf("%d", attempt+1)
		msg.Metadata["max_attempts"] = fmt.Sprintf("%d", c.retryPolicy.MaxRetries+1)

		err := c.executeRequest(ctx, msg, attempt+1)
		if err == nil {
			// Success - reset circuit breaker failure count
			if c.circuitBreaker != nil {
				c.circuitBreaker.onSuccess()
			}

			logger.LogInfo(ctx, "Message published successfully",
				zap.String("message_id", msg.ID),
				zap.String("routing_key", msg.RoutingKey),
				zap.Int("attempts", attempt+1))

			c.metrics.mu.Lock()
			c.metrics.successfulRequests++
			c.metrics.mu.Unlock()

			return nil
		}

		lastErr = err

		// Record failure
		if c.circuitBreaker != nil {
			c.circuitBreaker.onFailure()
		}

		// Check if error is retryable
		if queueErr, ok := err.(*QueueError); ok && !queueErr.Retryable {
			logger.LogError(ctx, "Non-retryable error occurred", err,
				zap.String("message_id", msg.ID),
				zap.String("error_code", queueErr.Code))
			break
		}

		// Don't retry on the last attempt
		if attempt == c.retryPolicy.MaxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := c.calculateRetryDelay(attempt)

		logger.LogWarn(ctx, "Request failed, retrying",
			zap.String("message_id", msg.ID),
			zap.Int("attempt", attempt+1),
			zap.Duration("retry_delay", delay),
			zap.Error(err))

		// Wait before retry
		select {
		case <-ctx.Done():
			return &QueueError{
				Code:       "CONTEXT_CANCELLED",
				Message:    "Context was cancelled during retry",
				RoutingKey: msg.RoutingKey,
				TaskType:   msg.Type,
				Timestamp:  time.Now(),
				Retryable:  false,
				Err:        ctx.Err(),
			}
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	c.metrics.mu.Lock()
	c.metrics.failedRequests++
	c.metrics.mu.Unlock()

	logger.LogError(ctx, "All retry attempts exhausted",
		lastErr,
		zap.String("message_id", msg.ID),
		zap.String("routing_key", msg.RoutingKey),
		zap.Int("total_attempts", c.retryPolicy.MaxRetries+1))

	return lastErr
}

// ✅ NEW: Execute single request attempt
func (c *QueueClient) executeRequest(ctx context.Context, msg *QueueMessage, attempt int) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return &QueueError{
			Code:       "SERIALIZATION_ERROR",
			Message:    "Failed to serialize message",
			RoutingKey: msg.RoutingKey,
			TaskType:   msg.Type,
			Timestamp:  time.Now(),
			Retryable:  false,
			Attempt:    attempt,
			Err:        err,
		}
	}

	// Check payload size
	if int64(len(body)) > c.config.MaxRequestSize {
		return &QueueError{
			Code:       "PAYLOAD_TOO_LARGE",
			Message:    fmt.Sprintf("Payload size %d exceeds limit %d", len(body), c.config.MaxRequestSize),
			RoutingKey: msg.RoutingKey,
			TaskType:   msg.Type,
			Timestamp:  time.Now(),
			Retryable:  false,
			Attempt:    attempt,
			Err:        ErrPayloadTooLarge,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/queue/messages", bytes.NewBuffer(body))
	if err != nil {
		return &QueueError{
			Code:       "REQUEST_CREATION_ERROR",
			Message:    "Failed to create HTTP request",
			RoutingKey: msg.RoutingKey,
			TaskType:   msg.Type,
			Timestamp:  time.Now(),
			Retryable:  false,
			Attempt:    attempt,
			Err:        err,
		}
	}

	// Set enhanced headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client", "profile-service")
	req.Header.Set("X-Message-ID", msg.ID)
	req.Header.Set("X-Routing-Key", msg.RoutingKey)
	req.Header.Set("X-Attempt", fmt.Sprintf("%d", attempt))

	resp, err := c.client.Do(req)
	if err != nil {
		return &QueueError{
			Code:       "NETWORK_ERROR",
			Message:    "Network request failed",
			RoutingKey: msg.RoutingKey,
			TaskType:   msg.Type,
			Timestamp:  time.Now(),
			Retryable:  true,
			Attempt:    attempt,
			Err:        err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil // Success
	}

	// Handle different HTTP status codes
	responseBody, _ := io.ReadAll(resp.Body)
	return c.createErrorFromResponse(resp.StatusCode, responseBody, msg, attempt)
}

// ✅ NEW: Create appropriate error based on HTTP response
func (c *QueueClient) createErrorFromResponse(statusCode int, body []byte, msg *QueueMessage, attempt int) error {
	baseError := &QueueError{
		RoutingKey: msg.RoutingKey,
		TaskType:   msg.Type,
		Timestamp:  time.Now(),
		Attempt:    attempt,
	}

	switch {
	case statusCode >= 400 && statusCode < 500:
		// Client errors - generally not retryable
		baseError.Retryable = false
		switch statusCode {
		case 400:
			baseError.Code = "BAD_REQUEST"
			baseError.Message = "Invalid request format"
		case 401:
			baseError.Code = "UNAUTHORIZED"
			baseError.Message = "Authentication required"
		case 403:
			baseError.Code = "FORBIDDEN"
			baseError.Message = "Access denied"
		case 404:
			baseError.Code = "NOT_FOUND"
			baseError.Message = "Queue service endpoint not found"
		case 422:
			baseError.Code = "UNPROCESSABLE_ENTITY"
			baseError.Message = "Invalid message content"
		case 429:
			baseError.Code = "RATE_LIMITED"
			baseError.Message = "Rate limit exceeded"
			baseError.Retryable = true // Rate limits are retryable
			baseError.Err = ErrRateLimited
		default:
			baseError.Code = "CLIENT_ERROR"
			baseError.Message = fmt.Sprintf("Client error (status: %d)", statusCode)
		}
	case statusCode >= 500:
		// Server errors - generally retryable
		baseError.Retryable = true
		switch statusCode {
		case 500:
			baseError.Code = "INTERNAL_SERVER_ERROR"
			baseError.Message = "Queue service internal error"
		case 502:
			baseError.Code = "BAD_GATEWAY"
			baseError.Message = "Queue service gateway error"
		case 503:
			baseError.Code = "SERVICE_UNAVAILABLE"
			baseError.Message = "Queue service temporarily unavailable"
			baseError.Err = ErrQueueServiceUnavailable
		case 504:
			baseError.Code = "GATEWAY_TIMEOUT"
			baseError.Message = "Queue service timeout"
			baseError.Err = ErrRequestTimeout
		default:
			baseError.Code = "SERVER_ERROR"
			baseError.Message = fmt.Sprintf("Server error (status: %d)", statusCode)
		}
	default:
		baseError.Code = "UNKNOWN_ERROR"
		baseError.Message = fmt.Sprintf("Unexpected status code: %d", statusCode)
		baseError.Retryable = false
	}

	// Add response body if available
	if len(body) > 0 {
		baseError.Message += fmt.Sprintf(" - Response: %s", string(body))
	}

	return baseError
}

// ✅ NEW: Message validation
func (c *QueueClient) validateMessage(msg *QueueMessage) error {
	if msg == nil {
		return errors.New("message cannot be nil")
	}

	if msg.Type == "" {
		return errors.New("message type is required")
	}

	if msg.RoutingKey == "" {
		return errors.New("routing key is required")
	}

	if len(msg.Payload) == 0 {
		return errors.New("message payload is required")
	}

	// Validate routing key format
	validRoutingKeys := []string{"profile.task", "email.send", "image.process"}
	isValid := false
	for _, validKey := range validRoutingKeys {
		if msg.RoutingKey == validKey {
			isValid = true
			break
		}
	}

	if !isValid {
		return &QueueError{
			Code:       "INVALID_ROUTING_KEY",
			Message:    fmt.Sprintf("Invalid routing key: %s", msg.RoutingKey),
			RoutingKey: msg.RoutingKey,
			TaskType:   msg.Type,
			Timestamp:  time.Now(),
			Retryable:  false,
			Err:        ErrInvalidRoutingKey,
		}
	}

	return nil
}

// ✅ NEW: Circuit breaker implementation
func (c *CircuitBreaker) onSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failureCount = 0
	if c.state == CircuitBreakerHalfOpen {
		c.state = CircuitBreakerClosed
	}
}

func (c *CircuitBreaker) onFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failureCount++
	c.lastFailureTime = time.Now()

	if c.failureCount >= c.failureThreshold {
		c.state = CircuitBreakerOpen
	}
}

func (c *QueueClient) checkCircuitBreaker(ctx context.Context) error {
	if c.circuitBreaker == nil {
		return nil
	}

	c.circuitBreaker.mu.RLock()
	state := c.circuitBreaker.state
	lastFailure := c.circuitBreaker.lastFailureTime
	currentRequests := c.circuitBreaker.currentRequests
	maxConcurrent := c.circuitBreaker.maxConcurrentRequests
	c.circuitBreaker.mu.RUnlock()

	switch state {
	case CircuitBreakerClosed:
		// Check concurrent request limit
		if currentRequests >= maxConcurrent {
			return &QueueError{
				Code:      "TOO_MANY_REQUESTS",
				Message:   "Too many concurrent requests",
				Timestamp: time.Now(),
				Retryable: true,
			}
		}
		return nil

	case CircuitBreakerOpen:
		if time.Since(lastFailure) > c.circuitBreaker.recoveryTimeout {
			c.circuitBreaker.mu.Lock()
			c.circuitBreaker.state = CircuitBreakerHalfOpen
			c.circuitBreaker.mu.Unlock()
			return nil
		}

		c.metrics.mu.Lock()
		c.metrics.circuitBreakerOpens++
		c.metrics.mu.Unlock()

		return ErrCircuitBreakerOpen

	case CircuitBreakerHalfOpen:
		// Allow one request to test if service is recovered
		return nil

	default:
		return nil
	}
}

// ✅ NEW: Calculate retry delay with exponential backoff
func (c *QueueClient) calculateRetryDelay(attempt int) time.Duration {
	delay := c.retryPolicy.InitialDelay

	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * c.retryPolicy.BackoffFactor)
		if delay > c.retryPolicy.MaxDelay {
			delay = c.retryPolicy.MaxDelay
			break
		}
	}

	return delay
}

// ✅ NEW: Update metrics
func (c *QueueClient) updateMetrics(duration time.Duration) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	c.metrics.totalRequests++
	c.metrics.lastRequestTime = time.Now()

	// Simple moving average for response time
	if c.metrics.averageResponseTime == 0 {
		c.metrics.averageResponseTime = duration
	} else {
		c.metrics.averageResponseTime = (c.metrics.averageResponseTime + duration) / 2
	}
}

// ✅ NEW: Get client metrics
func (c *QueueClient) GetMetrics() map[string]interface{} {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	return map[string]interface{}{
		"total_requests":        c.metrics.totalRequests,
		"successful_requests":   c.metrics.successfulRequests,
		"failed_requests":       c.metrics.failedRequests,
		"circuit_breaker_opens": c.metrics.circuitBreakerOpens,
		"average_response_time": c.metrics.averageResponseTime.String(),
		"last_request_time":     c.metrics.lastRequestTime.Format(time.RFC3339),
		"success_rate":          c.calculateSuccessRate(),
	}
}

// calculateSuccessRate calculates the success rate percentage
func (c *QueueClient) calculateSuccessRate() float64 {
	if c.metrics.totalRequests == 0 {
		return 0.0
	}
	return float64(c.metrics.successfulRequests) / float64(c.metrics.totalRequests) * 100.0
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
