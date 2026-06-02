package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	adminrepo "github.com/KnifeFly/token-gateway/internal/app/admin/repository"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
)

func TestLoginSessionAndCSRF(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)

	login, err := svc.Login(ctx, adminapp.LoginRequest{Email: "admin@example.com", Password: "admin-local"}, "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if !login.Authenticated || login.Session.SessionID == "" || login.CSRFToken == "" {
		t.Fatalf("login response missing session metadata: %#v", login)
	}
	if _, _, err := svc.Session(ctx, login.Session.SessionID); err != nil {
		t.Fatalf("Session() error = %v", err)
	}
	if err := svc.ValidateCSRF(ctx, login.Session.SessionID, "wrong"); err == nil {
		t.Fatal("ValidateCSRF(wrong) succeeded")
	}
	if err := svc.ValidateCSRF(ctx, login.Session.SessionID, login.CSRFToken); err != nil {
		t.Fatalf("ValidateCSRF(valid) error = %v", err)
	}
}

func TestRBACDenyWritesFailedAudit(t *testing.T) {
	ctx := context.Background()
	svc, repo := testService(t)
	if _, err := svc.EnsureBootstrapOperator(ctx, "viewer@example.com", "viewer-local", []adminapp.Role{adminapp.RoleReadOnly}); err != nil {
		t.Fatalf("EnsureBootstrapOperator(viewer) error = %v", err)
	}
	login, err := svc.Login(ctx, adminapp.LoginRequest{Email: "viewer@example.com", Password: "viewer-local"}, "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login(viewer) error = %v", err)
	}
	_, actor, err := svc.Session(ctx, login.Session.SessionID)
	if err != nil {
		t.Fatalf("Session(viewer) error = %v", err)
	}

	_, err = svc.UpsertTenant(ctx, actor, configadmin.Tenant{Name: "Denied"}, adminapp.MutationOptions{
		RequestID:      "req_rbac",
		IdempotencyKey: "idem_rbac",
		Reason:         "rbac test",
	})
	if err == nil {
		t.Fatal("UpsertTenant(read_only) succeeded")
	}
	events, err := repo.ListAuditEvents(ctx, adminapp.AuditFilter{Resource: "tenant"})
	if err != nil {
		t.Fatalf("ListAuditEvents() error = %v", err)
	}
	if len(events) != 1 || events[0].Status != "failed" || events[0].ErrorCode != "forbidden" {
		t.Fatalf("audit event = %#v", events)
	}
}

func TestMutationAuditRedactsSecrets(t *testing.T) {
	ctx := context.Background()
	svc, repo := testService(t)
	login, err := svc.Login(ctx, adminapp.LoginRequest{Email: "admin@example.com", Password: "admin-local"}, "ua-secret", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	_, actor, err := svc.Session(ctx, login.Session.SessionID)
	if err != nil {
		t.Fatalf("Session() error = %v", err)
	}
	created, err := svc.CreateAPIKey(ctx, actor, adminapp.APIKeyCreateRequest{
		TenantID:  "tenant_1",
		ProjectID: "project_1",
		Name:      "secret key",
	}, adminapp.MutationOptions{
		RequestID:      "req_key",
		IdempotencyKey: "idem_key",
		Reason:         "create derived key",
		UserAgent:      "ua-secret",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if created.PlaintextKey == "" {
		t.Fatal("CreateAPIKey() returned empty one-time plaintext")
	}
	events, err := repo.ListAuditEvents(ctx, adminapp.AuditFilter{Resource: "api_key"})
	if err != nil {
		t.Fatalf("ListAuditEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	combined := string(events[0].Before) + string(events[0].After)
	if strings.Contains(combined, created.PlaintextKey) || strings.Contains(combined, "key_hash") || strings.Contains(combined, "sha256:") {
		t.Fatalf("audit event leaked key material: before=%s after=%s", events[0].Before, events[0].After)
	}
}

func TestAPIKeyManagementWorkflowsReturnSafeViews(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)
	login, err := svc.Login(ctx, adminapp.LoginRequest{Email: "admin@example.com", Password: "admin-local"}, "ua-key", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	_, actor, err := svc.Session(ctx, login.Session.SessionID)
	if err != nil {
		t.Fatalf("Session() error = %v", err)
	}
	expiresAt := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)

	created, err := svc.CreateAPIKey(ctx, actor, adminapp.APIKeyCreateRequest{
		TenantID:      "tenant_1",
		ProjectID:     "project_1",
		Name:          "primary",
		AllowedModels: []string{"gpt-4o-mini"},
		IPAllowlist:   []string{"203.0.113.10", "2001:db8::/32"},
		ExpiresAt:     &expiresAt,
	}, mutationOptions("api_key_create"))
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if created.PlaintextKey == "" || created.APIKey.Fingerprint == "" {
		t.Fatalf("created key missing one-time plaintext or fingerprint: %#v", created)
	}
	if len(created.APIKey.AllowedModels) != 1 || created.APIKey.AllowedModels[0] != "gpt-4o-mini" {
		t.Fatalf("allowed models = %#v", created.APIKey.AllowedModels)
	}
	if len(created.APIKey.IPAllowlist) != 2 || created.APIKey.ExpiresAt == nil {
		t.Fatalf("metadata = %#v", created.APIKey)
	}

	updated, err := svc.UpdateAPIKey(ctx, actor, created.APIKey.ID, adminapp.APIKeyUpdateRequest{Name: "renamed"}, mutationOptions("api_key_update"))
	if err != nil {
		t.Fatalf("UpdateAPIKey() error = %v", err)
	}
	if updated.Name != "renamed" || len(updated.AllowedModels) != 1 || updated.AllowedModels[0] != "gpt-4o-mini" {
		t.Fatalf("updated key expanded or lost scope: %#v", updated)
	}

	rotated, err := svc.RotateAPIKey(ctx, actor, created.APIKey.ID, adminapp.APIKeyRotateRequest{}, mutationOptions("api_key_rotate"))
	if err != nil {
		t.Fatalf("RotateAPIKey() error = %v", err)
	}
	if rotated.PlaintextKey == "" || rotated.PlaintextKey == created.PlaintextKey {
		t.Fatalf("rotated plaintext = %q original=%q", rotated.PlaintextKey, created.PlaintextKey)
	}

	disabled, err := svc.DisableAPIKey(ctx, actor, created.APIKey.ID, mutationOptions("api_key_disable"))
	if err != nil {
		t.Fatalf("DisableAPIKey() error = %v", err)
	}
	if disabled.Enabled || disabled.RevokedAt == nil {
		t.Fatalf("disabled key = %#v", disabled)
	}

	enabled, err := svc.EnableAPIKey(ctx, actor, created.APIKey.ID, mutationOptions("api_key_enable"))
	if err != nil {
		t.Fatalf("EnableAPIKey() error = %v", err)
	}
	if !enabled.Enabled || enabled.RevokedAt != nil {
		t.Fatalf("enabled key = %#v", enabled)
	}

	list, err := svc.ListAPIKeys(ctx, actor, "tenant_1", "project_1")
	if err != nil {
		t.Fatalf("ListAPIKeys() error = %v", err)
	}
	content := mustJSON(t, list)
	if strings.Contains(content, created.PlaintextKey) || strings.Contains(content, rotated.PlaintextKey) || strings.Contains(content, "key_hash") {
		t.Fatalf("safe list leaked key material: %s", content)
	}
}

func TestChannelManagementWorkflowsReturnSafeViews(t *testing.T) {
	ctx := context.Background()
	svc, repo := testService(t)
	login, err := svc.Login(ctx, adminapp.LoginRequest{Email: "admin@example.com", Password: "admin-local"}, "ua-channel", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	_, actor, err := svc.Session(ctx, login.Session.SessionID)
	if err != nil {
		t.Fatalf("Session() error = %v", err)
	}

	channel, err := svc.UpsertChannel(ctx, actor, configadmin.ChannelConfig{
		ID:           "channel_primary",
		ProviderType: "openai",
		BaseURL:      "https://provider.example/v1",
		APIKey:       "sk_live_secret",
		Enabled:      true,
		EnabledSet:   true,
		Models: []configadmin.ChannelModel{{
			PublicModel:      "gpt-public",
			UpstreamModel:    "gpt-upstream",
			HealthStatus:     "healthy",
			TestStatus:       "passed",
			CostConfigStatus: "configured",
		}},
	}, mutationOptions("channel_create"))
	if err != nil {
		t.Fatalf("UpsertChannel() error = %v", err)
	}
	if !channel.CredentialConfigured || channel.ModelCount != 1 || channel.HealthStatus != "healthy" {
		t.Fatalf("channel safe view = %#v", channel)
	}
	assertNoChannelSecret(t, channel, "sk_live_secret")

	rotated, err := svc.RotateChannelCredential(ctx, actor, "channel_primary", "sk_rotated_secret", mutationOptions("channel_rotate"))
	if err != nil {
		t.Fatalf("RotateChannelCredential() error = %v", err)
	}
	assertNoChannelSecret(t, rotated, "sk_rotated_secret")

	testResult, err := svc.TestChannel(ctx, actor, "channel_primary", mutationOptions("channel_test"))
	if err != nil {
		t.Fatalf("TestChannel() error = %v", err)
	}
	if testResult.Status != "ready" || !testResult.CredentialConfigured {
		t.Fatalf("test result = %#v", testResult)
	}

	preview, err := svc.PreviewChannelModelSync(ctx, actor, configadmin.ChannelModelSyncPreviewRequest{
		ChannelID: "channel_primary",
		UpstreamModels: []configadmin.ChannelModel{{
			PublicModel:      "gpt-public",
			UpstreamModel:    "gpt-upstream-v2",
			CostConfigStatus: "configured",
		}},
	}, mutationOptions("channel_sync_preview"))
	if err != nil {
		t.Fatalf("PreviewChannelModelSync() error = %v", err)
	}
	if len(preview.Changed) != 1 {
		t.Fatalf("preview = %#v", preview)
	}

	applied, err := svc.ApplyChannelModelSync(ctx, actor, configadmin.ChannelModelSyncPreviewRequest{
		ChannelID: "channel_primary",
		UpstreamModels: []configadmin.ChannelModel{{
			PublicModel:      "gpt-public",
			UpstreamModel:    "gpt-upstream-v2",
			CostConfigStatus: "configured",
		}},
	}, mutationOptions("channel_sync_apply"))
	if err != nil {
		t.Fatalf("ApplyChannelModelSync() error = %v", err)
	}
	if applied.Channel.ModelCount != 1 || applied.Channel.Models[0].UpstreamModel != "gpt-upstream-v2" {
		t.Fatalf("sync apply = %#v", applied)
	}

	events, err := svc.ListChannelHealthEvents(ctx, actor, "channel_primary")
	if err != nil {
		t.Fatalf("ListChannelHealthEvents() error = %v", err)
	}
	if len(events.Data) < 2 || events.Data[0].ChannelID != "channel_primary" {
		t.Fatalf("health events = %#v", events)
	}
	auditEvents, err := repo.ListAuditEvents(ctx, adminapp.AuditFilter{Resource: "channel"})
	if err != nil {
		t.Fatalf("ListAuditEvents(channel) error = %v", err)
	}
	if len(auditEvents) < 5 {
		t.Fatalf("audit event count = %d, want >= 5", len(auditEvents))
	}
	for _, event := range auditEvents {
		combined := string(event.Before) + string(event.After)
		if strings.Contains(combined, "sk_live_secret") || strings.Contains(combined, "sk_rotated_secret") {
			t.Fatalf("audit leaked channel secret: %#v", event)
		}
	}
}

func TestModelManagementWorkflowsReturnSafeViews(t *testing.T) {
	ctx := context.Background()
	svc, repo := testService(t)
	login, err := svc.Login(ctx, adminapp.LoginRequest{Email: "admin@example.com", Password: "admin-local"}, "ua-model", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	_, actor, err := svc.Session(ctx, login.Session.SessionID)
	if err != nil {
		t.Fatalf("Session() error = %v", err)
	}

	model, err := svc.UpsertModel(ctx, actor, configadmin.ModelConfig{
		PublicModel:     "gpt-public",
		DisplayName:     "GPT Public",
		Protocol:        "native_openai",
		Capability:      "chat",
		Category:        "chat",
		Modalities:      []string{"text"},
		Capabilities:    []string{"chat", "tool"},
		ContextWindow:   8192,
		MaxOutputTokens: 1024,
		Metadata:        json.RawMessage(`{"owner":"ops","credential":"model-secret"}`),
		Schema:          json.RawMessage(`{"type":"object","required":["model"]}`),
	}, mutationOptions("model_create"))
	if err != nil {
		t.Fatalf("UpsertModel() error = %v", err)
	}
	if model.PublicModel != "gpt-public" || !model.Enabled || !model.SchemaAvailable {
		t.Fatalf("model safe view = %#v", model)
	}
	if strings.Contains(string(model.Metadata), "model-secret") {
		t.Fatalf("model metadata leaked secret: %s", model.Metadata)
	}

	if _, err := svc.UpsertPrice(ctx, actor, configadmin.PriceRuleConfig{
		PublicModel: "gpt-public",
		Category:    "chat",
		Currency:    "CNY",
		Components: []pricing.Component{{
			Unit:          pricing.UnitInputToken,
			MicrosPerUnit: 2,
		}},
	}, mutationOptions("model_price")); err != nil {
		t.Fatalf("UpsertPrice() error = %v", err)
	}
	if _, err := svc.UpsertChannel(ctx, actor, configadmin.ChannelConfig{
		ID:           "channel_primary",
		ProviderType: "openai",
		BaseURL:      "https://provider.example/v1",
		APIKey:       "sk_model_secret",
		Enabled:      true,
		EnabledSet:   true,
		Models: []configadmin.ChannelModel{{
			PublicModel:         "gpt-public",
			UpstreamModel:       "gpt-upstream",
			Capabilities:        []string{"chat"},
			SupportedParameters: []string{"temperature"},
			HealthStatus:        "healthy",
			TestStatus:          "passed",
			CostConfigStatus:    "configured",
		}},
	}, mutationOptions("model_channel")); err != nil {
		t.Fatalf("UpsertChannel() error = %v", err)
	}

	detail, err := svc.GetModel(ctx, actor, "gpt-public")
	if err != nil {
		t.Fatalf("GetModel() error = %v", err)
	}
	if !detail.PricingSummary.Configured || detail.PricingSummary.Currency != "CNY" || len(detail.ChannelCoverage) != 1 {
		t.Fatalf("model detail = %#v", detail)
	}
	if strings.Contains(mustJSON(t, detail), "ratio") || strings.Contains(mustJSON(t, detail), "sk_model_secret") {
		t.Fatalf("model detail exposed cut-scope or secret fields: %s", mustJSON(t, detail))
	}

	schema, err := svc.GetModelSchemaPreview(ctx, actor, "gpt-public")
	if err != nil {
		t.Fatalf("GetModelSchemaPreview() error = %v", err)
	}
	if schema.Model != "gpt-public" || schema.Schema["type"] != "object" {
		t.Fatalf("schema preview = %#v", schema)
	}

	channels, err := svc.ListModelChannels(ctx, actor, "gpt-public")
	if err != nil {
		t.Fatalf("ListModelChannels() error = %v", err)
	}
	if len(channels.Data) != 1 || channels.Data[0].UpstreamModel != "gpt-upstream" {
		t.Fatalf("model channels = %#v", channels)
	}

	preview, err := svc.PreviewModelCatalogSync(ctx, actor, adminapp.ModelCatalogSyncPreviewRequest{
		Models: []adminapp.ModelCatalogSyncModel{{
			PublicModel:  "gpt-public",
			DisplayName:  "GPT Public Updated",
			Protocol:     "native_openai",
			Capability:   "chat",
			Category:     "chat",
			Capabilities: []string{"chat", "tool"},
		}, {
			PublicModel: "image-public",
			Protocol:    "unified",
			Capability:  "image",
		}},
	}, mutationOptions("model_sync_preview"))
	if err != nil {
		t.Fatalf("PreviewModelCatalogSync() error = %v", err)
	}
	if len(preview.Added) != 1 || len(preview.Changed) != 1 || preview.Added[0].PublicModel != "image-public" {
		t.Fatalf("catalog preview = %#v", preview)
	}

	deprecated, err := svc.DeprecateModel(ctx, actor, "gpt-public", mutationOptions("model_deprecate"))
	if err != nil {
		t.Fatalf("DeprecateModel() error = %v", err)
	}
	if !deprecated.Deprecated || deprecated.Status != "deprecated" {
		t.Fatalf("deprecated model = %#v", deprecated)
	}

	disabled, err := svc.SetModelEnabled(ctx, actor, "gpt-public", false, mutationOptions("model_disable"))
	if err != nil {
		t.Fatalf("SetModelEnabled() error = %v", err)
	}
	if disabled.Enabled {
		t.Fatalf("disabled model = %#v", disabled)
	}

	auditEvents, err := repo.ListAuditEvents(ctx, adminapp.AuditFilter{Resource: "model"})
	if err != nil {
		t.Fatalf("ListAuditEvents(model) error = %v", err)
	}
	if len(auditEvents) < 4 {
		t.Fatalf("audit event count = %d, want >= 4", len(auditEvents))
	}
	for _, event := range auditEvents {
		combined := string(event.Before) + string(event.After)
		if strings.Contains(combined, "model-secret") || strings.Contains(combined, "ratio") {
			t.Fatalf("model audit leaked secret or cut-scope field: %#v", event)
		}
	}
}

func TestCustomerAccountManagementWorkflows(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	repo := adminrepo.NewMemoryRepository()
	owner := configadmin.NewService(configadmin.NewMemoryRepository(), configadmin.NewCredentialCodec("test-secret"), nil)
	resetter := &fakePortalSessionResetter{count: 2}
	svc := New(
		repo,
		owner,
		WithClock(func() time.Time { return now }),
		WithCommercialReporting(reporting.NewService(reporting.NewMemoryRepository())),
		WithPortalSessionResetter(resetter),
	)
	if _, err := svc.EnsureBootstrapOperator(ctx, "admin@example.com", "admin-local", []adminapp.Role{adminapp.RoleSuperAdmin}); err != nil {
		t.Fatalf("EnsureBootstrapOperator() error = %v", err)
	}
	login, err := svc.Login(ctx, adminapp.LoginRequest{Email: "admin@example.com", Password: "admin-local"}, "ua-customer", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	_, actor, err := svc.Session(ctx, login.Session.SessionID)
	if err != nil {
		t.Fatalf("Session() error = %v", err)
	}

	created, err := svc.CreateCustomerAccount(ctx, actor, adminapp.CustomerAccountCreateRequest{
		TenantName:          "Acme",
		ProjectName:         "Production",
		APIKeyName:          "primary",
		AllowedModels:       []string{"gpt-public"},
		Currency:            "CNY",
		InitialCreditMicros: 5_000_000,
		InitialCreditReason: "seed credits",
	}, mutationOptions("customer_create"))
	if err != nil {
		t.Fatalf("CreateCustomerAccount() error = %v", err)
	}
	accountID := created.Account.CustomerAccountID
	if created.Account.Status != "active" || created.Account.ActiveAPIKeyCount != 1 || len(created.Account.Credits) != 1 {
		t.Fatalf("created account = %#v", created)
	}
	if created.Account.Credits[0].AvailableMicros != 5_000_000 || created.Account.AllowedModels.Models[0] != "gpt-public" {
		t.Fatalf("created account credits/models = %#v", created.Account)
	}
	encoded := mustJSON(t, created)
	if strings.Contains(encoded, "plaintext_key") || strings.Contains(encoded, "key_hash") || strings.Contains(encoded, "user_group") {
		t.Fatalf("customer account leaked secret or cut-scope field: %s", encoded)
	}

	list, err := svc.ListCustomerAccounts(ctx, actor, adminapp.CustomerAccountFilter{Status: "active", Keyword: "acme"})
	if err != nil {
		t.Fatalf("ListCustomerAccounts() error = %v", err)
	}
	if len(list.Data) != 1 || list.Data[0].CustomerAccountID != accountID {
		t.Fatalf("customer list = %#v", list)
	}

	adjusted, err := svc.AdjustCustomerCredits(ctx, actor, accountID, adminapp.CustomerCreditAdjustmentRequest{
		Currency:     "CNY",
		AmountMicros: 2_000_000,
		Reason:       "support credit",
	}, mutationOptions("customer_adjust"))
	if err != nil {
		t.Fatalf("AdjustCustomerCredits() error = %v", err)
	}
	if adjusted.Account.Account.Credits[0].AvailableMicros != 7_000_000 || len(adjusted.Account.Ledger) < 2 {
		t.Fatalf("adjusted account = %#v", adjusted.Account)
	}

	reset, err := svc.ResetCustomerPortalSessions(ctx, actor, accountID, "", mutationOptions("customer_reset"))
	if err != nil {
		t.Fatalf("ResetCustomerPortalSessions() error = %v", err)
	}
	if reset.RevokedSessions != 2 || resetter.filter.ProjectID == "" {
		t.Fatalf("reset result = %#v filter=%#v", reset, resetter.filter)
	}

	disabled, err := svc.SetCustomerAccountEnabled(ctx, actor, accountID, false, mutationOptions("customer_disable"))
	if err != nil {
		t.Fatalf("SetCustomerAccountEnabled() error = %v", err)
	}
	if disabled.Account.Status != "disabled" || disabled.Account.ProjectEnabled {
		t.Fatalf("disabled account = %#v", disabled.Account)
	}

	auditEvents, err := repo.ListAuditEvents(ctx, adminapp.AuditFilter{Resource: "customer_account"})
	if err != nil {
		t.Fatalf("ListAuditEvents(customer_account) error = %v", err)
	}
	if len(auditEvents) < 4 {
		t.Fatalf("audit event count = %d, want >= 4", len(auditEvents))
	}
	for _, event := range auditEvents {
		combined := string(event.Before) + string(event.After)
		if strings.Contains(combined, "plaintext_key") || strings.Contains(combined, "key_hash") || strings.Contains(combined, "user_group") {
			t.Fatalf("customer audit leaked secret or cut-scope field: %#v", event)
		}
	}
}

func testService(t *testing.T) (*Service, *adminrepo.MemoryRepository) {
	t.Helper()
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	repo := adminrepo.NewMemoryRepository()
	ownerRepo := configadmin.NewMemoryRepository()
	owner := configadmin.NewService(ownerRepo, configadmin.NewCredentialCodec("test-secret"), nil)
	svc := New(repo, owner, WithClock(func() time.Time { return now }))
	if _, err := svc.EnsureBootstrapOperator(context.Background(), "admin@example.com", "admin-local", []adminapp.Role{adminapp.RoleSuperAdmin}); err != nil {
		t.Fatalf("EnsureBootstrapOperator() error = %v", err)
	}
	return svc, repo
}

type fakePortalSessionResetter struct {
	count  int
	filter PortalSessionResetFilter
}

func (f *fakePortalSessionResetter) ResetPortalSessions(_ context.Context, filter PortalSessionResetFilter) (int, error) {
	f.filter = filter
	return f.count, nil
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal value: %v", err)
	}
	return string(content)
}

func mutationOptions(seed string) adminapp.MutationOptions {
	return adminapp.MutationOptions{
		RequestID:      "req_" + seed,
		IdempotencyKey: "idem_" + seed,
		Reason:         "test " + seed,
	}
}

func assertNoChannelSecret(t *testing.T, value any, secrets ...string) {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal channel value: %v", err)
	}
	combined := string(content)
	for _, secret := range secrets {
		if strings.Contains(combined, secret) {
			t.Fatalf("safe channel response leaked secret %q: %s", secret, combined)
		}
	}
	if strings.Contains(combined, "encrypted_api_key") || strings.Contains(combined, "api_key") {
		t.Fatalf("safe channel response exposed credential field: %s", combined)
	}
}
