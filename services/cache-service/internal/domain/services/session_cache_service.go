package services

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"cache-service/internal/config"
	"cache-service/internal/domain/models"
	"cache-service/internal/infrastructure/metrics"
)

// SessionCacheService implements session and authentication caching operations
// Supports session management and JWT token blacklisting
type SessionCacheService struct {
	cache   *CacheService
	logger  *zap.Logger
	metrics *metrics.Metrics
	config  *config.CacheConfig
}

// NewSessionCacheService creates a new session cache service
func NewSessionCacheService(
	cacheService *CacheService,
	logger *zap.Logger,
	metrics *metrics.Metrics,
	config *config.CacheConfig,
) *SessionCacheService {
	return &SessionCacheService{
		cache:   cacheService,
		logger:  logger,
		metrics: metrics,
		config:  config,
	}
}

// GetSession retrieves a session from cache by session ID
func (s *SessionCacheService) GetSession(ctx context.Context, sessionID string) (*models.Session, error) {
	start := time.Now()
	key := s.getSessionKey(sessionID)

	var session models.Session
	err := s.cache.GetJSON(ctx, key, &session)
	duration := time.Since(start)

	if err != nil {
		if err == ErrKeyNotFound {
			s.metrics.RecordSessionCacheOp("get_session", "miss")
			s.logger.Debug("Session cache miss", zap.String("session_id", sessionID))
		} else {
			s.metrics.RecordSessionCacheOp("get_session", "error")
			s.logger.Error("Session cache get failed",
				zap.String("session_id", sessionID),
				zap.Error(err))
		}
		return nil, err
	}

	// Check if session is expired
	if session.ExpiresAt.Before(time.Now()) {
		s.metrics.RecordSessionCacheOp("get_session", "expired")
		s.logger.Debug("Session expired", zap.String("session_id", sessionID))

		// Clean up expired session
		s.cache.Delete(ctx, key)
		return nil, ErrKeyNotFound
	}

	s.metrics.RecordSessionCacheOp("get_session", "hit")
	s.metrics.RecordCacheLatency("get_session", "hit", duration)
	s.logger.Debug("Session cache hit",
		zap.String("session_id", sessionID),
		zap.String("user_id", session.UserID),
		zap.Duration("latency", duration))

	return &session, nil
}

// SetSession stores a session in cache with session-specific TTL
func (s *SessionCacheService) SetSession(ctx context.Context, sessionID string, session *models.Session, ttl time.Duration) error {
	start := time.Now()
	key := s.getSessionKey(sessionID)

	// Use session TTL if not specified
	if ttl <= 0 {
		ttl = s.config.SessionTTL
	}

	// Update session metadata
	session.LastUsed = time.Now()

	err := s.cache.SetJSON(ctx, key, session, ttl)
	duration := time.Since(start)

	if err != nil {
		s.metrics.RecordSessionCacheOp("set_session", "error")
		s.logger.Error("Session cache set failed",
			zap.String("session_id", sessionID),
			zap.String("user_id", session.UserID),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return err
	}

	s.metrics.RecordSessionCacheOp("set_session", "success")
	s.metrics.RecordCacheLatency("set_session", "success", duration)
	s.logger.Debug("Session cached",
		zap.String("session_id", sessionID),
		zap.String("user_id", session.UserID),
		zap.Duration("ttl", ttl),
		zap.Duration("latency", duration))

	return nil
}

// DeleteSession removes a session from cache
func (s *SessionCacheService) DeleteSession(ctx context.Context, sessionID string) error {
	start := time.Now()
	key := s.getSessionKey(sessionID)

	err := s.cache.Delete(ctx, key)
	duration := time.Since(start)

	if err != nil {
		s.metrics.RecordSessionCacheOp("delete_session", "error")
		s.logger.Error("Session cache delete failed",
			zap.String("session_id", sessionID),
			zap.Error(err))
		return err
	}

	s.metrics.RecordSessionCacheOp("delete_session", "success")
	s.metrics.RecordCacheLatency("delete_session", "success", duration)
	s.logger.Debug("Session deleted",
		zap.String("session_id", sessionID),
		zap.Duration("latency", duration))

	return nil
}

// IsTokenBlacklisted checks if a JWT token is blacklisted
func (s *SessionCacheService) IsTokenBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	start := time.Now()
	key := s.getBlacklistKey(tokenID)

	exists, err := s.cache.Exists(ctx, key)
	duration := time.Since(start)

	if err != nil {
		s.metrics.RecordSessionCacheOp("check_token_blacklist", "error")
		s.logger.Error("Token blacklist check failed",
			zap.String("token_id", tokenID),
			zap.Error(err))
		return false, err
	}

	if exists {
		s.metrics.RecordSessionCacheOp("check_token_blacklist", "blacklisted")
		s.logger.Debug("Token is blacklisted",
			zap.String("token_id", tokenID),
			zap.Duration("latency", duration))
	} else {
		s.metrics.RecordSessionCacheOp("check_token_blacklist", "valid")
	}

	s.metrics.RecordCacheLatency("check_token_blacklist", "success", duration)
	return exists, nil
}

// BlacklistToken adds a JWT token to the blacklist
func (s *SessionCacheService) BlacklistToken(ctx context.Context, tokenID string, ttl time.Duration) error {
	start := time.Now()
	key := s.getBlacklistKey(tokenID)

	// Store minimal data for blacklisted tokens
	blacklistEntry := map[string]interface{}{
		"blacklisted_at": time.Now(),
		"token_id":       tokenID,
	}

	err := s.cache.SetJSON(ctx, key, blacklistEntry, ttl)
	duration := time.Since(start)

	if err != nil {
		s.metrics.RecordSessionCacheOp("blacklist_token", "error")
		s.logger.Error("Token blacklist failed",
			zap.String("token_id", tokenID),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return err
	}

	s.metrics.RecordSessionCacheOp("blacklist_token", "success")
	s.metrics.RecordCacheLatency("blacklist_token", "success", duration)
	s.logger.Info("Token blacklisted",
		zap.String("token_id", tokenID),
		zap.Duration("ttl", ttl),
		zap.Duration("latency", duration))

	return nil
}

// GetSessionsByUserID retrieves all sessions for a specific user
func (s *SessionCacheService) GetSessionsByUserID(ctx context.Context, userID string) ([]*models.Session, error) {
	start := time.Now()

	// Use a simplified approach with a user session index
	indexKey := s.getUserSessionIndexKey(userID)

	var sessionIDs []string
	err := s.cache.GetJSON(ctx, indexKey, &sessionIDs)
	duration := time.Since(start)

	if err != nil {
		if err == ErrKeyNotFound {
			s.metrics.RecordSessionCacheOp("get_sessions_by_user", "miss")
			return []*models.Session{}, nil
		}
		s.metrics.RecordSessionCacheOp("get_sessions_by_user", "error")
		s.logger.Error("User sessions index get failed",
			zap.String("user_id", userID),
			zap.Error(err))
		return nil, err
	}

	// Batch get all sessions for the user
	sessions := make([]*models.Session, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if session, err := s.GetSession(ctx, sessionID); err == nil {
			sessions = append(sessions, session)
		}
	}

	s.metrics.RecordSessionCacheOp("get_sessions_by_user", "success")
	s.metrics.RecordCacheLatency("get_sessions_by_user", "success", duration)
	s.logger.Debug("User sessions retrieved",
		zap.String("user_id", userID),
		zap.Int("session_count", len(sessions)),
		zap.Duration("latency", duration))

	return sessions, nil
}

// InvalidateUserSessions removes all sessions for a specific user
func (s *SessionCacheService) InvalidateUserSessions(ctx context.Context, userID string) error {
	start := time.Now()

	// Get all sessions for the user
	sessions, err := s.GetSessionsByUserID(ctx, userID)
	if err != nil {
		return err
	}

	// Delete all sessions
	sessionKeys := make([]string, len(sessions))
	for i, session := range sessions {
		sessionKeys[i] = s.getSessionKey(session.ID)
	}

	// Also delete the user session index
	indexKey := s.getUserSessionIndexKey(userID)
	sessionKeys = append(sessionKeys, indexKey)

	err = s.cache.MDelete(ctx, sessionKeys)
	duration := time.Since(start)

	if err != nil {
		s.metrics.RecordSessionCacheOp("invalidate_user_sessions", "error")
		s.logger.Error("User sessions invalidation failed",
			zap.String("user_id", userID),
			zap.Error(err))
		return err
	}

	s.metrics.RecordSessionCacheOp("invalidate_user_sessions", "success")
	s.metrics.RecordCacheLatency("invalidate_user_sessions", "success", duration)
	s.logger.Info("User sessions invalidated",
		zap.String("user_id", userID),
		zap.Int("sessions_deleted", len(sessions)),
		zap.Duration("latency", duration))

	return nil
}

// UpdateSessionActivity updates the last accessed time for a session
func (s *SessionCacheService) UpdateSessionActivity(ctx context.Context, sessionID string) error {
	start := time.Now()

	// Get current session
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Update last used time
	session.LastUsed = time.Now()

	// Calculate remaining TTL
	remainingTTL := session.ExpiresAt.Sub(time.Now())
	if remainingTTL <= 0 {
		return s.DeleteSession(ctx, sessionID)
	}

	// Update session with remaining TTL
	err = s.SetSession(ctx, sessionID, session, remainingTTL)
	duration := time.Since(start)

	if err != nil {
		s.metrics.RecordSessionCacheOp("update_session_activity", "error")
		return err
	}

	s.metrics.RecordSessionCacheOp("update_session_activity", "success")
	s.metrics.RecordCacheLatency("update_session_activity", "success", duration)

	return nil
}

// Key generation helpers
func (s *SessionCacheService) getSessionKey(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

func (s *SessionCacheService) getBlacklistKey(tokenID string) string {
	return fmt.Sprintf("blacklist:token:%s", tokenID)
}

func (s *SessionCacheService) getUserSessionPattern(userID string) string {
	return fmt.Sprintf("session:user:%s:*", userID)
}

func (s *SessionCacheService) getUserSessionIndexKey(userID string) string {
	return fmt.Sprintf("session:user:index:%s", userID)
}
