package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	adminrepo "github.com/KnifeFly/token-gateway/internal/app/admin/repository"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
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
	_, err = svc.CreateAPIKey(ctx, actor, configadmin.APIKey{
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		Name:         "secret key",
		PlaintextKey: "tg_should_not_appear",
	}, adminapp.MutationOptions{
		RequestID:      "req_key",
		IdempotencyKey: "idem_key",
		Reason:         "create derived key",
		UserAgent:      "ua-secret",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	events, err := repo.ListAuditEvents(ctx, adminapp.AuditFilter{Resource: "api_key"})
	if err != nil {
		t.Fatalf("ListAuditEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	combined := string(events[0].Before) + string(events[0].After)
	if strings.Contains(combined, "tg_should_not_appear") || strings.Contains(combined, "sha256:") {
		t.Fatalf("audit event leaked key material: before=%s after=%s", events[0].Before, events[0].After)
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
