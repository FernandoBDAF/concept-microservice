package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/services"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/infrastructure/session"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"go.uber.org/zap"
)

// SessionMiddleware handles session validation for API requests
func SessionMiddleware(sessionManager session.SessionManagerInterface) gin.HandlerFunc {
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("unkown error validating session: %v", err)})
			}
			c.Abort()
			return
		}

		// Store session info in context
		c.Set("user_id", sess.UserID)
		c.Set("user_role", sess.Role) // Updated to match authorization middleware expectations
		c.Set("role", sess.Role)

		c.Next()
	}
}

// AuthServiceMiddleware validates tokens using the auth service
func AuthServiceMiddleware(authClient services.AuthServiceClientInterface, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		token := parts[1]

		// Create context with timeout for auth service call
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		// Validate token with auth service
		validateResp, err := authClient.ValidateToken(ctx, token)
		if err != nil {
			logger.Warn("Token validation failed", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		if !validateResp.Data.Valid {
			logger.Warn("Token validation returned invalid", zap.String("token", token[:10]+"..."))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// Store user info in context for use by other middlewares/handlers
		c.Set("user_id", validateResp.Data.User.ID)
		c.Set("user_role", validateResp.Data.User.Role)
		c.Set("user_email", validateResp.Data.User.Email)
		c.Set("role", validateResp.Data.User.Role) // For backward compatibility

		c.Next()
	}
}

// JWTAuthMiddleware validates JWT tokens in incoming requests (for direct JWT validation)
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

		// Extract claims if needed
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			c.Set("user_id", claims["user_id"])
			c.Set("user_role", claims["role"])
			c.Set("role", claims["role"])
		}

		c.Next()
	}
}
