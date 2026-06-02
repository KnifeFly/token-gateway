package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
)

// CreateSession stores an Admin browser session.
func (r *MySQLRepository) CreateSession(ctx context.Context, session adminapp.Session) (adminapp.Session, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO admin_sessions (id, operator_id, csrf_hash, user_agent_hash, remote_addr, expires_at, revoked_at, last_seen_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE csrf_hash = VALUES(csrf_hash), user_agent_hash = VALUES(user_agent_hash),
  remote_addr = VALUES(remote_addr), expires_at = VALUES(expires_at), revoked_at = VALUES(revoked_at),
  last_seen_at = VALUES(last_seen_at), updated_at = VALUES(updated_at)`,
		session.ID, session.OperatorID, session.CSRFHash, session.UserAgentHash, session.RemoteAddr,
		session.ExpiresAt, session.RevokedAt, session.LastSeenAt, session.CreatedAt, session.LastSeenAt)
	if err != nil {
		return adminapp.Session{}, err
	}
	return session, nil
}

// GetSession returns an Admin browser session by ID.
func (r *MySQLRepository) GetSession(ctx context.Context, sessionID string) (adminapp.Session, bool, error) {
	session, err := scanSession(r.db.QueryRowContext(ctx, `
SELECT id, operator_id, csrf_hash, user_agent_hash, remote_addr, expires_at, revoked_at, last_seen_at, created_at
FROM admin_sessions
WHERE id = ?`, strings.TrimSpace(sessionID)))
	if err == sql.ErrNoRows {
		return adminapp.Session{}, false, nil
	}
	if err != nil {
		return adminapp.Session{}, false, err
	}
	return session, true, nil
}

// TouchSession records the last seen time for a session.
func (r *MySQLRepository) TouchSession(ctx context.Context, sessionID string, seenAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE admin_sessions SET last_seen_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, seenAt, strings.TrimSpace(sessionID))
	return err
}

// RevokeSession marks an Admin session revoked.
func (r *MySQLRepository) RevokeSession(ctx context.Context, sessionID string, revokedAt time.Time) (adminapp.Session, bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE admin_sessions SET revoked_at = ?, last_seen_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, revokedAt, revokedAt, strings.TrimSpace(sessionID))
	if err != nil {
		return adminapp.Session{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return adminapp.Session{}, false, err
	}
	if affected == 0 {
		return adminapp.Session{}, false, nil
	}
	session, ok, err := r.GetSession(ctx, sessionID)
	return session, ok, err
}

// DeleteSession removes an Admin browser session.
func (r *MySQLRepository) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE id = ?`, strings.TrimSpace(sessionID))
	return err
}

func scanSession(row rowScanner) (adminapp.Session, error) {
	var session adminapp.Session
	var revokedAt sql.NullTime
	err := row.Scan(&session.ID, &session.OperatorID, &session.CSRFHash, &session.UserAgentHash, &session.RemoteAddr,
		&session.ExpiresAt, &revokedAt, &session.LastSeenAt, &session.CreatedAt)
	if err != nil {
		return adminapp.Session{}, err
	}
	if revokedAt.Valid {
		session.RevokedAt = &revokedAt.Time
	}
	return session, nil
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

func cloneSession(session adminapp.Session) adminapp.Session {
	if session.RevokedAt != nil {
		revokedAt := *session.RevokedAt
		session.RevokedAt = &revokedAt
	}
	return session
}
