package bootstrap

import (
	"context"
	"encoding/json"
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
		{name: "moderation", path: "/v1/moderations", body: `{"model":"moderation-latest","input":"hello"}`, want: `"flagged":false`},
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

func TestGatewayEngineM4VideoTaskIdempotencyAndCancel(t *testing.T) {
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

	body := `{"model":"seedance-2.0-text-to-video","prompt":"camera move"}`
	response, err := gateway.Handle(context.Background(), testIncomingWithHeaders(cfg.Gateway.Seed.APIKey, "/v1/videos/generations", body, http.Header{"Idempotency-Key": []string{"task-idem"}}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	taskID := taskIDFromResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.StatusCode, response.Body)
	}

	duplicate, err := gateway.Handle(context.Background(), testIncomingWithHeaders(cfg.Gateway.Seed.APIKey, "/v1/videos/generations", body, http.Header{"Idempotency-Key": []string{"task-idem"}}))
	if err != nil {
		t.Fatalf("duplicate Handle() error = %v", err)
	}
	if got := taskIDFromResponse(t, duplicate); got != taskID {
		t.Fatalf("duplicate task id = %q, want %q", got, taskID)
	}

	detail, err := gateway.Handle(context.Background(), testIncomingWithMethod(cfg.Gateway.Seed.APIKey, http.MethodGet, "/v1/tasks/"+taskID, ""))
	if err != nil {
		t.Fatalf("detail Handle() error = %v", err)
	}
	if got := taskIDFromResponse(t, detail); got != taskID {
		t.Fatalf("detail task id = %q, want %q", got, taskID)
	}

	cancel, err := gateway.Handle(context.Background(), testIncomingWithMethod(cfg.Gateway.Seed.APIKey, http.MethodPost, "/v1/tasks/"+taskID+"/cancel", ""))
	if err != nil {
		t.Fatalf("cancel Handle() error = %v", err)
	}
	if !strings.Contains(string(cancel.Body), `"status":"cancelled"`) {
		t.Fatalf("cancel body = %s", cancel.Body)
	}
}

func TestGatewayEngineM4FileUpload(t *testing.T) {
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

	response, err := gateway.Handle(context.Background(), testIncomingWithHeaders(cfg.Gateway.Seed.APIKey, "/v1/files/upload/base64", `{"base64_data":"aGk=","file_name":"hi.txt"}`, http.Header{"Idempotency-Key": []string{"file-idem"}}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(response.Body), `"success":true`) {
		t.Fatalf("response = %d %s", response.StatusCode, response.Body)
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
	return testIncomingWithMethod(apiKey, http.MethodPost, path, body)
}

func testIncomingWithMethod(apiKey string, method string, path string, body string) engine.IncomingRequest {
	return engine.IncomingRequest{
		Method: method,
		Path:   path,
		Header: http.Header{"Authorization": []string{"Bearer " + apiKey}},
		Body:   io.NopCloser(strings.NewReader(body)),
	}
}

func testIncomingWithHeaders(apiKey string, path string, body string, header http.Header) engine.IncomingRequest {
	request := testIncoming(apiKey, path, body)
	for key, values := range header {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	return request
}

func taskIDFromResponse(t *testing.T, response *engine.GatewayResponse) string {
	t.Helper()
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v body = %s", err, response.Body)
	}
	if payload.ID == "" {
		t.Fatalf("missing id in body = %s", response.Body)
	}
	return payload.ID
}
