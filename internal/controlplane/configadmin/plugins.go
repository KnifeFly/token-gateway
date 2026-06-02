package configadmin

import (
	"context"
	"encoding/json"
	"time"
)

// UpsertPluginBinding creates or updates a built-in plugin binding.
func (r *MySQLRepository) UpsertPluginBinding(ctx context.Context, binding PluginBindingConfig) (*PluginBindingConfig, error) {
	if binding.ID == "" {
		binding.ID = newID("plugin")
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cp_plugin_bindings (id, name, phase, tenant_id, project_id, model, priority, enabled, failure_policy, config_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE name = VALUES(name), phase = VALUES(phase), tenant_id = VALUES(tenant_id),
  project_id = VALUES(project_id), model = VALUES(model), priority = VALUES(priority), enabled = VALUES(enabled),
  failure_policy = VALUES(failure_policy), config_json = VALUES(config_json), updated_at = CURRENT_TIMESTAMP`,
		binding.ID, binding.Name, binding.Phase, binding.TenantID, binding.ProjectID, binding.Model, binding.Priority, binding.Enabled, binding.FailurePolicy, []byte(binding.Config))
	if err != nil {
		return nil, err
	}
	return r.getPluginBinding(ctx, binding.ID)
}

func (r *MySQLRepository) getPluginBinding(ctx context.Context, id string) (*PluginBindingConfig, error) {
	return scanPluginBinding(r.db.QueryRowContext(ctx, `
SELECT id, name, phase, tenant_id, project_id, model, priority, enabled, failure_policy, config_json, created_at, updated_at
FROM cp_plugin_bindings
WHERE id = ?`, id))
}

func (r *MySQLRepository) listPluginBindings(ctx context.Context) ([]PluginBindingConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, phase, tenant_id, project_id, model, priority, enabled, failure_policy, config_json, created_at, updated_at
FROM cp_plugin_bindings
ORDER BY phase, priority, name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bindings []PluginBindingConfig
	for rows.Next() {
		binding, err := scanPluginBinding(rows)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, *binding)
	}
	return bindings, rows.Err()
}

func scanPluginBinding(row rowScanner) (*PluginBindingConfig, error) {
	var binding PluginBindingConfig
	var config []byte
	err := row.Scan(&binding.ID, &binding.Name, &binding.Phase, &binding.TenantID, &binding.ProjectID, &binding.Model,
		&binding.Priority, &binding.Enabled, &binding.FailurePolicy, &config, &binding.CreatedAt, &binding.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(config) == 0 {
		config = []byte(`{}`)
	}
	binding.Config = json.RawMessage(config)
	return &binding, nil
}

// UpsertPluginBinding creates or updates a plugin binding.
func (r *MemoryRepository) UpsertPluginBinding(_ context.Context, binding PluginBindingConfig) (*PluginBindingConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if binding.ID == "" {
		binding.ID = newID("plugin")
	}
	if existing, ok := r.plugins[binding.ID]; ok {
		binding.CreatedAt = existing.CreatedAt
	} else {
		binding.CreatedAt = now
	}
	binding.UpdatedAt = now
	r.plugins[binding.ID] = binding
	return clone(binding), nil
}
