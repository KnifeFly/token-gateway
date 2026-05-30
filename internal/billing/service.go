package billing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// service.go owns balance holds, provider attempt writes, final settlement, and repair replay.

// BalanceService creates and releases balance holds.
type BalanceService struct {
	repo Repository
}

// NewBalanceService returns a balance service backed by repo.
func NewBalanceService(repo Repository) *BalanceService {
	return &BalanceService{repo: repo}
}

// CreateHold creates or returns an existing idempotent balance hold.
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

// ReleaseHold releases a previously created hold when a request does not settle.
func (s *BalanceService) ReleaseHold(ctx context.Context, holdID string, reason string) error {
	if holdID == "" || s == nil || s.repo == nil {
		return nil
	}
	return s.repo.ReleaseHold(ctx, holdID, reason)
}

// ReleaseExpiredHolds releases active holds whose reservation TTL elapsed.
func (s *BalanceService) ReleaseExpiredHolds(ctx context.Context, now time.Time, limit int) (int, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 100
	}
	return s.repo.ReleaseExpiredHolds(ctx, now.UTC(), limit)
}

// AttemptWriter persists provider attempts.
type AttemptWriter struct {
	repo Repository
}

// NewAttemptWriter returns a writer for durable provider attempts.
func NewAttemptWriter(repo Repository) *AttemptWriter {
	return &AttemptWriter{repo: repo}
}

// RecordProviderAttempt persists one provider attempt for auditing and reports.
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
		Retryable:             attempt.Retryable,
		RetryBudgetConsumed:   attempt.RetryBudgetConsumed,
		RetryBudgetRemaining:  attempt.RetryBudgetRemaining,
		FallbackFromChannelID: attempt.FallbackFromChannelID,
		FallbackFromProvider:  attempt.FallbackFromProvider,
		CircuitState:          attempt.CircuitState,
		Final:                 attempt.Final,
	})
}

// SettlementPlanner creates final settlement plans from request state.
type SettlementPlanner struct {
	price  pricing.TokenPrice
	policy BillabilityPolicy
}

// NewSettlementPlanner returns a planner using the default billability policy.
func NewSettlementPlanner(price pricing.TokenPrice) *SettlementPlanner {
	return NewSettlementPlannerWithPolicy(price, NewBillabilityPolicy())
}

// NewSettlementPlannerWithPolicy returns a settlement planner with an explicit billability policy.
func NewSettlementPlannerWithPolicy(price pricing.TokenPrice, policy BillabilityPolicy) *SettlementPlanner {
	return &SettlementPlanner{price: price, policy: policy}
}

// Plan converts the final request state into a replayable settlement plan.
func (p *SettlementPlanner) Plan(state *engine.RequestState) SettlementPlan {
	// Step 1: decide billability before calculating the final charge.
	decision := p.policy.Decide(RequestBillabilityContext(state))
	amount := p.price.QuoteActual(state.ActualUsage)
	if state.PriceRule.Enabled {
		amount = pricing.TokenPrice{
			Currency:             state.PriceRule.Currency,
			InputMicrosPerToken:  state.PriceRule.InputMicrosPerToken,
			OutputMicrosPerToken: state.PriceRule.OutputMicrosPerToken,
		}.QuoteActual(state.ActualUsage)
	}
	if !decision.Billable {
		amount.Micros = 0
	} else if amount.Micros == 0 && state.EstimatedChargeMicros > 0 {
		amount.Micros = state.EstimatedChargeMicros
		amount.Currency = state.Currency
	}

	// Step 2: copy provider selection and usage into a durable replay payload.
	candidate := engine.ProviderCandidate{}
	if state.ProviderResult != nil {
		candidate = state.ProviderResult.Candidate
	}
	return SettlementPlan{
		RequestID:      state.RequestID,
		TenantID:       state.TenantID,
		ProjectID:      state.ProjectID,
		APIKeyID:       state.APIKeyID,
		HoldID:         state.BalanceHoldID,
		Model:          state.RequestedModel,
		ProviderType:   candidate.ProviderType,
		ChannelID:      candidate.ChannelID,
		Usage:          state.ActualUsage,
		AmountMicros:   amount.Micros,
		Currency:       amount.Currency,
		Billable:       decision.Billable,
		BillableReason: decision.Reason,
	}
}

// SettlementService executes final settlement and records replayable failures.
type SettlementService struct {
	repo    Repository
	planner *SettlementPlanner
	metrics *Metrics
}

// NewSettlementService returns a service that settles completed requests.
func NewSettlementService(repo Repository, planner *SettlementPlanner, metrics *Metrics) *SettlementService {
	return &SettlementService{repo: repo, planner: planner, metrics: metrics}
}

// Settle writes usage, ledger movement, and request charge metadata.
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

// RecordFailed persists a settlement plan for repair when final settlement fails.
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

// NewFailedSettlementService returns a replay service without metrics.
func NewFailedSettlementService(repo Repository) *FailedSettlementService {
	return NewFailedSettlementServiceWithMetrics(repo, nil)
}

// NewFailedSettlementServiceWithMetrics returns a replay service with optional metrics.
func NewFailedSettlementServiceWithMetrics(repo Repository, metrics *Metrics) *FailedSettlementService {
	return &FailedSettlementService{repo: repo, metrics: metrics}
}

// ReplayPending retries pending settlement repair records up to limit.
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
		// Step 1: decode the original settlement plan exactly as recorded.
		var plan SettlementPlan
		if err := json.Unmarshal(failed.Payload, &plan); err != nil {
			if markErr := s.repo.MarkFailedSettlementFailed(ctx, failed.ID, time.Now().UTC().Add(time.Minute), err.Error()); markErr != nil {
				return replayed, markErr
			}
			continue
		}

		// Step 2: retry settlement and move failures to a later repair window.
		if _, err := s.repo.Settle(ctx, plan); err != nil {
			next := time.Now().UTC().Add(time.Duration(failed.RetryCount+1) * time.Minute)
			if markErr := s.repo.MarkFailedSettlementFailed(ctx, failed.ID, next, err.Error()); markErr != nil {
				return replayed, markErr
			}
			continue
		}

		// Step 3: mark successful replays so the repair worker is idempotent.
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

func settlementReason(plan SettlementPlan) string {
	if plan.BillableReason == "" {
		return "usage settlement"
	}
	if !plan.Billable {
		return "usage settlement:not_billable:" + plan.BillableReason
	}
	return "usage settlement:" + plan.BillableReason
}
