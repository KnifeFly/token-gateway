package configadmin

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
)

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

func (r *MySQLRepository) getModelMarketplace(ctx context.Context, id string) (*ModelMarketplaceConfig, error) {
	return scanModelMarketplace(r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, public_model, display_name, description,
       enabled, sort_order, metadata_json, created_at, updated_at
FROM cp_model_marketplace
WHERE id = ?`, id))
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

// UpsertModelMarketplace creates or updates a tenant-visible catalog row.
func (r *MemoryRepository) UpsertModelMarketplace(_ context.Context, config ModelMarketplaceConfig) (*ModelMarketplaceConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if config.ID == "" {
		config.ID = marketplaceID(config)
	}
	if existing, ok := r.market[config.ID]; ok {
		config.CreatedAt = existing.CreatedAt
	} else {
		config.CreatedAt = now
	}
	config.UpdatedAt = now
	r.market[config.ID] = config
	return clone(config), nil
}

// ListVisibleModels returns enabled catalog rows visible to tenantID and projectID.
func (r *MemoryRepository) ListVisibleModels(_ context.Context, tenantID, projectID string) ([]VisibleModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []VisibleModel
	for _, config := range r.market {
		if !config.Enabled || !marketplaceScopeMatches(config, tenantID, projectID) {
			continue
		}
		model := r.models[config.PublicModel]
		if !model.Enabled {
			continue
		}
		price := r.prices[config.PublicModel]
		out = append(out, VisibleModel{
			ID:                    config.ID,
			TenantID:              config.TenantID,
			ProjectID:             config.ProjectID,
			PublicModel:           config.PublicModel,
			DisplayName:           config.DisplayName,
			Description:           config.Description,
			Protocol:              model.Protocol,
			Capability:            model.Capability,
			Category:              model.Category,
			Tags:                  append([]string(nil), model.Tags...),
			ProviderFamily:        model.ProviderFamily,
			Modalities:            append([]string(nil), model.Modalities...),
			Capabilities:          append([]string(nil), model.Capabilities...),
			ContextWindow:         model.ContextWindow,
			MaxOutputTokens:       model.MaxOutputTokens,
			Status:                model.Status,
			Deprecated:            model.Deprecated,
			Currency:              price.Currency,
			Components:            append([]pricing.Component(nil), price.Components...),
			InputMicrosPerToken:   price.InputMicrosPerToken,
			OutputMicrosPerToken:  price.OutputMicrosPerToken,
			EstimatedOutputTokens: price.EstimatedOutputTokens,
			SortOrder:             config.SortOrder,
			Metadata:              append([]byte(nil), config.Metadata...),
		})
	}
	return out, nil
}

func marketplaceID(config ModelMarketplaceConfig) string {
	base := "market_" + config.TenantID + "_" + config.ProjectID + "_" + config.PublicModel
	return strings.Trim(pluginBindingIDRe.ReplaceAllString(base, "_"), "_")
}

func marketplaceScopeMatches(config ModelMarketplaceConfig, tenantID, projectID string) bool {
	if config.TenantID != "" && config.TenantID != tenantID {
		return false
	}
	if config.ProjectID != "" && config.ProjectID != projectID {
		return false
	}
	return true
}
