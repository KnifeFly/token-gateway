ALTER TABLE cp_api_keys
  ADD COLUMN ip_allowlist_json JSON NULL AFTER allowed_models_json,
  ADD COLUMN expires_at TIMESTAMP NULL AFTER ip_allowlist_json,
  ADD COLUMN last_used_at TIMESTAMP NULL AFTER expires_at;
