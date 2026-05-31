ALTER TABLE usage_attempts
  ADD COLUMN task_id VARCHAR(64) NOT NULL DEFAULT '' AFTER attempt_index,
  ADD COLUMN upstream_model VARCHAR(128) NOT NULL DEFAULT '' AFTER model,
  ADD KEY idx_usage_attempt_task (task_id),
  ADD KEY idx_usage_attempt_final (tenant_id, project_id, final, success, created_at);

ALTER TABLE failed_settlements
  ADD COLUMN owner_id VARCHAR(128) NOT NULL DEFAULT '' AFTER last_error,
  ADD COLUMN claimed_at TIMESTAMP NULL AFTER owner_id,
  ADD COLUMN heartbeat_at TIMESTAMP NULL AFTER claimed_at,
  ADD KEY idx_failed_settlement_claim (status, next_retry_at, heartbeat_at, owner_id);
