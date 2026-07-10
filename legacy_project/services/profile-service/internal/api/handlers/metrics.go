package handlers

import (
	"net/http"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/metrics"
	"github.com/gin-gonic/gin"
)

// MetricsHandler handles metrics-related requests
type MetricsHandler struct {
	metrics *metrics.StorageMetrics
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{
		metrics: metrics.GetMetrics(),
	}
}

// GetMetrics returns the current metrics
func (h *MetricsHandler) GetMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, h.metrics)
}

// ResetMetrics resets all metrics to zero
func (h *MetricsHandler) ResetMetrics(c *gin.Context) {
	metrics.Reset()
	c.Status(http.StatusNoContent)
}
