package contract_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/provider/claude"
	"github.com/KnifeFly/token-gateway/internal/provider/gemini"
	"github.com/KnifeFly/token-gateway/internal/provider/openai"
	"github.com/KnifeFly/token-gateway/internal/provider/relay"
)

func TestProviderProtocolHTTPFixtures(t *testing.T) {
	t.Run("openai chat tools", func(t *testing.T) {
		var payload map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/chat/completions" {
				t.Errorf("path = %q", r.URL.Path)
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("Decode() error = %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`))
		}))
		defer server.Close()

		_, err := openai.NewAdapter(server.Client()).Relay(context.Background(), relay.ChannelConfig{
			BaseURL:       server.URL,
			UpstreamModel: "gpt-upstream",
		}, relay.Request{
			CanonicalAPI: "openai.chat_completions",
			PublicModel:  "gpt-public",
			ContentType:  "application/json",
			RawBody:      []byte(`{"model":"gpt-public","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}],"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}],"tool_choice":"auto","stream_options":{"include_usage":true}}`),
		})
		if err != nil {
			t.Fatalf("Relay() error = %v", err)
		}
		if payload["model"] != "gpt-upstream" || payload["tools"] == nil || payload["stream_options"] == nil {
			t.Fatalf("payload = %#v", payload)
		}
	})

	t.Run("claude multimodal tools", func(t *testing.T) {
		var payload map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Anthropic-Version") != "2023-06-01" {
				t.Errorf("Anthropic-Version = %q", r.Header.Get("Anthropic-Version"))
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("Decode() error = %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":2,"output_tokens":3}}`))
		}))
		defer server.Close()

		_, err := claude.NewAdapter(server.Client()).Relay(context.Background(), relay.ChannelConfig{
			BaseURL:       server.URL,
			UpstreamModel: "claude-upstream",
		}, relay.Request{
			CanonicalAPI: "claude.messages",
			PublicModel:  "claude-public",
			Headers:      http.Header{"Anthropic-Version": []string{"2023-06-01"}},
			RawBody:      []byte(`{"model":"claude-public","max_tokens":64,"messages":[{"role":"user","content":[{"type":"text","text":"hi"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGk="}}]}],"tools":[{"name":"lookup","input_schema":{"type":"object"}}],"tool_choice":{"type":"auto"}}`),
		})
		if err != nil {
			t.Fatalf("Relay() error = %v", err)
		}
		if payload["model"] != "claude-upstream" || payload["tools"] == nil || payload["tool_choice"] == nil {
			t.Fatalf("payload = %#v", payload)
		}
	})

	t.Run("gemini tools safety", func(t *testing.T) {
		var payload map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1beta/models/gemini-upstream:generateContent" {
				t.Errorf("path = %q", r.URL.Path)
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("Decode() error = %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":3,"totalTokenCount":5}}`))
		}))
		defer server.Close()

		_, err := gemini.NewAdapter(server.Client()).Relay(context.Background(), relay.ChannelConfig{
			BaseURL:       server.URL + "/v1beta",
			UpstreamModel: "gemini-upstream",
		}, relay.Request{
			CanonicalAPI:  "gemini.generate_content",
			PublicModel:   "gemini-public",
			UpstreamModel: "gemini-upstream",
			RawBody:       []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"tools":[{"functionDeclarations":[{"name":"lookup"}]}],"safetySettings":[{"category":"HARM_CATEGORY_DANGEROUS_CONTENT","threshold":"BLOCK_NONE"}]}`),
		})
		if err != nil {
			t.Fatalf("Relay() error = %v", err)
		}
		if payload["tools"] == nil || payload["safetySettings"] == nil {
			t.Fatalf("payload = %#v", payload)
		}
	})
}
