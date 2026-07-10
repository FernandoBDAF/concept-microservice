package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/services"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/infrastructure/cache"

	"go.uber.org/zap"
)

// Session represents a user session
type Session struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// SessionManagerInterface defines the interface for session management
type SessionManagerInterface interface {
	CreateSession(userID, password string) (string, error)
	ValidateSession(tokenString string) (*Session, error)
	InvalidateSession(tokenString string) error
	Close() error
}

// SessionManager implements session management using HTTP cache service
type SessionManager struct {
	authClient  *services.AuthServiceClient
	cacheClient cache.CacheClientInterface
	logger      *zap.Logger
}

// NewSessionManager creates a new session manager using HTTP cache service
func NewSessionManager(authClient *services.AuthServiceClient, cacheClient cache.CacheClientInterface, logger *zap.Logger) (*SessionManager, error) {
	if cacheClient == nil {
		return nil, fmt.Errorf("cache client is required")
	}

	// Test the cache service connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cacheClient.Ping(ctx); err != nil {
		logger.Error("Failed to connect to cache service", zap.Error(err))
		return nil, fmt.Errorf("failed to connect to cache service: %w", err)
	}

	logger.Info("SessionManager successfully connected to cache service via HTTP")

	return &SessionManager{
		authClient:  authClient,
		cacheClient: cacheClient,
		logger:      logger,
	}, nil
}

// CreateSession creates a new session for a user
func (m *SessionManager) CreateSession(userID, password string) (string, error) {
	// Get token from auth service
	token, err := m.authClient.GetToken(context.Background(), userID, password)
	if err != nil {
		m.logger.Error("Error getting token from auth service", zap.String("user_id", userID), zap.Error(err))
		return "", fmt.Errorf("failed to get token from auth service: %w", err)
	}

	// Create session
	session := &Session{
		UserID:    userID,
		Role:      "user",
	}

	// Store session in cache service via HTTP
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		m.logger.Error("Error marshaling session", zap.String("user_id", userID), zap.Error(err))
		return "", fmt.Errorf("failed to marshal session: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the cache client's SetSession method with appropriate TTL
	err = m.cacheClient.SetSession(ctx, token, sessionJSON, 24*time.Hour)
	if err != nil {
		m.logger.Error("Error storing session in cache service", zap.String("user_id", userID), zap.Error(err))
		return "", fmt.Errorf("failed to store session in cache service: %w", err)
	}

	m.logger.Info("Session stored in cache service via HTTP",
		zap.String("user_id", userID),
		zap.String("session_id", token))
	return token, nil
}

// ValidateSession validates a session token
func (m *SessionManager) ValidateSession(tokenString string) (*Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get session from cache service via HTTP
	sessionJSON, err := m.cacheClient.GetSession(ctx, tokenString)
	if err == cache.ErrKeyNotFound {
		m.logger.Debug("Session not found in cache service", zap.String("token", tokenString))
		return nil, ErrInvalidSession
	} else if err != nil {
		m.logger.Error("Error getting session from cache service", zap.String("token", tokenString), zap.Error(err))
		return nil, fmt.Errorf("failed to get session from cache service: %w", err)
	}

	var session Session
	if err := json.Unmarshal(sessionJSON, &session); err != nil {
		m.logger.Error("Error unmarshaling session", zap.String("token", tokenString), zap.Error(err))
		return nil, ErrInvalidSession
	}

	// Validate token with auth service
	// REVIEW: we should consider remove this call to the auth service
	_, err = m.authClient.ValidateToken(context.Background(), tokenString)
	if err != nil {
		m.logger.Error("Auth service validation failed",
			zap.String("user_id", session.UserID),
			zap.String("token", tokenString),
			zap.Error(err))
		return nil, ErrInvalidSession
	}

	// TODO: Re-enable these checks when using real auth service
	// // Verify user ID and role match
	// if session.UserID != validateResp.Data.User.ID {
	// 	m.logger.Error("User ID mismatch",
	//		zap.String("session_user", session.UserID),
	//		zap.String("auth_user", validateResp.Data.User.ID))
	// 	return nil, ErrInvalidSession
	// }
	// if session.Role != validateResp.Data.User.Role {
	// 	m.logger.Error("Role mismatch",
	//		zap.String("session_role", session.Role),
	//		zap.String("auth_role", validateResp.Data.User.Role))
	// 	return nil, ErrInvalidSession
	// }

	m.logger.Debug("Session validated successfully via cache service",
		zap.String("user_id", session.UserID),
		zap.String("token", tokenString))
	return &session, nil
}

// InvalidateSession invalidates a session
func (m *SessionManager) InvalidateSession(tokenString string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete session from cache service via HTTP
	err := m.cacheClient.Delete(ctx, fmt.Sprintf("session:%s", tokenString))
	if err != nil {
		m.logger.Error("Error deleting session from cache service", zap.String("token", tokenString), zap.Error(err))
		return fmt.Errorf("failed to delete session from cache service: %w", err)
	}

	m.logger.Info("Session invalidated via cache service", zap.String("token", tokenString))
	return nil
}

// Close cleans up the session manager
func (m *SessionManager) Close() error {
	m.logger.Info("Closing session manager")
	return m.cacheClient.Close()
}

// Error definitions
var (
	ErrInvalidSession = fmt.Errorf("invalid session")
	ErrSessionExpired = fmt.Errorf("session expired")
)
