package repository

import (
	"context"
	"encoding/json"
	"sort"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
)

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

func cloneAuditEvent(event adminapp.AuditEvent) adminapp.AuditEvent {
	event.Actor.Roles = append([]adminapp.Role(nil), event.Actor.Roles...)
	event.Before = append([]byte(nil), event.Before...)
	event.After = append([]byte(nil), event.After...)
	return event
}
