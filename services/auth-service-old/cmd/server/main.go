package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// MockUser represents a user in the system
type MockUser struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Password string `json:"-"` // Not exposed in JSON
	Role     string `json:"role"`
}

// MockToken represents a JWT token
type MockToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// MockResponse represents a generic API response
type MockResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func main() {
	// Set Gin to release mode in production
	gin.SetMode(gin.ReleaseMode)

	// Create a new Gin router with default middleware
	r := gin.Default()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, MockResponse{
			Status:  "success",
			Message: "Auth service is healthy!",
		})
	})

	// Auth endpoints
	r.POST("/v1/auth/register", mockRegister)
	r.POST("/v1/auth/login", mockLogin)
	r.POST("/v1/auth/token/refresh", mockRefreshToken)
	r.POST("/v1/auth/token/validate", mockValidateToken)
	r.POST("/v1/auth/password/reset", mockResetPassword)

	// User endpoints
	r.GET("/v1/users/me", mockGetUser)
	r.GET("/v1/users/:id", mockGetUserByID)

	// OAuth endpoints
	r.GET("/v1/oauth/authorize", mockOAuthAuthorize)
	r.POST("/v1/oauth/token", mockOAuthToken)
	r.GET("/v1/oauth/userinfo", mockOAuthUserInfo)

	// RBAC endpoints
	r.GET("/v1/rbac/roles", mockGetRoles)
	r.GET("/v1/rbac/permissions", mockGetPermissions)

	// Start server
	log.Println("Starting Auth Service on :8080")
	log.Fatal(r.Run(":8080"))
}

func mockRegister(c *gin.Context) {
	var user MockUser
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, MockResponse{
			Status:  "error",
			Message: "Invalid request body",
		})
		return
	}

	c.JSON(http.StatusOK, MockResponse{
		Status:  "success",
		Message: "User registered successfully",
		Data:    user,
	})
}

func mockLogin(c *gin.Context) {
	var credentials struct {
		UserID   string `json:"user_id" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&credentials); err != nil {
		c.JSON(http.StatusBadRequest, MockResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	token := MockToken{
		AccessToken:  "mock_access_token" + credentials.UserID,
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: "mock_refresh_token",
	}

	c.JSON(http.StatusOK, MockResponse{
		Status:  "success",
		Message: "Login successful",
		Data:    token,
	})
}

func mockRefreshToken(c *gin.Context) {
	var request struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, MockResponse{
			Status:  "error",
			Message: "Invalid request body",
		})
		return
	}

	token := MockToken{
		AccessToken:  "mock_new_access_token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: "mock_new_refresh_token",
	}

	c.JSON(http.StatusOK, MockResponse{
		Status:  "success",
		Message: "Token refreshed successfully",
		Data:    token,
	})
}

func mockValidateToken(c *gin.Context) {
	var request struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, MockResponse{
			Status:  "error",
			Message: "Invalid request body",
		})
		return
	}

	c.JSON(http.StatusOK, MockResponse{
		Status:  "success",
		Message: "Token is valid",
		Data: map[string]interface{}{
			"valid": true,
			"user": map[string]string{
				"id":    "user1",
				"email": "user1@example.com",
				"role":  "user",
			},
		},
	})
}

func mockResetPassword(c *gin.Context) {
	var request struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, MockResponse{
			Status:  "error",
			Message: "Invalid request body",
		})
		return
	}

	c.JSON(http.StatusOK, MockResponse{
		Status:  "success",
		Message: "Password reset email sent",
	})
}

func mockGetUser(c *gin.Context) {
	user := MockUser{
		ID:   "user1",
		Role: "user",
	}

	c.JSON(http.StatusOK, MockResponse{
		Status:  "success",
		Message: "User retrieved successfully",
		Data:    user,
	})
}

func mockGetUserByID(c *gin.Context) {
	userID := c.Param("id")
	user := MockUser{
		ID:   userID,
		Role: "user",
	}

	c.JSON(http.StatusOK, MockResponse{
		Status:  "success",
		Message: "User retrieved successfully",
		Data:    user,
	})
}

func mockOAuthAuthorize(c *gin.Context) {
	c.JSON(http.StatusOK, MockResponse{
		Status:  "success",
		Message: "Authorization successful",
		Data: map[string]string{
			"code": "mock_authorization_code",
		},
	})
}

func mockOAuthToken(c *gin.Context) {
	token := MockToken{
		AccessToken:  "mock_oauth_access_token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: "mock_oauth_refresh_token",
	}

	c.JSON(http.StatusOK, MockResponse{
		Status:  "success",
		Message: "OAuth token generated successfully",
		Data:    token,
	})
}

func mockOAuthUserInfo(c *gin.Context) {
	c.JSON(http.StatusOK, MockResponse{
		Status:  "success",
		Message: "User info retrieved successfully",
		Data: map[string]interface{}{
			"sub":            "user1",
			"email":          "user1@example.com",
			"email_verified": true,
			"name":           "Mock User",
		},
	})
}

func mockGetRoles(c *gin.Context) {
	roles := []string{"user", "admin", "manager"}

	c.JSON(http.StatusOK, MockResponse{
		Status:  "success",
		Message: "Roles retrieved successfully",
		Data:    roles,
	})
}

func mockGetPermissions(c *gin.Context) {
	permissions := map[string][]string{
		"user":    {"read:own", "write:own"},
		"admin":   {"read:all", "write:all", "delete:all"},
		"manager": {"read:all", "write:own", "delete:own"},
	}

	c.JSON(http.StatusOK, MockResponse{
		Status:  "success",
		Message: "Permissions retrieved successfully",
		Data:    permissions,
	})
}
