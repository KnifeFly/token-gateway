ALTER TABLE file_assets
  ADD KEY idx_file_assets_scope_active (tenant_id, project_id, expires_at),
  ADD KEY idx_file_assets_expires (expires_at);
