package billing

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
	_ "github.com/go-sql-driver/mysql"
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

func TestReleaseHoldReturnsReservedBalance(t *testing.T) {
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
	balances := NewBalanceService(repo)
	hold, err := balances.CreateHold(ctx, HoldRequest{
		RequestID:    "req_release",
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
	if err := balances.ReleaseHold(ctx, hold.ID, "provider failed before output"); err != nil {
		t.Fatalf("ReleaseHold() error = %v", err)
	}
	if got := repo.holds[hold.ID]; got.Status != HoldStatusReleased || got.ReleaseReason == "" {
		t.Fatalf("hold = %#v", got)
	}
	account := repo.accounts[accountKey("tenant_1", "project_1", "USD")]
	if account.AvailableMicros != 1000 || account.HeldMicros != 0 {
		t.Fatalf("account = %#v", account)
	}
}

func TestZeroAmountHoldSettlesWithZeroLedger(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	hold, err := NewBalanceService(repo).CreateHold(ctx, HoldRequest{
		RequestID:    "req_1",
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		APIKeyID:     "key_1",
		Currency:     "USD",
		AmountMicros: 0,
		ExpiresAt:    time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateHold() error = %v", err)
	}
	if hold.ID == "" || hold.AmountMicros != 0 || hold.Status != HoldStatusActive {
		t.Fatalf("hold = %#v", hold)
	}
	service := NewSettlementService(repo, NewSettlementPlanner(pricing.TokenPrice{Currency: "USD"}), nil)
	state := settlementState(hold.ID)

	if err := service.Settle(ctx, state); err != nil {
		t.Fatalf("Settle() error = %v", err)
	}
	if state.ActualChargeMicros != 0 {
		t.Fatalf("charge = %d, want 0", state.ActualChargeMicros)
	}
	if got := repo.holds[hold.ID]; got.Status != HoldStatusSettled {
		t.Fatalf("hold status = %q", got.Status)
	}
	if got := repo.ledger[state.RequestID]; got.AmountMicros != 0 || got.Reason != "usage settlement:provider_success" {
		t.Fatalf("ledger = %#v", got)
	}
}

func TestNoHoldZeroSettlementWritesUsageAndLedger(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewSettlementService(repo, NewSettlementPlanner(pricing.TokenPrice{Currency: "USD"}), nil)
	state := settlementState("")
	state.RequestID = "req_no_hold"

	if err := service.Settle(ctx, state); err != nil {
		t.Fatalf("Settle() error = %v", err)
	}
	if repo.records[state.RequestID].ID == "" {
		t.Fatal("missing usage record")
	}
	if got := repo.ledger[state.RequestID]; got.ID == "" || got.AmountMicros != 0 {
		t.Fatalf("ledger = %#v", got)
	}
	if state.SettlementID == "" {
		t.Fatal("missing settlement id")
	}
}

func TestMySQLZeroAmountHoldSettlementIntegration(t *testing.T) {
	dsn := os.Getenv("TOKEN_GATEWAY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TOKEN_GATEWAY_MYSQL_DSN to run MySQL billing integration")
	}
	ctx := context.Background()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("PingContext() error = %v", err)
	}
	applyMySQLBillingMigration(t, db)
	tenantID := "tenant_p12_" + time.Now().Format("150405000000000")
	projectID := "project_p12"
	requestID := "req_p12_zero"
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM failed_settlements WHERE tenant_id = ?", tenantID)
		_, _ = db.ExecContext(ctx, "DELETE FROM ledger_entries WHERE tenant_id = ?", tenantID)
		_, _ = db.ExecContext(ctx, "DELETE FROM usage_records WHERE tenant_id = ?", tenantID)
		_, _ = db.ExecContext(ctx, "DELETE FROM usage_attempts WHERE tenant_id = ?", tenantID)
		_, _ = db.ExecContext(ctx, "DELETE FROM balance_holds WHERE tenant_id = ?", tenantID)
		_, _ = db.ExecContext(ctx, "DELETE FROM balance_accounts WHERE tenant_id = ?", tenantID)
	})
	repo := NewMySQLRepository(db)
	hold, err := NewBalanceService(repo).CreateHold(ctx, HoldRequest{
		RequestID:    requestID,
		TenantID:     tenantID,
		ProjectID:    projectID,
		APIKeyID:     "key_1",
		Currency:     "USD",
		AmountMicros: 0,
		ExpiresAt:    time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateHold() error = %v", err)
	}
	state := settlementState(hold.ID)
	state.RequestID = requestID
	state.TenantID = tenantID
	state.ProjectID = projectID
	if err := NewSettlementService(repo, NewSettlementPlanner(pricing.TokenPrice{Currency: "USD"}), nil).Settle(ctx, state); err != nil {
		t.Fatalf("Settle() error = %v", err)
	}
	var status string
	if err := db.QueryRowContext(ctx, "SELECT status FROM balance_holds WHERE id = ?", hold.ID).Scan(&status); err != nil {
		t.Fatalf("query hold status error = %v", err)
	}
	if status != HoldStatusSettled {
		t.Fatalf("hold status = %q, want %q", status, HoldStatusSettled)
	}
	var amount int64
	if err := db.QueryRowContext(ctx, "SELECT amount_micros FROM ledger_entries WHERE request_id = ?", requestID).Scan(&amount); err != nil {
		t.Fatalf("query ledger amount error = %v", err)
	}
	if amount != 0 {
		t.Fatalf("ledger amount = %d, want 0", amount)
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

func applyMySQLBillingMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	raw, err := os.ReadFile("../../migrations/mysql/000002_m2_billing.up.sql")
	if err != nil {
		t.Fatalf("ReadFile(migration) error = %v", err)
	}
	for _, statement := range strings.Split(string(raw), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("migration statement failed: %v", err)
		}
	}
}
