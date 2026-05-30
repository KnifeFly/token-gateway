package task

import (
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
