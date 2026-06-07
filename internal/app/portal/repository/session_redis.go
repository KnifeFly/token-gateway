package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	goredis "github.com/redis/go-redis/v9"
)

// RedisSessionStore stores Portal sessions in Redis.
type RedisSessionStore struct {
	client *goredis.Client
	prefix string
}

// NewRedisSessionStore returns a Redis-backed Portal session store.
func NewRedisSessionStore(client *goredis.Client, prefix string) *RedisSessionStore {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "token-gateway"
	}
	return &RedisSessionStore{client: client, prefix: prefix + ":portal:sessions"}
}

// Create stores a session until its expiry.
func (s *RedisSessionStore) Create(ctx context.Context, session portalapp.Session) (portalapp.Session, error) {
	if s == nil || s.client == nil {
		return session, nil
	}
	content, err := json.Marshal(session)
	if err != nil {
		return portalapp.Session{}, err
	}
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Minute
	}
	if err := s.client.Set(ctx, s.key(session.ID), content, ttl).Err(); err != nil {
		return portalapp.Session{}, err
	}
	return session, nil
}

// Get returns a session by ID.
func (s *RedisSessionStore) Get(ctx context.Context, sessionID string) (portalapp.Session, bool, error) {
	if s == nil || s.client == nil {
		return portalapp.Session{}, false, nil
	}
	content, err := s.client.Get(ctx, s.key(sessionID)).Bytes()
	if err == goredis.Nil {
		return portalapp.Session{}, false, nil
	}
	if err != nil {
		return portalapp.Session{}, false, err
	}
	var session portalapp.Session
	if err := json.Unmarshal(content, &session); err != nil {
		return portalapp.Session{}, false, err
	}
	return session, true, nil
}

// Touch records the last seen time for a session.
func (s *RedisSessionStore) Touch(ctx context.Context, sessionID string, seenAt time.Time) error {
	session, ok, err := s.Get(ctx, sessionID)
	if err != nil || !ok {
		return err
	}
	session.LastSeenAt = seenAt
	_, err = s.Create(ctx, session)
	return err
}

// Revoke marks a session revoked.
func (s *RedisSessionStore) Revoke(ctx context.Context, sessionID string, revokedAt time.Time) (portalapp.Session, bool, error) {
	session, ok, err := s.Get(ctx, sessionID)
	if err != nil || !ok {
		return portalapp.Session{}, ok, err
	}
	session.RevokedAt = &revokedAt
	session.LastSeenAt = revokedAt
	if _, err := s.Create(ctx, session); err != nil {
		return portalapp.Session{}, false, err
	}
	return session, true, nil
}

// RevokeByScope marks all matching tenant/project/API key sessions revoked.
func (s *RedisSessionStore) RevokeByScope(ctx context.Context, tenantID string, projectID string, apiKeyID string, revokedAt time.Time) (int, error) {
	if s == nil || s.client == nil {
		return 0, nil
	}
	count := 0
	cursor := uint64(0)
	for {
		keys, next, err := s.client.Scan(ctx, cursor, s.prefix+":*", 100).Result()
		if err != nil {
			return count, err
		}
		for _, key := range keys {
			content, err := s.client.Get(ctx, key).Bytes()
			if err == goredis.Nil {
				continue
			}
			if err != nil {
				return count, err
			}
			var session portalapp.Session
			if err := json.Unmarshal(content, &session); err != nil {
				return count, err
			}
			if !sessionMatchesScope(session, tenantID, projectID, apiKeyID) {
				continue
			}
			session.RevokedAt = &revokedAt
			session.LastSeenAt = revokedAt
			if _, err := s.Create(ctx, session); err != nil {
				return count, err
			}
			count++
		}
		cursor = next
		if cursor == 0 {
			return count, nil
		}
	}
}

// Delete removes a session.
func (s *RedisSessionStore) Delete(ctx context.Context, sessionID string) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Del(ctx, s.key(sessionID)).Err()
}

func (s *RedisSessionStore) key(sessionID string) string {
	return s.prefix + ":" + strings.TrimSpace(sessionID)
}
