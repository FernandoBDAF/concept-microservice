package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/fernandobarroso/microservices/api-service/internal/infrastructure/auth"
)

// bearerToken extracts the Bearer token from the Authorization header,
// writing the 401 response itself when the header is missing or malformed.
func bearerToken(c *gin.Context) (string, bool) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header is required"})
		c.Abort()
		return "", false
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
		c.Abort()
		return "", false
	}
	return parts[1], true
}

// LocalAuthMiddleware validates RS256 JWTs locally against the cached
// auth-service JWKS (ADR-009.1). This is the DEFAULT auth path: no
// per-request hop to auth-service. It sets the same context keys the
// introspection middleware sets.
func LocalAuthMiddleware(verifier *auth.JWKSVerifier, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok {
			return
		}

		claims, err := verifier.Verify(c.Request.Context(), token)
		if err != nil {
			logger.Warn("Local token verification failed", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("user_role", claims.Role)
		c.Set("user_email", claims.Email)

		c.Next()
	}
}

// AuthMiddleware validates JWT tokens by introspecting against
// auth-service over HTTP (circuit-breaker protected). Since ADR-009.1 this
// is the opt-in strict path (API_AUTH_STRICT_INTROSPECTION=true), kept
// intact for revocation-strict use and the EXP-43 comparison.
func AuthMiddleware(authClient *auth.Client, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok {
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), authClient.Timeout())
		defer cancel()

		resp, err := authClient.ValidateToken(ctx, token)
		if err != nil {
			logger.Warn("Token validation failed", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		if !resp.Data.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Set("user_id", resp.Data.User.ID)
		c.Set("user_role", resp.Data.User.Role)
		c.Set("user_email", resp.Data.User.Email)

		c.Next()
	}
}
