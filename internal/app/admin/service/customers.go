package service

import (
	"context"
	"sort"
	"strings"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// ListCustomerAccounts returns tenant/project scoped customer account rows.
func (s *Service) ListCustomerAccounts(ctx context.Context, actor adminapp.Actor, filter adminapp.CustomerAccountFilter) (adminapp.ListResponse[adminapp.CustomerAccountView], error) {
	if err := s.Authorize(actor, "read", "customer_account"); err != nil {
		return adminapp.ListResponse[adminapp.CustomerAccountView]{}, err
	}
	accounts, err := s.customerAccountRows(ctx, filter)
	if err != nil {
		return adminapp.ListResponse[adminapp.CustomerAccountView]{}, err
	}
	return adminapp.ListResponse[adminapp.CustomerAccountView]{Data: accounts}, nil
}

// GetCustomerAccount returns one customer account detail.
func (s *Service) GetCustomerAccount(ctx context.Context, actor adminapp.Actor, accountID string) (adminapp.CustomerAccountDetail, error) {
	if err := s.Authorize(actor, "read", "customer_account"); err != nil {
		return adminapp.CustomerAccountDetail{}, err
	}
	tenantID, projectID, err := parseCustomerAccountID(accountID)
	if err != nil {
		return adminapp.CustomerAccountDetail{}, err
	}
	return s.customerAccountDetail(ctx, tenantID, projectID)
}

// CreateCustomerAccount creates a tenant/project scoped customer account.
func (s *Service) CreateCustomerAccount(ctx context.Context, actor adminapp.Actor, request adminapp.CustomerAccountCreateRequest, opts adminapp.MutationOptions) (adminapp.CustomerAccountDetail, error) {
	return mutate(ctx, s, actor, opts, "write", "customer_account", request.ProjectID, request, func() (adminapp.CustomerAccountDetail, error) {
		request.TenantName = strings.TrimSpace(request.TenantName)
		request.ProjectName = strings.TrimSpace(request.ProjectName)
		if request.TenantName == "" || request.ProjectName == "" {
			return adminapp.CustomerAccountDetail{}, apperr.InvalidArgument("tenant_name and project_name are required")
		}

		tenant, err := s.owner.UpsertTenant(ctx, configadmin.Tenant{
			ID:         strings.TrimSpace(request.TenantID),
			Name:       request.TenantName,
			Enabled:    true,
			EnabledSet: true,
		})
		if err != nil {
			return adminapp.CustomerAccountDetail{}, err
		}
		project, err := s.owner.UpsertProject(ctx, configadmin.Project{
			ID:         strings.TrimSpace(request.ProjectID),
			TenantID:   tenant.ID,
			Name:       request.ProjectName,
			Enabled:    true,
			EnabledSet: true,
		})
		if err != nil {
			return adminapp.CustomerAccountDetail{}, err
		}

		if strings.TrimSpace(request.APIKeyName) != "" {
			if _, err := s.owner.CreateAPIKey(ctx, configadmin.APIKey{
				TenantID:      tenant.ID,
				ProjectID:     project.ID,
				Name:          request.APIKeyName,
				AllowedModels: cleanCustomerStrings(request.AllowedModels),
				Enabled:       true,
			}); err != nil {
				return adminapp.CustomerAccountDetail{}, err
			}
		}
		if request.InitialCreditMicros != 0 {
			if err := s.createInitialCustomerCredit(ctx, actor, request, tenant.ID, project.ID, opts); err != nil {
				return adminapp.CustomerAccountDetail{}, err
			}
		}
		return s.customerAccountDetail(ctx, tenant.ID, project.ID)
	})
}

// SetCustomerAccountEnabled updates tenant/project enabled state for one account.
func (s *Service) SetCustomerAccountEnabled(ctx context.Context, actor adminapp.Actor, accountID string, enabled bool, opts adminapp.MutationOptions) (adminapp.CustomerAccountDetail, error) {
	request := map[string]any{"id": accountID, "enabled": enabled}
	return mutate(ctx, s, actor, opts, "write", "customer_account", accountID, request, func() (adminapp.CustomerAccountDetail, error) {
		tenantID, projectID, err := parseCustomerAccountID(accountID)
		if err != nil {
			return adminapp.CustomerAccountDetail{}, err
		}
		detail, err := s.customerAccountDetail(ctx, tenantID, projectID)
		if err != nil {
			return adminapp.CustomerAccountDetail{}, err
		}
		if _, err := s.owner.UpsertProject(ctx, configadmin.Project{
			ID:         projectID,
			TenantID:   tenantID,
			Name:       detail.Account.ProjectName,
			Enabled:    enabled,
			EnabledSet: true,
		}); err != nil {
			return adminapp.CustomerAccountDetail{}, err
		}
		return s.customerAccountDetail(ctx, tenantID, projectID)
	})
}

// AdjustCustomerCredits writes an audited manual credit adjustment through billing reporting.
func (s *Service) AdjustCustomerCredits(ctx context.Context, actor adminapp.Actor, accountID string, request adminapp.CustomerCreditAdjustmentRequest, opts adminapp.MutationOptions) (adminapp.CustomerCreditAdjustmentResult, error) {
	return mutate(ctx, s, actor, opts, "write", "customer_account", accountID, request, func() (adminapp.CustomerCreditAdjustmentResult, error) {
		tenantID, projectID, err := parseCustomerAccountID(accountID)
		if err != nil {
			return adminapp.CustomerCreditAdjustmentResult{}, err
		}
		if s.commercial == nil {
			return adminapp.CustomerCreditAdjustmentResult{}, apperr.ConfigUnavailable("commercial reporting service is unavailable")
		}
		reason := strings.TrimSpace(request.Reason)
		if reason == "" {
			reason = strings.TrimSpace(opts.Reason)
		}
		adjustment, err := s.commercial.CreateManualAdjustment(ctx, reporting.ManualAdjustmentRequest{
			IdempotencyKey: strings.TrimSpace(opts.IdempotencyKey),
			TenantID:       tenantID,
			ProjectID:      projectID,
			Currency:       request.Currency,
			AmountMicros:   request.AmountMicros,
			Reason:         reason,
			OperatorID:     actor.OperatorID,
		})
		if err != nil {
			return adminapp.CustomerCreditAdjustmentResult{}, err
		}
		detail, err := s.customerAccountDetail(ctx, tenantID, projectID)
		if err != nil {
			return adminapp.CustomerCreditAdjustmentResult{}, err
		}
		return adminapp.CustomerCreditAdjustmentResult{Adjustment: adjustment, Account: detail}, nil
	})
}

// GetCustomerCreditReport returns the minimal commercial operations view for one customer account.
func (s *Service) GetCustomerCreditReport(ctx context.Context, actor adminapp.Actor, accountID string, filter adminapp.UsageLogFilter) (adminapp.CustomerCreditReport, error) {
	if err := s.Authorize(actor, "read", "customer_account"); err != nil {
		return adminapp.CustomerCreditReport{}, err
	}
	tenantID, projectID, err := parseCustomerAccountID(accountID)
	if err != nil {
		return adminapp.CustomerCreditReport{}, err
	}
	detail, report, err := s.customerCommercialReport(ctx, tenantID, projectID, filter)
	if err != nil {
		return adminapp.CustomerCreditReport{}, err
	}
	credits := customerCredits(report)
	return adminapp.CustomerCreditReport{
		GeneratedAt:       report.GeneratedAt,
		Account:           detail.Account,
		Credits:           credits,
		Usage:             customerUsageRows(report),
		Ledger:            customerLedgerLines(report),
		ActiveHolds:       customerHoldSummaries(credits),
		FailedSettlements: s.customerFailedSettlementLinks(ctx, tenantID, projectID),
		Exports:           customerExportLinks(accountID),
	}, nil
}

// ExportCustomerUsage returns a safe JSON usage export for one customer account.
func (s *Service) ExportCustomerUsage(ctx context.Context, actor adminapp.Actor, accountID string, filter adminapp.UsageLogFilter) (adminapp.CustomerReportExport, error) {
	if err := s.Authorize(actor, "read", "customer_account"); err != nil {
		return adminapp.CustomerReportExport{}, err
	}
	tenantID, projectID, err := parseCustomerAccountID(accountID)
	if err != nil {
		return adminapp.CustomerReportExport{}, err
	}
	_, report, err := s.customerCommercialReport(ctx, tenantID, projectID, filter)
	if err != nil {
		return adminapp.CustomerReportExport{}, err
	}
	return adminapp.CustomerReportExport{
		GeneratedAt: report.GeneratedAt,
		Kind:        "usage",
		Format:      "json",
		Filename:    safeExportFilename(accountID, "usage"),
		AccountID:   accountID,
		TenantID:    tenantID,
		ProjectID:   projectID,
		Currency:    report.Totals.Currency,
		Usage:       customerUsageRows(report),
		Totals:      customerUsageSummary(report),
		SafeFields:  []string{"model", "provider_type", "channel_id", "requests", "tokens", "amount_micros"},
		Metadata:    map[string]string{"scope": "admin_customer_account"},
	}, nil
}

// ExportCustomerLedger returns a safe JSON ledger export for one customer account.
func (s *Service) ExportCustomerLedger(ctx context.Context, actor adminapp.Actor, accountID string, filter adminapp.UsageLogFilter) (adminapp.CustomerReportExport, error) {
	if err := s.Authorize(actor, "read", "customer_account"); err != nil {
		return adminapp.CustomerReportExport{}, err
	}
	tenantID, projectID, err := parseCustomerAccountID(accountID)
	if err != nil {
		return adminapp.CustomerReportExport{}, err
	}
	_, report, err := s.customerCommercialReport(ctx, tenantID, projectID, filter)
	if err != nil {
		return adminapp.CustomerReportExport{}, err
	}
	return adminapp.CustomerReportExport{
		GeneratedAt: report.GeneratedAt,
		Kind:        "ledger",
		Format:      "json",
		Filename:    safeExportFilename(accountID, "ledger"),
		AccountID:   accountID,
		TenantID:    tenantID,
		ProjectID:   projectID,
		Currency:    report.Totals.Currency,
		Ledger:      customerLedgerLines(report),
		Totals:      customerUsageSummary(report),
		SafeFields:  []string{"request_id", "settlement_kind", "currency", "amount_micros", "balance_after_micros", "reason"},
		Metadata:    map[string]string{"scope": "admin_customer_account"},
	}, nil
}

// ResetCustomerPortalSessions revokes Portal browser sessions for one account.
func (s *Service) ResetCustomerPortalSessions(ctx context.Context, actor adminapp.Actor, accountID string, apiKeyID string, opts adminapp.MutationOptions) (adminapp.CustomerSessionResetResult, error) {
	request := map[string]string{"id": accountID, "api_key_id": apiKeyID}
	return mutate(ctx, s, actor, opts, "write", "customer_account", accountID, request, func() (adminapp.CustomerSessionResetResult, error) {
		tenantID, projectID, err := parseCustomerAccountID(accountID)
		if err != nil {
			return adminapp.CustomerSessionResetResult{}, err
		}
		if _, err := s.customerAccountDetail(ctx, tenantID, projectID); err != nil {
			return adminapp.CustomerSessionResetResult{}, err
		}
		resetAt := s.now()
		revoked := 0
		if s.portalSessions != nil {
			revoked, err = s.portalSessions.ResetPortalSessions(ctx, PortalSessionResetFilter{
				TenantID:  tenantID,
				ProjectID: projectID,
				APIKeyID:  strings.TrimSpace(apiKeyID),
				RevokedAt: resetAt,
			})
			if err != nil {
				return adminapp.CustomerSessionResetResult{}, err
			}
		}
		return adminapp.CustomerSessionResetResult{
			CustomerAccountID: customerAccountID(tenantID, projectID),
			TenantID:          tenantID,
			ProjectID:         projectID,
			APIKeyID:          strings.TrimSpace(apiKeyID),
			RevokedSessions:   revoked,
			ResetAt:           resetAt,
		}, nil
	})
}

func (s *Service) customerAccountRows(ctx context.Context, filter adminapp.CustomerAccountFilter) ([]adminapp.CustomerAccountView, error) {
	tenants, err := s.owner.ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	projects, err := s.owner.ListProjects(ctx, strings.TrimSpace(filter.TenantID))
	if err != nil {
		return nil, err
	}
	tenantByID := make(map[string]configadmin.Tenant, len(tenants))
	for _, tenant := range tenants {
		tenantByID[tenant.ID] = tenant
	}

	var out []adminapp.CustomerAccountView
	for _, project := range projects {
		if strings.TrimSpace(filter.ProjectID) != "" && project.ID != strings.TrimSpace(filter.ProjectID) {
			continue
		}
		tenant, ok := tenantByID[project.TenantID]
		if !ok {
			continue
		}
		view, err := s.customerAccountView(ctx, tenant, project)
		if err != nil {
			return nil, err
		}
		if !customerAccountFilterMatches(view, filter) {
			continue
		}
		out = append(out, view)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Status == out[j].Status {
			return out[i].CustomerAccountID < out[j].CustomerAccountID
		}
		return out[i].Status < out[j].Status
	})
	return out, nil
}

func (s *Service) customerAccountDetail(ctx context.Context, tenantID string, projectID string) (adminapp.CustomerAccountDetail, error) {
	rows, err := s.customerAccountRows(ctx, adminapp.CustomerAccountFilter{TenantID: tenantID, ProjectID: projectID})
	if err != nil {
		return adminapp.CustomerAccountDetail{}, err
	}
	if len(rows) == 0 {
		return adminapp.CustomerAccountDetail{}, apperr.NotFound("customer account not found")
	}
	keys, err := s.owner.ListAPIKeys(ctx, tenantID, projectID)
	if err != nil {
		return adminapp.CustomerAccountDetail{}, err
	}
	report, err := s.customerUsageReport(ctx, tenantID, projectID)
	if err != nil {
		return adminapp.CustomerAccountDetail{}, err
	}

	apiKeys := make([]adminapp.APIKeyView, 0, len(keys))
	for _, key := range keys {
		apiKeys = append(apiKeys, safeAPIKey(key))
	}
	return adminapp.CustomerAccountDetail{
		Account: rows[0],
		APIKeys: apiKeys,
		Usage:   customerUsageRows(report),
		Ledger:  customerLedgerLines(report),
	}, nil
}

func (s *Service) customerAccountView(ctx context.Context, tenant configadmin.Tenant, project configadmin.Project) (adminapp.CustomerAccountView, error) {
	keys, err := s.owner.ListAPIKeys(ctx, tenant.ID, project.ID)
	if err != nil {
		return adminapp.CustomerAccountView{}, err
	}
	report, err := s.customerUsageReport(ctx, tenant.ID, project.ID)
	if err != nil {
		return adminapp.CustomerAccountView{}, err
	}
	activeKeys := 0
	for _, key := range keys {
		if key.Enabled && key.RevokedAt == nil {
			activeKeys++
		}
	}
	return adminapp.CustomerAccountView{
		CustomerAccountID: customerAccountID(tenant.ID, project.ID),
		TenantID:          tenant.ID,
		TenantName:        tenant.Name,
		ProjectID:         project.ID,
		ProjectName:       project.Name,
		DisplayName:       project.Name,
		Status:            customerAccountStatus(tenant, project),
		Role:              "owner",
		TenantEnabled:     tenant.Enabled,
		ProjectEnabled:    project.Enabled,
		APIKeyCount:       len(keys),
		ActiveAPIKeyCount: activeKeys,
		AllowedModels:     customerAllowedModels(keys),
		Credits:           customerCredits(report),
		RecentUsage:       customerUsageSummary(report),
		CreatedAt:         project.CreatedAt,
		UpdatedAt:         project.UpdatedAt,
	}, nil
}

func (s *Service) customerUsageReport(ctx context.Context, tenantID string, projectID string) (*reporting.TenantUsageReport, error) {
	if s.commercial == nil {
		return &reporting.TenantUsageReport{}, nil
	}
	return s.commercial.TenantUsageReport(ctx, reporting.TenantUsageFilter{
		TenantID:  tenantID,
		ProjectID: projectID,
		Limit:     20,
	})
}

func (s *Service) createInitialCustomerCredit(ctx context.Context, actor adminapp.Actor, request adminapp.CustomerAccountCreateRequest, tenantID string, projectID string, opts adminapp.MutationOptions) error {
	if s.commercial == nil {
		return apperr.ConfigUnavailable("commercial reporting service is unavailable")
	}
	reason := strings.TrimSpace(request.InitialCreditReason)
	if reason == "" {
		reason = strings.TrimSpace(opts.Reason)
	}
	_, err := s.commercial.CreateManualAdjustment(ctx, reporting.ManualAdjustmentRequest{
		IdempotencyKey: strings.TrimSpace(opts.IdempotencyKey),
		TenantID:       tenantID,
		ProjectID:      projectID,
		Currency:       request.Currency,
		AmountMicros:   request.InitialCreditMicros,
		Reason:         reason,
		OperatorID:     actor.OperatorID,
	})
	return err
}

func (s *Service) customerCommercialReport(ctx context.Context, tenantID string, projectID string, filter adminapp.UsageLogFilter) (adminapp.CustomerAccountDetail, *reporting.TenantUsageReport, error) {
	detail, err := s.customerAccountDetail(ctx, tenantID, projectID)
	if err != nil {
		return adminapp.CustomerAccountDetail{}, nil, err
	}
	if s.commercial == nil {
		return detail, &reporting.TenantUsageReport{GeneratedAt: s.now()}, nil
	}
	report, err := s.commercial.TenantUsageReport(ctx, reporting.TenantUsageFilter{
		TenantID:     tenantID,
		ProjectID:    projectID,
		APIKeyID:     filter.APIKeyID,
		RequestID:    filter.RequestID,
		Model:        filter.Model,
		ProviderType: filter.ProviderType,
		ChannelID:    filter.ChannelID,
		Status:       filter.Status,
		Currency:     filter.Currency,
		From:         filter.From,
		To:           filter.To,
		Limit:        filter.Limit,
	})
	if err != nil {
		return adminapp.CustomerAccountDetail{}, nil, err
	}
	return detail, report, nil
}

func (s *Service) customerFailedSettlementLinks(ctx context.Context, tenantID string, projectID string) []adminapp.CustomerFailedSettlementLink {
	if s.commercial == nil {
		return nil
	}
	report, err := s.commercial.ReconciliationReport(ctx)
	if err != nil {
		return nil
	}
	links := make([]adminapp.CustomerFailedSettlementLink, 0)
	for _, failed := range report.FailedSettlements {
		if failed.TenantID != tenantID || failed.ProjectID != projectID {
			continue
		}
		links = append(links, adminapp.CustomerFailedSettlementLink{
			ID:         failed.ID,
			RequestID:  failed.RequestID,
			Status:     failed.Status,
			RetryCount: failed.RetryCount,
			LastError:  safeShort(failed.LastError),
			UpdatedAt:  failed.UpdatedAt,
		})
	}
	return links
}

func customerAccountID(tenantID string, projectID string) string {
	return strings.TrimSpace(tenantID) + ":" + strings.TrimSpace(projectID)
}

func parseCustomerAccountID(accountID string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(accountID), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", apperr.InvalidArgument("customer_account_id must be tenant_id:project_id")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func customerAccountStatus(tenant configadmin.Tenant, project configadmin.Project) string {
	if !tenant.Enabled || !project.Enabled {
		return "disabled"
	}
	return "active"
}

func customerAllowedModels(keys []configadmin.APIKey) adminapp.CustomerAllowedModels {
	seen := map[string]struct{}{}
	wildcard := false
	for _, key := range keys {
		if !key.Enabled || key.RevokedAt != nil {
			continue
		}
		for _, model := range key.AllowedModels {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			if model == "*" {
				wildcard = true
				continue
			}
			seen[model] = struct{}{}
		}
	}
	models := make([]string, 0, len(seen))
	for model := range seen {
		models = append(models, model)
	}
	sort.Strings(models)
	return adminapp.CustomerAllowedModels{Models: models, Wildcard: wildcard, UniqueCount: len(models)}
}

func customerCredits(report *reporting.TenantUsageReport) []adminapp.CustomerCreditSummary {
	if report == nil {
		return nil
	}
	grantedByCurrency := map[string]int64{}
	usedByCurrency := map[string]int64{}
	for _, line := range report.Ledger {
		if line.AmountMicros > 0 {
			grantedByCurrency[line.Currency] += line.AmountMicros
		} else {
			usedByCurrency[line.Currency] += -line.AmountMicros
		}
	}
	out := make([]adminapp.CustomerCreditSummary, 0, len(report.Balances))
	for _, balance := range report.Balances {
		out = append(out, adminapp.CustomerCreditSummary{
			AccountID:          balance.AccountID,
			Currency:           balance.Currency,
			AvailableMicros:    balance.AvailableMicros,
			HeldMicros:         balance.HeldMicros,
			OpeningMicros:      balance.OpeningMicros,
			TotalGrantedMicros: balance.OpeningMicros + grantedByCurrency[balance.Currency],
			UsedMicros:         usedByCurrency[balance.Currency],
		})
	}
	return out
}

func customerUsageSummary(report *reporting.TenantUsageReport) adminapp.CustomerUsageSummary {
	if report == nil {
		return adminapp.CustomerUsageSummary{}
	}
	return adminapp.CustomerUsageSummary{
		Requests:      report.Totals.Requests,
		InputTokens:   report.Totals.InputTokens,
		OutputTokens:  report.Totals.OutputTokens,
		RevenueMicros: report.Totals.RevenueMicros,
		Currency:      report.Totals.Currency,
	}
}

func customerUsageRows(report *reporting.TenantUsageReport) []adminapp.CustomerUsageRow {
	if report == nil {
		return nil
	}
	out := make([]adminapp.CustomerUsageRow, 0, len(report.Usage))
	for _, row := range report.Usage {
		out = append(out, adminapp.CustomerUsageRow{
			Model:        row.Model,
			ProviderType: row.ProviderType,
			ChannelID:    row.ChannelID,
			Currency:     row.Currency,
			Requests:     row.Requests,
			InputTokens:  row.InputTokens,
			OutputTokens: row.OutputTokens,
			TotalTokens:  row.TotalTokens,
			AmountMicros: row.AmountMicros,
		})
	}
	return out
}

func customerLedgerLines(report *reporting.TenantUsageReport) []adminapp.CustomerLedgerLine {
	if report == nil {
		return nil
	}
	out := make([]adminapp.CustomerLedgerLine, 0, len(report.Ledger))
	for _, line := range report.Ledger {
		out = append(out, adminapp.CustomerLedgerLine{
			ID:                 line.ID,
			RequestID:          line.RequestID,
			SettlementKind:     line.SettlementKind,
			AccountID:          line.AccountID,
			Currency:           line.Currency,
			AmountMicros:       line.AmountMicros,
			BalanceAfterMicros: line.BalanceAfterMicros,
			Reason:             line.Reason,
			CreatedAt:          line.CreatedAt,
		})
	}
	return out
}

func customerHoldSummaries(credits []adminapp.CustomerCreditSummary) []adminapp.HoldAgingView {
	var active int
	for _, credit := range credits {
		if credit.HeldMicros > 0 {
			active++
		}
	}
	return []adminapp.HoldAgingView{
		{Status: "active", Count: active},
		{Status: "protected_task_holds", Count: 0},
	}
}

func customerExportLinks(accountID string) []adminapp.CustomerExportLink {
	return []adminapp.CustomerExportLink{
		{
			Kind:       "usage",
			Path:       "/api/admin/v1/customer-accounts/" + accountID + "/usage/export",
			Format:     "json",
			SafeFields: []string{"model", "provider_type", "channel_id", "requests", "tokens", "amount_micros"},
		},
		{
			Kind:       "ledger",
			Path:       "/api/admin/v1/customer-accounts/" + accountID + "/ledger/export",
			Format:     "json",
			SafeFields: []string{"request_id", "settlement_kind", "currency", "amount_micros", "balance_after_micros", "reason"},
		},
	}
}

func safeExportFilename(accountID string, kind string) string {
	filename := strings.ReplaceAll(accountID, ":", "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	return "customer_" + filename + "_" + kind + ".json"
}

func customerAccountFilterMatches(view adminapp.CustomerAccountView, filter adminapp.CustomerAccountFilter) bool {
	if strings.TrimSpace(filter.Status) != "" && view.Status != strings.TrimSpace(filter.Status) {
		return false
	}
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	if keyword == "" {
		return true
	}
	haystack := strings.ToLower(view.CustomerAccountID + " " + view.TenantName + " " + view.ProjectName + " " + view.DisplayName + " " + view.Email)
	return strings.Contains(haystack, keyword)
}

func cleanCustomerStrings(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
