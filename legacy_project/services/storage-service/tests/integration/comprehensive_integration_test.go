package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"microservices/services/profile-storage/internal/api/rest"
	"microservices/services/profile-storage/internal/config"
	"microservices/services/profile-storage/internal/domain/models"
	"microservices/services/profile-storage/internal/domain/service"
	"microservices/services/profile-storage/internal/infrastructure/database"
	"microservices/services/profile-storage/internal/infrastructure/repository"
	"microservices/services/profile-storage/internal/messaging"
	"microservices/services/profile-storage/internal/performance"
)

// ComprehensiveIntegrationTestSuite tests the complete storage service integration
type ComprehensiveIntegrationTestSuite struct {
	suite.Suite

	// Database and connection
	connManager *database.ConnectionManager
	testDB      *sql.DB

	// Services
	profileService *service.ProfileService
	batchService   *service.AdvancedBatchOperationsService

	// Handlers
	profileHandler *rest.ProfileHandler
	batchHandler   *rest.BatchHandler

	// REST server
	restServer *rest.Server
	testServer *httptest.Server

	// Messaging (when enabled)
	batchMessageHandler *messaging.BatchMessageHandler

	// Performance optimization
	optimizationManager *performance.OptimizationManager

	// Test data tracking
	createdProfiles []string
	createdBatches  []string
}

func (suite *ComprehensiveIntegrationTestSuite) SetupSuite() {
	// Initialize configuration for tests
	cfg := &config.Config{
		DBHost:     "localhost",
		DBPort:     "5432",
		DBName:     "storage_integration_test",
		DBUser:     "test",
		DBPassword: "test",
		ServerPort: "8080",
	}

	// Initialize database connection
	suite.connManager = database.NewConnectionManager(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := suite.connManager.Connect(ctx)
	suite.Require().NoError(err, "Failed to connect to test database")

	suite.testDB = suite.connManager.GetDB().DB

	// Create repositories
	profileRepo := repository.NewProfileRepository(suite.connManager.GetDB())

	// Create services
	suite.profileService = service.NewProfileService(profileRepo)
	suite.batchService = service.NewAdvancedBatchOperationsService(
		suite.profileService,
		suite.connManager.GetDB(),
	)

	// Create handlers
	suite.profileHandler = rest.NewProfileHandler(suite.profileService)
	suite.batchHandler = rest.NewBatchHandler(suite.batchService)

	// Create messaging handlers
	suite.batchMessageHandler = messaging.NewBatchMessageHandler(suite.batchService)

	// Create performance optimization manager
	suite.optimizationManager = performance.NewOptimizationManager(suite.connManager.GetDB())
	err = suite.optimizationManager.Start(context.Background())
	suite.Require().NoError(err, "Failed to start optimization manager")

	// Initialize test data tracking
	suite.createdProfiles = make([]string, 0)
	suite.createdBatches = make([]string, 0)
}

// TearDownSuite cleans up the test environment
func (suite *ComprehensiveIntegrationTestSuite) TearDownSuite() {
	// Clean up created data
	suite.cleanupTestData()

	// Close test server
	if suite.testServer != nil {
		suite.testServer.Close()
	}

	// Close database connection
	if suite.connManager != nil {
		suite.connManager.Close()
	}
}

// TestComprehensiveBatchOperations validates batch operations across all modes
func (suite *ComprehensiveIntegrationTestSuite) TestComprehensiveBatchOperations() {
	suite.T().Log("Testing comprehensive batch operations")

	// Test 1: Profile batch operations (Individual mode)
	profileBatchReq := suite.createProfileBatchRequest(models.BatchModeIndividual, 5)

	result, err := suite.batchService.ProcessBatch(context.Background(), profileBatchReq)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)
	assert.Equal(suite.T(), models.BatchStatusCompleted, result.Status)
	suite.createdBatches = append(suite.createdBatches, result.ID)

	// Test 2: Parallel batch processing
	parallelBatchReq := suite.createProfileBatchRequest(models.BatchModeParallel, 10)
	parallelBatchReq.Options.MaxConcurrency = 3

	parallelResult, err := suite.batchService.ProcessBatch(context.Background(), parallelBatchReq)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), parallelResult)
	assert.Equal(suite.T(), models.BatchStatusCompleted, parallelResult.Status)

	// Test 3: Message-based batch processing
	batchMessage := &messaging.Message{
		ID:         "test-batch-msg-001",
		Type:       "batch.profile.process",
		RoutingKey: "batch.profile.process",
		Payload:    mustMarshal(profileBatchReq),
		Timestamp:  time.Now(),
	}

	msgResponse, err := suite.batchMessageHandler.Handle(context.Background(), batchMessage)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), msgResponse.Success)

	suite.T().Log("✅ Comprehensive batch operations test passed")
}

// TestPerformanceOptimization validates performance optimization features
func (suite *ComprehensiveIntegrationTestSuite) TestPerformanceOptimization() {
	suite.T().Log("Testing performance optimization")

	// Test 1: Get optimization report
	report := suite.optimizationManager.GetOptimizationReport()
	require.NotNil(suite.T(), report)
	assert.NotZero(suite.T(), report.Timestamp)
	assert.NotEmpty(suite.T(), report.Recommendations)

	// Test 2: Record performance samples
	suite.optimizationManager.RecordPerformanceSample("test_operation", 100*time.Millisecond, true)
	suite.optimizationManager.RecordPerformanceSample("test_operation", 150*time.Millisecond, true)
	suite.optimizationManager.RecordPerformanceSample("test_operation", 120*time.Millisecond, false)

	// Test 3: Get updated report with samples
	updatedReport := suite.optimizationManager.GetOptimizationReport()
	require.NotNil(suite.T(), updatedReport)

	// Test 4: Validate connection pool metrics
	assert.NotNil(suite.T(), updatedReport.ConnectionPool)
	assert.GreaterOrEqual(suite.T(), updatedReport.ConnectionPool.OpenConnections, 0)

	// Test 5: Validate resource usage monitoring
	assert.NotNil(suite.T(), updatedReport.ResourceUsage)
	assert.Greater(suite.T(), updatedReport.ResourceUsage.MemoryUsageMB, int64(0))
	assert.Greater(suite.T(), updatedReport.ResourceUsage.GoroutineCount, 0)

	suite.T().Log("✅ Performance optimization test passed")
}

// TestErrorScenariosAndRecovery validates error handling and recovery mechanisms
func (suite *ComprehensiveIntegrationTestSuite) TestErrorScenariosAndRecovery() {
	suite.T().Log("Testing error scenarios and recovery")

	// Test 1: Batch with invalid operations
	invalidBatch := &models.BatchRequest{
		Type: "profile",
		Operations: []models.BatchOperationItem{
			{
				ID:        "invalid-op-1",
				Operation: "invalid_operation", // Invalid operation type
				Data:      []byte(`{"invalid": "data"}`),
			},
		},
		Options: models.DefaultBatchOptions(),
	}

	validationErr := invalidBatch.Validate()
	assert.Error(suite.T(), validationErr)

	// Test 2: Non-existent batch status query
	_, exists := suite.batchService.GetBatchStatus("non-existent-batch-id")
	assert.False(suite.T(), exists)

	// Test 3: Invalid message routing key
	invalidMessage := &messaging.Message{
		ID:         "invalid-msg-1",
		Type:       "invalid.type",
		RoutingKey: "invalid.routing.key",
		Payload:    []byte(`{}`),
		Timestamp:  time.Now(),
	}

	invalidMsgResp, err := suite.batchMessageHandler.Handle(context.Background(), invalidMessage)
	require.NoError(suite.T(), err)
	assert.False(suite.T(), invalidMsgResp.Success)
	assert.NotEmpty(suite.T(), invalidMsgResp.Error)

	suite.T().Log("✅ Error scenarios and recovery test passed")
}

// TestConcurrencyAndLoadHandling validates concurrent operations
func (suite *ComprehensiveIntegrationTestSuite) TestConcurrencyAndLoadHandling() {
	suite.T().Log("Testing concurrency and load handling")

	const numConcurrentRequests = 10
	const numOperationsPerBatch = 5

	// Test concurrent batch operations
	doneCh := make(chan *models.BatchResult, numConcurrentRequests)
	errorsCh := make(chan error, numConcurrentRequests)

	for i := 0; i < numConcurrentRequests; i++ {
		go func(requestID int) {
			// Create a batch request
			batchReq := suite.createProfileBatchRequest(models.BatchModeParallel, numOperationsPerBatch)
			batchReq.Operations[0].ExternalID = fmt.Sprintf("concurrent-test-%d", requestID)

			result, err := suite.batchService.ProcessBatch(context.Background(), batchReq)
			if err != nil {
				errorsCh <- err
				return
			}

			doneCh <- result
		}(i)
	}

	// Wait for all requests to complete
	completed := 0
	timeout := time.After(30 * time.Second)

	for completed < numConcurrentRequests {
		select {
		case result := <-doneCh:
			completed++
			assert.True(suite.T(), result.IsCompleted())
		case err := <-errorsCh:
			suite.T().Errorf("Concurrent request failed: %v", err)
		case <-timeout:
			suite.T().Fatal("Timeout waiting for concurrent requests")
		}
	}

	// Verify performance metrics were collected
	report := suite.optimizationManager.GetOptimizationReport()
	assert.NotEmpty(suite.T(), report.PerformanceTrend)

	suite.T().Log("✅ Concurrency and load handling test passed")
}

// Helper methods

// createProfileBatchRequest creates a test profile batch request
func (suite *ComprehensiveIntegrationTestSuite) createProfileBatchRequest(mode models.BatchProcessingMode, numOps int) *models.BatchRequest {
	operations := make([]models.BatchOperationItem, numOps)

	for i := 0; i < numOps; i++ {
		profileData := map[string]interface{}{
			"first_name": fmt.Sprintf("BatchTest%d", i),
			"last_name":  "User",
			"email":      fmt.Sprintf("batch-test-%d@example.com", i),
			"phone":      "+1234567890",
		}

		dataJSON, _ := json.Marshal(profileData)

		operations[i] = models.BatchOperationItem{
			ID:         fmt.Sprintf("profile-op-%d", i),
			Operation:  models.BatchOperationCreate,
			Data:       dataJSON,
			ExternalID: fmt.Sprintf("ext-profile-%d", i),
		}
	}

	return &models.BatchRequest{
		Type:       "profile",
		Operations: operations,
		Options: models.BatchOptions{
			Mode:                mode,
			FailureHandling:     models.BatchContinueOnFail,
			MaxConcurrency:      5,
			TimeoutPerOperation: 30 * time.Second,
			TotalTimeout:        5 * time.Minute,
			ValidationLevel:     models.BatchValidationBasic,
			EnableRollback:      false,
			EnableProgressTrack: true,
		},
	}
}

// cleanupTestData removes all test data created during tests
func (suite *ComprehensiveIntegrationTestSuite) cleanupTestData() {
	suite.T().Log("Cleaning up test data")

	// Clean up profiles
	for _, profileID := range suite.createdProfiles {
		_, _ = suite.testDB.Exec("DELETE FROM profiles WHERE id = $1", profileID)
	}

	// Clean up any batch-related test data
	_, _ = suite.testDB.Exec("DELETE FROM profiles WHERE email LIKE 'batch-test-%@example.com'")

	suite.T().Log("Test data cleanup completed")
}

// mustMarshal is a helper function for marshaling test data
func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal test data: %v", err))
	}
	return data
}

// TestComprehensiveIntegrationTestSuite runs the comprehensive integration test suite
func TestComprehensiveIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ComprehensiveIntegrationTestSuite))
}
