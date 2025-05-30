package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/FBDAF/microservices/services/common/metrics"
	"github.com/FBDAF/microservices/services/queue-service/internal/domain/model"
	"github.com/FBDAF/microservices/services/queue-service/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler implements the HTTP handlers for the queue service
type Handler struct {
	queueService *service.QueueService
	metrics      *metrics.QueueCollector
}

// NewHandler creates a new HTTP handler
func NewHandler(queueService *service.QueueService) *Handler {
	// Create metrics collector without auto-registration
	collector := &metrics.QueueCollector{
		// HTTP metrics
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "queue_http_requests_total",
				Help: "Total number of HTTP requests to the queue service",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "queue_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),

		// Queue metrics
		QueueSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "queue_size",
				Help: "Current number of messages in the queue",
			},
			[]string{"queue_name"},
		),
		MessagePublishRate: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "queue_messages_published_total",
				Help: "Total number of messages published to the queue",
			},
			[]string{"queue_name", "message_type"},
		),
		MessageProcessTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "queue_message_process_time_seconds",
				Help:    "Time taken to process a message",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"queue_name", "message_type"},
		),
		MessageErrorRate: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "queue_message_errors_total",
				Help: "Total number of message processing errors",
			},
			[]string{"queue_name", "error_type"},
		),
		QueueLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "queue_latency_seconds",
				Help:    "Queue latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"queue_name"},
		),
		ActiveConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "queue_active_connections",
				Help: "Number of active connections to the queue",
			},
		),
	}

	// Register metrics with the default registry
	prometheus.MustRegister(
		collector.HTTPRequestsTotal,
		collector.HTTPRequestDuration,
		collector.QueueSize,
		collector.MessagePublishRate,
		collector.MessageProcessTime,
		collector.MessageErrorRate,
		collector.QueueLatency,
		collector.ActiveConnections,
	)

	return &Handler{
		queueService: queueService,
		metrics:      collector,
	}
}

// RegisterRoutes registers the HTTP routes
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	// Health check - skip logging
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.HandlerFor(
		prometheus.Gatherers{
			prometheus.DefaultGatherer,
			metrics.DefaultRegistry.Registry,
		},
		promhttp.HandlerOpts{},
	)))

	// API routes
	api := router.Group("/api/v1")
	{
		queue := api.Group("/queue")
		{
			queue.POST("/messages", h.metricsMiddleware(), h.PublishMessage)
			queue.GET("/status/:id", h.metricsMiddleware(), h.GetMessageStatus)
		}
	}
}

// metricsMiddleware returns a gin middleware that records metrics
func (h *Handler) metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		h.metrics.RecordHTTPRequest(c.Request.Method, c.Request.URL.Path, fmt.Sprintf("%d", c.Writer.Status()))
		h.metrics.RecordHTTPRequestDuration(c.Request.Method, c.Request.URL.Path, duration)
	}
}

// HealthCheck handles the health check endpoint
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// PublishMessage handles the publish message endpoint
func (h *Handler) PublishMessage(c *gin.Context) {
	var msg model.Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		h.metrics.RecordMessageError("default", "invalid_request")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	start := time.Now()
	if err := h.queueService.PublishMessage(&msg); err != nil {
		h.metrics.RecordMessageError("default", "publish_error")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Record metrics
	messageType := string(msg.Type)
	h.metrics.RecordMessagePublished("default", messageType)
	h.metrics.RecordMessageProcessTime("default", messageType, time.Since(start).Seconds())
	h.metrics.IncrementQueueSize("default")

	c.JSON(http.StatusAccepted, gin.H{
		"message_id": msg.ID,
	})
}

// GetMessageStatus handles the get message status endpoint
func (h *Handler) GetMessageStatus(c *gin.Context) {
	messageID := c.Param("id")
	start := time.Now()

	status, err := h.queueService.GetMessageStatus(messageID)
	if err != nil {
		h.metrics.RecordMessageError("default", "status_error")
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Record metrics
	h.metrics.RecordQueueLatency("default", time.Since(start).Seconds())

	c.JSON(http.StatusOK, status)
}
