package configadmin

import (
	"context"
	"sort"
	"time"
)

// UpsertTenant creates or updates a tenant row.
func (r *MySQLRepository) UpsertTenant(ctx context.Context, tenant Tenant) (*Tenant, error) {
	if tenant.ID == "" {
		tenant.ID = newID("tenant")
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cp_tenants (id, name, enabled) VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE name = VALUES(name), enabled = VALUES(enabled), updated_at = CURRENT_TIMESTAMP`,
		tenant.ID, tenant.Name, tenant.Enabled)
	if err != nil {
		return nil, err
	}
	return r.getTenant(ctx, tenant.ID)
}

// ListTenants returns all tenant rows ordered by creation time.
func (r *MySQLRepository) ListTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, enabled, created_at, updated_at FROM cp_tenants ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []Tenant
	for rows.Next() {
		var tenant Tenant
		if err := rows.Scan(&tenant.ID, &tenant.Name, &tenant.Enabled, &tenant.CreatedAt, &tenant.UpdatedAt); err != nil {
			return nil, err
		}
		tenants = append(tenants, tenant)
	}
	return tenants, rows.Err()
}

func (r *MySQLRepository) getTenant(ctx context.Context, id string) (*Tenant, error) {
	var tenant Tenant
	err := r.db.QueryRowContext(ctx, `SELECT id, name, enabled, created_at, updated_at FROM cp_tenants WHERE id = ?`, id).
		Scan(&tenant.ID, &tenant.Name, &tenant.Enabled, &tenant.CreatedAt, &tenant.UpdatedAt)
	return &tenant, err
}

// UpsertTenant creates or updates a tenant in memory.
func (r *MemoryRepository) UpsertTenant(_ context.Context, tenant Tenant) (*Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if tenant.ID == "" {
		tenant.ID = newID("tenant")
	}
	if existing, ok := r.tenants[tenant.ID]; ok {
		tenant.CreatedAt = existing.CreatedAt
	} else {
		tenant.CreatedAt = now
	}
	tenant.UpdatedAt = now
	r.tenants[tenant.ID] = tenant
	return clone(tenant), nil
}

// ListTenants returns all tenants ordered by ID.
func (r *MemoryRepository) ListTenants(_ context.Context) ([]Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tenants := make([]Tenant, 0, len(r.tenants))
	for _, tenant := range r.tenants {
		tenants = append(tenants, tenant)
	}
	sort.Slice(tenants, func(i, j int) bool { return tenants[i].ID < tenants[j].ID })
	return tenants, nil
}
