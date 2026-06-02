package adminhttp

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	adminrepo "github.com/KnifeFly/token-gateway/internal/app/admin/repository"
	adminservice "github.com/KnifeFly/token-gateway/internal/app/admin/service"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
)

func TestAdminAuthSessionAndCSRF(t *testing.T) {
	mux, _ := testMux(t)
	cookie, csrf := loginOperator(t, mux, "admin@example.com", "admin-local")

	me := request(t, mux, http.MethodGet, "/api/admin/v1/auth/me", "", cookie, "")
	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d body=%s", me.Code, me.Body.String())
	}
	var meBody struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("decode me body: %v", err)
	}
	csrf = meBody.CSRFToken

	missingCSRF := request(t, mux, http.MethodPost, "/api/admin/v1/tenants", `{"name":"Denied"}`, cookie, "")
	if missingCSRF.Code != http.StatusUnauthorized || !strings.Contains(missingCSRF.Body.String(), "csrf token is required") {
		t.Fatalf("missing csrf status=%d body=%s", missingCSRF.Code, missingCSRF.Body.String())
	}

	create := httptest.NewRequest(http.MethodPost, "/api/admin/v1/tenants", strings.NewReader(`{"name":"Acme"}`))
	create.Header.Set("Content-Type", "application/json")
	create.Header.Set(csrfHeaderName, csrf)
	create.Header.Set("Idempotency-Key", "idem_tenant")
	create.Header.Set("X-Reason", "create tenant")
	create.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, create)
	if rec.Code != http.StatusOK {
		t.Fatalf("create tenant status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminRBACDenyAndAuditQuery(t *testing.T) {
	mux, svc := testMux(t)
	if _, err := svc.EnsureBootstrapOperator(context.Background(), "viewer@example.com", "viewer-local", []adminapp.Role{adminapp.RoleReadOnly}); err != nil {
		t.Fatalf("EnsureBootstrapOperator(viewer) error = %v", err)
	}
	cookie, csrf := loginOperator(t, mux, "viewer@example.com", "viewer-local")

	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/tenants", strings.NewReader(`{"name":"Denied"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeaderName, csrf)
	req.Header.Set("Idempotency-Key", "idem_denied")
	req.Header.Set("X-Reason", "permission test")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("deny status=%d body=%s", rec.Code, rec.Body.String())
	}

	adminCookie, _ := loginOperator(t, mux, "admin@example.com", "admin-local")
	audit := request(t, mux, http.MethodGet, "/api/admin/v1/audit?resource=tenant", "", adminCookie, "")
	if audit.Code != http.StatusOK || !strings.Contains(audit.Body.String(), `"error_code":"forbidden"`) {
		t.Fatalf("audit status=%d body=%s", audit.Code, audit.Body.String())
	}
}

func TestAdminChannelWorkflows(t *testing.T) {
	mux, _ := testMux(t)
	cookie, csrf := loginOperator(t, mux, "admin@example.com", "admin-local")

	create := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels", `{
		"id":"channel_primary",
		"provider_type":"openai",
		"base_url":"https://provider.example/v1",
		"api_key":"sk_should_not_return",
		"enabled":true,
		"models":[{"public_model":"gpt-public","upstream_model":"gpt-upstream","health_status":"healthy","test_status":"passed","cost_config_status":"configured"}]
	}`, cookie, csrf, "channel_create")
	if create.Code != http.StatusOK {
		t.Fatalf("create channel status=%d body=%s", create.Code, create.Body.String())
	}
	if strings.Contains(create.Body.String(), "sk_should_not_return") || strings.Contains(create.Body.String(), "encrypted_api_key") {
		t.Fatalf("create channel leaked credential material: %s", create.Body.String())
	}

	detail := request(t, mux, http.MethodGet, "/api/admin/v1/channels/channel_primary", "", cookie, "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"model_count":1`) {
		t.Fatalf("channel detail status=%d body=%s", detail.Code, detail.Body.String())
	}

	testResult := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels/channel_primary/test", "", cookie, csrf, "channel_test")
	if testResult.Code != http.StatusOK || !strings.Contains(testResult.Body.String(), `"status":"ready"`) {
		t.Fatalf("channel test status=%d body=%s", testResult.Code, testResult.Body.String())
	}

	rotate := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels/channel_primary/rotate-credential", `{"api_key":"sk_rotated_should_not_return"}`, cookie, csrf, "channel_rotate")
	if rotate.Code != http.StatusOK {
		t.Fatalf("rotate status=%d body=%s", rotate.Code, rotate.Body.String())
	}
	if strings.Contains(rotate.Body.String(), "sk_rotated_should_not_return") || strings.Contains(rotate.Body.String(), "encrypted_api_key") {
		t.Fatalf("rotate leaked credential material: %s", rotate.Body.String())
	}

	preview := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels/channel_primary/sync-preview", `{
		"upstream_models":[{"public_model":"gpt-public","upstream_model":"gpt-upstream-v2","cost_config_status":"configured"}]
	}`, cookie, csrf, "channel_sync_preview")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), `"changed"`) {
		t.Fatalf("sync preview status=%d body=%s", preview.Code, preview.Body.String())
	}

	apply := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels/channel_primary/sync-apply", `{
		"upstream_models":[{"public_model":"gpt-public","upstream_model":"gpt-upstream-v2","cost_config_status":"configured"}]
	}`, cookie, csrf, "channel_sync_apply")
	if apply.Code != http.StatusOK || !strings.Contains(apply.Body.String(), `"gpt-upstream-v2"`) {
		t.Fatalf("sync apply status=%d body=%s", apply.Code, apply.Body.String())
	}

	health := request(t, mux, http.MethodGet, "/api/admin/v1/channels/channel_primary/health-events", "", cookie, "")
	if health.Code != http.StatusOK || !strings.Contains(health.Body.String(), `"channel_id":"channel_primary"`) {
		t.Fatalf("health status=%d body=%s", health.Code, health.Body.String())
	}
}

func TestAdminModelWorkflows(t *testing.T) {
	mux, _ := testMux(t)
	cookie, csrf := loginOperator(t, mux, "admin@example.com", "admin-local")

	create := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/models", `{
		"public_model":"gpt-public",
		"display_name":"GPT Public",
		"protocol":"native_openai",
		"capability":"chat",
		"category":"chat",
		"modalities":["text"],
		"capabilities":["chat","tool"],
		"metadata":{"credential":"model-secret"},
		"schema":{"type":"object","required":["model"]},
		"enabled":true
	}`, cookie, csrf, "model_create")
	if create.Code != http.StatusOK {
		t.Fatalf("create model status=%d body=%s", create.Code, create.Body.String())
	}
	if strings.Contains(create.Body.String(), "model-secret") || strings.Contains(create.Body.String(), "ratio") {
		t.Fatalf("create model exposed secret or cut-scope field: %s", create.Body.String())
	}

	price := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/pricing", `{
		"public_model":"gpt-public",
		"category":"chat",
		"currency":"CNY",
		"components":[{"unit":"input_token","micros_per_unit":2}],
		"enabled":true
	}`, cookie, csrf, "model_price")
	if price.Code != http.StatusOK {
		t.Fatalf("price status=%d body=%s", price.Code, price.Body.String())
	}

	channel := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels", `{
		"id":"channel_primary",
		"provider_type":"openai",
		"base_url":"https://provider.example/v1",
		"api_key":"sk_should_not_return",
		"enabled":true,
		"models":[{"public_model":"gpt-public","upstream_model":"gpt-upstream","health_status":"healthy","test_status":"passed","cost_config_status":"configured"}]
	}`, cookie, csrf, "model_channel")
	if channel.Code != http.StatusOK {
		t.Fatalf("channel status=%d body=%s", channel.Code, channel.Body.String())
	}

	detail := request(t, mux, http.MethodGet, "/api/admin/v1/models/gpt-public", "", cookie, "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"pricing_summary"`) || !strings.Contains(detail.Body.String(), `"channel_coverage"`) {
		t.Fatalf("model detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	if strings.Contains(detail.Body.String(), "sk_should_not_return") || strings.Contains(detail.Body.String(), "ratio") {
		t.Fatalf("model detail exposed secret or cut-scope field: %s", detail.Body.String())
	}

	channels := request(t, mux, http.MethodGet, "/api/admin/v1/models/gpt-public/channels", "", cookie, "")
	if channels.Code != http.StatusOK || !strings.Contains(channels.Body.String(), `"upstream_model":"gpt-upstream"`) {
		t.Fatalf("model channels status=%d body=%s", channels.Code, channels.Body.String())
	}

	schema := request(t, mux, http.MethodGet, "/api/admin/v1/models/gpt-public/schema-preview", "", cookie, "")
	if schema.Code != http.StatusOK || !strings.Contains(schema.Body.String(), `"model":"gpt-public"`) {
		t.Fatalf("schema status=%d body=%s", schema.Code, schema.Body.String())
	}

	preview := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/models/sync-preview", `{
		"models":[{"public_model":"gpt-public","display_name":"GPT Public Updated","protocol":"native_openai","capability":"chat"},{"public_model":"image-public","protocol":"unified","capability":"image"}]
	}`, cookie, csrf, "model_sync_preview")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), `"added"`) || !strings.Contains(preview.Body.String(), `"changed"`) {
		t.Fatalf("model sync preview status=%d body=%s", preview.Code, preview.Body.String())
	}

	deprecate := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/models/gpt-public/deprecate", "", cookie, csrf, "model_deprecate")
	if deprecate.Code != http.StatusOK || !strings.Contains(deprecate.Body.String(), `"deprecated":true`) {
		t.Fatalf("deprecate status=%d body=%s", deprecate.Code, deprecate.Body.String())
	}

	disable := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/models/gpt-public/disable", "", cookie, csrf, "model_disable")
	if disable.Code != http.StatusOK || !strings.Contains(disable.Body.String(), `"enabled":false`) {
		t.Fatalf("disable status=%d body=%s", disable.Code, disable.Body.String())
	}
}

func testMux(t *testing.T) (*http.ServeMux, *adminservice.Service) {
	t.Helper()
	repo := adminrepo.NewMemoryRepository()
	owner := configadmin.NewService(configadmin.NewMemoryRepository(), configadmin.NewCredentialCodec("test-secret"), nil)
	svc := adminservice.New(repo, owner)
	if _, err := svc.EnsureBootstrapOperator(context.Background(), "admin@example.com", "admin-local", []adminapp.Role{adminapp.RoleSuperAdmin}); err != nil {
		t.Fatalf("EnsureBootstrapOperator() error = %v", err)
	}
	mux := http.NewServeMux()
	NewHandlerWithService(svc, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	return mux, svc
}

func loginOperator(t *testing.T, mux http.Handler, email string, password string) (*http.Cookie, string) {
	t.Helper()
	rec := request(t, mux, http.MethodPost, "/api/admin/v1/auth/login", `{"email":"`+email+`","password":"`+password+`"}`, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode login body: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != sessionCookieName {
		t.Fatalf("missing session cookie: %#v", cookies)
	}
	return cookies[0], body.CSRFToken
}

func request(t *testing.T, mux http.Handler, method string, path string, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if csrf != "" {
		req.Header.Set(csrfHeaderName, csrf)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func mutationRequest(t *testing.T, mux http.Handler, method string, path string, body string, cookie *http.Cookie, csrf string, seed string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(csrfHeaderName, csrf)
	req.Header.Set("Idempotency-Key", "idem_"+seed)
	req.Header.Set("X-Reason", "test "+seed)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}
