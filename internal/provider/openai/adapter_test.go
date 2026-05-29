package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/provider/relay"
)

func TestAdapterMockChatCompletion(t *testing.T) {
	res, err := NewAdapter(nil).Relay(context.Background(), relay.ChannelConfig{
		BaseURL:       "mock://openai",
		UpstreamModel: "gpt-4o-mini",
	}, relay.Request{
		CanonicalAPI: "openai.chat_completions",
		PublicModel:  "gpt-4o-mini",
		RawBody:      []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if res.Usage.TotalTokens == 0 {
		t.Fatal("missing usage")
	}
}

func TestAdapterMockStream(t *testing.T) {
	res, err := NewAdapter(nil).Relay(context.Background(), relay.ChannelConfig{
		BaseURL:       "mock://openai",
		UpstreamModel: "gpt-4o-mini",
	}, relay.Request{
		CanonicalAPI: "openai.chat_completions",
		PublicModel:  "gpt-4o-mini",
		RawBody:      []byte(`{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
		Stream:       true,
	})
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if res.Stream == nil {
		t.Fatal("missing stream")
	}
	chunk, err := res.Stream.Recv(context.Background())
	if err != nil {
		t.Fatalf("Recv() error = %v", err)
	}
	if !strings.Contains(string(chunk), "chat.completion.chunk") {
		t.Fatalf("chunk = %q", chunk)
	}
	_, _ = res.Stream.Recv(context.Background())
	if _, err := res.Stream.Recv(context.Background()); err != io.EOF {
		t.Fatalf("Recv() err = %v, want EOF", err)
	}
}

func TestAdapterMapsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := NewAdapter(server.Client()).Relay(context.Background(), relay.ChannelConfig{
		BaseURL:       server.URL,
		UpstreamModel: "gpt-4o-mini",
	}, relay.Request{
		CanonicalAPI: "openai.chat_completions",
		PublicModel:  "gpt-4o-mini",
		RawBody:      []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`),
	})
	providerErr, ok := err.(*relay.ProviderError)
	if !ok {
		t.Fatalf("error = %T %v, want ProviderError", err, err)
	}
	if providerErr.Code != "provider_unavailable" || !providerErr.Retryable {
		t.Fatalf("provider error = %#v", providerErr)
	}
}

func TestChatCompletionsURLAcceptsV1BaseURL(t *testing.T) {
	got, err := chatCompletionsURL("https://api.example.com/v1")
	if err != nil {
		t.Fatalf("chatCompletionsURL() error = %v", err)
	}
	want := "https://api.example.com/v1/chat/completions"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}

func TestParseUsageResponsesShape(t *testing.T) {
	usage := parseUsage([]byte(`{"usage":{"input_tokens":11,"output_tokens":13,"total_tokens":24}}`))
	if usage.InputTokens != 11 || usage.OutputTokens != 13 || usage.TotalTokens != 24 {
		t.Fatalf("usage = %#v", usage)
	}
}
