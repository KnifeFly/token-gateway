package configadmin

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// LoadSnapshotConfig loads the complete config graph for snapshot building.
func (r *MySQLRepository) LoadSnapshotConfig(ctx context.Context) (*SnapshotConfig, error) {
	cfg := &SnapshotConfig{}
	keys, err := r.ListAPIKeys(ctx, "", "")
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if key.Enabled {
			cfg.APIKeys = append(cfg.APIKeys, key)
		}
		if key.RevokedAt != nil {
			cfg.RevokedKeys = append(cfg.RevokedKeys, key)
		}
	}
	if cfg.Models, err = r.listModels(ctx); err != nil {
		return nil, err
	}
	if cfg.Channels, err = r.listChannels(ctx); err != nil {
		return nil, err
	}
	if cfg.Routes, err = r.listRoutes(ctx); err != nil {
		return nil, err
	}
	if cfg.Prices, err = r.listPrices(ctx); err != nil {
		return nil, err
	}
	if cfg.Limits, err = r.listLimits(ctx); err != nil {
		return nil, err
	}
	if cfg.Plugins, err = r.listPluginBindings(ctx); err != nil {
		return nil, err
	}
	return cfg, nil
}

// SaveSnapshot persists a built runtime snapshot record.
func (r *MySQLRepository) SaveSnapshot(ctx context.Context, record SnapshotRecord) (*SnapshotRecord, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cp_runtime_snapshots (version, checksum, status, payload_json, error, created_at, active_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE checksum = VALUES(checksum), status = VALUES(status), payload_json = VALUES(payload_json), error = VALUES(error), active_at = VALUES(active_at)`,
		record.Version, record.Checksum, record.Status, record.Payload, record.Error, record.CreatedAt, record.ActiveAt)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// ActiveSnapshot returns the currently active snapshot record.
func (r *MySQLRepository) ActiveSnapshot(ctx context.Context) (*SnapshotRecord, bool, error) {
	return r.snapshotByStatus(ctx, SnapshotStatusActive)
}

// PreviousSnapshot returns the most recently inactive snapshot record.
func (r *MySQLRepository) PreviousSnapshot(ctx context.Context) (*SnapshotRecord, bool, error) {
	record, err := scanSnapshot(r.db.QueryRowContext(ctx, `
SELECT version, checksum, status, payload_json, error, created_at, active_at
FROM cp_runtime_snapshots
WHERE status = ?
ORDER BY active_at DESC, created_at DESC
LIMIT 1`, SnapshotStatusInactive))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return record, true, nil
}

// ActivateSnapshot marks version active and demotes any previous active snapshot.
func (r *MySQLRepository) ActivateSnapshot(ctx context.Context, version string) (*SnapshotRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Step 1: demote any active snapshot before promoting the target version.
	if _, err := tx.ExecContext(ctx, `UPDATE cp_runtime_snapshots SET status = ? WHERE status = ?`, SnapshotStatusInactive, SnapshotStatusActive); err != nil {
		return nil, err
	}

	// Step 2: activate the target version inside the same transaction.
	if _, err := tx.ExecContext(ctx, `UPDATE cp_runtime_snapshots SET status = ?, active_at = ? WHERE version = ?`, SnapshotStatusActive, time.Now().UTC(), version); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	record, ok, err := r.ActiveSnapshot(ctx)
	if err != nil || !ok {
		return nil, err
	}
	return record, nil
}

func (r *MySQLRepository) snapshotByStatus(ctx context.Context, status string) (*SnapshotRecord, bool, error) {
	record, err := scanSnapshot(r.db.QueryRowContext(ctx, `
SELECT version, checksum, status, payload_json, error, created_at, active_at
FROM cp_runtime_snapshots
WHERE status = ?
ORDER BY active_at DESC, created_at DESC
LIMIT 1`, status))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return record, true, nil
}

func scanSnapshot(row rowScanner) (*SnapshotRecord, error) {
	var record SnapshotRecord
	var activeAt sql.NullTime
	err := row.Scan(&record.Version, &record.Checksum, &record.Status, &record.Payload, &record.Error, &record.CreatedAt, &activeAt)
	if err != nil {
		return nil, err
	}
	if activeAt.Valid {
		record.ActiveAt = &activeAt.Time
	}
	return &record, nil
}

// LoadSnapshotConfig returns all configuration needed to build a runtime snapshot.
func (r *MemoryRepository) LoadSnapshotConfig(context.Context) (*SnapshotConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg := &SnapshotConfig{}
	for _, key := range r.apiKeys {
		if key.Enabled {
			cfg.APIKeys = append(cfg.APIKeys, key)
		}
		if key.RevokedAt != nil {
			cfg.RevokedKeys = append(cfg.RevokedKeys, key)
		}
	}
	for _, model := range r.models {
		cfg.Models = append(cfg.Models, model)
	}
	for _, channel := range r.channels {
		cfg.Channels = append(cfg.Channels, channel)
	}
	for _, route := range r.routes {
		cfg.Routes = append(cfg.Routes, route)
	}
	for _, price := range r.prices {
		cfg.Prices = append(cfg.Prices, price)
	}
	for _, limit := range r.limits {
		cfg.Limits = append(cfg.Limits, limit)
	}
	for _, binding := range r.plugins {
		cfg.Plugins = append(cfg.Plugins, binding)
	}
	return cfg, nil
}

// SaveSnapshot stores a built runtime snapshot record.
func (r *MemoryRepository) SaveSnapshot(_ context.Context, record SnapshotRecord) (*SnapshotRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	r.snapshots[record.Version] = record
	return clone(record), nil
}

// ActiveSnapshot returns the currently active runtime snapshot record.
func (r *MemoryRepository) ActiveSnapshot(context.Context) (*SnapshotRecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.snapshots[r.active]
	return clone(record), ok, nil
}

// PreviousSnapshot returns the most recently deactivated snapshot record.
func (r *MemoryRepository) PreviousSnapshot(context.Context) (*SnapshotRecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.snapshots[r.previous]
	return clone(record), ok, nil
}

// ActivateSnapshot marks version active and preserves the previous active version.
func (r *MemoryRepository) ActivateSnapshot(_ context.Context, version string) (*SnapshotRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record := r.snapshots[version]
	now := time.Now().UTC()
	// Step 1: demote the existing active snapshot before promotion.
	if r.active != "" && r.active != version {
		old := r.snapshots[r.active]
		old.Status = SnapshotStatusInactive
		r.snapshots[old.Version] = old
		r.previous = r.active
	}

	// Step 2: promote the requested snapshot as the active runtime version.
	record.Status = SnapshotStatusActive
	record.ActiveAt = &now
	r.snapshots[version] = record
	r.active = version
	return clone(record), nil
}
