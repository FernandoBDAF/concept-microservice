package rest

import (
	"encoding/json"
	"net/http"

	"microservices/services/profile-storage/internal/database"

	"github.com/gorilla/mux"
)

// MetricsHandler handles metrics endpoints
type MetricsHandler struct {
	connManager *database.ConnectionManager
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(connManager *database.ConnectionManager) *MetricsHandler {
	return &MetricsHandler{
		connManager: connManager,
	}
}

// RegisterRoutes registers the metrics routes
func (h *MetricsHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/metrics/pool", h.handlePoolMetrics).Methods("GET")
}

// handlePoolMetrics returns the current pool metrics
func (h *MetricsHandler) handlePoolMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics := h.connManager.GetMetrics()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}
