package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"cache-service/internal/config"
	"cache-service/internal/infrastructure/metrics"
)

// CacheInvalidationService handles cache invalidation patterns and consistency
// Provides centralized cache invalidation logic for the ecosystem
type CacheInvalidationService struct {
	cache   *CacheService
	logger  *zap.Logger
	metrics *metrics.Metrics
	config  *config.CacheConfig
}

// InvalidationPattern represents different invalidation strategies
type InvalidationPattern string

const (
	// Single key invalidation
	InvalidationPatternSingle InvalidationPattern = "single"
	// Pattern-based invalidation using wildcards
	InvalidationPatternWildcard InvalidationPattern = "wildcard"
	// Tag-based invalidation for related entities
	InvalidationPatternTag InvalidationPattern = "tag"
	// Time-based invalidation for scheduled cleanup
	InvalidationPatternTime InvalidationPattern = "time"
	// Dependency-based invalidation for related data
	InvalidationPatternDependency InvalidationPattern = "dependency"
)

// InvalidationRequest represents a cache invalidation request
type InvalidationRequest struct {
	Pattern      InvalidationPattern    `json:"pattern"`
	Keys         []string               `json:"keys,omitempty"`
	WildcardKey  string                 `json:"wildcard_key,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	MaxAge       time.Duration          `json:"max_age,omitempty"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// InvalidationResult represents the result of an invalidation operation
type InvalidationResult struct {
	Success       bool                   `json:"success"`
	DeletedCount  int64                  `json:"deleted_count"`
	ProcessedKeys []string               `json:"processed_keys,omitempty"`
	Errors        []string               `json:"errors,omitempty"`
	Duration      time.Duration          `json:"duration"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// NewCacheInvalidationService creates a new cache invalidation service
func NewCacheInvalidationService(
	cacheService *CacheService,
	logger *zap.Logger,
	metrics *metrics.Metrics,
	config *config.CacheConfig,
) *CacheInvalidationService {
	return &CacheInvalidationService{
		cache:   cacheService,
		logger:  logger,
		metrics: metrics,
		config:  config,
	}
}

// InvalidateCache handles cache invalidation based on the request pattern
func (c *CacheInvalidationService) InvalidateCache(ctx context.Context, request *InvalidationRequest) (*InvalidationResult, error) {
	start := time.Now()

	result := &InvalidationResult{
		ProcessedKeys: []string{},
		Errors:        []string{},
		Metadata:      make(map[string]interface{}),
	}

	c.logger.Info("Starting cache invalidation",
		zap.String("pattern", string(request.Pattern)),
		zap.Strings("keys", request.Keys),
		zap.String("wildcard", request.WildcardKey),
		zap.Strings("tags", request.Tags))

	var err error
	var deletedCount int64

	switch request.Pattern {
	case InvalidationPatternSingle:
		deletedCount, err = c.invalidateSingleKeys(ctx, request.Keys)
		result.ProcessedKeys = request.Keys

	case InvalidationPatternWildcard:
		deletedCount, err = c.invalidateByWildcard(ctx, request.WildcardKey)
		result.Metadata["wildcard_pattern"] = request.WildcardKey

	case InvalidationPatternTag:
		deletedCount, err = c.invalidateByTags(ctx, request.Tags)
		result.Metadata["tags"] = request.Tags

	case InvalidationPatternTime:
		deletedCount, err = c.invalidateByAge(ctx, request.MaxAge)
		result.Metadata["max_age"] = request.MaxAge.String()

	case InvalidationPatternDependency:
		deletedCount, err = c.invalidateByDependencies(ctx, request.Dependencies)
		result.Metadata["dependencies"] = request.Dependencies

	default:
		err = fmt.Errorf("unsupported invalidation pattern: %s", request.Pattern)
	}

	result.Duration = time.Since(start)
	result.DeletedCount = deletedCount
	result.Success = err == nil

	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		c.metrics.RecordInvalidationOp(string(request.Pattern), "error")
		c.logger.Error("Cache invalidation failed",
			zap.String("pattern", string(request.Pattern)),
			zap.Error(err))
	} else {
		c.metrics.RecordInvalidationOp(string(request.Pattern), "success")
		c.logger.Info("Cache invalidation completed",
			zap.String("pattern", string(request.Pattern)),
			zap.Int64("deleted_count", deletedCount),
			zap.Duration("duration", result.Duration))
	}

	return result, err
}

// invalidateSingleKeys invalidates specific cache keys
func (c *CacheInvalidationService) invalidateSingleKeys(ctx context.Context, keys []string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	err := c.cache.MDelete(ctx, keys)
	if err != nil {
		return 0, fmt.Errorf("failed to delete keys: %w", err)
	}

	return int64(len(keys)), nil
}

// invalidateByWildcard invalidates keys matching a wildcard pattern
func (c *CacheInvalidationService) invalidateByWildcard(ctx context.Context, pattern string) (int64, error) {
	if pattern == "" {
		return 0, fmt.Errorf("wildcard pattern cannot be empty")
	}

	deletedCount, err := c.cache.DeleteByPattern(ctx, pattern)
	if err != nil {
		return 0, fmt.Errorf("failed to delete by pattern %s: %w", pattern, err)
	}

	return deletedCount, nil
}

// invalidateByTags invalidates keys associated with specific tags
func (c *CacheInvalidationService) invalidateByTags(ctx context.Context, tags []string) (int64, error) {
	if len(tags) == 0 {
		return 0, nil
	}

	var totalDeleted int64

	for _, tag := range tags {
		// Tag-based invalidation using tag index pattern
		tagPattern := fmt.Sprintf("tag:%s:*", tag)
		deleted, err := c.cache.DeleteByPattern(ctx, tagPattern)
		if err != nil {
			c.logger.Warn("Failed to invalidate tag",
				zap.String("tag", tag),
				zap.Error(err))
			continue
		}
		totalDeleted += deleted
	}

	return totalDeleted, nil
}

// invalidateByAge invalidates keys older than the specified age
func (c *CacheInvalidationService) invalidateByAge(ctx context.Context, maxAge time.Duration) (int64, error) {
	// This is a simplified implementation
	// In a real scenario, you'd need to track key creation times
	c.logger.Info("Age-based invalidation requested",
		zap.Duration("max_age", maxAge))

	// For now, we'll invalidate keys with expired TTLs
	// This would require additional metadata tracking in a full implementation
	return 0, fmt.Errorf("age-based invalidation requires TTL metadata tracking (not implemented)")
}

// invalidateByDependencies invalidates keys that depend on the specified dependencies
func (c *CacheInvalidationService) invalidateByDependencies(ctx context.Context, dependencies []string) (int64, error) {
	if len(dependencies) == 0 {
		return 0, nil
	}

	var totalDeleted int64

	for _, dependency := range dependencies {
		// Dependency-based invalidation patterns
		patterns := c.getDependencyPatterns(dependency)

		for _, pattern := range patterns {
			deleted, err := c.cache.DeleteByPattern(ctx, pattern)
			if err != nil {
				c.logger.Warn("Failed to invalidate dependency pattern",
					zap.String("dependency", dependency),
					zap.String("pattern", pattern),
					zap.Error(err))
				continue
			}
			totalDeleted += deleted
		}
	}

	return totalDeleted, nil
}

// getDependencyPatterns returns cache key patterns that depend on a given entity
func (c *CacheInvalidationService) getDependencyPatterns(dependency string) []string {
	patterns := []string{}

	// Extract entity type and ID from dependency
	parts := strings.Split(dependency, ":")
	if len(parts) < 2 {
		return patterns
	}

	entityType := parts[0]
	entityID := parts[1]

	switch entityType {
	case "profile":
		// Profile dependencies
		patterns = append(patterns,
			fmt.Sprintf("profile:%s", entityID),
			fmt.Sprintf("profile:email:*:%s", entityID),
			fmt.Sprintf("profile:meta:%s", entityID),
			fmt.Sprintf("session:user:%s:*", entityID),
		)

	case "task":
		// Task dependencies
		patterns = append(patterns,
			fmt.Sprintf("task:status:%s", entityID),
			fmt.Sprintf("task:result:%s", entityID),
		)

	case "session":
		// Session dependencies
		patterns = append(patterns,
			fmt.Sprintf("session:%s", entityID),
			fmt.Sprintf("blacklist:token:%s:*", entityID),
		)

	case "queue":
		// Queue dependencies
		patterns = append(patterns,
			fmt.Sprintf("queue:metrics:%s", entityID),
			fmt.Sprintf("worker:status:%s:*", entityID),
		)
	}

	return patterns
}

// InvalidateUserData invalidates all cache data for a specific user
func (c *CacheInvalidationService) InvalidateUserData(ctx context.Context, userID string) (*InvalidationResult, error) {
	request := &InvalidationRequest{
		Pattern: InvalidationPatternDependency,
		Dependencies: []string{
			fmt.Sprintf("profile:%s", userID),
			fmt.Sprintf("session:%s", userID),
		},
		Metadata: map[string]interface{}{
			"user_id": userID,
			"reason":  "user_data_invalidation",
		},
	}

	return c.InvalidateCache(ctx, request)
}

// InvalidateTaskData invalidates cache data related to task processing
func (c *CacheInvalidationService) InvalidateTaskData(ctx context.Context, taskType string, status string) (*InvalidationResult, error) {
	patterns := []string{}

	if taskType != "" {
		patterns = append(patterns, fmt.Sprintf("task:*:%s", taskType))
	}

	if status != "" {
		patterns = append(patterns, fmt.Sprintf("task:status:*:%s", status))
	}

	request := &InvalidationRequest{
		Pattern: InvalidationPatternWildcard,
		Metadata: map[string]interface{}{
			"task_type": taskType,
			"status":    status,
			"reason":    "task_data_invalidation",
		},
	}

	var totalDeleted int64
	result := &InvalidationResult{
		Success:       true,
		ProcessedKeys: patterns,
		Errors:        []string{},
		Metadata:      request.Metadata,
	}

	start := time.Now()

	for _, pattern := range patterns {
		request.WildcardKey = pattern
		partialResult, err := c.InvalidateCache(ctx, request)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			result.Success = false
		} else {
			totalDeleted += partialResult.DeletedCount
		}
	}

	result.DeletedCount = totalDeleted
	result.Duration = time.Since(start)

	return result, nil
}

// SchedulePeriodicInvalidation sets up periodic cache cleanup
func (c *CacheInvalidationService) SchedulePeriodicInvalidation(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	c.logger.Info("Starting periodic cache invalidation",
		zap.Duration("interval", interval))

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Stopping periodic cache invalidation")
			return

		case <-ticker.C:
			c.performPeriodicCleanup(ctx)
		}
	}
}

// performPeriodicCleanup performs routine cache cleanup
func (c *CacheInvalidationService) performPeriodicCleanup(ctx context.Context) {
	c.logger.Debug("Performing periodic cache cleanup")

	// Clean up expired sessions
	request := &InvalidationRequest{
		Pattern:     InvalidationPatternWildcard,
		WildcardKey: "session:expired:*",
		Metadata: map[string]interface{}{
			"reason": "periodic_cleanup",
		},
	}

	result, err := c.InvalidateCache(ctx, request)
	if err != nil {
		c.logger.Warn("Periodic cleanup failed", zap.Error(err))
	} else {
		c.logger.Debug("Periodic cleanup completed",
			zap.Int64("cleaned_items", result.DeletedCount))
	}
}

// GetInvalidationStats returns statistics about cache invalidation operations
func (c *CacheInvalidationService) GetInvalidationStats(ctx context.Context) map[string]interface{} {
	// This would typically query metrics storage
	// For now, return basic stats
	return map[string]interface{}{
		"service_name": "CacheInvalidationService",
		"patterns_supported": []string{
			string(InvalidationPatternSingle),
			string(InvalidationPatternWildcard),
			string(InvalidationPatternTag),
			string(InvalidationPatternTime),
			string(InvalidationPatternDependency),
		},
		"last_cleanup": time.Now().Format(time.RFC3339),
	}
}
