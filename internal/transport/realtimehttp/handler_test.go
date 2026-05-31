package realtimehttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/dataplane/realtime"
	dpsnapshot "github.com/KnifeFly/token-gateway/internal/dataplane/snapshot"
	"github.com/prometheus/client_golang/prometheus"
)

func TestCreateSessionRequiresAuth(t *testing.T) {
	handler := newTestHandler(t, realtime.DisabledEngine{})
	req := httptest.NewRequest(http.MethodPost, "/v1/realtime/sessions", strings.NewReader(`{"model":"gpt-realtime"}`))
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	assertError(t, res, http.StatusUnauthorized, "unauthorized")
}

func TestCreateSessionDisabledReturnsFeatureNotEnabled(t *testing.T) {
	handler := newTestHandler(t, realtime.DisabledEngine{})
	req := httptest.NewRequest(http.MethodPost, "/v1/realtime/sessions", strings.NewReader(`{"model":"gpt-realtime"}`))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("X-Request-ID", "req_realtime_1")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	assertError(t, res, http.StatusNotImplemented, "feature_not_enabled")
	if strings.Contains(res.Body.String(), `"request_id":"req_realtime_1"`) {
		t.Fatalf("client request id leaked into internal request_id: %s", res.Body.String())
	}
	if got := res.Header().Get("X-Request-ID"); got == "" || got == "req_realtime_1" {
		t.Fatalf("internal request id header = %q", got)
	}
	if got := res.Header().Get("X-Client-Request-ID"); got != "req_realtime_1" {
		t.Fatalf("client request id header = %q", got)
	}
}

func TestCreateSessionRejectsForbiddenModel(t *testing.T) {
	handler := newTestHandler(t, realtime.DisabledEngine{})
	req := httptest.NewRequest(http.MethodPost, "/v1/realtime/sessions", strings.NewReader(`{"model":"other-realtime"}`))
	req.Header.Set("Authorization", "Bearer test-key")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	assertError(t, res, http.StatusForbidden, "forbidden")
}

func TestGetSessionDisabledReturnsFeatureNotEnabled(t *testing.T) {
	handler := newTestHandler(t, realtime.DisabledEngine{})
	req := httptest.NewRequest(http.MethodGet, "/v1/realtime/sessions/sess_123", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	assertError(t, res, http.StatusNotImplemented, "feature_not_enabled")
}

func TestWebSocketStubDisabledReturnsFeatureNotEnabled(t *testing.T) {
	handler := newTestHandler(t, realtime.DisabledEngine{})
	req := httptest.NewRequest(http.MethodGet, "/v1/realtime?session_id=sess_123", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	assertError(t, res, http.StatusNotImplemented, "feature_not_enabled")
}

func TestCreateSessionSuccessEncodesContract(t *testing.T) {
	handler := newTestHandler(t, successfulEngine{})
	req := httptest.NewRequest(http.MethodPost, "/v1/realtime/sessions", strings.NewReader(`{"model":"gpt-realtime"}`))
	req.Header.Set("Authorization", "Bearer test-key")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body sessionResponseBody
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if body.ID != "sess_ok" || body.Object != "realtime.session" || body.ExpiresAt == 0 {
		t.Fatalf("response = %+v", body)
	}
}

func newTestHandler(t *testing.T, rt realtime.Engine) http.Handler {
	t.Helper()
	indexed, err := dpsnapshot.Build(cpsnapshot.RuntimeSnapshot{
		Version:   "test-snapshot",
		CreatedAt: time.Now().UTC(),
		APIKeys: []cpsnapshot.APIKeyRuntime{
			{
				ID:            "key_1",
				TenantID:      "tenant_1",
				ProjectID:     "project_1",
				Name:          "test",
				KeyHash:       auth.HashAPIKey("test-key"),
				Enabled:       true,
				AllowedModels: []string{"gpt-realtime"},
			},
		},
		Models: []cpsnapshot.ModelRuntime{
			{PublicModel: "gpt-realtime", Protocol: string(engine.ProtocolUnified), Capability: "realtime", Enabled: true},
			{PublicModel: "other-realtime", Protocol: string(engine.ProtocolUnified), Capability: "realtime", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	handler, err := NewHandler(
		dpsnapshot.NewProvider(dpsnapshot.NewStore(indexed)),
		auth.NewSnapshotAuthenticator(),
		rt,
		engine.NoopObserveRecorder{},
		prometheus.NewRegistry(),
		nil,
	)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	mux := http.NewServeMux()
	handler.Register(mux)
	return mux
}

func assertError(t *testing.T, res *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if res.Code != status {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid error response: %v", err)
	}
	if body.Error.Code != code {
		t.Fatalf("error code = %q, body = %s", body.Error.Code, res.Body.String())
	}
	if body.Error.RequestID == "" {
		t.Fatalf("missing request_id: %s", res.Body.String())
	}
}

type successfulEngine struct{}

func (successfulEngine) CreateSession(context.Context, realtime.SessionRequest) (*realtime.Session, error) {
	return &realtime.Session{ID: "sess_ok", Object: "realtime.session", ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func (successfulEngine) GetSession(context.Context, string, string, string) (*realtime.Session, error) {
	return &realtime.Session{ID: "sess_ok", Object: "realtime.session", ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func (successfulEngine) HandleConnection(context.Context, realtime.Connection) error {
	return nil
}
