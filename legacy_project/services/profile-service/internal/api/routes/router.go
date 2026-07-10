package routes

import (
	"net/http"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/api/handlers"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/api/middleware"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/config"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/services"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/infrastructure/session"
	"github.com/gin-gonic/gin"
)

// Router represents the API router
type Router struct {
	engine         *gin.Engine
	cfg            *config.Config
	authClient     *services.AuthServiceClient
	sessionManager session.SessionManagerInterface
	profileService *services.ProfileService
}

// NewRouter creates a new API router
func NewRouter(cfg *config.Config, authClient *services.AuthServiceClient, sessionManager session.SessionManagerInterface, profileService *services.ProfileService) *Router {
	router := &Router{
		engine:         gin.Default(),
		cfg:            cfg,
		authClient:     authClient,
		sessionManager: sessionManager,
		profileService: profileService,
	}
	router.setupRoutes()
	return router
}

// setupRoutes configures all the routes for the API
func (r *Router) setupRoutes() {
	// Health check endpoint - skip logging
	r.engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	// Metrics endpoints
	metrics := r.engine.Group("/metrics")
	{
		metricsHandler := handlers.NewMetricsHandler()
		metrics.GET("", metricsHandler.GetMetrics)
		metrics.DELETE("", metricsHandler.ResetMetrics)
	}

	// API v1 group
	v1 := r.engine.Group("/api/v1")
	{
		// Auth endpoints
		auth := v1.Group("/auth")
		{
			authHandler := handlers.NewAuthHandler(r.sessionManager)
			auth.POST("/token", authHandler.Authenticate)
			auth.POST("/validate", authHandler.ValidateToken)
		}

		// NEW: User management endpoints
		users := v1.Group("/users")
		{
			userHandler := handlers.NewUserHandler(r.profileService)

			// Apply authorization middleware for user management
			users.Use(middleware.RoleMiddleware("admin")) // Only admins can manage users

			users.POST("", userHandler.CreateUser)
			users.GET("/email/:email", userHandler.GetUserByEmail)
			users.PUT("/:id", userHandler.UpdateUser)
			users.DELETE("/:id", userHandler.DeleteUser)
		}

		// Profile endpoints
		profiles := v1.Group("/profiles")
		{
			// Initialize profile handler
			profileHandler := handlers.NewProfileHandler(r.profileService)

			// Apply session middleware
			profiles.Use(middleware.SessionMiddleware(r.sessionManager))

			// Profile routes
			profiles.GET("", profileHandler.GetProfiles)
			profiles.GET("/:id", profileHandler.GetProfile)
			profiles.POST("", profileHandler.CreateProfile)
			profiles.PUT("/:id", profileHandler.UpdateProfile)
			profiles.DELETE("/:id", profileHandler.DeleteProfile)

			// ✅ ENHANCED: Task routes with multi-worker support
			taskHandler := handlers.NewTaskHandler(r.profileService)

			// Generic task submission endpoint (maintain backward compatibility)
			profiles.POST("/:id/tasks", taskHandler.SubmitTask)

			// ✅ NEW: Specialized task endpoints for Phase 2 Multi-Worker Task Support
			profiles.POST("/:id/tasks/email", taskHandler.SubmitEmailTask)     // Email notifications
			profiles.POST("/:id/tasks/image", taskHandler.SubmitImageTask)     // Image processing
			profiles.POST("/:id/tasks/profile", taskHandler.SubmitProfileTask) // Profile tasks

			// ✅ NEW: Task statistics and monitoring endpoints
			profiles.GET("/:id/tasks/stats", taskHandler.GetTaskTypeStats) // Task statistics
		}
	}
}

// Run starts the API server
func (r *Router) Run(addr string) error {
	return r.engine.Run(addr)
}

// ServeHTTP implements the http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.engine.ServeHTTP(w, req)
}
