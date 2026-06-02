package reporting

import (
	"context"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

const defaultLedgerLimit = 100

// Repository reads and writes commercial reporting state.
type Repository interface {
	TenantUsageReport(ctx context.Context, filter TenantUsageFilter) (*TenantUsageReport, error)
	UpsertProviderCostProfile(ctx context.Context, profile ProviderCostProfile) (*ProviderCostProfile, error)
	ProviderProfitReport(ctx context.Context, filter ProviderProfitFilter) (*ProviderProfitReport, error)
	ReconciliationReport(ctx context.Context) (*ReconciliationReport, error)
	CreateManualAdjustment(ctx context.Context, request ManualAdjustmentRequest) (*ManualAdjustment, error)
	AgentMetadataReport(ctx context.Context, filter AgentMetadataFilter) (*AgentMetadataReport, error)
}

// Service validates commercial reporting and operator mutation requests.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService returns a commercial reporting service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

// TenantUsageReport returns customer balance, usage, and ledger rows.
func (s *Service) TenantUsageReport(ctx context.Context, filter TenantUsageFilter) (*TenantUsageReport, error) {
	if s == nil || s.repo == nil {
		return nil, apperr.ConfigUnavailable("reporting repository is unavailable")
	}
	filter.TenantID = strings.TrimSpace(filter.TenantID)
	filter.ProjectID = strings.TrimSpace(filter.ProjectID)
	filter.APIKeyID = strings.TrimSpace(filter.APIKeyID)
	filter.Currency = normalizeCurrency(filter.Currency)
	if filter.TenantID == "" {
		return nil, apperr.InvalidArgument("tenant_id is required")
	}
	filter.Limit = normalizeLimit(filter.Limit)
	return s.repo.TenantUsageReport(ctx, filter)
}

// UpsertProviderCostProfile creates or updates provider cost assumptions.
func (s *Service) UpsertProviderCostProfile(ctx context.Context, profile ProviderCostProfile) (*ProviderCostProfile, error) {
	if s == nil || s.repo == nil {
		return nil, apperr.ConfigUnavailable("reporting repository is unavailable")
	}
	profile.ProviderType = strings.TrimSpace(profile.ProviderType)
	profile.ChannelID = strings.TrimSpace(profile.ChannelID)
	profile.PublicModel = strings.TrimSpace(profile.PublicModel)
	profile.Currency = normalizeCurrency(profile.Currency)
	if profile.ProviderType == "" || profile.PublicModel == "" || profile.Currency == "" {
		return nil, apperr.InvalidArgument("provider_type, public_model, and currency are required")
	}
	if profile.InputMicrosPerToken < 0 || profile.OutputMicrosPerToken < 0 || profile.FixedMicrosPerRequest < 0 {
		return nil, apperr.InvalidArgument("provider cost values must be non-negative")
	}
	category, err := pricing.InferCategory(profile.Category, profile.PublicModel)
	if err != nil {
		return nil, apperr.InvalidArgument(err.Error())
	}
	book, err := pricing.NormalizePriceBook(pricing.PriceBook{
		Category:   category,
		Currency:   profile.Currency,
		Components: profile.Components,
	}, pricing.TokenPrice{
		Currency:             profile.Currency,
		InputMicrosPerToken:  profile.InputMicrosPerToken,
		OutputMicrosPerToken: profile.OutputMicrosPerToken,
	})
	if err != nil {
		return nil, apperr.InvalidArgument(err.Error())
	}
	legacy := pricing.LegacyTokenPrice(book.Currency, book.Components)
	profile.Category = string(book.Category)
	profile.Currency = book.Currency
	profile.Components = book.Components
	profile.InputMicrosPerToken = legacy.InputMicrosPerToken
	profile.OutputMicrosPerToken = legacy.OutputMicrosPerToken
	if profile.EffectiveFrom.IsZero() {
		profile.EffectiveFrom = s.now()
	}
	if !profile.Enabled {
		profile.Enabled = true
	}
	return s.repo.UpsertProviderCostProfile(ctx, profile)
}

// ProviderProfitReport returns provider cost, customer revenue, and profit rows.
func (s *Service) ProviderProfitReport(ctx context.Context, filter ProviderProfitFilter) (*ProviderProfitReport, error) {
	if s == nil || s.repo == nil {
		return nil, apperr.ConfigUnavailable("reporting repository is unavailable")
	}
	filter.TenantID = strings.TrimSpace(filter.TenantID)
	filter.ProjectID = strings.TrimSpace(filter.ProjectID)
	return s.repo.ProviderProfitReport(ctx, filter)
}

// ReconciliationReport returns balance/ledger issues and failed settlement state.
func (s *Service) ReconciliationReport(ctx context.Context) (*ReconciliationReport, error) {
	if s == nil || s.repo == nil {
		return nil, apperr.ConfigUnavailable("reporting repository is unavailable")
	}
	return s.repo.ReconciliationReport(ctx)
}

// CreateManualAdjustment writes an idempotent manual ledger correction.
func (s *Service) CreateManualAdjustment(ctx context.Context, request ManualAdjustmentRequest) (*ManualAdjustment, error) {
	if s == nil || s.repo == nil {
		return nil, apperr.ConfigUnavailable("reporting repository is unavailable")
	}
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.ProjectID = strings.TrimSpace(request.ProjectID)
	request.Currency = normalizeCurrency(request.Currency)
	request.Reason = strings.TrimSpace(request.Reason)
	request.OperatorID = strings.TrimSpace(request.OperatorID)
	if request.IdempotencyKey == "" || request.TenantID == "" || request.ProjectID == "" || request.Currency == "" {
		return nil, apperr.InvalidArgument("idempotency_key, tenant_id, project_id, and currency are required")
	}
	if request.AmountMicros == 0 {
		return nil, apperr.InvalidArgument("amount_micros must be non-zero")
	}
	if request.Reason == "" {
		return nil, apperr.InvalidArgument("reason is required")
	}
	if request.OperatorID == "" {
		request.OperatorID = "admin"
	}
	return s.repo.CreateManualAdjustment(ctx, request)
}

// AgentMetadataReport returns workflow, scene, and shot aggregate rows.
func (s *Service) AgentMetadataReport(ctx context.Context, filter AgentMetadataFilter) (*AgentMetadataReport, error) {
	if s == nil || s.repo == nil {
		return nil, apperr.ConfigUnavailable("reporting repository is unavailable")
	}
	filter.TenantID = strings.TrimSpace(filter.TenantID)
	filter.ProjectID = strings.TrimSpace(filter.ProjectID)
	return s.repo.AgentMetadataReport(ctx, filter)
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultLedgerLimit
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func normalizeCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

func providerCostMicros(profile ProviderCostProfile, row ProviderProfitRow) int64 {
	book, err := pricing.NormalizePriceBook(pricing.PriceBook{
		Category:   pricing.Category(profile.Category),
		Currency:   profile.Currency,
		Components: profile.Components,
	}, pricing.TokenPrice{
		Currency:             profile.Currency,
		InputMicrosPerToken:  profile.InputMicrosPerToken,
		OutputMicrosPerToken: profile.OutputMicrosPerToken,
	})
	if err != nil {
		book = pricing.TokenPrice{
			Currency:             profile.Currency,
			InputMicrosPerToken:  profile.InputMicrosPerToken,
			OutputMicrosPerToken: profile.OutputMicrosPerToken,
		}.PriceBook(pricing.CategoryChat)
	}
	amount := book.QuoteMetered(pricing.MeteredUsage{
		InputTokens:  row.InputTokens,
		OutputTokens: row.OutputTokens,
		Requests:     row.Requests,
	})
	if hasCostComponent(profile.Components, pricing.UnitRequest) {
		return amount.Micros
	}
	return amount.Micros + row.Requests*profile.FixedMicrosPerRequest
}

func hasCostComponent(components []pricing.Component, unit pricing.Unit) bool {
	for _, component := range components {
		if component.Unit == unit {
			return true
		}
	}
	return false
}

func admissionBudgetSemantics() BudgetSemantics {
	return BudgetSemantics{
		AdmissionGuardField: "daily_admission_budget_micros",
		ActualSpendSource:   "usage_records + ledger_entries + reconciliation",
		Notes:               "Redis daily admission budget is an estimate guard; customer-visible actual spend is ledger based.",
	}
}
