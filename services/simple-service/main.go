package main

import (
	"net/http"
	"time"

	"github.com/FBDAF/microservices/services/common/config"
	"github.com/FBDAF/microservices/services/common/errors"
	"github.com/FBDAF/microservices/services/common/logging"
	"github.com/FBDAF/microservices/services/common/middleware"
	"github.com/FBDAF/microservices/services/common/models"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Service represents our simple service
type Service struct {
	config *config.Config
	logger *logging.Logger
	// metrics  *metrics.Collector
	// security *security.Manager
}

// NewService creates a new service instance
func NewService() (*Service, error) {
	// Initialize configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		return nil, errors.Wrap(err, "failed to load configuration")
	}

	// Initialize logger
	logger, err := logging.New()
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize logger")
	}

	// TODO: Integrate metrics and security once APIs are available
	// collector := metrics.NewCollector()
	// manager := security.NewManager()

	return &Service{
		config: cfg,
		logger: logger,
		// metrics:  collector,
		// security: manager,
	}, nil
}

// UserHandler handles user-related requests
type UserHandler struct {
	service *Service
}

// CreateUserRequest represents a request to create a user
type CreateUserRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// CreateUserResponse represents the response for user creation
type CreateUserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// GetUserRequest represents a request to get user details
type GetUserRequest struct {
	ID string `json:"id" validate:"required"`
}

// GetUserResponse represents the response for user details
type GetUserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateUser handles requests to create a user
func (h *UserHandler) CreateUser(c *gin.Context) {
	// Log request
	h.service.logger.Info("Processing create user request",
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("ip", c.ClientIP()),
	)

	// Parse request body
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.service.logger.Error("Failed to parse request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate request
	if err := h.validateRequest(req); err != nil {
		h.service.logger.Error("Validation failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create user
	user := &models.User{
		ID:        "generated-id", // h.service.security.GenerateUUID(),
		Name:      req.Name,
		Email:     req.Email,
		Password:  req.Password, // hashedPassword,
		CreatedAt: time.Now(),
	}

	// Create response
	resp := CreateUserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}

	// Log success
	h.service.logger.Info("User created successfully",
		zap.String("user_id", user.ID),
		zap.String("email", user.Email),
	)

	c.JSON(http.StatusOK, resp)
}

// GetUser handles requests to retrieve user details
func (h *UserHandler) GetUser(c *gin.Context) {
	// Log request
	h.service.logger.Info("Processing get user request",
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("ip", c.ClientIP()),
	)

	// Parse request body
	var req GetUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.service.logger.Error("Failed to parse request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate request
	if req.ID == "" {
		h.service.logger.Error("Validation failed", zap.Error(errors.New("id is required")))
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	// TODO: Retrieve user details from a database or service
	user := &models.User{
		ID:        req.ID,
		Name:      "Test User",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
	}

	// Create response
	resp := GetUserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}

	// Log success
	h.service.logger.Info("User retrieved successfully",
		zap.String("user_id", user.ID),
		zap.String("email", user.Email),
	)

	c.JSON(http.StatusOK, resp)
}

// validateRequest validates the create user request
func (h *UserHandler) validateRequest(req CreateUserRequest) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.Email == "" {
		return errors.New("email is required")
	}
	// TODO: Integrate security once available
	// if !h.service.security.IsValidEmail(req.Email) {
	// 	return errors.New("invalid email format")
	// }
	if len(req.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

func main() {
	// Create service
	service, err := NewService()
	if err != nil {
		panic(err)
	}

	// Create user handler
	userHandler := &UserHandler{service: service}

	// Create gin engine
	r := gin.Default()

	// Use middleware
	r.Use(middleware.LoggingMiddleware(service.logger))
	r.Use(middleware.RecoveryMiddleware())
	r.Use(middleware.CORSMiddleware())

	// Define routes
	r.POST("/users", userHandler.CreateUser)
	r.GET("/users", userHandler.GetUser)

	// Start server
	service.logger.Info("Starting server", zap.Int("port", 8080))
	if err := r.Run(":8080"); err != nil {
		service.logger.Error("Server failed", zap.Error(err))
		panic(err)
	}
}
