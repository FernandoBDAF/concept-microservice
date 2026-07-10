package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/config"
	"go.uber.org/zap"
)

// CacheClient represents the HTTP-based cache client for communicating with cache-service
type CacheClient struct {
	httpClient     *http.Client
	baseURL        string
	config         *config.CacheConfig
	logger         *zap.Logger
	circuitBreaker CircuitBreakerInterface
}

// CacheClientInterface defines the interface for cache operations
type CacheClientInterface interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	GetSession(ctx context.Context, sessionID string) ([]byte, error)
	SetSession(ctx context.Context, sessionID string, data []byte, ttl time.Duration) error
	GetProfile(ctx context.Context, profileID string) ([]byte, error)
	SetProfile(ctx context.Context, profileID string, data []byte, ttl time.Duration) error
	Ping(ctx context.Context) error
	Close() error
	// ✅ NEW: Circuit breaker monitoring
	GetCircuitBreakerStats() CircuitBreakerStats
	IsCircuitBreakerOpen() bool
}

// CacheRequest represents a cache operation request
type CacheRequest struct {
	Key   string        `json:"key"`
	Value string        `json:"value,omitempty"`
	TTL   time.Duration `json:"ttl,omitempty"`
}

// CacheResponse represents a cache operation response
type CacheResponse struct {
	Status string `json:"status"`
	Data   string `json:"data,omitempty"`
	Error  string `json:"error,omitempty"`
}

// CircuitBreakerInterface defines circuit breaker operations
type CircuitBreakerInterface interface {
	Execute(fn func() error) error
	IsOpen() bool
	GetStats() CircuitBreakerStats
}

// CircuitBreakerStats represents circuit breaker statistics
type CircuitBreakerStats struct {
	Failures    int64  `json:"failures"`
	Successes   int64  `json:"successes"`
	Requests    int64  `json:"requests"`
	IsOpen      bool   `json:"is_open"`
	LastFailure string `json:"last_failure,omitempty"`
}

// SimpleCircuitBreaker implements a basic circuit breaker pattern
type SimpleCircuitBreaker struct {
	failureCount     int64
	successCount     int64
	requestCount     int64
	failureThreshold int64
	isOpen           bool
	lastFailureTime  time.Time
	recoveryTimeout  time.Duration
	logger           *zap.Logger
}

// NewSimpleCircuitBreaker creates a new circuit breaker
func NewSimpleCircuitBreaker(failureThreshold int64, recoveryTimeout time.Duration, logger *zap.Logger) *SimpleCircuitBreaker {
	return &SimpleCircuitBreaker{
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
		logger:           logger,
	}
}

func (cb *SimpleCircuitBreaker) Execute(fn func() error) error {
	cb.requestCount++

	// Check if circuit is open and if recovery timeout has passed
	if cb.isOpen {
		if time.Since(cb.lastFailureTime) < cb.recoveryTimeout {
			return fmt.Errorf("circuit breaker is open")
		}
		// Try to recover
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
		cb.failureCount = 0 // Reset failure count on recovery
	}

	return nil
}

func (cb *SimpleCircuitBreaker) IsOpen() bool {
	return cb.isOpen
}

func (cb *SimpleCircuitBreaker) GetStats() CircuitBreakerStats {
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

// NewCacheClient creates a new HTTP-based cache client with circuit breaker
func NewCacheClient(config *config.CacheConfig, logger *zap.Logger) (*CacheClient, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("cache service is disabled")
	}

	baseURL := fmt.Sprintf("http://%s:%d", config.Host, config.Port)

	// Create HTTP client with timeouts and connection pooling
	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Create circuit breaker with configurable parameters
	circuitBreaker := NewSimpleCircuitBreaker(
		int64(config.Retries+2), // Failure threshold (slightly more than retries)
		30*time.Second,          // Recovery timeout
		logger,
	)

	client := &CacheClient{
		httpClient:     httpClient,
		baseURL:        baseURL,
		config:         config,
		logger:         logger,
		circuitBreaker: circuitBreaker,
	}

	// Test connection to cache service
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		logger.Error("Failed to connect to cache service", zap.String("url", baseURL), zap.Error(err))
		return nil, fmt.Errorf("failed to connect to cache service at %s: %w", baseURL, err)
	}

	logger.Info("Successfully connected to cache service with circuit breaker",
		zap.String("url", baseURL),
		zap.Int64("circuit_breaker_threshold", int64(config.Retries+2)))
	return client, nil
}

// Get retrieves a value from the cache with circuit breaker protection
func (c *CacheClient) Get(ctx context.Context, key string) ([]byte, error) {
	var result []byte
	var getError error

	// Execute cache operation through circuit breaker
	err := c.circuitBreaker.Execute(func() error {
		url := fmt.Sprintf("%s/api/v1/cache/%s", c.baseURL, key)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.logger.Error("Cache GET request failed", zap.String("key", key), zap.Error(err))
			return fmt.Errorf("cache GET request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			getError = ErrKeyNotFound
			return nil // Not a circuit breaker failure, just a cache miss
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			c.logger.Error("Cache GET request failed",
				zap.String("key", key),
				zap.Int("status", resp.StatusCode),
				zap.String("body", string(body)))
			return fmt.Errorf("cache GET request failed with status %d", resp.StatusCode)
		}

		var response CacheResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		if response.Status != "success" {
			return fmt.Errorf("cache operation failed: %s", response.Status)
		}

		result = []byte(response.Data)
		c.logger.Debug("Cache GET successful", zap.String("key", key))
		return nil
	})

	// Handle circuit breaker errors
	if err != nil {
		if c.circuitBreaker.IsOpen() {
			c.logger.Warn("Cache GET failed due to circuit breaker being open",
				zap.String("key", key),
				zap.Error(err))
			return nil, fmt.Errorf("cache service unavailable (circuit breaker open): %w", err)
		}
		return nil, err
	}

	// Return the specific error if it was a cache miss
	if getError != nil {
		return nil, getError
	}

	return result, nil
}

// Set stores a value in the cache with circuit breaker protection
func (c *CacheClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	// Execute cache operation through circuit breaker
	err := c.circuitBreaker.Execute(func() error {
		url := fmt.Sprintf("%s/api/v1/cache/%s", c.baseURL, key)

		cacheReq := CacheRequest{
			Key:   key,
			Value: string(value),
			TTL:   ttl,
		}

		jsonData, err := json.Marshal(cacheReq)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.logger.Error("Cache SET request failed", zap.String("key", key), zap.Error(err))
			return fmt.Errorf("cache SET request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			c.logger.Error("Cache SET request failed",
				zap.String("key", key),
				zap.Int("status", resp.StatusCode),
				zap.String("body", string(body)))
			return fmt.Errorf("cache SET request failed with status %d", resp.StatusCode)
		}

		var response CacheResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		if response.Status != "success" {
			return fmt.Errorf("cache operation failed: %s", response.Status)
		}

		c.logger.Debug("Cache SET successful", zap.String("key", key), zap.Duration("ttl", ttl))
		return nil
	})

	// Handle circuit breaker errors
	if err != nil {
		if c.circuitBreaker.IsOpen() {
			c.logger.Warn("Cache SET failed due to circuit breaker being open",
				zap.String("key", key),
				zap.Error(err))
			return fmt.Errorf("cache service unavailable (circuit breaker open): %w", err)
		}
		return err
	}

	return nil
}

// Delete removes a value from the cache with circuit breaker protection
func (c *CacheClient) Delete(ctx context.Context, key string) error {
	// Execute cache operation through circuit breaker
	err := c.circuitBreaker.Execute(func() error {
		url := fmt.Sprintf("%s/api/v1/cache/%s", c.baseURL, key)

		req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.logger.Error("Cache DELETE request failed", zap.String("key", key), zap.Error(err))
			return fmt.Errorf("cache DELETE request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			body, _ := io.ReadAll(resp.Body)
			c.logger.Error("Cache DELETE request failed",
				zap.String("key", key),
				zap.Int("status", resp.StatusCode),
				zap.String("body", string(body)))
			return fmt.Errorf("cache DELETE request failed with status %d", resp.StatusCode)
		}

		c.logger.Debug("Cache DELETE successful", zap.String("key", key))
		return nil
	})

	// Handle circuit breaker errors
	if err != nil {
		if c.circuitBreaker.IsOpen() {
			c.logger.Warn("Cache DELETE failed due to circuit breaker being open",
				zap.String("key", key),
				zap.Error(err))
			// For DELETE operations, we can be more lenient - if cache is down,
			// we don't necessarily need to fail the operation
			c.logger.Info("Continuing despite failed cache DELETE due to circuit breaker",
				zap.String("key", key))
			return nil // Don't fail DELETE operations due to cache unavailability
		}
		return err
	}

	return nil
}

// GetSession retrieves session data from cache
func (c *CacheClient) GetSession(ctx context.Context, sessionID string) ([]byte, error) {
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	return c.Get(ctx, sessionKey)
}

// SetSession stores session data in cache
func (c *CacheClient) SetSession(ctx context.Context, sessionID string, data []byte, ttl time.Duration) error {
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	return c.Set(ctx, sessionKey, data, ttl)
}

// GetProfile retrieves profile data from cache
func (c *CacheClient) GetProfile(ctx context.Context, profileID string) ([]byte, error) {
	profileKey := fmt.Sprintf("profile:%s", profileID)
	return c.Get(ctx, profileKey)
}

// SetProfile stores profile data in cache
func (c *CacheClient) SetProfile(ctx context.Context, profileID string, data []byte, ttl time.Duration) error {
	profileKey := fmt.Sprintf("profile:%s", profileID)
	return c.Set(ctx, profileKey, data, ttl)
}

// Ping tests the connection to the cache service
func (c *CacheClient) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ping request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cache service health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up the cache client (no-op for HTTP client)
func (c *CacheClient) Close() error {
	c.logger.Info("Cache client closed")
	return nil
}

// GetCircuitBreakerStats returns circuit breaker statistics for monitoring
func (c *CacheClient) GetCircuitBreakerStats() CircuitBreakerStats {
	if c.circuitBreaker != nil {
		return c.circuitBreaker.GetStats()
	}
	return CircuitBreakerStats{}
}

// IsCircuitBreakerOpen returns whether the circuit breaker is currently open
func (c *CacheClient) IsCircuitBreakerOpen() bool {
	if c.circuitBreaker != nil {
		return c.circuitBreaker.IsOpen()
	}
	return false
}

// Error definitions
var (
	ErrKeyNotFound = fmt.Errorf("key not found in cache")
)
