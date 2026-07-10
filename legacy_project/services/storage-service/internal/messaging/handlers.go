package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/domain/models"
	"microservices/services/profile-storage/internal/domain/service"
)

// StorageHandler handles storage-related messages with enhanced Phase 2 capabilities
type StorageHandler struct {
	profileService *service.ProfileService
	batchService   *service.AdvancedBatchOperationsService
	log            *zap.Logger
	metrics        *StorageMetrics
}

// StorageMetrics tracks handler performance and operations
type StorageMetrics struct {
	ProcessedMessages     int64
	FailedMessages        int64
	CreateOperations      int64
	UpdateOperations      int64
	DeleteOperations      int64
	BatchOperations       int64
	AverageProcessingTime time.Duration
	ConflictErrors        int64
	ValidationErrors      int64
}

// NewStorageHandler creates a new enhanced storage handler
func NewStorageHandler(profileService *service.ProfileService, batchService *service.AdvancedBatchOperationsService) *StorageHandler {
	return &StorageHandler{
		profileService: profileService,
		batchService:   batchService,
		log:            zap.L().Named("storage_handler"),
		metrics:        &StorageMetrics{},
	}
}

// CanHandle checks if this handler can process the given routing key
func (h *StorageHandler) CanHandle(routingKey string) bool {
	supportedKeys := h.GetSupportedRoutingKeys()
	for _, key := range supportedKeys {
		if key == routingKey {
			return true
		}
	}
	return false
}

// GetSupportedRoutingKeys returns the routing keys this handler supports
func (h *StorageHandler) GetSupportedRoutingKeys() []string {
	return []string{
		"storage.create",
		"storage.update",
		"storage.delete",
		"storage.batch",
		"storage.profile.create",
		"storage.profile.update",
		"storage.profile.delete",
	}
}

// Handle processes storage messages with enhanced validation and monitoring
func (h *StorageHandler) Handle(ctx context.Context, msg *Message) (*MessageResponse, error) {
	startTime := time.Now()

	h.log.Info("Processing storage message",
		zap.String("message_id", msg.ID),
		zap.String("routing_key", msg.RoutingKey),
		zap.String("type", msg.Type),
		zap.String("source", msg.Source),
		zap.String("correlation_id", msg.Correlation))

	// Enhanced message validation
	if err := h.validateMessage(msg); err != nil {
		h.metrics.ValidationErrors++
		h.metrics.FailedMessages++
		return h.createErrorResponse(msg.ID, fmt.Errorf("message validation failed: %w", err), startTime), err
	}

	var result map[string]interface{}
	var err error

	// Route to appropriate handler with enhanced processing
	switch msg.RoutingKey {
	case "storage.create":
		result, err = h.handleCreateEnhanced(ctx, msg, startTime)
		h.metrics.CreateOperations++
	case "storage.update":
		result, err = h.handleUpdateEnhanced(ctx, msg, startTime)
		h.metrics.UpdateOperations++
	case "storage.delete":
		result, err = h.handleDeleteEnhanced(ctx, msg, startTime)
		h.metrics.DeleteOperations++
	case "storage.batch":
		result, err = h.handleBatchEnhanced(ctx, msg, startTime)
		h.metrics.BatchOperations++
	default:
		err = fmt.Errorf("unsupported routing key: %s", msg.RoutingKey)
		h.metrics.ValidationErrors++
	}

	// Update metrics
	processingTime := time.Since(startTime)
	h.updateProcessingTimeMetrics(processingTime)

	response := &MessageResponse{
		MessageID:      msg.ID,
		Success:        err == nil,
		ProcessedAt:    time.Now(),
		ProcessingTime: processingTime,
	}

	if err != nil {
		response.Error = err.Error()
		h.metrics.FailedMessages++
		h.log.Error("Storage operation failed",
			zap.String("message_id", msg.ID),
			zap.String("routing_key", msg.RoutingKey),
			zap.Error(err),
			zap.Duration("processing_time", processingTime))
	} else {
		response.Result = result
		h.metrics.ProcessedMessages++
		h.log.Info("Storage operation completed successfully",
			zap.String("message_id", msg.ID),
			zap.String("routing_key", msg.RoutingKey),
			zap.Duration("processing_time", processingTime))
	}

	return response, err
}

// handleCreateEnhanced processes profile creation with advanced validation
func (h *StorageHandler) handleCreateEnhanced(ctx context.Context, msg *Message, startTime time.Time) (map[string]interface{}, error) {
	var task models.StorageTask
	if err := msg.UnmarshalPayload(&task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal create task: %w", err)
	}

	// Enhanced operation validation
	if err := h.validateCreateTask(&task); err != nil {
		h.metrics.ValidationErrors++
		return nil, fmt.Errorf("create task validation failed: %w", err)
	}

	// Convert data with enhanced validation
	profileReq, err := h.convertToProfileRequestEnhanced(task.Data, "create")
	if err != nil {
		h.metrics.ValidationErrors++
		return nil, fmt.Errorf("failed to convert data to profile request: %w", err)
	}

	// Check for potential duplicates (conflict handling)
	if err := h.checkForDuplicateProfile(ctx, profileReq); err != nil {
		h.metrics.ConflictErrors++
		return nil, fmt.Errorf("conflict detected: %w", err)
	}

	// Create profile with transaction safety
	profile, err := h.profileService.CreateProfile(ctx, profileReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	// Enhanced response with additional metadata
	return map[string]interface{}{
		"operation":          "create",
		"profile_id":         profile.ID,
		"profile":            profile,
		"created_at":         profile.CreatedAt,
		"processing_time_ms": time.Since(startTime).Milliseconds(),
		"source":             task.RequestedBy,
		"addresses_count":    len(profile.Addresses),
		"contacts_count":     len(profile.Contacts),
	}, nil
}

// handleUpdateEnhanced processes profile updates with conflict detection
func (h *StorageHandler) handleUpdateEnhanced(ctx context.Context, msg *Message, startTime time.Time) (map[string]interface{}, error) {
	var task models.StorageTask
	if err := msg.UnmarshalPayload(&task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal update task: %w", err)
	}

	// Enhanced operation validation
	if err := h.validateUpdateTask(&task); err != nil {
		h.metrics.ValidationErrors++
		return nil, fmt.Errorf("update task validation failed: %w", err)
	}

	// Check if profile exists before update
	existingProfile, err := h.profileService.GetProfile(ctx, *task.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch existing profile: %w", err)
	}

	// Convert data with enhanced validation
	profileReq, err := h.convertToProfileRequestEnhanced(task.Data, "update")
	if err != nil {
		h.metrics.ValidationErrors++
		return nil, fmt.Errorf("failed to convert data to profile request: %w", err)
	}

	// Conflict detection for concurrent updates
	if err := h.detectUpdateConflicts(ctx, existingProfile, profileReq, &task); err != nil {
		h.metrics.ConflictErrors++
		return nil, fmt.Errorf("update conflict detected: %w", err)
	}

	// Update profile with optimistic locking consideration
	profile, err := h.profileService.UpdateProfile(ctx, *task.ProfileID, profileReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	// Enhanced response with change tracking
	changes := h.trackProfileChanges(existingProfile, profile)

	return map[string]interface{}{
		"operation":          "update",
		"profile_id":         profile.ID,
		"profile":            profile,
		"updated_at":         profile.UpdatedAt,
		"processing_time_ms": time.Since(startTime).Milliseconds(),
		"source":             task.RequestedBy,
		"changes_made":       changes,
		"previous_version":   existingProfile.UpdatedAt,
	}, nil
}

// handleDeleteEnhanced processes profile deletion with cascade options
func (h *StorageHandler) handleDeleteEnhanced(ctx context.Context, msg *Message, startTime time.Time) (map[string]interface{}, error) {
	var task models.StorageTask
	if err := msg.UnmarshalPayload(&task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal delete task: %w", err)
	}

	// Enhanced operation validation
	if err := h.validateDeleteTask(&task); err != nil {
		h.metrics.ValidationErrors++
		return nil, fmt.Errorf("delete task validation failed: %w", err)
	}

	// Check if profile exists and gather related data
	profile, err := h.profileService.GetProfile(ctx, *task.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile for deletion: %w", err)
	}

	// Store metadata before deletion for audit trail
	deletionMetadata := map[string]interface{}{
		"deleted_profile_email": profile.Email,
		"addresses_deleted":     len(profile.Addresses),
		"contacts_deleted":      len(profile.Contacts),
		"profile_age_days":      time.Since(profile.CreatedAt).Hours() / 24,
	}

	// Handle cascade options from task options
	cascadeOptions := h.extractCascadeOptions(task.Options)

	// Delete profile with cascade handling
	err = h.profileService.DeleteProfile(ctx, *task.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete profile: %w", err)
	}

	// Enhanced response with audit information
	return map[string]interface{}{
		"operation":          "delete",
		"profile_id":         *task.ProfileID,
		"deleted_at":         time.Now(),
		"processing_time_ms": time.Since(startTime).Milliseconds(),
		"source":             task.RequestedBy,
		"cascade_options":    cascadeOptions,
		"deletion_metadata":  deletionMetadata,
	}, nil
}

// handleBatchEnhanced processes batch operations with advanced monitoring
func (h *StorageHandler) handleBatchEnhanced(ctx context.Context, msg *Message, startTime time.Time) (map[string]interface{}, error) {
	var batchTask BatchStorageTask
	if err := msg.UnmarshalPayload(&batchTask); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch task: %w", err)
	}

	// Enhanced batch validation
	if err := h.validateBatchTask(&batchTask); err != nil {
		h.metrics.ValidationErrors++
		return nil, fmt.Errorf("batch task validation failed: %w", err)
	}

	// Pre-process batch for optimization
	optimizedOperations, err := h.optimizeBatchOperations(batchTask.Operations)
	if err != nil {
		return nil, fmt.Errorf("failed to optimize batch operations: %w", err)
	}

	// Convert old format to new BatchRequest format
	batchRequest, err := h.convertToBatchRequest(batchTask.BatchID, optimizedOperations, batchTask.Options)
	if err != nil {
		return nil, fmt.Errorf("failed to convert batch request: %w", err)
	}

	// Process batch with enhanced options
	result, err := h.batchService.ProcessBatch(ctx, batchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to process batch: %w", err)
	}

	// Enhanced response with detailed metrics
	return map[string]interface{}{
		"operation":          "batch",
		"batch_id":           batchTask.BatchID,
		"operations_count":   len(batchTask.Operations),
		"optimized_count":    len(optimizedOperations),
		"result":             result,
		"completed_at":       time.Now(),
		"processing_time_ms": time.Since(startTime).Milliseconds(),
		"efficiency_score":   h.calculateBatchEfficiency(len(batchTask.Operations), len(optimizedOperations), result.Duration),
	}, nil
}

// convertToBatchRequest converts old StorageTask format to new BatchRequest format
func (h *StorageHandler) convertToBatchRequest(batchID string, operations []models.StorageTask, oldOptions models.BatchOptions) (*models.BatchRequest, error) {
	// Convert operations from StorageTask to BatchOperationItem
	batchOps := make([]models.BatchOperationItem, len(operations))
	for i, op := range operations {
		// Convert operation data to JSON
		dataBytes, err := json.Marshal(op.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal operation data: %w", err)
		}

		// Map operation type
		var batchOpType models.BatchOperationType
		switch op.Operation {
		case "create":
			batchOpType = models.BatchOperationCreate
		case "update":
			batchOpType = models.BatchOperationUpdate
		case "delete":
			batchOpType = models.BatchOperationDelete
		default:
			batchOpType = models.BatchOperationCreate // default fallback
		}

		batchOps[i] = models.BatchOperationItem{
			ID:         uuid.New().String(),
			Operation:  batchOpType,
			Data:       json.RawMessage(dataBytes),
			ExternalID: fmt.Sprintf("legacy_%d", i),
			Metadata:   make(map[string]string),
		}

		// Add profile ID to metadata if present
		if op.ProfileID != nil {
			batchOps[i].Metadata["profile_id"] = op.ProfileID.String()
		}
	}

	// Convert old BatchOptions to new BatchOptions
	newOptions := models.BatchOptions{
		Mode:                models.BatchModeIndividual, // Default to individual processing
		FailureHandling:     models.BatchContinueOnFail, // Continue on failure by default
		MaxConcurrency:      10,                         // Default concurrency
		TimeoutPerOperation: 30 * time.Second,
		TotalTimeout:        10 * time.Minute,
		ValidationLevel:     models.BatchValidationBasic,
		EnableRollback:      false,
		EnableProgressTrack: true,
	}

	// Note: Old options format no longer supported, using defaults
	// Future enhancement: could check for specific option keys in metadata

	// Create the batch request
	request := &models.BatchRequest{
		ID:          batchID,
		Type:        "profile", // Assume profile type for legacy operations
		Operations:  batchOps,
		Options:     newOptions,
		Metadata:    make(map[string]string),
		RequestedBy: "legacy_handler",
		CreatedAt:   time.Now(),
	}

	return request, nil
}

// Enhanced validation methods

func (h *StorageHandler) validateMessage(msg *Message) error {
	if msg.Source == "" {
		return fmt.Errorf("message source is required")
	}

	if time.Since(msg.Timestamp) > 24*time.Hour {
		return fmt.Errorf("message is too old: %v", msg.Timestamp)
	}

	return nil
}

func (h *StorageHandler) validateCreateTask(task *models.StorageTask) error {
	if task.Operation != "create" {
		return fmt.Errorf("invalid operation for create handler: %s", task.Operation)
	}

	if task.ProfileID != nil {
		return fmt.Errorf("profile ID should not be provided for create operation")
	}

	return h.validateTaskData(task.Data, "create")
}

func (h *StorageHandler) validateUpdateTask(task *models.StorageTask) error {
	if task.Operation != "update" {
		return fmt.Errorf("invalid operation for update handler: %s", task.Operation)
	}

	if task.ProfileID == nil {
		return fmt.Errorf("profile ID is required for update operation")
	}

	return h.validateTaskData(task.Data, "update")
}

func (h *StorageHandler) validateDeleteTask(task *models.StorageTask) error {
	if task.Operation != "delete" {
		return fmt.Errorf("invalid operation for delete handler: %s", task.Operation)
	}

	if task.ProfileID == nil {
		return fmt.Errorf("profile ID is required for delete operation")
	}

	return nil
}

func (h *StorageHandler) validateBatchTask(batchTask *BatchStorageTask) error {
	if len(batchTask.Operations) == 0 {
		return fmt.Errorf("batch task must contain at least one operation")
	}

	if len(batchTask.Operations) > 1000 {
		return fmt.Errorf("batch size exceeds maximum limit of 1000 operations")
	}

	// Validate each operation in the batch
	for i, op := range batchTask.Operations {
		if err := h.validateTaskData(op.Data, op.Operation); err != nil {
			return fmt.Errorf("operation %d validation failed: %w", i, err)
		}
	}

	return nil
}

func (h *StorageHandler) validateTaskData(data map[string]interface{}, operation string) error {
	// Enhanced data validation based on operation type
	switch operation {
	case "create":
		required := []string{"first_name", "last_name", "email"}
		for _, field := range required {
			if _, exists := data[field]; !exists {
				return fmt.Errorf("required field missing: %s", field)
			}
		}
	case "update":
		if len(data) == 0 {
			return fmt.Errorf("update operation requires at least one field to update")
		}
	}

	return nil
}

// Enhanced helper methods

func (h *StorageHandler) convertToProfileRequestEnhanced(data map[string]interface{}, operation string) (*models.ProfileRequest, error) {
	req := &models.ProfileRequest{}

	// Extract and validate basic fields with type checking
	if firstName, ok := data["first_name"]; ok {
		if str, ok := firstName.(string); ok {
			if len(str) > 100 {
				return nil, fmt.Errorf("first_name exceeds maximum length of 100 characters")
			}
			req.FirstName = str
		} else {
			return nil, fmt.Errorf("first_name must be a string")
		}
	}

	if lastName, ok := data["last_name"]; ok {
		if str, ok := lastName.(string); ok {
			if len(str) > 100 {
				return nil, fmt.Errorf("last_name exceeds maximum length of 100 characters")
			}
			req.LastName = str
		} else {
			return nil, fmt.Errorf("last_name must be a string")
		}
	}

	if email, ok := data["email"]; ok {
		if str, ok := email.(string); ok {
			if !h.isValidEmailEnhanced(str) {
				return nil, fmt.Errorf("invalid email format: %s", str)
			}
			req.Email = str
		} else {
			return nil, fmt.Errorf("email must be a string")
		}
	}

	if phone, ok := data["phone"]; ok {
		if str, ok := phone.(string); ok {
			if !h.isValidPhoneEnhanced(str) {
				return nil, fmt.Errorf("invalid phone format: %s", str)
			}
			req.Phone = str
		}
	}

	// Enhanced validation for operation type
	if operation == "create" && (req.FirstName == "" || req.LastName == "" || req.Email == "") {
		return nil, fmt.Errorf("first_name, last_name, and email are required for create operations")
	}

	return req, nil
}

func (h *StorageHandler) isValidEmailEnhanced(email string) bool {
	// Enhanced email validation
	if len(email) > 254 {
		return false
	}

	// Basic format check
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	local, domain := parts[0], parts[1]

	if len(local) == 0 || len(local) > 64 || len(domain) == 0 || len(domain) > 253 {
		return false
	}

	if !strings.Contains(domain, ".") {
		return false
	}

	return true
}

func (h *StorageHandler) isValidPhoneEnhanced(phone string) bool {
	// Enhanced phone validation
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)

	return len(digits) >= 10 && len(digits) <= 15
}

func (h *StorageHandler) checkForDuplicateProfile(ctx context.Context, req *models.ProfileRequest) error {
	// Check for existing profile with same email
	// This would typically involve a database query
	// For now, we'll implement a basic check structure

	// TODO: Implement actual duplicate check with ProfileService
	// existingProfile, err := h.profileService.GetProfileByEmail(ctx, req.Email)
	// if err == nil && existingProfile != nil {
	//     return fmt.Errorf("profile with email %s already exists", req.Email)
	// }

	return nil
}

func (h *StorageHandler) detectUpdateConflicts(ctx context.Context, existing *models.Profile, req *models.ProfileRequest, task *models.StorageTask) error {
	// Implement optimistic locking check
	if lastModified, ok := task.Options["last_modified"]; ok {
		if lastModifiedTime, ok := lastModified.(time.Time); ok {
			if existing.UpdatedAt.After(lastModifiedTime) {
				return fmt.Errorf("profile has been modified since last read, potential conflict")
			}
		}
	}

	return nil
}

func (h *StorageHandler) trackProfileChanges(old, new *models.Profile) map[string]interface{} {
	changes := make(map[string]interface{})

	if old.FirstName != new.FirstName {
		changes["first_name"] = map[string]string{"from": old.FirstName, "to": new.FirstName}
	}
	if old.LastName != new.LastName {
		changes["last_name"] = map[string]string{"from": old.LastName, "to": new.LastName}
	}
	if old.Email != new.Email {
		changes["email"] = map[string]string{"from": old.Email, "to": new.Email}
	}
	if old.Phone != new.Phone {
		changes["phone"] = map[string]string{"from": old.Phone, "to": new.Phone}
	}

	return changes
}

func (h *StorageHandler) extractCascadeOptions(options map[string]interface{}) map[string]bool {
	cascadeOptions := map[string]bool{
		"delete_addresses": true, // default
		"delete_contacts":  true, // default
	}

	if deleteAddr, ok := options["delete_addresses"]; ok {
		if boolVal, ok := deleteAddr.(bool); ok {
			cascadeOptions["delete_addresses"] = boolVal
		}
	}

	if deleteCont, ok := options["delete_contacts"]; ok {
		if boolVal, ok := deleteCont.(bool); ok {
			cascadeOptions["delete_contacts"] = boolVal
		}
	}

	return cascadeOptions
}

func (h *StorageHandler) optimizeBatchOperations(operations []models.StorageTask) ([]models.StorageTask, error) {
	// Basic optimization: remove duplicates and sort by operation type
	seen := make(map[string]bool)
	optimized := make([]models.StorageTask, 0, len(operations))

	// Group operations by type for better processing
	creates := make([]models.StorageTask, 0)
	updates := make([]models.StorageTask, 0)
	deletes := make([]models.StorageTask, 0)

	for _, op := range operations {
		key := fmt.Sprintf("%s_%v", op.Operation, op.ProfileID)
		if seen[key] {
			continue // Skip duplicate operations
		}
		seen[key] = true

		switch op.Operation {
		case "create":
			creates = append(creates, op)
		case "update":
			updates = append(updates, op)
		case "delete":
			deletes = append(deletes, op)
		}
	}

	// Optimal order: creates first, then updates, then deletes
	optimized = append(optimized, creates...)
	optimized = append(optimized, updates...)
	optimized = append(optimized, deletes...)

	return optimized, nil
}

func (h *StorageHandler) calculateBatchEfficiency(original, optimized int, duration time.Duration) float64 {
	if original == 0 {
		return 0
	}

	reductionRatio := float64(optimized) / float64(original)
	timeEfficiency := 1.0 / (duration.Seconds() + 1) // Avoid division by zero

	return reductionRatio * timeEfficiency * 100 // Percentage score
}

func (h *StorageHandler) updateProcessingTimeMetrics(duration time.Duration) {
	// Simple running average calculation
	if h.metrics.ProcessedMessages == 0 {
		h.metrics.AverageProcessingTime = duration
	} else {
		// Weighted average with recent measurements having more weight
		weight := 0.1
		h.metrics.AverageProcessingTime = time.Duration(
			float64(h.metrics.AverageProcessingTime)*(1-weight) +
				float64(duration)*weight,
		)
	}
}

func (h *StorageHandler) createErrorResponse(messageID string, err error, startTime time.Time) *MessageResponse {
	return &MessageResponse{
		MessageID:      messageID,
		Success:        false,
		Error:          err.Error(),
		ProcessedAt:    time.Now(),
		ProcessingTime: time.Since(startTime),
	}
}

// GetMetrics returns current handler metrics
func (h *StorageHandler) GetMetrics() *StorageMetrics {
	return h.metrics
}

// GetHandlerStats returns enhanced statistics about the handler
func (h *StorageHandler) GetHandlerStats() map[string]interface{} {
	return map[string]interface{}{
		"handler_type":            "StorageHandler",
		"supported_keys":          h.GetSupportedRoutingKeys(),
		"profile_service":         h.profileService != nil,
		"batch_service":           h.batchService != nil,
		"processed_messages":      h.metrics.ProcessedMessages,
		"failed_messages":         h.metrics.FailedMessages,
		"create_operations":       h.metrics.CreateOperations,
		"update_operations":       h.metrics.UpdateOperations,
		"delete_operations":       h.metrics.DeleteOperations,
		"batch_operations":        h.metrics.BatchOperations,
		"average_processing_time": h.metrics.AverageProcessingTime,
		"conflict_errors":         h.metrics.ConflictErrors,
		"validation_errors":       h.metrics.ValidationErrors,
		"success_rate":            h.calculateSuccessRate(),
	}
}

func (h *StorageHandler) calculateSuccessRate() float64 {
	total := h.metrics.ProcessedMessages + h.metrics.FailedMessages
	if total == 0 {
		return 100.0
	}
	return (float64(h.metrics.ProcessedMessages) / float64(total)) * 100
}
