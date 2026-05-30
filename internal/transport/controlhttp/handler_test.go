package controlhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
)

func TestHandlerRequiresAdminToken(t *testing.T) {
	repo := admin.NewMemoryRepository()
	service := admin.NewService(repo, admin.NewCredentialCodec("secret"), nil)
	handler := NewHandler(service, cpsnapshot.NewPublisher(repo, nil), "secret-token", nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/tenants", strings.NewReader(`{"name":"tenant"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerRejectsEmptyAdminToken(t *testing.T) {
	repo := admin.NewMemoryRepository()
	service := admin.NewService(repo, admin.NewCredentialCodec("secret"), nil)
	handler := NewHandler(service, cpsnapshot.NewPublisher(repo, nil), "", nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/tenants", strings.NewReader(`{"name":"tenant"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerCreatesModelWithAdminToken(t *testing.T) {
	repo := admin.NewMemoryRepository()
	service := admin.NewService(repo, admin.NewCredentialCodec("secret"), nil)
	handler := NewHandler(service, cpsnapshot.NewPublisher(repo, nil), "secret-token", nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/models", bytes.NewBufferString(`{
		"public_model":"gpt-4o-mini",
		"protocol":"native_openai",
		"capability":"chat",
		"enabled":true
	}`))
	req.Header.Set("X-Admin-Token", "secret-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	cfg, err := repo.LoadSnapshotConfig(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshotConfig() error = %v", err)
	}
	if len(cfg.Models) != 1 || cfg.Models[0].PublicModel != "gpt-4o-mini" {
		t.Fatalf("models = %#v", cfg.Models)
	}
}

func TestHandlerCreatesPluginBindingWithAdminToken(t *testing.T) {
	repo := admin.NewMemoryRepository()
	service := admin.NewService(repo, admin.NewCredentialCodec("secret"), nil)
	handler := NewHandler(service, cpsnapshot.NewPublisher(repo, nil), "secret-token", nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/plugin-bindings", bytes.NewBufferString(`{
		"name":"prompt_guard",
		"phase":"pre_prompt",
		"priority":10,
		"config":{"deny_terms":["blocked"]}
	}`))
	req.Header.Set("X-Admin-Token", "secret-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	cfg, err := repo.LoadSnapshotConfig(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshotConfig() error = %v", err)
	}
	if len(cfg.Plugins) != 1 || cfg.Plugins[0].Name != "prompt_guard" {
		t.Fatalf("plugins = %#v", cfg.Plugins)
	}
}

func TestHandlerCreatesManualAdjustmentReport(t *testing.T) {
	repo := admin.NewMemoryRepository()
	service := admin.NewService(repo, admin.NewCredentialCodec("secret"), nil)
	reportingService := reporting.NewService(reporting.NewMemoryRepository())
	handler := NewHandler(service, cpsnapshot.NewPublisher(repo, nil), "secret-token", nil, reportingService)

	req := httptest.NewRequest(http.MethodPost, "/admin/billing/adjustments", bytes.NewBufferString(`{
		"idempotency_key":"adj-1",
		"tenant_id":"tenant_1",
		"project_id":"project_1",
		"currency":"USD",
		"amount_micros":1000,
		"reason":"top up",
		"operator_id":"ops"
	}`))
	req.Header.Set("X-Admin-Token", "secret-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/reports/tenant-usage?tenant_id=tenant_1&project_id=project_1", nil)
	req.Header.Set("X-Admin-Token", "secret-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var report reporting.TenantUsageReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("invalid report: %v", err)
	}
	if len(report.Balances) != 1 || report.Balances[0].AvailableMicros != 1000 {
		t.Fatalf("report = %#v", report)
	}
}

func TestHandlerListsVisibleModels(t *testing.T) {
	repo := admin.NewMemoryRepository()
	service := admin.NewService(repo, admin.NewCredentialCodec("secret"), nil)
	handler := NewHandler(service, cpsnapshot.NewPublisher(repo, nil), "secret-token", nil)

	for _, body := range []string{
		`{"public_model":"gpt-4o-mini","protocol":"native_openai","capability":"chat","enabled":true}`,
		`{"public_model":"gpt-4o-mini","currency":"USD","input_micros_per_token":1,"output_micros_per_token":2,"enabled":true}`,
		`{"tenant_id":"tenant_1","project_id":"project_1","public_model":"gpt-4o-mini","display_name":"GPT 4o Mini","enabled":true}`,
	} {
		path := "/admin/models"
		if strings.Contains(body, "currency") {
			path = "/admin/prices"
		}
		if strings.Contains(body, "display_name") {
			path = "/admin/model-marketplace"
		}
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("X-Admin-Token", "secret-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("path %s status = %d body = %s", path, rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/model-marketplace?tenant_id=tenant_1&project_id=project_1", nil)
	req.Header.Set("X-Admin-Token", "secret-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var models []admin.VisibleModel
	if err := json.Unmarshal(rec.Body.Bytes(), &models); err != nil {
		t.Fatalf("invalid models: %v", err)
	}
	if len(models) != 1 || models[0].PublicModel != "gpt-4o-mini" || models[0].Currency != "USD" {
		t.Fatalf("models = %#v", models)
	}
}
