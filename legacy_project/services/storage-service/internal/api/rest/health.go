package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	db *sqlx.DB
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *sqlx.DB) *HealthHandler {
	return &HealthHandler{
		db: db,
	}
}

// RegisterRoutes registers the health check routes
func (h *HealthHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/health", h.handleHealth).Methods("GET")
}

// handleHealth handles health check requests
func (h *HealthHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check database connection
	err := h.db.PingContext(ctx)
	status := "healthy"
	if err != nil {
		status = "unhealthy"
	}

	// Prepare response
	response := map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().UTC(),
		"database": map[string]interface{}{
			"status": status,
			"error":  err,
		},
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	if status == "unhealthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// Send response
	json.NewEncoder(w).Encode(response)
}
