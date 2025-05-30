package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"microservices/services/profile-storage/internal/logger"
	"microservices/services/profile-storage/internal/models"
	"microservices/services/profile-storage/internal/service"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string    `json:"error"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// ProfileHandler handles profile-related HTTP endpoints
type ProfileHandler struct {
	service *service.ProfileService
	log     *zap.Logger
}

// NewProfileHandler creates a new profile handler
func NewProfileHandler(service *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{
		service: service,
		log:     logger.Get(),
	}
}

// RegisterRoutes registers the profile routes
func (h *ProfileHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/profiles", h.listProfiles).Methods("GET")
	router.HandleFunc("/profiles", h.createProfile).Methods("POST")
	router.HandleFunc("/profiles/{id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := uuid.Parse(vars["id"])
		if err != nil {
			http.Error(w, "Invalid profile ID", http.StatusBadRequest)
			return
		}
		h.getProfile(w, r, id)
	}).Methods("GET")
	router.HandleFunc("/profiles/{id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := uuid.Parse(vars["id"])
		if err != nil {
			http.Error(w, "Invalid profile ID", http.StatusBadRequest)
			return
		}
		h.updateProfile(w, r, id)
	}).Methods("PUT")
	router.HandleFunc("/profiles/{id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id, err := uuid.Parse(vars["id"])
		if err != nil {
			http.Error(w, "Invalid profile ID", http.StatusBadRequest)
			return
		}
		h.deleteProfile(w, r, id)
	}).Methods("DELETE")
}

// listProfiles handles GET /profiles
func (h *ProfileHandler) listProfiles(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	h.log.Info("Listing profiles",
		logger.String("path", r.URL.Path),
	)

	// Use default pagination values for now
	page := 1
	pageSize := 10

	profiles, err := h.service.ListProfiles(r.Context(), page, pageSize)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTimeout):
			h.log.Error("Profile listing timed out",
				logger.ErrorField(err),
			)
			h.sendError(w, http.StatusGatewayTimeout, "Operation timed out", err)
		default:
			h.log.Error("Failed to list profiles",
				logger.ErrorField(err),
			)
			h.sendError(w, http.StatusInternalServerError, "Internal server error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(profiles)

	h.log.Info("Successfully listed profiles",
		logger.Int("count", len(profiles)),
		logger.Duration("duration", time.Since(startTime)),
	)
}

// createProfile handles POST /api/v1/profiles
func (h *ProfileHandler) createProfile(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	h.log.Info("Creating new profile",
		logger.String("path", r.URL.Path),
		logger.String("content_length", r.Header.Get("Content-Length")),
		logger.String("content_type", r.Header.Get("Content-Type")),
	)

	// Validate content type
	if r.Header.Get("Content-Type") != "application/json" {
		h.log.Error("Invalid content type",
			logger.String("content_type", r.Header.Get("Content-Type")),
		)
		h.sendError(w, http.StatusBadRequest, "Content-Type must be application/json", nil)
		return
	}

	// Validate content length
	if r.ContentLength == 0 {
		h.log.Error("Empty request body")
		h.sendError(w, http.StatusBadRequest, "Empty request body", nil)
		return
	}

	// Read and buffer request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Error("Failed to read request body",
			logger.ErrorField(err),
		)
		h.sendError(w, http.StatusBadRequest, "Failed to read request body", err)
		return
	}
	defer r.Body.Close()

	// Validate body size (e.g., 1MB limit)
	const maxBodySize = 1 << 20 // 1MB
	if len(body) > maxBodySize {
		h.log.Error("Request body too large",
			logger.Int("size", len(body)),
			logger.Int("max_size", maxBodySize),
		)
		h.sendError(w, http.StatusRequestEntityTooLarge, "Request body too large", nil)
		return
	}

	// Parse request body
	var req models.ProfileRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.log.Error("Failed to decode request body",
			logger.ErrorField(err),
			logger.String("body", string(body)),
		)
		h.sendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Add request correlation ID
	correlationID := uuid.New().String()
	ctx := context.WithValue(r.Context(), "correlation_id", correlationID)

	profile, err := h.service.CreateProfile(ctx, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest):
			h.log.Error("Invalid profile request",
				logger.ErrorField(err),
				logger.String("email", req.Email),
				logger.String("correlation_id", correlationID),
			)
			h.sendError(w, http.StatusBadRequest, "Invalid request", err)
		case errors.Is(err, service.ErrDuplicateEmail):
			h.log.Error("Email already in use",
				logger.ErrorField(err),
				logger.String("email", req.Email),
				logger.String("correlation_id", correlationID),
			)
			h.sendError(w, http.StatusConflict, "Email already in use", err)
		case errors.Is(err, service.ErrTimeout):
			h.log.Error("Profile creation timed out",
				logger.ErrorField(err),
				logger.String("email", req.Email),
				logger.String("correlation_id", correlationID),
			)
			h.sendError(w, http.StatusGatewayTimeout, "Operation timed out", err)
		default:
			h.log.Error("Failed to create profile",
				logger.ErrorField(err),
				logger.String("email", req.Email),
				logger.String("correlation_id", correlationID),
			)
			h.sendError(w, http.StatusInternalServerError, "Internal server error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(profile)

	h.log.Info("Successfully created profile",
		logger.String("profile_id", profile.ID.String()),
		logger.String("email", profile.Email),
		logger.Duration("duration", time.Since(startTime)),
		logger.String("correlation_id", correlationID),
	)
}

// getProfile handles GET /api/v1/profiles/{id}
func (h *ProfileHandler) getProfile(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	startTime := time.Now()
	h.log.Debug("Getting profile",
		logger.String("profile_id", id.String()),
	)

	profile, err := h.service.GetProfile(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProfileNotFound):
			h.log.Debug("Profile not found",
				logger.String("profile_id", id.String()),
			)
			h.sendError(w, http.StatusNotFound, "Profile not found", err)
		case errors.Is(err, service.ErrTimeout):
			h.log.Error("Profile retrieval timed out",
				logger.ErrorField(err),
				logger.String("profile_id", id.String()),
			)
			h.sendError(w, http.StatusGatewayTimeout, "Operation timed out", err)
		default:
			h.log.Error("Failed to get profile",
				logger.ErrorField(err),
				logger.String("profile_id", id.String()),
			)
			h.sendError(w, http.StatusInternalServerError, "Internal server error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(profile)

	h.log.Debug("Successfully retrieved profile",
		logger.String("profile_id", id.String()),
		logger.String("email", profile.Email),
		logger.Duration("duration", time.Since(startTime)),
	)
}

// updateProfile handles PUT /api/v1/profiles/{id}
func (h *ProfileHandler) updateProfile(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	startTime := time.Now()
	h.log.Info("Updating profile",
		logger.String("profile_id", id.String()),
	)

	var req models.ProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Failed to decode request body",
			logger.ErrorField(err),
			logger.String("profile_id", id.String()),
		)
		h.sendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	profile, err := h.service.UpdateProfile(r.Context(), id, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRequest):
			h.log.Error("Invalid profile update request",
				logger.ErrorField(err),
				logger.String("profile_id", id.String()),
				logger.String("email", req.Email),
			)
			h.sendError(w, http.StatusBadRequest, "Invalid request", err)
		case errors.Is(err, service.ErrProfileNotFound):
			h.log.Debug("Profile not found for update",
				logger.String("profile_id", id.String()),
			)
			h.sendError(w, http.StatusNotFound, "Profile not found", err)
		case errors.Is(err, service.ErrDuplicateEmail):
			h.log.Error("Email already in use",
				logger.ErrorField(err),
				logger.String("profile_id", id.String()),
				logger.String("email", req.Email),
			)
			h.sendError(w, http.StatusConflict, "Email already in use", err)
		case errors.Is(err, service.ErrTimeout):
			h.log.Error("Profile update timed out",
				logger.ErrorField(err),
				logger.String("profile_id", id.String()),
				logger.String("email", req.Email),
			)
			h.sendError(w, http.StatusGatewayTimeout, "Operation timed out", err)
		default:
			h.log.Error("Failed to update profile",
				logger.ErrorField(err),
				logger.String("profile_id", id.String()),
				logger.String("email", req.Email),
			)
			h.sendError(w, http.StatusInternalServerError, "Internal server error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(profile)

	h.log.Info("Successfully updated profile",
		logger.String("profile_id", id.String()),
		logger.String("email", profile.Email),
		logger.Duration("duration", time.Since(startTime)),
	)
}

// deleteProfile handles DELETE /api/v1/profiles/{id}
func (h *ProfileHandler) deleteProfile(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	startTime := time.Now()
	h.log.Info("Deleting profile",
		logger.String("profile_id", id.String()),
	)

	err := h.service.DeleteProfile(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProfileNotFound):
			h.log.Debug("Profile not found for deletion",
				logger.String("profile_id", id.String()),
			)
			h.sendError(w, http.StatusNotFound, "Profile not found", err)
		case errors.Is(err, service.ErrTimeout):
			h.log.Error("Profile deletion timed out",
				logger.ErrorField(err),
				logger.String("profile_id", id.String()),
			)
			h.sendError(w, http.StatusGatewayTimeout, "Operation timed out", err)
		default:
			h.log.Error("Failed to delete profile",
				logger.ErrorField(err),
				logger.String("profile_id", id.String()),
			)
			h.sendError(w, http.StatusInternalServerError, "Internal server error", err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)

	h.log.Info("Successfully deleted profile",
		logger.String("profile_id", id.String()),
		logger.Duration("duration", time.Since(startTime)),
	)
}

// sendError sends an error response
func (h *ProfileHandler) sendError(w http.ResponseWriter, status int, message string, err error) {
	response := ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Time:    time.Now().UTC(),
	}

	if err != nil {
		response.Message = fmt.Sprintf("%s: %v", message, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
