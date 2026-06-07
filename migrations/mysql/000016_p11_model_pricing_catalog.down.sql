ALTER TABLE provider_cost_profiles
  DROP COLUMN components_json,
  DROP COLUMN category;

ALTER TABLE cp_price_rules
  DROP COLUMN metadata_json,
  DROP COLUMN components_json,
  DROP COLUMN category;

ALTER TABLE cp_channel_models
  DROP COLUMN metadata_json,
  DROP COLUMN cost_config_status,
  DROP COLUMN test_status,
  DROP COLUMN health_status,
  DROP COLUMN supported_parameters_json,
  DROP COLUMN capabilities_json;

ALTER TABLE cp_models
  DROP COLUMN metadata_json,
  DROP COLUMN sort_order,
  DROP COLUMN deprecated,
  DROP COLUMN status,
  DROP COLUMN max_output_tokens,
  DROP COLUMN context_window,
  DROP COLUMN capabilities_json,
  DROP COLUMN modalities_json,
  DROP COLUMN provider_family,
  DROP COLUMN tags_json,
  DROP COLUMN category;
