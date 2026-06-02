package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func TestCredentialExtractorSupportsBearerAndAPIKey(t *testing.T) {
	extractor := CredentialExtractor{}
	for name, header := range map[string]http.Header{
		"bearer":    {"Authorization": []string{"Bearer sk-test"}},
		"x-api-key": {"X-API-Key": []string{"sk-test"}},
	} {
		t.Run(name, func(t *testing.T) {
			key, err := extractor.Extract(header)
			if err != nil {
				t.Fatalf("Extract() error = %v", err)
			}
			if key != "sk-test" {
				t.Fatalf("key = %q", key)
			}
		})
	}
}

func TestSnapshotAuthenticator(t *testing.T) {
	key := "sk-local"
	state := &engine.RequestState{
		Snapshot: fakeSnapshot{apiKey: engine.APIKeyView{
			ID:            "key_1",
			TenantID:      "tenant_1",
			ProjectID:     "project_1",
			Hash:          HashAPIKey(key),
			Enabled:       true,
			AllowedModels: []string{"gpt-4o-mini"},
		}},
		Incoming: engine.IncomingRequest{Header: http.Header{"Authorization": []string{"Bearer " + key}}},
	}

	err := NewSnapshotAuthenticator().Authenticate(context.Background(), state)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if state.Principal == nil || state.Principal.APIKeyID != "key_1" {
		t.Fatalf("principal = %#v", state.Principal)
	}
}

func TestSnapshotAuthenticatorRejectsExpiredAPIKey(t *testing.T) {
	key := "sk-local"
	expiresAt := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	state := &engine.RequestState{
		StartedAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		Snapshot: fakeSnapshot{apiKey: engine.APIKeyView{
			ID:        "key_1",
			TenantID:  "tenant_1",
			ProjectID: "project_1",
			Hash:      HashAPIKey(key),
			Enabled:   true,
			ExpiresAt: &expiresAt,
		}},
		Incoming: engine.IncomingRequest{Header: http.Header{"Authorization": []string{"Bearer " + key}}},
	}

	err := NewSnapshotAuthenticator().Authenticate(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeUnauthorized {
		t.Fatalf("error = %v, want unauthorized", err)
	}
}

func TestSnapshotAuthenticatorEnforcesIPAllowlist(t *testing.T) {
	key := "sk-local"
	state := &engine.RequestState{
		Snapshot: fakeSnapshot{apiKey: engine.APIKeyView{
			ID:          "key_1",
			TenantID:    "tenant_1",
			ProjectID:   "project_1",
			Hash:        HashAPIKey(key),
			Enabled:     true,
			IPAllowlist: []string{"203.0.113.0/24"},
		}},
		ClientIP: "198.51.100.10:443",
		Incoming: engine.IncomingRequest{Header: http.Header{"Authorization": []string{"Bearer " + key}}},
	}

	err := NewSnapshotAuthenticator().Authenticate(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeUnauthorized {
		t.Fatalf("error = %v, want unauthorized", err)
	}

	state.ClientIP = "203.0.113.10:443"
	if err := NewSnapshotAuthenticator().Authenticate(context.Background(), state); err != nil {
		t.Fatalf("Authenticate(allowed IP) error = %v", err)
	}
}

func TestSnapshotAuthenticatorSupportsHMACAndLegacyHashes(t *testing.T) {
	key := "sk-local"
	for name, hash := range map[string]string{
		"hmac":   HashAPIKeyHMAC(key, "server-secret"),
		"legacy": HashAPIKey(key),
	} {
		t.Run(name, func(t *testing.T) {
			state := &engine.RequestState{
				Snapshot: fakeSnapshot{apiKey: engine.APIKeyView{
					ID:        "key_1",
					TenantID:  "tenant_1",
					ProjectID: "project_1",
					Hash:      hash,
					Enabled:   true,
				}},
				Incoming: engine.IncomingRequest{Header: http.Header{"Authorization": []string{"Bearer " + key}}},
			}

			err := NewSnapshotAuthenticatorWithOptions(nil, WithAPIKeyHashSecret("server-secret")).Authenticate(context.Background(), state)
			if err != nil {
				t.Fatalf("Authenticate() error = %v", err)
			}
			if state.APIKeyID != "key_1" {
				t.Fatalf("api key id = %q", state.APIKeyID)
			}
		})
	}
}

func TestSnapshotAuthenticatorRejectsInvalidKey(t *testing.T) {
	state := &engine.RequestState{
		Snapshot: fakeSnapshot{},
		Incoming: engine.IncomingRequest{Header: http.Header{"Authorization": []string{"Bearer wrong"}}},
	}

	err := NewSnapshotAuthenticator().Authenticate(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeUnauthorized {
		t.Fatalf("error = %v, want unauthorized", err)
	}
}

type fakeSnapshot struct {
	apiKey engine.APIKeyView
}

func (s fakeSnapshot) Ref() engine.SnapshotRef { return engine.SnapshotRef{Version: "test"} }

func (s fakeSnapshot) ListModels() []engine.ModelView { return nil }

func (s fakeSnapshot) LookupAPIKeyHash(hash string) (engine.APIKeyView, bool) {
	return s.apiKey, s.apiKey.Hash == hash
}

func (s fakeSnapshot) LookupModel(string) (engine.ModelView, bool) { return engine.ModelView{}, false }

func (s fakeSnapshot) LookupRoute(string) (engine.RoutePolicyView, bool) {
	return engine.RoutePolicyView{}, false
}

func (s fakeSnapshot) LookupChannel(string) (engine.ChannelView, bool) {
	return engine.ChannelView{}, false
}

func (s fakeSnapshot) LookupPrice(string) (engine.PriceRuleView, bool) {
	return engine.PriceRuleView{}, false
}

func (s fakeSnapshot) LookupLimit(string) (engine.LimitRuleView, bool) {
	return engine.LimitRuleView{}, false
}

func (s fakeSnapshot) LookupLimits(engine.LimitScope) []engine.LimitRuleView { return nil }

func (s fakeSnapshot) LookupPluginBindings(string) []engine.PluginBindingView {
	return nil
}

func (s fakeSnapshot) IsAPIKeyRevoked(string) bool {
	return false
}
