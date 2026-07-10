package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/config"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/metrics"
)

// getEnvOrDefault returns the value of the environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// StorageError represents a storage service error
type StorageError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *StorageError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// StorageClient handles communication with the storage service
type StorageClient struct {
	client  *http.Client
	baseURL string
	config  *config.StorageConfig
	auth    *config.SecurityConfig
}

// NewStorageClient creates a new storage service client
func NewStorageClient(cfg *config.Config) *StorageClient {
	return &StorageClient{
		client: &http.Client{
			Timeout: time.Second * 5,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		baseURL: getEnvOrDefault("STORAGE_SERVICE_URL", fmt.Sprintf("http://%s:%d/api/v1", cfg.Storage.Host, cfg.Storage.Port)),
		config:  &cfg.Storage,
		auth:    &cfg.Security,
	}
}

// doRequest performs an HTTP request with retry mechanism
func (c *StorageClient) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < c.config.MaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		if err != nil {
			return nil, &StorageError{
				Code:    http.StatusInternalServerError,
				Message: "Failed to create request",
				Err:     err,
			}
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		if c.auth.Enabled {
			// TODO: Get JWT token from context or session
			// req.Header.Set("Authorization", "Bearer "+token)
		}

		// Add request ID for tracing
		if requestID := ctx.Value("request_id"); requestID != nil {
			req.Header.Set("X-Request-ID", requestID.(string))
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = &StorageError{
				Code:    http.StatusServiceUnavailable,
				Message: fmt.Sprintf("Attempt %d: Failed to send request", attempt+1),
				Err:     err,
			}
			time.Sleep(time.Duration(attempt+1) * c.config.RetryDelay)
			continue
		}

		// Check if we should retry based on status code
		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = &StorageError{
				Code:    resp.StatusCode,
				Message: fmt.Sprintf("Attempt %d: Server error", attempt+1),
				Err:     fmt.Errorf("response body: %s", string(body)),
			}
			time.Sleep(time.Duration(attempt+1) * c.config.RetryDelay)
			continue
		}

		return resp, nil
	}

	return nil, &StorageError{
		Code:    http.StatusServiceUnavailable,
		Message: "All retry attempts failed",
		Err:     lastErr,
	}
}

// CreateProfile creates a new profile in the storage service
func (c *StorageClient) CreateProfile(ctx context.Context, profile *models.Profile) (*models.Profile, error) {
	start := time.Now()
	defer func() {
		metrics.RecordCreateProfile(time.Since(start))
	}()

	body, err := json.Marshal(profile)
	if err != nil {
		metrics.RecordError()
		return nil, fmt.Errorf("failed to marshal profile: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", "/profiles", bytes.NewBuffer(body))
	if err != nil {
		metrics.RecordError()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		metrics.RecordError()
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var createdProfile models.Profile
	if err := json.NewDecoder(resp.Body).Decode(&createdProfile); err != nil {
		metrics.RecordError()
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &createdProfile, nil
}

// GetProfile retrieves a profile from the storage service
func (c *StorageClient) GetProfile(ctx context.Context, id string) (*models.Profile, error) {
	start := time.Now()
	defer func() {
		metrics.RecordGetProfile(time.Since(start))
	}()

	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/profiles/%s", id), nil)
	if err != nil {
		metrics.RecordError()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		metrics.RecordError()
		return nil, fmt.Errorf("profile not found")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		metrics.RecordError()
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var profile models.Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		metrics.RecordError()
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	profile.GetFrom = "storage"
	return &profile, nil
}

// UpdateProfile updates an existing profile in the storage service
func (c *StorageClient) UpdateProfile(ctx context.Context, id string, profile *models.Profile) (*models.Profile, error) {
	start := time.Now()
	defer func() {
		metrics.RecordUpdateProfile(time.Since(start))
	}()

	body, err := json.Marshal(profile)
	if err != nil {
		metrics.RecordError()
		return nil, fmt.Errorf("failed to marshal profile: %w", err)
	}

	resp, err := c.doRequest(ctx, "PUT", fmt.Sprintf("/profiles/%s", id), bytes.NewBuffer(body))
	if err != nil {
		metrics.RecordError()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		metrics.RecordError()
		return nil, fmt.Errorf("profile not found")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		metrics.RecordError()
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var updatedProfile models.Profile
	if err := json.NewDecoder(resp.Body).Decode(&updatedProfile); err != nil {
		metrics.RecordError()
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &updatedProfile, nil
}

// DeleteProfile deletes a profile from the storage service
func (c *StorageClient) DeleteProfile(ctx context.Context, id string) error {
	start := time.Now()
	defer func() {
		metrics.RecordDeleteProfile(time.Since(start))
	}()

	resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/profiles/%s", id), nil)
	if err != nil {
		metrics.RecordError()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		metrics.RecordError()
		return fmt.Errorf("profile not found")
	}

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		metrics.RecordError()
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetProfiles retrieves all profiles from the storage service
func (c *StorageClient) GetProfiles(ctx context.Context) ([]*models.Profile, error) {
	start := time.Now()
	defer func() {
		metrics.RecordGetProfiles(time.Since(start))
	}()

	resp, err := c.doRequest(ctx, "GET", "/profiles", nil)
	if err != nil {
		metrics.RecordError()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		metrics.RecordError()
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var profiles []*models.Profile
	if err := json.NewDecoder(resp.Body).Decode(&profiles); err != nil {
		metrics.RecordError()
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return profiles, nil
}
