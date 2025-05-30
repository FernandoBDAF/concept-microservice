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
)

// AuthServiceClient handles communication with the auth service
type AuthServiceClient struct {
	client  *http.Client
	baseURL string
}

// NewAuthServiceClient creates a new auth service client
func NewAuthServiceClient(cfg *config.Config) *AuthServiceClient {
	return &AuthServiceClient{
		client: &http.Client{
			Timeout: time.Second * 5,
		},
		baseURL: cfg.Auth.URL,
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
