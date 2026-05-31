ALTER TABLE tasks
  ADD COLUMN price_snapshot_json JSON NULL AFTER balance_hold_id;
