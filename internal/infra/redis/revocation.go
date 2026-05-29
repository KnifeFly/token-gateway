package redis

import (
	"context"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
	goredis "github.com/redis/go-redis/v9"
)

const revocationPrefix = "token-gateway:revoked-api-key:"

// RevocationStore stores fast API key revocations in Redis.
type RevocationStore struct {
	client *goredis.Client
	ttl    time.Duration
}

// NewRevocationStore returns a Redis-backed revocation store.
func NewRevocationStore(client *goredis.Client, ttl time.Duration) *RevocationStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &RevocationStore{client: client, ttl: ttl}
}

// Revoke records a revoked API key hash.
func (s *RevocationStore) Revoke(ctx context.Context, keyHash string) error {
	if s == nil || s.client == nil || keyHash == "" {
		return nil
	}
	if err := s.client.Set(ctx, revocationPrefix+keyHash, "1", s.ttl).Err(); err != nil {
		return apperr.ServiceUnavailable("api key revocation store is unavailable", apperr.WithCause(err), apperr.WithTemporary())
	}
	return nil
}

// IsRevoked checks whether an API key hash is revoked.
func (s *RevocationStore) IsRevoked(ctx context.Context, keyHash string) (bool, error) {
	if s == nil || s.client == nil || keyHash == "" {
		return false, nil
	}
	value, err := s.client.Exists(ctx, revocationPrefix+keyHash).Result()
	if err != nil {
		return false, apperr.ServiceUnavailable("api key revocation store is unavailable", apperr.WithCause(err), apperr.WithTemporary())
	}
	return value > 0, nil
}
