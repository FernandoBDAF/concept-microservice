package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/fernandobarroso/microservices/api-service/internal/infrastructure/rabbitmq"
	"github.com/fernandobarroso/microservices/api-service/internal/infrastructure/redis"
)

type HealthHandler struct {
	db       *sqlx.DB
	redis    *redis.Client
	rabbitmq *rabbitmq.Client
}

func NewHealthHandler(db *sqlx.DB, redisClient *redis.Client, rabbitmqClient *rabbitmq.Client) *HealthHandler {
	return &HealthHandler{
		db:       db,
		redis:    redisClient,
		rabbitmq: rabbitmqClient,
	}
}

func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	status := http.StatusOK
	details := gin.H{}

	if err := h.db.PingContext(ctx); err != nil {
		status = http.StatusServiceUnavailable
		details["postgres"] = "down"
	} else {
		details["postgres"] = "ok"
	}

	if h.redis != nil {
		if err := h.redis.Ping(ctx); err != nil {
			status = http.StatusServiceUnavailable
			details["redis"] = "down"
		} else {
			details["redis"] = "ok"
		}
	} else {
		details["redis"] = "disabled"
	}

	if h.rabbitmq != nil && !h.rabbitmq.IsConnected() {
		status = http.StatusServiceUnavailable
		details["rabbitmq"] = "down"
	} else if h.rabbitmq != nil {
		details["rabbitmq"] = "ok"
	}

	c.JSON(status, gin.H{
		"status":  statusText(status),
		"checks":  details,
		"service": "api-service",
	})
}

func statusText(status int) string {
	if status == http.StatusOK {
		return "ok"
	}
	return "degraded"
}

