CREATE TABLE IF NOT EXISTS admin_operators (
  id VARCHAR(64) PRIMARY KEY,
  email VARCHAR(255) NOT NULL,
  display_name VARCHAR(255) NOT NULL DEFAULT '',
  password_hash VARCHAR(255) NOT NULL,
  roles_json JSON NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  last_login_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_admin_operators_email (email),
  KEY idx_admin_operators_enabled (enabled)
);

CREATE TABLE IF NOT EXISTS admin_sessions (
  id VARCHAR(128) PRIMARY KEY,
  operator_id VARCHAR(64) NOT NULL,
  csrf_hash VARCHAR(128) NOT NULL,
  user_agent_hash VARCHAR(128) NOT NULL DEFAULT '',
  remote_addr VARCHAR(255) NOT NULL DEFAULT '',
  expires_at TIMESTAMP NOT NULL,
  revoked_at TIMESTAMP NULL,
  last_seen_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_admin_sessions_operator (operator_id),
  KEY idx_admin_sessions_expires (expires_at),
  CONSTRAINT fk_admin_sessions_operator FOREIGN KEY (operator_id) REFERENCES admin_operators (id)
);

CREATE TABLE IF NOT EXISTS admin_audit_events (
  id VARCHAR(64) PRIMARY KEY,
  operator_id VARCHAR(64) NOT NULL,
  operator_email VARCHAR(255) NOT NULL DEFAULT '',
  roles_json JSON NOT NULL,
  action VARCHAR(64) NOT NULL,
  resource VARCHAR(64) NOT NULL,
  resource_id VARCHAR(255) NOT NULL DEFAULT '',
  request_id VARCHAR(128) NOT NULL,
  idempotency_key VARCHAR(255) NOT NULL DEFAULT '',
  reason VARCHAR(1024) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  error_code VARCHAR(64) NOT NULL DEFAULT '',
  before_json JSON NULL,
  after_json JSON NULL,
  remote_addr VARCHAR(255) NOT NULL DEFAULT '',
  user_agent_hash VARCHAR(128) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_admin_audit_events_operator_created (operator_id, created_at),
  KEY idx_admin_audit_events_action_resource (action, resource, created_at),
  KEY idx_admin_audit_events_request_id (request_id)
);
