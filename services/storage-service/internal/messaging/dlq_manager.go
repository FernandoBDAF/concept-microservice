package messaging

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// DLQManager provides enhanced Dead Letter Queue management capabilities
type DLQManager struct {
	channel          *amqp.Channel
	config           *DLQConfig
	metrics          *DLQMetrics
	retryPolicies    map[string]*RetryPolicy
	messageInspector *MessageInspector
	alertManager     *AlertManager
	log              *zap.Logger
	mu               sync.RWMutex
}

// DLQConfig holds configuration for enhanced DLQ management
type DLQConfig struct {
	ExchangeName        string
	QueueName           string
	RetryExchangeName   string
	RetryQueueName      string
	InspectionQueueName string
	MaxRetryAttempts    int
	BaseRetryDelay      time.Duration
	MaxRetryDelay       time.Duration
	BackoffMultiplier   float64
	AlertThreshold      int64
	CleanupOlderThan    time.Duration
	BatchReprocessSize  int
	MonitoringInterval  time.Duration
}

// DLQMetrics tracks DLQ performance and statistics
type DLQMetrics struct {
	TotalDLQMessages        int64
	ProcessedRetries        int64
	SuccessfulRetries       int64
	FailedRetries           int64
	ExpiredMessages         int64
	ManualReprocessRequests int64
	AlertsSent              int64
	MessagesByError         map[string]int64
	MessagesBySource        map[string]int64
	AverageRetryDelay       time.Duration
	LastCleanupTime         time.Time
	mu                      sync.RWMutex
}

// RetryPolicy defines retry behavior for different message types
type RetryPolicy struct {
	MessageType        string
	MaxAttempts        int
	BaseDelay          time.Duration
	MaxDelay           time.Duration
	BackoffMultiplier  float64
	ExponentialBackoff bool
	FixedDelay         time.Duration
	Conditions         []RetryCondition
}

// RetryCondition defines when a message should be retried
type RetryCondition struct {
	ErrorPattern string
	Retryable    bool
	CustomDelay  time.Duration
	MaxAttempts  int
}

// MessageInspector provides DLQ message inspection capabilities
type MessageInspector struct {
	recentMessages []DLQMessage
	maxHistory     int
	mu             sync.RWMutex
}

// DLQMessage represents a message in the DLQ with metadata
type DLQMessage struct {
	ID                string                 `json:"id"`
	OriginalMessage   *Message               `json:"original_message"`
	Error             string                 `json:"error"`
	FailedAt          time.Time              `json:"failed_at"`
	RetryCount        int                    `json:"retry_count"`
	LastRetryAt       time.Time              `json:"last_retry_at,omitempty"`
	Source            string                 `json:"source"`
	RoutingKey        string                 `json:"routing_key"`
	ProcessingHistory []ProcessingAttempt    `json:"processing_history"`
	Metadata          map[string]interface{} `json:"metadata"`
	Status            DLQMessageStatus       `json:"status"`
}

// ProcessingAttempt tracks individual processing attempts
type ProcessingAttempt struct {
	AttemptNumber int           `json:"attempt_number"`
	AttemptedAt   time.Time     `json:"attempted_at"`
	Error         string        `json:"error,omitempty"`
	Duration      time.Duration `json:"duration"`
	Success       bool          `json:"success"`
}

// DLQMessageStatus represents the current status of a DLQ message
type DLQMessageStatus string

const (
	DLQStatusPending     DLQMessageStatus = "pending"
	DLQStatusRetrying    DLQMessageStatus = "retrying"
	DLQStatusExhausted   DLQMessageStatus = "exhausted"
	DLQStatusReprocessed DLQMessageStatus = "reprocessed"
	DLQStatusExpired     DLQMessageStatus = "expired"
)

// AlertManager handles DLQ-related alerts and notifications
type AlertManager struct {
	threshold     int64
	alertsSent    int64
	lastAlertTime time.Time
	alertCooldown time.Duration
	alertCallback func(alert DLQAlert)
	mu            sync.RWMutex
}

// DLQAlert represents an alert about DLQ issues
type DLQAlert struct {
	Type         string    `json:"type"`
	Message      string    `json:"message"`
	Severity     string    `json:"severity"`
	Timestamp    time.Time `json:"timestamp"`
	MessageCount int64     `json:"message_count,omitempty"`
	ErrorPattern string    `json:"error_pattern,omitempty"`
}

// NewDLQManager creates a new enhanced DLQ manager
func NewDLQManager(channel *amqp.Channel, config *DLQConfig) *DLQManager {
	if config == nil {
		config = &DLQConfig{
			ExchangeName:        "storage-dlq",
			QueueName:           "storage-dlq",
			RetryExchangeName:   "storage-retry",
			RetryQueueName:      "storage-retry",
			InspectionQueueName: "storage-dlq-inspection",
			MaxRetryAttempts:    5,
			BaseRetryDelay:      time.Second,
			MaxRetryDelay:       time.Minute * 30,
			BackoffMultiplier:   2.0,
			AlertThreshold:      100,
			CleanupOlderThan:    24 * time.Hour,
			BatchReprocessSize:  10,
			MonitoringInterval:  time.Minute * 5,
		}
	}

	manager := &DLQManager{
		channel: channel,
		config:  config,
		metrics: &DLQMetrics{
			MessagesByError:  make(map[string]int64),
			MessagesBySource: make(map[string]int64),
		},
		retryPolicies: make(map[string]*RetryPolicy),
		messageInspector: &MessageInspector{
			recentMessages: make([]DLQMessage, 0),
			maxHistory:     1000,
		},
		alertManager: &AlertManager{
			threshold:     config.AlertThreshold,
			alertCooldown: time.Minute * 15,
		},
		log: zap.L().Named("dlq_manager"),
	}

	// Set up default retry policies
	manager.setupDefaultRetryPolicies()

	// Start monitoring
	go manager.startMonitoring()

	return manager
}

// ProcessDLQMessage handles a message that has been sent to the DLQ
func (dlq *DLQManager) ProcessDLQMessage(originalMsg *Message, processingError error) error {
	dlqMessage := &DLQMessage{
		ID:              fmt.Sprintf("dlq_%d", time.Now().UnixNano()),
		OriginalMessage: originalMsg,
		Error:           processingError.Error(),
		FailedAt:        time.Now(),
		RetryCount:      originalMsg.RetryCount,
		Source:          originalMsg.Source,
		RoutingKey:      originalMsg.RoutingKey,
		ProcessingHistory: []ProcessingAttempt{
			{
				AttemptNumber: originalMsg.RetryCount + 1,
				AttemptedAt:   time.Now(),
				Error:         processingError.Error(),
				Success:       false,
			},
		},
		Metadata: make(map[string]interface{}),
		Status:   DLQStatusPending,
	}

	// Update metrics
	dlq.updateDLQMetrics(dlqMessage)

	// Store message for inspection
	dlq.messageInspector.addMessage(*dlqMessage)

	// Check if we should attempt retry
	if dlq.shouldRetryMessage(dlqMessage) {
		return dlq.scheduleRetry(dlqMessage)
	}

	// Send to DLQ for manual processing
	return dlq.sendToDLQ(dlqMessage)
}

// shouldRetryMessage determines if a message should be automatically retried
func (dlq *DLQManager) shouldRetryMessage(dlqMsg *DLQMessage) bool {
	policy := dlq.getRetryPolicy(dlqMsg.OriginalMessage.Type)
	if policy == nil {
		return dlqMsg.RetryCount < dlq.config.MaxRetryAttempts
	}

	// Check retry conditions
	for _, condition := range policy.Conditions {
		if dlq.matchesErrorPattern(dlqMsg.Error, condition.ErrorPattern) {
			return condition.Retryable && dlqMsg.RetryCount < condition.MaxAttempts
		}
	}

	return dlqMsg.RetryCount < policy.MaxAttempts
}

// scheduleRetry schedules a message for retry with appropriate delay
func (dlq *DLQManager) scheduleRetry(dlqMsg *DLQMessage) error {
	dlqMsg.Status = DLQStatusRetrying

	delay := dlq.calculateRetryDelay(dlqMsg)
	dlqMsg.LastRetryAt = time.Now().Add(delay)

	dlq.log.Info("Scheduling message retry",
		zap.String("message_id", dlqMsg.ID),
		zap.Duration("delay", delay),
		zap.Int("retry_count", dlqMsg.RetryCount),
		zap.String("error", dlqMsg.Error))

	// Publish to retry queue with delay
	return dlq.publishToRetryQueue(dlqMsg, delay)
}

// calculateRetryDelay calculates the delay before retrying a message
func (dlq *DLQManager) calculateRetryDelay(dlqMsg *DLQMessage) time.Duration {
	policy := dlq.getRetryPolicy(dlqMsg.OriginalMessage.Type)

	var baseDelay time.Duration
	var maxDelay time.Duration
	var multiplier float64
	var exponential bool

	if policy != nil {
		baseDelay = policy.BaseDelay
		maxDelay = policy.MaxDelay
		multiplier = policy.BackoffMultiplier
		exponential = policy.ExponentialBackoff

		if policy.FixedDelay > 0 {
			return policy.FixedDelay
		}
	} else {
		baseDelay = dlq.config.BaseRetryDelay
		maxDelay = dlq.config.MaxRetryDelay
		multiplier = dlq.config.BackoffMultiplier
		exponential = true
	}

	var delay time.Duration
	if exponential {
		// Exponential backoff with jitter
		backoffFactor := math.Pow(multiplier, float64(dlqMsg.RetryCount))
		delay = time.Duration(float64(baseDelay) * backoffFactor)

		// Add jitter (±25%)
		jitter := time.Duration(float64(delay) * 0.25 * float64(2*time.Now().UnixNano()%2-1))
		delay += jitter
	} else {
		// Linear backoff
		delay = time.Duration(int64(baseDelay) * int64(dlqMsg.RetryCount+1))
	}

	// Cap at maximum delay
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// publishToRetryQueue publishes a message to the retry queue with delay
func (dlq *DLQManager) publishToRetryQueue(dlqMsg *DLQMessage, delay time.Duration) error {
	// Prepare message for retry
	retryMsg := *dlqMsg.OriginalMessage
	retryMsg.RetryCount++
	retryMsg.Timestamp = time.Now()

	msgBody, err := json.Marshal(retryMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal retry message: %w", err)
	}

	headers := amqp.Table{
		"x-delay":          int64(delay.Milliseconds()),
		"x-dlq-message-id": dlqMsg.ID,
		"x-retry-count":    retryMsg.RetryCount,
		"x-original-error": dlqMsg.Error,
	}

	err = dlq.channel.Publish(
		dlq.config.RetryExchangeName,
		retryMsg.RoutingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        msgBody,
			Headers:     headers,
			Timestamp:   time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish to retry queue: %w", err)
	}

	dlq.metrics.ProcessedRetries++
	return nil
}

// sendToDLQ sends a message to the DLQ for manual processing
func (dlq *DLQManager) sendToDLQ(dlqMsg *DLQMessage) error {
	dlqMsg.Status = DLQStatusExhausted

	msgBody, err := json.Marshal(dlqMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ message: %w", err)
	}

	headers := amqp.Table{
		"x-dlq-message-id": dlqMsg.ID,
		"x-failed-at":      dlqMsg.FailedAt.Format(time.RFC3339),
		"x-error":          dlqMsg.Error,
		"x-retry-count":    dlqMsg.RetryCount,
		"x-source":         dlqMsg.Source,
		"x-routing-key":    dlqMsg.RoutingKey,
	}

	err = dlq.channel.Publish(
		dlq.config.ExchangeName,
		dlq.config.QueueName,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        msgBody,
			Headers:     headers,
			Timestamp:   time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish to DLQ: %w", err)
	}

	dlq.log.Warn("Message sent to DLQ",
		zap.String("message_id", dlqMsg.ID),
		zap.String("error", dlqMsg.Error),
		zap.Int("retry_count", dlqMsg.RetryCount))

	dlq.metrics.TotalDLQMessages++
	return nil
}

// ManualRetry manually retries a specific DLQ message
func (dlq *DLQManager) ManualRetry(messageID string) error {
	dlq.log.Info("Manual retry requested", zap.String("message_id", messageID))

	message := dlq.messageInspector.getMessage(messageID)
	if message == nil {
		return fmt.Errorf("message not found: %s", messageID)
	}

	// Reset retry count for manual retry
	message.RetryCount = 0
	message.Status = DLQStatusRetrying
	message.LastRetryAt = time.Now()

	// Add processing attempt
	message.ProcessingHistory = append(message.ProcessingHistory, ProcessingAttempt{
		AttemptNumber: len(message.ProcessingHistory) + 1,
		AttemptedAt:   time.Now(),
		Success:       false, // Will be updated when processed
	})

	dlq.metrics.ManualReprocessRequests++

	// Send directly to processing queue (bypass delay)
	return dlq.reprocessMessage(message)
}

// BatchRetry retries multiple messages based on criteria
func (dlq *DLQManager) BatchRetry(criteria BatchRetryCriteria) (*BatchRetryResult, error) {
	dlq.log.Info("Batch retry requested",
		zap.String("error_pattern", criteria.ErrorPattern),
		zap.String("source", criteria.Source),
		zap.Int("max_messages", criteria.MaxMessages))

	messages := dlq.messageInspector.getMessagesByCriteria(criteria)
	result := &BatchRetryResult{
		TotalMessages:     len(messages),
		ProcessedMessages: 0,
		SuccessfulRetries: 0,
		FailedRetries:     0,
		StartedAt:         time.Now(),
	}

	for _, msg := range messages {
		if result.ProcessedMessages >= criteria.MaxMessages {
			break
		}

		err := dlq.ManualRetry(msg.ID)
		result.ProcessedMessages++

		if err != nil {
			result.FailedRetries++
			result.Errors = append(result.Errors, fmt.Sprintf("Message %s: %v", msg.ID, err))
		} else {
			result.SuccessfulRetries++
		}
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)

	dlq.log.Info("Batch retry completed",
		zap.Int("processed", result.ProcessedMessages),
		zap.Int("successful", result.SuccessfulRetries),
		zap.Int("failed", result.FailedRetries),
		zap.Duration("duration", result.Duration))

	return result, nil
}

// GetDLQAnalytics returns comprehensive DLQ analytics
func (dlq *DLQManager) GetDLQAnalytics() *DLQAnalytics {
	dlq.metrics.mu.RLock()
	defer dlq.metrics.mu.RUnlock()

	analytics := &DLQAnalytics{
		GeneratedAt:             time.Now(),
		TotalDLQMessages:        dlq.metrics.TotalDLQMessages,
		ProcessedRetries:        dlq.metrics.ProcessedRetries,
		SuccessfulRetries:       dlq.metrics.SuccessfulRetries,
		FailedRetries:           dlq.metrics.FailedRetries,
		ExpiredMessages:         dlq.metrics.ExpiredMessages,
		ManualReprocessRequests: dlq.metrics.ManualReprocessRequests,
		AlertsSent:              dlq.metrics.AlertsSent,
		MessagesByError:         make(map[string]int64),
		MessagesBySource:        make(map[string]int64),
		AverageRetryDelay:       dlq.metrics.AverageRetryDelay,
		LastCleanupTime:         dlq.metrics.LastCleanupTime,
	}

	// Copy maps to avoid race conditions
	for k, v := range dlq.metrics.MessagesByError {
		analytics.MessagesByError[k] = v
	}
	for k, v := range dlq.metrics.MessagesBySource {
		analytics.MessagesBySource[k] = v
	}

	// Calculate success rate
	totalRetries := dlq.metrics.ProcessedRetries
	if totalRetries > 0 {
		analytics.RetrySuccessRate = float64(dlq.metrics.SuccessfulRetries) / float64(totalRetries) * 100
	}

	// Get recent messages summary
	analytics.RecentMessages = dlq.messageInspector.getRecentMessagesSummary(50)

	return analytics
}

// Helper types for DLQ management

type BatchRetryCriteria struct {
	ErrorPattern string
	Source       string
	RoutingKey   string
	Since        time.Time
	Until        time.Time
	MaxMessages  int
}

type BatchRetryResult struct {
	TotalMessages     int           `json:"total_messages"`
	ProcessedMessages int           `json:"processed_messages"`
	SuccessfulRetries int           `json:"successful_retries"`
	FailedRetries     int           `json:"failed_retries"`
	StartedAt         time.Time     `json:"started_at"`
	CompletedAt       time.Time     `json:"completed_at"`
	Duration          time.Duration `json:"duration"`
	Errors            []string      `json:"errors,omitempty"`
}

type DLQAnalytics struct {
	GeneratedAt             time.Time           `json:"generated_at"`
	TotalDLQMessages        int64               `json:"total_dlq_messages"`
	ProcessedRetries        int64               `json:"processed_retries"`
	SuccessfulRetries       int64               `json:"successful_retries"`
	FailedRetries           int64               `json:"failed_retries"`
	ExpiredMessages         int64               `json:"expired_messages"`
	ManualReprocessRequests int64               `json:"manual_reprocess_requests"`
	AlertsSent              int64               `json:"alerts_sent"`
	RetrySuccessRate        float64             `json:"retry_success_rate"`
	MessagesByError         map[string]int64    `json:"messages_by_error"`
	MessagesBySource        map[string]int64    `json:"messages_by_source"`
	AverageRetryDelay       time.Duration       `json:"average_retry_delay"`
	LastCleanupTime         time.Time           `json:"last_cleanup_time"`
	RecentMessages          []DLQMessageSummary `json:"recent_messages"`
}

type DLQMessageSummary struct {
	ID         string           `json:"id"`
	Error      string           `json:"error"`
	FailedAt   time.Time        `json:"failed_at"`
	RetryCount int              `json:"retry_count"`
	Source     string           `json:"source"`
	RoutingKey string           `json:"routing_key"`
	Status     DLQMessageStatus `json:"status"`
}

// Implementation of helper methods

func (dlq *DLQManager) updateDLQMetrics(dlqMsg *DLQMessage) {
	dlq.metrics.mu.Lock()
	defer dlq.metrics.mu.Unlock()

	dlq.metrics.MessagesByError[dlqMsg.Error]++
	dlq.metrics.MessagesBySource[dlqMsg.Source]++
}

func (dlq *DLQManager) getRetryPolicy(messageType string) *RetryPolicy {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	return dlq.retryPolicies[messageType]
}

func (dlq *DLQManager) matchesErrorPattern(error, pattern string) bool {
	// Simple pattern matching - in production, use regex
	return error == pattern || pattern == "*"
}

func (dlq *DLQManager) reprocessMessage(message *DLQMessage) error {
	// Reprocess message by sending it back to the original queue
	msgBody, err := json.Marshal(message.OriginalMessage)
	if err != nil {
		return err
	}

	return dlq.channel.Publish(
		"", // Use default exchange
		message.RoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        msgBody,
			Headers: amqp.Table{
				"x-reprocessed":    true,
				"x-dlq-message-id": message.ID,
			},
		},
	)
}

func (dlq *DLQManager) setupDefaultRetryPolicies() {
	// Default retry policy for storage operations
	dlq.retryPolicies["storage.create"] = &RetryPolicy{
		MessageType:        "storage.create",
		MaxAttempts:        5,
		BaseDelay:          time.Second,
		MaxDelay:           time.Minute * 5,
		BackoffMultiplier:  2.0,
		ExponentialBackoff: true,
		Conditions: []RetryCondition{
			{
				ErrorPattern: "connection timeout",
				Retryable:    true,
				MaxAttempts:  7,
			},
			{
				ErrorPattern: "validation error",
				Retryable:    false,
			},
		},
	}

	// Add more default policies as needed
}

func (dlq *DLQManager) startMonitoring() {
	ticker := time.NewTicker(dlq.config.MonitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			dlq.performMonitoringCheck()
		}
	}
}

func (dlq *DLQManager) performMonitoringCheck() {
	// Check if we need to send alerts
	if dlq.metrics.TotalDLQMessages >= dlq.config.AlertThreshold {
		dlq.sendAlert(DLQAlert{
			Type:         "high_dlq_volume",
			Message:      fmt.Sprintf("DLQ contains %d messages, exceeding threshold of %d", dlq.metrics.TotalDLQMessages, dlq.config.AlertThreshold),
			Severity:     "warning",
			Timestamp:    time.Now(),
			MessageCount: dlq.metrics.TotalDLQMessages,
		})
	}

	// Cleanup old messages
	dlq.cleanupExpiredMessages()
}

func (dlq *DLQManager) sendAlert(alert DLQAlert) {
	dlq.alertManager.mu.Lock()
	defer dlq.alertManager.mu.Unlock()

	// Check cooldown
	if time.Since(dlq.alertManager.lastAlertTime) < dlq.alertManager.alertCooldown {
		return
	}

	dlq.log.Warn("DLQ Alert",
		zap.String("type", alert.Type),
		zap.String("message", alert.Message),
		zap.String("severity", alert.Severity))

	if dlq.alertManager.alertCallback != nil {
		dlq.alertManager.alertCallback(alert)
	}

	dlq.alertManager.alertsSent++
	dlq.alertManager.lastAlertTime = time.Now()
	dlq.metrics.AlertsSent++
}

func (dlq *DLQManager) cleanupExpiredMessages() {
	// Implementation would remove messages older than CleanupOlderThan
	dlq.log.Debug("Performing DLQ cleanup")
	dlq.metrics.LastCleanupTime = time.Now()
}

// MessageInspector methods

func (mi *MessageInspector) addMessage(msg DLQMessage) {
	mi.mu.Lock()
	defer mi.mu.Unlock()

	mi.recentMessages = append(mi.recentMessages, msg)
	if len(mi.recentMessages) > mi.maxHistory {
		mi.recentMessages = mi.recentMessages[1:]
	}
}

func (mi *MessageInspector) getMessage(id string) *DLQMessage {
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	for i := range mi.recentMessages {
		if mi.recentMessages[i].ID == id {
			return &mi.recentMessages[i]
		}
	}
	return nil
}

func (mi *MessageInspector) getMessagesByCriteria(criteria BatchRetryCriteria) []DLQMessage {
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	var matches []DLQMessage
	for _, msg := range mi.recentMessages {
		if mi.matchesCriteria(msg, criteria) {
			matches = append(matches, msg)
		}
	}
	return matches
}

func (mi *MessageInspector) matchesCriteria(msg DLQMessage, criteria BatchRetryCriteria) bool {
	if criteria.Source != "" && msg.Source != criteria.Source {
		return false
	}
	if criteria.RoutingKey != "" && msg.RoutingKey != criteria.RoutingKey {
		return false
	}
	if criteria.ErrorPattern != "" && !mi.containsPattern(msg.Error, criteria.ErrorPattern) {
		return false
	}
	if !criteria.Since.IsZero() && msg.FailedAt.Before(criteria.Since) {
		return false
	}
	if !criteria.Until.IsZero() && msg.FailedAt.After(criteria.Until) {
		return false
	}
	return true
}

func (mi *MessageInspector) containsPattern(text, pattern string) bool {
	// Simple pattern matching - in production, use regex
	return text == pattern || pattern == "*"
}

func (mi *MessageInspector) getRecentMessagesSummary(limit int) []DLQMessageSummary {
	mi.mu.RLock()
	defer mi.mu.RUnlock()

	var summaries []DLQMessageSummary
	count := len(mi.recentMessages)
	if count > limit {
		count = limit
	}

	for i := len(mi.recentMessages) - count; i < len(mi.recentMessages); i++ {
		msg := mi.recentMessages[i]
		summaries = append(summaries, DLQMessageSummary{
			ID:         msg.ID,
			Error:      msg.Error,
			FailedAt:   msg.FailedAt,
			RetryCount: msg.RetryCount,
			Source:     msg.Source,
			RoutingKey: msg.RoutingKey,
			Status:     msg.Status,
		})
	}

	return summaries
}
