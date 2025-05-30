package middleware

import (
	"net/http"
	"strings"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/api/handlers"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/infrastructure/session"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

// SessionMiddleware handles session validation for API requests
func SessionMiddleware(sessionManager handlers.SessionManagerInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header is required"})
			c.Abort()
			return
		}

		// Check if the header has the Bearer prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		// Validate session
		sess, err := sessionManager.ValidateSession(parts[1])
		if err != nil {
			switch err {
			case session.ErrSessionExpired:
				c.JSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
			case session.ErrInvalidSession:
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
			c.Abort()
			return
		}

		// Store session info in context
		c.Set("user_id", sess.UserID)
		c.Set("role", sess.Role)

		c.Next()
	}
}

// validateToken validates the JWT token
func validateToken(token string) bool {
	// TODO: Implement JWT validation
	return true
}

// JWTAuthMiddleware validates JWT tokens in incoming requests
func JWTAuthMiddleware(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			c.Abort()
			return
		}

		tokenString := parts[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(secretKey), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		c.Next()
	}
}
