ALTER TABLE cp_api_keys
  DROP COLUMN last_used_at,
  DROP COLUMN expires_at,
  DROP COLUMN ip_allowlist_json;
