package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sony/gobreaker"

	"github.com/fernandobarroso/microservices/api-service/internal/config"
)

type Client struct {
	baseURL        string
	httpClient     *http.Client
	circuitBreaker *gobreaker.CircuitBreaker
}

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

func NewClient(cfg config.AuthConfig, cbCfg config.CircuitBreakerConfig) *Client {
	cbSettings := gobreaker.Settings{
		Name:        "auth-service",
		MaxRequests: cbCfg.MaxRequests,
		Interval:    cbCfg.Interval,
		Timeout:     cbCfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cbCfg.ReadyToTrip
		},
	}

	return &Client{
		baseURL: cfg.URL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		circuitBreaker: gobreaker.NewCircuitBreaker(cbSettings),
	}
}

func (c *Client) ValidateToken(ctx context.Context, token string) (*ValidateResponse, error) {
	body, err := json.Marshal(struct {
		Token string `json:"token"`
	}{Token: token})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/auth/token/validate", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	respAny, err := c.circuitBreaker.Execute(func() (interface{}, error) {
		return c.httpClient.Do(req)
	})
	if err != nil {
		return nil, fmt.Errorf("auth service request failed: %w", err)
	}

	resp := respAny.(*http.Response)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected auth status code %d", resp.StatusCode)
	}

	var validateResp ValidateResponse
	if err := json.NewDecoder(resp.Body).Decode(&validateResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if validateResp.Error != "" {
		return nil, fmt.Errorf("auth service error: %s", validateResp.Error)
	}

	return &validateResp, nil
}

