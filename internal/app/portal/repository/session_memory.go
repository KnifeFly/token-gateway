package repository

import (
	"context"
	"sync"
	"time"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
)

// MemorySessionStore stores Portal sessions for local development and tests.
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]portalapp.Session
}

// NewMemorySessionStore returns an in-memory Portal session store.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{sessions: map[string]portalapp.Session{}}
}

// Create stores a session.
func (s *MemorySessionStore) Create(_ context.Context, session portalapp.Session) (portalapp.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.ID] = cloneSession(session)
	return cloneSession(session), nil
}

// Get returns a session by ID.
func (s *MemorySessionStore) Get(_ context.Context, sessionID string) (portalapp.Session, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	return cloneSession(session), ok, nil
}

// Touch records the last seen time for a session.
func (s *MemorySessionStore) Touch(_ context.Context, sessionID string, seenAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil
	}
	session.LastSeenAt = seenAt
	s.sessions[sessionID] = session
	return nil
}

// Revoke marks a session revoked.
func (s *MemorySessionStore) Revoke(_ context.Context, sessionID string, revokedAt time.Time) (portalapp.Session, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return portalapp.Session{}, false, nil
	}
	session.RevokedAt = &revokedAt
	s.sessions[sessionID] = session
	return cloneSession(session), true, nil
}

// RevokeByScope marks all matching tenant/project/API key sessions revoked.
func (s *MemorySessionStore) RevokeByScope(_ context.Context, tenantID string, projectID string, apiKeyID string, revokedAt time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for id, session := range s.sessions {
		if !sessionMatchesScope(session, tenantID, projectID, apiKeyID) {
			continue
		}
		session.RevokedAt = &revokedAt
		session.LastSeenAt = revokedAt
		s.sessions[id] = session
		count++
	}
	return count, nil
}

// Delete removes a session.
func (s *MemorySessionStore) Delete(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
	return nil
}

func cloneSession(session portalapp.Session) portalapp.Session {
	session.AllowedModels = append([]string(nil), session.AllowedModels...)
	if session.RevokedAt != nil {
		revokedAt := *session.RevokedAt
		session.RevokedAt = &revokedAt
	}
	return session
}

func sessionMatchesScope(session portalapp.Session, tenantID string, projectID string, apiKeyID string) bool {
	if tenantID != "" && session.TenantID != tenantID {
		return false
	}
	if projectID != "" && session.ProjectID != projectID {
		return false
	}
	if apiKeyID != "" && session.APIKeyID != apiKeyID {
		return false
	}
	return session.RevokedAt == nil
}
