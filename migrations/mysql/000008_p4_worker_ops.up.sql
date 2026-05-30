ALTER TABLE balance_holds
  ADD KEY idx_balance_holds_expiry (status, expires_at);
