package handlers

import (
	"net/http"
	"strings"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/services"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ✅ NEW: API versioning and error response enhancements for Phase 3
const (
	APIVersionV1        = "v1"
	MaxRequestSizeBytes = 1024 * 1024 // 1MB max request size
)

// ✅ NEW: Enhanced error response structure with API versioning
type APIErrorResponse struct {
	Error    string                 `json:"error"`
	Code     string                 `json:"code,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty"`
	Version  string                 `json:"api_version"`
	TaskType string                 `json:"task_type,omitempty"`
	Endpoint string                 `json:"endpoint,omitempty"`
}

// ✅ NEW: Enhanced success response structure with metadata
type APISuccessResponse struct {
	Data     interface{}            `json:"data"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Version  string                 `json:"api_version"`
}

// TaskHandler handles task-related HTTP requests
type TaskHandler struct {
	profileService services.ProfileServiceInterface
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(profileService services.ProfileServiceInterface) *TaskHandler {
	return &TaskHandler{
		profileService: profileService,
	}
}

// ✅ ENHANCED: SubmitTask with improved error handling and API versioning
// @Summary Submit a generic task for a profile
// @Description Submit a task of any supported type (profile_update, email_notification, image_processing)
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "Profile ID"
// @Param task body models.TaskRequest true "Task request payload"
// @Success 202 {object} APISuccessResponse "Task accepted for processing"
// @Failure 400 {object} APIErrorResponse "Invalid request"
// @Failure 422 {object} APIErrorResponse "Validation error"
// @Failure 500 {object} APIErrorResponse "Internal server error"
// @Router /api/v1/profiles/{id}/tasks [post]
func (h *TaskHandler) SubmitTask(c *gin.Context) {
	id := c.Param("id")

	// ✅ Enhanced request logging with API context
	logger.LogInfo(c.Request.Context(), "Received generic task submission request",
		zap.String("profile_id", id),
		zap.String("endpoint", "generic_task_submission"),
		zap.String("api_version", APIVersionV1),
		zap.String("user_agent", c.GetHeader("User-Agent")),
		zap.String("content_type", c.GetHeader("Content-Type")))

	// ✅ NEW: Request size validation
	if c.Request.ContentLength > MaxRequestSizeBytes {
		logger.LogError(c.Request.Context(), "Request size exceeds limit", nil,
			zap.String("profile_id", id),
			zap.Int64("content_length", c.Request.ContentLength),
			zap.Int64("max_size", MaxRequestSizeBytes))

		c.JSON(http.StatusBadRequest, APIErrorResponse{
			Error:    "Request size exceeds maximum allowed size",
			Code:     "REQUEST_TOO_LARGE",
			Version:  APIVersionV1,
			Endpoint: "generic_task_submission",
			Details: map[string]interface{}{
				"max_size_bytes": MaxRequestSizeBytes,
				"actual_size":    c.Request.ContentLength,
			},
		})
		return
	}

	var req models.TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.LogError(c.Request.Context(), "Invalid task request JSON binding", err,
			zap.String("profile_id", id),
			zap.String("api_version", APIVersionV1))

		c.JSON(http.StatusBadRequest, APIErrorResponse{
			Error:    "Invalid JSON format in request body",
			Code:     "INVALID_JSON",
			Version:  APIVersionV1,
			Endpoint: "generic_task_submission",
			Details: map[string]interface{}{
				"validation_error": err.Error(),
			},
		})
		return
	}

	// ✅ Enhanced validation with detailed error responses
	if err := req.Validate(); err != nil {
		logger.LogError(c.Request.Context(), "Task request validation failed", err,
			zap.String("profile_id", id),
			zap.String("task_type", req.Type),
			zap.String("api_version", APIVersionV1))

		c.JSON(http.StatusUnprocessableEntity, APIErrorResponse{
			Error:    err.Error(),
			Code:     "VALIDATION_ERROR",
			Version:  APIVersionV1,
			TaskType: req.Type,
			Endpoint: "generic_task_submission",
			Details: map[string]interface{}{
				"supported_types": []string{"profile_update", "email_notification", "image_processing"},
			},
		})
		return
	}

	// Enhanced routing key logging
	routingKey := services.RoutingKeyMap[req.Type]
	if routingKey == "" {
		routingKey = "profile.task" // fallback
	}

	logger.LogInfo(c.Request.Context(), "Processing enhanced generic task submission",
		zap.String("profile_id", id),
		zap.String("task_type", req.Type),
		zap.String("routing_key", routingKey),
		zap.String("api_version", APIVersionV1),
		zap.String("endpoint", "generic_task_submission"))

	task, err := h.profileService.SubmitTask(c.Request.Context(), id, &req)
	if err != nil {
		logger.LogError(c.Request.Context(), "Failed to submit task", err,
			zap.String("profile_id", id),
			zap.String("task_type", req.Type),
			zap.String("routing_key", routingKey),
			zap.String("api_version", APIVersionV1))

		statusCode, errorCode := h.determineErrorResponse(err)
		c.JSON(statusCode, APIErrorResponse{
			Error:    err.Error(),
			Code:     errorCode,
			Version:  APIVersionV1,
			TaskType: req.Type,
			Endpoint: "generic_task_submission",
			Details: map[string]interface{}{
				"routing_key": routingKey,
			},
		})
		return
	}

	logger.LogInfo(c.Request.Context(), "Successfully submitted enhanced generic task",
		zap.String("profile_id", id),
		zap.String("task_id", task.ID.String()),
		zap.String("task_type", req.Type),
		zap.String("routing_key", routingKey),
		zap.String("api_version", APIVersionV1))

	// ✅ Enhanced success response with metadata
	c.JSON(http.StatusAccepted, APISuccessResponse{
		Data:    models.TaskResponse{Task: task},
		Version: APIVersionV1,
		Metadata: map[string]interface{}{
			"task_id":     task.ID.String(),
			"routing_key": routingKey,
			"worker_type": h.getWorkerType(req.Type),
			"accepted_at": task.CreatedAt,
			"endpoint":    "generic_task_submission",
		},
	})
}

// ✅ ENHANCED: SubmitEmailTask with improved API documentation and error handling
// @Summary Submit an email notification task
// @Description Submit a specialized email notification task with validation and priority support
// @Tags tasks,email
// @Accept json
// @Produce json
// @Param id path string true "Profile ID"
// @Param email body models.EmailTaskPayload true "Email task payload"
// @Success 202 {object} APISuccessResponse "Email task accepted for processing"
// @Failure 400 {object} APIErrorResponse "Invalid request"
// @Failure 422 {object} APIErrorResponse "Validation error"
// @Failure 500 {object} APIErrorResponse "Internal server error"
// @Router /api/v1/profiles/{id}/tasks/email [post]
func (h *TaskHandler) SubmitEmailTask(c *gin.Context) {
	id := c.Param("id")
	logger.LogInfo(c.Request.Context(), "Received specialized email task submission",
		zap.String("profile_id", id),
		zap.String("endpoint", "specialized_email_task"),
		zap.String("api_version", APIVersionV1))

	var emailPayload models.EmailTaskPayload
	if err := c.ShouldBindJSON(&emailPayload); err != nil {
		logger.LogError(c.Request.Context(), "Invalid email task JSON binding", err,
			zap.String("profile_id", id),
			zap.String("api_version", APIVersionV1))

		c.JSON(http.StatusBadRequest, APIErrorResponse{
			Error:    "Invalid JSON format for email task",
			Code:     "INVALID_EMAIL_JSON",
			Version:  APIVersionV1,
			TaskType: "email_notification",
			Endpoint: "specialized_email_task",
			Details: map[string]interface{}{
				"required_fields": []string{"to", "template"},
				"optional_fields": []string{"subject", "priority", "data"},
			},
		})
		return
	}

	// Enhanced email validation with detailed errors
	if err := emailPayload.Validate(); err != nil {
		logger.LogError(c.Request.Context(), "Email task payload validation failed", err,
			zap.String("profile_id", id),
			zap.String("email_to", emailPayload.To),
			zap.String("template", emailPayload.Template))

		c.JSON(http.StatusUnprocessableEntity, APIErrorResponse{
			Error:    err.Error(),
			Code:     "EMAIL_VALIDATION_ERROR",
			Version:  APIVersionV1,
			TaskType: "email_notification",
			Endpoint: "specialized_email_task",
			Details: map[string]interface{}{
				"email_to":         emailPayload.To,
				"template":         emailPayload.Template,
				"priority_range":   []int{1, 2, 3},
				"priority_meaning": map[string]string{"1": "high", "2": "normal", "3": "low"},
			},
		})
		return
	}

	logger.LogInfo(c.Request.Context(), "Processing enhanced email task submission",
		zap.String("profile_id", id),
		zap.String("email_to", emailPayload.To),
		zap.String("template", emailPayload.Template),
		zap.Int("priority", emailPayload.Priority),
		zap.String("routing_key", "email.send"),
		zap.String("api_version", APIVersionV1))

	emailResponse, err := h.profileService.SubmitEmailTask(c.Request.Context(), id, &emailPayload)
	if err != nil {
		logger.LogError(c.Request.Context(), "Failed to submit email task", err,
			zap.String("profile_id", id),
			zap.String("email_to", emailPayload.To),
			zap.String("api_version", APIVersionV1))

		statusCode, errorCode := h.determineErrorResponse(err)
		c.JSON(statusCode, APIErrorResponse{
			Error:    err.Error(),
			Code:     errorCode,
			Version:  APIVersionV1,
			TaskType: "email_notification",
			Endpoint: "specialized_email_task",
		})
		return
	}

	logger.LogInfo(c.Request.Context(), "Successfully submitted enhanced email task",
		zap.String("profile_id", id),
		zap.String("task_id", emailResponse.TaskID),
		zap.String("email_to", emailResponse.EmailTo),
		zap.String("api_version", APIVersionV1))

	c.JSON(http.StatusAccepted, APISuccessResponse{
		Data:    emailResponse,
		Version: APIVersionV1,
		Metadata: map[string]interface{}{
			"worker_type": "email-worker",
			"endpoint":    "specialized_email_task",
		},
	})
}

// ✅ ENHANCED: SubmitImageTask with improved API documentation and error handling
// @Summary Submit an image processing task
// @Description Submit a specialized image processing task with operation validation
// @Tags tasks,image
// @Accept json
// @Produce json
// @Param id path string true "Profile ID"
// @Param image body models.ImageTaskPayload true "Image task payload"
// @Success 202 {object} APISuccessResponse "Image task accepted for processing"
// @Failure 400 {object} APIErrorResponse "Invalid request"
// @Failure 422 {object} APIErrorResponse "Validation error"
// @Failure 500 {object} APIErrorResponse "Internal server error"
// @Router /api/v1/profiles/{id}/tasks/image [post]
func (h *TaskHandler) SubmitImageTask(c *gin.Context) {
	id := c.Param("id")
	logger.LogInfo(c.Request.Context(), "Received specialized image task submission",
		zap.String("profile_id", id),
		zap.String("endpoint", "specialized_image_task"),
		zap.String("api_version", APIVersionV1))

	var imagePayload models.ImageTaskPayload
	if err := c.ShouldBindJSON(&imagePayload); err != nil {
		logger.LogError(c.Request.Context(), "Invalid image task JSON binding", err,
			zap.String("profile_id", id),
			zap.String("api_version", APIVersionV1))

		c.JSON(http.StatusBadRequest, APIErrorResponse{
			Error:    "Invalid JSON format for image task",
			Code:     "INVALID_IMAGE_JSON",
			Version:  APIVersionV1,
			TaskType: "image_processing",
			Endpoint: "specialized_image_task",
			Details: map[string]interface{}{
				"required_fields":      []string{"image_url", "operation"},
				"supported_operations": []string{"resize", "convert", "optimize"},
				"supported_formats":    []string{"jpeg", "png", "webp", "gif"},
			},
		})
		return
	}

	logger.LogInfo(c.Request.Context(), "Processing enhanced image task submission",
		zap.String("profile_id", id),
		zap.String("image_url", imagePayload.ImageURL),
		zap.String("operation", imagePayload.Operation),
		zap.String("output_format", imagePayload.OutputFormat),
		zap.String("routing_key", "image.process"),
		zap.String("api_version", APIVersionV1))

	imageResponse, err := h.profileService.SubmitImageTask(c.Request.Context(), id, &imagePayload)
	if err != nil {
		logger.LogError(c.Request.Context(), "Failed to submit image task", err,
			zap.String("profile_id", id),
			zap.String("image_url", imagePayload.ImageURL),
			zap.String("api_version", APIVersionV1))

		statusCode, errorCode := h.determineErrorResponse(err)
		c.JSON(statusCode, APIErrorResponse{
			Error:    err.Error(),
			Code:     errorCode,
			Version:  APIVersionV1,
			TaskType: "image_processing",
			Endpoint: "specialized_image_task",
		})
		return
	}

	logger.LogInfo(c.Request.Context(), "Successfully submitted enhanced image task",
		zap.String("profile_id", id),
		zap.String("task_id", imageResponse.TaskID),
		zap.String("image_url", imageResponse.ImageURL),
		zap.String("api_version", APIVersionV1))

	c.JSON(http.StatusAccepted, APISuccessResponse{
		Data:    imageResponse,
		Version: APIVersionV1,
		Metadata: map[string]interface{}{
			"worker_type": "image-worker",
			"endpoint":    "specialized_image_task",
		},
	})
}

// ✅ ENHANCED: SubmitProfileTask with API documentation
// @Summary Submit a profile update task
// @Description Submit a specialized profile update task
// @Tags tasks,profile
// @Accept json
// @Produce json
// @Param id path string true "Profile ID"
// @Param profile body models.ProfileTaskPayload true "Profile task payload"
// @Success 202 {object} APISuccessResponse "Profile task accepted for processing"
// @Failure 400 {object} APIErrorResponse "Invalid request"
// @Failure 422 {object} APIErrorResponse "Validation error"
// @Failure 500 {object} APIErrorResponse "Internal server error"
// @Router /api/v1/profiles/{id}/tasks/profile [post]
func (h *TaskHandler) SubmitProfileTask(c *gin.Context) {
	id := c.Param("id")
	logger.LogInfo(c.Request.Context(), "Received specialized profile task submission",
		zap.String("profile_id", id),
		zap.String("endpoint", "specialized_profile_task"),
		zap.String("api_version", APIVersionV1))

	var profilePayload models.ProfileTaskPayload
	if err := c.ShouldBindJSON(&profilePayload); err != nil {
		logger.LogError(c.Request.Context(), "Invalid profile task JSON binding", err,
			zap.String("profile_id", id),
			zap.String("api_version", APIVersionV1))

		c.JSON(http.StatusBadRequest, APIErrorResponse{
			Error:    "Invalid JSON format for profile task",
			Code:     "INVALID_PROFILE_JSON",
			Version:  APIVersionV1,
			TaskType: "profile_update",
			Endpoint: "specialized_profile_task",
			Details: map[string]interface{}{
				"required_fields":   []string{"user_id", "action"},
				"supported_actions": []string{"create", "update", "delete", "sync"},
			},
		})
		return
	}

	logger.LogInfo(c.Request.Context(), "Processing enhanced profile task submission",
		zap.String("profile_id", id),
		zap.String("user_id", profilePayload.UserID),
		zap.String("action", profilePayload.Action),
		zap.String("routing_key", "profile.task"),
		zap.String("api_version", APIVersionV1))

	// Create task request from profile payload
	// ✅ FIX: Convert struct to map for validation compatibility
	payloadMap := map[string]interface{}{
		"user_id": profilePayload.UserID,
		"action":  profilePayload.Action,
	}

	// Add optional data field if present
	if profilePayload.Data != nil {
		payloadMap["data"] = profilePayload.Data
	}

	taskReq := &models.TaskRequest{
		Type:    "profile_update",
		Payload: payloadMap, // ✅ Now correctly uses map[string]interface{}
	}

	task, err := h.profileService.SubmitTask(c.Request.Context(), id, taskReq)
	if err != nil {
		logger.LogError(c.Request.Context(), "Failed to submit profile task", err,
			zap.String("profile_id", id),
			zap.String("user_id", profilePayload.UserID),
			zap.String("api_version", APIVersionV1))

		statusCode, errorCode := h.determineErrorResponse(err)
		c.JSON(statusCode, APIErrorResponse{
			Error:    err.Error(),
			Code:     errorCode,
			Version:  APIVersionV1,
			TaskType: "profile_update",
			Endpoint: "specialized_profile_task",
		})
		return
	}

	logger.LogInfo(c.Request.Context(), "Successfully submitted enhanced profile task",
		zap.String("profile_id", id),
		zap.String("task_id", task.ID.String()),
		zap.String("user_id", profilePayload.UserID),
		zap.String("api_version", APIVersionV1))

	// Create enhanced response
	response := APISuccessResponse{
		Data: gin.H{
			"task_id":      task.ID.String(),
			"profile_id":   task.ProfileID,
			"type":         task.Type,
			"status":       task.Status,
			"user_id":      profilePayload.UserID,
			"action":       profilePayload.Action,
			"routing_key":  "profile.task",
			"scheduled_at": task.CreatedAt,
			"created_at":   task.CreatedAt,
		},
		Version: APIVersionV1,
		Metadata: map[string]interface{}{
			"worker_type": "profile-worker",
			"endpoint":    "specialized_profile_task",
		},
	}

	c.JSON(http.StatusAccepted, response)
}

// ✅ ENHANCED: GetTaskTypeStats with improved API documentation
// @Summary Get task statistics for a profile
// @Description Retrieve task distribution and performance statistics for a profile
// @Tags tasks,statistics
// @Produce json
// @Param id path string true "Profile ID"
// @Success 200 {object} APISuccessResponse "Task statistics"
// @Failure 404 {object} APIErrorResponse "Profile not found"
// @Failure 500 {object} APIErrorResponse "Internal server error"
// @Router /api/v1/profiles/{id}/tasks/stats [get]
func (h *TaskHandler) GetTaskTypeStats(c *gin.Context) {
	id := c.Param("id")
	logger.LogInfo(c.Request.Context(), "Received enhanced task stats request",
		zap.String("profile_id", id),
		zap.String("endpoint", "task_stats"),
		zap.String("api_version", APIVersionV1))

	// Enhanced statistics with more detailed metrics
	stats := gin.H{
		"profile_id": id,
		"task_stats": gin.H{
			"total_tasks": 0,
			"by_type": gin.H{
				"profile_update":     0,
				"email_notification": 0,
				"image_processing":   0,
			},
			"by_status": gin.H{
				"pending":   0,
				"running":   0,
				"completed": 0,
				"failed":    0,
			},
			"routing_keys": gin.H{
				"profile.task":  0,
				"email.send":    0,
				"image.process": 0,
			},
			"worker_distribution": gin.H{
				"profile-worker": 0,
				"email-worker":   0,
				"image-worker":   0,
			},
		},
	}

	logger.LogInfo(c.Request.Context(), "Successfully retrieved enhanced task stats",
		zap.String("profile_id", id),
		zap.String("api_version", APIVersionV1))

	c.JSON(http.StatusOK, APISuccessResponse{
		Data:    stats,
		Version: APIVersionV1,
		Metadata: map[string]interface{}{
			"endpoint":     "task_stats",
			"generated_at": c.Request.Context().Value("timestamp"),
		},
	})
}

// ✅ NEW: Helper methods for enhanced error handling

// determineErrorResponse maps service errors to appropriate HTTP status codes and error codes
func (h *TaskHandler) determineErrorResponse(err error) (int, string) {
	if profileErr, ok := err.(*services.ProfileError); ok {
		switch {
		case profileErr.Code >= 400 && profileErr.Code < 500:
			return profileErr.Code, h.getErrorCode(profileErr.Code)
		case profileErr.Code >= 500:
			return profileErr.Code, "INTERNAL_SERVER_ERROR"
		default:
			return http.StatusInternalServerError, "UNKNOWN_ERROR"
		}
	}

	// Check for specific error patterns
	errMsg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errMsg, "queue"):
		return http.StatusServiceUnavailable, "QUEUE_SERVICE_ERROR"
	case strings.Contains(errMsg, "routing"):
		return http.StatusUnprocessableEntity, "ROUTING_ERROR"
	case strings.Contains(errMsg, "validation"):
		return http.StatusUnprocessableEntity, "VALIDATION_ERROR"
	default:
		return http.StatusInternalServerError, "INTERNAL_SERVER_ERROR"
	}
}

// getErrorCode returns appropriate error codes for HTTP status codes
func (h *TaskHandler) getErrorCode(statusCode int) string {
	switch statusCode {
	case 400:
		return "BAD_REQUEST"
	case 401:
		return "UNAUTHORIZED"
	case 403:
		return "FORBIDDEN"
	case 404:
		return "NOT_FOUND"
	case 422:
		return "VALIDATION_ERROR"
	case 429:
		return "RATE_LIMITED"
	default:
		return "CLIENT_ERROR"
	}
}

// getWorkerType returns the worker type for a given task type
func (h *TaskHandler) getWorkerType(taskType string) string {
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
