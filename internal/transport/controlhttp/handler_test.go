package controlhttp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
