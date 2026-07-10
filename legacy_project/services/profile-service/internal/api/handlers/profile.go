package handlers

import (
	"net/http"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/services"
	"github.com/gin-gonic/gin"
)

// ProfileHandler handles profile-related HTTP requests
type ProfileHandler struct {
	profileService services.ProfileServiceInterface
}

type ProfileError struct {
	Error string `json:"error"`
}

// NewProfileHandler creates a new profile handler
func NewProfileHandler(profileService *services.ProfileService) *ProfileHandler {
	return &ProfileHandler{
		profileService: profileService,
	}
}

// NewProfileHandlerWithInterface creates a new profile handler with an interface (for testing)
func NewProfileHandlerWithInterface(profileService services.ProfileServiceInterface) *ProfileHandler {
	return &ProfileHandler{
		profileService: profileService,
	}
}

// GetProfiles handles GET /api/v1/profiles
func (h *ProfileHandler) GetProfiles(c *gin.Context) {
	// TODO: Implement pagination and filtering
	profiles, err := h.profileService.GetProfiles(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ProfileResponse{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, profiles)
}

// GetProfile handles GET /api/v1/profiles/:id
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	id := c.Param("id")
	profile, err := h.profileService.GetProfile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ProfileResponse{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.ProfileResponse{
		Profile: profile,
	})
}

// CreateProfile handles POST /api/v1/profiles
func (h *ProfileHandler) CreateProfile(c *gin.Context) {
	var req models.ProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ProfileResponse{
			Error: err.Error(),
		})
		return
	}

	profile, err := h.profileService.CreateProfile(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ProfileResponse{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, models.ProfileResponse{
		Profile: profile,
	})
}

// UpdateProfile handles PUT /api/v1/profiles/:id
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	id := c.Param("id")
	var req models.ProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ProfileResponse{
			Error: err.Error(),
		})
		return
	}

	profile, err := h.profileService.UpdateProfile(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ProfileResponse{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.ProfileResponse{
		Profile: profile,
	})
}

// DeleteProfile handles DELETE /api/v1/profiles/:id
func (h *ProfileHandler) DeleteProfile(c *gin.Context) {
	id := c.Param("id")
	err := h.profileService.DeleteProfile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ProfileResponse{
			Error: err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}
