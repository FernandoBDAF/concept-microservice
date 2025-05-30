package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/FBDAF/microservices/services/common/config"
	"github.com/FBDAF/microservices/services/common/errors"
	"github.com/FBDAF/microservices/services/common/interfaces/transport"
	"github.com/FBDAF/microservices/services/common/logging"
	"github.com/FBDAF/microservices/services/common/middleware"
	"github.com/FBDAF/microservices/services/common/models"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UserService implements the service.Service interface for User entities
type UserService struct {
	logger *logging.Logger
	// TODO: Add repository once available
}

// NewUserService creates a new UserService
func NewUserService(logger *logging.Logger) *UserService {
	return &UserService{
		logger: logger,
	}
}

// Create implements service.Service.Create
func (s *UserService) Create(ctx context.Context, user *models.User) error {
	// TODO: Add repository integration
	s.logger.Info("Creating user", zap.String("email", user.Email))
	return nil
}

// Get implements service.Service.Get
func (s *UserService) Get(ctx context.Context, id string) (*models.User, error) {
	// TODO: Add repository integration
	s.logger.Info("Getting user", zap.String("id", id))
	return &models.User{
		ID:        id,
		Name:      "Test User",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
	}, nil
}

// List implements service.Service.List
func (s *UserService) List(ctx context.Context) ([]*models.User, error) {
	// TODO: Add repository integration
	s.logger.Info("Listing users")
	return []*models.User{}, nil
}

// Update implements service.Service.Update
func (s *UserService) Update(ctx context.Context, user *models.User) error {
	// TODO: Add repository integration
	s.logger.Info("Updating user", zap.String("id", user.ID))
	return nil
}

// Delete implements service.Service.Delete
func (s *UserService) Delete(ctx context.Context, id string) error {
	// TODO: Add repository integration
	s.logger.Info("Deleting user", zap.String("id", id))
	return nil
}

// Validate implements service.Validator.Validate
func (s *UserService) Validate(ctx context.Context, user *models.User) error {
	if user.Name == "" {
		return errors.New("name is required")
	}
	if user.Email == "" {
		return errors.New("email is required")
	}
	if len(user.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

// UserHandler implements transport.Handler
type UserHandler struct {
	service *UserService
	logger  *logging.Logger
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(service *UserService, logger *logging.Logger) *UserHandler {
	return &UserHandler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes implements transport.Handler.RegisterRoutes
func (h *UserHandler) RegisterRoutes(router *gin.Engine) {
	router.POST("/users", h.CreateUser)
	router.GET("/users/:id", h.GetUser)
}

// CreateUser handles user creation
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req models.User
	if err := c.ShouldBindJSON(&req); err != nil {
		transport.ErrorResponse(c, http.StatusBadRequest, err)
		return
	}

	if err := h.service.Validate(c.Request.Context(), &req); err != nil {
		transport.ErrorResponse(c, http.StatusBadRequest, err)
		return
	}

	if err := h.service.Create(c.Request.Context(), &req); err != nil {
		transport.ErrorResponse(c, http.StatusInternalServerError, err)
		return
	}

	transport.SuccessResponse(c, http.StatusCreated, req)
}

// GetUser handles user retrieval
func (h *UserHandler) GetUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		transport.ErrorResponse(c, http.StatusBadRequest, errors.New("id is required"))
		return
	}

	user, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		transport.ErrorResponse(c, http.StatusInternalServerError, err)
		return
	}

	transport.SuccessResponse(c, http.StatusOK, user)
}

// GinLoggerAdapter implements middleware.LogRequest interface
type GinLoggerAdapter struct {
	logger *logging.Logger
}

func (g *GinLoggerAdapter) LogRequest(r *http.Request, statusCode int, duration time.Duration) {
	g.logger.Info("HTTP request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("ip", r.RemoteAddr),
		zap.Int("status", statusCode),
		zap.Duration("duration", duration),
	)
}

func main() {
	// Initialize configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		panic(errors.Wrap(err, "failed to load configuration"))
	}

	// Initialize logger
	logger, err := logging.New()
	if err != nil {
		panic(errors.Wrap(err, "failed to initialize logger"))
	}

	// Create service
	userService := NewUserService(logger)

	// Create handler
	userHandler := NewUserHandler(userService, logger)

	// Create gin engine
	r := gin.Default()

	// Use middleware with the adapter
	ginLogger := &GinLoggerAdapter{logger: logger}
	r.Use(middleware.LoggingMiddleware(ginLogger))
	r.Use(middleware.RecoveryMiddleware())
	r.Use(middleware.CORSMiddleware())

	// Register routes
	userHandler.RegisterRoutes(r)

	// Start server
	port := 8080 // Default port
	if cfg != nil {
		if cfgPort := cfg.GetInt("server.port"); cfgPort > 0 {
			port = cfgPort
		}
	}
	logger.Info("Starting server", zap.Int("port", port))
	if err := r.Run(fmt.Sprintf(":%d", port)); err != nil {
		logger.Error("Server failed", zap.Error(err))
		panic(err)
	}
}
