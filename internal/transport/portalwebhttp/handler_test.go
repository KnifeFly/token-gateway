package portalwebhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	portalrepo "github.com/KnifeFly/token-gateway/internal/app/portal/repository"
	portalservice "github.com/KnifeFly/token-gateway/internal/app/portal/service"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	dpsnapshot "github.com/KnifeFly/token-gateway/internal/dataplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

func TestPortalWebLoginDashboardAPIKeyAndLogout(t *testing.T) {
	handler, ids := testPortalWebHandler(t)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/portal/v1/auth/api-key-login", strings.NewReader(`{"api_key":"test-key"}`))
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRec.Code, loginRec.Body.String())
	}
	if strings.Contains(loginRec.Body.String(), "test-key") {
		t.Fatalf("login body leaked api key: %s", loginRec.Body.String())
	}
	cookie := portalSessionCookie(t, loginRec.Result())
	if !cookie.HttpOnly {
		t.Fatalf("portal session cookie is not HttpOnly: %#v", cookie)
	}
	var login struct {
		CSRFToken string `json:"csrf_token"`
		Session   struct {
			Authenticated bool   `json:"authenticated"`
			TenantID      string `json:"tenant_id"`
			ProjectID     string `json:"project_id"`
		} `json:"session"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &login); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if login.CSRFToken == "" || !login.Session.Authenticated || login.Session.TenantID != "tenant_1" || login.Session.ProjectID != "project_1" {
		t.Fatalf("unexpected login response: %s", loginRec.Body.String())
	}

	dashboardReq := httptest.NewRequest(http.MethodGet, "/api/portal/v1/dashboard", nil)
	dashboardReq.AddCookie(cookie)
	dashboardRec := httptest.NewRecorder()
	handler.ServeHTTP(dashboardRec, dashboardReq)
	if dashboardRec.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, body = %s", dashboardRec.Code, dashboardRec.Body.String())
	}
	body := dashboardRec.Body.String()
	if !strings.Contains(body, ids.ownTaskID) || strings.Contains(body, ids.otherTaskID) || strings.Contains(body, "secret-token") {
		t.Fatalf("dashboard body was not safely scoped: %s", body)
	}

	usageReq := httptest.NewRequest(http.MethodGet, "/api/portal/v1/usage?request_id=req_usage_1&api_key_id=key_current&model=gpt-public&provider_type=openai&channel_id=channel_primary", nil)
	usageReq.AddCookie(cookie)
	usageRec := httptest.NewRecorder()
	handler.ServeHTTP(usageRec, usageReq)
	if usageRec.Code != http.StatusOK || !strings.Contains(usageRec.Body.String(), `"request_id":"req_usage_1"`) || !strings.Contains(usageRec.Body.String(), `"channel_id":"channel_primary"`) {
		t.Fatalf("usage filter status = %d, body = %s", usageRec.Code, usageRec.Body.String())
	}
	if strings.Contains(usageRec.Body.String(), "req_usage_other") || strings.Contains(usageRec.Body.String(), "secret") {
		t.Fatalf("usage filter leaked cross-scope or unsafe data: %s", usageRec.Body.String())
	}

	emptyUsageReq := httptest.NewRequest(http.MethodGet, "/api/portal/v1/usage?model=no-match", nil)
	emptyUsageReq.AddCookie(cookie)
	emptyUsageRec := httptest.NewRecorder()
	handler.ServeHTTP(emptyUsageRec, emptyUsageReq)
	if emptyUsageRec.Code != http.StatusOK || !strings.Contains(emptyUsageRec.Body.String(), `"items":[]`) {
		t.Fatalf("empty usage filter status = %d, body = %s", emptyUsageRec.Code, emptyUsageRec.Body.String())
	}
	if strings.Contains(emptyUsageRec.Body.String(), "usage_debit") || strings.Contains(emptyUsageRec.Body.String(), "req_usage_1") {
		t.Fatalf("empty usage filter fell back to ledger rows: %s", emptyUsageRec.Body.String())
	}

	taskReq := httptest.NewRequest(http.MethodGet, "/api/portal/v1/tasks?request_id=req_1&api_key_id=key_current&model=gpt-public", nil)
	taskReq.AddCookie(cookie)
	taskRec := httptest.NewRecorder()
	handler.ServeHTTP(taskRec, taskReq)
	if taskRec.Code != http.StatusOK || !strings.Contains(taskRec.Body.String(), ids.ownTaskID) || strings.Contains(taskRec.Body.String(), ids.otherTaskID) {
		t.Fatalf("task filter status = %d, body = %s", taskRec.Code, taskRec.Body.String())
	}
	if strings.Contains(taskRec.Body.String(), "secret-token") || strings.Contains(taskRec.Body.String(), "callback_url") {
		t.Fatalf("task filter leaked unsafe data: %s", taskRec.Body.String())
	}

	playgroundReq := httptest.NewRequest(http.MethodPost, "/api/portal/v1/playground/run", strings.NewReader(`{
		"model":"gpt-public",
		"mode":"chat",
		"payload":{"model":"gpt-public","input":"do not return prompt","api_key":"sk_leak"}
	}`))
	playgroundReq.Header.Set(csrfHeaderName, login.CSRFToken)
	playgroundReq.AddCookie(cookie)
	playgroundRec := httptest.NewRecorder()
	handler.ServeHTTP(playgroundRec, playgroundReq)
	if playgroundRec.Code != http.StatusOK || !strings.Contains(playgroundRec.Body.String(), `"scope":"portal"`) || !strings.Contains(playgroundRec.Body.String(), `"channel_id":"channel_primary"`) {
		t.Fatalf("playground status = %d, body = %s", playgroundRec.Code, playgroundRec.Body.String())
	}
	if strings.Contains(playgroundRec.Body.String(), "do not return prompt") || strings.Contains(playgroundRec.Body.String(), "sk_leak") || strings.Contains(playgroundRec.Body.String(), "key_hash") {
		t.Fatalf("playground leaked unsafe data: %s", playgroundRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/portal/v1/api-keys", strings.NewReader(`{"name":"derived","allowed_models":["gpt-public"]}`))
	createReq.AddCookie(cookie)
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusUnauthorized {
		t.Fatalf("create without csrf status = %d, body = %s", createRec.Code, createRec.Body.String())
	}

	expandedReq := httptest.NewRequest(http.MethodPost, "/api/portal/v1/api-keys", strings.NewReader(`{"name":"bad","allowed_models":["image-public"]}`))
	expandedReq.Header.Set(csrfHeaderName, login.CSRFToken)
	expandedReq.AddCookie(cookie)
	expandedRec := httptest.NewRecorder()
	handler.ServeHTTP(expandedRec, expandedReq)
	if expandedRec.Code != http.StatusForbidden {
		t.Fatalf("expanded key status = %d, body = %s", expandedRec.Code, expandedRec.Body.String())
	}

	createReq = httptest.NewRequest(http.MethodPost, "/api/portal/v1/api-keys", strings.NewReader(`{"name":"derived","allowed_models":["gpt-public"],"ip_allowlist":["203.0.113.10"],"expires_at":"2026-06-30T00:00:00Z"}`))
	createReq.Header.Set(csrfHeaderName, login.CSRFToken)
	createReq.AddCookie(cookie)
	createRec = httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK || !strings.Contains(createRec.Body.String(), `"plaintext_key"`) || strings.Contains(createRec.Body.String(), "key_hash") {
		t.Fatalf("create key status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		APIKey struct {
			ID          string     `json:"id"`
			IPAllowlist []string   `json:"ip_allowlist"`
			ExpiresAt   *time.Time `json:"expires_at"`
		} `json:"api_key"`
		PlaintextKey string `json:"plaintext_key"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created key: %v", err)
	}
	if created.APIKey.ID == "" || created.PlaintextKey == "" || len(created.APIKey.IPAllowlist) != 1 || created.APIKey.ExpiresAt == nil {
		t.Fatalf("created key missing T06 metadata: %s", createRec.Body.String())
	}

	rotateReq := httptest.NewRequest(http.MethodPost, "/api/portal/v1/api-keys/"+created.APIKey.ID+"/rotate", nil)
	rotateReq.Header.Set(csrfHeaderName, login.CSRFToken)
	rotateReq.AddCookie(cookie)
	rotateRec := httptest.NewRecorder()
	handler.ServeHTTP(rotateRec, rotateReq)
	if rotateRec.Code != http.StatusOK || !strings.Contains(rotateRec.Body.String(), `"plaintext_key"`) || strings.Contains(rotateRec.Body.String(), "key_hash") {
		t.Fatalf("rotate key status = %d, body = %s", rotateRec.Code, rotateRec.Body.String())
	}
	var rotated struct {
		PlaintextKey string `json:"plaintext_key"`
	}
	if err := json.Unmarshal(rotateRec.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("decode rotated key: %v", err)
	}
	if rotated.PlaintextKey == "" || rotated.PlaintextKey == created.PlaintextKey {
		t.Fatalf("rotated plaintext = %q original=%q", rotated.PlaintextKey, created.PlaintextKey)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/portal/v1/auth/logout", nil)
	logoutReq.Header.Set(csrfHeaderName, login.CSRFToken)
	logoutReq.AddCookie(cookie)
	logoutRec := httptest.NewRecorder()
	handler.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout status = %d, body = %s", logoutRec.Code, logoutRec.Body.String())
	}

	afterLogoutReq := httptest.NewRequest(http.MethodGet, "/api/portal/v1/dashboard", nil)
	afterLogoutReq.AddCookie(cookie)
	afterLogoutRec := httptest.NewRecorder()
	handler.ServeHTTP(afterLogoutRec, afterLogoutReq)
	if afterLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("dashboard after logout status = %d, body = %s", afterLogoutRec.Code, afterLogoutRec.Body.String())
	}
}

func TestPortalWebRejectsMissingSessionAndBadUsageLimit(t *testing.T) {
	handler, _ := testPortalWebHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/portal/v1/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status = %d, body = %s", rec.Code, rec.Body.String())
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/portal/v1/auth/api-key-login", strings.NewReader(`{"api_key":"test-key"}`))
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	cookie := portalSessionCookie(t, loginRec.Result())

	req = httptest.NewRequest(http.MethodGet, "/api/portal/v1/usage?limit=bad", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad limit status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/portal/v1/models/gpt-public", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"pricing_summary"`) || strings.Contains(rec.Body.String(), "ratio") {
		t.Fatalf("model detail status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/portal/v1/models/image-public", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("disallowed model detail status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

type portalWebTestIDs struct {
	ownTaskID   string
	otherTaskID string
}

func testPortalWebHandler(t *testing.T) (http.Handler, portalWebTestIDs) {
	t.Helper()

	ctx := context.Background()
	adminRepo := configadmin.NewMemoryRepository()
	adminService := configadmin.NewService(adminRepo, nil, nil)
	if _, err := adminService.CreateAPIKey(ctx, configadmin.APIKey{
		ID:            "key_current",
		TenantID:      "tenant_1",
		ProjectID:     "project_1",
		Name:          "primary",
		PlaintextKey:  "test-key",
		AllowedModels: []string{"gpt-public"},
	}); err != nil {
		t.Fatalf("CreateAPIKey(current) error = %v", err)
	}
	if _, err := adminService.CreateAPIKey(ctx, configadmin.APIKey{
		ID:            "key_other_project",
		TenantID:      "tenant_1",
		ProjectID:     "project_2",
		Name:          "other",
		PlaintextKey:  "other-key",
		AllowedModels: []string{"gpt-public"},
	}); err != nil {
		t.Fatalf("CreateAPIKey(other) error = %v", err)
	}

	reportRepo := reporting.NewMemoryRepository()
	reportService := reporting.NewService(reportRepo)
	if _, err := reportService.CreateManualAdjustment(ctx, reporting.ManualAdjustmentRequest{
		IdempotencyKey: "seed",
		TenantID:       "tenant_1",
		ProjectID:      "project_1",
		Currency:       "CNY",
		AmountMicros:   5_000_000,
		Reason:         "seed",
		OperatorID:     "test",
	}); err != nil {
		t.Fatalf("CreateManualAdjustment() error = %v", err)
	}
	reportRepo.SeedUsageRecord(reporting.UsageLogRow{
		RequestID:    "req_usage_1",
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		APIKeyID:     "key_current",
		Model:        "gpt-public",
		ProviderType: "openai",
		ChannelID:    "channel_primary",
		InputTokens:  10,
		OutputTokens: 20,
		TotalTokens:  30,
		AmountMicros: 1200,
		Currency:     "CNY",
	})
	reportRepo.SeedUsageRecord(reporting.UsageLogRow{
		RequestID:    "req_usage_other",
		TenantID:     "tenant_2",
		ProjectID:    "project_2",
		APIKeyID:     "other",
		Model:        "gpt-public",
		ProviderType: "openai",
		ChannelID:    "channel_primary",
		AmountMicros: 9000,
		Currency:     "CNY",
	})

	taskRepo := tasksvc.NewMemoryRepository()
	taskService := tasksvc.NewService(taskRepo, 0)
	ownTask, _, err := taskService.CreateMediaTask(ctx, tasksvc.CreateTaskRequest{
		TenantID:  "tenant_1",
		ProjectID: "project_1",
		APIKeyID:  "key_current",
		RequestID: "req_1",
		Endpoint:  "/v1/images/generations",
		Kind:      tasksvc.KindImageGeneration,
		MediaType: "image",
		Model:     "gpt-public",
		Input:     []byte(`{"model":"gpt-public","prompt":"hi"}`),
		Metadata:  map[string]string{"workflow": "wf_1", "api_secret": "secret-token"},
	})
	if err != nil {
		t.Fatalf("CreateMediaTask(own) error = %v", err)
	}
	otherTask, _, err := taskService.CreateMediaTask(ctx, tasksvc.CreateTaskRequest{
		TenantID:  "tenant_2",
		ProjectID: "project_2",
		APIKeyID:  "other",
		RequestID: "req_2",
		Endpoint:  "/v1/images/generations",
		Kind:      tasksvc.KindImageGeneration,
		MediaType: "image",
		Model:     "gpt-public",
		Input:     []byte(`{"model":"gpt-public","prompt":"hi"}`),
	})
	if err != nil {
		t.Fatalf("CreateMediaTask(other) error = %v", err)
	}

	indexed, err := dpsnapshot.Build(cpsnapshot.RuntimeSnapshot{
		Version:   "v1",
		CreatedAt: time.Now().UTC(),
		APIKeys: []cpsnapshot.APIKeyRuntime{{
			ID:            "key_current",
			TenantID:      "tenant_1",
			ProjectID:     "project_1",
			KeyHash:       auth.HashAPIKey("test-key"),
			AllowedModels: []string{"gpt-public"},
			Enabled:       true,
		}},
		Models: []cpsnapshot.ModelRuntime{{
			PublicModel: "gpt-public",
			DisplayName: "GPT Public",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Capability:  "text",
			Schema:      json.RawMessage(`{"type":"object","required":["model","input"]}`),
			Enabled:     true,
		}, {
			PublicModel: "image-public",
			DisplayName: "Image Public",
			Protocol:    string(engine.ProtocolUnified),
			Capability:  "image",
			Schema:      json.RawMessage(`{"type":"object"}`),
			Enabled:     true,
		}},
		Channels: []cpsnapshot.ChannelRuntime{{
			ID:           "channel_primary",
			ProviderType: "openai",
			APIKey:       "sk_should_not_return",
			Enabled:      true,
			Models: []cpsnapshot.ChannelModelRuntime{{
				PublicModel:   "gpt-public",
				UpstreamModel: "gpt-upstream",
			}},
		}},
		RoutePolicies: []cpsnapshot.RoutePolicyRuntime{{
			ID:          "route_gpt_public",
			PublicModel: "gpt-public",
			Strategy:    "priority",
			Candidates:  []cpsnapshot.RouteCandidateRuntime{{ChannelID: "channel_primary", Priority: 1, Weight: 100}},
		}},
		PriceRules: []cpsnapshot.PriceRuleRuntime{{
			PublicModel: "gpt-public",
			Category:    "chat",
			Currency:    "CNY",
			Components: []pricing.Component{{
				Unit:          pricing.UnitInputToken,
				MicrosPerUnit: 2,
			}},
			Enabled: true,
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	webService := portalservice.New(
		dpsnapshot.NewProvider(dpsnapshot.NewStore(indexed)),
		auth.NewSnapshotAuthenticator(),
		adminService,
		reportService,
		taskRepo,
		portalrepo.NewMemorySessionStore(),
	)
	registrar := NewHandler(webService, nil)
	mux := http.NewServeMux()
	registrar.Register(mux)
	return mux, portalWebTestIDs{ownTaskID: ownTask.ID, otherTaskID: otherTask.ID}
}

func portalSessionCookie(t *testing.T, response *http.Response) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie
		}
	}
	t.Fatalf("missing %s cookie", sessionCookieName)
	return nil
}
