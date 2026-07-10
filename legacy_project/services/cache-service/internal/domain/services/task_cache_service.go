package services

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"cache-service/internal/config"
	"cache-service/internal/domain/models"
	"cache-service/internal/infrastructure/metrics"
)

// TaskCacheService implements task and queue-related caching operations
// Supports caching for Queue-Service and Worker-Service integration
type TaskCacheService struct {
	cache   *CacheService
	logger  *zap.Logger
	metrics *metrics.Metrics
	config  *config.CacheConfig
}

// NewTaskCacheService creates a new task cache service
func NewTaskCacheService(
	cacheService *CacheService,
	logger *zap.Logger,
	metrics *metrics.Metrics,
	config *config.CacheConfig,
) *TaskCacheService {
	return &TaskCacheService{
		cache:   cacheService,
		logger:  logger,
		metrics: metrics,
		config:  config,
	}
}

// GetTaskStatus retrieves a task status from cache by task ID
func (t *TaskCacheService) GetTaskStatus(ctx context.Context, taskID string) (*models.TaskStatus, error) {
	start := time.Now()
	key := t.getTaskStatusKey(taskID)

	var taskStatus models.TaskStatus
	err := t.cache.GetJSON(ctx, key, &taskStatus)
	duration := time.Since(start)

	if err != nil {
		if err == ErrKeyNotFound {
			t.metrics.RecordTaskCacheOp("get_task_status", "miss")
			t.logger.Debug("Task status cache miss", zap.String("task_id", taskID))
		} else {
			t.metrics.RecordTaskCacheOp("get_task_status", "error")
			t.logger.Error("Task status cache get failed",
				zap.String("task_id", taskID),
				zap.Error(err))
		}
		return nil, err
	}

	t.metrics.RecordTaskCacheOp("get_task_status", "hit")
	t.metrics.RecordCacheLatency("get_task_status", "hit", duration)
	t.logger.Debug("Task status cache hit",
		zap.String("task_id", taskID),
		zap.Duration("latency", duration))

	return &taskStatus, nil
}

// SetTaskStatus stores a task status in cache with task-specific TTL
func (t *TaskCacheService) SetTaskStatus(ctx context.Context, taskID string, status *models.TaskStatus, ttl time.Duration) error {
	start := time.Now()
	key := t.getTaskStatusKey(taskID)

	// Use task-specific TTL if not specified (shorter for active tasks)
	if ttl <= 0 {
		switch status.Status {
		case "pending", "running":
			ttl = t.config.TaskTTL / 4 // Shorter TTL for active tasks
		case "completed", "failed":
			ttl = t.config.TaskTTL // Longer TTL for completed tasks
		default:
			ttl = t.config.TaskTTL / 2
		}
	}

	err := t.cache.SetJSON(ctx, key, status, ttl)
	duration := time.Since(start)

	if err != nil {
		t.metrics.RecordTaskCacheOp("set_task_status", "error")
		t.logger.Error("Task status cache set failed",
			zap.String("task_id", taskID),
			zap.String("status", status.Status),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return err
	}

	t.metrics.RecordTaskCacheOp("set_task_status", "success")
	t.metrics.RecordCacheLatency("set_task_status", "success", duration)
	t.logger.Debug("Task status cached",
		zap.String("task_id", taskID),
		zap.String("status", status.Status),
		zap.Duration("ttl", ttl),
		zap.Duration("latency", duration))

	return nil
}

// GetQueueMetrics retrieves queue metrics from cache by queue name
func (t *TaskCacheService) GetQueueMetrics(ctx context.Context, queueName string) (*models.QueueMetrics, error) {
	start := time.Now()
	key := t.getQueueMetricsKey(queueName)

	var queueMetrics models.QueueMetrics
	err := t.cache.GetJSON(ctx, key, &queueMetrics)
	duration := time.Since(start)

	if err != nil {
		if err == ErrKeyNotFound {
			t.metrics.RecordTaskCacheOp("get_queue_metrics", "miss")
			t.logger.Debug("Queue metrics cache miss", zap.String("queue_name", queueName))
		} else {
			t.metrics.RecordTaskCacheOp("get_queue_metrics", "error")
			t.logger.Error("Queue metrics cache get failed",
				zap.String("queue_name", queueName),
				zap.Error(err))
		}
		return nil, err
	}

	t.metrics.RecordTaskCacheOp("get_queue_metrics", "hit")
	t.metrics.RecordCacheLatency("get_queue_metrics", "hit", duration)
	t.logger.Debug("Queue metrics cache hit",
		zap.String("queue_name", queueName),
		zap.Duration("latency", duration))

	return &queueMetrics, nil
}

// SetQueueMetrics stores queue metrics in cache with short TTL (frequently updated)
func (t *TaskCacheService) SetQueueMetrics(ctx context.Context, queueName string, metrics *models.QueueMetrics, ttl time.Duration) error {
	start := time.Now()
	key := t.getQueueMetricsKey(queueName)

	// Use short TTL for frequently updated metrics
	if ttl <= 0 {
		ttl = t.config.QueueMetricsTTL // Typically 30 seconds to 2 minutes
	}

	err := t.cache.SetJSON(ctx, key, metrics, ttl)
	duration := time.Since(start)

	if err != nil {
		t.metrics.RecordTaskCacheOp("set_queue_metrics", "error")
		t.logger.Error("Queue metrics cache set failed",
			zap.String("queue_name", queueName),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return err
	}

	t.metrics.RecordTaskCacheOp("set_queue_metrics", "success")
	t.metrics.RecordCacheLatency("set_queue_metrics", "success", duration)
	t.logger.Debug("Queue metrics cached",
		zap.String("queue_name", queueName),
		zap.Int64("pending_jobs", metrics.PendingJobs),
		zap.Int64("processing_jobs", metrics.ProcessingJobs),
		zap.Duration("ttl", ttl),
		zap.Duration("latency", duration))

	return nil
}

// GetWorkerStatus retrieves worker status from cache by worker type
func (t *TaskCacheService) GetWorkerStatus(ctx context.Context, workerType string) (*models.WorkerStatus, error) {
	start := time.Now()
	key := t.getWorkerStatusKey(workerType)

	var workerStatus models.WorkerStatus
	err := t.cache.GetJSON(ctx, key, &workerStatus)
	duration := time.Since(start)

	if err != nil {
		if err == ErrKeyNotFound {
			t.metrics.RecordTaskCacheOp("get_worker_status", "miss")
			t.logger.Debug("Worker status cache miss", zap.String("worker_type", workerType))
		} else {
			t.metrics.RecordTaskCacheOp("get_worker_status", "error")
			t.logger.Error("Worker status cache get failed",
				zap.String("worker_type", workerType),
				zap.Error(err))
		}
		return nil, err
	}

	t.metrics.RecordTaskCacheOp("get_worker_status", "hit")
	t.metrics.RecordCacheLatency("get_worker_status", "hit", duration)
	t.logger.Debug("Worker status cache hit",
		zap.String("worker_type", workerType),
		zap.Duration("latency", duration))

	return &workerStatus, nil
}

// SetWorkerStatus stores worker status in cache with medium TTL
func (t *TaskCacheService) SetWorkerStatus(ctx context.Context, workerType string, status *models.WorkerStatus, ttl time.Duration) error {
	start := time.Now()
	key := t.getWorkerStatusKey(workerType)

	// Use medium TTL for worker status (updated every few minutes)
	if ttl <= 0 {
		ttl = t.config.WorkerStatusTTL // Typically 5-15 minutes
	}

	err := t.cache.SetJSON(ctx, key, status, ttl)
	duration := time.Since(start)

	if err != nil {
		t.metrics.RecordTaskCacheOp("set_worker_status", "error")
		t.logger.Error("Worker status cache set failed",
			zap.String("worker_type", workerType),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return err
	}

	t.metrics.RecordTaskCacheOp("set_worker_status", "success")
	t.metrics.RecordCacheLatency("set_worker_status", "success", duration)
	t.logger.Debug("Worker status cached",
		zap.String("worker_type", workerType),
		zap.String("worker_type_field", status.WorkerType),
		zap.Int("active_workers", status.ActiveWorkers),
		zap.Duration("ttl", ttl),
		zap.Duration("latency", duration))

	return nil
}

// BatchGetTaskStatuses retrieves multiple task statuses efficiently
func (t *TaskCacheService) BatchGetTaskStatuses(ctx context.Context, taskIDs []string) (map[string]*models.TaskStatus, error) {
	start := time.Now()

	if len(taskIDs) == 0 {
		return make(map[string]*models.TaskStatus), nil
	}

	// Build cache keys
	keys := make([]string, len(taskIDs))
	keyToID := make(map[string]string)

	for i, taskID := range taskIDs {
		key := t.getTaskStatusKey(taskID)
		keys[i] = key
		keyToID[key] = taskID
	}

	// Batch get from cache
	values, err := t.cache.MGet(ctx, keys)
	duration := time.Since(start)

	if err != nil {
		t.metrics.RecordTaskCacheOp("batch_get_task_statuses", "error")
		t.logger.Error("Task status batch get failed",
			zap.Int("task_count", len(taskIDs)),
			zap.Error(err))
		return nil, err
	}

	// Parse results
	taskStatuses := make(map[string]*models.TaskStatus)
	hits := 0

	for key := range values {
		taskID := keyToID[key]
		var taskStatus models.TaskStatus

		if err := t.cache.GetJSON(ctx, key, &taskStatus); err == nil {
			taskStatuses[taskID] = &taskStatus
			hits++
		}
	}

	misses := len(taskIDs) - hits
	t.metrics.RecordTaskCacheOp("batch_get_task_statuses", "success")
	t.metrics.RecordCacheLatency("batch_get_task_statuses", "success", duration)

	t.logger.Debug("Task status batch get completed",
		zap.Int("requested", len(taskIDs)),
		zap.Int("hits", hits),
		zap.Int("misses", misses),
		zap.Duration("latency", duration))

	return taskStatuses, nil
}

// InvalidateTasksByStatus removes tasks from cache by status pattern
func (t *TaskCacheService) InvalidateTasksByStatus(ctx context.Context, status string) (int64, error) {
	start := time.Now()
	pattern := fmt.Sprintf("task:status:*:%s", status)

	deletedCount, err := t.cache.DeleteByPattern(ctx, pattern)
	duration := time.Since(start)

	if err != nil {
		t.metrics.RecordTaskCacheOp("invalidate_tasks_by_status", "error")
		t.logger.Error("Task invalidation by status failed",
			zap.String("status", status),
			zap.Error(err))
		return 0, err
	}

	t.metrics.RecordTaskCacheOp("invalidate_tasks_by_status", "success")
	t.metrics.RecordCacheLatency("invalidate_tasks_by_status", "success", duration)
	t.logger.Info("Tasks invalidated by status",
		zap.String("status", status),
		zap.Int64("deleted_count", deletedCount),
		zap.Duration("latency", duration))

	return deletedCount, nil
}

// Key generation helpers
func (t *TaskCacheService) getTaskStatusKey(taskID string) string {
	return fmt.Sprintf("task:status:%s", taskID)
}

func (t *TaskCacheService) getQueueMetricsKey(queueName string) string {
	return fmt.Sprintf("queue:metrics:%s", queueName)
}

func (t *TaskCacheService) getWorkerStatusKey(workerType string) string {
	return fmt.Sprintf("worker:status:%s", workerType)
}
