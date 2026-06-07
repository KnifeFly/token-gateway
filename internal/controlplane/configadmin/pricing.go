package configadmin

import (
	"context"
	"encoding/json"
)

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

// UpsertPrice creates or updates a model price rule.
func (r *MemoryRepository) UpsertPrice(_ context.Context, price PriceRuleConfig) (*PriceRuleConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prices[price.PublicModel] = price
	return clone(price), nil
}
