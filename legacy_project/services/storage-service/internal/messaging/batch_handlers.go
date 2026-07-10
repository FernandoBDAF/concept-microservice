package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/domain/models"
	"microservices/services/profile-storage/internal/domain/service"
	"microservices/services/profile-storage/internal/pkg/logger"
)

// BatchMessageHandler handles batch-related messages from the queue
type BatchMessageHandler struct {
	batchService *service.AdvancedBatchOperationsService
	log          *zap.Logger
}

// NewBatchMessageHandler creates a new batch message handler
func NewBatchMessageHandler(batchService *service.AdvancedBatchOperationsService) *BatchMessageHandler {
	return &BatchMessageHandler{
		batchService: batchService,
		log:          logger.Get().Named("batch_message_handler"),
	}
}

// CanHandle checks if this handler can process the given routing key
func (h *BatchMessageHandler) CanHandle(routingKey string) bool {
	supportedKeys := h.GetSupportedRoutingKeys()
	for _, key := range supportedKeys {
		if key == routingKey {
			return true
		}
	}
	return false
}

// GetSupportedRoutingKeys returns the routing keys this handler supports
func (h *BatchMessageHandler) GetSupportedRoutingKeys() []string {
	return []string{
		"batch.process",
		"batch.profile.process",
		"batch.status",
		"batch.operation.create",
		"batch.operation.update",
		"batch.operation.delete",
	}
}

// Handle processes batch-related messages based on routing key
func (h *BatchMessageHandler) Handle(ctx context.Context, msg *Message) (*MessageResponse, error) {
	startTime := time.Now()
	h.log.Info("Processing batch message",
		logger.String("routing_key", msg.RoutingKey),
		logger.String("message_id", msg.ID),
		logger.String("message_type", msg.Type),
	)

	var response *MessageResponse
	var err error

	switch msg.RoutingKey {
	case "batch.process":
		response, err = h.handleBatchProcess(ctx, msg, startTime)
	case "batch.profile.process":
		response, err = h.handleProfileBatchProcess(ctx, msg, startTime)
	case "batch.status":
		response, err = h.handleBatchStatus(ctx, msg, startTime)
	case "batch.cancel":
		response, err = h.handleBatchCancel(ctx, msg, startTime)
	case "batch.validate":
		response, err = h.handleBatchValidate(ctx, msg, startTime)
	case "batch.metrics":
		response, err = h.handleBatchMetrics(ctx, msg, startTime)
	default:
		h.log.Error("Unsupported routing key for batch handler",
			logger.String("routing_key", msg.RoutingKey),
			logger.String("message_id", msg.ID),
		)
		return h.createErrorResponse(msg, fmt.Errorf("unsupported batch routing key: %s", msg.RoutingKey)), nil
	}

	if err != nil {
		h.log.Error("Failed to process batch message",
			logger.String("routing_key", msg.RoutingKey),
			logger.String("message_id", msg.ID),
			logger.ErrorField(err),
			logger.Duration("duration", time.Since(startTime)),
		)
		return h.createErrorResponse(msg, err), nil
	}

	h.log.Info("Successfully processed batch message",
		logger.String("routing_key", msg.RoutingKey),
		logger.String("message_id", msg.ID),
		logger.Duration("duration", time.Since(startTime)),
	)

	return response, nil
}

// handleBatchProcess processes generic batch.process messages
func (h *BatchMessageHandler) handleBatchProcess(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing generic batch process message",
		logger.String("message_id", msg.ID),
	)

	var req models.BatchRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch request: %w", err)
	}

	// Set default options if not provided
	if req.Options.Mode == "" {
		req.Options = models.DefaultBatchOptions()
	}

	// Process batch
	result, err := h.batchService.ProcessBatch(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to process batch: %w", err)
	}

	return &MessageResponse{
		MessageID: fmt.Sprintf("resp_%s", msg.ID),
		Success:   true,
		Result: map[string]interface{}{
			"batch_result":   result,
			"batch_id":       result.ID,
			"status":         result.Status,
			"successful_ops": result.SuccessfulOps,
			"failed_ops":     result.FailedOps,
		},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// handleProfileBatchProcess processes batch.profile.process messages
func (h *BatchMessageHandler) handleProfileBatchProcess(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing profile batch process message",
		logger.String("message_id", msg.ID),
	)

	var req models.BatchRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile batch request: %w", err)
	}

	// Set batch type to profile
	req.Type = "profile"

	// Set default options if not provided
	if req.Options.Mode == "" {
		req.Options = models.DefaultBatchOptions()
	}

	// Process batch
	result, err := h.batchService.ProcessBatch(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to process profile batch: %w", err)
	}

	return &MessageResponse{
		MessageID: fmt.Sprintf("resp_%s", msg.ID),
		Success:   true,
		Result: map[string]interface{}{
			"batch_result":     result,
			"batch_type":       "profile",
			"batch_id":         result.ID,
			"status":           result.Status,
			"successful_ops":   result.SuccessfulOps,
			"failed_ops":       result.FailedOps,
			"processing_stats": result.ProcessingStats,
		},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// handleBatchStatus processes batch.status messages
func (h *BatchMessageHandler) handleBatchStatus(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing batch status message",
		logger.String("message_id", msg.ID),
	)

	var statusReq struct {
		BatchID string `json:"batch_id"`
	}

	if err := json.Unmarshal(msg.Payload, &statusReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch status request: %w", err)
	}

	job, exists := h.batchService.GetBatchStatus(statusReq.BatchID)
	if !exists {
		return nil, fmt.Errorf("batch not found: %s", statusReq.BatchID)
	}

	return &MessageResponse{
		MessageID: fmt.Sprintf("resp_%s", msg.ID),
		Success:   true,
		Result: map[string]interface{}{
			"batch_id":   statusReq.BatchID,
			"status":     job.Status,
			"progress":   job.Progress,
			"current_op": job.CurrentOp,
			"total_ops":  job.TotalOps,
			"start_time": job.StartTime,
			"elapsed":    time.Since(job.StartTime),
		},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// handleBatchCancel processes batch.cancel messages
func (h *BatchMessageHandler) handleBatchCancel(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing batch cancel message",
		logger.String("message_id", msg.ID),
	)

	var cancelReq struct {
		BatchID string `json:"batch_id"`
	}

	if err := json.Unmarshal(msg.Payload, &cancelReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch cancel request: %w", err)
	}

	err := h.batchService.CancelBatch(cancelReq.BatchID)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel batch: %w", err)
	}

	return &MessageResponse{
		MessageID: fmt.Sprintf("resp_%s", msg.ID),
		Success:   true,
		Result: map[string]interface{}{
			"batch_id": cancelReq.BatchID,
			"status":   "cancelled",
			"message":  "Batch cancelled successfully",
		},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// handleBatchValidate processes batch.validate messages (preview mode)
func (h *BatchMessageHandler) handleBatchValidate(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing batch validate message",
		logger.String("message_id", msg.ID),
	)

	var req models.BatchRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch validation request: %w", err)
	}

	// Set validation level to preview
	req.Options.ValidationLevel = models.BatchValidationPreview

	// Validate request
	validationResult := map[string]interface{}{
		"batch_id": req.ID,
		"valid":    true,
		"errors":   []string{},
		"warnings": []string{},
	}

	if err := req.Validate(); err != nil {
		validationResult["valid"] = false
		validationResult["errors"] = []string{err.Error()}
	} else {
		// Add validation insights
		validationResult["total_operations"] = len(req.Operations)
		validationResult["processing_mode"] = req.Options.Mode
		validationResult["failure_handling"] = req.Options.FailureHandling
		validationResult["estimated_duration"] = h.estimateBatchDuration(&req)
		validationResult["warnings"] = h.generateValidationWarnings(&req)
	}

	return &MessageResponse{
		MessageID: fmt.Sprintf("resp_%s", msg.ID),
		Success:   true,
		Result: map[string]interface{}{
			"validation_result": validationResult,
		},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// handleBatchMetrics processes batch.metrics messages
func (h *BatchMessageHandler) handleBatchMetrics(ctx context.Context, msg *Message, startTime time.Time) (*MessageResponse, error) {
	h.log.Debug("Processing batch metrics message",
		logger.String("message_id", msg.ID),
	)

	metrics := h.batchService.GetMetrics()

	return &MessageResponse{
		MessageID: fmt.Sprintf("resp_%s", msg.ID),
		Success:   true,
		Result: map[string]interface{}{
			"batch_metrics": metrics,
			"timestamp":     time.Now(),
		},
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// Helper methods

// estimateBatchDuration estimates how long a batch might take to process
func (h *BatchMessageHandler) estimateBatchDuration(req *models.BatchRequest) time.Duration {
	baseTimePerOp := 100 * time.Millisecond

	switch req.Options.Mode {
	case models.BatchModeTransactional:
		baseTimePerOp = 150 * time.Millisecond
	case models.BatchModeParallel:
		parallelFactor := float64(req.Options.MaxConcurrency)
		if parallelFactor == 0 {
			parallelFactor = 5
		}
		baseTimePerOp = time.Duration(float64(baseTimePerOp) / parallelFactor * 1.2)
	}

	return time.Duration(len(req.Operations)) * baseTimePerOp
}

// generateValidationWarnings generates warnings for potentially problematic configurations
func (h *BatchMessageHandler) generateValidationWarnings(req *models.BatchRequest) []string {
	var warnings []string

	if len(req.Operations) > 500 {
		warnings = append(warnings, "Large batch size (>500 operations) may impact performance")
	}

	if req.Options.Mode == models.BatchModeTransactional && len(req.Operations) > 100 {
		warnings = append(warnings, "Large transactional batches may cause lock contention")
	}

	if req.Options.MaxConcurrency > 20 {
		warnings = append(warnings, "High concurrency (>20) may overwhelm database connections")
	}

	if req.Options.TotalTimeout < time.Minute && len(req.Operations) > 50 {
		warnings = append(warnings, "Short timeout may not be sufficient for batch size")
	}

	return warnings
}

// createErrorResponse creates a standardized error response
func (h *BatchMessageHandler) createErrorResponse(msg *Message, err error) *MessageResponse {
	return &MessageResponse{
		MessageID: fmt.Sprintf("resp_%s", msg.ID),
		Success:   false,
		Error:     err.Error(),
		Result: map[string]interface{}{
			"error":   err.Error(),
			"success": false,
			"type":    "batch_error",
		},
		ProcessedAt: time.Now(),
	}
}
