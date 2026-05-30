package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/provider/relay"
)

func TestMockMessageStream(t *testing.T) {
	response, err := NewAdapter(nil).Relay(context.Background(), relay.ChannelConfig{BaseURL: "mock://claude"}, relay.Request{
		CanonicalAPI: "claude.messages",
		PublicModel:  "claude-3-5-sonnet-latest",
		RawBody:      []byte(`{"model":"claude-3-5-sonnet-latest","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
		Stream:       true,
	})
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if response.Stream == nil {
		t.Fatal("missing stream")
	}
	chunk, err := response.Stream.Recv(context.Background())
	if err != nil {
		t.Fatalf("Recv() error = %v", err)
	}
	if !strings.Contains(string(chunk), "content_block_delta") {
		t.Fatalf("chunk = %q", chunk)
	}
	_, _ = response.Stream.Recv(context.Background())
	if _, err := response.Stream.Recv(context.Background()); err != io.EOF {
		t.Fatalf("Recv() err = %v, want EOF", err)
	}
}

func TestParseUsageMessageStartShape(t *testing.T) {
	usage := parseUsage([]byte(`{"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":1}}}`))
	if usage.InputTokens != 5 || usage.OutputTokens != 1 || usage.TotalTokens != 6 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestAdapterRewritesModelAndForwardsClaudeCompatibilityHeaders(t *testing.T) {
	var gotVersion string
	var gotBeta string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("Anthropic-Version")
		gotBeta = r.Header.Get("Anthropic-Beta")
		if r.Header.Get("X-API-Key") != "provider-key" {
			t.Errorf("missing provider key")
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Decode() error = %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"message","usage":{"input_tokens":3,"output_tokens":4,"cache_read_input_tokens":2}}`))
	}))
	defer server.Close()

	res, err := NewAdapter(server.Client()).Relay(context.Background(), relay.ChannelConfig{
		BaseURL:       server.URL,
		APIKey:        "provider-key",
		UpstreamModel: "claude-upstream",
	}, relay.Request{
		CanonicalAPI: "claude.messages",
		PublicModel:  "claude-public",
		Headers: http.Header{
			"Anthropic-Version": []string{"2023-06-01"},
			"Anthropic-Beta":    []string{"tools-2024-05-16"},
		},
		RawBody: []byte(`{"model":"claude-public","max_tokens":64,"messages":[{"role":"user","content":[{"type":"text","text":"hi"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGk="}}]}],"tools":[{"name":"lookup","input_schema":{"type":"object"}}]}`),
	})
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if gotVersion != "2023-06-01" || gotBeta != "tools-2024-05-16" {
		t.Fatalf("version = %q beta = %q", gotVersion, gotBeta)
	}
	if payload["model"] != "claude-upstream" {
		t.Fatalf("model = %#v", payload["model"])
	}
	if _, ok := payload["tools"]; !ok {
		t.Fatalf("tools were not preserved: %#v", payload)
	}
	if res.Usage.CachedInputTokens != 2 {
		t.Fatalf("usage = %#v", res.Usage)
	}
}

func TestAdapterHTTPStreamRemainsOpenAfterRelayReturns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":5}}}\n\n"))
		_, _ = w.Write([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":7}}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer server.Close()

	res, err := NewAdapter(server.Client()).Relay(context.Background(), relay.ChannelConfig{
		BaseURL:       server.URL,
		UpstreamModel: "claude-upstream",
	}, relay.Request{
		CanonicalAPI: "claude.messages",
		RawBody:      []byte(`{"model":"claude-public","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"hi"}]}`),
		Stream:       true,
	})
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	defer res.Stream.Close()

	for {
		_, err := res.Stream.Recv(context.Background())
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}
	}
	usage := res.Stream.Usage()
	if usage.InputTokens != 5 || usage.OutputTokens != 7 || usage.TotalTokens != 12 {
		t.Fatalf("usage = %#v", usage)
	}
}
