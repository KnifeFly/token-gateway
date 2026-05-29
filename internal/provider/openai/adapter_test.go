package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/provider/relay"
)

func TestAdapterMockChatCompletion(t *testing.T) {
	res, err := NewAdapter(nil).ChatCompletions(context.Background(), relay.ChannelConfig{
		BaseURL:       "mock://openai",
		UpstreamModel: "gpt-4o-mini",
	}, relay.ChatCompletionRequest{
		PublicModel: "gpt-4o-mini",
		RawBody:     []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`),
	})
	if err != nil {
		t.Fatalf("ChatCompletions() error = %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if res.Usage.TotalTokens == 0 {
		t.Fatal("missing usage")
	}
}

func TestAdapterMapsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := NewAdapter(server.Client()).ChatCompletions(context.Background(), relay.ChannelConfig{
		BaseURL:       server.URL,
		UpstreamModel: "gpt-4o-mini",
	}, relay.ChatCompletionRequest{
		PublicModel: "gpt-4o-mini",
		RawBody:     []byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`),
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
