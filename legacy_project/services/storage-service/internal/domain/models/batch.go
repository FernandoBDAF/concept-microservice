package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// BatchOperationType represents the type of batch operation
type BatchOperationType string

const (
	BatchOperationCreate BatchOperationType = "create"
	BatchOperationUpdate BatchOperationType = "update"
	BatchOperationDelete BatchOperationType = "delete"
	BatchOperationUpsert BatchOperationType = "upsert"
)

// BatchProcessingMode represents how batch operations should be processed
type BatchProcessingMode string

const (
	BatchModeTransactional BatchProcessingMode = "transactional" // All operations in single transaction
	BatchModeIndividual    BatchProcessingMode = "individual"    // Each operation separately
	BatchModeParallel      BatchProcessingMode = "parallel"      // Parallel processing with controlled concurrency
)

// BatchFailureHandling represents how failures should be handled
type BatchFailureHandling string

const (
	BatchFailOnFirst    BatchFailureHandling = "fail_on_first"    // Stop on first failure
	BatchContinueOnFail BatchFailureHandling = "continue_on_fail" // Continue processing after failures
	BatchPartialSuccess BatchFailureHandling = "partial_success"  // Allow partial success with rollback options
)

// BatchRequest represents a batch operation request
type BatchRequest struct {
	ID          string               `json:"id"`
	Type        string               `json:"type"` // "profile" only
	Operations  []BatchOperationItem `json:"operations" validate:"required,min=1,max=1000"`
	Options     BatchOptions         `json:"options"`
	Metadata    map[string]string    `json:"metadata,omitempty"`
	RequestedBy string               `json:"requested_by,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
}

// BatchOperationItem represents a single operation within a batch
type BatchOperationItem struct {
	ID         string             `json:"id"`
	Operation  BatchOperationType `json:"operation" validate:"required,oneof=create update delete upsert"`
	Data       json.RawMessage    `json:"data" validate:"required"`
	ExternalID string             `json:"external_id,omitempty"` // For client reference
	DependsOn  []string           `json:"depends_on,omitempty"`  // Operation dependencies
	Metadata   map[string]string  `json:"metadata,omitempty"`
}

// BatchOptions represents processing options for batch operations
type BatchOptions struct {
	Mode                BatchProcessingMode  `json:"mode" validate:"required,oneof=transactional individual parallel"`
	FailureHandling     BatchFailureHandling `json:"failure_handling" validate:"required"`
	MaxConcurrency      int                  `json:"max_concurrency" validate:"min=1,max=50"`
	TimeoutPerOperation time.Duration        `json:"timeout_per_operation"`
	TotalTimeout        time.Duration        `json:"total_timeout"`
	RetryPolicy         *BatchRetryPolicy    `json:"retry_policy,omitempty"`
	ValidationLevel     BatchValidationLevel `json:"validation_level"`
	EnableRollback      bool                 `json:"enable_rollback"`
	EnableProgressTrack bool                 `json:"enable_progress_tracking"`
}

// BatchRetryPolicy defines retry behavior for failed operations
type BatchRetryPolicy struct {
	MaxRetries      int           `json:"max_retries" validate:"min=0,max=10"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor" validate:"min=1.0,max=5.0"`
	RetryableErrors []string      `json:"retryable_errors,omitempty"`
}

// BatchValidationLevel represents the level of validation to perform
type BatchValidationLevel string

const (
	BatchValidationNone    BatchValidationLevel = "none"    // No validation
	BatchValidationBasic   BatchValidationLevel = "basic"   // Basic field validation
	BatchValidationStrict  BatchValidationLevel = "strict"  // Full business rule validation
	BatchValidationPreview BatchValidationLevel = "preview" // Validate without executing
)

// BatchResult represents the result of a batch operation
type BatchResult struct {
	ID              string                 `json:"id"`
	RequestID       string                 `json:"request_id"`
	Status          BatchStatus            `json:"status"`
	TotalOperations int                    `json:"total_operations"`
	SuccessfulOps   int                    `json:"successful_operations"`
	FailedOps       int                    `json:"failed_operations"`
	SkippedOps      int                    `json:"skipped_operations"`
	Results         []BatchOperationResult `json:"results"`
	Errors          []BatchError           `json:"errors,omitempty"`
	Warnings        []BatchWarning         `json:"warnings,omitempty"`
	ProcessingStats BatchProcessingStats   `json:"processing_stats"`
	RollbackInfo    *BatchRollbackInfo     `json:"rollback_info,omitempty"`
	StartedAt       time.Time              `json:"started_at"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	Duration        time.Duration          `json:"duration"`
}

// BatchStatus represents the status of a batch operation
type BatchStatus string

const (
	BatchStatusPending    BatchStatus = "pending"
	BatchStatusProcessing BatchStatus = "processing"
	BatchStatusCompleted  BatchStatus = "completed"
	BatchStatusFailed     BatchStatus = "failed"
	BatchStatusPartial    BatchStatus = "partial_success"
	BatchStatusCancelled  BatchStatus = "cancelled"
	BatchStatusRolledBack BatchStatus = "rolled_back"
)

// BatchOperationResult represents the result of a single operation
type BatchOperationResult struct {
	ID          string             `json:"id"`
	ExternalID  string             `json:"external_id,omitempty"`
	Operation   BatchOperationType `json:"operation"`
	Status      OperationStatus    `json:"status"`
	Data        json.RawMessage    `json:"data,omitempty"`
	Error       *BatchError        `json:"error,omitempty"`
	ProcessedAt time.Time          `json:"processed_at"`
	Duration    time.Duration      `json:"duration"`
	RetryCount  int                `json:"retry_count"`
}

// OperationStatus represents the status of a single operation
type OperationStatus string

const (
	OperationStatusSuccess    OperationStatus = "success"
	OperationStatusFailed     OperationStatus = "failed"
	OperationStatusSkipped    OperationStatus = "skipped"
	OperationStatusRetrying   OperationStatus = "retrying"
	OperationStatusRolledBack OperationStatus = "rolled_back"
)

// BatchError represents an error in batch processing
type BatchError struct {
	OperationID string    `json:"operation_id,omitempty"`
	ExternalID  string    `json:"external_id,omitempty"`
	Code        string    `json:"code"`
	Message     string    `json:"message"`
	Details     string    `json:"details,omitempty"`
	Retryable   bool      `json:"retryable"`
	Timestamp   time.Time `json:"timestamp"`
}

// BatchWarning represents a warning in batch processing
type BatchWarning struct {
	OperationID string    `json:"operation_id,omitempty"`
	ExternalID  string    `json:"external_id,omitempty"`
	Code        string    `json:"code"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

// BatchProcessingStats contains statistics about batch processing
type BatchProcessingStats struct {
	QueueTime           time.Duration `json:"queue_time"`
	ProcessingTime      time.Duration `json:"processing_time"`
	ValidationTime      time.Duration `json:"validation_time"`
	DatabaseTime        time.Duration `json:"database_time"`
	TotalTime           time.Duration `json:"total_time"`
	AvgOpTime           time.Duration `json:"average_operation_time"`
	ThroughputOpsPerSec float64       `json:"throughput_ops_per_second"`
	PeakMemoryUsage     int64         `json:"peak_memory_usage_bytes"`
	DatabaseConnections int           `json:"database_connections_used"`
	CacheHitRate        float64       `json:"cache_hit_rate,omitempty"`
}

// BatchRollbackInfo contains information about rollback operations
type BatchRollbackInfo struct {
	Enabled             bool                `json:"enabled"`
	RollbackOperations  []RollbackOperation `json:"rollback_operations,omitempty"`
	RollbackStatus      RollbackStatus      `json:"rollback_status"`
	RollbackStartedAt   *time.Time          `json:"rollback_started_at,omitempty"`
	RollbackCompletedAt *time.Time          `json:"rollback_completed_at,omitempty"`
	RollbackErrors      []BatchError        `json:"rollback_errors,omitempty"`
}

// RollbackOperation represents a rollback operation for a successful operation
type RollbackOperation struct {
	OriginalOperationID string             `json:"original_operation_id"`
	RollbackType        BatchOperationType `json:"rollback_type"`
	RollbackData        json.RawMessage    `json:"rollback_data,omitempty"`
	Status              OperationStatus    `json:"status"`
	ExecutedAt          *time.Time         `json:"executed_at,omitempty"`
}

// RollbackStatus represents the status of rollback operations
type RollbackStatus string

const (
	RollbackStatusNone       RollbackStatus = "none"
	RollbackStatusPending    RollbackStatus = "pending"
	RollbackStatusInProgress RollbackStatus = "in_progress"
	RollbackStatusCompleted  RollbackStatus = "completed"
	RollbackStatusFailed     RollbackStatus = "failed"
	RollbackStatusPartial    RollbackStatus = "partial"
)

// Validate methods

// Validate validates the batch request
func (br *BatchRequest) Validate() error {
	if br.Type == "" {
		return fmt.Errorf("batch type is required")
	}

	if br.Type != "profile" {
		return fmt.Errorf("batch type must be 'profile'")
	}

	if len(br.Operations) == 0 {
		return fmt.Errorf("at least one operation is required")
	}

	if len(br.Operations) > 1000 {
		return fmt.Errorf("maximum 1000 operations allowed per batch")
	}

	// Validate options
	if err := br.Options.Validate(); err != nil {
		return fmt.Errorf("invalid batch options: %w", err)
	}

	// Validate each operation
	operationIDs := make(map[string]bool)
	for i, op := range br.Operations {
		if err := op.Validate(); err != nil {
			return fmt.Errorf("invalid operation at index %d: %w", i, err)
		}

		// Check for duplicate operation IDs
		if operationIDs[op.ID] {
			return fmt.Errorf("duplicate operation ID: %s", op.ID)
		}
		operationIDs[op.ID] = true
	}

	// Validate dependencies
	if err := br.validateDependencies(); err != nil {
		return fmt.Errorf("invalid dependencies: %w", err)
	}

	return nil
}

// Validate validates a batch operation item
func (boi *BatchOperationItem) Validate() error {
	if boi.ID == "" {
		return fmt.Errorf("operation ID is required")
	}

	if boi.Operation == "" {
		return fmt.Errorf("operation type is required")
	}

	validOps := map[BatchOperationType]bool{
		BatchOperationCreate: true,
		BatchOperationUpdate: true,
		BatchOperationDelete: true,
		BatchOperationUpsert: true,
	}

	if !validOps[boi.Operation] {
		return fmt.Errorf("invalid operation type: %s", boi.Operation)
	}

	if len(boi.Data) == 0 {
		return fmt.Errorf("operation data is required")
	}

	return nil
}

// Validate validates batch options
func (bo *BatchOptions) Validate() error {
	validModes := map[BatchProcessingMode]bool{
		BatchModeTransactional: true,
		BatchModeIndividual:    true,
		BatchModeParallel:      true,
	}

	if !validModes[bo.Mode] {
		return fmt.Errorf("invalid processing mode: %s", bo.Mode)
	}

	validFailureHandling := map[BatchFailureHandling]bool{
		BatchFailOnFirst:    true,
		BatchContinueOnFail: true,
		BatchPartialSuccess: true,
	}

	if !validFailureHandling[bo.FailureHandling] {
		return fmt.Errorf("invalid failure handling: %s", bo.FailureHandling)
	}

	if bo.MaxConcurrency < 1 || bo.MaxConcurrency > 50 {
		return fmt.Errorf("max concurrency must be between 1 and 50")
	}

	if bo.RetryPolicy != nil {
		if err := bo.RetryPolicy.Validate(); err != nil {
			return fmt.Errorf("invalid retry policy: %w", err)
		}
	}

	return nil
}

// Validate validates retry policy
func (brp *BatchRetryPolicy) Validate() error {
	if brp.MaxRetries < 0 || brp.MaxRetries > 10 {
		return fmt.Errorf("max retries must be between 0 and 10")
	}

	if brp.BackoffFactor < 1.0 || brp.BackoffFactor > 5.0 {
		return fmt.Errorf("backoff factor must be between 1.0 and 5.0")
	}

	return nil
}

// validateDependencies validates operation dependencies
func (br *BatchRequest) validateDependencies() error {
	operationIDs := make(map[string]bool)
	for _, op := range br.Operations {
		operationIDs[op.ID] = true
	}

	for _, op := range br.Operations {
		for _, depID := range op.DependsOn {
			if !operationIDs[depID] {
				return fmt.Errorf("operation %s depends on non-existent operation %s", op.ID, depID)
			}
		}
	}

	// Check for circular dependencies (simplified check)
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	var hasCycle func(string) bool
	hasCycle = func(opID string) bool {
		visited[opID] = true
		recursionStack[opID] = true

		// Find the operation
		var op *BatchOperationItem
		for i := range br.Operations {
			if br.Operations[i].ID == opID {
				op = &br.Operations[i]
				break
			}
		}

		if op != nil {
			for _, depID := range op.DependsOn {
				if !visited[depID] {
					if hasCycle(depID) {
						return true
					}
				} else if recursionStack[depID] {
					return true
				}
			}
		}

		recursionStack[opID] = false
		return false
	}

	for _, op := range br.Operations {
		if !visited[op.ID] {
			if hasCycle(op.ID) {
				return fmt.Errorf("circular dependency detected involving operation %s", op.ID)
			}
		}
	}

	return nil
}

// NewBatchRequest creates a new batch request with default values
func NewBatchRequest(batchType string, operations []BatchOperationItem) *BatchRequest {
	return &BatchRequest{
		ID:         uuid.New().String(),
		Type:       batchType,
		Operations: operations,
		Options: BatchOptions{
			Mode:                BatchModeIndividual,
			FailureHandling:     BatchContinueOnFail,
			MaxConcurrency:      5,
			TimeoutPerOperation: 30 * time.Second,
			TotalTimeout:        10 * time.Minute,
			ValidationLevel:     BatchValidationBasic,
			EnableRollback:      false,
			EnableProgressTrack: true,
		},
		CreatedAt: time.Now(),
	}
}

// DefaultBatchOptions returns default batch processing options
func DefaultBatchOptions() BatchOptions {
	return BatchOptions{
		Mode:                BatchModeIndividual,
		FailureHandling:     BatchContinueOnFail,
		MaxConcurrency:      5,
		TimeoutPerOperation: 30 * time.Second,
		TotalTimeout:        10 * time.Minute,
		ValidationLevel:     BatchValidationBasic,
		EnableRollback:      false,
		EnableProgressTrack: true,
	}
}

// IsCompleted returns whether the batch operation has completed
func (br *BatchResult) IsCompleted() bool {
	return br.Status == BatchStatusCompleted ||
		br.Status == BatchStatusFailed ||
		br.Status == BatchStatusPartial ||
		br.Status == BatchStatusCancelled ||
		br.Status == BatchStatusRolledBack
}

// IsSuccessful returns whether the batch operation was successful
func (br *BatchResult) IsSuccessful() bool {
	return br.Status == BatchStatusCompleted && br.FailedOps == 0
}

// HasPartialSuccess returns whether the batch had partial success
func (br *BatchResult) HasPartialSuccess() bool {
	return br.Status == BatchStatusPartial || (br.SuccessfulOps > 0 && br.FailedOps > 0)
}

// GetSuccessRate returns the success rate as a percentage
func (br *BatchResult) GetSuccessRate() float64 {
	if br.TotalOperations == 0 {
		return 0.0
	}
	return float64(br.SuccessfulOps) / float64(br.TotalOperations) * 100.0
}
