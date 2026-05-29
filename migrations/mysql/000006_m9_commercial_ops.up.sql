CREATE TABLE IF NOT EXISTS provider_cost_profiles (
  id VARCHAR(64) PRIMARY KEY,
  provider_type VARCHAR(64) NOT NULL,
  channel_id VARCHAR(64) NOT NULL DEFAULT '',
  public_model VARCHAR(128) NOT NULL,
  currency VARCHAR(16) NOT NULL,
  input_micros_per_token BIGINT NOT NULL DEFAULT 0,
  output_micros_per_token BIGINT NOT NULL DEFAULT 0,
  fixed_micros_per_request BIGINT NOT NULL DEFAULT 0,
  effective_from TIMESTAMP NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_provider_cost_scope (provider_type, channel_id, public_model, currency),
  KEY idx_provider_cost_model (public_model, enabled)
);

CREATE TABLE IF NOT EXISTS manual_adjustments (
  id VARCHAR(64) PRIMARY KEY,
  idempotency_key VARCHAR(128) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  account_id VARCHAR(64) NOT NULL,
  ledger_entry_id VARCHAR(64) NOT NULL,
  currency VARCHAR(16) NOT NULL,
  amount_micros BIGINT NOT NULL,
  reason VARCHAR(255) NOT NULL,
  operator_id VARCHAR(128) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_manual_adjustments_idempotency (idempotency_key),
  KEY idx_manual_adjustments_scope (tenant_id, project_id, created_at),
  CONSTRAINT fk_manual_adjustment_account FOREIGN KEY (account_id) REFERENCES balance_accounts(id),
  CONSTRAINT fk_manual_adjustment_ledger FOREIGN KEY (ledger_entry_id) REFERENCES ledger_entries(id)
);

CREATE TABLE IF NOT EXISTS cp_model_marketplace (
  id VARCHAR(128) PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL DEFAULT '',
  project_id VARCHAR(64) NOT NULL DEFAULT '',
  public_model VARCHAR(128) NOT NULL,
  display_name VARCHAR(255) NOT NULL,
  description TEXT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort_order INT NOT NULL DEFAULT 100,
  metadata_json JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_model_marketplace_scope (tenant_id, project_id, public_model),
  KEY idx_model_marketplace_visible (tenant_id, project_id, enabled, sort_order)
);

CREATE INDEX idx_usage_records_scope_created ON usage_records (tenant_id, project_id, created_at);
CREATE INDEX idx_usage_records_provider_model ON usage_records (provider_type, channel_id, model, created_at);
CREATE INDEX idx_ledger_scope_created ON ledger_entries (tenant_id, project_id, created_at);
CREATE INDEX idx_failed_settlement_scope_status ON failed_settlements (tenant_id, project_id, status, updated_at);
CREATE INDEX idx_tasks_metadata_report ON tasks (tenant_id, project_id, kind, status, created_at);
