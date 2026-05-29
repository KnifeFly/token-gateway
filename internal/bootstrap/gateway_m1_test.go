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

	gateway, err := newGatewayEngine(context.Background(), cfg, tel, nil, nil, nil)
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

	gateway, err := newGatewayEngine(context.Background(), cfg, tel, nil, nil, nil)
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

func TestGatewayEngineM3Protocols(t *testing.T) {
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

	gateway, err := newGatewayEngine(context.Background(), cfg, tel, nil, nil, nil)
	if err != nil {
		t.Fatalf("newGatewayEngine() error = %v", err)
	}

	tests := []struct {
		name string
		path string
		body string
		want string
	}{
		{name: "responses", path: "/v1/responses", body: `{"model":"gpt-4o-mini","input":"hello"}`, want: "M3 mock response"},
		{name: "embeddings", path: "/v1/embeddings", body: `{"model":"text-embedding-3-small","input":"hello"}`, want: "embedding"},
		{name: "claude", path: "/v1/messages", body: `{"model":"claude-3-5-sonnet-latest","messages":[{"role":"user","content":"hello"}]}`, want: "M3 Claude mock response"},
		{name: "gemini", path: "/v1beta/models/gemini-2.5-flash:generateContent", body: `{"contents":[{"parts":[{"text":"hello"}]}]}`, want: "M3 Gemini mock response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := gateway.Handle(context.Background(), testIncoming(cfg.Gateway.Seed.APIKey, tt.path, tt.body))
			if err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.StatusCode, response.Body)
			}
			if !strings.Contains(string(response.Body), tt.want) {
				t.Fatalf("body = %s", response.Body)
			}
		})
	}
}

func TestGatewayEngineM3Stream(t *testing.T) {
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

	gateway, err := newGatewayEngine(context.Background(), cfg, tel, nil, nil, nil)
	if err != nil {
		t.Fatalf("newGatewayEngine() error = %v", err)
	}

	response, err := gateway.Handle(context.Background(), testIncoming(cfg.Gateway.Seed.APIKey, "/v1/chat/completions", `{
		"model":"gpt-4o-mini",
		"stream":true,
		"messages":[{"role":"user","content":"hello"}]
	}`))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if response.Stream == nil {
		t.Fatal("missing stream")
	}
	chunk, err := response.Stream.Recv(context.Background())
	if err != nil {
		t.Fatalf("Recv() error = %v", err)
	}
	if !strings.Contains(string(chunk), "chat.completion.chunk") {
		t.Fatalf("chunk = %q", chunk)
	}
	if err := response.Stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
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
	return testIncoming(apiKey, "/v1/chat/completions", `{
		"model":"gpt-4o-mini",
		"messages":[{"role":"user","content":"hello"}]
	}`)
}

func testIncoming(apiKey string, path string, body string) engine.IncomingRequest {
	return engine.IncomingRequest{
		Method: http.MethodPost,
		Path:   path,
		Header: http.Header{"Authorization": []string{"Bearer " + apiKey}},
		Body:   io.NopCloser(strings.NewReader(body)),
	}
}
