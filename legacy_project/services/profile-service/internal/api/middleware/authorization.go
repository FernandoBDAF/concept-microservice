package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RoleMiddleware checks if the user has the required role
func RoleMiddleware(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from session
		userID := c.GetString("user_id")
		userRole := c.GetString("user_role")

		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			c.Abort()
			return
		}

		// Check if user has required role
		if userRole != requiredRole && userRole != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// UserOwnershipMiddleware ensures users can only access their own data
// REVIEW: we should implement this middleware
func UserOwnershipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		requestedUserID := c.Param("id")

		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			c.Abort()
			return
		}

		// Users can only access their own data unless they're admin
		userRole := c.GetString("user_role")
		if userID != requestedUserID && userRole != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "access denied",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
