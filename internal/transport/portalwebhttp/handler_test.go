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
	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	dpsnapshot "github.com/KnifeFly/token-gateway/internal/dataplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/portal"
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

	createReq = httptest.NewRequest(http.MethodPost, "/api/portal/v1/api-keys", strings.NewReader(`{"name":"derived","allowed_models":["gpt-public"]}`))
	createReq.Header.Set(csrfHeaderName, login.CSRFToken)
	createReq.AddCookie(cookie)
	createRec = httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK || !strings.Contains(createRec.Body.String(), `"plaintext_key"`) || strings.Contains(createRec.Body.String(), "key_hash") {
		t.Fatalf("create key status = %d, body = %s", createRec.Code, createRec.Body.String())
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
	if rec.Code != http.StatusNotFound {
		t.Fatalf("model route without schema suffix status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

type portalWebTestIDs struct {
	ownTaskID   string
	otherTaskID string
}

func testPortalWebHandler(t *testing.T) (http.Handler, portalWebTestIDs) {
	t.Helper()

	ctx := context.Background()
	adminRepo := admin.NewMemoryRepository()
	adminService := admin.NewService(adminRepo, nil, nil)
	if _, err := adminService.CreateAPIKey(ctx, admin.APIKey{
		ID:            "key_current",
		TenantID:      "tenant_1",
		ProjectID:     "project_1",
		Name:          "primary",
		PlaintextKey:  "test-key",
		AllowedModels: []string{"gpt-public"},
	}); err != nil {
		t.Fatalf("CreateAPIKey(current) error = %v", err)
	}
	if _, err := adminService.CreateAPIKey(ctx, admin.APIKey{
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
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	webService := portalservice.New(
		dpsnapshot.NewProvider(dpsnapshot.NewStore(indexed)),
		auth.NewSnapshotAuthenticator(),
		portal.NewService(adminService, reportService, taskRepo),
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
