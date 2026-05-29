CREATE TABLE IF NOT EXISTS cp_plugin_bindings (
  id VARCHAR(128) PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  phase VARCHAR(64) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL DEFAULT '',
  project_id VARCHAR(64) NOT NULL DEFAULT '',
  model VARCHAR(128) NOT NULL DEFAULT '',
  priority INT NOT NULL DEFAULT 100,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  failure_policy VARCHAR(32) NOT NULL DEFAULT 'fail_closed',
  config_json JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_cp_plugin_bindings_phase (phase, enabled, priority),
  KEY idx_cp_plugin_bindings_scope (tenant_id, project_id, model)
);
