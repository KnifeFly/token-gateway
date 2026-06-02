package service

import (
	"context"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// Dashboard returns Admin dashboard counters.
func (s *Service) Dashboard(ctx context.Context, actor adminapp.Actor) (adminapp.Dashboard, error) {
	if err := s.Authorize(actor, "read", "dashboard"); err != nil {
		return adminapp.Dashboard{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	if err != nil {
		return adminapp.Dashboard{}, err
	}
	tenants, err := s.owner.ListTenants(ctx)
	if err != nil {
		return adminapp.Dashboard{}, err
	}
	projects, err := s.owner.ListProjects(ctx, "")
	if err != nil {
		return adminapp.Dashboard{}, err
	}
	taskCount, _ := s.taskCount(ctx)
	failedSettlementCount, _ := s.failedSettlementCount(ctx)
	dueCallbackCount, _ := s.dueCallbackCount(ctx)
	return adminapp.Dashboard{
		GeneratedAt: s.now(),
		Counts: adminapp.DashboardCounts{
			Tenants:           len(tenants),
			Projects:          len(projects),
			APIKeys:           len(cfg.APIKeys) + len(cfg.RevokedKeys),
			Models:            len(cfg.Models),
			Channels:          len(cfg.Channels),
			Routes:            len(cfg.Routes),
			PricingRules:      len(cfg.Prices),
			LimitRules:        len(cfg.Limits),
			Tasks:             taskCount,
			FailedSettlements: failedSettlementCount,
			DueCallbacks:      dueCallbackCount,
		},
	}, nil
}

func (s *Service) snapshotConfig(ctx context.Context) (*configadmin.SnapshotConfig, error) {
	if s == nil || s.owner == nil {
		return nil, apperr.ConfigUnavailable("control-plane admin service is unavailable")
	}
	return s.owner.LoadSnapshotConfig(ctx)
}

func (s *Service) taskCount(ctx context.Context) (int, error) {
	if s.tasks == nil {
		return 0, nil
	}
	tasks, err := s.tasks.ListTasks(ctx, tasksvc.TaskListFilter{Limit: 200})
	return len(tasks), err
}

func (s *Service) failedSettlementCount(ctx context.Context) (int, error) {
	if s.commercial == nil {
		return 0, nil
	}
	report, err := s.commercial.ReconciliationReport(ctx)
	if err != nil {
		return 0, err
	}
	return len(report.FailedSettlements), nil
}

func (s *Service) dueCallbackCount(ctx context.Context) (int, error) {
	if s.tasks == nil {
		return 0, nil
	}
	callbacks, err := s.tasks.ListDueCallbacks(ctx, 200, s.now())
	return len(callbacks), err
}
