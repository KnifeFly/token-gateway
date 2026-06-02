package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
)

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

func cloneOperator(operator adminapp.Operator) adminapp.Operator {
	operator.Roles = append([]adminapp.Role(nil), operator.Roles...)
	if operator.LastLoginAt != nil {
		lastLoginAt := *operator.LastLoginAt
		operator.LastLoginAt = &lastLoginAt
	}
	return operator
}
