package bootstrap

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/infra/telemetry"
)

func TestGatewayEngineM1ChatCompletion(t *testing.T) {
	cfg := m1TestConfig()
	tel, err := telemetry.New(context.Background(), telemetry.Config{
		ServiceName:    cfg.Service.Name,
		ServiceVersion: cfg.Service.Version,
		MetricsEnabled: true,
	})
	if err != nil {
		t.Fatalf("telemetry.New() error = %v", err)
	}
	t.Cleanup(func() { _ = tel.Shutdown(context.Background()) })

	gateway, err := newGatewayEngine(cfg, tel, nil)
	if err != nil {
		t.Fatalf("newGatewayEngine() error = %v", err)
	}

	response, err := gateway.Handle(context.Background(), testIncomingRequest(cfg.Gateway.Seed.APIKey))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.StatusCode, response.Body)
	}
	if !strings.Contains(string(response.Body), "M1 mock response") {
		t.Fatalf("body = %s", response.Body)
	}
}

func TestGatewayEngineM1InvalidAPIKey(t *testing.T) {
	cfg := m1TestConfig()
	tel, err := telemetry.New(context.Background(), telemetry.Config{
		ServiceName:    cfg.Service.Name,
		ServiceVersion: cfg.Service.Version,
		MetricsEnabled: true,
	})
	if err != nil {
		t.Fatalf("telemetry.New() error = %v", err)
	}
	t.Cleanup(func() { _ = tel.Shutdown(context.Background()) })

	gateway, err := newGatewayEngine(cfg, tel, nil)
	if err != nil {
		t.Fatalf("newGatewayEngine() error = %v", err)
	}

	response, err := gateway.Handle(context.Background(), testIncomingRequest("wrong"))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.StatusCode, response.Body)
	}
}

func m1TestConfig() Config {
	cfg := DefaultConfig()
	cfg.Gateway.Seed.Enabled = true
	cfg.Gateway.Seed.APIKey = "tg-test-key"
	cfg.Gateway.Seed.ProviderBaseURL = "mock://openai"
	cfg.Normalize()
	return cfg
}

func testIncomingRequest(apiKey string) engine.IncomingRequest {
	return engine.IncomingRequest{
		Method: http.MethodPost,
		Path:   "/v1/chat/completions",
		Header: http.Header{"Authorization": []string{"Bearer " + apiKey}},
		Body: io.NopCloser(strings.NewReader(`{
			"model":"gpt-4o-mini",
			"messages":[{"role":"user","content":"hello"}]
		}`)),
	}
}
