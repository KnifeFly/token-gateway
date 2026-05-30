package billing

import (
	"context"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

func TestBalanceHoldAndSettlementIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	if err := repo.EnsureBalanceAccount(ctx, BalanceAccount{
		ID:              "acct_1",
		TenantID:        "tenant_1",
		ProjectID:       "project_1",
		Currency:        "USD",
		OpeningMicros:   1000,
		AvailableMicros: 1000,
	}); err != nil {
		t.Fatalf("EnsureBalanceAccount() error = %v", err)
	}
	balances := NewBalanceService(repo)
	hold, err := balances.CreateHold(ctx, HoldRequest{
		RequestID:    "req_1",
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		APIKeyID:     "key_1",
		Currency:     "USD",
		AmountMicros: 300,
		ExpiresAt:    time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateHold() error = %v", err)
	}
	planner := NewSettlementPlanner(pricing.TokenPrice{Currency: "USD", InputMicrosPerToken: 10, OutputMicrosPerToken: 20})
	service := NewSettlementService(repo, planner, nil)
	state := settlementState(hold.ID)

	if err := service.Settle(ctx, state); err != nil {
		t.Fatalf("Settle() error = %v", err)
	}
	firstCharge := state.ActualChargeMicros
	if err := service.Settle(ctx, state); err != nil {
		t.Fatalf("second Settle() error = %v", err)
	}
	if state.ActualChargeMicros != firstCharge {
		t.Fatalf("charge changed from %d to %d", firstCharge, state.ActualChargeMicros)
	}
	issues, err := repo.Reconcile(ctx)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("reconciliation issues = %#v", issues)
	}
}

func TestFailedSettlementReplay(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	_ = repo.EnsureBalanceAccount(ctx, BalanceAccount{
		ID:              "acct_1",
		TenantID:        "tenant_1",
		ProjectID:       "project_1",
		Currency:        "USD",
		OpeningMicros:   1000,
		AvailableMicros: 1000,
	})
	hold, err := NewBalanceService(repo).CreateHold(ctx, HoldRequest{
		RequestID:    "req_1",
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		APIKeyID:     "key_1",
		Currency:     "USD",
		AmountMicros: 300,
		ExpiresAt:    time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateHold() error = %v", err)
	}
	planner := NewSettlementPlanner(pricing.TokenPrice{Currency: "USD", InputMicrosPerToken: 10, OutputMicrosPerToken: 20})
	service := NewSettlementService(repo, planner, nil)
	repo.FailNextSettle = true
	state := settlementState(hold.ID)

	if err := service.Settle(ctx, state); err == nil {
		t.Fatal("expected injected settlement failure")
	}
	if err := service.RecordFailed(ctx, state, errInjected); err != nil {
		t.Fatalf("RecordFailed() error = %v", err)
	}
	replayed, err := NewFailedSettlementService(repo).ReplayPending(ctx, 10)
	if err != nil {
		t.Fatalf("ReplayPending() error = %v", err)
	}
	if replayed != 1 {
		t.Fatalf("replayed = %d", replayed)
	}
	issues, err := repo.Reconcile(ctx)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("reconciliation issues = %#v", issues)
	}
}

func TestSettlementPlannerMarksNoOutputNotBillable(t *testing.T) {
	planner := NewSettlementPlanner(pricing.TokenPrice{Currency: "USD", InputMicrosPerToken: 10, OutputMicrosPerToken: 20})
	state := settlementState("hold_1")
	state.ActualUsage = tokenusage.Actual{}
	state.EstimatedChargeMicros = 300
	state.ProviderResult.Response = nil

	plan := planner.Plan(state)
	if plan.Billable {
		t.Fatalf("Billable = true, want false")
	}
	if plan.AmountMicros != 0 {
		t.Fatalf("AmountMicros = %d, want 0", plan.AmountMicros)
	}
	if plan.BillableReason != BillabilityReasonNoEffectiveOutput {
		t.Fatalf("BillableReason = %q", plan.BillableReason)
	}
}

func TestSettlementPlannerBillsPartialStreamClientDisconnect(t *testing.T) {
	planner := NewSettlementPlanner(pricing.TokenPrice{Currency: "USD", InputMicrosPerToken: 10, OutputMicrosPerToken: 20})
	state := settlementState("hold_1")
	state.Stream = true
	state.Internal = map[string]any{
		"stream_chunks":           int64(1),
		"stream_upstream_bytes":   int64(32),
		"stream_downstream_error": "client_disconnected",
	}

	plan := planner.Plan(state)
	if !plan.Billable {
		t.Fatalf("Billable = false, reason = %q", plan.BillableReason)
	}
	if plan.BillableReason != BillabilityReasonPartialOutputClientDisconnected {
		t.Fatalf("BillableReason = %q", plan.BillableReason)
	}
}

func TestAttemptWriterRecordsReliabilityFields(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	state := &engine.RequestState{
		RequestID:      "req_attempt",
		TenantID:       "tenant_1",
		ProjectID:      "project_1",
		APIKeyID:       "key_1",
		EstimatedUsage: tokenusage.Estimate{InputTokens: 10, OutputTokens: 20},
		ActualUsage:    tokenusage.Actual{InputTokens: 10, OutputTokens: 5},
	}
	attempt := engine.ProviderAttempt{
		AttemptIndex:          2,
		ChannelID:             "channel_2",
		ProviderType:          "openai_compatible",
		PublicModel:           "gpt-4o-mini",
		StatusCode:            200,
		Success:               true,
		Retryable:             true,
		RetryBudgetConsumed:   2,
		RetryBudgetRemaining:  0,
		FallbackFromChannelID: "channel_1",
		FallbackFromProvider:  "openai_compatible",
		CircuitState:          "half_open",
		Final:                 true,
	}

	if err := NewAttemptWriter(repo).RecordProviderAttempt(ctx, state, attempt); err != nil {
		t.Fatalf("RecordProviderAttempt() error = %v", err)
	}
	got := repo.attempts["req_attempt:2:channel_2"]
	if !got.Retryable || got.RetryBudgetConsumed != 2 || got.RetryBudgetRemaining != 0 || got.FallbackFromChannelID != "channel_1" || got.CircuitState != "half_open" || !got.Final {
		t.Fatalf("usage attempt = %#v", got)
	}
}

func settlementState(holdID string) *engine.RequestState {
	return &engine.RequestState{
		RequestID:      "req_1",
		TenantID:       "tenant_1",
		ProjectID:      "project_1",
		APIKeyID:       "key_1",
		RequestedModel: "gpt-4o-mini",
		BalanceHoldID:  holdID,
		ActualUsage:    tokenusage.Actual{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
		ProviderResult: &engine.ProviderResult{Candidate: engine.ProviderCandidate{
			ProviderType: "openai_compatible",
			ChannelID:    "channel_1",
		}, Response: &engine.GatewayResponse{StatusCode: 200, Body: []byte(`{"ok":true}`)}},
	}
}

var errInjected = assertErr("settlement failed")

type assertErr string

func (e assertErr) Error() string { return string(e) }
