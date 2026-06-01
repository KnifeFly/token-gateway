package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
)

// MySQLRepository stores Admin operators, sessions, and audit events in MySQL.
type MySQLRepository struct {
	db *sql.DB
}

// NewMySQLRepository returns a MySQL-backed Admin app repository.
func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

// SaveOperator creates or updates one Admin operator.
func (r *MySQLRepository) SaveOperator(ctx context.Context, operator adminapp.Operator) (adminapp.Operator, error) {
	roles, _ := json.Marshal(operator.Roles)
	_, err := r.db.ExecContext(ctx, `
INSERT INTO admin_operators (id, email, display_name, password_hash, roles_json, enabled, last_login_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE email = VALUES(email), display_name = VALUES(display_name),
  password_hash = VALUES(password_hash), roles_json = VALUES(roles_json), enabled = VALUES(enabled),
  last_login_at = VALUES(last_login_at), updated_at = CURRENT_TIMESTAMP`,
		operator.ID, strings.ToLower(strings.TrimSpace(operator.Email)), operator.DisplayName, operator.PasswordHash,
		jsonOrDefault(roles, `[]`), operator.Enabled, operator.LastLoginAt)
	if err != nil {
		return adminapp.Operator{}, err
	}
	return r.GetOperatorMust(ctx, operator.ID)
}

// GetOperatorMust returns an Admin operator by ID or the query error.
func (r *MySQLRepository) GetOperatorMust(ctx context.Context, operatorID string) (adminapp.Operator, error) {
	operator, _, err := r.GetOperator(ctx, operatorID)
	return operator, err
}

// GetOperator returns an Admin operator by ID.
func (r *MySQLRepository) GetOperator(ctx context.Context, operatorID string) (adminapp.Operator, bool, error) {
	operator, err := scanOperator(r.db.QueryRowContext(ctx, `
SELECT id, email, display_name, password_hash, roles_json, enabled, last_login_at, created_at, updated_at
FROM admin_operators
WHERE id = ?`, strings.TrimSpace(operatorID)))
	if err == sql.ErrNoRows {
		return adminapp.Operator{}, false, nil
	}
	if err != nil {
		return adminapp.Operator{}, false, err
	}
	return operator, true, nil
}

// GetOperatorByEmail returns an Admin operator by normalized email.
func (r *MySQLRepository) GetOperatorByEmail(ctx context.Context, email string) (adminapp.Operator, bool, error) {
	operator, err := scanOperator(r.db.QueryRowContext(ctx, `
SELECT id, email, display_name, password_hash, roles_json, enabled, last_login_at, created_at, updated_at
FROM admin_operators
WHERE email = ?`, strings.ToLower(strings.TrimSpace(email))))
	if err == sql.ErrNoRows {
		return adminapp.Operator{}, false, nil
	}
	if err != nil {
		return adminapp.Operator{}, false, err
	}
	return operator, true, nil
}

// ListOperators returns all Admin operators ordered by email.
func (r *MySQLRepository) ListOperators(ctx context.Context) ([]adminapp.Operator, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, email, display_name, password_hash, roles_json, enabled, last_login_at, created_at, updated_at
FROM admin_operators
ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var operators []adminapp.Operator
	for rows.Next() {
		operator, err := scanOperator(rows)
		if err != nil {
			return nil, err
		}
		operators = append(operators, operator)
	}
	return operators, rows.Err()
}

// DisableOperator marks an Admin operator disabled.
func (r *MySQLRepository) DisableOperator(ctx context.Context, operatorID string, _ time.Time) (adminapp.Operator, bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE admin_operators SET enabled = FALSE, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, strings.TrimSpace(operatorID))
	if err != nil {
		return adminapp.Operator{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return adminapp.Operator{}, false, err
	}
	if affected == 0 {
		return adminapp.Operator{}, false, nil
	}
	operator, ok, err := r.GetOperator(ctx, operatorID)
	return operator, ok, err
}

// UpdateOperatorLastLogin records the last successful operator login.
func (r *MySQLRepository) UpdateOperatorLastLogin(ctx context.Context, operatorID string, seenAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE admin_operators SET last_login_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, seenAt, strings.TrimSpace(operatorID))
	return err
}

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

// CreateAuditEvent stores one Admin audit event.
func (r *MySQLRepository) CreateAuditEvent(ctx context.Context, event adminapp.AuditEvent) (adminapp.AuditEvent, error) {
	roles, _ := json.Marshal(event.Actor.Roles)
	_, err := r.db.ExecContext(ctx, `
INSERT INTO admin_audit_events (
  id, operator_id, operator_email, roles_json, action, resource, resource_id, request_id,
  idempotency_key, reason, status, error_code, before_json, after_json, remote_addr, user_agent_hash, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.Actor.OperatorID, event.Actor.Email, jsonOrDefault(roles, `[]`), event.Action, event.Resource,
		event.ResourceID, event.RequestID, event.IdempotencyKey, event.Reason, event.Status, event.ErrorCode,
		[]byte(event.Before), []byte(event.After), event.RemoteAddr, event.UserAgentHash, event.CreatedAt)
	if err != nil {
		return adminapp.AuditEvent{}, err
	}
	return event, nil
}

// ListAuditEvents returns audit events ordered newest first.
func (r *MySQLRepository) ListAuditEvents(ctx context.Context, filter adminapp.AuditFilter) ([]adminapp.AuditEvent, error) {
	limit := normalizeLimit(filter.Limit)
	query := `
SELECT id, operator_id, operator_email, roles_json, action, resource, resource_id, request_id,
       idempotency_key, reason, status, error_code, before_json, after_json, remote_addr, user_agent_hash, created_at
FROM admin_audit_events
WHERE 1=1`
	var args []any
	if filter.OperatorID != "" {
		query += ` AND operator_id = ?`
		args = append(args, filter.OperatorID)
	}
	if filter.Action != "" {
		query += ` AND action = ?`
		args = append(args, filter.Action)
	}
	if filter.Resource != "" {
		query += ` AND resource = ?`
		args = append(args, filter.Resource)
	}
	if !filter.From.IsZero() {
		query += ` AND created_at >= ?`
		args = append(args, filter.From)
	}
	if !filter.To.IsZero() {
		query += ` AND created_at <= ?`
		args = append(args, filter.To)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []adminapp.AuditEvent
	for rows.Next() {
		event, err := scanAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanOperator(row rowScanner) (adminapp.Operator, error) {
	var operator adminapp.Operator
	var roles []byte
	var lastLoginAt sql.NullTime
	err := row.Scan(&operator.ID, &operator.Email, &operator.DisplayName, &operator.PasswordHash, &roles, &operator.Enabled, &lastLoginAt, &operator.CreatedAt, &operator.UpdatedAt)
	if err != nil {
		return adminapp.Operator{}, err
	}
	if len(roles) > 0 {
		_ = json.Unmarshal(roles, &operator.Roles)
	}
	if lastLoginAt.Valid {
		operator.LastLoginAt = &lastLoginAt.Time
	}
	return operator, nil
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

func scanAuditEvent(row rowScanner) (adminapp.AuditEvent, error) {
	var event adminapp.AuditEvent
	var roles []byte
	err := row.Scan(&event.ID, &event.Actor.OperatorID, &event.Actor.Email, &roles, &event.Action, &event.Resource,
		&event.ResourceID, &event.RequestID, &event.IdempotencyKey, &event.Reason, &event.Status,
		&event.ErrorCode, &event.Before, &event.After, &event.RemoteAddr, &event.UserAgentHash, &event.CreatedAt)
	if err != nil {
		return adminapp.AuditEvent{}, err
	}
	_ = json.Unmarshal(roles, &event.Actor.Roles)
	return event, nil
}

func jsonOrDefault(data []byte, fallback string) []byte {
	if len(data) == 0 || string(data) == "null" {
		return []byte(fallback)
	}
	return data
}
