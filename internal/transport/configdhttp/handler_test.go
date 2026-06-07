package configdhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

func TestHandlerPublishesAndReportsDiagnostics(t *testing.T) {
	ctx := context.Background()
	repo := configadmin.NewMemoryRepository()
	service := configadmin.NewService(repo, configadmin.NewCredentialCodec("secret"), nil)
	seedSnapshotConfig(t, ctx, service)

	publisher := cpsnapshot.NewPublisher(repo, cpsnapshot.NewBuilder(repo))
	mux := http.NewServeMux()
	NewHandler(publisher, "secret-token", nil).Register(mux)

	unauthorized := httptest.NewRecorder()
	mux.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/configd/snapshots/publish", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	publish := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/configd/snapshots/publish", nil)
	req.Header.Set("X-Admin-Token", "secret-token")
	mux.ServeHTTP(publish, req)
	if publish.Code != http.StatusOK {
		t.Fatalf("publish status = %d body = %s", publish.Code, publish.Body.String())
	}

	diagnostics := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/configd/snapshots/diagnostics", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	mux.ServeHTTP(diagnostics, req)
	if diagnostics.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d body = %s", diagnostics.Code, diagnostics.Body.String())
	}
	var payload cpsnapshot.Diagnostics
	if err := json.Unmarshal(diagnostics.Body.Bytes(), &payload); err != nil {
		t.Fatalf("diagnostics json = %v", err)
	}
	if payload.Active == nil || !payload.Active.Valid || payload.Active.Version == "" {
		t.Fatalf("diagnostics active = %#v", payload.Active)
	}
}

func TestHandlerRejectsEmptyAdminToken(t *testing.T) {
	repo := configadmin.NewMemoryRepository()
	publisher := cpsnapshot.NewPublisher(repo, cpsnapshot.NewBuilder(repo))
	mux := http.NewServeMux()
	NewHandler(publisher, "", nil).Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/configd/snapshots/publish", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerRollsBackActiveSnapshot(t *testing.T) {
	ctx := context.Background()
	repo := configadmin.NewMemoryRepository()
	service := configadmin.NewService(repo, configadmin.NewCredentialCodec("secret"), nil)
	seedSnapshotConfig(t, ctx, service)

	publisher := cpsnapshot.NewPublisher(repo, cpsnapshot.NewBuilder(repo))
	mux := http.NewServeMux()
	NewHandler(publisher, "secret-token", nil).Register(mux)

	first := publishSnapshot(t, mux)
	if _, err := service.UpsertModel(ctx, configadmin.ModelConfig{
		PublicModel: "gpt-4.1-mini",
		Protocol:    string(engine.ProtocolNativeOpenAI),
		Capability:  "chat",
		Enabled:     true,
	}); err != nil {
		t.Fatalf("UpsertModel(second) error = %v", err)
	}
	if _, err := service.UpsertChannel(ctx, configadmin.ChannelConfig{
		ID:           "channel_1",
		ProviderType: "openai_compatible",
		BaseURL:      "mock://openai",
		Enabled:      true,
		Models: []configadmin.ChannelModel{
			{PublicModel: "gpt-4o-mini", UpstreamModel: "gpt-4o-mini"},
			{PublicModel: "gpt-4.1-mini", UpstreamModel: "gpt-4.1-mini"},
		},
	}); err != nil {
		t.Fatalf("UpsertChannel(second) error = %v", err)
	}
	if _, err := service.UpsertRoute(ctx, configadmin.RoutePolicyConfig{
		PublicModel: "gpt-4.1-mini",
		Candidates:  []configadmin.RouteCandidate{{ChannelID: "channel_1", Priority: 1, Weight: 100}},
	}); err != nil {
		t.Fatalf("UpsertRoute(second) error = %v", err)
	}
	second := publishSnapshot(t, mux)
	if second.Version == first.Version {
		t.Fatal("snapshot version did not advance")
	}

	rollback := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/configd/snapshots/rollback", nil)
	req.Header.Set("X-Admin-Token", "secret-token")
	mux.ServeHTTP(rollback, req)
	if rollback.Code != http.StatusOK {
		t.Fatalf("rollback status = %d body = %s", rollback.Code, rollback.Body.String())
	}
	var rolledBack cpsnapshot.RuntimeSnapshot
	if err := json.Unmarshal(rollback.Body.Bytes(), &rolledBack); err != nil {
		t.Fatalf("rollback json = %v", err)
	}
	if rolledBack.Version != first.Version {
		t.Fatalf("rollback version = %q, want %q", rolledBack.Version, first.Version)
	}

	diagnostics := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/configd/snapshots/diagnostics", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	mux.ServeHTTP(diagnostics, req)
	if diagnostics.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d body = %s", diagnostics.Code, diagnostics.Body.String())
	}
	var payload cpsnapshot.Diagnostics
	if err := json.Unmarshal(diagnostics.Body.Bytes(), &payload); err != nil {
		t.Fatalf("diagnostics json = %v", err)
	}
	if payload.Active == nil || payload.Active.Version != first.Version || !payload.Active.Valid {
		t.Fatalf("active diagnostics = %#v", payload.Active)
	}
	if payload.Previous == nil || payload.Previous.Version != second.Version || !payload.Previous.Valid {
		t.Fatalf("previous diagnostics = %#v", payload.Previous)
	}
}

func publishSnapshot(t *testing.T, mux *http.ServeMux) cpsnapshot.RuntimeSnapshot {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/configd/snapshots/publish", nil)
	req.Header.Set("X-Admin-Token", "secret-token")
	mux.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("publish status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var runtime cpsnapshot.RuntimeSnapshot
	if err := json.Unmarshal(recorder.Body.Bytes(), &runtime); err != nil {
		t.Fatalf("publish json = %v", err)
	}
	if runtime.Version == "" || runtime.Checksum == "" {
		t.Fatalf("published runtime = %#v", runtime)
	}
	return runtime
}

func seedSnapshotConfig(t *testing.T, ctx context.Context, service *configadmin.Service) {
	t.Helper()
	if _, err := service.CreateAPIKey(ctx, configadmin.APIKey{TenantID: "tenant", ProjectID: "project", PlaintextKey: "tg-test"}); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if _, err := service.UpsertModel(ctx, configadmin.ModelConfig{PublicModel: "gpt-4o-mini", Protocol: string(engine.ProtocolNativeOpenAI), Capability: "chat", Enabled: true}); err != nil {
		t.Fatalf("UpsertModel() error = %v", err)
	}
	if _, err := service.UpsertChannel(ctx, configadmin.ChannelConfig{
		ID:           "channel_1",
		ProviderType: "openai_compatible",
		BaseURL:      "mock://openai",
		Enabled:      true,
		Models:       []configadmin.ChannelModel{{PublicModel: "gpt-4o-mini", UpstreamModel: "gpt-4o-mini"}},
	}); err != nil {
		t.Fatalf("UpsertChannel() error = %v", err)
	}
	if _, err := service.UpsertRoute(ctx, configadmin.RoutePolicyConfig{PublicModel: "gpt-4o-mini", Candidates: []configadmin.RouteCandidate{{ChannelID: "channel_1", Priority: 1, Weight: 100}}}); err != nil {
		t.Fatalf("UpsertRoute() error = %v", err)
	}
}
