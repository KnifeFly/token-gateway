ALTER TABLE cp_models
  ADD COLUMN aliases_json JSON NULL AFTER public_model,
  ADD COLUMN display_name VARCHAR(255) NOT NULL DEFAULT '' AFTER aliases_json,
  ADD COLUMN description TEXT NULL AFTER display_name,
  ADD COLUMN schema_json JSON NULL AFTER capability;

UPDATE cp_models
SET aliases_json = JSON_ARRAY(),
    display_name = public_model,
    schema_json = JSON_OBJECT();

ALTER TABLE cp_models
  MODIFY aliases_json JSON NOT NULL,
  MODIFY schema_json JSON NOT NULL;

ALTER TABLE cp_limit_rules
  ADD COLUMN id VARCHAR(160) NULL FIRST,
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT '' AFTER id,
  ADD COLUMN project_id VARCHAR(64) NOT NULL DEFAULT '' AFTER tenant_id,
  ADD COLUMN api_key_id VARCHAR(64) NOT NULL DEFAULT '' AFTER project_id,
  ADD COLUMN provider_type VARCHAR(64) NOT NULL DEFAULT '' AFTER public_model,
  ADD COLUMN channel_id VARCHAR(64) NOT NULL DEFAULT '' AFTER provider_type,
  ADD COLUMN rpm BIGINT NOT NULL DEFAULT 0 AFTER channel_id,
  ADD COLUMN daily_budget_micros BIGINT NOT NULL DEFAULT 0 AFTER concurrency,
  ADD COLUMN cost_per_minute_micros BIGINT NOT NULL DEFAULT 0 AFTER daily_budget_micros;

UPDATE cp_limit_rules
SET id = CONCAT('limit_', public_model)
WHERE id IS NULL OR id = '';

ALTER TABLE cp_limit_rules
  MODIFY id VARCHAR(160) NOT NULL,
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (id),
  ADD KEY idx_cp_limit_scope (tenant_id, project_id, api_key_id, public_model, provider_type, channel_id);
