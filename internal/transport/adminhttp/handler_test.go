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
	cpadmin "github.com/KnifeFly/token-gateway/internal/controlplane/admin"
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

func testMux(t *testing.T) (*http.ServeMux, *adminservice.Service) {
	t.Helper()
	repo := adminrepo.NewMemoryRepository()
	owner := cpadmin.NewService(cpadmin.NewMemoryRepository(), cpadmin.NewCredentialCodec("test-secret"), nil)
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
