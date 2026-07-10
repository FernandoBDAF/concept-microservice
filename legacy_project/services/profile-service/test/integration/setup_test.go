package integration

import (
	"testing"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/config"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/services"
	"go.uber.org/zap"
)

// setupTestService creates a ProfileService instance for testing
func setupTestService(t *testing.T) *services.ProfileService {
	// Initialize logger for tests
	logger := zap.NewNop() // Use no-op logger for tests

	cfg := &config.Config{
		Auth: config.AuthConfig{
			URL: "http://localhost:8080", // Mock URL for testing
		},
	}

	// Create auth client with test configuration
	authClient := services.NewAuthServiceClient(cfg)

	// Create profile service with test dependencies
	service := services.NewProfileService(
		cfg,        // Config
		nil,        // StorageClient (not needed for user tests)
		nil,        // CacheClient (not needed for user tests)
		authClient, // AuthClient
		logger,     // Logger
	)

	return service
}
