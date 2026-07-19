package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/fernandobarroso/microservices/api-service/internal/config"
)

// authBreakerState exposes the auth-service circuit breaker state for
// alerting (AuthBreakerOpen fires on == 2): 0 closed, 1 half-open, 2 open.
var authBreakerState = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "api_auth_breaker_state",
	Help: "State of the auth-service circuit breaker (0=closed, 1=half-open, 2=open)",
})

func init() {
	prometheus.MustRegister(authBreakerState)
}

// breakerStateValue maps a gobreaker state to the gauge encoding above.
func breakerStateValue(state gobreaker.State) float64 {
	switch state {
	case gobreaker.StateHalfOpen:
		return 1
	case gobreaker.StateOpen:
		return 2
	default: // gobreaker.StateClosed
		return 0
	}
}

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
		OnStateChange: func(_ string, _, to gobreaker.State) {
			authBreakerState.Set(breakerStateValue(to))
		},
	}
	// A fresh breaker starts closed.
	authBreakerState.Set(breakerStateValue(gobreaker.StateClosed))

	return &Client{
		baseURL: cfg.URL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			// otelhttp gives client spans (and traceparent header injection)
			// for introspection calls; a no-op tracer provider makes this free.
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		circuitBreaker: gobreaker.NewCircuitBreaker(cbSettings),
	}
}

// Timeout returns the configured request timeout used for auth-service calls.
func (c *Client) Timeout() time.Duration {
	return c.httpClient.Timeout
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

	// Only network failures and server errors (5xx) should trip the circuit
	// breaker: those indicate auth-service itself is unhealthy. A per the
	// contract, a non-200 response (typically 401) just means "invalid
	// token" - a normal business outcome, not a service failure. Counting
	// those against the breaker would let a burst of bad/expired tokens from
	// legitimate clients trip the breaker and lock out everyone else.
	respAny, err := c.circuitBreaker.Execute(func() (interface{}, error) {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= http.StatusInternalServerError {
			return nil, fmt.Errorf("auth service returned status %d", resp.StatusCode)
		}

		var validateResp ValidateResponse
		if err := json.NewDecoder(resp.Body).Decode(&validateResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			// Non-200 (e.g. 401) is treated as an invalid token, not an error.
			validateResp.Data.Valid = false
		}

		return &validateResp, nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth service request failed: %w", err)
	}

	validateResp := respAny.(*ValidateResponse)
	if validateResp.Error != "" {
		return nil, fmt.Errorf("auth service error: %s", validateResp.Error)
	}

	return validateResp, nil
}
