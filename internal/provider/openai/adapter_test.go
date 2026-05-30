package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
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

func TestAdapterHTTPStreamRemainsOpenAfterRelayReturns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer server.Close()

	res, err := NewAdapter(server.Client()).Relay(context.Background(), relay.ChannelConfig{
		BaseURL:       server.URL,
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
	defer res.Stream.Close()

	chunk, err := res.Stream.Recv(context.Background())
	if err != nil {
		t.Fatalf("Recv() error = %v", err)
	}
	if !strings.Contains(string(chunk), "hello") {
		t.Fatalf("chunk = %q", chunk)
	}
}

func TestAdapterMapsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":"server_overloaded","message":"do not leak"}}`))
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
	if strings.Contains(providerErr.Message, "do not leak") {
		t.Fatalf("provider error leaked body: %#v", providerErr)
	}
	if providerErr.ProviderCode != "server_overloaded" {
		t.Fatalf("provider code = %q", providerErr.ProviderCode)
	}
}

func TestAdapterPreservesChatToolWireShapeAndRewritesModel(t *testing.T) {
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[]}}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`))
	}))
	defer server.Close()

	_, err := NewAdapter(server.Client()).Relay(context.Background(), relay.ChannelConfig{
		BaseURL:       server.URL,
		UpstreamModel: "gpt-upstream",
	}, relay.Request{
		CanonicalAPI: "openai.chat_completions",
		PublicModel:  "gpt-public",
		RawBody:      []byte(`{"model":"gpt-public","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]},{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]}],"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]}`),
		ContentType:  "application/json",
	})
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if payload["model"] != "gpt-upstream" {
		t.Fatalf("model = %#v", payload["model"])
	}
	if _, ok := payload["tools"]; !ok {
		t.Fatalf("tools were not preserved: %#v", payload)
	}
	messages := payload["messages"].([]any)
	assistant := messages[1].(map[string]any)
	if _, ok := assistant["tool_calls"]; !ok {
		t.Fatalf("tool_calls were not preserved: %#v", assistant)
	}
}

func TestAdapterRelaysNativeImageEndpointAndRewritesModel(t *testing.T) {
	var gotPath string
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://example.com/image.png"}]}`))
	}))
	defer server.Close()

	_, err := NewAdapter(server.Client()).Relay(context.Background(), relay.ChannelConfig{
		BaseURL:       server.URL,
		UpstreamModel: "upstream-image",
	}, relay.Request{
		CanonicalAPI: "unified.image_generation",
		PublicModel:  "gpt-image-public",
		RawBody:      []byte(`{"model":"gpt-image-public","prompt":"cat"}`),
		ContentType:  "application/json",
	})
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if gotPath != "/v1/images/generations" {
		t.Fatalf("path = %q", gotPath)
	}
	if !strings.Contains(gotBody, `"model":"upstream-image"`) {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestAdapterRelaysMultipartMediaAndRewritesModel(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", "public-audio"); err != nil {
		t.Fatalf("WriteField() error = %v", err)
	}
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte("wav")); err != nil {
		t.Fatalf("part.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var gotModel string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("ParseMultipartForm() error = %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		gotModel = r.FormValue("model")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"hello"}`))
	}))
	defer server.Close()

	_, err = NewAdapter(server.Client()).Relay(context.Background(), relay.ChannelConfig{
		BaseURL:       server.URL,
		UpstreamModel: "upstream-audio",
	}, relay.Request{
		CanonicalAPI: "unified.audio_transcription",
		PublicModel:  "public-audio",
		RawBody:      body.Bytes(),
		ContentType:  writer.FormDataContentType(),
	})
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if gotPath != "/v1/audio/transcriptions" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotModel != "upstream-audio" {
		t.Fatalf("model = %q", gotModel)
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

func TestParseUsageDetails(t *testing.T) {
	usage := parseUsage([]byte(`{"usage":{"prompt_tokens":11,"completion_tokens":13,"total_tokens":24,"prompt_tokens_details":{"cached_tokens":5,"audio_tokens":2},"completion_tokens_details":{"reasoning_tokens":7,"audio_tokens":3}}}`))
	if usage.InputTokens != 11 || usage.OutputTokens != 13 || usage.TotalTokens != 24 {
		t.Fatalf("usage = %#v", usage)
	}
	if usage.CachedInputTokens != 5 || usage.ReasoningTokens != 7 || usage.AudioInputTokens != 2 || usage.AudioOutputTokens != 3 {
		t.Fatalf("detail usage = %#v", usage)
	}
}

func TestParseUsageResponsesCompletedEvent(t *testing.T) {
	usage := parseUsage([]byte(`{"type":"response.completed","response":{"usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5,"input_tokens_details":{"cached_tokens":1},"output_tokens_details":{"reasoning_tokens":2}}}}`))
	if usage.InputTokens != 2 || usage.OutputTokens != 3 || usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v", usage)
	}
	if usage.CachedInputTokens != 1 || usage.ReasoningTokens != 2 {
		t.Fatalf("detail usage = %#v", usage)
	}
}
