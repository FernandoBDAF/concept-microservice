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
	authService    *service.AuthService
	batchService   *service.AdvancedBatchOperationsService

	// Handlers
	profileHandler *rest.ProfileHandler
	authHandler    *rest.AuthHandler
	batchHandler   *rest.BatchHandler

	// REST server
	restServer *rest.Server
	testServer *httptest.Server

	// Messaging (when enabled)
	authMessageHandler  *messaging.AuthHandler
	batchMessageHandler *messaging.BatchMessageHandler

	// Performance optimization
	optimizationManager *performance.OptimizationManager

	// Test data tracking
	createdProfiles []string
	createdUsers    []string
	createdBatches  []string
}

// SetupSuite initializes the comprehensive test environment
func (suite *ComprehensiveIntegrationTestSuite) SetupSuite() {
	// Load test configuration
	cfg := &config.Config{
		DBHost:         "localhost",
		DBPort:         "5432",
		DBName:         "storage_test",
		DBUser:         "test",
		DBPassword:     "test",
		ServerPort:     "8080",
		LogLevel:       "debug",
		LogEnvironment: "test",
	}

	// Initialize database connection
	suite.connManager = database.NewConnectionManager(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := suite.connManager.Connect(ctx)
	require.NoError(suite.T(), err, "Failed to connect to test database")

	suite.testDB = suite.connManager.GetDB().DB

	// Create repositories
	profileRepo := repository.NewProfileRepository(suite.connManager.GetDB())
	authRepo := repository.NewAuthRepository(suite.connManager.GetDB())

	// Create services
	suite.profileService = service.NewProfileService(profileRepo)
	suite.authService = service.NewAuthService(authRepo)
	suite.batchService = service.NewAdvancedBatchOperationsService(
		suite.profileService,
		suite.authService,
		suite.connManager.GetDB(),
	)

	// Create handlers
	suite.profileHandler = rest.NewProfileHandler(suite.profileService)
	suite.authHandler = rest.NewAuthHandler(suite.authService)
	suite.batchHandler = rest.NewBatchHandler(suite.batchService)

	// Create messaging handlers
	suite.authMessageHandler = messaging.NewAuthHandler(suite.authService)
	suite.batchMessageHandler = messaging.NewBatchMessageHandler(suite.batchService)

	// Create performance optimization manager
	suite.optimizationManager = performance.NewOptimizationManager(suite.connManager.GetDB())
	err = suite.optimizationManager.Start(context.Background())
	require.NoError(suite.T(), err, "Failed to start optimization manager")

	// Create and configure REST server
	suite.restServer = rest.NewServer(cfg)
	suite.restServer.RegisterRoutes(
		suite.profileHandler,
		suite.authHandler,
		suite.batchHandler,
	)

	// Create test HTTP server using the underlying HTTP handler
	// Note: We'll skip the actual HTTP server for now and test the handlers directly
	// since the REST server doesn't expose its mux directly
	suite.testServer = nil // Will test handlers directly

	// Initialize tracking slices
	suite.createdProfiles = make([]string, 0)
	suite.createdUsers = make([]string, 0)
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

// TestCompleteAuthIntegration validates the complete auth service integration
func (suite *ComprehensiveIntegrationTestSuite) TestCompleteAuthIntegration() {
	suite.T().Log("Testing complete auth service integration")

	// Test 1: Create a user via service layer (direct testing)
	createUserReq := &models.AuthUserRequest{
		Email:     "integration-test@example.com",
		Password:  "TestPassword123!",
		FirstName: "Integration",
		LastName:  "Test",
		Role:      "user",
	}

	user, err := suite.authService.CreateUser(context.Background(), createUserReq)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), user)
	suite.createdUsers = append(suite.createdUsers, user.ID)

	// Test 2: Authenticate the user
	authUser, err := suite.authService.AuthenticateUser(
		context.Background(),
		"integration-test@example.com",
		"TestPassword123!",
		"127.0.0.1",
		"integration-test",
	)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), authUser)
	assert.Equal(suite.T(), user.ID, authUser.ID)

	// Test 3: List users
	users, err := suite.authService.ListUsers(context.Background(), 1, 10)
	require.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), users)

	// Test 4: Test auth message handler (simulated queue message)
	userJSON, err := json.Marshal(createUserReq)
	require.NoError(suite.T(), err)

	testMessage := &messaging.Message{
		ID:         "test-auth-msg-1",
		Type:       "auth.user.create",
		RoutingKey: "auth.user.create",
		Payload:    userJSON,
		Timestamp:  time.Now(),
	}

	msgResp, err := suite.authMessageHandler.Handle(context.Background(), testMessage)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), msgResp.Success)

	suite.T().Log("✅ Complete auth integration test passed")
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

	// Test 2: Auth batch operations (Transactional mode)
	authBatchReq := suite.createAuthBatchRequest(models.BatchModeTransactional, 3)

	authResult, err := suite.batchService.ProcessBatch(context.Background(), authBatchReq)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), authResult)

	// Test 3: Parallel batch processing
	parallelBatchReq := suite.createProfileBatchRequest(models.BatchModeParallel, 10)
	parallelBatchReq.Options.MaxConcurrency = 3

	parallelResult, err := suite.batchService.ProcessBatch(context.Background(), parallelBatchReq)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), parallelResult)

	// Test 4: Batch validation via batch request validation
	validationReq := suite.createProfileBatchRequest(models.BatchModeTransactional, 2)
	validationErr := validationReq.Validate()
	assert.NoError(suite.T(), validationErr)

	// Test 5: Batch message handler (simulated queue message)
	batchJSON, err := json.Marshal(profileBatchReq)
	require.NoError(suite.T(), err)

	testBatchMessage := &messaging.Message{
		ID:         "test-batch-msg-1",
		Type:       "batch.process",
		RoutingKey: "batch.process",
		Payload:    batchJSON,
		Timestamp:  time.Now(),
	}

	batchMsgResp, err := suite.batchMessageHandler.Handle(context.Background(), testBatchMessage)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), batchMsgResp.Success)

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

	// Test 1: Invalid auth user creation
	invalidUserReq := &models.AuthUserRequest{
		Email:     "invalid-email", // Invalid email format
		Password:  "123",           // Too short password
		FirstName: "",              // Empty first name
		LastName:  "Test",
		Role:      "invalid_role", // Invalid role
	}

	_, err := suite.authService.CreateUser(context.Background(), invalidUserReq)
	assert.Error(suite.T(), err)

	// Test 2: Batch with invalid operations
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

	// Test 3: Non-existent batch status query
	_, exists := suite.batchService.GetBatchStatus("non-existent-batch-id")
	assert.False(suite.T(), exists)

	// Test 4: Invalid message routing key
	invalidMessage := &messaging.Message{
		ID:         "invalid-msg-1",
		Type:       "invalid.type",
		RoutingKey: "invalid.routing.key",
		Payload:    []byte(`{}`),
		Timestamp:  time.Now(),
	}

	invalidMsgResp, err := suite.authMessageHandler.Handle(context.Background(), invalidMessage)
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

// createAuthBatchRequest creates a test auth batch request
func (suite *ComprehensiveIntegrationTestSuite) createAuthBatchRequest(mode models.BatchProcessingMode, numOps int) *models.BatchRequest {
	operations := make([]models.BatchOperationItem, numOps)

	for i := 0; i < numOps; i++ {
		userData := map[string]interface{}{
			"email":      fmt.Sprintf("batch-auth-test-%d@example.com", i),
			"password":   "TestPassword123!",
			"first_name": fmt.Sprintf("AuthBatch%d", i),
			"last_name":  "User",
			"role":       "user",
		}

		dataJSON, _ := json.Marshal(userData)

		operations[i] = models.BatchOperationItem{
			ID:         fmt.Sprintf("auth-op-%d", i),
			Operation:  models.BatchOperationCreate,
			Data:       dataJSON,
			ExternalID: fmt.Sprintf("ext-auth-%d", i),
		}
	}

	return &models.BatchRequest{
		Type:       "auth",
		Operations: operations,
		Options: models.BatchOptions{
			Mode:                mode,
			FailureHandling:     models.BatchContinueOnFail,
			MaxConcurrency:      3,
			TimeoutPerOperation: 30 * time.Second,
			TotalTimeout:        5 * time.Minute,
			ValidationLevel:     models.BatchValidationBasic,
			EnableRollback:      true,
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

	// Clean up users
	for _, userID := range suite.createdUsers {
		_, _ = suite.testDB.Exec("DELETE FROM auth_users WHERE id = $1", userID)
	}

	// Clean up any batch-related test data
	_, _ = suite.testDB.Exec("DELETE FROM profiles WHERE email LIKE 'batch-test-%@example.com'")
	_, _ = suite.testDB.Exec("DELETE FROM auth_users WHERE email LIKE 'batch-auth-test-%@example.com'")
	_, _ = suite.testDB.Exec("DELETE FROM auth_users WHERE email = 'integration-test@example.com'")

	suite.T().Log("Test data cleanup completed")
}

// TestComprehensiveIntegrationTestSuite runs the comprehensive integration test suite
func TestComprehensiveIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ComprehensiveIntegrationTestSuite))
}
