package configadmin

import (
	"context"
	"encoding/json"
)

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

// UpsertModel creates or updates public model configuration.
func (r *MemoryRepository) UpsertModel(_ context.Context, model ModelConfig) (*ModelConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(model.Schema) == 0 {
		model.Schema = json.RawMessage(`{}`)
	}
	r.models[model.PublicModel] = model
	return clone(model), nil
}
