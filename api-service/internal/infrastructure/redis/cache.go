package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/fernandobarroso/microservices/api-service/internal/config"
	"github.com/fernandobarroso/microservices/api-service/internal/domain/profile"
)

// Cache provides profile-specific caching operations
type Cache struct {
	client    *Client
	profileTTL time.Duration
	listTTL    time.Duration
}

func NewCache(client *Client, cfg config.CacheConfig) *Cache {
	return &Cache{
		client:    client,
		profileTTL: cfg.ProfileTTL,
		listTTL:    cfg.ListTTL,
	}
}

func (c *Cache) GetProfile(ctx context.Context, id uuid.UUID) (*profile.Profile, error) {
	key := fmt.Sprintf("profile:%s", id.String())
	data, err := c.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var p profile.Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile: %w", err)
	}
	return &p, nil
}

func (c *Cache) SetProfile(ctx context.Context, p *profile.Profile) error {
	key := fmt.Sprintf("profile:%s", p.ID.String())
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}
	return c.client.Set(ctx, key, data, c.profileTTL)
}

func (c *Cache) InvalidateProfile(ctx context.Context, id uuid.UUID) error {
	return c.client.Delete(ctx, fmt.Sprintf("profile:%s", id.String()))
}

func (c *Cache) GetProfileList(ctx context.Context, page int) ([]*profile.Profile, error) {
	key := fmt.Sprintf("profiles:list:%d", page)
	data, err := c.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var profiles []*profile.Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile list: %w", err)
	}
	return profiles, nil
}

func (c *Cache) SetProfileList(ctx context.Context, page int, profiles []*profile.Profile) error {
	key := fmt.Sprintf("profiles:list:%d", page)
	data, err := json.Marshal(profiles)
	if err != nil {
		return fmt.Errorf("failed to marshal profile list: %w", err)
	}
	return c.client.Set(ctx, key, data, c.listTTL)
}

