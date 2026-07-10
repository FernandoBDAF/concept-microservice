package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/domain/models"
	"microservices/services/profile-storage/internal/domain/service"
	"microservices/services/profile-storage/internal/pkg/logger"
)

// BatchHandler handles batch operation HTTP requests
type BatchHandler struct {
	batchService *service.AdvancedBatchOperationsService
	log          *zap.Logger
}

// NewBatchHandler creates a new batch handler
func NewBatchHandler(batchService *service.AdvancedBatchOperationsService) *BatchHandler {
	return &BatchHandler{
		batchService: batchService,
		log:          logger.Get().Named("batch_handler"),
	}
}

// RegisterRoutes registers batch routes with the router
func (h *BatchHandler) RegisterRoutes(router *mux.Router) {
	// Profile batch operations
	router.HandleFunc("/api/v1/profiles/batch", h.ProcessProfileBatch).Methods("POST")
	router.HandleFunc("/api/v1/profiles/batch/{batch_id}/status", h.GetBatchStatus).Methods("GET")
	router.HandleFunc("/api/v1/profiles/batch/{batch_id}/cancel", h.CancelBatch).Methods("POST")

	// Auth batch operations
	router.HandleFunc("/api/v1/auth/users/batch", h.ProcessAuthBatch).Methods("POST")
	router.HandleFunc("/api/v1/auth/batch/{batch_id}/status", h.GetBatchStatus).Methods("GET")
	router.HandleFunc("/api/v1/auth/batch/{batch_id}/cancel", h.CancelBatch).Methods("POST")

	// General batch endpoints
	router.HandleFunc("/api/v1/batch", h.ProcessGenericBatch).Methods("POST")
	router.HandleFunc("/api/v1/batch/{batch_id}", h.GetBatchResult).Methods("GET")
	router.HandleFunc("/api/v1/batch/{batch_id}/status", h.GetBatchStatus).Methods("GET")
	router.HandleFunc("/api/v1/batch/{batch_id}/cancel", h.CancelBatch).Methods("POST")
	router.HandleFunc("/api/v1/batch/metrics", h.GetBatchMetrics).Methods("GET")

	// Batch validation (preview mode)
	router.HandleFunc("/api/v1/batch/validate", h.ValidateBatch).Methods("POST")
}

// ProcessProfileBatch handles profile batch operations
func (h *BatchHandler) ProcessProfileBatch(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Processing profile batch request")

	var req models.BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Failed to decode batch request", logger.ErrorField(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", err.Error())
		return
	}

	// Set batch type to profile
	req.Type = "profile"

	// Set default options if not provided
	if req.Options.Mode == "" {
		req.Options = models.DefaultBatchOptions()
	}

	// Validate request
	if err := req.Validate(); err != nil {
		h.log.Error("Invalid batch request", logger.ErrorField(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_FAILED", "Batch request validation failed", err.Error())
		return
	}

	// Process batch
	result, err := h.batchService.ProcessBatch(r.Context(), &req)
	if err != nil {
		h.log.Error("Failed to process batch", logger.ErrorField(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "BATCH_PROCESSING_FAILED", "Failed to process batch", err.Error())
		return
	}

	h.log.Info("Profile batch processed successfully",
		logger.String("batch_id", result.RequestID),
		logger.String("status", string(result.Status)),
		logger.Int("successful_ops", result.SuccessfulOps),
		logger.Int("failed_ops", result.FailedOps),
	)

	h.writeSuccessResponse(w, http.StatusAccepted, result, "Batch processing completed")
}

// ProcessAuthBatch handles auth batch operations
func (h *BatchHandler) ProcessAuthBatch(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Processing auth batch request")

	var req models.BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Failed to decode batch request", logger.ErrorField(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", err.Error())
		return
	}

	// Set batch type to auth
	req.Type = "auth"

	// Set default options if not provided
	if req.Options.Mode == "" {
		req.Options = models.DefaultBatchOptions()
	}

	// Validate request
	if err := req.Validate(); err != nil {
		h.log.Error("Invalid batch request", logger.ErrorField(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_FAILED", "Batch request validation failed", err.Error())
		return
	}

	// Process batch
	result, err := h.batchService.ProcessBatch(r.Context(), &req)
	if err != nil {
		h.log.Error("Failed to process batch", logger.ErrorField(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "BATCH_PROCESSING_FAILED", "Failed to process batch", err.Error())
		return
	}

	h.log.Info("Auth batch processed successfully",
		logger.String("batch_id", result.RequestID),
		logger.String("status", string(result.Status)),
		logger.Int("successful_ops", result.SuccessfulOps),
		logger.Int("failed_ops", result.FailedOps),
	)

	h.writeSuccessResponse(w, http.StatusAccepted, result, "Batch processing completed")
}

// ProcessGenericBatch handles generic batch operations (type specified in request)
func (h *BatchHandler) ProcessGenericBatch(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Processing generic batch request")

	var req models.BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Failed to decode batch request", logger.ErrorField(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", err.Error())
		return
	}

	// Set default options if not provided
	if req.Options.Mode == "" {
		req.Options = models.DefaultBatchOptions()
	}

	// Validate request
	if err := req.Validate(); err != nil {
		h.log.Error("Invalid batch request", logger.ErrorField(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_FAILED", "Batch request validation failed", err.Error())
		return
	}

	// Process batch
	result, err := h.batchService.ProcessBatch(r.Context(), &req)
	if err != nil {
		h.log.Error("Failed to process batch", logger.ErrorField(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "BATCH_PROCESSING_FAILED", "Failed to process batch", err.Error())
		return
	}

	h.log.Info("Generic batch processed successfully",
		logger.String("batch_id", result.RequestID),
		logger.String("batch_type", req.Type),
		logger.String("status", string(result.Status)),
		logger.Int("successful_ops", result.SuccessfulOps),
		logger.Int("failed_ops", result.FailedOps),
	)

	h.writeSuccessResponse(w, http.StatusAccepted, result, "Batch processing completed")
}

// GetBatchResult retrieves the result of a batch operation
func (h *BatchHandler) GetBatchResult(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	batchID := vars["batch_id"]

	h.log.Debug("Getting batch result", logger.String("batch_id", batchID))

	// For now, return status since we don't persist batch results
	// In a production system, you'd store and retrieve batch results from a database
	job, exists := h.batchService.GetBatchStatus(batchID)
	if !exists {
		h.writeErrorResponse(w, http.StatusNotFound, "BATCH_NOT_FOUND", "Batch not found", fmt.Sprintf("No batch found with ID: %s", batchID))
		return
	}

	response := map[string]interface{}{
		"batch_id":      batchID,
		"status":        job.Status,
		"progress":      job.Progress,
		"current_op":    job.CurrentOp,
		"total_ops":     job.TotalOps,
		"start_time":    job.StartTime,
		"success_count": job.SuccessCount,
		"errors_count":  job.ErrorsCount,
	}

	h.writeSuccessResponse(w, http.StatusOK, response, "Batch result retrieved successfully")
}

// GetBatchStatus retrieves the current status of a batch operation
func (h *BatchHandler) GetBatchStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	batchID := vars["batch_id"]

	h.log.Debug("Getting batch status", logger.String("batch_id", batchID))

	job, exists := h.batchService.GetBatchStatus(batchID)
	if !exists {
		h.writeErrorResponse(w, http.StatusNotFound, "BATCH_NOT_FOUND", "Batch not found", fmt.Sprintf("No active batch found with ID: %s", batchID))
		return
	}

	status := map[string]interface{}{
		"batch_id":   batchID,
		"status":     job.Status,
		"progress":   job.Progress,
		"current_op": job.CurrentOp,
		"total_ops":  job.TotalOps,
		"start_time": job.StartTime,
		"elapsed":    time.Since(job.StartTime),
	}

	h.writeSuccessResponse(w, http.StatusOK, status, "Batch status retrieved successfully")
}

// CancelBatch cancels a running batch operation
func (h *BatchHandler) CancelBatch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	batchID := vars["batch_id"]

	h.log.Info("Cancelling batch operation", logger.String("batch_id", batchID))

	err := h.batchService.CancelBatch(batchID)
	if err != nil {
		h.log.Error("Failed to cancel batch", logger.ErrorField(err))
		if strings.Contains(err.Error(), "not found") {
			h.writeErrorResponse(w, http.StatusNotFound, "BATCH_NOT_FOUND", "Batch not found", err.Error())
		} else {
			h.writeErrorResponse(w, http.StatusInternalServerError, "CANCEL_FAILED", "Failed to cancel batch", err.Error())
		}
		return
	}

	h.log.Info("Batch cancelled successfully", logger.String("batch_id", batchID))
	h.writeSuccessResponse(w, http.StatusOK, map[string]string{"batch_id": batchID, "status": "cancelled"}, "Batch cancelled successfully")
}

// GetBatchMetrics returns batch processing metrics
func (h *BatchHandler) GetBatchMetrics(w http.ResponseWriter, r *http.Request) {
	h.log.Debug("Getting batch metrics")

	metrics := h.batchService.GetMetrics()

	h.writeSuccessResponse(w, http.StatusOK, metrics, "Batch metrics retrieved successfully")
}

// ValidateBatch validates a batch request without processing it (preview mode)
func (h *BatchHandler) ValidateBatch(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Validating batch request")

	var req models.BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Failed to decode batch request", logger.ErrorField(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body", err.Error())
		return
	}

	// Set validation level to preview
	req.Options.ValidationLevel = models.BatchValidationPreview

	// Validate request
	if err := req.Validate(); err != nil {
		h.log.Error("Batch validation failed", logger.ErrorField(err))

		validationResult := map[string]interface{}{
			"valid":   false,
			"errors":  []string{err.Error()},
			"request": req,
		}

		h.writeSuccessResponse(w, http.StatusOK, validationResult, "Batch validation completed")
		return
	}

	// Additional validation logic could go here
	// For now, if basic validation passes, consider it valid
	validationResult := map[string]interface{}{
		"valid":              true,
		"total_operations":   len(req.Operations),
		"estimated_duration": h.estimateBatchDuration(&req),
		"processing_mode":    req.Options.Mode,
		"failure_handling":   req.Options.FailureHandling,
		"warnings":           h.generateValidationWarnings(&req),
		"request":            req,
	}

	h.writeSuccessResponse(w, http.StatusOK, validationResult, "Batch validation completed successfully")
}

// Helper methods

// estimateBatchDuration estimates how long a batch might take to process
func (h *BatchHandler) estimateBatchDuration(req *models.BatchRequest) time.Duration {
	// Simple estimation based on operation count and type
	baseTimePerOp := 100 * time.Millisecond

	switch req.Options.Mode {
	case models.BatchModeTransactional:
		// Transactions have overhead
		baseTimePerOp = 150 * time.Millisecond
	case models.BatchModeParallel:
		// Parallel processing is faster but has concurrency overhead
		parallelFactor := float64(req.Options.MaxConcurrency)
		if parallelFactor == 0 {
			parallelFactor = 5
		}
		baseTimePerOp = time.Duration(float64(baseTimePerOp) / parallelFactor * 1.2)
	}

	return time.Duration(len(req.Operations)) * baseTimePerOp
}

// generateValidationWarnings generates warnings for potentially problematic configurations
func (h *BatchHandler) generateValidationWarnings(req *models.BatchRequest) []string {
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

// writeSuccessResponse writes a standardized success response
func (h *BatchHandler) writeSuccessResponse(w http.ResponseWriter, statusCode int, data interface{}, message string) {
	response := map[string]interface{}{
		"success":   true,
		"data":      data,
		"message":   message,
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// writeErrorResponse writes a standardized error response
func (h *BatchHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message, details string) {
	response := map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"code":    errorCode,
			"message": message,
			"details": details,
		},
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
