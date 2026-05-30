package redis

import (
	"context"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// EmergencyDisableStore stores hot provider/channel disables outside snapshots.
type EmergencyDisableStore struct {
	client *goredis.Client
	prefix string
}

// NewEmergencyDisableStore returns a Redis-backed emergency disable store.
func NewEmergencyDisableStore(client *goredis.Client, prefix string) *EmergencyDisableStore {
	if prefix == "" {
		prefix = "token-gateway"
	}
	return &EmergencyDisableStore{client: client, prefix: prefix}
}

// DisableProvider disables a provider type on the hot path.
func (s *EmergencyDisableStore) DisableProvider(ctx context.Context, providerType string, ttl time.Duration) error {
	return s.set(ctx, "provider", providerType, ttl)
}

// EnableProvider removes a hot provider disable.
func (s *EmergencyDisableStore) EnableProvider(ctx context.Context, providerType string) error {
	return s.del(ctx, "provider", providerType)
}

// DisableChannel disables a channel on the hot path.
func (s *EmergencyDisableStore) DisableChannel(ctx context.Context, channelID string, ttl time.Duration) error {
	return s.set(ctx, "channel", channelID, ttl)
}

// EnableChannel removes a hot channel disable.
func (s *EmergencyDisableStore) EnableChannel(ctx context.Context, channelID string) error {
	return s.del(ctx, "channel", channelID)
}

// IsProviderDisabled reports whether providerType is hot-disabled.
func (s *EmergencyDisableStore) IsProviderDisabled(ctx context.Context, providerType string) (bool, error) {
	return s.exists(ctx, "provider", providerType)
}

// IsChannelDisabled reports whether channelID is hot-disabled.
func (s *EmergencyDisableStore) IsChannelDisabled(ctx context.Context, channelID string) (bool, error) {
	return s.exists(ctx, "channel", channelID)
}

func (s *EmergencyDisableStore) set(ctx context.Context, kind, value string, ttl time.Duration) error {
	if s == nil || s.client == nil || strings.TrimSpace(value) == "" {
		return nil
	}
	return s.client.Set(ctx, s.key(kind, value), "1", ttl).Err()
}

func (s *EmergencyDisableStore) del(ctx context.Context, kind, value string) error {
	if s == nil || s.client == nil || strings.TrimSpace(value) == "" {
		return nil
	}
	return s.client.Del(ctx, s.key(kind, value)).Err()
}

func (s *EmergencyDisableStore) exists(ctx context.Context, kind, value string) (bool, error) {
	if s == nil || s.client == nil || strings.TrimSpace(value) == "" {
		return false, nil
	}
	n, err := s.client.Exists(ctx, s.key(kind, value)).Result()
	return n > 0, err
}

func (s *EmergencyDisableStore) key(kind, value string) string {
	return s.prefix + ":emergency_disable:" + kind + ":" + strings.TrimSpace(value)
}
