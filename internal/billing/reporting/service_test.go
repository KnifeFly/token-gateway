package reporting

import (
	"context"
	"testing"
	"time"
)

func TestManualAdjustmentIsIdempotentAndFeedsTenantUsage(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo)
	ctx := context.Background()

	first, err := service.CreateManualAdjustment(ctx, ManualAdjustmentRequest{
		IdempotencyKey: "adj-1",
		TenantID:       "tenant_1",
		ProjectID:      "project_1",
		Currency:       "usd",
		AmountMicros:   1000,
		Reason:         "top up",
		OperatorID:     "ops",
	})
	if err != nil {
		t.Fatalf("CreateManualAdjustment() error = %v", err)
	}
	second, err := service.CreateManualAdjustment(ctx, ManualAdjustmentRequest{
		IdempotencyKey: "adj-1",
		TenantID:       "tenant_1",
		ProjectID:      "project_1",
		Currency:       "USD",
		AmountMicros:   1000,
		Reason:         "top up",
		OperatorID:     "ops",
	})
	if err != nil {
		t.Fatalf("CreateManualAdjustment() repeat error = %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("idempotent adjustment IDs differ: %q vs %q", second.ID, first.ID)
	}

	report, err := service.TenantUsageReport(ctx, TenantUsageFilter{TenantID: "tenant_1", ProjectID: "project_1"})
	if err != nil {
		t.Fatalf("TenantUsageReport() error = %v", err)
	}
	if len(report.Balances) != 1 || report.Balances[0].AvailableMicros != 1000 {
		t.Fatalf("balances = %#v", report.Balances)
	}
	if len(report.Ledger) != 1 || report.Ledger[0].SettlementKind != "manual_adjustment" {
		t.Fatalf("ledger = %#v", report.Ledger)
	}
}

func TestProviderProfitReportUsesCostProfile(t *testing.T) {
	repo := NewMemoryRepository()
	repo.usage = append(repo.usage, memoryUsageRecord{
		UsageSummary: UsageSummary{
			Model:        "gpt-4o-mini",
			ProviderType: "openai_compatible",
			ChannelID:    "channel_1",
			Currency:     "USD",
			Requests:     2,
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
			AmountMicros: 1000,
		},
		TenantID:  "tenant_1",
		ProjectID: "project_1",
		CreatedAt: time.Now().UTC(),
	})
	service := NewService(repo)
	if _, err := service.UpsertProviderCostProfile(context.Background(), ProviderCostProfile{
		ProviderType:          "openai_compatible",
		ChannelID:             "channel_1",
		PublicModel:           "gpt-4o-mini",
		Currency:              "USD",
		InputMicrosPerToken:   10,
		OutputMicrosPerToken:  20,
		FixedMicrosPerRequest: 5,
	}); err != nil {
		t.Fatalf("UpsertProviderCostProfile() error = %v", err)
	}

	report, err := service.ProviderProfitReport(context.Background(), ProviderProfitFilter{TenantID: "tenant_1"})
	if err != nil {
		t.Fatalf("ProviderProfitReport() error = %v", err)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("rows = %#v", report.Rows)
	}
	wantCost := int64(10*10 + 20*20 + 2*5)
	if report.Rows[0].ProviderCostMicros != wantCost || report.Rows[0].ProfitMicros != 1000-wantCost {
		t.Fatalf("row = %#v", report.Rows[0])
	}
}

func TestAgentMetadataReportGroupsWorkflowSceneShot(t *testing.T) {
	repo := NewMemoryRepository()
	repo.agentTasks = append(repo.agentTasks,
		memoryAgentTask{
			AgentMetadataRow: AgentMetadataRow{Workflow: "story", Scene: "s1", Shot: "sh1", Kind: "image", MediaType: "image", Model: "img", AmountMicros: 100, Currency: "USD"},
			TenantID:         "tenant_1",
			ProjectID:        "project_1",
			Status:           "succeeded",
			CreatedAt:        time.Now().UTC(),
		},
		memoryAgentTask{
			AgentMetadataRow: AgentMetadataRow{Workflow: "story", Scene: "s1", Shot: "sh1", Kind: "image", MediaType: "image", Model: "img", AmountMicros: 50, Currency: "USD"},
			TenantID:         "tenant_1",
			ProjectID:        "project_1",
			Status:           "failed",
			CreatedAt:        time.Now().UTC(),
		},
	)
	report, err := NewService(repo).AgentMetadataReport(context.Background(), AgentMetadataFilter{TenantID: "tenant_1"})
	if err != nil {
		t.Fatalf("AgentMetadataReport() error = %v", err)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("rows = %#v", report.Rows)
	}
	row := report.Rows[0]
	if row.Tasks != 2 || row.Succeeded != 1 || row.Failed != 1 || row.AmountMicros != 150 {
		t.Fatalf("row = %#v", row)
	}
}
