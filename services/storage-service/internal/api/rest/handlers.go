package rest

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"microservices/services/profile-storage/internal/models"
	"microservices/services/profile-storage/internal/service"
)

// Handler handles HTTP requests for the profile storage service
type Handler struct {
	service *service.ProfileService
}

// NewHandler creates a new handler instance
func NewHandler(service *service.ProfileService) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all routes for the handler
func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/profiles", h.CreateProfile).Methods("POST")
	router.HandleFunc("/profiles/{id}", h.GetProfile).Methods("GET")
	router.HandleFunc("/profiles/{id}", h.UpdateProfile).Methods("PUT")
	router.HandleFunc("/profiles/{id}", h.DeleteProfile).Methods("DELETE")
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// CreateProfile handles profile creation requests
func (h *Handler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	var req models.ProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	profile, err := h.service.CreateProfile(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(profile)
}

// GetProfile handles profile retrieval requests
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid profile ID", http.StatusBadRequest)
		return
	}

	profile, err := h.service.GetProfile(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if profile == nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(profile)
}

// UpdateProfile handles profile update requests
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid profile ID", http.StatusBadRequest)
		return
	}

	var req models.ProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	profile, err := h.service.UpdateProfile(r.Context(), id, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(profile)
}

// DeleteProfile handles profile deletion requests
func (h *Handler) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid profile ID", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteProfile(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
