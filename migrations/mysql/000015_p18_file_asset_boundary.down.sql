ALTER TABLE file_assets
  DROP KEY idx_file_assets_expires,
  DROP KEY idx_file_assets_scope_active;
