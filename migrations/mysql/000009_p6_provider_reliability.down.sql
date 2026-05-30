ALTER TABLE usage_attempts
  DROP COLUMN final,
  DROP COLUMN circuit_state,
  DROP COLUMN fallback_from_provider,
  DROP COLUMN fallback_from_channel_id,
  DROP COLUMN retry_budget_remaining,
  DROP COLUMN retry_budget_consumed,
  DROP COLUMN retryable;
