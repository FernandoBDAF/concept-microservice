package handlers

import (
	"net/http"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/services"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

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

// SubmitTask handles POST /api/v1/profiles/:id/tasks
func (h *TaskHandler) SubmitTask(c *gin.Context) {
	id := c.Param("id")
	logger.LogInfo(c.Request.Context(), "Received task submission request",
		zap.String("profile_id", id))

	var req models.TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.LogError(c.Request.Context(), "Invalid task request", err,
			zap.String("profile_id", id))
		c.JSON(http.StatusBadRequest, models.TaskResponse{
			Error: err.Error(),
		})
		return
	}

	logger.LogInfo(c.Request.Context(), "Processing task submission",
		zap.String("profile_id", id),
		zap.String("task_type", req.Type))

	task, err := h.profileService.SubmitTask(c.Request.Context(), id, &req)
	if err != nil {
		logger.LogError(c.Request.Context(), "Failed to submit task", err,
			zap.String("profile_id", id),
			zap.String("task_type", req.Type))
		c.JSON(http.StatusInternalServerError, models.TaskResponse{
			Error: err.Error(),
		})
		return
	}

	logger.LogInfo(c.Request.Context(), "Successfully submitted task",
		zap.String("profile_id", id),
		zap.String("task_id", task.ID.String()),
		zap.String("task_type", req.Type))

	c.JSON(http.StatusAccepted, models.TaskResponse{
		Task: task,
	})
}
