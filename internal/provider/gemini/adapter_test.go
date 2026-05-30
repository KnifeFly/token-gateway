package gemini

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

func TestMockGenerateContentStream(t *testing.T) {
	response, err := NewAdapter(nil).Relay(context.Background(), relay.ChannelConfig{BaseURL: "mock://gemini"}, relay.Request{
		CanonicalAPI:  "gemini.generate_content",
		PublicModel:   "gemini-2.5-flash",
		UpstreamModel: "gemini-2.5-flash",
		RawBody:       []byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`),
		Stream:        true,
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
	if !strings.Contains(string(chunk), "M3 Gemini mock response") {
		t.Fatalf("chunk = %q", chunk)
	}
	_, _ = response.Stream.Recv(context.Background())
	if _, err := response.Stream.Recv(context.Background()); err != io.EOF {
		t.Fatalf("Recv() err = %v, want EOF", err)
	}
}

func TestParseUsageMetadataShape(t *testing.T) {
	usage := parseUsage([]byte(`{"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":9,"totalTokenCount":16}}`))
	if usage.InputTokens != 7 || usage.OutputTokens != 9 || usage.TotalTokens != 16 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestAdapterUsesV1BetaBaseURLAndPreservesToolsSafetySettings(t *testing.T) {
	var gotKey string
	var payload map[string]any
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.URL.Query().Get("key")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Decode() error = %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":9,"totalTokenCount":16,"cachedContentTokenCount":3,"thoughtsTokenCount":2,"promptTokensDetails":[{"modality":"IMAGE","tokenCount":4}],"candidatesTokensDetails":[{"modality":"AUDIO","tokenCount":5}]}}`))
	}))
	defer server.Close()

	res, err := NewAdapter(server.Client()).Relay(context.Background(), relay.ChannelConfig{
		BaseURL:       server.URL + "/v1beta",
		APIKey:        "gemini-key",
		UpstreamModel: "gemini-upstream",
	}, relay.Request{
		CanonicalAPI:  "gemini.generate_content",
		PublicModel:   "gemini-public",
		UpstreamModel: "gemini-upstream",
		RawBody:       []byte(`{"contents":[{"parts":[{"text":"hi"}]}],"tools":[{"functionDeclarations":[{"name":"lookup"}]}],"safetySettings":[{"category":"HARM_CATEGORY_DANGEROUS_CONTENT","threshold":"BLOCK_NONE"}]}`),
	})
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if gotPath != "/v1beta/models/gemini-upstream:generateContent" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotKey != "gemini-key" {
		t.Fatalf("key = %q", gotKey)
	}
	if _, ok := payload["tools"]; !ok {
		t.Fatalf("tools were not preserved: %#v", payload)
	}
	if _, ok := payload["safetySettings"]; !ok {
		t.Fatalf("safetySettings were not preserved: %#v", payload)
	}
	if res.Usage.CachedInputTokens != 3 || res.Usage.ReasoningTokens != 2 || res.Usage.ImageInputTokens != 4 || res.Usage.AudioOutputTokens != 5 {
		t.Fatalf("usage = %#v", res.Usage)
	}
}

func TestAdapterHTTPStreamRemainsOpenAfterRelayReturns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"usageMetadata\":{\"promptTokenCount\":2}}\n\n"))
		_, _ = w.Write([]byte("data: {\"usageMetadata\":{\"candidatesTokenCount\":3}}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer server.Close()

	res, err := NewAdapter(server.Client()).Relay(context.Background(), relay.ChannelConfig{
		BaseURL:       server.URL,
		UpstreamModel: "gemini-upstream",
	}, relay.Request{
		CanonicalAPI:  "gemini.generate_content",
		PublicModel:   "gemini-public",
		UpstreamModel: "gemini-upstream",
		RawBody:       []byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`),
		Stream:        true,
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
	if usage.InputTokens != 2 || usage.OutputTokens != 3 || usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v", usage)
	}
}
