package configadmin

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// CreateAPIKey stores a hashed API key row.
func (r *MySQLRepository) CreateAPIKey(ctx context.Context, key APIKey) (*APIKey, error) {
	if key.ID == "" {
		key.ID = newID("key")
	}
	allowed, _ := json.Marshal(key.AllowedModels)
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cp_api_keys (id, tenant_id, project_id, name, key_hash, enabled, allowed_models_json)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		key.ID, key.TenantID, key.ProjectID, key.Name, key.KeyHash, key.Enabled, allowed)
	if err != nil {
		return nil, err
	}
	return r.getAPIKey(ctx, key.ID)
}

// ListAPIKeys returns API key metadata for the requested scope.
func (r *MySQLRepository) ListAPIKeys(ctx context.Context, tenantID, projectID string) ([]APIKey, error) {
	query := `SELECT id, tenant_id, project_id, name, key_hash, enabled, allowed_models_json, revoked_at, created_at, updated_at FROM cp_api_keys WHERE 1=1`
	var args []any
	if tenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, tenantID)
	}
	if projectID != "" {
		query += ` AND project_id = ?`
		args = append(args, projectID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []APIKey
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, *key)
	}
	return keys, rows.Err()
}

// DisableAPIKey disables an API key and records revocation time.
func (r *MySQLRepository) DisableAPIKey(ctx context.Context, keyID string, revokedAt *time.Time) (*APIKey, error) {
	_, err := r.db.ExecContext(ctx, `UPDATE cp_api_keys SET enabled = FALSE, revoked_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, revokedAt, keyID)
	if err != nil {
		return nil, err
	}
	return r.getAPIKey(ctx, keyID)
}

func (r *MySQLRepository) getAPIKey(ctx context.Context, id string) (*APIKey, error) {
	return scanAPIKey(r.db.QueryRowContext(ctx, `SELECT id, tenant_id, project_id, name, key_hash, enabled, allowed_models_json, revoked_at, created_at, updated_at FROM cp_api_keys WHERE id = ?`, id))
}

func scanAPIKey(row rowScanner) (*APIKey, error) {
	var key APIKey
	var allowed []byte
	var revokedAt sql.NullTime
	err := row.Scan(&key.ID, &key.TenantID, &key.ProjectID, &key.Name, &key.KeyHash, &key.Enabled, &allowed, &revokedAt, &key.CreatedAt, &key.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(allowed) > 0 {
		_ = json.Unmarshal(allowed, &key.AllowedModels)
	}
	if revokedAt.Valid {
		key.RevokedAt = &revokedAt.Time
	}
	return &key, nil
}

// CreateAPIKey stores a hashed API key record.
func (r *MemoryRepository) CreateAPIKey(_ context.Context, key APIKey) (*APIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if key.ID == "" {
		key.ID = newID("key")
	}
	key.CreatedAt = now
	key.UpdatedAt = now
	r.apiKeys[key.ID] = key
	return clone(key), nil
}

// ListAPIKeys returns safe API key metadata for a tenant or project scope.
func (r *MemoryRepository) ListAPIKeys(_ context.Context, tenantID, projectID string) ([]APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]APIKey, 0, len(r.apiKeys))
	for _, key := range r.apiKeys {
		if tenantID != "" && key.TenantID != tenantID {
			continue
		}
		if projectID != "" && key.ProjectID != projectID {
			continue
		}
		key.PlaintextKey = ""
		keys = append(keys, key)
	}
	return keys, nil
}

// DisableAPIKey disables a stored API key and records revocation time.
func (r *MemoryRepository) DisableAPIKey(_ context.Context, keyID string, revokedAt *time.Time) (*APIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.apiKeys[keyID]
	key.Enabled = false
	key.RevokedAt = revokedAt
	key.UpdatedAt = time.Now().UTC()
	r.apiKeys[keyID] = key
	return clone(key), nil
}
