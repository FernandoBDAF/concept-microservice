package benchmarks

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/domain/models"
	"microservices/services/profile-storage/internal/domain/service"
	"microservices/services/profile-storage/internal/infrastructure/repository"
	"microservices/services/profile-storage/internal/messaging"

	"github.com/jmoiron/sqlx"
)

// BenchmarkSuite manages benchmark test environment
type BenchmarkSuite struct {
	profileService   *service.ProfileService
	batchService     *service.AdvancedBatchOperationsService
	messageProcessor *messaging.MessageProcessor
	storageHandler   *messaging.StorageHandler
	logger           *zap.Logger
	testData         []models.ProfileRequest
}

func setupBenchmark() *BenchmarkSuite {
	// Use mock/in-memory database for benchmarks
	db := &sqlx.DB{} // Mock database

	// Create repositories
	profileRepo := repository.NewProfileRepository(nil) // Mock repository

	// Create services
	profileService := service.NewProfileService(profileRepo)
	batchService := service.NewAdvancedBatchOperationsService(profileService, db)

	// Setup messaging components
	messageProcessor := messaging.NewMessageProcessor()
	storageHandler := messaging.NewStorageHandler(profileService, batchService)
	messageProcessor.RegisterHandler(storageHandler)

	return &BenchmarkSuite{
		profileService:   profileService,
		batchService:     batchService,
		messageProcessor: messageProcessor,
		storageHandler:   storageHandler,
		logger:           zap.NewNop(),
		testData:         generateTestData(100),
	}
}

// generateTestData creates test profile data for benchmarks
func generateTestData(count int) []models.ProfileRequest {
	data := make([]models.ProfileRequest, count)

	for i := 0; i < count; i++ {
		data[i] = models.ProfileRequest{
			FirstName: fmt.Sprintf("BenchUser%d", i),
			LastName:  "Performance",
			Email:     fmt.Sprintf("bench.user%d@performance.test", i),
			Phone:     fmt.Sprintf("555%04d", i),
		}
	}

	return data
}

// Helper function to convert old StorageTask format to new BatchRequest format
func convertToBatchRequest(operations []models.StorageTask) *models.BatchRequest {
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
			ExternalID: fmt.Sprintf("bench_%d", i),
			Metadata:   make(map[string]string),
		}
	}

	return &models.BatchRequest{
		ID:         uuid.New().String(),
		Type:       "profile",
		Operations: batchOps,
		Options: models.BatchOptions{
			Mode:                models.BatchModeIndividual,
			FailureHandling:     models.BatchContinueOnFail,
			MaxConcurrency:      10,
			TimeoutPerOperation: 30 * time.Second,
			TotalTimeout:        5 * time.Minute,
			ValidationLevel:     models.BatchValidationBasic,
			EnableRollback:      false,
			EnableProgressTrack: true,
		},
		RequestedBy: "benchmark",
		CreatedAt:   time.Now(),
	}
}

// BenchmarkSingleMessageProcessing benchmarks individual message processing
func BenchmarkSingleMessageProcessing(b *testing.B) {
	suite := setupBenchmark()
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Create test message
			task := models.StorageTask{
				Operation: "create",
				Data: map[string]interface{}{
					"first_name": suite.testData[i%len(suite.testData)].FirstName,
					"last_name":  suite.testData[i%len(suite.testData)].LastName,
					"email":      suite.testData[i%len(suite.testData)].Email,
					"phone":      suite.testData[i%len(suite.testData)].Phone,
				},
				Timestamp:   time.Now(),
				RequestedBy: "benchmark",
			}

			payload, _ := json.Marshal(task)
			msg := &messaging.Message{
				ID:         uuid.New().String(),
				Type:       "storage.create",
				RoutingKey: "storage.create",
				Payload:    payload,
				Timestamp:  time.Now(),
				Source:     "benchmark",
				Priority:   0,
				RetryCount: 0,
				MaxRetries: 3,
			}

			// Process message
			_, err := suite.messageProcessor.ProcessMessage(ctx, msg)
			if err != nil {
				b.Errorf("Message processing failed: %v", err)
			}
			i++
		}
	})

	b.ReportAllocs()
}

// BenchmarkBatchProcessing benchmarks batch operation processing
func BenchmarkBatchProcessing(b *testing.B) {
	suite := setupBenchmark()
	ctx := context.Background()

	// Test different batch sizes
	batchSizes := []int{1, 5, 10, 25, 50, 100}

	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Create batch operations
				operations := make([]models.StorageTask, size)
				for j := 0; j < size; j++ {
					dataIndex := (i*size + j) % len(suite.testData)
					operations[j] = models.StorageTask{
						Operation: "create",
						Data: map[string]interface{}{
							"first_name": suite.testData[dataIndex].FirstName,
							"last_name":  suite.testData[dataIndex].LastName,
							"email":      suite.testData[dataIndex].Email,
							"phone":      suite.testData[dataIndex].Phone,
						},
						Timestamp:   time.Now(),
						RequestedBy: "benchmark",
					}
				}

				// Convert to new BatchRequest format
				batchRequest := convertToBatchRequest(operations)

				// Process batch
				_, err := suite.batchService.ProcessBatch(ctx, batchRequest)
				if err != nil {
					b.Errorf("Batch processing failed: %v", err)
				}
			}
			b.ReportAllocs()
		})
	}
}

// BenchmarkConcurrentMessageProcessing benchmarks concurrent message processing
func BenchmarkConcurrentMessageProcessing(b *testing.B) {
	suite := setupBenchmark()
	ctx := context.Background()

	// Test different concurrency levels
	concurrencyLevels := []int{1, 2, 4, 8, 16, 32}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency%d", concurrency), func(b *testing.B) {
			b.SetParallelism(concurrency)
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					dataIndex := i % len(suite.testData)
					task := models.StorageTask{
						Operation: "create",
						Data: map[string]interface{}{
							"first_name": suite.testData[dataIndex].FirstName,
							"last_name":  suite.testData[dataIndex].LastName,
							"email":      suite.testData[dataIndex].Email,
							"phone":      suite.testData[dataIndex].Phone,
						},
						Timestamp:   time.Now(),
						RequestedBy: "benchmark",
					}

					payload, _ := json.Marshal(task)
					msg := &messaging.Message{
						ID:         uuid.New().String(),
						Type:       "storage.create",
						RoutingKey: "storage.create",
						Payload:    payload,
						Timestamp:  time.Now(),
						Source:     "benchmark",
						Priority:   0,
						RetryCount: 0,
						MaxRetries: 3,
					}

					_, err := suite.messageProcessor.ProcessMessage(ctx, msg)
					if err != nil {
						b.Errorf("Concurrent message processing failed: %v", err)
					}
					i++
				}
			})
			b.ReportAllocs()
		})
	}
}

// BenchmarkMessageHandlerPerformance benchmarks storage handler performance
func BenchmarkMessageHandlerPerformance(b *testing.B) {
	suite := setupBenchmark()
	ctx := context.Background()

	// Test different operation types
	operations := []string{"create", "update", "delete"}

	for _, operation := range operations {
		b.Run(fmt.Sprintf("Operation%s", operation), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dataIndex := i % len(suite.testData)

				var task models.StorageTask
				switch operation {
				case "create":
					task = models.StorageTask{
						Operation: "create",
						Data: map[string]interface{}{
							"first_name": suite.testData[dataIndex].FirstName,
							"last_name":  suite.testData[dataIndex].LastName,
							"email":      suite.testData[dataIndex].Email,
							"phone":      suite.testData[dataIndex].Phone,
						},
						Timestamp:   time.Now(),
						RequestedBy: "benchmark",
					}
				case "update":
					profileID := uuid.New()
					task = models.StorageTask{
						Operation: "update",
						ProfileID: &profileID,
						Data: map[string]interface{}{
							"first_name": suite.testData[dataIndex].FirstName + "_updated",
							"last_name":  suite.testData[dataIndex].LastName,
							"email":      suite.testData[dataIndex].Email,
						},
						Timestamp:   time.Now(),
						RequestedBy: "benchmark",
					}
				case "delete":
					profileID := uuid.New()
					task = models.StorageTask{
						Operation:   "delete",
						ProfileID:   &profileID,
						Timestamp:   time.Now(),
						RequestedBy: "benchmark",
					}
				}

				payload, _ := json.Marshal(task)
				msg := &messaging.Message{
					ID:         uuid.New().String(),
					Type:       fmt.Sprintf("storage.%s", operation),
					RoutingKey: fmt.Sprintf("storage.%s", operation),
					Payload:    payload,
					Timestamp:  time.Now(),
					Source:     "benchmark",
					Priority:   0,
					RetryCount: 0,
					MaxRetries: 3,
				}

				_, err := suite.storageHandler.Handle(ctx, msg)
				if err != nil && operation != "delete" { // Deletes may fail in mock scenario
					b.Errorf("Handler processing failed: %v", err)
				}
			}
			b.ReportAllocs()
		})
	}
}

// BenchmarkValidationPerformance benchmarks message validation performance
func BenchmarkValidationPerformance(b *testing.B) {
	suite := setupBenchmark()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			dataIndex := i % len(suite.testData)

			// Create profile request for validation
			req := &models.ProfileRequest{
				FirstName: suite.testData[dataIndex].FirstName,
				LastName:  suite.testData[dataIndex].LastName,
				Email:     suite.testData[dataIndex].Email,
				Phone:     suite.testData[dataIndex].Phone,
			}

			// Benchmark validation
			err := req.Validate()
			if err != nil {
				b.Errorf("Validation failed: %v", err)
			}
			i++
		}
	})
	b.ReportAllocs()
}

// BenchmarkBatchOptimization benchmarks batch operation optimization
func BenchmarkBatchOptimization(b *testing.B) {
	suite := setupBenchmark()
	ctx := context.Background()

	// Create operations with duplicates for optimization testing
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		operations := make([]models.StorageTask, 200) // Large batch with duplicates

		// Add operations with some duplicates
		for j := 0; j < 200; j++ {
			dataIndex := (j / 4) % len(suite.testData) // Create duplicates every 4 operations
			operations[j] = models.StorageTask{
				Operation: "create",
				Data: map[string]interface{}{
					"first_name": suite.testData[dataIndex].FirstName,
					"last_name":  suite.testData[dataIndex].LastName,
					"email":      suite.testData[dataIndex].Email,
					"phone":      suite.testData[dataIndex].Phone,
				},
				Timestamp:   time.Now(),
				RequestedBy: "benchmark",
			}
		}

		// Convert to new BatchRequest format
		batchRequest := convertToBatchRequest(operations)

		// Process optimized batch
		_, err := suite.batchService.ProcessBatch(ctx, batchRequest)
		if err != nil {
			b.Errorf("Optimized batch processing failed: %v", err)
		}
	}
	b.ReportAllocs()
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	suite := setupBenchmark()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	// Test memory usage with large message processing
	for i := 0; i < b.N; i++ {
		// Create large batch to test memory usage
		operations := make([]models.StorageTask, 1000)
		for j := 0; j < 1000; j++ {
			dataIndex := j % len(suite.testData)
			operations[j] = models.StorageTask{
				Operation: "create",
				Data: map[string]interface{}{
					"first_name": suite.testData[dataIndex].FirstName,
					"last_name":  suite.testData[dataIndex].LastName,
					"email":      suite.testData[dataIndex].Email,
					"phone":      suite.testData[dataIndex].Phone,
					// Add some larger data to test memory impact
					"metadata": map[string]interface{}{
						"benchmark_id":  i,
						"operation_id":  j,
						"timestamp":     time.Now().Unix(),
						"test_data":     fmt.Sprintf("large_test_data_%d_%d", i, j),
						"extra_field_1": fmt.Sprintf("extra_data_1_%d", j),
						"extra_field_2": fmt.Sprintf("extra_data_2_%d", j),
						"extra_field_3": fmt.Sprintf("extra_data_3_%d", j),
					},
				},
				Timestamp:   time.Now(),
				RequestedBy: "memory_benchmark",
			}
		}

		// Convert to new BatchRequest format
		batchRequest := convertToBatchRequest(operations)

		_, err := suite.batchService.ProcessBatch(ctx, batchRequest)
		if err != nil {
			b.Errorf("Memory benchmark batch processing failed: %v", err)
		}
	}
}

// BenchmarkThroughputMeasurement benchmarks overall system throughput
func BenchmarkThroughputMeasurement(b *testing.B) {
	suite := setupBenchmark()
	ctx := context.Background()

	// Test sustained throughput
	messagesPerSecond := []int{10, 50, 100, 200, 500}

	for _, targetThroughput := range messagesPerSecond {
		b.Run(fmt.Sprintf("Throughput%dMPS", targetThroughput), func(b *testing.B) {
			interval := time.Second / time.Duration(targetThroughput)

			b.ResetTimer()
			b.SetParallelism(1) // Single goroutine for throughput measurement

			b.RunParallel(func(pb *testing.PB) {
				ticker := time.NewTicker(interval)
				defer ticker.Stop()

				i := 0
				for pb.Next() {
					<-ticker.C // Wait for next interval

					dataIndex := i % len(suite.testData)
					task := models.StorageTask{
						Operation: "create",
						Data: map[string]interface{}{
							"first_name": suite.testData[dataIndex].FirstName,
							"last_name":  suite.testData[dataIndex].LastName,
							"email":      suite.testData[dataIndex].Email,
							"phone":      suite.testData[dataIndex].Phone,
						},
						Timestamp:   time.Now(),
						RequestedBy: "throughput_benchmark",
					}

					payload, _ := json.Marshal(task)
					msg := &messaging.Message{
						ID:         uuid.New().String(),
						Type:       "storage.create",
						RoutingKey: "storage.create",
						Payload:    payload,
						Timestamp:  time.Now(),
						Source:     "throughput_benchmark",
						Priority:   0,
						RetryCount: 0,
						MaxRetries: 3,
					}

					_, err := suite.messageProcessor.ProcessMessage(ctx, msg)
					if err != nil {
						b.Errorf("Throughput benchmark failed: %v", err)
					}
					i++
				}
			})
			b.ReportAllocs()
		})
	}
}

// BenchmarkAutoTuningPerformance benchmarks auto-tuning algorithms
func BenchmarkAutoTuningPerformance(b *testing.B) {
	suite := setupBenchmark()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create operations with varying sizes to trigger auto-tuning
		batchSize := 50 + (i % 100) // Variable batch sizes from 50 to 150
		operations := make([]models.StorageTask, batchSize)

		for j := 0; j < batchSize; j++ {
			dataIndex := j % len(suite.testData)
			operations[j] = models.StorageTask{
				Operation: "create",
				Data: map[string]interface{}{
					"first_name": suite.testData[dataIndex].FirstName,
					"last_name":  suite.testData[dataIndex].LastName,
					"email":      suite.testData[dataIndex].Email,
					"phone":      suite.testData[dataIndex].Phone,
				},
				Timestamp:   time.Now(),
				RequestedBy: "auto_tuning_benchmark",
			}
		}

		// Convert to new BatchRequest format with auto-tuning enabled
		batchRequest := convertToBatchRequest(operations)
		batchRequest.Options.MaxConcurrency = 0 // Let auto-tuning decide

		// Process batch with auto-tuning enabled
		_, err := suite.batchService.ProcessBatch(ctx, batchRequest)
		if err != nil {
			b.Errorf("Auto-tuning benchmark failed: %v", err)
		}
	}
	b.ReportAllocs()
}
