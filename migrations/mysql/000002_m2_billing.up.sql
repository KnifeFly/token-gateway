CREATE TABLE IF NOT EXISTS balance_accounts (
  id VARCHAR(64) PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  currency VARCHAR(16) NOT NULL,
  opening_micros BIGINT NOT NULL DEFAULT 0,
  available_micros BIGINT NOT NULL DEFAULT 0,
  held_micros BIGINT NOT NULL DEFAULT 0,
  version BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_balance_account_scope (tenant_id, project_id, currency)
);

CREATE TABLE IF NOT EXISTS balance_holds (
  id VARCHAR(64) PRIMARY KEY,
  request_id VARCHAR(128) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  api_key_id VARCHAR(64) NOT NULL,
  account_id VARCHAR(64) NOT NULL,
  currency VARCHAR(16) NOT NULL,
  amount_micros BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL,
  release_reason VARCHAR(255) NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_balance_holds_request (request_id),
  KEY idx_balance_holds_account (account_id),
  CONSTRAINT fk_balance_holds_account FOREIGN KEY (account_id) REFERENCES balance_accounts(id)
);

CREATE TABLE IF NOT EXISTS usage_attempts (
  id VARCHAR(64) PRIMARY KEY,
  request_id VARCHAR(128) NOT NULL,
  attempt_index INT NOT NULL,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  api_key_id VARCHAR(64) NOT NULL,
  channel_id VARCHAR(64) NOT NULL,
  provider_type VARCHAR(64) NOT NULL,
  model VARCHAR(128) NOT NULL,
  status_code INT NOT NULL DEFAULT 0,
  error_code VARCHAR(128) NOT NULL DEFAULT '',
  success BOOLEAN NOT NULL DEFAULT FALSE,
  estimated_input_tokens BIGINT NOT NULL DEFAULT 0,
  estimated_output_tokens BIGINT NOT NULL DEFAULT 0,
  actual_input_tokens BIGINT NOT NULL DEFAULT 0,
  actual_output_tokens BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_usage_attempt_request_channel (request_id, api_key_id, channel_id, attempt_index)
);

CREATE TABLE IF NOT EXISTS usage_records (
  id VARCHAR(64) PRIMARY KEY,
  request_id VARCHAR(128) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  api_key_id VARCHAR(64) NOT NULL,
  model VARCHAR(128) NOT NULL,
  provider_type VARCHAR(64) NOT NULL,
  channel_id VARCHAR(64) NOT NULL,
  input_tokens BIGINT NOT NULL DEFAULT 0,
  output_tokens BIGINT NOT NULL DEFAULT 0,
  total_tokens BIGINT NOT NULL DEFAULT 0,
  amount_micros BIGINT NOT NULL,
  currency VARCHAR(16) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_usage_records_request (request_id)
);

CREATE TABLE IF NOT EXISTS ledger_entries (
  id VARCHAR(64) PRIMARY KEY,
  request_id VARCHAR(128) NOT NULL,
  settlement_kind VARCHAR(64) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  account_id VARCHAR(64) NOT NULL,
  currency VARCHAR(16) NOT NULL,
  amount_micros BIGINT NOT NULL,
  balance_after_micros BIGINT NOT NULL,
  reason VARCHAR(255) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_ledger_request_kind (request_id, settlement_kind),
  KEY idx_ledger_account (account_id),
  CONSTRAINT fk_ledger_account FOREIGN KEY (account_id) REFERENCES balance_accounts(id)
);

CREATE TABLE IF NOT EXISTS failed_settlements (
  id VARCHAR(64) PRIMARY KEY,
  request_id VARCHAR(128) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  api_key_id VARCHAR(64) NOT NULL,
  hold_id VARCHAR(64) NOT NULL,
  payload_json JSON NOT NULL,
  status VARCHAR(32) NOT NULL,
  retry_count INT NOT NULL DEFAULT 0,
  next_retry_at TIMESTAMP NOT NULL,
  last_error TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_failed_settlement_request (request_id),
  KEY idx_failed_settlement_status_retry (status, next_retry_at)
);
