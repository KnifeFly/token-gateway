ALTER TABLE callback_outbox
  ADD COLUMN owner_id VARCHAR(128) NOT NULL DEFAULT '' AFTER last_error,
  ADD COLUMN claimed_at TIMESTAMP NULL AFTER owner_id,
  ADD COLUMN heartbeat_at TIMESTAMP NULL AFTER claimed_at,
  ADD COLUMN delivery_id VARCHAR(128) NOT NULL DEFAULT '' AFTER heartbeat_at,
  ADD COLUMN last_status_code INT NOT NULL DEFAULT 0 AFTER delivery_id,
  ADD COLUMN last_latency_ms BIGINT NOT NULL DEFAULT 0 AFTER last_status_code,
  ADD KEY idx_callback_outbox_claim (status, next_retry_at, heartbeat_at, owner_id);
