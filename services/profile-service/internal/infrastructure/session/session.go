package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fernandobarroso/profile-service/microservices/services/profile-service/internal/domain/services"
	"github.com/redis/go-redis/v9"
)

// Session represents a user session
type Session struct {
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SessionManagerInterface defines the interface for session management
type SessionManagerInterface interface {
	CreateSession(userID, password string) (string, error)
	ValidateSession(tokenString string) (*Session, error)
	InvalidateSession(tokenString string) error
	Close() error
}

// SessionManager implements session management using Redis
type SessionManager struct {
	authClient *services.AuthServiceClient
	redis      *redis.Client
}

// NewSessionManager creates a new session manager
func NewSessionManager(authClient *services.AuthServiceClient) (*SessionManager, error) {
	// Get Redis configuration from environment variables with defaults
	redisAddr := getEnvOrDefault("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnvOrDefault("REDIS_PASSWORD", "")
	redisDB := 0 // Default DB

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[NewSessionManager] Failed to connect to Redis at %s: %v", redisAddr, err)
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	log.Printf("[NewSessionManager] Successfully connected to Redis at %s", redisAddr)

	return &SessionManager{
		authClient: authClient,
		redis:      rdb,
	}, nil
}

// getEnvOrDefault returns the value of the environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// CreateSession creates a new session for a user
func (m *SessionManager) CreateSession(userID, password string) (string, error) {
	// Get token from auth service
	token, err := m.authClient.GetToken(context.Background(), userID, password)
	if err != nil {
		log.Printf("[CreateSession] Error getting token: %v", err)
		return "", err
	}

	// Create session
	session := &Session{
		UserID:    userID,
		Role:      "user", // Set default role
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	// Store session in Redis
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		log.Printf("[CreateSession] Error marshaling session: %v", err)
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = m.redis.Set(ctx, token, sessionJSON, 24*time.Hour).Err()
	if err != nil {
		log.Printf("[CreateSession] Error storing session in Redis: %v", err)
		return "", err
	}

	log.Printf("[CreateSession] Session stored in Redis for user: %s", userID)
	return token, nil
}

// ValidateSession validates a session token
func (m *SessionManager) ValidateSession(tokenString string) (*Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get session from Redis
	sessionJSON, err := m.redis.Get(ctx, tokenString).Bytes()
	if err == redis.Nil {
		log.Printf("[ValidateSession] Session not found in Redis for token: %s", tokenString)
		return nil, ErrInvalidSession
	} else if err != nil {
		log.Printf("[ValidateSession] Error getting session from Redis: %v", err)
		return nil, ErrInvalidSession
	}

	var session Session
	if err := json.Unmarshal(sessionJSON, &session); err != nil {
		log.Printf("[ValidateSession] Error unmarshaling session: %v", err)
		return nil, ErrInvalidSession
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		log.Printf("[ValidateSession] Session expired for user: %s", session.UserID)
		m.InvalidateSession(tokenString)
		return nil, ErrSessionExpired
	}

	// Validate token with auth service
	_, err = m.authClient.ValidateToken(context.Background(), tokenString)
	if err != nil {
		log.Printf("[ValidateSession] Auth service validation failed for user: %s, error: %v", session.UserID, err)
		return nil, ErrInvalidSession
	}

	// TODO: Re-enable these checks when using real auth service
	// // Verify user ID and role match
	// if session.UserID != validateResp.Data.User.ID {
	// 	log.Printf("[ValidateSession] User ID mismatch: session=%s, auth=%s", session.UserID, validateResp.Data.User.ID)
	// 	return nil, ErrInvalidSession
	// }
	// if session.Role != validateResp.Data.User.Role {
	// 	log.Printf("[ValidateSession] Role mismatch: session=%s, auth=%s", session.Role, validateResp.Data.User.Role)
	// 	return nil, ErrInvalidSession
	// }

	log.Printf("[ValidateSession] Session validated successfully for user: %s", session.UserID)
	return &session, nil
}

// InvalidateSession invalidates a session
func (m *SessionManager) InvalidateSession(tokenString string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.redis.Del(ctx, tokenString).Err()
	if err != nil {
		log.Printf("[InvalidateSession] Error deleting session from Redis: %v", err)
		return err
	}
	return nil
}

// Close cleans up the session manager
func (m *SessionManager) Close() error {
	return m.redis.Close()
}

// Error definitions
var (
	ErrInvalidSession = fmt.Errorf("invalid session")
	ErrSessionExpired = fmt.Errorf("session expired")
)
