package handlers

import (
	"net/http"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/services"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	profileService services.ProfileServiceInterface
	logger         *zap.Logger
}

// NewUserHandler creates a new user handler
func NewUserHandler(profileService services.ProfileServiceInterface) *UserHandler {
	return &UserHandler{
		profileService: profileService,
		logger:         zap.L().Named("user_handler"),
	}
}

// CreateUser handles POST /api/v1/users
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to decode create user request", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.UserResponse{
			Error: err.Error(),
		})
		return
	}

	if err := req.Validate(); err != nil {
		h.logger.Error("Invalid create user request", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.UserResponse{
			Error: err.Error(),
		})
		return
	}

	user, err := h.profileService.CreateUser(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to create user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.UserResponse{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, models.UserResponse{
		User: user,
	})
}

// GetUserByEmail handles GET /api/v1/users/email/{email}
func (h *UserHandler) GetUserByEmail(c *gin.Context) {
	email := c.Param("email")
	user, err := h.profileService.GetUserByEmail(c.Request.Context(), email)
	if err != nil {
		if err == models.ErrUserNotFound {
			c.JSON(http.StatusNotFound, models.UserResponse{
				Error: "User not found",
			})
			return
		}
		h.logger.Error("Failed to get user by email", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.UserResponse{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.UserResponse{
		User: user,
	})
}

// UpdateUser handles PUT /api/v1/users/{id}
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("id")
	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to decode update user request", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.UserResponse{
			Error: err.Error(),
		})
		return
	}

	user, err := h.profileService.UpdateUser(c.Request.Context(), userID, &req)
	if err != nil {
		if err == models.ErrUserNotFound {
			c.JSON(http.StatusNotFound, models.UserResponse{
				Error: "User not found",
			})
			return
		}
		h.logger.Error("Failed to update user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.UserResponse{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.UserResponse{
		User: user,
	})
}

// DeleteUser handles DELETE /api/v1/users/{id}
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	err := h.profileService.DeleteUser(c.Request.Context(), userID)
	if err != nil {
		if err == models.ErrUserNotFound {
			c.JSON(http.StatusNotFound, models.UserResponse{
				Error: "User not found",
			})
			return
		}
		h.logger.Error("Failed to delete user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.UserResponse{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User deleted successfully",
	})
}
