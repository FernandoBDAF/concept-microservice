package benchmarks

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"microservices/services/profile-storage/internal/config"
	"microservices/services/profile-storage/internal/domain/models"
	"microservices/services/profile-storage/internal/domain/service"
	"microservices/services/profile-storage/internal/infrastructure/database"
	"microservices/services/profile-storage/internal/infrastructure/repository"
	"microservices/services/profile-storage/internal/messaging"
	"microservices/services/profile-storage/internal/performance"
)

// ComprehensiveBenchmarkSuite holds the comprehensive benchmarking environment
type ComprehensiveBenchmarkSuite struct {
	connManager         *database.ConnectionManager
	profileService      *service.ProfileService
	batchService        *service.AdvancedBatchOperationsService
	batchMessageHandler *messaging.BatchMessageHandler
	optimizationManager *performance.OptimizationManager
}

// setupComprehensiveBenchmarkSuite initializes the comprehensive benchmark environment
func setupComprehensiveBenchmarkSuite(b *testing.B) *ComprehensiveBenchmarkSuite {
	cfg := &config.Config{
		DBHost:         "localhost",
		DBPort:         "5432",
		DBName:         "storage_benchmark",
		DBUser:         "test",
		DBPassword:     "test",
		LogLevel:       "error", // Reduce logging for benchmarks
		LogEnvironment: "benchmark",
	}

	// Initialize database connection
	connManager := database.NewConnectionManager(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := connManager.Connect(ctx)
	require.NoError(b, err, "Failed to connect to benchmark database")

	// Create repositories
	profileRepo := repository.NewProfileRepository(connManager.GetDB())

	// Create services
	profileService := service.NewProfileService(profileRepo)
	batchService := service.NewAdvancedBatchOperationsService(
		profileService,
		connManager.GetDB(),
	)

	// Create message handlers
	batchMessageHandler := messaging.NewBatchMessageHandler(batchService)

	// Create optimization manager
	optimizationManager := performance.NewOptimizationManager(connManager.GetDB())
	err = optimizationManager.Start(context.Background())
	require.NoError(b, err, "Failed to start optimization manager")

	return &ComprehensiveBenchmarkSuite{
		connManager:         connManager,
		profileService:      profileService,
		batchService:        batchService,
		batchMessageHandler: batchMessageHandler,
		optimizationManager: optimizationManager,
	}
}

// BenchmarkBatchOperationsIndividual benchmarks individual batch processing
func BenchmarkBatchOperationsIndividual(b *testing.B) {
	suite := setupComprehensiveBenchmarkSuite(b)
	defer suite.connManager.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		batchReq := createProfileBatchRequest(models.BatchModeIndividual, 10, i)

		startTime := time.Now()
		result, err := suite.batchService.ProcessBatch(context.Background(), batchReq)
		duration := time.Since(startTime)

		require.NoError(b, err)
		require.NotNil(b, result)
		require.True(b, result.IsCompleted())

		// Record performance sample
		suite.optimizationManager.RecordPerformanceSample("batch_individual_10ops", duration, result.IsSuccessful())

		// Target: < 5s for 10 operations
		if duration > 5*time.Second {
			b.Logf("Warning: Individual batch (10 ops) took %v (target: <5s)", duration)
		}
	}
}

// BenchmarkBatchOperationsTransactional benchmarks transactional batch processing
func BenchmarkBatchOperationsTransactional(b *testing.B) {
	suite := setupComprehensiveBenchmarkSuite(b)
	defer suite.connManager.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		batchReq := createProfileBatchRequest(models.BatchModeTransactional, 20, i)

		startTime := time.Now()
		result, err := suite.batchService.ProcessBatch(context.Background(), batchReq)
		duration := time.Since(startTime)

		require.NoError(b, err)
		require.NotNil(b, result)

		// Record performance sample
		suite.optimizationManager.RecordPerformanceSample("batch_transactional_20ops", duration, result.IsSuccessful())

		// Target: < 10s for 20 operations in transaction
		if duration > 10*time.Second {
			b.Logf("Warning: Transactional batch (20 ops) took %v (target: <10s)", duration)
		}
	}
}

// BenchmarkBatchOperationsParallel benchmarks parallel batch processing
func BenchmarkBatchOperationsParallel(b *testing.B) {
	suite := setupComprehensiveBenchmarkSuite(b)
	defer suite.connManager.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		batchReq := createProfileBatchRequest(models.BatchModeParallel, 50, i)
		batchReq.Options.MaxConcurrency = 5

		startTime := time.Now()
		result, err := suite.batchService.ProcessBatch(context.Background(), batchReq)
		duration := time.Since(startTime)

		require.NoError(b, err)
		require.NotNil(b, result)

		// Record performance sample
		suite.optimizationManager.RecordPerformanceSample("batch_parallel_50ops", duration, result.IsSuccessful())

		// Target: < 15s for 50 operations with 5 concurrency
		if duration > 15*time.Second {
			b.Logf("Warning: Parallel batch (50 ops, 5 concurrent) took %v (target: <15s)", duration)
		}
	}
}

// BenchmarkLargeBatchOperations benchmarks large batch processing (performance target validation)
func BenchmarkLargeBatchOperations(b *testing.B) {
	suite := setupComprehensiveBenchmarkSuite(b)
	defer suite.connManager.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		batchReq := createProfileBatchRequest(models.BatchModeParallel, 100, i)
		batchReq.Options.MaxConcurrency = 10

		startTime := time.Now()
		result, err := suite.batchService.ProcessBatch(context.Background(), batchReq)
		duration := time.Since(startTime)

		require.NoError(b, err)
		require.NotNil(b, result)

		// Record performance sample
		suite.optimizationManager.RecordPerformanceSample("batch_large_100ops", duration, result.IsSuccessful())

		// Target: < 30s for 100 operations (as specified in implementation prompt)
		if duration > 30*time.Second {
			b.Logf("Warning: Large batch (100 ops) took %v (target: <30s)", duration)
		}

		// Validate success rate
		successRate := result.GetSuccessRate()
		if successRate < 95.0 {
			b.Logf("Warning: Success rate %v%% (target: >95%%)", successRate)
		}
	}
}

// BenchmarkMessageHandlerBatch benchmarks batch message handler performance
func BenchmarkMessageHandlerBatch(b *testing.B) {
	suite := setupComprehensiveBenchmarkSuite(b)
	defer suite.connManager.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		batchReq := createProfileBatchRequest(models.BatchModeIndividual, 15, i)
		batchJSON, err := json.Marshal(batchReq)
		require.NoError(b, err)

		message := &messaging.Message{
			ID:         fmt.Sprintf("bench-batch-msg-%d", i),
			Type:       "batch.process",
			RoutingKey: "batch.process",
			Payload:    batchJSON,
			Timestamp:  time.Now(),
		}

		startTime := time.Now()
		msgResp, err := suite.batchMessageHandler.Handle(context.Background(), message)
		duration := time.Since(startTime)

		require.NoError(b, err)
		require.True(b, msgResp.Success)

		// Record performance sample
		suite.optimizationManager.RecordPerformanceSample("message_batch_15ops", duration, msgResp.Success)

		// Target: < 8s for 15 operations via message
		if duration > 8*time.Second {
			b.Logf("Warning: Batch message processing (15 ops) took %v (target: <8s)", duration)
		}
	}
}

// BenchmarkPerformanceOptimization benchmarks the performance optimization system itself
func BenchmarkPerformanceOptimization(b *testing.B) {
	suite := setupComprehensiveBenchmarkSuite(b)
	defer suite.connManager.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		startTime := time.Now()

		// Test optimization report generation
		report := suite.optimizationManager.GetOptimizationReport()
		require.NotNil(b, report)

		// Test performance sample recording
		suite.optimizationManager.RecordPerformanceSample("benchmark_test", 100*time.Millisecond, true)

		duration := time.Since(startTime)

		// Target: < 10ms for optimization operations
		if duration > 10*time.Millisecond {
			b.Logf("Warning: Performance optimization took %v (target: <10ms)", duration)
		}
	}
}

// BenchmarkConcurrentBatchOperations benchmarks concurrent batch processing
func BenchmarkConcurrentBatchOperations(b *testing.B) {
	suite := setupComprehensiveBenchmarkSuite(b)
	defer suite.connManager.Close()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			batchReq := createProfileBatchRequest(models.BatchModeParallel, 20, i)
			batchReq.Options.MaxConcurrency = 3

			startTime := time.Now()
			result, err := suite.batchService.ProcessBatch(context.Background(), batchReq)
			duration := time.Since(startTime)

			require.NoError(b, err)
			require.NotNil(b, result)

			// Record performance sample
			suite.optimizationManager.RecordPerformanceSample("concurrent_batch_20ops", duration, result.IsSuccessful())

			i++
		}
	})
}

// BenchmarkMemoryUsageUnderLoad benchmarks memory usage under load
func BenchmarkMemoryUsageUnderLoad(b *testing.B) {
	suite := setupComprehensiveBenchmarkSuite(b)
	defer suite.connManager.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create multiple batch operations to simulate load
		results := make([]*models.BatchResult, 0, 5)

		for j := 0; j < 5; j++ {
			batchReq := createProfileBatchRequest(models.BatchModeParallel, 30, i*5+j)
			result, err := suite.batchService.ProcessBatch(context.Background(), batchReq)
			require.NoError(b, err)
			results = append(results, result)
		}

		// Get resource metrics
		report := suite.optimizationManager.GetOptimizationReport()
		memoryUsage := report.ResourceUsage.MemoryUsageMB

		// Target: < 512MB memory usage under load
		if memoryUsage > 512 {
			b.Logf("Warning: Memory usage %dMB (target: <512MB)", memoryUsage)
		}

		// Verify all batches completed successfully
		for _, result := range results {
			require.True(b, result.IsCompleted())
		}
	}
}

// Helper functions

// createProfileBatchRequest creates a benchmark profile batch request
func createProfileBatchRequest(mode models.BatchProcessingMode, numOps, seed int) *models.BatchRequest {
	operations := make([]models.BatchOperationItem, numOps)

	for i := 0; i < numOps; i++ {
		profileData := map[string]interface{}{
			"first_name": fmt.Sprintf("BenchTest%d", seed*1000+i),
			"last_name":  "User",
			"email":      fmt.Sprintf("bench-test-%d-%d@example.com", seed, i),
			"phone":      "+1234567890",
		}

		dataJSON, _ := json.Marshal(profileData)

		operations[i] = models.BatchOperationItem{
			ID:         fmt.Sprintf("bench-profile-op-%d-%d", seed, i),
			Operation:  models.BatchOperationCreate,
			Data:       dataJSON,
			ExternalID: fmt.Sprintf("ext-bench-profile-%d-%d", seed, i),
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
			EnableProgressTrack: false, // Disable for benchmarks
		},
	}
}
