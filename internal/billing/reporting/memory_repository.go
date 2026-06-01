package reporting

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

type memoryUsageRecord struct {
	UsageSummary
	TenantID  string
	ProjectID string
	CreatedAt time.Time
}

type memoryAgentTask struct {
	AgentMetadataRow
	TenantID  string
	ProjectID string
	Status    string
	CreatedAt time.Time
}

// MemoryRepository is a deterministic reporting repository for tests and local control API.
type MemoryRepository struct {
	mu          sync.Mutex
	balances    map[string]BalanceSummary
	usage       []memoryUsageRecord
	ledger      []LedgerLine
	profiles    map[string]ProviderCostProfile
	failed      []FailedSettlementSummary
	adjustments map[string]ManualAdjustment
	agentTasks  []memoryAgentTask
}

// NewMemoryRepository returns an empty commercial reporting repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		balances:    make(map[string]BalanceSummary),
		profiles:    make(map[string]ProviderCostProfile),
		adjustments: make(map[string]ManualAdjustment),
	}
}

// TenantUsageReport returns balance, usage, and ledger rows from memory.
func (r *MemoryRepository) TenantUsageReport(_ context.Context, filter TenantUsageFilter) (*TenantUsageReport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	report := &TenantUsageReport{GeneratedAt: time.Now().UTC(), Filter: filter}
	for _, balance := range r.balances {
		if !balanceMatches(balance, filter.TenantID, filter.ProjectID, filter.Currency) {
			continue
		}
		report.Balances = append(report.Balances, balance)
	}
	usageByKey := make(map[string]*UsageSummary)
	for _, record := range r.usage {
		if !timeMatches(record.CreatedAt, filter.From, filter.To) || record.TenantID != filter.TenantID {
			continue
		}
		if filter.ProjectID != "" && record.ProjectID != filter.ProjectID {
			continue
		}
		key := fmt.Sprintf("%s:%s:%s:%s", record.Model, record.ProviderType, record.ChannelID, record.Currency)
		row := usageByKey[key]
		if row == nil {
			row = &UsageSummary{Model: record.Model, ProviderType: record.ProviderType, ChannelID: record.ChannelID, Currency: record.Currency}
			usageByKey[key] = row
		}
		addUsage(row, record.UsageSummary)
		addTotals(&report.Totals, record.UsageSummary)
	}
	for _, row := range usageByKey {
		report.Usage = append(report.Usage, *row)
	}
	sort.Slice(report.Usage, func(i, j int) bool {
		return report.Usage[i].Model < report.Usage[j].Model
	})
	for i := len(r.ledger) - 1; i >= 0 && len(report.Ledger) < filter.Limit; i-- {
		line := r.ledger[i]
		if line.TenantID != filter.TenantID || !timeMatches(line.CreatedAt, filter.From, filter.To) {
			continue
		}
		if filter.ProjectID != "" && line.ProjectID != filter.ProjectID {
			continue
		}
		if filter.Currency != "" && line.Currency != filter.Currency {
			continue
		}
		report.Ledger = append(report.Ledger, line)
	}
	return report, nil
}

// UpsertProviderCostProfile creates or updates a provider cost profile.
func (r *MemoryRepository) UpsertProviderCostProfile(_ context.Context, profile ProviderCostProfile) (*ProviderCostProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if profile.ID == "" {
		profile.ID = newID("cost")
	}
	now := time.Now().UTC()
	if existing, ok := r.profiles[costProfileKey(profile.ProviderType, profile.ChannelID, profile.PublicModel, profile.Currency)]; ok {
		profile.ID = existing.ID
		profile.CreatedAt = existing.CreatedAt
	} else {
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now
	r.profiles[costProfileKey(profile.ProviderType, profile.ChannelID, profile.PublicModel, profile.Currency)] = profile
	return clone(profile), nil
}

// ProviderProfitReport aggregates revenue and provider cost estimates.
func (r *MemoryRepository) ProviderProfitReport(_ context.Context, filter ProviderProfitFilter) (*ProviderProfitReport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	report := &ProviderProfitReport{GeneratedAt: time.Now().UTC(), Filter: filter}
	rows := make(map[string]*ProviderProfitRow)
	for _, record := range r.usage {
		if !timeMatches(record.CreatedAt, filter.From, filter.To) {
			continue
		}
		if filter.TenantID != "" && record.TenantID != filter.TenantID {
			continue
		}
		if filter.ProjectID != "" && record.ProjectID != filter.ProjectID {
			continue
		}
		key := fmt.Sprintf("%s:%s:%s:%s", record.ProviderType, record.ChannelID, record.Model, record.Currency)
		row := rows[key]
		if row == nil {
			row = &ProviderProfitRow{ProviderType: record.ProviderType, ChannelID: record.ChannelID, Model: record.Model, Currency: record.Currency}
			rows[key] = row
		}
		addProfitUsage(row, record.UsageSummary)
	}
	for _, row := range rows {
		profile, ok := r.profiles[costProfileKey(row.ProviderType, row.ChannelID, row.Model, row.Currency)]
		if !ok || !profile.Enabled {
			row.CostProfileMissing = true
		} else {
			row.ProviderCostMicros = providerCostMicros(profile, *row)
		}
		row.ProfitMicros = row.RevenueMicros - row.ProviderCostMicros
		report.Rows = append(report.Rows, *row)
		report.Totals.Requests += row.Requests
		report.Totals.InputTokens += row.InputTokens
		report.Totals.OutputTokens += row.OutputTokens
		report.Totals.TotalTokens += row.TotalTokens
		report.Totals.RevenueMicros += row.RevenueMicros
		report.Totals.ProviderCostMicros += row.ProviderCostMicros
		report.Totals.ProfitMicros += row.ProfitMicros
		report.Totals.Currency = row.Currency
	}
	sort.Slice(report.Rows, func(i, j int) bool {
		return report.Rows[i].ProviderType+report.Rows[i].ChannelID+report.Rows[i].Model < report.Rows[j].ProviderType+report.Rows[j].ChannelID+report.Rows[j].Model
	})
	return report, nil
}

// ReconciliationReport returns balance mismatches and failed settlement backlog.
func (r *MemoryRepository) ReconciliationReport(context.Context) (*ReconciliationReport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var issues []billing.ReconciliationIssue
	for _, balance := range r.balances {
		var debits, credits int64
		for _, line := range r.ledger {
			if line.AccountID != balance.AccountID {
				continue
			}
			if line.AmountMicros < 0 {
				debits += -line.AmountMicros
			} else {
				credits += line.AmountMicros
			}
		}
		expected := balance.OpeningMicros + credits - debits
		actual := balance.AvailableMicros + balance.HeldMicros
		if expected != actual || balance.AvailableMicros < 0 || balance.HeldMicros < 0 {
			issues = append(issues, billing.ReconciliationIssue{
				AccountID:           balance.AccountID,
				Currency:            balance.Currency,
				AvailableMicros:     balance.AvailableMicros,
				HeldMicros:          balance.HeldMicros,
				LedgerDebitsMicros:  debits,
				LedgerCreditsMicros: credits,
				Message:             fmt.Sprintf("balance total %d does not match ledger expected total %d", actual, expected),
			})
		}
	}
	return &ReconciliationReport{
		GeneratedAt:       time.Now().UTC(),
		Issues:            issues,
		FailedSettlements: append([]FailedSettlementSummary(nil), r.failed...),
		BudgetSemantics:   admissionBudgetSemantics(),
	}, nil
}

// CreateManualAdjustment writes an idempotent balance adjustment and ledger line.
func (r *MemoryRepository) CreateManualAdjustment(_ context.Context, request ManualAdjustmentRequest) (*ManualAdjustment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.adjustments[request.IdempotencyKey]; ok {
		return clone(existing), nil
	}
	key := balanceKey(request.TenantID, request.ProjectID, request.Currency)
	balance := r.balances[key]
	if balance.AccountID == "" {
		if request.AmountMicros < 0 {
			return nil, apperr.InsufficientBalance("balance account is missing")
		}
		balance = BalanceSummary{
			AccountID: newID("acct"),
			TenantID:  request.TenantID,
			ProjectID: request.ProjectID,
			Currency:  request.Currency,
		}
	}
	nextAvailable := balance.AvailableMicros + request.AmountMicros
	if nextAvailable < 0 {
		return nil, apperr.InsufficientBalance("manual adjustment would make balance negative")
	}

	// Step 1: update the operator-visible balance snapshot.
	now := time.Now().UTC()
	balance.AvailableMicros = nextAvailable
	balance.UpdatedAt = now
	r.balances[key] = balance

	// Step 2: create the adjustment record and matching ledger entry.
	adjustment := ManualAdjustment{
		ID:             newID("adj"),
		IdempotencyKey: request.IdempotencyKey,
		TenantID:       request.TenantID,
		ProjectID:      request.ProjectID,
		AccountID:      balance.AccountID,
		Currency:       request.Currency,
		AmountMicros:   request.AmountMicros,
		Reason:         request.Reason,
		OperatorID:     request.OperatorID,
		CreatedAt:      now,
	}
	line := LedgerLine{
		ID:                 newID("ledger"),
		RequestID:          adjustment.ID,
		SettlementKind:     "manual_adjustment",
		TenantID:           request.TenantID,
		ProjectID:          request.ProjectID,
		AccountID:          balance.AccountID,
		Currency:           request.Currency,
		AmountMicros:       request.AmountMicros,
		BalanceAfterMicros: nextAvailable,
		Reason:             request.Reason,
		CreatedAt:          now,
	}
	adjustment.LedgerEntryID = line.ID
	r.ledger = append(r.ledger, line)
	r.adjustments[request.IdempotencyKey] = adjustment
	return clone(adjustment), nil
}

// AgentMetadataReport aggregates async task metadata for commercial reports.
func (r *MemoryRepository) AgentMetadataReport(_ context.Context, filter AgentMetadataFilter) (*AgentMetadataReport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	report := &AgentMetadataReport{GeneratedAt: time.Now().UTC(), Filter: filter}
	rows := make(map[string]*AgentMetadataRow)
	for _, task := range r.agentTasks {
		if !timeMatches(task.CreatedAt, filter.From, filter.To) {
			continue
		}
		if filter.TenantID != "" && task.TenantID != filter.TenantID {
			continue
		}
		if filter.ProjectID != "" && task.ProjectID != filter.ProjectID {
			continue
		}
		key := task.Workflow + ":" + task.Scene + ":" + task.Shot + ":" + task.Kind + ":" + task.MediaType + ":" + task.Model
		row := rows[key]
		if row == nil {
			row = &AgentMetadataRow{Workflow: task.Workflow, Scene: task.Scene, Shot: task.Shot, Kind: task.Kind, MediaType: task.MediaType, Model: task.Model, Currency: task.Currency}
			rows[key] = row
		}
		row.Tasks++
		switch task.Status {
		case "succeeded":
			row.Succeeded++
		case "failed":
			row.Failed++
		}
		row.AmountMicros += task.AmountMicros
	}
	for _, row := range rows {
		report.Rows = append(report.Rows, *row)
	}
	sort.Slice(report.Rows, func(i, j int) bool {
		return report.Rows[i].Workflow+report.Rows[i].Scene+report.Rows[i].Shot < report.Rows[j].Workflow+report.Rows[j].Scene+report.Rows[j].Shot
	})
	return report, nil
}

func balanceMatches(balance BalanceSummary, tenantID, projectID, currency string) bool {
	if balance.TenantID != tenantID {
		return false
	}
	if projectID != "" && balance.ProjectID != projectID {
		return false
	}
	return currency == "" || balance.Currency == currency
}

func addUsage(row *UsageSummary, usage UsageSummary) {
	row.Requests += usage.Requests
	row.InputTokens += usage.InputTokens
	row.OutputTokens += usage.OutputTokens
	row.TotalTokens += usage.TotalTokens
	row.AmountMicros += usage.AmountMicros
}

func addTotals(totals *ReportTotals, usage UsageSummary) {
	totals.Requests += usage.Requests
	totals.InputTokens += usage.InputTokens
	totals.OutputTokens += usage.OutputTokens
	totals.TotalTokens += usage.TotalTokens
	totals.RevenueMicros += usage.AmountMicros
	totals.ProfitMicros += usage.AmountMicros
	totals.Currency = usage.Currency
}

func addProfitUsage(row *ProviderProfitRow, usage UsageSummary) {
	row.Requests += usage.Requests
	row.InputTokens += usage.InputTokens
	row.OutputTokens += usage.OutputTokens
	row.TotalTokens += usage.TotalTokens
	row.RevenueMicros += usage.AmountMicros
}

func timeMatches(value, from, to time.Time) bool {
	if !from.IsZero() && value.Before(from) {
		return false
	}
	if !to.IsZero() && !value.Before(to) {
		return false
	}
	return true
}

func balanceKey(tenantID, projectID, currency string) string {
	return tenantID + ":" + projectID + ":" + currency
}

func costProfileKey(providerType, channelID, model, currency string) string {
	return providerType + ":" + channelID + ":" + model + ":" + currency
}

func clone[T any](value T) *T {
	return &value
}

var _ Repository = (*MemoryRepository)(nil)
