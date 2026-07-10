package http

import (
	"encoding/json"
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

// PublishMessageRequest represents the request payload for publishing messages
type PublishMessageRequest struct {
	Type       string            `json:"type" binding:"required"`
	Payload    json.RawMessage   `json:"payload" binding:"required"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Priority   int32             `json:"priority,omitempty"`
	RoutingKey string            `json:"routing_key,omitempty"`
}

// PublishMessageResponse represents the response for published messages
type PublishMessageResponse struct {
	MessageID  string `json:"message_id"`
	Status     string `json:"status"`
	RoutingKey string `json:"routing_key"`
}

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
			queue.GET("/routing-keys", h.metricsMiddleware(), h.GetSupportedRoutingKeys)
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

// PublishMessage handles the publish message endpoint with routing key support
func (h *Handler) PublishMessage(c *gin.Context) {
	var req PublishMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.metrics.RecordMessageError("default", "invalid_request")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Validate routing key format (worker_type.action)
	if req.RoutingKey != "" {
		if !isValidRoutingKey(req.RoutingKey) {
			h.metrics.RecordMessageError("default", "invalid_routing_key")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid routing key format. Expected format: worker_type.action (e.g., profile.task, email.send, image.process)",
			})
			return
		}
	}

	// Create message from request
	msg := &model.Message{
		Type:          req.Type,
		Payload:       req.Payload,
		Metadata:      req.Metadata,
		Priority:      req.Priority,
		Timestamp:     time.Now().UTC(),
		CorrelationID: fmt.Sprintf("http-%d", time.Now().UnixNano()),
	}

	// Initialize metadata if nil
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]string)
	}

	// Determine routing key
	routingKey := req.RoutingKey
	if routingKey == "" {
		// Infer routing key from message type for backward compatibility
		switch req.Type {
		case "profile_update", "profile_task":
			routingKey = "profile.task"
		case "email_send", "email_task":
			routingKey = "email.send"
		case "image_process", "image_task":
			routingKey = "image.process"
		default:
			routingKey = "profile.task" // default routing key
		}
	}

	// Store routing key in metadata for reference
	msg.Metadata["routing_key"] = routingKey

	start := time.Now()
	var err error

	// Publish with routing key
	if req.RoutingKey != "" {
		err = h.queueService.PublishWithRoutingKey(routingKey, msg)
	} else {
		// Use backward compatible method
		err = h.queueService.PublishMessage(msg)
	}

	if err != nil {
		h.metrics.RecordMessageError("default", "publish_error")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to publish message",
			"details": err.Error(),
		})
		return
	}

	// Record metrics
	h.metrics.RecordMessagePublished("default", req.Type)
	h.metrics.RecordMessageProcessTime("default", req.Type, time.Since(start).Seconds())
	h.metrics.IncrementQueueSize("default")

	c.JSON(http.StatusAccepted, PublishMessageResponse{
		MessageID:  msg.ID,
		Status:     "accepted",
		RoutingKey: routingKey,
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

// GetSupportedRoutingKeys handles the get supported routing keys endpoint
func (h *Handler) GetSupportedRoutingKeys(c *gin.Context) {
	routingKeys := h.queueService.GetSupportedRoutingKeys()

	// Create detailed response with routing key information
	response := make(map[string]interface{})
	response["routing_keys"] = routingKeys
	response["configurations"] = model.DefaultRoutingMap

	c.JSON(http.StatusOK, response)
}

// isValidRoutingKey validates routing key format (worker_type.action)
func isValidRoutingKey(routingKey string) bool {
	// Check if routing key exists in supported routing keys
	_, exists := model.DefaultRoutingMap[routingKey]
	return exists
}
