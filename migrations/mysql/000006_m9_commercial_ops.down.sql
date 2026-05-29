DROP INDEX idx_tasks_metadata_report ON tasks;
DROP INDEX idx_failed_settlement_scope_status ON failed_settlements;
DROP INDEX idx_ledger_scope_created ON ledger_entries;
DROP INDEX idx_usage_records_provider_model ON usage_records;
DROP INDEX idx_usage_records_scope_created ON usage_records;

DROP TABLE IF EXISTS cp_model_marketplace;
DROP TABLE IF EXISTS manual_adjustments;
DROP TABLE IF EXISTS provider_cost_profiles;
