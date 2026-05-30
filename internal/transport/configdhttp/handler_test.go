package configdhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

func TestHandlerPublishesAndReportsDiagnostics(t *testing.T) {
	ctx := context.Background()
	repo := admin.NewMemoryRepository()
	service := admin.NewService(repo, admin.NewCredentialCodec("secret"), nil)
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

func seedSnapshotConfig(t *testing.T, ctx context.Context, service *admin.Service) {
	t.Helper()
	if _, err := service.CreateAPIKey(ctx, admin.APIKey{TenantID: "tenant", ProjectID: "project", PlaintextKey: "tg-test"}); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if _, err := service.UpsertModel(ctx, admin.ModelConfig{PublicModel: "gpt-4o-mini", Protocol: string(engine.ProtocolNativeOpenAI), Capability: "chat", Enabled: true}); err != nil {
		t.Fatalf("UpsertModel() error = %v", err)
	}
	if _, err := service.UpsertChannel(ctx, admin.ChannelConfig{
		ID:           "channel_1",
		ProviderType: "openai_compatible",
		BaseURL:      "mock://openai",
		Enabled:      true,
		Models:       []admin.ChannelModel{{PublicModel: "gpt-4o-mini", UpstreamModel: "gpt-4o-mini"}},
	}); err != nil {
		t.Fatalf("UpsertChannel() error = %v", err)
	}
	if _, err := service.UpsertRoute(ctx, admin.RoutePolicyConfig{PublicModel: "gpt-4o-mini", Candidates: []admin.RouteCandidate{{ChannelID: "channel_1", Priority: 1, Weight: 100}}}); err != nil {
		t.Fatalf("UpsertRoute() error = %v", err)
	}
}
