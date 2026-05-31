ALTER TABLE callback_outbox
  DROP KEY idx_callback_outbox_claim,
  DROP COLUMN last_latency_ms,
  DROP COLUMN last_status_code,
  DROP COLUMN delivery_id,
  DROP COLUMN heartbeat_at,
  DROP COLUMN claimed_at,
  DROP COLUMN owner_id;
