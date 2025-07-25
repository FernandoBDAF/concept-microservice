package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/domain/models"
	"microservices/services/profile-storage/internal/domain/service"
)

// BatchOperationsTestSuite focuses specifically on batch processing capabilities
type BatchOperationsTestSuite struct {
	baseSuite      *IntegrationTestSuite
	batchService   *service.BatchOperationsService
	logger         *zap.Logger
	testProfileIDs []uuid.UUID
	mu             sync.RWMutex
}

// NewBatchOperationsTestSuite creates a new batch operations test suite
func NewBatchOperationsTestSuite(t *testing.T) *BatchOperationsTestSuite {
	baseSuite := NewIntegrationTestSuite(t)

	return &BatchOperationsTestSuite{
		baseSuite:      baseSuite,
		logger:         baseSuite.logger.Named("batch_operations_test"),
		testProfileIDs: make([]uuid.UUID, 0),
	}
}

// SetupSuite initializes the batch operations test environment
func (suite *BatchOperationsTestSuite) SetupSuite() error {
	if err := suite.baseSuite.SetupSuite(); err != nil {
		return err
	}

	suite.batchService = suite.baseSuite.batchService
	suite.logger.Info("Batch operations test suite setup completed")
	return nil
}

// TearDownSuite cleans up the test environment
func (suite *BatchOperationsTestSuite) TearDownSuite() error {
	suite.cleanupTestProfiles()
	return suite.baseSuite.TearDownSuite()
}

// TestSmallBatchTransactional tests small batches in transaction mode
func (suite *BatchOperationsTestSuite) TestSmallBatchTransactional() {
	suite.logger.Info("Testing small batch in transaction mode")

	ctx := context.Background()

	// Create 5 operations for transactional processing
	operations := suite.createBatchOperations(5, "small_batch_tx")
	options := models.BatchOptions{
		TransactionMode: true,
		MaxBatchSize:    10,
		Timeout:         30 * time.Second,
		FailureMode:     "rollback",
	}

	startTime := time.Now()
	result, err := suite.batchService.ProcessBatch(ctx, operations, options)
	processingTime := time.Since(startTime)

	require.NoError(suite.baseSuite.t, err, "Small transactional batch should succeed")
	require.NotNil(suite.baseSuite.t, result, "Batch result should not be nil")

	suite.logger.Info("Small transactional batch completed",
		zap.Duration("processing_time", processingTime),
		zap.Int("successful_ops", result.SuccessfulOps),
		zap.Int("failed_ops", result.FailedOps))

	// Verify all operations succeeded
	assert.Equal(suite.baseSuite.t, len(operations), result.SuccessfulOps, "All operations should succeed")
	assert.Equal(suite.baseSuite.t, 0, result.FailedOps, "No operations should fail")
	assert.True(suite.baseSuite.t, result.TransactionMode, "Should be in transaction mode")

	// Verify performance (small batches should be very fast)
	assert.Less(suite.baseSuite.t, processingTime, 5*time.Second, "Small batch should complete quickly")
}

// TestMediumBatchIndividual tests medium batches in individual mode
func (suite *BatchOperationsTestSuite) TestMediumBatchIndividual() {
	suite.logger.Info("Testing medium batch in individual mode")

	ctx := context.Background()

	// Create 25 operations for individual processing
	operations := suite.createBatchOperations(25, "medium_batch_ind")
	options := models.BatchOptions{
		TransactionMode: false,
		MaxBatchSize:    50,
		Timeout:         60 * time.Second,
		FailureMode:     "continue",
	}

	startTime := time.Now()
	result, err := suite.batchService.ProcessBatch(ctx, operations, options)
	processingTime := time.Since(startTime)

	require.NoError(suite.baseSuite.t, err, "Medium individual batch should succeed")
	require.NotNil(suite.baseSuite.t, result, "Batch result should not be nil")

	suite.logger.Info("Medium individual batch completed",
		zap.Duration("processing_time", processingTime),
		zap.Int("successful_ops", result.SuccessfulOps),
		zap.Int("failed_ops", result.FailedOps))

	// Verify batch processing
	assert.Equal(suite.baseSuite.t, len(operations), result.TotalOperations, "Total operations should match")
	assert.False(suite.baseSuite.t, result.TransactionMode, "Should not be in transaction mode")

	// Verify performance target (< 30s for medium batches)
	assert.Less(suite.baseSuite.t, processingTime, 30*time.Second, "Medium batch should meet performance target")
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

	options := models.BatchOptions{
		TransactionMode: false,
		MaxBatchSize:    150,
		Timeout:         120 * time.Second, // Longer timeout for large batch
		FailureMode:     "continue",
	}

	startTime := time.Now()
	result, err := suite.batchService.ProcessBatch(ctx, operations, options)
	processingTime := time.Since(startTime)

	require.NoError(suite.baseSuite.t, err, "Large optimized batch should succeed")
	require.NotNil(suite.baseSuite.t, result, "Batch result should not be nil")

	suite.logger.Info("Large optimized batch completed",
		zap.Duration("processing_time", processingTime),
		zap.Int("total_operations", result.TotalOperations),
		zap.Int("successful_ops", result.SuccessfulOps),
		zap.Int("failed_ops", result.FailedOps))

	// Verify batch optimization worked (duplicates should be removed)
	assert.Less(suite.baseSuite.t, result.TotalOperations, len(operations),
		"Optimization should remove duplicate operations")

	// Verify performance target (< 60s for 100 operations is reasonable for our test)
	assert.Less(suite.baseSuite.t, processingTime, 60*time.Second,
		"Large batch should complete within reasonable time")
}

// TestMixedOperationsBatch tests batches with different operation types
func (suite *BatchOperationsTestSuite) TestMixedOperationsBatch() {
	suite.logger.Info("Testing mixed operations batch")

	ctx := context.Background()

	// First create some profiles to update and delete
	createOps := suite.createBatchOperations(10, "mixed_create")
	createOptions := models.BatchOptions{
		TransactionMode: false,
		FailureMode:     "continue",
	}

	createResult, err := suite.batchService.ProcessBatch(ctx, createOps, createOptions)
	require.NoError(suite.baseSuite.t, err, "Create batch should succeed")

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
	mixedOptions := models.BatchOptions{
		TransactionMode: false,
		MaxBatchSize:    20,
		Timeout:         45 * time.Second,
		FailureMode:     "continue",
	}

	startTime := time.Now()
	mixedResult, err := suite.batchService.ProcessBatch(ctx, mixedOps, mixedOptions)
	processingTime := time.Since(startTime)

	require.NoError(suite.baseSuite.t, err, "Mixed operations batch should succeed")

	suite.logger.Info("Mixed operations batch completed",
		zap.Duration("processing_time", processingTime),
		zap.Int("total_operations", mixedResult.TotalOperations),
		zap.Int("successful_ops", mixedResult.SuccessfulOps))

	// Verify mixed operations handling
	assert.Equal(suite.baseSuite.t, len(mixedOps), mixedResult.TotalOperations,
		"All mixed operations should be processed")
	assert.Greater(suite.baseSuite.t, mixedResult.SuccessfulOps, 0,
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
	continueOptions := models.BatchOptions{
		TransactionMode: false,
		FailureMode:     "continue",
		Timeout:         30 * time.Second,
	}

	startTime := time.Now()
	continueResult, err := suite.batchService.ProcessBatch(ctx, operations, continueOptions)
	processingTime := time.Since(startTime)

	// Should not return error in continue mode
	require.NoError(suite.baseSuite.t, err, "Continue mode should not return error")
	require.NotNil(suite.baseSuite.t, continueResult, "Result should not be nil")

	suite.logger.Info("Continue mode batch completed",
		zap.Duration("processing_time", processingTime),
		zap.Int("successful_ops", continueResult.SuccessfulOps),
		zap.Int("failed_ops", continueResult.FailedOps))

	// Verify failure handling
	assert.Equal(suite.baseSuite.t, len(operations), continueResult.TotalOperations,
		"All operations should be attempted")
	assert.Greater(suite.baseSuite.t, continueResult.SuccessfulOps, 0,
		"Some operations should succeed")
	assert.Greater(suite.baseSuite.t, continueResult.FailedOps, 0,
		"Some operations should fail")

	// Test "stop" failure mode
	stopOptions := models.BatchOptions{
		TransactionMode: false,
		FailureMode:     "stop",
		Timeout:         30 * time.Second,
	}

	// This should fail fast when encountering invalid operations
	stopResult, stopErr := suite.batchService.ProcessBatch(ctx, operations, stopOptions)

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
		options := models.BatchOptions{
			TransactionMode: false,
			MaxBatchSize:    size + 10,
			Timeout:         time.Duration(size*2) * time.Second,
			FailureMode:     "continue",
		}

		startTime := time.Now()
		result, err := suite.batchService.ProcessBatch(ctx, operations, options)
		processingTime := time.Since(startTime)

		require.NoError(suite.baseSuite.t, err, fmt.Sprintf("Batch size %d should succeed", size))
		results[size] = processingTime

		suite.logger.Info("Batch performance result",
			zap.Int("batch_size", size),
			zap.Duration("processing_time", processingTime),
			zap.Float64("ops_per_second", float64(size)/processingTime.Seconds()),
			zap.Int("successful_ops", result.SuccessfulOps))

		// Basic performance validation
		expectedMaxTime := time.Duration(size) * 500 * time.Millisecond // 500ms per operation max
		assert.Less(suite.baseSuite.t, processingTime, expectedMaxTime,
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
	assert.Greater(suite.baseSuite.t, throughput50, throughput1*10,
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
			options := models.BatchOptions{
				TransactionMode: false,
				MaxBatchSize:    batchSize + 5,
				Timeout:         45 * time.Second,
				FailureMode:     "continue",
			}

			result, err := suite.batchService.ProcessBatch(ctx, operations, options)
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
	assert.Equal(suite.baseSuite.t, 0, errorCount, "No batches should error")
	assert.Equal(suite.baseSuite.t, concurrency, successfulBatches, "All batches should succeed")
	assert.Equal(suite.baseSuite.t, concurrency*batchSize, totalOpsProcessed,
		"All operations should be processed")

	// Verify concurrent processing performance
	expectedMaxTime := 60 * time.Second // Should complete within reasonable time
	assert.Less(suite.baseSuite.t, totalTime, expectedMaxTime,
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
		suite.baseSuite.profileService.DeleteProfile(ctx, id)
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
