ALTER TABLE usage_attempts
  ADD COLUMN retryable BOOLEAN NOT NULL DEFAULT FALSE AFTER actual_output_tokens,
  ADD COLUMN retry_budget_consumed INT NOT NULL DEFAULT 0 AFTER retryable,
  ADD COLUMN retry_budget_remaining INT NOT NULL DEFAULT 0 AFTER retry_budget_consumed,
  ADD COLUMN fallback_from_channel_id VARCHAR(64) NOT NULL DEFAULT '' AFTER retry_budget_remaining,
  ADD COLUMN fallback_from_provider VARCHAR(64) NOT NULL DEFAULT '' AFTER fallback_from_channel_id,
  ADD COLUMN circuit_state VARCHAR(32) NOT NULL DEFAULT '' AFTER fallback_from_provider,
  ADD COLUMN final BOOLEAN NOT NULL DEFAULT FALSE AFTER circuit_state;
