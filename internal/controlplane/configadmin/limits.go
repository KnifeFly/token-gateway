package configadmin

import (
	"context"
)

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

// UpsertLimit creates or updates a scoped limit rule.
func (r *MemoryRepository) UpsertLimit(_ context.Context, limit LimitRuleConfig) (*LimitRuleConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit.ID == "" {
		limit.ID = limitRuleID(limit)
	}
	r.limits[limit.ID] = limit
	return clone(limit), nil
}
