ALTER TABLE cp_limit_rules
  DROP KEY idx_cp_limit_scope,
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (public_model),
  DROP COLUMN cost_per_minute_micros,
  DROP COLUMN daily_budget_micros,
  DROP COLUMN rpm,
  DROP COLUMN channel_id,
  DROP COLUMN provider_type,
  DROP COLUMN api_key_id,
  DROP COLUMN project_id,
  DROP COLUMN tenant_id,
  DROP COLUMN id;

ALTER TABLE cp_models
  DROP COLUMN schema_json,
  DROP COLUMN description,
  DROP COLUMN display_name,
  DROP COLUMN aliases_json;
