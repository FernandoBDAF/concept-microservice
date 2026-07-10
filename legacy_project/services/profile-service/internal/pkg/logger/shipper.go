package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// LogShipper handles shipping logs to a centralized system
type LogShipper struct {
	client      *http.Client
	endpoint    string
	buffer      []map[string]interface{}
	bufferSize  int
	bufferMutex sync.Mutex
	maxRetries  int
	retryDelay  time.Duration
}

// NewLogShipper creates a new log shipper
func NewLogShipper(endpoint string, bufferSize, maxRetries int, retryDelay time.Duration) *LogShipper {
	return &LogShipper{
		client: &http.Client{
			Timeout: time.Second * 5,
		},
		endpoint:   endpoint,
		buffer:     make([]map[string]interface{}, 0, bufferSize),
		bufferSize: bufferSize,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

// Ship sends logs to the centralized system
func (s *LogShipper) Ship(ctx context.Context, logEntry map[string]interface{}) error {
	s.bufferMutex.Lock()
	s.buffer = append(s.buffer, logEntry)
	shouldShip := len(s.buffer) >= s.bufferSize
	bufferCopy := make([]map[string]interface{}, len(s.buffer))
	copy(bufferCopy, s.buffer)
	s.buffer = s.buffer[:0]
	s.bufferMutex.Unlock()

	if shouldShip {
		return s.sendLogs(ctx, bufferCopy)
	}
	return nil
}

// Flush sends any remaining logs in the buffer
func (s *LogShipper) Flush(ctx context.Context) error {
	s.bufferMutex.Lock()
	if len(s.buffer) == 0 {
		s.bufferMutex.Unlock()
		return nil
	}
	bufferCopy := make([]map[string]interface{}, len(s.buffer))
	copy(bufferCopy, s.buffer)
	s.buffer = s.buffer[:0]
	s.bufferMutex.Unlock()

	return s.sendLogs(ctx, bufferCopy)
}

// sendLogs sends logs to the centralized system with retry mechanism
func (s *LogShipper) sendLogs(ctx context.Context, logs []map[string]interface{}) error {
	data, err := json.Marshal(logs)
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	var lastErr error
	for i := 0; i < s.maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "POST", s.endpoint, bytes.NewBuffer(data))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(s.retryDelay)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		time.Sleep(s.retryDelay)
	}

	return fmt.Errorf("failed to ship logs after %d retries: %w", s.maxRetries, lastErr)
}

// Config holds the log shipping configuration
type ShippingConfig struct {
	Enabled    bool
	Endpoint   string
	BufferSize int
	MaxRetries int
	RetryDelay time.Duration
}

// InitializeShipper initializes the log shipper if enabled
func InitializeShipper(cfg *ShippingConfig) (*LogShipper, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("log shipping endpoint is required")
	}

	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 100 // default buffer size
	}

	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3 // default max retries
	}

	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = time.Second // default retry delay
	}

	return NewLogShipper(
		cfg.Endpoint,
		cfg.BufferSize,
		cfg.MaxRetries,
		cfg.RetryDelay,
	), nil
}
