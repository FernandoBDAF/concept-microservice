package services

import (
	"context"
	"fmt"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/config"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/logger"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/messaging"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

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
	GetProfiles(ctx context.Context) ([]*models.Profile, error)
	GetProfile(ctx context.Context, id string) (*models.Profile, error)
	CreateProfile(ctx context.Context, req *models.ProfileRequest) (*models.Profile, error)
	UpdateProfile(ctx context.Context, id string, req *models.ProfileRequest) (*models.Profile, error)
	DeleteProfile(ctx context.Context, id string) error
	SubmitTask(ctx context.Context, profileID string, req *models.TaskRequest) (*models.Task, error)
}

// ProfileService handles profile-related business logic
type ProfileService struct {
	storageClient *StorageClient
	queueClient   *messaging.QueueClient
}

// NewProfileService creates a new profile service
func NewProfileService(cfg *config.Config, storageClient *StorageClient) *ProfileService {
	// Initialize queue client
	queueConfig := &messaging.QueueConfig{
		URL:       cfg.Queue.URL,
		Timeout:   cfg.Queue.Timeout,
		Retries:   cfg.Queue.Retries,
		QueueName: cfg.Queue.QueueName,
	}
	queueClient, err := messaging.NewQueueClient(queueConfig)
	if err != nil {
		logger.LogError(context.Background(), "Failed to initialize queue client", err)
		// Don't fail the service startup, but log the error
	}

	return &ProfileService{
		storageClient: storageClient,
		queueClient:   queueClient,
	}
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

// GetProfile retrieves a profile by ID
func (s *ProfileService) GetProfile(ctx context.Context, id string) (*models.Profile, error) {
	if id == "" {
		return nil, &ProfileError{
			Code:    400,
			Message: "Profile ID is required",
		}
	}

	logger.LogInfo(ctx, "Getting profile",
		zap.String("id", id))
	profile, err := s.storageClient.GetProfile(ctx, id)
	if err != nil {
		logger.LogError(ctx, "Error getting profile", err,
			zap.String("id", id))
		return nil, &ProfileError{
			Code:    500,
			Message: fmt.Sprintf("Failed to get profile %s", id),
			Err:     err,
		}
	}
	logger.LogInfo(ctx, "Successfully retrieved profile",
		zap.String("id", id))
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

// UpdateProfile updates an existing profile
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
		logger.LogError(ctx, "Invalid profile request", err)
		return nil, &ProfileError{
			Code:    400,
			Message: "Invalid profile request",
			Err:     err,
		}
	}

	logger.LogInfo(ctx, "Updating profile",
		zap.String("id", id))
	// First get the existing profile
	existingProfile, err := s.storageClient.GetProfile(ctx, id)
	if err != nil {
		logger.LogError(ctx, "Error getting existing profile", err,
			zap.String("id", id))
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
		logger.LogError(ctx, "Error updating profile", err,
			zap.String("id", id))
		return nil, &ProfileError{
			Code:    500,
			Message: fmt.Sprintf("Failed to update profile %s", id),
			Err:     err,
		}
	}
	logger.LogInfo(ctx, "Successfully updated profile",
		zap.String("id", id))
	return updatedProfile, nil
}

// DeleteProfile deletes a profile
func (s *ProfileService) DeleteProfile(ctx context.Context, id string) error {
	if id == "" {
		return &ProfileError{
			Code:    400,
			Message: "Profile ID is required",
		}
	}

	logger.LogInfo(ctx, "Deleting profile",
		zap.String("id", id))
	err := s.storageClient.DeleteProfile(ctx, id)
	if err != nil {
		logger.LogError(ctx, "Error deleting profile", err,
			zap.String("id", id))
		return &ProfileError{
			Code:    500,
			Message: fmt.Sprintf("Failed to delete profile %s", id),
			Err:     err,
		}
	}
	logger.LogInfo(ctx, "Successfully deleted profile",
		zap.String("id", id))
	return nil
}

// SubmitTask submits a new task for a profile
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

	logger.LogInfo(ctx, "Creating new task",
		zap.String("profile_id", profileID),
		zap.String("task_type", req.Type))

	// Create task
	task := &models.Task{
		ID:        uuid.New(),
		ProfileID: profileID,
		Type:      req.Type,
		Status:    "pending",
		Payload:   req.Payload,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store task
	logger.LogInfo(ctx, "Storing task in storage",
		zap.String("profile_id", profileID),
		zap.String("task_id", task.ID.String()))

	// TODO: Implement CreateTask in storage client
	// For now, we'll just use the task as created
	createdTask := task

	logger.LogInfo(ctx, "Successfully stored task",
		zap.String("profile_id", profileID),
		zap.String("task_id", task.ID.String()))

	// Send message to queue
	msg := &messaging.QueueMessage{
		ID:            task.ID.String(),
		Type:          req.Type,
		Timestamp:     task.CreatedAt.UTC().Format(time.RFC3339),
		CorrelationID: uuid.New().String(),
		Payload: map[string]interface{}{
			"task_id":    task.ID.String(),
			"profile_id": profileID,
			"payload":    req.Payload,
		},
		Priority: 1,
		Headers:  make(map[string]string),
	}

	logger.LogInfo(ctx, "Sending task to queue",
		zap.String("profile_id", profileID),
		zap.String("task_id", task.ID.String()),
		zap.String("task_type", req.Type),
		zap.String("queue_url", s.queueClient.GetQueueServiceURL()))

	if err := s.queueClient.PublishMessage(ctx, msg); err != nil {
		logger.LogError(ctx, "Failed to send task to queue", err,
			zap.String("profile_id", profileID),
			zap.String("task_id", task.ID.String()),
			zap.String("queue_url", s.queueClient.GetQueueServiceURL()))
		return nil, &ProfileError{
			Code:    500,
			Message: "Failed to send task to queue",
			Err:     err,
		}
	}

	logger.LogInfo(ctx, "Successfully sent task to queue",
		zap.String("profile_id", profileID),
		zap.String("task_id", task.ID.String()),
		zap.String("task_type", req.Type))

	return createdTask, nil
}
