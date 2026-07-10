package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/config"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"go.uber.org/zap"
)

// AuthServiceClientInterface defines the interface for auth service operations
type AuthServiceClientInterface interface {
	GetToken(ctx context.Context, userID, password string) (string, error)
	ValidateToken(ctx context.Context, token string) (*ValidateResponse, error)
	CreateUser(ctx context.Context, userData *models.CreateUserRequest) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	UpdateUser(ctx context.Context, userID string, userData *models.UpdateUserRequest) (*models.User, error)
	DeleteUser(ctx context.Context, userID string) error
	GetCircuitBreakerStats() CircuitBreakerStats
	IsCircuitBreakerOpen() bool
}

// AuthServiceClient handles communication with the auth service
type AuthServiceClient struct {
	client         *http.Client
	baseURL        string
	circuitBreaker *CircuitBreaker
	logger         *zap.Logger
}

// NewAuthServiceClient creates a new auth service client
func NewAuthServiceClient(cfg *config.Config) *AuthServiceClient {
	return &AuthServiceClient{
		client: &http.Client{
			Timeout: time.Second * 5,
		},
		baseURL: cfg.Auth.URL,
		logger:  zap.L().Named("auth_client"), // Initialize logger
	}
}

// TokenRequest represents a request to get a token
type TokenRequest struct {
	UserID   string `json:"user_id"`
	Password string `json:"password"`
}

// TokenResponse represents a response containing a token
type TokenResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Data    struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	} `json:"data"`
}

// Status:  "success",
//
//	Message: "Token is valid",
//	Data: map[string]interface{}{
//		"valid": true,
//		"user": map[string]string{
//			"id":    "user1",
//			"email": "user1@example.com",
//			"role":  "user",
//		},
//	},
//
// ValidateResponse represents a response from token validation
type ValidateResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Valid bool `json:"valid"`
		User  struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"user"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

// GetToken gets a token from the auth service
func (c *AuthServiceClient) GetToken(ctx context.Context, userID, password string) (string, error) {
	req := TokenRequest{
		UserID:   userID,
		Password: password,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log the full URL for debugging
	fullURL := c.baseURL + "/v1/auth/login"
	fmt.Printf("Making request to: %s\n", fullURL)

	resp, err := c.client.Post(fullURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("auth service error: %s", tokenResp.Error)
	}

	return tokenResp.Data.AccessToken, nil
}

// ValidateToken validates a token with the auth service
func (c *AuthServiceClient) ValidateToken(ctx context.Context, token string) (*ValidateResponse, error) {
	body, err := json.Marshal(struct {
		Token string `json:"token"`
	}{
		Token: token,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/auth/token/validate", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %v\n", string(body))
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var validateResp ValidateResponse
	if err := json.NewDecoder(resp.Body).Decode(&validateResp); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if validateResp.Error != "" {
		fmt.Printf("Error: %v\n", validateResp.Error)
		return nil, fmt.Errorf("auth service error: %s", validateResp.Error)
	}
	fmt.Printf("ValidateResponse: %v\n", validateResp)

	return &validateResp, nil
}

func (c *AuthServiceClient) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	c.logger.Debug("Getting user by email via auth service", zap.String("email", email))

	url := fmt.Sprintf("%s/api/v1/auth/users/email/%s", c.baseURL, email)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("Failed to get user by email", zap.Error(err))
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, models.ErrUserNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Success bool        `json:"success"`
		Data    models.User `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response.Data, nil
}

func (c *AuthServiceClient) CreateUser(ctx context.Context, userData *models.CreateUserRequest) (*models.User, error) {
	c.logger.Info("Creating user via auth service", zap.String("email", userData.Email))

	body, err := json.Marshal(userData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/auth/users", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("Failed to create user", zap.Error(err))
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Success bool        `json:"success"`
		Data    models.User `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response.Data, nil
}

func (c *AuthServiceClient) UpdateUser(ctx context.Context, userID string, userData *models.UpdateUserRequest) (*models.User, error) {
	c.logger.Info("Updating user via auth service", zap.String("user_id", userID))

	body, err := json.Marshal(userData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/auth/users/%s", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("Failed to update user", zap.Error(err))
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, models.ErrUserNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Success bool        `json:"success"`
		Data    models.User `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response.Data, nil
}

func (c *AuthServiceClient) DeleteUser(ctx context.Context, userID string) error {
	c.logger.Info("Deleting user via auth service", zap.String("user_id", userID))

	url := fmt.Sprintf("%s/api/v1/auth/users/%s", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("Failed to delete user", zap.Error(err))
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return models.ErrUserNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Circuit breaker methods
func (c *AuthServiceClient) GetCircuitBreakerStats() CircuitBreakerStats {
	if c.circuitBreaker != nil {
		return c.circuitBreaker.GetStats()
	}
	return CircuitBreakerStats{}
}

func (c *AuthServiceClient) IsCircuitBreakerOpen() bool {
	if c.circuitBreaker != nil {
		return c.circuitBreaker.IsOpen()
	}
	return false
}

type CircuitBreaker struct {
	failureCount     int64
	successCount     int64
	requestCount     int64
	failureThreshold int64
	isOpen           bool
	lastFailureTime  time.Time
	recoveryTimeout  time.Duration
	logger           *zap.Logger
}

type CircuitBreakerStats struct {
	Failures    int64  `json:"failures"`
	Successes   int64  `json:"successes"`
	Requests    int64  `json:"requests"`
	IsOpen      bool   `json:"is_open"`
	LastFailure string `json:"last_failure,omitempty"`
}

func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	lastFailure := ""
	if !cb.lastFailureTime.IsZero() {
		lastFailure = cb.lastFailureTime.Format(time.RFC3339)
	}

	return CircuitBreakerStats{
		Failures:    cb.failureCount,
		Successes:   cb.successCount,
		Requests:    cb.requestCount,
		IsOpen:      cb.isOpen,
		LastFailure: lastFailure,
	}
}

func (cb *CircuitBreaker) IsOpen() bool {
	return cb.isOpen
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.requestCount++

	if cb.isOpen {
		if time.Since(cb.lastFailureTime) < cb.recoveryTimeout {
			return fmt.Errorf("circuit breaker is open")
		}
		cb.logger.Info("Circuit breaker attempting recovery")
		cb.isOpen = false
	}

	err := fn()
	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		if cb.failureCount >= cb.failureThreshold {
			cb.isOpen = true
			cb.logger.Warn("Circuit breaker opened due to failures",
				zap.Int64("failure_count", cb.failureCount),
				zap.Int64("threshold", cb.failureThreshold))
		}
		return err
	}

	cb.successCount++
	if cb.isOpen {
		cb.logger.Info("Circuit breaker recovered")
		cb.isOpen = false
		cb.failureCount = 0
	}

	return nil
}
