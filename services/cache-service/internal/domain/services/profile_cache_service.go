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

// ProfileCacheService implements profile-specific caching operations
// Supports the expected interface from profile-service integration
type ProfileCacheService struct {
	cache   *CacheService
	logger  *zap.Logger
	metrics *metrics.Metrics
	config  *config.CacheConfig
}

// NewProfileCacheService creates a new profile cache service
func NewProfileCacheService(
	cacheService *CacheService,
	logger *zap.Logger,
	metrics *metrics.Metrics,
	config *config.CacheConfig,
) *ProfileCacheService {
	return &ProfileCacheService{
		cache:   cacheService,
		logger:  logger,
		metrics: metrics,
		config:  config,
	}
}

// GetProfile retrieves a profile from cache by profile ID
func (p *ProfileCacheService) GetProfile(ctx context.Context, profileID string) (*models.Profile, error) {
	start := time.Now()
	key := p.getProfileKey(profileID)

	var profile models.Profile
	err := p.cache.GetJSON(ctx, key, &profile)
	duration := time.Since(start)

	if err != nil {
		if err == ErrKeyNotFound {
			p.metrics.RecordProfileCacheOp("get_profile", "miss")
			p.logger.Debug("Profile cache miss", zap.String("profile_id", profileID))
		} else {
			p.metrics.RecordProfileCacheOp("get_profile", "error")
			p.logger.Error("Profile cache get failed",
				zap.String("profile_id", profileID),
				zap.Error(err))
		}
		return nil, err
	}

	p.metrics.RecordProfileCacheOp("get_profile", "hit")
	p.metrics.RecordCacheLatency("get_profile", "hit", duration)
	p.logger.Debug("Profile cache hit",
		zap.String("profile_id", profileID),
		zap.Duration("latency", duration))

	return &profile, nil
}

// SetProfile stores a profile in cache with profile-specific TTL
func (p *ProfileCacheService) SetProfile(ctx context.Context, profileID string, profile *models.Profile, ttl time.Duration) error {
	start := time.Now()
	key := p.getProfileKey(profileID)

	// Use profile-specific TTL if not specified
	if ttl <= 0 {
		ttl = p.config.ProfileTTL
	}

	err := p.cache.SetJSON(ctx, key, profile, ttl)
	duration := time.Since(start)

	if err != nil {
		p.metrics.RecordProfileCacheOp("set_profile", "error")
		p.logger.Error("Profile cache set failed",
			zap.String("profile_id", profileID),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return err
	}

	p.metrics.RecordProfileCacheOp("set_profile", "success")
	p.metrics.RecordCacheLatency("set_profile", "success", duration)
	p.logger.Debug("Profile cached",
		zap.String("profile_id", profileID),
		zap.Duration("ttl", ttl),
		zap.Duration("latency", duration))

	return nil
}

// GetProfileByEmail retrieves a profile from cache by email address
func (p *ProfileCacheService) GetProfileByEmail(ctx context.Context, email string) (*models.Profile, error) {
	start := time.Now()
	key := p.getProfileEmailKey(email)

	var profile models.Profile
	err := p.cache.GetJSON(ctx, key, &profile)
	duration := time.Since(start)

	if err != nil {
		if err == ErrKeyNotFound {
			p.metrics.RecordProfileCacheOp("get_profile_email", "miss")
			p.logger.Debug("Profile email cache miss", zap.String("email", email))
		} else {
			p.metrics.RecordProfileCacheOp("get_profile_email", "error")
			p.logger.Error("Profile email cache get failed",
				zap.String("email", email),
				zap.Error(err))
		}
		return nil, err
	}

	p.metrics.RecordProfileCacheOp("get_profile_email", "hit")
	p.metrics.RecordCacheLatency("get_profile_email", "hit", duration)
	p.logger.Debug("Profile email cache hit",
		zap.String("email", email),
		zap.Duration("latency", duration))

	return &profile, nil
}

// SetProfileByEmail stores a profile in cache indexed by email
func (p *ProfileCacheService) SetProfileByEmail(ctx context.Context, email string, profile *models.Profile, ttl time.Duration) error {
	start := time.Now()
	key := p.getProfileEmailKey(email)

	// Use profile-specific TTL if not specified (shorter for email lookups)
	if ttl <= 0 {
		ttl = p.config.ProfileTTL / 2 // Email lookups get shorter TTL
	}

	err := p.cache.SetJSON(ctx, key, profile, ttl)
	duration := time.Since(start)

	if err != nil {
		p.metrics.RecordProfileCacheOp("set_profile_email", "error")
		p.logger.Error("Profile email cache set failed",
			zap.String("email", email),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return err
	}

	p.metrics.RecordProfileCacheOp("set_profile_email", "success")
	p.metrics.RecordCacheLatency("set_profile_email", "success", duration)
	p.logger.Debug("Profile cached by email",
		zap.String("email", email),
		zap.Duration("ttl", ttl),
		zap.Duration("latency", duration))

	return nil
}

// InvalidateProfile removes a profile from cache (both by ID and email)
func (p *ProfileCacheService) InvalidateProfile(ctx context.Context, profileID string, email string) error {
	start := time.Now()
	keys := []string{
		p.getProfileKey(profileID),
	}

	// Add email key if provided
	if email != "" {
		keys = append(keys, p.getProfileEmailKey(email))
	}

	err := p.cache.MDelete(ctx, keys)
	duration := time.Since(start)

	if err != nil {
		p.metrics.RecordProfileCacheOp("invalidate_profile", "error")
		p.logger.Error("Profile cache invalidation failed",
			zap.String("profile_id", profileID),
			zap.String("email", email),
			zap.Error(err))
		return err
	}

	p.metrics.RecordProfileCacheOp("invalidate_profile", "success")
	p.metrics.RecordCacheLatency("invalidate_profile", "success", duration)
	p.logger.Info("Profile cache invalidated",
		zap.String("profile_id", profileID),
		zap.String("email", email),
		zap.Int("keys_deleted", len(keys)),
		zap.Duration("latency", duration))

	return nil
}

// GetProfileMetadata retrieves profile metadata from cache
func (p *ProfileCacheService) GetProfileMetadata(ctx context.Context, profileID string) (map[string]interface{}, error) {
	start := time.Now()
	key := p.getProfileMetadataKey(profileID)

	var metadata map[string]interface{}
	err := p.cache.GetJSON(ctx, key, &metadata)
	duration := time.Since(start)

	if err != nil {
		if err == ErrKeyNotFound {
			p.metrics.RecordProfileCacheOp("get_profile_metadata", "miss")
		} else {
			p.metrics.RecordProfileCacheOp("get_profile_metadata", "error")
			p.logger.Error("Profile metadata cache get failed",
				zap.String("profile_id", profileID),
				zap.Error(err))
		}
		return nil, err
	}

	p.metrics.RecordProfileCacheOp("get_profile_metadata", "hit")
	p.metrics.RecordCacheLatency("get_profile_metadata", "hit", duration)

	return metadata, nil
}

// SetProfileMetadata stores profile metadata in cache
func (p *ProfileCacheService) SetProfileMetadata(ctx context.Context, profileID string, metadata map[string]interface{}, ttl time.Duration) error {
	start := time.Now()
	key := p.getProfileMetadataKey(profileID)

	// Use longer TTL for metadata (less frequently changed)
	if ttl <= 0 {
		ttl = p.config.ProfileTTL * 2
	}

	err := p.cache.SetJSON(ctx, key, metadata, ttl)
	duration := time.Since(start)

	if err != nil {
		p.metrics.RecordProfileCacheOp("set_profile_metadata", "error")
		p.logger.Error("Profile metadata cache set failed",
			zap.String("profile_id", profileID),
			zap.Error(err))
		return err
	}

	p.metrics.RecordProfileCacheOp("set_profile_metadata", "success")
	p.metrics.RecordCacheLatency("set_profile_metadata", "success", duration)

	return nil
}

// BatchGetProfiles retrieves multiple profiles efficiently
func (p *ProfileCacheService) BatchGetProfiles(ctx context.Context, profileIDs []string) (map[string]*models.Profile, error) {
	start := time.Now()

	if len(profileIDs) == 0 {
		return make(map[string]*models.Profile), nil
	}

	// Build cache keys
	keys := make([]string, len(profileIDs))
	keyToID := make(map[string]string)

	for i, profileID := range profileIDs {
		key := p.getProfileKey(profileID)
		keys[i] = key
		keyToID[key] = profileID
	}

	// Batch get from cache
	values, err := p.cache.MGet(ctx, keys)
	duration := time.Since(start)

	if err != nil {
		p.metrics.RecordProfileCacheOp("batch_get_profiles", "error")
		p.logger.Error("Profile batch get failed",
			zap.Int("profile_count", len(profileIDs)),
			zap.Error(err))
		return nil, err
	}

	// Parse results
	profiles := make(map[string]*models.Profile)
	hits := 0

	for key := range values {
		profileID := keyToID[key]
		var profile models.Profile

		if err := p.cache.GetJSON(ctx, key, &profile); err == nil {
			profiles[profileID] = &profile
			hits++
		}
	}

	misses := len(profileIDs) - hits
	p.metrics.RecordProfileCacheOp("batch_get_profiles", "success")
	p.metrics.RecordCacheLatency("batch_get_profiles", "success", duration)

	p.logger.Debug("Profile batch get completed",
		zap.Int("requested", len(profileIDs)),
		zap.Int("hits", hits),
		zap.Int("misses", misses),
		zap.Duration("latency", duration))

	return profiles, nil
}

// Key generation helpers
func (p *ProfileCacheService) getProfileKey(profileID string) string {
	return fmt.Sprintf("profile:%s", profileID)
}

func (p *ProfileCacheService) getProfileEmailKey(email string) string {
	return fmt.Sprintf("profile:email:%s", email)
}

func (p *ProfileCacheService) getProfileMetadataKey(profileID string) string {
	return fmt.Sprintf("profile:meta:%s", profileID)
}
