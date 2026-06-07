package service

import (
	"context"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// Credits returns customer credits.
func (s *Service) Credits(ctx context.Context, principal portalapp.Principal, currency string) (portalapp.CreditsResponse, error) {
	if s == nil || s.reporting == nil {
		return portalapp.CreditsResponse{}, apperr.ConfigUnavailable("reporting service is unavailable")
	}
	report, err := s.reporting.TenantUsageReport(ctx, reporting.TenantUsageFilter{
		TenantID:  principal.TenantID,
		ProjectID: principal.ProjectID,
		Currency:  currency,
		Limit:     1,
	})
	if err != nil {
		return portalapp.CreditsResponse{}, err
	}
	bucket := creditsBucket(report)
	return portalapp.CreditsResponse{
		Success: true,
		Message: "ok",
		Data: map[string]portalapp.CreditsBucket{
			"token":   bucket,
			"user":    bucket,
			"account": bucket,
		},
	}, nil
}

// CreditLedger returns customer-visible ledger rows.
func (s *Service) CreditLedger(ctx context.Context, principal portalapp.Principal, filter portalapp.UsageFilter) (portalapp.CreditLedgerResponse, error) {
	if s == nil || s.reporting == nil {
		return portalapp.CreditLedgerResponse{}, apperr.ConfigUnavailable("reporting service is unavailable")
	}
	report, err := s.reporting.TenantUsageReport(ctx, reporting.TenantUsageFilter{
		TenantID:     principal.TenantID,
		ProjectID:    principal.ProjectID,
		APIKeyID:     filter.APIKeyID,
		RequestID:    filter.RequestID,
		Model:        filter.Model,
		ProviderType: filter.ProviderType,
		ChannelID:    filter.ChannelID,
		Status:       filter.Status,
		Currency:     filter.Currency,
		From:         filter.From,
		To:           filter.To,
		Limit:        normalizeLimit(filter.Limit),
	})
	if err != nil {
		return portalapp.CreditLedgerResponse{}, err
	}
	return portalLedgerResponse(report), nil
}

func creditsBucket(report *reporting.TenantUsageReport) portalapp.CreditsBucket {
	var remaining, held int64
	currency := ""
	if report != nil {
		for _, balance := range report.Balances {
			remaining += balance.AvailableMicros
			held += balance.HeldMicros
			if currency == "" {
				currency = balance.Currency
			}
		}
		if currency == "" {
			currency = report.Totals.Currency
		}
	}
	if currency == "" {
		currency = "USD"
	}
	used := int64(0)
	if report != nil {
		used = report.Totals.RevenueMicros
	}
	return portalapp.CreditsBucket{
		RemainingCredits: microsToCredits(remaining),
		UsedCredits:      microsToCredits(used),
		HeldCredits:      microsToCredits(held),
		UnlimitedCredits: false,
		Currency:         currency,
	}
}

func microsToCredits(value int64) float64 {
	return float64(value) / 1_000_000
}

func portalLedgerResponse(report *reporting.TenantUsageReport) portalapp.CreditLedgerResponse {
	response := portalapp.CreditLedgerResponse{Items: []portalapp.CreditLedgerItem{}, NextCursor: nil}
	if report == nil {
		return response
	}
	response.GeneratedAt = report.GeneratedAt
	response.Currency = report.Totals.Currency
	for _, line := range report.Ledger {
		response.Items = append(response.Items, portalapp.CreditLedgerItem{
			ID:                  line.ID,
			RequestID:           line.RequestID,
			SettlementKind:      line.SettlementKind,
			Currency:            line.Currency,
			AmountCredits:       microsToCredits(line.AmountMicros),
			BalanceAfterCredits: microsToCredits(line.BalanceAfterMicros),
			Reason:              line.Reason,
			CreatedAt:           line.CreatedAt,
		})
		if response.Currency == "" {
			response.Currency = line.Currency
		}
	}
	if response.Currency == "" {
		response.Currency = report.Filter.Currency
	}
	return response
}
