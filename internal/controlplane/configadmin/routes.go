package configadmin

import (
	"context"
)

// UpsertRoute creates or updates a route policy and candidate order.
func (r *MySQLRepository) UpsertRoute(ctx context.Context, route RoutePolicyConfig) (*RoutePolicyConfig, error) {
	if route.ID == "" {
		route.ID = "route_" + route.PublicModel
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO cp_route_policies (id, public_model, strategy, enabled) VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE public_model = VALUES(public_model), strategy = VALUES(strategy), enabled = VALUES(enabled), updated_at = CURRENT_TIMESTAMP`,
		route.ID, route.PublicModel, route.Strategy, route.Enabled); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cp_route_candidates WHERE route_id = ?`, route.ID); err != nil {
		return nil, err
	}
	for _, candidate := range route.Candidates {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO cp_route_candidates (route_id, channel_id, priority, weight) VALUES (?, ?, ?, ?)`,
			route.ID, candidate.ChannelID, candidate.Priority, candidate.Weight); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &route, nil
}

func (r *MySQLRepository) listRoutes(ctx context.Context) ([]RoutePolicyConfig, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, public_model, strategy, enabled FROM cp_route_policies ORDER BY public_model`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var routes []RoutePolicyConfig
	for rows.Next() {
		var route RoutePolicyConfig
		if err := rows.Scan(&route.ID, &route.PublicModel, &route.Strategy, &route.Enabled); err != nil {
			return nil, err
		}
		candidates, err := r.routeCandidates(ctx, route.ID)
		if err != nil {
			return nil, err
		}
		route.Candidates = candidates
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

func (r *MySQLRepository) routeCandidates(ctx context.Context, routeID string) ([]RouteCandidate, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT channel_id, priority, weight FROM cp_route_candidates WHERE route_id = ? ORDER BY priority, channel_id`, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []RouteCandidate
	for rows.Next() {
		var candidate RouteCandidate
		if err := rows.Scan(&candidate.ChannelID, &candidate.Priority, &candidate.Weight); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

// UpsertRoute creates or updates a route policy.
func (r *MemoryRepository) UpsertRoute(_ context.Context, route RoutePolicyConfig) (*RoutePolicyConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[route.ID] = route
	return clone(route), nil
}
