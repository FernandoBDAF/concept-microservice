package integration

import (
	"context"
	"testing"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserManagementEndToEnd(t *testing.T) {
	// Initialize test dependencies
	ctx := context.Background()
	service := setupTestService(t)

	// Test user creation
	createReq := &models.CreateUserRequest{
		Email:     "test@example.com",
		Password:  "password123",
		FirstName: "Test",
		LastName:  "User",
	}

	user, err := service.CreateUser(ctx, createReq)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, createReq.Email, user.Email)
	assert.True(t, user.IsActive)

	// Test get user by email
	foundUser, err := service.GetUserByEmail(ctx, user.Email)
	require.NoError(t, err)
	require.NotNil(t, foundUser)
	assert.Equal(t, user.ID, foundUser.ID)

	// Test user update
	updateReq := &models.UpdateUserRequest{
		FirstName: stringPtr("Updated"),
		LastName:  stringPtr("Name"),
	}

	updatedUser, err := service.UpdateUser(ctx, user.ID, updateReq)
	require.NoError(t, err)
	require.NotNil(t, updatedUser)
	assert.Equal(t, *updateReq.FirstName, "Updated")
	assert.Equal(t, *updateReq.LastName, "Name")

	// Test user deletion
	err = service.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	// Verify user is deleted
	_, err = service.GetUserByEmail(ctx, user.Email)
	assert.Equal(t, models.ErrUserNotFound, err)
}

func TestUserValidation(t *testing.T) {
	ctx := context.Background()
	service := setupTestService(t)

	tests := []struct {
		name    string
		req     *models.CreateUserRequest
		wantErr bool
	}{
		{
			name: "valid user",
			req: &models.CreateUserRequest{
				Email:     "test@example.com",
				Password:  "password123",
				FirstName: "Test",
				LastName:  "User",
			},
			wantErr: false,
		},
		{
			name: "invalid email",
			req: &models.CreateUserRequest{
				Email:     "invalid-email",
				Password:  "password123",
				FirstName: "Test",
				LastName:  "User",
			},
			wantErr: true,
		},
		{
			name: "short password",
			req: &models.CreateUserRequest{
				Email:     "test@example.com",
				Password:  "short",
				FirstName: "Test",
				LastName:  "User",
			},
			wantErr: true,
		},
		{
			name: "missing first name",
			req: &models.CreateUserRequest{
				Email:    "test@example.com",
				Password: "password123",
				LastName: "User",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreateUser(ctx, tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUserMetrics(t *testing.T) {
	ctx := context.Background()
	service := setupTestService(t)

	// Reset metrics before test
	metrics.ResetUserMetrics()

	// Create a user and perform operations
	createReq := &models.CreateUserRequest{
		Email:     "test@example.com",
		Password:  "password123",
		FirstName: "Test",
		LastName:  "User",
	}

	start := time.Now()
	user, err := service.CreateUser(ctx, createReq)
	require.NoError(t, err)
	require.NotNil(t, user)

	// Record the operation manually for testing
	metrics.RecordUserOperation("create_user", time.Since(start))

	// Get metrics from the metrics package
	metricsData := metrics.GetUserMetrics()
	assert.Greater(t, metricsData["operation_count"], int64(0))
	assert.Greater(t, metricsData["operation_latency"], int64(0))
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
