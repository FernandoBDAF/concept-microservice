package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fernandobarroso/microservices/api-service/internal/domain/task"
)

type TaskHandler struct {
	service *task.Service
}

func NewTaskHandler(service *task.Service) *TaskHandler {
	return &TaskHandler{service: service}
}

type TaskRequest struct {
	RoutingKey string                 `json:"routing_key"`
	Type       string                 `json:"type"`
	Payload    map[string]interface{} `json:"payload"`
	Metadata   map[string]string      `json:"metadata"`
}

func (h *TaskHandler) SubmitTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile id"})
		return
	}

	var req TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.RoutingKey == "" || req.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "routing_key and type are required"})
		return
	}

	if req.Payload == nil {
		req.Payload = map[string]interface{}{}
	}
	req.Payload["profile_id"] = id.String()

	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	if userID, ok := c.Get("user_id"); ok {
		metadata["user_id"] = toString(userID)
	}

	taskID, err := h.service.Submit(c.Request.Context(), req.RoutingKey, req.Type, req.Payload, metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"task_id": taskID})
}

func (h *TaskHandler) SubmitEmailTask(c *gin.Context) {
	h.submitTypedTask(c, "email.send", "email.send")
}

func (h *TaskHandler) SubmitImageTask(c *gin.Context) {
	h.submitTypedTask(c, "image.process", "image.process")
}

func (h *TaskHandler) SubmitProfileTask(c *gin.Context) {
	h.submitTypedTask(c, "profile.task", "profile.task")
}

func (h *TaskHandler) submitTypedTask(c *gin.Context, routingKey, msgType string) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile id"})
		return
	}

	payload := map[string]interface{}{}
	if err := c.ShouldBindJSON(&payload); err != nil && err != http.ErrBodyNotAllowed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	payload["profile_id"] = id.String()

	metadata := map[string]string{}
	if userID, ok := c.Get("user_id"); ok {
		metadata["user_id"] = toString(userID)
	}

	taskID, err := h.service.Submit(c.Request.Context(), routingKey, msgType, payload, metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"task_id": taskID})
}

func toString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

