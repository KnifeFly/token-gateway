package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// MySQLRepository persists control-plane configuration in MySQL.
type MySQLRepository struct {
	db *sql.DB
}

// NewMySQLRepository returns a MySQL control-plane repository.
func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

// UpsertTenant creates or updates a tenant row.
func (r *MySQLRepository) UpsertTenant(ctx context.Context, tenant Tenant) (*Tenant, error) {
	if tenant.ID == "" {
		tenant.ID = newID("tenant")
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cp_tenants (id, name, enabled) VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE name = VALUES(name), enabled = VALUES(enabled), updated_at = CURRENT_TIMESTAMP`,
		tenant.ID, tenant.Name, tenant.Enabled)
	if err != nil {
		return nil, err
	}
	return r.getTenant(ctx, tenant.ID)
}

// UpsertProject creates or updates a project row.
func (r *MySQLRepository) UpsertProject(ctx context.Context, project Project) (*Project, error) {
	if project.ID == "" {
		project.ID = newID("project")
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cp_projects (id, tenant_id, name, enabled) VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE tenant_id = VALUES(tenant_id), name = VALUES(name), enabled = VALUES(enabled), updated_at = CURRENT_TIMESTAMP`,
		project.ID, project.TenantID, project.Name, project.Enabled)
	if err != nil {
		return nil, err
	}
	return r.getProject(ctx, project.ID)
}

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

// UpsertModel creates or updates public model configuration.
func (r *MySQLRepository) UpsertModel(ctx context.Context, model ModelConfig) (*ModelConfig, error) {
	aliases, _ := json.Marshal(model.Aliases)
	tags, _ := json.Marshal(model.Tags)
	modalities, _ := json.Marshal(model.Modalities)
	capabilities, _ := json.Marshal(model.Capabilities)
	aliases = jsonOrDefault(aliases, `[]`)
	tags = jsonOrDefault(tags, `[]`)
	modalities = jsonOrDefault(modalities, `[]`)
	capabilities = jsonOrDefault(capabilities, `[]`)
	schema := []byte(model.Schema)
	if len(schema) == 0 {
		schema = []byte(`{}`)
	}
	metadata := []byte(model.Metadata)
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cp_models (
  public_model, aliases_json, display_name, description, protocol, capability,
  category, tags_json, provider_family, modalities_json, capabilities_json,
  context_window, max_output_tokens, status, deprecated, sort_order,
  metadata_json, schema_json, enabled
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE aliases_json = VALUES(aliases_json), display_name = VALUES(display_name), description = VALUES(description),
  protocol = VALUES(protocol), capability = VALUES(capability), category = VALUES(category), tags_json = VALUES(tags_json),
  provider_family = VALUES(provider_family), modalities_json = VALUES(modalities_json), capabilities_json = VALUES(capabilities_json),
  context_window = VALUES(context_window), max_output_tokens = VALUES(max_output_tokens), status = VALUES(status),
  deprecated = VALUES(deprecated), sort_order = VALUES(sort_order), metadata_json = VALUES(metadata_json),
  schema_json = VALUES(schema_json), enabled = VALUES(enabled),
  updated_at = CURRENT_TIMESTAMP`,
		model.PublicModel, aliases, model.DisplayName, model.Description, model.Protocol, model.Capability,
		model.Category, tags, model.ProviderFamily, modalities, capabilities, model.ContextWindow,
		model.MaxOutputTokens, model.Status, model.Deprecated, model.SortOrder, metadata, schema, model.Enabled)
	if err != nil {
		return nil, err
	}
	return &model, nil
}

// UpsertChannel creates or updates a provider channel and its model mappings.
func (r *MySQLRepository) UpsertChannel(ctx context.Context, channel ChannelConfig) (*ChannelConfig, error) {
	if channel.ID == "" {
		channel.ID = newID("channel")
	}
	if channel.TimeoutMillis == 0 && channel.Timeout > 0 {
		channel.TimeoutMillis = channel.Timeout.Milliseconds()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO cp_channels (id, provider_type, base_url, credential_ref, encrypted_api_key, enabled, timeout_millis)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE provider_type = VALUES(provider_type), base_url = VALUES(base_url), credential_ref = VALUES(credential_ref),
  encrypted_api_key = VALUES(encrypted_api_key), enabled = VALUES(enabled), timeout_millis = VALUES(timeout_millis), updated_at = CURRENT_TIMESTAMP`,
		channel.ID, channel.ProviderType, channel.BaseURL, channel.CredentialRef, channel.EncryptedAPIKey, channel.Enabled, channel.TimeoutMillis); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cp_channel_models WHERE channel_id = ?`, channel.ID); err != nil {
		return nil, err
	}
	for _, model := range channel.Models {
		capabilities, _ := json.Marshal(model.Capabilities)
		parameters, _ := json.Marshal(model.SupportedParameters)
		capabilities = jsonOrDefault(capabilities, `[]`)
		parameters = jsonOrDefault(parameters, `[]`)
		metadata := []byte(model.Metadata)
		if len(metadata) == 0 {
			metadata = []byte(`{}`)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO cp_channel_models (
  channel_id, public_model, upstream_model, capabilities_json,
  supported_parameters_json, health_status, test_status, cost_config_status, metadata_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			channel.ID, model.PublicModel, model.UpstreamModel, capabilities, parameters,
			model.HealthStatus, model.TestStatus, model.CostConfigStatus, metadata); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &channel, nil
}

// UpsertRoute creates or updates a route policy and candidate order.
func (r *MySQLRepository) UpsertRoute(ctx context.Context, route RoutePolicyConfig) (*RoutePolicyConfig, error) {
	if route.ID == "" {
		route.ID = "route_" + route.PublicModel
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO cp_route_policies (id, public_model, strategy, enabled) VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE public_model = VALUES(public_model), strategy = VALUES(strategy), enabled = VALUES(enabled), updated_at = CURRENT_TIMESTAMP`,
		route.ID, route.PublicModel, route.Strategy, route.Enabled); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cp_route_candidates WHERE route_id = ?`, route.ID); err != nil {
		return nil, err
	}
	for _, candidate := range route.Candidates {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO cp_route_candidates (route_id, channel_id, priority, weight) VALUES (?, ?, ?, ?)`,
			route.ID, candidate.ChannelID, candidate.Priority, candidate.Weight); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &route, nil
}

// UpsertPrice creates or updates a model price rule.
func (r *MySQLRepository) UpsertPrice(ctx context.Context, price PriceRuleConfig) (*PriceRuleConfig, error) {
	components, _ := json.Marshal(price.Components)
	components = jsonOrDefault(components, `[]`)
	metadata := []byte(price.Metadata)
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cp_price_rules (
  public_model, category, currency, components_json, input_micros_per_token,
  output_micros_per_token, estimated_output_tokens, metadata_json, enabled
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE category = VALUES(category), currency = VALUES(currency), components_json = VALUES(components_json),
  input_micros_per_token = VALUES(input_micros_per_token),
  output_micros_per_token = VALUES(output_micros_per_token), estimated_output_tokens = VALUES(estimated_output_tokens),
  metadata_json = VALUES(metadata_json), enabled = VALUES(enabled), updated_at = CURRENT_TIMESTAMP`,
		price.PublicModel, price.Category, price.Currency, components, price.InputMicrosPerToken,
		price.OutputMicrosPerToken, price.EstimatedOutputTokens, metadata, price.Enabled)
	if err != nil {
		return nil, err
	}
	return &price, nil
}

// UpsertLimit creates or updates a scoped limit rule.
func (r *MySQLRepository) UpsertLimit(ctx context.Context, limit LimitRuleConfig) (*LimitRuleConfig, error) {
	if limit.ID == "" {
		limit.ID = limitRuleID(limit)
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cp_limit_rules (
  id, tenant_id, project_id, api_key_id, public_model, provider_type, channel_id,
  rpm, qps, tpm, concurrency, daily_budget_micros, cost_per_minute_micros, enabled
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE tenant_id = VALUES(tenant_id), project_id = VALUES(project_id), api_key_id = VALUES(api_key_id),
  public_model = VALUES(public_model), provider_type = VALUES(provider_type), channel_id = VALUES(channel_id),
  rpm = VALUES(rpm), qps = VALUES(qps), tpm = VALUES(tpm), concurrency = VALUES(concurrency),
  daily_budget_micros = VALUES(daily_budget_micros), cost_per_minute_micros = VALUES(cost_per_minute_micros),
  enabled = VALUES(enabled), updated_at = CURRENT_TIMESTAMP`,
		limit.ID, limit.TenantID, limit.ProjectID, limit.APIKeyID, limit.PublicModel, limit.ProviderType, limit.ChannelID,
		limit.RPM, limit.QPS, limit.TPM, limit.Concurrency, limit.DailyBudgetMicros, limit.CostPerMinuteMicros, limit.Enabled)
	if err != nil {
		return nil, err
	}
	return &limit, nil
}

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

// UpsertModelMarketplace creates or updates a tenant-visible catalog row.
func (r *MySQLRepository) UpsertModelMarketplace(ctx context.Context, config ModelMarketplaceConfig) (*ModelMarketplaceConfig, error) {
	if config.ID == "" {
		config.ID = marketplaceID(config)
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cp_model_marketplace (
  id, tenant_id, project_id, public_model, display_name, description,
  enabled, sort_order, metadata_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  display_name = VALUES(display_name),
  description = VALUES(description),
  enabled = VALUES(enabled),
  sort_order = VALUES(sort_order),
  metadata_json = VALUES(metadata_json),
  updated_at = CURRENT_TIMESTAMP`,
		config.ID, config.TenantID, config.ProjectID, config.PublicModel, config.DisplayName,
		config.Description, config.Enabled, config.SortOrder, []byte(config.Metadata),
	)
	if err != nil {
		return nil, err
	}
	return r.getModelMarketplace(ctx, config.ID)
}

// ListVisibleModels returns model catalog rows visible to tenantID and projectID.
func (r *MySQLRepository) ListVisibleModels(ctx context.Context, tenantID, projectID string) ([]VisibleModel, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
  market.id, market.tenant_id, market.project_id, market.public_model,
  market.display_name, market.description, model.protocol, model.capability,
  model.category, model.tags_json, model.provider_family, model.modalities_json,
  model.capabilities_json, model.context_window, model.max_output_tokens,
  model.status, model.deprecated,
  COALESCE(price.currency, ''), COALESCE(price.components_json, JSON_ARRAY()), COALESCE(price.input_micros_per_token, 0),
  COALESCE(price.output_micros_per_token, 0), COALESCE(price.estimated_output_tokens, 0),
  market.sort_order, market.metadata_json
FROM cp_model_marketplace market
JOIN cp_models model ON model.public_model = market.public_model AND model.enabled = TRUE
LEFT JOIN cp_price_rules price ON price.public_model = market.public_model AND price.enabled = TRUE
WHERE market.enabled = TRUE
  AND (market.tenant_id = '' OR market.tenant_id = ?)
  AND (market.project_id = '' OR market.project_id = ?)
ORDER BY market.sort_order, market.public_model, market.tenant_id DESC, market.project_id DESC`, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VisibleModel
	for rows.Next() {
		var model VisibleModel
		var tags, modalities, capabilities, components []byte
		if err := rows.Scan(
			&model.ID, &model.TenantID, &model.ProjectID, &model.PublicModel,
			&model.DisplayName, &model.Description, &model.Protocol, &model.Capability,
			&model.Category, &tags, &model.ProviderFamily, &modalities, &capabilities,
			&model.ContextWindow, &model.MaxOutputTokens, &model.Status, &model.Deprecated,
			&model.Currency, &components, &model.InputMicrosPerToken, &model.OutputMicrosPerToken,
			&model.EstimatedOutputTokens, &model.SortOrder, &model.Metadata,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(tags, &model.Tags)
		_ = json.Unmarshal(modalities, &model.Modalities)
		_ = json.Unmarshal(capabilities, &model.Capabilities)
		_ = json.Unmarshal(components, &model.Components)
		out = append(out, model)
	}
	return out, rows.Err()
}

// LoadSnapshotConfig loads the complete config graph for snapshot building.
func (r *MySQLRepository) LoadSnapshotConfig(ctx context.Context) (*SnapshotConfig, error) {
	cfg := &SnapshotConfig{}
	keys, err := r.ListAPIKeys(ctx, "", "")
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if key.Enabled {
			cfg.APIKeys = append(cfg.APIKeys, key)
		}
		if key.RevokedAt != nil {
			cfg.RevokedKeys = append(cfg.RevokedKeys, key)
		}
	}
	if cfg.Models, err = r.listModels(ctx); err != nil {
		return nil, err
	}
	if cfg.Channels, err = r.listChannels(ctx); err != nil {
		return nil, err
	}
	if cfg.Routes, err = r.listRoutes(ctx); err != nil {
		return nil, err
	}
	if cfg.Prices, err = r.listPrices(ctx); err != nil {
		return nil, err
	}
	if cfg.Limits, err = r.listLimits(ctx); err != nil {
		return nil, err
	}
	if cfg.Plugins, err = r.listPluginBindings(ctx); err != nil {
		return nil, err
	}
	return cfg, nil
}

// SaveSnapshot persists a built runtime snapshot record.
func (r *MySQLRepository) SaveSnapshot(ctx context.Context, record SnapshotRecord) (*SnapshotRecord, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cp_runtime_snapshots (version, checksum, status, payload_json, error, created_at, active_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE checksum = VALUES(checksum), status = VALUES(status), payload_json = VALUES(payload_json), error = VALUES(error), active_at = VALUES(active_at)`,
		record.Version, record.Checksum, record.Status, record.Payload, record.Error, record.CreatedAt, record.ActiveAt)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// ActiveSnapshot returns the currently active snapshot record.
func (r *MySQLRepository) ActiveSnapshot(ctx context.Context) (*SnapshotRecord, bool, error) {
	return r.snapshotByStatus(ctx, SnapshotStatusActive)
}

// PreviousSnapshot returns the most recently inactive snapshot record.
func (r *MySQLRepository) PreviousSnapshot(ctx context.Context) (*SnapshotRecord, bool, error) {
	record, err := scanSnapshot(r.db.QueryRowContext(ctx, `
SELECT version, checksum, status, payload_json, error, created_at, active_at
FROM cp_runtime_snapshots
WHERE status = ?
ORDER BY active_at DESC, created_at DESC
LIMIT 1`, SnapshotStatusInactive))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return record, true, nil
}

// ActivateSnapshot marks version active and demotes any previous active snapshot.
func (r *MySQLRepository) ActivateSnapshot(ctx context.Context, version string) (*SnapshotRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Step 1: demote any active snapshot before promoting the target version.
	if _, err := tx.ExecContext(ctx, `UPDATE cp_runtime_snapshots SET status = ? WHERE status = ?`, SnapshotStatusInactive, SnapshotStatusActive); err != nil {
		return nil, err
	}

	// Step 2: activate the target version inside the same transaction.
	if _, err := tx.ExecContext(ctx, `UPDATE cp_runtime_snapshots SET status = ?, active_at = ? WHERE version = ?`, SnapshotStatusActive, time.Now().UTC(), version); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	record, ok, err := r.ActiveSnapshot(ctx)
	if err != nil || !ok {
		return nil, err
	}
	return record, nil
}

func (r *MySQLRepository) getTenant(ctx context.Context, id string) (*Tenant, error) {
	var tenant Tenant
	err := r.db.QueryRowContext(ctx, `SELECT id, name, enabled, created_at, updated_at FROM cp_tenants WHERE id = ?`, id).
		Scan(&tenant.ID, &tenant.Name, &tenant.Enabled, &tenant.CreatedAt, &tenant.UpdatedAt)
	return &tenant, err
}

func (r *MySQLRepository) getProject(ctx context.Context, id string) (*Project, error) {
	var project Project
	err := r.db.QueryRowContext(ctx, `SELECT id, tenant_id, name, enabled, created_at, updated_at FROM cp_projects WHERE id = ?`, id).
		Scan(&project.ID, &project.TenantID, &project.Name, &project.Enabled, &project.CreatedAt, &project.UpdatedAt)
	return &project, err
}

func (r *MySQLRepository) getAPIKey(ctx context.Context, id string) (*APIKey, error) {
	return scanAPIKey(r.db.QueryRowContext(ctx, `SELECT id, tenant_id, project_id, name, key_hash, enabled, allowed_models_json, revoked_at, created_at, updated_at FROM cp_api_keys WHERE id = ?`, id))
}

func (r *MySQLRepository) getPluginBinding(ctx context.Context, id string) (*PluginBindingConfig, error) {
	return scanPluginBinding(r.db.QueryRowContext(ctx, `
SELECT id, name, phase, tenant_id, project_id, model, priority, enabled, failure_policy, config_json, created_at, updated_at
FROM cp_plugin_bindings
WHERE id = ?`, id))
}

func (r *MySQLRepository) getModelMarketplace(ctx context.Context, id string) (*ModelMarketplaceConfig, error) {
	return scanModelMarketplace(r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, public_model, display_name, description,
       enabled, sort_order, metadata_json, created_at, updated_at
FROM cp_model_marketplace
WHERE id = ?`, id))
}

func (r *MySQLRepository) listModels(ctx context.Context) ([]ModelConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT public_model, aliases_json, display_name, description, protocol, capability,
       category, tags_json, provider_family, modalities_json, capabilities_json,
       context_window, max_output_tokens, status, deprecated, sort_order,
       metadata_json, schema_json, enabled
FROM cp_models
ORDER BY public_model`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var models []ModelConfig
	for rows.Next() {
		var model ModelConfig
		var aliases, tags, modalities, capabilities, metadata, schema []byte
		if err := rows.Scan(
			&model.PublicModel, &aliases, &model.DisplayName, &model.Description, &model.Protocol, &model.Capability,
			&model.Category, &tags, &model.ProviderFamily, &modalities, &capabilities,
			&model.ContextWindow, &model.MaxOutputTokens, &model.Status, &model.Deprecated, &model.SortOrder,
			&metadata, &schema, &model.Enabled,
		); err != nil {
			return nil, err
		}
		if len(aliases) > 0 {
			_ = json.Unmarshal(aliases, &model.Aliases)
		}
		_ = json.Unmarshal(tags, &model.Tags)
		_ = json.Unmarshal(modalities, &model.Modalities)
		_ = json.Unmarshal(capabilities, &model.Capabilities)
		if len(metadata) == 0 {
			metadata = []byte(`{}`)
		}
		model.Metadata = json.RawMessage(metadata)
		if len(schema) == 0 {
			schema = []byte(`{}`)
		}
		model.Schema = json.RawMessage(schema)
		models = append(models, model)
	}
	return models, rows.Err()
}

func (r *MySQLRepository) listChannels(ctx context.Context) ([]ChannelConfig, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, provider_type, base_url, credential_ref, encrypted_api_key, enabled, timeout_millis FROM cp_channels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var channels []ChannelConfig
	for rows.Next() {
		var channel ChannelConfig
		if err := rows.Scan(&channel.ID, &channel.ProviderType, &channel.BaseURL, &channel.CredentialRef, &channel.EncryptedAPIKey, &channel.Enabled, &channel.TimeoutMillis); err != nil {
			return nil, err
		}
		channel.Timeout = time.Duration(channel.TimeoutMillis) * time.Millisecond
		models, err := r.channelModels(ctx, channel.ID)
		if err != nil {
			return nil, err
		}
		channel.Models = models
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

func (r *MySQLRepository) channelModels(ctx context.Context, channelID string) ([]ChannelModel, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT public_model, upstream_model, capabilities_json, supported_parameters_json,
       health_status, test_status, cost_config_status, metadata_json
FROM cp_channel_models
WHERE channel_id = ?
ORDER BY public_model`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var models []ChannelModel
	for rows.Next() {
		var model ChannelModel
		var capabilities, parameters, metadata []byte
		if err := rows.Scan(
			&model.PublicModel, &model.UpstreamModel, &capabilities, &parameters,
			&model.HealthStatus, &model.TestStatus, &model.CostConfigStatus, &metadata,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(capabilities, &model.Capabilities)
		_ = json.Unmarshal(parameters, &model.SupportedParameters)
		if len(metadata) == 0 {
			metadata = []byte(`{}`)
		}
		model.Metadata = json.RawMessage(metadata)
		models = append(models, model)
	}
	return models, rows.Err()
}

func (r *MySQLRepository) listRoutes(ctx context.Context) ([]RoutePolicyConfig, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, public_model, strategy, enabled FROM cp_route_policies ORDER BY public_model`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var routes []RoutePolicyConfig
	for rows.Next() {
		var route RoutePolicyConfig
		if err := rows.Scan(&route.ID, &route.PublicModel, &route.Strategy, &route.Enabled); err != nil {
			return nil, err
		}
		candidates, err := r.routeCandidates(ctx, route.ID)
		if err != nil {
			return nil, err
		}
		route.Candidates = candidates
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

func (r *MySQLRepository) routeCandidates(ctx context.Context, routeID string) ([]RouteCandidate, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT channel_id, priority, weight FROM cp_route_candidates WHERE route_id = ? ORDER BY priority, channel_id`, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []RouteCandidate
	for rows.Next() {
		var candidate RouteCandidate
		if err := rows.Scan(&candidate.ChannelID, &candidate.Priority, &candidate.Weight); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func (r *MySQLRepository) listPrices(ctx context.Context) ([]PriceRuleConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT public_model, category, currency, components_json, input_micros_per_token,
       output_micros_per_token, estimated_output_tokens, metadata_json, enabled
FROM cp_price_rules`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var prices []PriceRuleConfig
	for rows.Next() {
		var price PriceRuleConfig
		var components, metadata []byte
		if err := rows.Scan(
			&price.PublicModel, &price.Category, &price.Currency, &components, &price.InputMicrosPerToken,
			&price.OutputMicrosPerToken, &price.EstimatedOutputTokens, &metadata, &price.Enabled,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(components, &price.Components)
		if len(metadata) == 0 {
			metadata = []byte(`{}`)
		}
		price.Metadata = json.RawMessage(metadata)
		prices = append(prices, price)
	}
	return prices, rows.Err()
}

func (r *MySQLRepository) listLimits(ctx context.Context) ([]LimitRuleConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, tenant_id, project_id, api_key_id, public_model, provider_type, channel_id,
       rpm, qps, tpm, concurrency, daily_budget_micros, cost_per_minute_micros, enabled
FROM cp_limit_rules`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var limits []LimitRuleConfig
	for rows.Next() {
		var limit LimitRuleConfig
		if err := rows.Scan(&limit.ID, &limit.TenantID, &limit.ProjectID, &limit.APIKeyID, &limit.PublicModel, &limit.ProviderType, &limit.ChannelID,
			&limit.RPM, &limit.QPS, &limit.TPM, &limit.Concurrency, &limit.DailyBudgetMicros, &limit.CostPerMinuteMicros, &limit.Enabled); err != nil {
			return nil, err
		}
		normalizeLimitBudgetAlias(&limit)
		limits = append(limits, limit)
	}
	return limits, rows.Err()
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

func (r *MySQLRepository) snapshotByStatus(ctx context.Context, status string) (*SnapshotRecord, bool, error) {
	record, err := scanSnapshot(r.db.QueryRowContext(ctx, `
SELECT version, checksum, status, payload_json, error, created_at, active_at
FROM cp_runtime_snapshots
WHERE status = ?
ORDER BY active_at DESC, created_at DESC
LIMIT 1`, status))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return record, true, nil
}

type rowScanner interface {
	Scan(dest ...any) error
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

func scanSnapshot(row rowScanner) (*SnapshotRecord, error) {
	var record SnapshotRecord
	var activeAt sql.NullTime
	err := row.Scan(&record.Version, &record.Checksum, &record.Status, &record.Payload, &record.Error, &record.CreatedAt, &activeAt)
	if err != nil {
		return nil, err
	}
	if activeAt.Valid {
		record.ActiveAt = &activeAt.Time
	}
	return &record, nil
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

func scanModelMarketplace(row rowScanner) (*ModelMarketplaceConfig, error) {
	var config ModelMarketplaceConfig
	var metadata []byte
	err := row.Scan(
		&config.ID, &config.TenantID, &config.ProjectID, &config.PublicModel,
		&config.DisplayName, &config.Description, &config.Enabled, &config.SortOrder,
		&metadata, &config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	config.Metadata = json.RawMessage(metadata)
	return &config, nil
}

func jsonOrDefault(data []byte, fallback string) []byte {
	if len(data) == 0 || string(data) == "null" {
		return []byte(fallback)
	}
	return data
}
