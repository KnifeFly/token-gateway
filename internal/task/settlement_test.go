package task

import (
	"context"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/billing"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

func TestBillingSettlementPlanUsesBillabilityPolicy(t *testing.T) {
	settlement := NewBillingSettlement(nil, pricing.TokenPrice{Currency: "USD", InputMicrosPerToken: 10, OutputMicrosPerToken: 20})

	canceled := settlement.plan(Task{RequestID: "req_1", Status: StatusCanceled, Result: []byte(`{"url":"x"}`)}, tokenusage.Actual{OutputTokens: 1, TotalTokens: 1})
	if canceled.Billable || canceled.AmountMicros != 0 || canceled.BillableReason != billing.BillabilityReasonTaskCanceled {
		t.Fatalf("canceled plan = %#v", canceled)
	}

	succeeded := settlement.plan(Task{RequestID: "req_2", Status: StatusSucceeded, Result: []byte(`{"url":"x"}`)}, tokenusage.Actual{OutputTokens: 1, TotalTokens: 1})
	if !succeeded.Billable || succeeded.AmountMicros == 0 || succeeded.BillableReason != billing.BillabilityReasonProviderSuccess {
		t.Fatalf("succeeded plan = %#v", succeeded)
	}
}

func TestBillingSettlementAllowsNoHoldZeroAmountAudit(t *testing.T) {
	ctx := context.Background()
	repo := billing.NewMemoryRepository()
	settlement := NewBillingSettlement(repo, pricing.TokenPrice{Currency: "USD"})
	task := Task{
		RequestID:    "req_no_hold_task",
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		APIKeyID:     "key_1",
		Model:        "free-image",
		Status:       StatusSucceeded,
		ProviderType: "mock_media",
		ChannelID:    "channel_1",
		Result:       []byte(`{"results":["https://provider.example/result.png"]}`),
	}

	if err := settlement.Settle(ctx, task, tokenusage.Actual{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}); err != nil {
		t.Fatalf("Settle() error = %v", err)
	}
}
