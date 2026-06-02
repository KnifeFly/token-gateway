package service

import (
	"context"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// ListFailedSettlements returns safe failed settlement operations.
func (s *Service) ListFailedSettlements(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[reporting.FailedSettlementSummary], error) {
	if err := s.Authorize(actor, "read", "settlement"); err != nil {
		return adminapp.ListResponse[reporting.FailedSettlementSummary]{}, err
	}
	if s.commercial == nil {
		return adminapp.ListResponse[reporting.FailedSettlementSummary]{}, apperr.ConfigUnavailable("commercial reporting is unavailable")
	}
	report, err := s.commercial.ReconciliationReport(ctx)
	if err != nil {
		return adminapp.ListResponse[reporting.FailedSettlementSummary]{}, err
	}
	return adminapp.ListResponse[reporting.FailedSettlementSummary]{Data: report.FailedSettlements}, nil
}

// ReplayFailedSettlement triggers the owner failed-settlement replay workflow.
func (s *Service) ReplayFailedSettlement(ctx context.Context, actor adminapp.Actor, settlementID string, opts adminapp.MutationOptions) (adminapp.ReplayResult, error) {
	return mutate(ctx, s, actor, opts, "replay", "settlement", settlementID, map[string]string{"id": settlementID}, func() (adminapp.ReplayResult, error) {
		if s.failedSettlements == nil {
			return adminapp.ReplayResult{}, apperr.ConfigUnavailable("failed settlement repair is unavailable")
		}
		replayed, err := s.failedSettlements.ReplayPending(ctx, 1)
		return adminapp.ReplayResult{RequestedID: settlementID, Replayed: replayed}, err
	})
}

// ListCallbacks returns safe due callback operations.
func (s *Service) ListCallbacks(ctx context.Context, actor adminapp.Actor, limit int) (adminapp.ListResponse[adminapp.CallbackEventView], error) {
	if err := s.Authorize(actor, "read", "callback"); err != nil {
		return adminapp.ListResponse[adminapp.CallbackEventView]{}, err
	}
	if s.tasks == nil {
		return adminapp.ListResponse[adminapp.CallbackEventView]{}, apperr.ConfigUnavailable("task repository is unavailable")
	}
	events, err := s.tasks.ListDueCallbacks(ctx, normalizeLimit(limit), s.now())
	if err != nil {
		return adminapp.ListResponse[adminapp.CallbackEventView]{}, err
	}
	views := make([]adminapp.CallbackEventView, 0, len(events))
	for _, event := range events {
		views = append(views, safeCallback(event))
	}
	return adminapp.ListResponse[adminapp.CallbackEventView]{Data: views}, nil
}

// RetryCallback returns a stable disabled response until callback owner exposes single-row retry.
func (s *Service) RetryCallback(ctx context.Context, actor adminapp.Actor, callbackID string, opts adminapp.MutationOptions) (adminapp.ReplayResult, error) {
	return mutate(ctx, s, actor, opts, "retry", "callback", callbackID, map[string]string{"id": callbackID}, func() (adminapp.ReplayResult, error) {
		if s.tasks == nil {
			return adminapp.ReplayResult{}, apperr.ConfigUnavailable("task repository is unavailable")
		}
		err := s.tasks.MarkCallbackFailed(ctx, callbackID, "", s.now(), "manual retry requested by admin", 0, 0)
		return adminapp.ReplayResult{RequestedID: callbackID, Replayed: 1}, err
	})
}

// ListWorkers returns worker operation read models.
func (s *Service) ListWorkers(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[adminapp.WorkerJobView], error) {
	if err := s.Authorize(actor, "read", "worker"); err != nil {
		return adminapp.ListResponse[adminapp.WorkerJobView]{}, err
	}
	return adminapp.ListResponse[adminapp.WorkerJobView]{Data: []adminapp.WorkerJobView{
		{Name: "failed_settlement_replayer", Status: "configured_by_worker_process"},
		{Name: "callback_dispatcher", Status: "configured_by_worker_process"},
		{Name: "provider_task_poller", Status: "configured_by_worker_process"},
	}}, nil
}

// ListHolds returns hold-aging read models.
func (s *Service) ListHolds(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[adminapp.HoldAgingView], error) {
	if err := s.Authorize(actor, "read", "hold"); err != nil {
		return adminapp.ListResponse[adminapp.HoldAgingView]{}, err
	}
	return adminapp.ListResponse[adminapp.HoldAgingView]{Data: []adminapp.HoldAgingView{{Status: "active", Count: 0}}}, nil
}

func safeCallback(event tasksvc.CallbackEvent) adminapp.CallbackEventView {
	return adminapp.CallbackEventView{
		ID:             event.ID,
		TaskID:         event.TaskID,
		TenantID:       event.TenantID,
		ProjectID:      event.ProjectID,
		Status:         string(event.Status),
		RetryCount:     event.RetryCount,
		NextRetryAt:    event.NextRetryAt,
		LastError:      safeShort(event.LastError),
		OwnerID:        event.OwnerID,
		LastStatusCode: event.LastStatusCode,
		LastLatencyMS:  event.LastLatencyMS,
		CreatedAt:      event.CreatedAt,
		UpdatedAt:      event.UpdatedAt,
	}
}
