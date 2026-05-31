ALTER TABLE failed_settlements
  DROP KEY idx_failed_settlement_claim,
  DROP COLUMN heartbeat_at,
  DROP COLUMN claimed_at,
  DROP COLUMN owner_id;

ALTER TABLE usage_attempts
  DROP KEY idx_usage_attempt_final,
  DROP KEY idx_usage_attempt_task,
  DROP COLUMN upstream_model,
  DROP COLUMN task_id;
