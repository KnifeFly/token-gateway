package admission

import (
	"context"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

func TestControllerReserve(t *testing.T) {
	repo := billing.NewMemoryRepository()
	ctx := context.Background()
	_ = repo.EnsureBalanceAccount(ctx, billing.BalanceAccount{
		ID:              "acct_1",
		TenantID:        "tenant_1",
		ProjectID:       "project_1",
		Currency:        "USD",
		OpeningMicros:   1000,
		AvailableMicros: 1000,
	})
	controller := NewController(
		billing.NewBalanceService(repo),
		NewPriceEstimator(pricing.TokenPrice{Currency: "USD", InputMicrosPerToken: 10, OutputMicrosPerToken: 20}, 5),
		time.Minute,
	)
	state := &engine.RequestState{
		RequestID:      "req_1",
		TenantID:       "tenant_1",
		ProjectID:      "project_1",
		APIKeyID:       "key_1",
		EstimatedUsage: tokenusage.Estimate{InputTokens: 10},
	}

	if err := controller.Reserve(ctx, state); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if state.BalanceHoldID == "" {
		t.Fatal("missing hold id")
	}
	if state.EstimatedChargeMicros != 200 {
		t.Fatalf("estimated charge = %d", state.EstimatedChargeMicros)
	}
}
