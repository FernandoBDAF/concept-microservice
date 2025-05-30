package http

import (
	"net/http"

	"github.com/FBDAF/microservices/services/queue-service/internal/domain/model"
	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for the queue service
type Handler struct {
	queue QueueService
}

// QueueService defines the interface for queue operations
type QueueService interface {
	PublishMessage(msg *model.Message) error
	GetMessageStatus(messageID string) (*MessageStatus, error)
}

// MessageStatus represents the status of a message
type MessageStatus struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// NewHandler creates a new HTTP handler
func NewHandler(queue QueueService) *Handler {
	return &Handler{
		queue: queue,
	}
}

// RegisterRoutes registers the HTTP routes
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	v1 := router.Group("/api/v1/queue")
	{
		v1.POST("/messages", h.publishMessage)
		v1.GET("/status/:messageId", h.getMessageStatus)
	}
}

// publishMessage handles POST /api/v1/queue/messages
func (h *Handler) publishMessage(c *gin.Context) {
	var msg model.Message
	if err := c.ShouldBindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid message format",
		})
		return
	}

	if err := h.queue.PublishMessage(&msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to publish message",
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message_id": msg.ID,
		"status":     "accepted",
	})
}

// getMessageStatus handles GET /api/v1/queue/status/:messageId
func (h *Handler) getMessageStatus(c *gin.Context) {
	messageID := c.Param("messageId")
	if messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Message ID is required",
		})
		return
	}

	status, err := h.queue.GetMessageStatus(messageID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get message status",
		})
		return
	}

	if status == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Message not found",
		})
		return
	}

	c.JSON(http.StatusOK, status)
}
