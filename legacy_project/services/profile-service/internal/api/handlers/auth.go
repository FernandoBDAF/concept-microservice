package handlers

import (
	"net/http"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/infrastructure/session"
	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	sessionManager session.SessionManagerInterface
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(sessionManager session.SessionManagerInterface) *AuthHandler {
	return &AuthHandler{
		sessionManager: sessionManager,
	}
}

// AuthenticateRequest represents a request to authenticate a user
type AuthenticateRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AuthenticateResponse represents a response containing a token
type AuthenticateResponse struct {
	Token string `json:"token"`
	Error string `json:"error,omitempty"`
}

// Authenticate handles POST /api/v1/auth/token
func (h *AuthHandler) Authenticate(c *gin.Context) {
	var req AuthenticateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AuthenticateResponse{
			Error: err.Error(),
		})
		return
	}

	token, err := h.sessionManager.CreateSession(req.UserID, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AuthenticateResponse{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, AuthenticateResponse{
		Token: token,
	})
}

// ValidateToken handles POST /api/v1/auth/validate
func (h *AuthHandler) ValidateToken(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authorization header is required",
		})
		return
	}

	// Extract token from Bearer header
	token := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	// Validate session
	sess, err := h.sessionManager.ValidateSession(token)
	if err != nil {
		switch err {
		case session.ErrSessionExpired:
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "session expired",
			})
		case session.ErrInvalidSession:
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid session",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "unkown error validating token",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": sess.UserID,
		"role":    sess.Role,
	})
}
