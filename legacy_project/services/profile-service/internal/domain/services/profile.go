package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/config"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/infrastructure/cache"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/logger"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/messaging"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ✅ NEW: Routing key mapping for multi-worker architecture
var RoutingKeyMap = map[string]string{
	"profile_update":     "profile.task",  // → Profile Worker
	"email_notification": "email.send",    // → Email Worker
	"image_processing":   "image.process", // → Image Worker
}

// ✅ NEW: Cache metrics for monitoring cache performance
type CacheMetrics struct {
	ProfileCacheHits   int64 `json:"profile_cache_hits"`
	ProfileCacheMisses int64 `json:"profile_cache_misses"`
	CacheOperations    int64 `json:"cache_operations"`
	CacheErrors        int64 `json:"cache_errors"`
}

// ✅ NEW: Comprehensive service health status
type ServiceHealthStatus struct {
	Status              string                    `json:"status"`
	CacheEnabled        bool                      `json:"cache_enabled"`
	CacheMetrics        *CacheMetrics             `json:"cache_metrics"`
	CircuitBreakerStats cache.CircuitBreakerStats `json:"circuit_breaker_stats"`
	CacheHitRatio       float64                   `json:"cache_hit_ratio"`
	HealthChecks        map[string]string         `json:"health_checks"`
	Timestamp           time.Time                 `json:"timestamp"`
}

// ProfileError represents a profile service error
type ProfileError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *ProfileError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// ProfileServiceInterface defines the interface for profile-related operations
type ProfileServiceInterface interface {
	// Existing profile methods
	GetProfiles(ctx context.Context) ([]*models.Profile, error)
	GetProfile(ctx context.Context, id string) (*models.Profile, error)
	CreateProfile(ctx context.Context, req *models.ProfileRequest) (*models.Profile, error)
	UpdateProfile(ctx context.Context, id string, req *models.ProfileRequest) (*models.Profile, error)
	DeleteProfile(ctx context.Context, id string) error
	SubmitTask(ctx context.Context, profileID string, req *models.TaskRequest) (*models.Task, error)

	// NEW: User management methods
	CreateUser(ctx context.Context, userData *models.CreateUserRequest) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	UpdateUser(ctx context.Context, userID string, userData *models.UpdateUserRequest) (*models.User, error)
	DeleteUser(ctx context.Context, userID string) error
	// ✅ NEW: Specialized task submission methods
	SubmitEmailTask(ctx context.Context, profileID string, emailPayload *models.EmailTaskPayload) (*models.EmailTaskResponse, error)
	SubmitImageTask(ctx context.Context, profileID string, imagePayload *models.ImageTaskPayload) (*models.ImageTaskResponse, error)
	// ✅ NEW: Cache management methods
	InvalidateProfileCache(ctx context.Context, profileID string) error
	GetCacheMetrics() *CacheMetrics
	// ✅ NEW: Batch operations for enhanced performance
	GetProfilesBatch(ctx context.Context, profileIDs []string) ([]*models.Profile, error)
	WarmProfileCache(ctx context.Context, profileIDs []string) error
	// ✅ NEW: Comprehensive monitoring
	GetServiceHealth() *ServiceHealthStatus
}

// ProfileService handles profile-related business logic with cache-aside pattern
type ProfileService struct {
	storageClient *StorageClient
	queueClient   *messaging.QueueClient
	cacheClient   cache.CacheClientInterface
	authClient    AuthServiceClientInterface // Changed from *AuthServiceClient to interface
	config        *config.Config
	metrics       *CacheMetrics
	logger        *zap.Logger
}

// NewProfileService creates a new profile service with cache integration
func NewProfileService(cfg *config.Config, storageClient *StorageClient, cacheClient cache.CacheClientInterface, authClient AuthServiceClientInterface, logger *zap.Logger) *ProfileService {
	// ✅ ENHANCED: Initialize queue client with new configuration structure
	queueConfig := &messaging.QueueConfig{
		URL:                   cfg.Queue.URL,
		Timeout:               cfg.Queue.Timeout,
		Retries:               cfg.Queue.Retries,
		MaxRequestSize:        cfg.Queue.MaxRequestSize,
		CircuitBreakerEnabled: cfg.Queue.CircuitBreaker.Enabled,
		FailureThreshold:      cfg.Queue.CircuitBreaker.FailureThreshold,
		RecoveryTimeout:       cfg.Queue.CircuitBreaker.RecoveryTimeout,
		MaxConcurrentRequests: cfg.Queue.CircuitBreaker.MaxConcurrentRequests,
	}

	queueClient, err := messaging.NewQueueClient(queueConfig)
	if err != nil {
		logger.Error("Failed to initialize enhanced queue client",
			zap.String("queue_url", cfg.Queue.URL),
			zap.Bool("circuit_breaker_enabled", cfg.Queue.CircuitBreaker.Enabled),
			zap.Error(err))
		// Don't fail the service startup, but log the error
	} else {
		logger.Info("Successfully initialized enhanced queue client",
			zap.String("queue_url", cfg.Queue.URL),
			zap.Bool("circuit_breaker_enabled", cfg.Queue.CircuitBreaker.Enabled),
			zap.Int("failure_threshold", cfg.Queue.CircuitBreaker.FailureThreshold),
			zap.Duration("recovery_timeout", cfg.Queue.CircuitBreaker.RecoveryTimeout))
	}

	service := &ProfileService{
		storageClient: storageClient,
		queueClient:   queueClient,
		cacheClient:   cacheClient,
		authClient:    authClient,
		config:        cfg,
		metrics:       &CacheMetrics{},
		logger:        logger,
	}

	logger.Info("ProfileService initialized with cache-aside pattern",
		zap.Bool("cache_enabled", cfg.Cache.Enabled),
		zap.Duration("profile_ttl", cfg.Cache.TTL.Profile))

	return service
}

// GetProfiles retrieves all profiles
func (s *ProfileService) GetProfiles(ctx context.Context) ([]*models.Profile, error) {
	logger.LogInfo(ctx, "Getting all profiles")
	profiles, err := s.storageClient.GetProfiles(ctx)
	if err != nil {
		logger.LogError(ctx, "Error getting profiles", err)
		return nil, &ProfileError{
			Code:    500,
			Message: "Failed to get profiles",
			Err:     err,
		}
	}
	logger.LogInfo(ctx, "Successfully retrieved profiles",
		zap.Int("count", len(profiles)))
	return profiles, nil
}

// GetProfile retrieves a profile by ID using cache-aside pattern
func (s *ProfileService) GetProfile(ctx context.Context, id string) (*models.Profile, error) {
	if id == "" {
		return nil, &ProfileError{
			Code:    400,
			Message: "Profile ID is required",
		}
	}

	s.logger.Debug("Getting profile with cache-aside pattern", zap.String("id", id))

	// Step 1: Try cache first (cache-aside pattern)
	if s.config.Cache.Enabled && s.cacheClient != nil {
		s.metrics.CacheOperations++
		cachedData, err := s.cacheClient.GetProfile(ctx, id)
		if err == nil {
			// Cache hit - deserialize and return
			var profile models.Profile
			if err := json.Unmarshal(cachedData, &profile); err == nil {
				s.metrics.ProfileCacheHits++
				profile.GetFrom = "cache" // Mark as retrieved from cache
				s.logger.Debug("Profile cache hit",
					zap.String("id", id),
					zap.String("source", "cache"))
				return &profile, nil
			} else {
				s.logger.Warn("Failed to unmarshal cached profile, will fetch from storage",
					zap.String("id", id),
					zap.Error(err))
			}
		} else if err != cache.ErrKeyNotFound {
			// Cache error (not a miss) - log but continue to storage
			s.metrics.CacheErrors++
			s.logger.Warn("Cache error for profile retrieval, falling back to storage",
				zap.String("id", id),
				zap.Error(err))
		}
		// Cache miss - continue to storage
		s.metrics.ProfileCacheMisses++
	}

	// Step 2: Cache miss or cache disabled - get from storage
	s.logger.Debug("Profile cache miss, fetching from storage", zap.String("id", id))
	profile, err := s.storageClient.GetProfile(ctx, id)
	if err != nil {
		s.logger.Error("Error getting profile from storage", zap.String("id", id), zap.Error(err))
		return nil, &ProfileError{
			Code:    500,
			Message: fmt.Sprintf("Failed to get profile %s", id),
			Err:     err,
		}
	}

	// Step 3: Cache the retrieved profile asynchronously (fire and forget)
	if s.config.Cache.Enabled && s.cacheClient != nil && profile != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			profileJSON, err := json.Marshal(profile)
			if err != nil {
				s.logger.Warn("Failed to marshal profile for caching",
					zap.String("id", id),
					zap.Error(err))
				return
			}

			// Cache with configured TTL
			if err := s.cacheClient.SetProfile(cacheCtx, id, profileJSON, s.config.Cache.TTL.Profile); err != nil {
				s.metrics.CacheErrors++
				s.logger.Warn("Failed to cache profile after storage retrieval",
					zap.String("id", id),
					zap.Error(err))
			} else {
				s.logger.Debug("Successfully cached profile after storage retrieval",
					zap.String("id", id),
					zap.Duration("ttl", s.config.Cache.TTL.Profile))
			}
		}()
	}

	profile.GetFrom = "storage" // Mark as retrieved from storage
	s.logger.Info("Successfully retrieved profile",
		zap.String("id", id),
		zap.String("source", "storage"))
	return profile, nil
}

// CreateProfile creates a new profile
func (s *ProfileService) CreateProfile(ctx context.Context, req *models.ProfileRequest) (*models.Profile, error) {
	if req == nil {
		return nil, &ProfileError{
			Code:    400,
			Message: "Profile request is required",
		}
	}

	if err := req.Validate(); err != nil {
		logger.LogError(ctx, "Invalid profile request", err)
		return nil, &ProfileError{
			Code:    400,
			Message: "Invalid profile request",
			Err:     err,
		}
	}

	logger.LogInfo(ctx, "Creating new profile",
		zap.String("email", req.Email))
	profile := &models.Profile{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Phone:     req.Phone,
		Bio:       req.Bio,
		ImageURLs: req.ImageURLs,
		Address:   req.Address,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	createdProfile, err := s.storageClient.CreateProfile(ctx, profile)
	if err != nil {
		logger.LogError(ctx, "Error creating profile", err,
			zap.String("email", req.Email))
		return nil, &ProfileError{
			Code:    500,
			Message: "Failed to create profile",
			Err:     err,
		}
	}
	logger.LogInfo(ctx, "Successfully created profile",
		zap.String("id", createdProfile.ID.String()),
		zap.String("email", req.Email))
	return createdProfile, nil
}

// UpdateProfile updates an existing profile with cache invalidation
func (s *ProfileService) UpdateProfile(ctx context.Context, id string, req *models.ProfileRequest) (*models.Profile, error) {
	if id == "" {
		return nil, &ProfileError{
			Code:    400,
			Message: "Profile ID is required",
		}
	}

	if req == nil {
		return nil, &ProfileError{
			Code:    400,
			Message: "Profile request is required",
		}
	}

	if err := req.Validate(); err != nil {
		s.logger.Error("Invalid profile request", zap.String("id", id), zap.Error(err))
		return nil, &ProfileError{
			Code:    400,
			Message: "Invalid profile request",
			Err:     err,
		}
	}

	s.logger.Info("Updating profile", zap.String("id", id))

	// First get the existing profile
	existingProfile, err := s.storageClient.GetProfile(ctx, id)
	if err != nil {
		s.logger.Error("Error getting existing profile",
			zap.String("id", id),
			zap.Error(err))
		return nil, &ProfileError{
			Code:    500,
			Message: fmt.Sprintf("Failed to get existing profile %s", id),
			Err:     err,
		}
	}

	// Update the fields
	existingProfile.FirstName = req.FirstName
	existingProfile.LastName = req.LastName
	existingProfile.Email = req.Email
	existingProfile.Phone = req.Phone
	existingProfile.Bio = req.Bio
	existingProfile.ImageURLs = req.ImageURLs
	existingProfile.Address = req.Address
	existingProfile.UpdatedAt = time.Now()

	updatedProfile, err := s.storageClient.UpdateProfile(ctx, id, existingProfile)
	if err != nil {
		s.logger.Error("Error updating profile",
			zap.String("id", id),
			zap.Error(err))
		return nil, &ProfileError{
			Code:    500,
			Message: fmt.Sprintf("Failed to update profile %s", id),
			Err:     err,
		}
	}

	// ✅ NEW: Invalidate cache after successful update (async)
	if s.config.Cache.Enabled && s.cacheClient != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			if err := s.cacheClient.Delete(cacheCtx, fmt.Sprintf("profile:%s", id)); err != nil {
				s.metrics.CacheErrors++
				s.logger.Warn("Failed to invalidate profile cache after update",
					zap.String("id", id),
					zap.Error(err))
			} else {
				s.logger.Debug("Successfully invalidated profile cache after update",
					zap.String("id", id))
			}
		}()
	}

	s.logger.Info("Successfully updated profile", zap.String("id", id))
	return updatedProfile, nil
}

// DeleteProfile deletes a profile with cache invalidation
func (s *ProfileService) DeleteProfile(ctx context.Context, id string) error {
	if id == "" {
		return &ProfileError{
			Code:    400,
			Message: "Profile ID is required",
		}
	}

	s.logger.Info("Deleting profile", zap.String("id", id))

	err := s.storageClient.DeleteProfile(ctx, id)
	if err != nil {
		s.logger.Error("Error deleting profile",
			zap.String("id", id),
			zap.Error(err))
		return &ProfileError{
			Code:    500,
			Message: fmt.Sprintf("Failed to delete profile %s", id),
			Err:     err,
		}
	}

	// ✅ NEW: Invalidate cache after successful deletion (async)
	if s.config.Cache.Enabled && s.cacheClient != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			if err := s.cacheClient.Delete(cacheCtx, fmt.Sprintf("profile:%s", id)); err != nil {
				s.metrics.CacheErrors++
				s.logger.Warn("Failed to invalidate profile cache after deletion",
					zap.String("id", id),
					zap.Error(err))
			} else {
				s.logger.Debug("Successfully invalidated profile cache after deletion",
					zap.String("id", id))
			}
		}()
	}

	s.logger.Info("Successfully deleted profile", zap.String("id", id))
	return nil
}

// ✅ NEW: determineRoutingKey maps task types to appropriate routing keys
func (s *ProfileService) determineRoutingKey(messageType string) string {
	if routingKey, exists := RoutingKeyMap[messageType]; exists {
		logger.LogInfo(context.Background(), "Determined routing key",
			zap.String("message_type", messageType),
			zap.String("routing_key", routingKey))
		return routingKey
	}
	logger.LogInfo(context.Background(), "Using fallback routing key",
		zap.String("message_type", messageType),
		zap.String("routing_key", "profile.task"))
	return "profile.task" // Default fallback
}

// ✅ NEW: Enhanced task submission with task-specific handling
func (s *ProfileService) SubmitTask(ctx context.Context, profileID string, req *models.TaskRequest) (*models.Task, error) {
	if profileID == "" {
		logger.LogError(ctx, "Profile ID is required", nil)
		return nil, &ProfileError{
			Code:    400,
			Message: "Profile ID is required",
		}
	}

	if req == nil {
		logger.LogError(ctx, "Task request is required", nil)
		return nil, &ProfileError{
			Code:    400,
			Message: "Task request is required",
		}
	}

	// Enhanced validation with task-specific logic
	if err := req.Validate(); err != nil {
		logger.LogError(ctx, "Task validation failed", err,
			zap.String("profile_id", profileID),
			zap.String("task_type", req.Type))
		return nil, &ProfileError{
			Code:    400,
			Message: fmt.Sprintf("Task validation failed: %s", err.Error()),
			Err:     err,
		}
	}

	// Determine routing key for message type
	routingKey := s.determineRoutingKey(req.Type)

	// ✅ NEW: Enhanced logging with task type context
	logger.LogInfo(ctx, "Creating new task with enhanced context",
		zap.String("profile_id", profileID),
		zap.String("task_type", req.Type),
		zap.String("routing_key", routingKey),
		zap.String("worker_target", s.getWorkerTarget(req.Type)))

	// Create task with enhanced metadata
	task := &models.Task{
		ID:        uuid.New(),
		ProfileID: profileID,
		Type:      req.Type,
		Status:    "pending",
		Payload:   req.Payload,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// ✅ NEW: Task-specific processing
	if err := s.processTaskByType(ctx, task, routingKey); err != nil {
		return nil, err
	}

	// Store task with enhanced logging
	logger.LogInfo(ctx, "Successfully stored task with routing context",
		zap.String("profile_id", profileID),
		zap.String("task_id", task.ID.String()),
		zap.String("task_type", req.Type),
		zap.String("routing_key", routingKey))

	// Send message to queue with enhanced payload
	if err := s.sendTaskToQueue(ctx, task, routingKey); err != nil {
		return nil, err
	}

	logger.LogInfo(ctx, "Successfully processed multi-worker task",
		zap.String("profile_id", profileID),
		zap.String("task_id", task.ID.String()),
		zap.String("task_type", req.Type),
		zap.String("routing_key", routingKey),
		zap.String("worker_target", s.getWorkerTarget(req.Type)))

	return task, nil
}

// ✅ NEW: Specialized email task submission
func (s *ProfileService) SubmitEmailTask(ctx context.Context, profileID string, emailPayload *models.EmailTaskPayload) (*models.EmailTaskResponse, error) {
	logger.LogInfo(ctx, "Processing email task submission",
		zap.String("profile_id", profileID),
		zap.String("email_to", emailPayload.To),
		zap.String("template", emailPayload.Template),
		zap.Int("priority", emailPayload.Priority))

	// Validate email payload
	if err := emailPayload.Validate(); err != nil {
		logger.LogError(ctx, "Email task validation failed", err,
			zap.String("profile_id", profileID),
			zap.String("email_to", emailPayload.To))
		return nil, &ProfileError{
			Code:    400,
			Message: fmt.Sprintf("Email task validation failed: %s", err.Error()),
			Err:     err,
		}
	}

	// Create task request - convert struct to map for validation compatibility
	payloadMap := map[string]interface{}{
		"to":       emailPayload.To,
		"template": emailPayload.Template,
		"subject":  emailPayload.Subject,
		"priority": emailPayload.Priority,
		"data":     emailPayload.Data,
	}

	taskReq := &models.TaskRequest{
		Type:    "email_notification",
		Payload: payloadMap,
	}

	// Submit task
	task, err := s.SubmitTask(ctx, profileID, taskReq)
	if err != nil {
		return nil, err
	}

	// Create email-specific response
	routingKey := s.determineRoutingKey("email_notification")
	response := models.CreateEmailTaskResponse(task, routingKey)

	logger.LogInfo(ctx, "Successfully created email task",
		zap.String("task_id", response.TaskID),
		zap.String("email_to", response.EmailTo),
		zap.String("template", response.Template),
		zap.String("routing_key", response.RoutingKey))

	return response, nil
}

// ✅ NEW: Specialized image task submission
func (s *ProfileService) SubmitImageTask(ctx context.Context, profileID string, imagePayload *models.ImageTaskPayload) (*models.ImageTaskResponse, error) {
	logger.LogInfo(ctx, "Processing image task submission",
		zap.String("profile_id", profileID),
		zap.String("image_url", imagePayload.ImageURL),
		zap.String("operation", imagePayload.Operation),
		zap.String("output_format", imagePayload.OutputFormat))

	// Create task request - convert struct to map for validation compatibility
	payloadMap := map[string]interface{}{
		"image_url":     imagePayload.ImageURL,
		"operation":     imagePayload.Operation,
		"output_format": imagePayload.OutputFormat,
		"options":       imagePayload.Options,
	}

	taskReq := &models.TaskRequest{
		Type:    "image_processing",
		Payload: payloadMap,
	}

	// Submit task
	task, err := s.SubmitTask(ctx, profileID, taskReq)
	if err != nil {
		return nil, err
	}

	// Create image-specific response
	routingKey := s.determineRoutingKey("image_processing")
	response := models.CreateImageTaskResponse(task, routingKey)

	logger.LogInfo(ctx, "Successfully created image task",
		zap.String("task_id", response.TaskID),
		zap.String("image_url", response.ImageURL),
		zap.String("operation", response.Operation),
		zap.String("routing_key", response.RoutingKey))

	return response, nil
}

// ✅ NEW: Task-specific processing logic
func (s *ProfileService) processTaskByType(ctx context.Context, task *models.Task, routingKey string) error {
	switch task.Type {
	case "email_notification":
		return s.processEmailTask(ctx, task, routingKey)
	case "image_processing":
		return s.processImageTask(ctx, task, routingKey)
	case "profile_update":
		return s.processProfileTask(ctx, task, routingKey)
	default:
		logger.LogWarn(ctx, "Unknown task type, using default processing",
			zap.String("task_type", task.Type),
			zap.String("task_id", task.ID.String()))
		return nil
	}
}

// ✅ NEW: Email task processing
func (s *ProfileService) processEmailTask(ctx context.Context, task *models.Task, routingKey string) error {
	emailPayload, ok := task.Payload.(map[string]interface{})
	if !ok {
		return &ProfileError{
			Code:    400,
			Message: "Invalid email task payload format",
		}
	}

	// Convert map to EmailTaskPayload
	emailPayloadStruct := &models.EmailTaskPayload{
		To:       emailPayload["to"].(string),
		Template: emailPayload["template"].(string),
	}
	if subject, exists := emailPayload["subject"]; exists {
		emailPayloadStruct.Subject = subject.(string)
	}
	if priority, exists := emailPayload["priority"]; exists {
		if p, ok := priority.(float64); ok {
			emailPayloadStruct.Priority = int(p)
		}
	}
	if data, exists := emailPayload["data"]; exists {
		emailPayloadStruct.Data = data.(map[string]interface{})
	}

	logger.LogInfo(ctx, "Processing email task with enhanced metadata",
		zap.String("task_id", task.ID.String()),
		zap.String("email_to", emailPayloadStruct.To),
		zap.String("template", emailPayloadStruct.Template),
		zap.String("routing_key", routingKey),
		zap.String("worker_target", "email-worker"))

	// TODO: Add email-specific business logic here
	// e.g., template validation, recipient verification, etc.

	return nil
}

// ✅ NEW: Image task processing
func (s *ProfileService) processImageTask(ctx context.Context, task *models.Task, routingKey string) error {
	imagePayload, ok := task.Payload.(map[string]interface{})
	if !ok {
		return &ProfileError{
			Code:    400,
			Message: "Invalid image task payload format",
		}
	}

	// Convert map to ImageTaskPayload
	imagePayloadStruct := &models.ImageTaskPayload{
		ImageURL:  imagePayload["image_url"].(string),
		Operation: imagePayload["operation"].(string),
	}
	if outputFormat, exists := imagePayload["output_format"]; exists {
		imagePayloadStruct.OutputFormat = outputFormat.(string)
	}
	if options, exists := imagePayload["options"]; exists {
		imagePayloadStruct.Options = options.(map[string]interface{})
	}

	logger.LogInfo(ctx, "Processing image task with enhanced metadata",
		zap.String("task_id", task.ID.String()),
		zap.String("image_url", imagePayloadStruct.ImageURL),
		zap.String("operation", imagePayloadStruct.Operation),
		zap.String("output_format", imagePayloadStruct.OutputFormat),
		zap.String("routing_key", routingKey),
		zap.String("worker_target", "image-worker"))

	// TODO: Add image-specific business logic here
	// e.g., URL validation, operation parameter validation, etc.

	return nil
}

// ✅ NEW: Profile task processing
func (s *ProfileService) processProfileTask(ctx context.Context, task *models.Task, routingKey string) error {
	logger.LogInfo(ctx, "Processing profile task with enhanced metadata",
		zap.String("task_id", task.ID.String()),
		zap.String("task_type", task.Type),
		zap.String("routing_key", routingKey),
		zap.String("worker_target", "profile-worker"))

	// TODO: Add profile-specific business logic here
	// e.g., profile validation, permission checks, etc.

	return nil
}

// ✅ NEW: Enhanced queue message sending with task-specific metadata
func (s *ProfileService) sendTaskToQueue(ctx context.Context, task *models.Task, routingKey string) error {
	// Create enhanced payload with task metadata
	payloadData := map[string]interface{}{
		"task_id":       task.ID.String(),
		"profile_id":    task.ProfileID,
		"task_type":     task.Type,
		"payload":       task.Payload,
		"worker_target": s.getWorkerTarget(task.Type),
		"created_at":    task.CreatedAt.UTC(),
	}

	// Serialize payload to json.RawMessage
	payloadBytes, err := json.Marshal(payloadData)
	if err != nil {
		logger.LogError(ctx, "Failed to serialize enhanced task payload", err,
			zap.String("profile_id", task.ProfileID),
			zap.String("task_id", task.ID.String()),
			zap.String("task_type", task.Type))
		return &ProfileError{
			Code:    500,
			Message: "Failed to serialize task payload",
			Err:     err,
		}
	}

	// Create queue message with enhanced metadata
	msg := &messaging.QueueMessage{
		ID:        task.ID.String(),
		Type:      task.Type,
		Payload:   json.RawMessage(payloadBytes),
		Timestamp: task.CreatedAt,
		Metadata: map[string]string{
			"source":        "profile-service",
			"worker_target": s.getWorkerTarget(task.Type),
			"profile_id":    task.ProfileID,
		},
		RoutingKey: routingKey,
	}

	logger.LogInfo(ctx, "Sending enhanced task to queue",
		zap.String("profile_id", task.ProfileID),
		zap.String("task_id", task.ID.String()),
		zap.String("task_type", task.Type),
		zap.String("routing_key", routingKey),
		zap.String("worker_target", s.getWorkerTarget(task.Type)),
		zap.String("queue_url", s.queueClient.GetQueueServiceURL()))

	if err := s.queueClient.PublishMessage(ctx, msg); err != nil {
		logger.LogError(ctx, "Failed to send enhanced task to queue", err,
			zap.String("profile_id", task.ProfileID),
			zap.String("task_id", task.ID.String()),
			zap.String("task_type", task.Type),
			zap.String("routing_key", routingKey),
			zap.String("worker_target", s.getWorkerTarget(task.Type)),
			zap.String("queue_url", s.queueClient.GetQueueServiceURL()))
		return &ProfileError{
			Code:    500,
			Message: "Failed to send task to queue",
			Err:     err,
		}
	}

	return nil
}

// ✅ NEW: Helper method to determine worker target
func (s *ProfileService) getWorkerTarget(taskType string) string {
	switch taskType {
	case "email_notification":
		return "email-worker"
	case "image_processing":
		return "image-worker"
	case "profile_update":
		return "profile-worker"
	default:
		return "unknown-worker"
	}
}

// ✅ NEW: Cache management methods
func (s *ProfileService) InvalidateProfileCache(ctx context.Context, profileID string) error {
	s.metrics.CacheOperations++
	if err := s.cacheClient.Delete(ctx, profileID); err != nil {
		s.metrics.CacheErrors++
		logger.LogError(ctx, "Failed to invalidate profile cache", err, zap.String("profile_id", profileID))
		return &ProfileError{
			Code:    500,
			Message: "Failed to invalidate profile cache",
			Err:     err,
		}
	}
	s.metrics.ProfileCacheMisses++
	logger.LogInfo(ctx, "Successfully invalidated profile cache", zap.String("profile_id", profileID))
	return nil
}

func (s *ProfileService) GetCacheMetrics() *CacheMetrics {
	return s.metrics
}

// ✅ NEW: Comprehensive monitoring
func (s *ProfileService) GetServiceHealth() *ServiceHealthStatus {
	healthChecks := make(map[string]string)
	healthChecks["profile_service_up"] = "ok"

	cacheHitRatio := 0.0
	if s.metrics.CacheOperations > 0 {
		cacheHitRatio = float64(s.metrics.ProfileCacheHits) / float64(s.metrics.CacheOperations)
	}

	serviceHealth := &ServiceHealthStatus{
		Status:              "ok",
		CacheEnabled:        s.config.Cache.Enabled,
		CacheMetrics:        s.metrics,
		CircuitBreakerStats: s.cacheClient.GetCircuitBreakerStats(),
		CacheHitRatio:       cacheHitRatio,
		HealthChecks:        healthChecks,
		Timestamp:           time.Now(),
	}

	// Check if any health check failed
	for _, status := range serviceHealth.HealthChecks {
		if status != "ok" {
			serviceHealth.Status = "warning"
			break
		}
	}

	return serviceHealth
}

// ✅ NEW: Batch operations for enhanced performance
func (s *ProfileService) GetProfilesBatch(ctx context.Context, profileIDs []string) ([]*models.Profile, error) {
	if len(profileIDs) == 0 {
		return nil, &ProfileError{
			Code:    400,
			Message: "Profile IDs list is empty",
		}
	}

	// Attempt to fetch from cache first
	profiles := make([]*models.Profile, 0, len(profileIDs))
	missedIDs := make([]string, 0)

	for _, id := range profileIDs {
		if s.config.Cache.Enabled && s.cacheClient != nil {
			s.metrics.CacheOperations++
			cachedData, err := s.cacheClient.GetProfile(ctx, id)
			if err == nil {
				var profile models.Profile
				if err := json.Unmarshal(cachedData, &profile); err == nil {
					s.metrics.ProfileCacheHits++
					profile.GetFrom = "cache"
					profiles = append(profiles, &profile)
				} else {
					s.logger.Warn("Failed to unmarshal cached profile, will fetch from storage",
						zap.String("id", id),
						zap.Error(err))
					missedIDs = append(missedIDs, id)
				}
			} else if err != cache.ErrKeyNotFound {
				s.metrics.CacheErrors++
				s.logger.Warn("Cache error for profile retrieval, falling back to storage",
					zap.String("id", id),
					zap.Error(err))
				missedIDs = append(missedIDs, id)
			} else {
				missedIDs = append(missedIDs, id)
			}
		} else {
			missedIDs = append(missedIDs, id)
		}
	}

	// If all profiles were in cache, return
	if len(missedIDs) == 0 {
		s.logger.Info("All profiles found in cache", zap.Int("count", len(profiles)))
		return profiles, nil
	}

	// Fetch missed profiles from storage (individually since batch doesn't exist yet)
	for _, id := range missedIDs {
		profile, err := s.storageClient.GetProfile(ctx, id)
		if err != nil {
			s.logger.Error("Error fetching profile from storage",
				zap.String("id", id),
				zap.Error(err))
			// Continue with other profiles instead of failing completely
			continue
		}

		profile.GetFrom = "storage"
		profiles = append(profiles, profile)

		// Cache profile asynchronously
		if s.config.Cache.Enabled && s.cacheClient != nil {
			go func(p *models.Profile, profileID string) {
				cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				profileJSON, err := json.Marshal(p)
				if err != nil {
					s.logger.Warn("Failed to marshal profile for caching",
						zap.String("id", profileID),
						zap.Error(err))
					return
				}

				if err := s.cacheClient.SetProfile(cacheCtx, profileID, profileJSON, s.config.Cache.TTL.Profile); err != nil {
					s.metrics.CacheErrors++
					s.logger.Warn("Failed to cache profile after storage retrieval",
						zap.String("id", profileID),
						zap.Error(err))
				} else {
					s.logger.Debug("Successfully cached profile after storage retrieval",
						zap.String("id", profileID),
						zap.Duration("ttl", s.config.Cache.TTL.Profile))
				}
			}(profile, id)
		}
	}

	s.logger.Info("Successfully retrieved profiles batch",
		zap.Int("total_count", len(profiles)),
		zap.Int("cache_hits", len(profileIDs)-len(missedIDs)),
		zap.Int("storage_fetches", len(missedIDs)))

	return profiles, nil
}

func (s *ProfileService) WarmProfileCache(ctx context.Context, profileIDs []string) error {
	if len(profileIDs) == 0 {
		s.logger.Info("No profile IDs to warm cache for", zap.Int("count", 0))
		return nil
	}

	if s.storageClient == nil {
		s.logger.Error("Storage client not initialized for cache warming")
		return &ProfileError{
			Code:    500,
			Message: "Storage client not initialized for cache warming",
		}
	}

	// Fetch profiles individually and cache them
	warmedCount := 0
	for _, id := range profileIDs {
		profile, err := s.storageClient.GetProfile(ctx, id)
		if err != nil {
			s.logger.Warn("Error fetching profile for cache warming",
				zap.String("id", id),
				zap.Error(err))
			continue // Continue with other profiles
		}

		// Cache profile asynchronously
		if s.config.Cache.Enabled && s.cacheClient != nil {
			go func(p *models.Profile, profileID string) {
				cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				profileJSON, err := json.Marshal(p)
				if err != nil {
					s.logger.Warn("Failed to marshal profile for caching",
						zap.String("id", profileID),
						zap.Error(err))
					return
				}

				if err := s.cacheClient.SetProfile(cacheCtx, profileID, profileJSON, s.config.Cache.TTL.Profile); err != nil {
					s.metrics.CacheErrors++
					s.logger.Warn("Failed to cache profile during cache warming",
						zap.String("id", profileID),
						zap.Error(err))
				} else {
					s.logger.Debug("Successfully cached profile during cache warming",
						zap.String("id", profileID),
						zap.Duration("ttl", s.config.Cache.TTL.Profile))
				}
			}(profile, id)
		}
		warmedCount++
	}

	s.logger.Info("Successfully warmed profile cache", zap.Int("count", warmedCount))
	return nil
}

func (s *ProfileService) CreateUser(ctx context.Context, userData *models.CreateUserRequest) (*models.User, error) {
	s.logger.Info("Creating user via auth service", zap.String("email", userData.Email))

	user, err := s.authClient.CreateUser(ctx, userData)
	if err != nil {
		s.logger.Error("Failed to create user via auth service",
			zap.String("email", userData.Email),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	s.logger.Info("User created successfully", zap.String("user_id", user.ID))
	return user, nil
}

func (s *ProfileService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	s.logger.Debug("Getting user by email via auth service", zap.String("email", email))

	user, err := s.authClient.GetUserByEmail(ctx, email)
	if err != nil {
		s.logger.Error("Failed to get user by email via auth service",
			zap.String("email", email),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (s *ProfileService) UpdateUser(ctx context.Context, userID string, userData *models.UpdateUserRequest) (*models.User, error) {
	s.logger.Info("Updating user via auth service", zap.String("user_id", userID))

	user, err := s.authClient.UpdateUser(ctx, userID, userData)
	if err != nil {
		s.logger.Error("Failed to update user via auth service",
			zap.String("user_id", userID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	s.logger.Info("User updated successfully", zap.String("user_id", user.ID))
	return user, nil
}

func (s *ProfileService) DeleteUser(ctx context.Context, userID string) error {
	s.logger.Info("Deleting user via auth service", zap.String("user_id", userID))

	// Delete user via auth service
	err := s.authClient.DeleteUser(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to delete user via auth service",
			zap.String("user_id", userID),
			zap.Error(err))
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Delete associated profile (only if storage client is available)
	if s.storageClient != nil {
		if err := s.DeleteProfile(ctx, userID); err != nil {
			s.logger.Warn("Failed to delete associated profile",
				zap.String("user_id", userID),
				zap.Error(err))
			// Don't fail user deletion if profile deletion fails
		}
	} else {
		s.logger.Debug("Storage client not available, skipping profile deletion",
			zap.String("user_id", userID))
	}

	s.logger.Info("User deleted successfully", zap.String("user_id", userID))
	return nil
}
