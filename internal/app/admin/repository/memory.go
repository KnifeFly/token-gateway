package repository

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
)

// MemoryRepository stores Admin operators, sessions, and audit events for tests and local development.
type MemoryRepository struct {
	mu        sync.RWMutex
	operators map[string]adminapp.Operator
	sessions  map[string]adminapp.Session
	audit     map[string]adminapp.AuditEvent
}

// NewMemoryRepository returns an empty Admin app repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		operators: map[string]adminapp.Operator{},
		sessions:  map[string]adminapp.Session{},
		audit:     map[string]adminapp.AuditEvent{},
	}
}

// SaveOperator creates or updates one Admin operator.
func (r *MemoryRepository) SaveOperator(_ context.Context, operator adminapp.Operator) (adminapp.Operator, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	if operator.CreatedAt.IsZero() {
		operator.CreatedAt = now
	}
	operator.UpdatedAt = now
	r.operators[operator.ID] = cloneOperator(operator)
	return cloneOperator(operator), nil
}

// GetOperator returns an Admin operator by ID.
func (r *MemoryRepository) GetOperator(_ context.Context, operatorID string) (adminapp.Operator, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	operator, ok := r.operators[strings.TrimSpace(operatorID)]
	return cloneOperator(operator), ok, nil
}

// GetOperatorByEmail returns an Admin operator by normalized email.
func (r *MemoryRepository) GetOperatorByEmail(_ context.Context, email string) (adminapp.Operator, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	email = strings.ToLower(strings.TrimSpace(email))
	for _, operator := range r.operators {
		if strings.ToLower(operator.Email) == email {
			return cloneOperator(operator), true, nil
		}
	}
	return adminapp.Operator{}, false, nil
}

// ListOperators returns all Admin operators ordered by email.
func (r *MemoryRepository) ListOperators(_ context.Context) ([]adminapp.Operator, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	operators := make([]adminapp.Operator, 0, len(r.operators))
	for _, operator := range r.operators {
		operators = append(operators, cloneOperator(operator))
	}
	sort.Slice(operators, func(i, j int) bool { return operators[i].Email < operators[j].Email })
	return operators, nil
}

// DisableOperator marks an Admin operator disabled.
func (r *MemoryRepository) DisableOperator(_ context.Context, operatorID string, disabledAt time.Time) (adminapp.Operator, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	operator, ok := r.operators[strings.TrimSpace(operatorID)]
	if !ok {
		return adminapp.Operator{}, false, nil
	}
	operator.Enabled = false
	operator.UpdatedAt = disabledAt
	r.operators[operator.ID] = operator
	return cloneOperator(operator), true, nil
}

// UpdateOperatorLastLogin records the last successful operator login.
func (r *MemoryRepository) UpdateOperatorLastLogin(_ context.Context, operatorID string, seenAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	operator, ok := r.operators[strings.TrimSpace(operatorID)]
	if !ok {
		return nil
	}
	operator.LastLoginAt = &seenAt
	operator.UpdatedAt = seenAt
	r.operators[operator.ID] = operator
	return nil
}

// CreateSession stores an Admin browser session.
func (r *MemoryRepository) CreateSession(_ context.Context, session adminapp.Session) (adminapp.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[session.ID] = cloneSession(session)
	return cloneSession(session), nil
}

// GetSession returns an Admin browser session by ID.
func (r *MemoryRepository) GetSession(_ context.Context, sessionID string) (adminapp.Session, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, ok := r.sessions[strings.TrimSpace(sessionID)]
	return cloneSession(session), ok, nil
}

// TouchSession records the last seen time for a session.
func (r *MemoryRepository) TouchSession(_ context.Context, sessionID string, seenAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return nil
	}
	session.LastSeenAt = seenAt
	r.sessions[session.ID] = session
	return nil
}

// RevokeSession marks an Admin session revoked.
func (r *MemoryRepository) RevokeSession(_ context.Context, sessionID string, revokedAt time.Time) (adminapp.Session, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return adminapp.Session{}, false, nil
	}
	session.RevokedAt = &revokedAt
	session.LastSeenAt = revokedAt
	r.sessions[session.ID] = session
	return cloneSession(session), true, nil
}

// DeleteSession removes an Admin browser session.
func (r *MemoryRepository) DeleteSession(_ context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.sessions, strings.TrimSpace(sessionID))
	return nil
}

// CreateAuditEvent stores one Admin audit event.
func (r *MemoryRepository) CreateAuditEvent(_ context.Context, event adminapp.AuditEvent) (adminapp.AuditEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.audit[event.ID] = cloneAuditEvent(event)
	return cloneAuditEvent(event), nil
}

// ListAuditEvents returns audit events ordered newest first.
func (r *MemoryRepository) ListAuditEvents(_ context.Context, filter adminapp.AuditFilter) ([]adminapp.AuditEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limit := normalizeLimit(filter.Limit)
	events := make([]adminapp.AuditEvent, 0, len(r.audit))
	for _, event := range r.audit {
		if filter.OperatorID != "" && event.Actor.OperatorID != filter.OperatorID {
			continue
		}
		if filter.Action != "" && event.Action != filter.Action {
			continue
		}
		if filter.Resource != "" && event.Resource != filter.Resource {
			continue
		}
		if !filter.From.IsZero() && event.CreatedAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && event.CreatedAt.After(filter.To) {
			continue
		}
		events = append(events, cloneAuditEvent(event))
	}
	sort.Slice(events, func(i, j int) bool { return events[i].CreatedAt.After(events[j].CreatedAt) })
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func cloneOperator(operator adminapp.Operator) adminapp.Operator {
	operator.Roles = append([]adminapp.Role(nil), operator.Roles...)
	if operator.LastLoginAt != nil {
		lastLoginAt := *operator.LastLoginAt
		operator.LastLoginAt = &lastLoginAt
	}
	return operator
}

func cloneSession(session adminapp.Session) adminapp.Session {
	if session.RevokedAt != nil {
		revokedAt := *session.RevokedAt
		session.RevokedAt = &revokedAt
	}
	return session
}

func cloneAuditEvent(event adminapp.AuditEvent) adminapp.AuditEvent {
	event.Actor.Roles = append([]adminapp.Role(nil), event.Actor.Roles...)
	event.Before = append([]byte(nil), event.Before...)
	event.After = append([]byte(nil), event.After...)
	return event
}
