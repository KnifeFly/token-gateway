package service

import (
	"context"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// Usage returns customer usage.
func (s *Service) Usage(ctx context.Context, principal portalapp.Principal, filter portalapp.UsageFilter) (portalapp.UsageResponse, error) {
	if s == nil || s.reporting == nil {
		return portalapp.UsageResponse{}, apperr.ConfigUnavailable("reporting service is unavailable")
	}
	logReport, err := s.reporting.UsageLogReport(ctx, reporting.UsageLogFilter{
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
		return portalapp.UsageResponse{}, err
	}
	if len(logReport.Rows) > 0 {
		return usageLogResponse(logReport), nil
	}
	if hasUsageLogFilters(filter) {
		return usageLogResponse(logReport), nil
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
		return portalapp.UsageResponse{}, err
	}
	return usageResponse(report), nil
}

func hasUsageLogFilters(filter portalapp.UsageFilter) bool {
	return filter.APIKeyID != "" ||
		filter.RequestID != "" ||
		filter.Model != "" ||
		filter.ProviderType != "" ||
		filter.ChannelID != "" ||
		filter.Status != ""
}

func usageLogResponse(report *reporting.UsageLogReport) portalapp.UsageResponse {
	response := portalapp.UsageResponse{Items: []portalapp.UsageItem{}, NextCursor: nil}
	if report == nil {
		return response
	}
	response.GeneratedAt = report.GeneratedAt
	response.Currency = report.Totals.Currency
	response.Totals = portalapp.UsageTotals{
		Requests:     report.Totals.Requests,
		InputTokens:  report.Totals.InputTokens,
		OutputTokens: report.Totals.OutputTokens,
		CreditsUsed:  microsToCredits(report.Totals.RevenueMicros),
	}
	for _, row := range report.Rows {
		response.Items = append(response.Items, portalapp.UsageItem{
			RequestID:    row.RequestID,
			APIKeyID:     row.APIKeyID,
			Model:        row.Model,
			Capability:   row.ProviderType,
			ProviderType: row.ProviderType,
			ChannelID:    row.ChannelID,
			Status:       row.Status,
			InputTokens:  row.InputTokens,
			OutputTokens: row.OutputTokens,
			TotalTokens:  row.TotalTokens,
			CreditsUsed:  microsToCredits(row.AmountMicros),
			CreatedAt:    row.CreatedAt,
		})
	}
	if response.Currency == "" && len(response.Items) > 0 {
		response.Currency = report.Filter.Currency
	}
	return response
}

func usageResponse(report *reporting.TenantUsageReport) portalapp.UsageResponse {
	response := portalapp.UsageResponse{Items: []portalapp.UsageItem{}, NextCursor: nil}
	if report == nil {
		return response
	}
	response.GeneratedAt = report.GeneratedAt
	response.Currency = report.Totals.Currency
	response.Totals = portalapp.UsageTotals{
		Requests:     report.Totals.Requests,
		InputTokens:  report.Totals.InputTokens,
		OutputTokens: report.Totals.OutputTokens,
		CreditsUsed:  microsToCredits(report.Totals.RevenueMicros),
	}
	for _, row := range report.Usage {
		response.Items = append(response.Items, portalapp.UsageItem{
			Model:        row.Model,
			Capability:   row.ProviderType,
			ProviderType: row.ProviderType,
			ChannelID:    row.ChannelID,
			Status:       "settled",
			InputTokens:  row.InputTokens,
			OutputTokens: row.OutputTokens,
			TotalTokens:  row.TotalTokens,
			CreditsUsed:  microsToCredits(row.AmountMicros),
		})
	}
	if len(response.Items) == 0 {
		for _, line := range report.Ledger {
			response.Items = append(response.Items, portalapp.UsageItem{
				RequestID:   line.RequestID,
				Status:      line.SettlementKind,
				CreditsUsed: microsToCredits(-line.AmountMicros),
				CreatedAt:   line.CreatedAt,
			})
		}
	}
	if response.Currency == "" && len(response.Items) > 0 {
		response.Currency = report.Filter.Currency
	}
	return response
}
