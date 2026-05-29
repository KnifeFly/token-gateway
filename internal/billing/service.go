package billing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// BalanceService creates and releases balance holds.
type BalanceService struct {
	repo Repository
}

func NewBalanceService(repo Repository) *BalanceService {
	return &BalanceService{repo: repo}
}

func (s *BalanceService) CreateHold(ctx context.Context, request HoldRequest) (*BalanceHold, error) {
	if s == nil || s.repo == nil {
		return nil, apperr.ConfigUnavailable("billing repository is unavailable")
	}
	if request.AmountMicros <= 0 {
		return &BalanceHold{ID: "", RequestID: request.RequestID, Status: HoldStatusReleased}, nil
	}
	if hold, ok, err := s.repo.GetHoldByRequestID(ctx, request.RequestID); err != nil {
		return nil, err
	} else if ok {
		return hold, nil
	}
	return s.repo.CreateHold(ctx, request)
}

func (s *BalanceService) ReleaseHold(ctx context.Context, holdID string, reason string) error {
	if holdID == "" || s == nil || s.repo == nil {
		return nil
	}
	return s.repo.ReleaseHold(ctx, holdID, reason)
}

// AttemptWriter persists provider attempts.
type AttemptWriter struct {
	repo Repository
}

func NewAttemptWriter(repo Repository) *AttemptWriter {
	return &AttemptWriter{repo: repo}
}

func (w *AttemptWriter) RecordProviderAttempt(ctx context.Context, state *engine.RequestState, attempt engine.ProviderAttempt) error {
	if w == nil || w.repo == nil {
		return nil
	}
	return w.repo.RecordUsageAttempt(ctx, UsageAttempt{
		RequestID:             state.RequestID,
		AttemptIndex:          attempt.AttemptIndex,
		TenantID:              state.TenantID,
		ProjectID:             state.ProjectID,
		APIKeyID:              state.APIKeyID,
		ChannelID:             attempt.ChannelID,
		ProviderType:          attempt.ProviderType,
		Model:                 attempt.PublicModel,
		StatusCode:            attempt.StatusCode,
		ErrorCode:             attempt.ErrorCode,
		Success:               attempt.Success,
		EstimatedInputTokens:  state.EstimatedUsage.InputTokens,
		EstimatedOutputTokens: state.EstimatedUsage.OutputTokens,
		ActualInputTokens:     state.ActualUsage.InputTokens,
		ActualOutputTokens:    state.ActualUsage.OutputTokens,
	})
}

// SettlementPlanner creates final settlement plans from request state.
type SettlementPlanner struct {
	price pricing.TokenPrice
}

func NewSettlementPlanner(price pricing.TokenPrice) *SettlementPlanner {
	return &SettlementPlanner{price: price}
}

func (p *SettlementPlanner) Plan(state *engine.RequestState) SettlementPlan {
	amount := p.price.QuoteActual(state.ActualUsage)
	if state.PriceRule.Enabled {
		amount = pricing.TokenPrice{
			Currency:             state.PriceRule.Currency,
			InputMicrosPerToken:  state.PriceRule.InputMicrosPerToken,
			OutputMicrosPerToken: state.PriceRule.OutputMicrosPerToken,
		}.QuoteActual(state.ActualUsage)
	}
	if amount.Micros == 0 && state.EstimatedChargeMicros > 0 {
		amount.Micros = state.EstimatedChargeMicros
		amount.Currency = state.Currency
	}
	candidate := engine.ProviderCandidate{}
	if state.ProviderResult != nil {
		candidate = state.ProviderResult.Candidate
	}
	return SettlementPlan{
		RequestID:    state.RequestID,
		TenantID:     state.TenantID,
		ProjectID:    state.ProjectID,
		APIKeyID:     state.APIKeyID,
		HoldID:       state.BalanceHoldID,
		Model:        state.RequestedModel,
		ProviderType: candidate.ProviderType,
		ChannelID:    candidate.ChannelID,
		Usage:        state.ActualUsage,
		AmountMicros: amount.Micros,
		Currency:     amount.Currency,
		Billable:     true,
	}
}

// SettlementService executes final settlement and records replayable failures.
type SettlementService struct {
	repo    Repository
	planner *SettlementPlanner
	metrics *Metrics
}

func NewSettlementService(repo Repository, planner *SettlementPlanner, metrics *Metrics) *SettlementService {
	return &SettlementService{repo: repo, planner: planner, metrics: metrics}
}

func (s *SettlementService) Settle(ctx context.Context, state *engine.RequestState) error {
	if s == nil || s.repo == nil || s.planner == nil {
		return nil
	}
	plan := s.planner.Plan(state)
	result, err := s.repo.Settle(ctx, plan)
	if err != nil {
		return err
	}
	state.ActualChargeMicros = result.Amount.Micros
	state.Currency = result.Amount.Currency
	state.BalanceAccountID = result.AccountID
	state.SettlementID = result.LedgerEntryID
	return nil
}

func (s *SettlementService) RecordFailed(ctx context.Context, state *engine.RequestState, cause error) error {
	if s == nil || s.repo == nil || s.planner == nil {
		return cause
	}
	plan := s.planner.Plan(state)
	payload, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if s.metrics != nil {
		s.metrics.RecordSettlementFailure()
	}
	err = s.repo.SaveFailedSettlement(ctx, FailedSettlement{
		RequestID:   state.RequestID,
		TenantID:    state.TenantID,
		ProjectID:   state.ProjectID,
		APIKeyID:    state.APIKeyID,
		HoldID:      state.BalanceHoldID,
		Payload:     payload,
		Status:      FailedSettlementPending,
		NextRetryAt: time.Now().UTC(),
		LastError:   cause.Error(),
	})
	if err == nil && s.metrics != nil {
		s.metrics.IncrementFailedBacklog()
	}
	return err
}

// FailedSettlementService replays pending failed settlements.
type FailedSettlementService struct {
	repo    Repository
	metrics *Metrics
}

func NewFailedSettlementService(repo Repository) *FailedSettlementService {
	return NewFailedSettlementServiceWithMetrics(repo, nil)
}

func NewFailedSettlementServiceWithMetrics(repo Repository, metrics *Metrics) *FailedSettlementService {
	return &FailedSettlementService{repo: repo, metrics: metrics}
}

func (s *FailedSettlementService) ReplayPending(ctx context.Context, limit int) (int, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if limit <= 0 {
		limit = 100
	}
	pending, err := s.repo.ListPendingFailedSettlements(ctx, limit)
	if err != nil {
		return 0, err
	}
	replayed := 0
	for _, failed := range pending {
		var plan SettlementPlan
		if err := json.Unmarshal(failed.Payload, &plan); err != nil {
			if markErr := s.repo.MarkFailedSettlementFailed(ctx, failed.ID, time.Now().UTC().Add(time.Minute), err.Error()); markErr != nil {
				return replayed, markErr
			}
			continue
		}
		if _, err := s.repo.Settle(ctx, plan); err != nil {
			next := time.Now().UTC().Add(time.Duration(failed.RetryCount+1) * time.Minute)
			if markErr := s.repo.MarkFailedSettlementFailed(ctx, failed.ID, next, err.Error()); markErr != nil {
				return replayed, markErr
			}
			continue
		}
		if err := s.repo.MarkFailedSettlementReplayed(ctx, failed.ID); err != nil {
			return replayed, err
		}
		s.metrics.DecrementFailedBacklog()
		replayed++
	}
	if s.metrics != nil {
		s.metrics.SetFailedBacklog(len(pending) - replayed)
	}
	return replayed, nil
}
