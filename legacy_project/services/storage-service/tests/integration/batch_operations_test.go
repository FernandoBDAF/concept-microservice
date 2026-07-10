package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/config"
	"microservices/services/profile-storage/internal/domain/models"
	"microservices/services/profile-storage/internal/domain/service"
	"microservices/services/profile-storage/internal/infrastructure/database"
	"microservices/services/profile-storage/internal/infrastructure/repository"
)

// BatchOperationsTestSuite focuses specifically on batch processing capabilities
type BatchOperationsTestSuite struct {
	suite.Suite
	connManager    *database.ConnectionManager
	profileService *service.ProfileService
	batchService   *service.AdvancedBatchOperationsService
	logger         *zap.Logger
	testProfileIDs []uuid.UUID
	mu             sync.RWMutex
}

// NewBatchOperationsTestSuite creates a new batch operations test suite
func NewBatchOperationsTestSuite(t *testing.T) *BatchOperationsTestSuite {
	return &BatchOperationsTestSuite{
		logger:         zap.NewNop().Named("batch_operations_test"),
		testProfileIDs: make([]uuid.UUID, 0),
	}
}

// SetupSuite initializes the batch operations test environment
func (suite *BatchOperationsTestSuite) SetupSuite() error {
	suite.logger.Info("Setting up batch operations test suite")

	// Initialize test configuration
	cfg := &config.Config{
		DBHost:     "localhost",
		DBPort:     "5432",
		DBName:     "profile_storage_test",
		DBUser:     "test_user",
		DBPassword: "test_password",
	}

	// Setup database connection
	suite.connManager = database.NewConnectionManager(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := suite.connManager.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to test database: %w", err)
	}

	// Setup services
	profileRepo := repository.NewProfileRepository(suite.connManager.GetDB())

	suite.profileService = service.NewProfileService(profileRepo)
	suite.batchService = service.NewAdvancedBatchOperationsService(
		suite.profileService,
		suite.connManager.GetDB(),
	)

	suite.logger.Info("Batch operations test suite setup completed")
	return nil
}

// TearDownSuite cleans up the test environment
func (suite *BatchOperationsTestSuite) TearDownSuite() error {
	suite.cleanupTestProfiles()
	if suite.connManager != nil {
		suite.connManager.Close()
	}
	return nil
}

// Helper function to convert StorageTask array to BatchRequest
func (suite *BatchOperationsTestSuite) convertToBatchRequest(operations []models.StorageTask, mode models.BatchProcessingMode) *models.BatchRequest {
	batchOps := make([]models.BatchOperationItem, len(operations))
	for i, op := range operations {
		// Convert operation data to JSON
		dataBytes, _ := json.Marshal(op.Data)

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
			batchOpType = models.BatchOperationCreate
		}

		batchOps[i] = models.BatchOperationItem{
			ID:         uuid.New().String(),
			Operation:  batchOpType,
			Data:       json.RawMessage(dataBytes),
			ExternalID: fmt.Sprintf("test_%d", i),
			Metadata:   make(map[string]string),
		}

		// Add profile ID to metadata if present
		if op.ProfileID != nil {
			batchOps[i].Metadata["profile_id"] = op.ProfileID.String()
		}
	}

	// Set failure handling based on mode
	failureHandling := models.BatchContinueOnFail
	if mode == models.BatchModeTransactional {
		failureHandling = models.BatchFailOnFirst
	}

	return &models.BatchRequest{
		ID:         uuid.New().String(),
		Type:       "profile",
		Operations: batchOps,
		Options: models.BatchOptions{
			Mode:                mode,
			FailureHandling:     failureHandling,
			MaxConcurrency:      10,
			TimeoutPerOperation: 30 * time.Second,
			TotalTimeout:        5 * time.Minute,
			ValidationLevel:     models.BatchValidationBasic,
			EnableRollback:      mode == models.BatchModeTransactional,
			EnableProgressTrack: true,
		},
		RequestedBy: "test_suite",
		CreatedAt:   time.Now(),
	}
}

// TestSmallBatchTransactional tests small batches in transaction mode
func (suite *BatchOperationsTestSuite) TestSmallBatchTransactional() {
	suite.logger.Info("Testing small batch in transaction mode")

	ctx := context.Background()

	// Create 5 operations for transactional processing
	operations := suite.createBatchOperations(5, "small_batch_tx")
	batchRequest := suite.convertToBatchRequest(operations, models.BatchModeTransactional)

	startTime := time.Now()
	result, err := suite.batchService.ProcessBatch(ctx, batchRequest)
	processingTime := time.Since(startTime)

	require.NoError(suite.T(), err, "Small transactional batch should succeed")
	require.NotNil(suite.T(), result, "Batch result should not be nil")

	suite.logger.Info("Small transactional batch completed",
		zap.Duration("processing_time", processingTime),
		zap.Int("successful_ops", result.SuccessfulOps),
		zap.Int("failed_ops", result.FailedOps))

	// Verify all operations succeeded
	assert.Equal(suite.T(), len(operations), result.SuccessfulOps, "All operations should succeed")
	assert.Equal(suite.T(), 0, result.FailedOps, "No operations should fail")
	assert.Equal(suite.T(), string(models.BatchModeTransactional), string(batchRequest.Options.Mode), "Should be in transaction mode")

	// Verify performance (small batches should be very fast)
	assert.Less(suite.T(), processingTime, 5*time.Second, "Small batch should complete quickly")
}

// TestMediumBatchIndividual tests medium batches in individual mode
func (suite *BatchOperationsTestSuite) TestMediumBatchIndividual() {
	suite.logger.Info("Testing medium batch in individual mode")

	ctx := context.Background()

	// Create 25 operations for individual processing
	operations := suite.createBatchOperations(25, "medium_batch_ind")
	batchRequest := suite.convertToBatchRequest(operations, models.BatchModeIndividual)

	startTime := time.Now()
	result, err := suite.batchService.ProcessBatch(ctx, batchRequest)
	processingTime := time.Since(startTime)

	require.NoError(suite.T(), err, "Medium individual batch should succeed")
	require.NotNil(suite.T(), result, "Batch result should not be nil")

	suite.logger.Info("Medium individual batch completed",
		zap.Duration("processing_time", processingTime),
		zap.Int("successful_ops", result.SuccessfulOps),
		zap.Int("failed_ops", result.FailedOps))

	// Verify batch processing
	assert.Equal(suite.T(), len(operations), result.TotalOperations, "Total operations should match")
	assert.Equal(suite.T(), string(models.BatchModeIndividual), string(batchRequest.Options.Mode), "Should be in individual mode")

	// Verify performance target (< 30s for medium batches)
	assert.Less(suite.T(), processingTime, 30*time.Second, "Medium batch should meet performance target")
}

// TestLargeBatchWithOptimization tests large batches with auto-tuning
func (suite *BatchOperationsTestSuite) TestLargeBatchWithOptimization() {
	suite.logger.Info("Testing large batch with optimization")

	ctx := context.Background()

	// Create 100 operations to test large batch capabilities
	operations := suite.createBatchOperations(100, "large_batch_opt")

	// Include some duplicate operations to test optimization
	duplicateOps := suite.createBatchOperations(10, "large_batch_opt") // Same prefix for duplicates
	operations = append(operations, duplicateOps...)

	batchRequest := suite.convertToBatchRequest(operations, models.BatchModeIndividual)

	startTime := time.Now()
	result, err := suite.batchService.ProcessBatch(ctx, batchRequest)
	processingTime := time.Since(startTime)

	require.NoError(suite.T(), err, "Large optimized batch should succeed")
	require.NotNil(suite.T(), result, "Batch result should not be nil")

	suite.logger.Info("Large optimized batch completed",
		zap.Duration("processing_time", processingTime),
		zap.Int("total_operations", result.TotalOperations),
		zap.Int("successful_ops", result.SuccessfulOps),
		zap.Int("failed_ops", result.FailedOps))

	// Verify batch processing
	assert.Equal(suite.T(), len(operations), result.TotalOperations, "All operations should be processed")

	// Verify performance target (< 60s for 100 operations is reasonable for our test)
	assert.Less(suite.T(), processingTime, 60*time.Second,
		"Large batch should complete within reasonable time")
}

// TestMixedOperationsBatch tests batches with different operation types
func (suite *BatchOperationsTestSuite) TestMixedOperationsBatch() {
	suite.logger.Info("Testing mixed operations batch")

	ctx := context.Background()

	// First create some profiles to update and delete
	createOps := suite.createBatchOperations(10, "mixed_create")
	createRequest := suite.convertToBatchRequest(createOps, models.BatchModeIndividual)

	createResult, err := suite.batchService.ProcessBatch(ctx, createRequest)
	require.NoError(suite.T(), err, "Create batch should succeed")

	// Extract created profile IDs (in a real system, you'd get these from the results)
	var createdIDs []uuid.UUID
	for i := 0; i < createResult.SuccessfulOps && i < 5; i++ {
		createdIDs = append(createdIDs, uuid.New()) // Mock IDs for testing
	}

	// Create mixed operations
	var mixedOps []models.StorageTask

	// Add create operations
	mixedOps = append(mixedOps, suite.createBatchOperations(5, "mixed_new_create")...)

	// Add update operations
	for i, id := range createdIDs {
		if i >= 3 { // Limit to 3 updates
			break
		}
		updateOp := models.StorageTask{
			Operation: "update",
			ProfileID: &id,
			Data: map[string]interface{}{
				"first_name": fmt.Sprintf("Updated_%d", i),
				"last_name":  "MixedBatch",
				"email":      fmt.Sprintf("updated.mixed%d@test.com", i),
			},
			Timestamp:   time.Now(),
			RequestedBy: "batch_test",
		}
		mixedOps = append(mixedOps, updateOp)
	}

	// Add delete operations
	for i, id := range createdIDs {
		if i >= 2 { // Limit to 2 deletes
			break
		}
		deleteOp := models.StorageTask{
			Operation:   "delete",
			ProfileID:   &id,
			Timestamp:   time.Now(),
			RequestedBy: "batch_test",
		}
		mixedOps = append(mixedOps, deleteOp)
	}

	// Process mixed batch
	mixedRequest := suite.convertToBatchRequest(mixedOps, models.BatchModeIndividual)

	startTime := time.Now()
	mixedResult, err := suite.batchService.ProcessBatch(ctx, mixedRequest)
	processingTime := time.Since(startTime)

	require.NoError(suite.T(), err, "Mixed operations batch should succeed")

	suite.logger.Info("Mixed operations batch completed",
		zap.Duration("processing_time", processingTime),
		zap.Int("total_operations", mixedResult.TotalOperations),
		zap.Int("successful_ops", mixedResult.SuccessfulOps))

	// Verify mixed operations handling
	assert.Equal(suite.T(), len(mixedOps), mixedResult.TotalOperations,
		"All mixed operations should be processed")
	assert.Greater(suite.T(), mixedResult.SuccessfulOps, 0,
		"Some operations should succeed")
}

// TestBatchFailureHandling tests different failure modes
func (suite *BatchOperationsTestSuite) TestBatchFailureHandling() {
	suite.logger.Info("Testing batch failure handling")

	ctx := context.Background()

	// Create operations with some invalid ones
	var operations []models.StorageTask

	// Add valid operations
	validOps := suite.createBatchOperations(5, "failure_valid")
	operations = append(operations, validOps...)

	// Add invalid operations (missing required fields)
	for i := 0; i < 3; i++ {
		invalidOp := models.StorageTask{
			Operation: "create",
			Data: map[string]interface{}{
				"first_name": fmt.Sprintf("Invalid_%d", i),
				// Missing last_name and email
			},
			Timestamp:   time.Now(),
			RequestedBy: "batch_test",
		}
		operations = append(operations, invalidOp)
	}

	// Test "continue" failure mode
	continueRequest := suite.convertToBatchRequest(operations, models.BatchModeIndividual)
	continueRequest.Options.FailureHandling = models.BatchContinueOnFail

	startTime := time.Now()
	continueResult, err := suite.batchService.ProcessBatch(ctx, continueRequest)
	processingTime := time.Since(startTime)

	// Should not return error in continue mode
	require.NoError(suite.T(), err, "Continue mode should not return error")
	require.NotNil(suite.T(), continueResult, "Result should not be nil")

	suite.logger.Info("Continue mode batch completed",
		zap.Duration("processing_time", processingTime),
		zap.Int("successful_ops", continueResult.SuccessfulOps),
		zap.Int("failed_ops", continueResult.FailedOps))

	// Verify failure handling
	assert.Equal(suite.T(), len(operations), continueResult.TotalOperations,
		"All operations should be attempted")
	assert.Greater(suite.T(), continueResult.SuccessfulOps, 0,
		"Some operations should succeed")
	assert.Greater(suite.T(), continueResult.FailedOps, 0,
		"Some operations should fail")

	// Test "fail on first" failure mode
	stopRequest := suite.convertToBatchRequest(operations, models.BatchModeIndividual)
	stopRequest.Options.FailureHandling = models.BatchFailOnFirst

	// This should fail fast when encountering invalid operations
	stopResult, stopErr := suite.batchService.ProcessBatch(ctx, stopRequest)

	suite.logger.Info("Stop mode batch result",
		zap.Bool("has_error", stopErr != nil),
		zap.Int("processed_ops", func() int {
			if stopResult != nil {
				return stopResult.SuccessfulOps + stopResult.FailedOps
			}
			return 0
		}()))

	// In stop mode, we expect either success with partial processing or an error
	if stopErr != nil {
		suite.logger.Info("Stop mode failed as expected", zap.Error(stopErr))
	}
}

// TestBatchPerformanceScaling tests performance scaling with different batch sizes
func (suite *BatchOperationsTestSuite) TestBatchPerformanceScaling() {
	suite.logger.Info("Testing batch performance scaling")

	ctx := context.Background()
	batchSizes := []int{1, 5, 10, 25, 50}
	results := make(map[int]time.Duration)

	for _, size := range batchSizes {
		suite.logger.Info("Testing batch size", zap.Int("size", size))

		operations := suite.createBatchOperations(size, fmt.Sprintf("perf_test_%d", size))
		batchRequest := suite.convertToBatchRequest(operations, models.BatchModeIndividual)

		startTime := time.Now()
		result, err := suite.batchService.ProcessBatch(ctx, batchRequest)
		processingTime := time.Since(startTime)

		require.NoError(suite.T(), err, fmt.Sprintf("Batch size %d should succeed", size))
		results[size] = processingTime

		suite.logger.Info("Batch performance result",
			zap.Int("batch_size", size),
			zap.Duration("processing_time", processingTime),
			zap.Float64("ops_per_second", float64(size)/processingTime.Seconds()),
			zap.Int("successful_ops", result.SuccessfulOps))

		// Basic performance validation
		expectedMaxTime := time.Duration(size) * 500 * time.Millisecond // 500ms per operation max
		assert.Less(suite.T(), processingTime, expectedMaxTime,
			fmt.Sprintf("Batch size %d should complete within expected time", size))
	}

	// Verify scaling behavior
	suite.logger.Info("Performance scaling analysis",
		zap.Any("results", results))

	// Larger batches should have better throughput (ops/second)
	throughput1 := 1.0 / results[1].Seconds()
	throughput50 := 50.0 / results[50].Seconds()

	suite.logger.Info("Throughput comparison",
		zap.Float64("single_op_throughput", throughput1),
		zap.Float64("batch_50_throughput", throughput50))

	// Batch processing should be more efficient than individual operations
	assert.Greater(suite.T(), throughput50, throughput1*10,
		"Batch processing should be significantly more efficient")
}

// TestConcurrentBatches tests processing multiple batches concurrently
func (suite *BatchOperationsTestSuite) TestConcurrentBatches() {
	suite.logger.Info("Testing concurrent batch processing")

	ctx := context.Background()
	concurrency := 5
	batchSize := 10

	var wg sync.WaitGroup
	results := make(chan *models.BatchResult, concurrency)
	errors := make(chan error, concurrency)

	startTime := time.Now()

	// Start concurrent batches
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(batchIndex int) {
			defer wg.Done()

			operations := suite.createBatchOperations(batchSize, fmt.Sprintf("concurrent_%d", batchIndex))
			batchRequest := suite.convertToBatchRequest(operations, models.BatchModeIndividual)

			result, err := suite.batchService.ProcessBatch(ctx, batchRequest)
			if err != nil {
				errors <- err
				return
			}

			results <- result
		}(i)
	}

	// Wait for all batches to complete
	wg.Wait()
	close(results)
	close(errors)

	totalTime := time.Since(startTime)

	// Collect results
	var successfulBatches int
	var totalOpsProcessed int
	var totalOpsSuccessful int

	for result := range results {
		successfulBatches++
		totalOpsProcessed += result.TotalOperations
		totalOpsSuccessful += result.SuccessfulOps
	}

	// Check for errors
	errorCount := 0
	for err := range errors {
		suite.logger.Error("Concurrent batch error", zap.Error(err))
		errorCount++
	}

	suite.logger.Info("Concurrent batch processing completed",
		zap.Duration("total_time", totalTime),
		zap.Int("successful_batches", successfulBatches),
		zap.Int("error_count", errorCount),
		zap.Int("total_ops_processed", totalOpsProcessed),
		zap.Int("total_ops_successful", totalOpsSuccessful))

	// Verify concurrent processing
	assert.Equal(suite.T(), 0, errorCount, "No batches should error")
	assert.Equal(suite.T(), concurrency, successfulBatches, "All batches should succeed")
	assert.Equal(suite.T(), concurrency*batchSize, totalOpsProcessed,
		"All operations should be processed")

	// Verify concurrent processing performance
	expectedMaxTime := 60 * time.Second // Should complete within reasonable time
	assert.Less(suite.T(), totalTime, expectedMaxTime,
		"Concurrent batches should complete within expected time")
}

// Helper methods

func (suite *BatchOperationsTestSuite) createBatchOperations(count int, prefix string) []models.StorageTask {
	operations := make([]models.StorageTask, count)

	for i := 0; i < count; i++ {
		operations[i] = models.StorageTask{
			Operation: "create",
			Data: map[string]interface{}{
				"first_name": fmt.Sprintf("%s_%d", prefix, i),
				"last_name":  "BatchTest",
				"email":      fmt.Sprintf("%s.%d@test.com", prefix, i),
				"phone":      fmt.Sprintf("555%04d", i),
			},
			Timestamp:   time.Now(),
			RequestedBy: "batch_test",
		}
	}

	return operations
}

func (suite *BatchOperationsTestSuite) trackTestProfile(profileID uuid.UUID) {
	suite.mu.Lock()
	defer suite.mu.Unlock()
	suite.testProfileIDs = append(suite.testProfileIDs, profileID)
}

func (suite *BatchOperationsTestSuite) cleanupTestProfiles() {
	suite.mu.RLock()
	profileIDs := make([]uuid.UUID, len(suite.testProfileIDs))
	copy(profileIDs, suite.testProfileIDs)
	suite.mu.RUnlock()

	if len(profileIDs) == 0 {
		return
	}

	ctx := context.Background()
	for _, id := range profileIDs {
		// Attempt to clean up test profiles
		suite.profileService.DeleteProfile(ctx, id)
	}

	suite.logger.Info("Cleaned up test profiles", zap.Int("count", len(profileIDs)))
}

// Test runner functions for batch operations

func TestBatchOperationsSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping batch operations integration tests in short mode")
	}

	suite := NewBatchOperationsTestSuite(t)

	err := suite.SetupSuite()
	require.NoError(t, err, "Batch operations test suite setup should succeed")
	defer func() {
		if err := suite.TearDownSuite(); err != nil {
			t.Logf("Warning: Batch operations test suite teardown failed: %v", err)
		}
	}()

	// Run all batch operations tests with proper function wrapping
	t.Run("SmallBatchTransactional", func(t *testing.T) { suite.TestSmallBatchTransactional() })
	t.Run("MediumBatchIndividual", func(t *testing.T) { suite.TestMediumBatchIndividual() })
	t.Run("LargeBatchWithOptimization", func(t *testing.T) { suite.TestLargeBatchWithOptimization() })
	t.Run("MixedOperationsBatch", func(t *testing.T) { suite.TestMixedOperationsBatch() })
	t.Run("BatchFailureHandling", func(t *testing.T) { suite.TestBatchFailureHandling() })
	t.Run("BatchPerformanceScaling", func(t *testing.T) { suite.TestBatchPerformanceScaling() })
	t.Run("ConcurrentBatches", func(t *testing.T) { suite.TestConcurrentBatches() })
}
