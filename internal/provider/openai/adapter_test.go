package openai

import (
	"bytes"
	"context"
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
