package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
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
	jwksVerifier *auth.JWKSVerifier,
	profileService *profile.Service,
	taskService *task.Service,
	documentService *document.Service,
	healthHandler *handlers.HealthHandler,
	logger *zap.Logger,
) *Router {
	engine := gin.New()
	// otelgin goes first so every downstream middleware/handler (including
	// the logging middleware's trace_id field) sees the server span in the
	// request context. With the no-op tracer provider (tracing disabled)
	// this is effectively free.
	engine.Use(otelgin.Middleware("api-service"))
	engine.Use(gin.Recovery())
	engine.Use(middleware.LoggingMiddleware(logger))
	engine.Use(middleware.MetricsMiddleware())

	// Liveness/readiness stay on the main API port. Prometheus metrics are
	// served on their own port (cfg.Metrics.Port) via NewMetricsServer - see
	// metrics_server.go - to match the platform contract's dedicated metrics
	// port instead of sharing the public API surface.
	engine.GET("/health", healthHandler.Liveness)
	engine.GET("/ready", healthHandler.Readiness)

	v1 := engine.Group("/api/v1")
	// ADR-009.1: local JWKS verification is the default auth path;
	// API_AUTH_STRICT_INTROSPECTION=true switches back to per-request HTTP
	// introspection (breaker path kept intact for EXP-43).
	if cfg.Auth.StrictIntrospection {
		logger.Info("auth: strict introspection mode (per-request auth-service validation)")
		v1.Use(middleware.AuthMiddleware(authClient, logger))
	} else {
		logger.Info("auth: local JWKS verification mode")
		v1.Use(middleware.LocalAuthMiddleware(jwksVerifier, logger))
	}
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
