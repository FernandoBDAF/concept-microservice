package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/fernandobarroso/microservices/api-service/internal/api/handlers"
	"github.com/fernandobarroso/microservices/api-service/internal/api/middleware"
	"github.com/fernandobarroso/microservices/api-service/internal/config"
	"github.com/fernandobarroso/microservices/api-service/internal/domain/document"
	"github.com/fernandobarroso/microservices/api-service/internal/domain/profile"
	"github.com/fernandobarroso/microservices/api-service/internal/domain/task"
	"github.com/fernandobarroso/microservices/api-service/internal/infrastructure/auth"
)

type Router struct {
	engine *gin.Engine
}

func NewRouter(
	cfg *config.Config,
	authClient *auth.Client,
	profileService *profile.Service,
	taskService *task.Service,
	documentService *document.Service,
	healthHandler *handlers.HealthHandler,
	logger *zap.Logger,
) *Router {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(middleware.LoggingMiddleware(logger))
	engine.Use(middleware.MetricsMiddleware())

	// Health and metrics
	engine.GET("/health", healthHandler.Liveness)
	engine.GET("/ready", healthHandler.Readiness)
	if cfg.Metrics.Enabled {
		engine.GET(cfg.Metrics.Path, gin.WrapH(promhttp.Handler()))
	}

	v1 := engine.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(authClient, logger))
	{
		profileHandler := handlers.NewProfileHandler(profileService)
		taskHandler := handlers.NewTaskHandler(taskService)
		documentHandler := handlers.NewDocumentHandler(documentService, logger)

		profiles := v1.Group("/profiles")
		{
			profiles.GET("", profileHandler.GetProfiles)
			profiles.GET("/:id", profileHandler.GetProfile)
			profiles.POST("", profileHandler.CreateProfile)
			profiles.PUT("/:id", profileHandler.UpdateProfile)
			profiles.DELETE("/:id", profileHandler.DeleteProfile)

			profiles.POST("/:id/tasks", taskHandler.SubmitTask)
			profiles.POST("/:id/tasks/email", taskHandler.SubmitEmailTask)
			profiles.POST("/:id/tasks/image", taskHandler.SubmitImageTask)
			profiles.POST("/:id/tasks/profile", taskHandler.SubmitProfileTask)
			profiles.GET("/:id/documents", documentHandler.ListByProfile)
		}

		documents := v1.Group("/documents")
		{
			documents.POST("/upload", documentHandler.Upload)
			documents.GET("/:id", documentHandler.GetByID)
			documents.GET("/:id/status", documentHandler.GetStatus)
			documents.GET("/:id/download", documentHandler.Download)
			documents.DELETE("/:id", documentHandler.Delete)
		}
	}

	engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	return &Router{engine: engine}
}

func (r *Router) Run(addr string) error {
	return r.engine.Run(addr)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.engine.ServeHTTP(w, req)
}

