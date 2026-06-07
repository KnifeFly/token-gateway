ALTER TABLE cp_models
  ADD COLUMN category VARCHAR(64) NOT NULL DEFAULT 'chat' AFTER capability,
  ADD COLUMN tags_json JSON NULL AFTER category,
  ADD COLUMN provider_family VARCHAR(128) NOT NULL DEFAULT '' AFTER tags_json,
  ADD COLUMN modalities_json JSON NULL AFTER provider_family,
  ADD COLUMN capabilities_json JSON NULL AFTER modalities_json,
  ADD COLUMN context_window BIGINT NOT NULL DEFAULT 0 AFTER capabilities_json,
  ADD COLUMN max_output_tokens BIGINT NOT NULL DEFAULT 0 AFTER context_window,
  ADD COLUMN status VARCHAR(32) NOT NULL DEFAULT 'active' AFTER max_output_tokens,
  ADD COLUMN deprecated BOOLEAN NOT NULL DEFAULT FALSE AFTER status,
  ADD COLUMN sort_order INT NOT NULL DEFAULT 100 AFTER deprecated,
  ADD COLUMN metadata_json JSON NULL AFTER sort_order;

UPDATE cp_models
SET tags_json = JSON_ARRAY(),
    modalities_json = JSON_ARRAY(),
    capabilities_json = JSON_ARRAY(),
    metadata_json = JSON_OBJECT()
WHERE tags_json IS NULL
   OR modalities_json IS NULL
   OR capabilities_json IS NULL
   OR metadata_json IS NULL;

ALTER TABLE cp_models
  MODIFY tags_json JSON NOT NULL,
  MODIFY modalities_json JSON NOT NULL,
  MODIFY capabilities_json JSON NOT NULL,
  MODIFY metadata_json JSON NOT NULL;

ALTER TABLE cp_channel_models
  ADD COLUMN capabilities_json JSON NULL AFTER upstream_model,
  ADD COLUMN supported_parameters_json JSON NULL AFTER capabilities_json,
  ADD COLUMN health_status VARCHAR(32) NOT NULL DEFAULT 'unknown' AFTER supported_parameters_json,
  ADD COLUMN test_status VARCHAR(32) NOT NULL DEFAULT 'untested' AFTER health_status,
  ADD COLUMN cost_config_status VARCHAR(32) NOT NULL DEFAULT 'unknown' AFTER test_status,
  ADD COLUMN metadata_json JSON NULL AFTER cost_config_status;

UPDATE cp_channel_models
SET capabilities_json = JSON_ARRAY(),
    supported_parameters_json = JSON_ARRAY(),
    metadata_json = JSON_OBJECT()
WHERE capabilities_json IS NULL
   OR supported_parameters_json IS NULL
   OR metadata_json IS NULL;

ALTER TABLE cp_channel_models
  MODIFY capabilities_json JSON NOT NULL,
  MODIFY supported_parameters_json JSON NOT NULL,
  MODIFY metadata_json JSON NOT NULL;

ALTER TABLE cp_price_rules
  ADD COLUMN category VARCHAR(64) NOT NULL DEFAULT 'chat' AFTER public_model,
  ADD COLUMN components_json JSON NULL AFTER currency,
  ADD COLUMN metadata_json JSON NULL AFTER estimated_output_tokens;

UPDATE cp_price_rules
SET components_json = JSON_ARRAY(
      JSON_OBJECT('unit', 'input_token', 'micros_per_unit', input_micros_per_token),
      JSON_OBJECT('unit', 'output_token', 'micros_per_unit', output_micros_per_token)
    ),
    metadata_json = JSON_OBJECT()
WHERE components_json IS NULL
   OR metadata_json IS NULL;

ALTER TABLE cp_price_rules
  MODIFY components_json JSON NOT NULL,
  MODIFY metadata_json JSON NOT NULL;

ALTER TABLE provider_cost_profiles
  ADD COLUMN category VARCHAR(64) NOT NULL DEFAULT 'chat' AFTER public_model,
  ADD COLUMN components_json JSON NULL AFTER currency;

UPDATE provider_cost_profiles
SET components_json = JSON_ARRAY(
      JSON_OBJECT('unit', 'input_token', 'micros_per_unit', input_micros_per_token),
      JSON_OBJECT('unit', 'output_token', 'micros_per_unit', output_micros_per_token),
      JSON_OBJECT('unit', 'request', 'micros_per_unit', fixed_micros_per_request)
    )
WHERE components_json IS NULL;

ALTER TABLE provider_cost_profiles
  MODIFY components_json JSON NOT NULL;
