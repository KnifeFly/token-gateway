package configadmin

import (
	"context"
	"sort"
	"time"
)

// UpsertProject creates or updates a project row.
func (r *MySQLRepository) UpsertProject(ctx context.Context, project Project) (*Project, error) {
	if project.ID == "" {
		project.ID = newID("project")
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO cp_projects (id, tenant_id, name, enabled) VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE tenant_id = VALUES(tenant_id), name = VALUES(name), enabled = VALUES(enabled), updated_at = CURRENT_TIMESTAMP`,
		project.ID, project.TenantID, project.Name, project.Enabled)
	if err != nil {
		return nil, err
	}
	return r.getProject(ctx, project.ID)
}

// ListProjects returns project rows ordered by creation time and optionally filtered by tenant.
func (r *MySQLRepository) ListProjects(ctx context.Context, tenantID string) ([]Project, error) {
	query := `SELECT id, tenant_id, name, enabled, created_at, updated_at FROM cp_projects WHERE 1=1`
	var args []any
	if tenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, tenantID)
	}
	query += ` ORDER BY created_at DESC, id`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.TenantID, &project.Name, &project.Enabled, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (r *MySQLRepository) getProject(ctx context.Context, id string) (*Project, error) {
	var project Project
	err := r.db.QueryRowContext(ctx, `SELECT id, tenant_id, name, enabled, created_at, updated_at FROM cp_projects WHERE id = ?`, id).
		Scan(&project.ID, &project.TenantID, &project.Name, &project.Enabled, &project.CreatedAt, &project.UpdatedAt)
	return &project, err
}

// UpsertProject creates or updates a project in memory.
func (r *MemoryRepository) UpsertProject(_ context.Context, project Project) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if project.ID == "" {
		project.ID = newID("project")
	}
	if existing, ok := r.projects[project.ID]; ok {
		project.CreatedAt = existing.CreatedAt
	} else {
		project.CreatedAt = now
	}
	project.UpdatedAt = now
	r.projects[project.ID] = project
	return clone(project), nil
}

// ListProjects returns projects ordered by ID and optionally filtered by tenant.
func (r *MemoryRepository) ListProjects(_ context.Context, tenantID string) ([]Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	projects := make([]Project, 0, len(r.projects))
	for _, project := range r.projects {
		if tenantID != "" && project.TenantID != tenantID {
			continue
		}
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
	return projects, nil
}
