package publichttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	dpsnapshot "github.com/KnifeFly/token-gateway/internal/dataplane/snapshot"
)

func TestListModelsAndSchema(t *testing.T) {
	handler := testHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"moderation-latest"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/models/moderation-latest/schema", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("schema status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"version":"v1"`) {
		t.Fatalf("schema body = %s", rec.Body.String())
	}
}

func TestCreditsRequiresAuthAndReturnsPublicShape(t *testing.T) {
	handler := testHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/credits", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/credits", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("credits status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"success":true`) || strings.Contains(rec.Body.String(), "available_micros") {
		t.Fatalf("credits body = %s", rec.Body.String())
	}
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	indexed, err := dpsnapshot.Build(cpsnapshot.RuntimeSnapshot{
		Version:   "v1",
		CreatedAt: time.Now().UTC(),
		APIKeys: []cpsnapshot.APIKeyRuntime{{
			ID:            "key_1",
			TenantID:      "tenant_1",
			ProjectID:     "project_1",
			KeyHash:       auth.HashAPIKey("test-key"),
			AllowedModels: []string{"*"},
			Enabled:       true,
		}},
		Models: []cpsnapshot.ModelRuntime{{
			PublicModel: "moderation-latest",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Capability:  "moderation",
			Enabled:     true,
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	registrar := NewHandler(
		dpsnapshot.NewProvider(dpsnapshot.NewStore(indexed)),
		auth.NewSnapshotAuthenticator(),
		reporting.NewService(reporting.NewMemoryRepository()),
		nil,
	)
	mux := http.NewServeMux()
	registrar.Register(mux)
	return mux
}
