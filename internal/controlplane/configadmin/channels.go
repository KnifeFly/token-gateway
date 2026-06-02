package configadmin

import (
	"context"
	"encoding/json"
	"time"
)

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

// UpsertChannel creates or updates provider channel configuration.
func (r *MemoryRepository) UpsertChannel(_ context.Context, channel ChannelConfig) (*ChannelConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[channel.ID] = channel
	return clone(channel), nil
}
