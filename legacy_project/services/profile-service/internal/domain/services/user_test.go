package services

import (
	"context"
	"testing"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockAuthClient is a mock implementation of AuthServiceClientInterface
type MockAuthClient struct {
	mock.Mock
}

// Implement all AuthServiceClientInterface methods
func (m *MockAuthClient) GetToken(ctx context.Context, userID, password string) (string, error) {
	args := m.Called(ctx, userID, password)
	return args.String(0), args.Error(1)
}

func (m *MockAuthClient) ValidateToken(ctx context.Context, token string) (*ValidateResponse, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ValidateResponse), args.Error(1)
}

func (m *MockAuthClient) CreateUser(ctx context.Context, userData *models.CreateUserRequest) (*models.User, error) {
	args := m.Called(ctx, userData)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockAuthClient) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockAuthClient) UpdateUser(ctx context.Context, userID string, userData *models.UpdateUserRequest) (*models.User, error) {
	args := m.Called(ctx, userID, userData)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockAuthClient) DeleteUser(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockAuthClient) GetCircuitBreakerStats() CircuitBreakerStats {
	args := m.Called()
	return args.Get(0).(CircuitBreakerStats)
}

func (m *MockAuthClient) IsCircuitBreakerOpen() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestCreateUser(t *testing.T) {
	mockAuth := new(MockAuthClient)
	service := &ProfileService{
		authClient: mockAuth,
		logger:     zap.NewNop(),
	}

	ctx := context.Background()
	req := &models.CreateUserRequest{
		Email:     "test@example.com",
		Password:  "password123",
		FirstName: "Test",
		LastName:  "User",
	}

	expectedUser := &models.User{
		ID:        "user123",
		Email:     req.Email,
		Role:      "user",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockAuth.On("CreateUser", ctx, req).Return(expectedUser, nil)

	user, err := service.CreateUser(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, expectedUser, user)
	mockAuth.AssertExpectations(t)
}

func TestGetUserByEmail(t *testing.T) {
	mockAuth := new(MockAuthClient)
	service := &ProfileService{
		authClient: mockAuth,
		logger:     zap.NewNop(),
	}

	ctx := context.Background()
	email := "test@example.com"

	expectedUser := &models.User{
		ID:        "user123",
		Email:     email,
		Role:      "user",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockAuth.On("GetUserByEmail", ctx, email).Return(expectedUser, nil)

	user, err := service.GetUserByEmail(ctx, email)
	assert.NoError(t, err)
	assert.Equal(t, expectedUser, user)
	mockAuth.AssertExpectations(t)
}

func TestUpdateUser(t *testing.T) {
	mockAuth := new(MockAuthClient)
	service := &ProfileService{
		authClient: mockAuth,
		logger:     zap.NewNop(),
	}

	ctx := context.Background()
	userID := "user123"
	req := &models.UpdateUserRequest{
		FirstName: stringPtr("Updated"),
		LastName:  stringPtr("User"),
	}

	expectedUser := &models.User{
		ID:        userID,
		Email:     "test@example.com",
		Role:      "user",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockAuth.On("UpdateUser", ctx, userID, req).Return(expectedUser, nil)

	user, err := service.UpdateUser(ctx, userID, req)
	assert.NoError(t, err)
	assert.Equal(t, expectedUser, user)
	mockAuth.AssertExpectations(t)
}

func TestDeleteUser(t *testing.T) {
	mockAuth := new(MockAuthClient)

	// Create a simple mock storage client
	service := &ProfileService{
		authClient:    mockAuth,
		logger:        zap.NewNop(),
		storageClient: nil, // Keep nil to test the user deletion without profile
	}

	ctx := context.Background()
	userID := "user123"

	mockAuth.On("DeleteUser", ctx, userID).Return(nil)

	err := service.DeleteUser(ctx, userID)
	assert.NoError(t, err)
	mockAuth.AssertExpectations(t)
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
