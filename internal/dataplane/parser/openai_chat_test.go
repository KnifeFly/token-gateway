package parser

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func TestOpenAIChatParser(t *testing.T) {
	state := &engine.RequestState{
		CanonicalAPI: engine.CanonicalOpenAIChatCompletions,
		Incoming: engine.IncomingRequest{
			Header: http.Header{},
			Body: io.NopCloser(strings.NewReader(`{
				"model":"gpt-4o-mini",
				"messages":[{"role":"user","content":"hello"}]
			}`)),
		},
	}

	err := NewOpenAIChatParser(1024).Parse(context.Background(), state)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if state.RequestedModel != "gpt-4o-mini" {
		t.Fatalf("RequestedModel = %q", state.RequestedModel)
	}
	if state.EstimatedUsage.InputTokens == 0 {
		t.Fatal("missing usage estimate")
	}
}

func TestOpenAIChatParserAcceptsStream(t *testing.T) {
	state := &engine.RequestState{
		CanonicalAPI: engine.CanonicalOpenAIChatCompletions,
		Incoming: engine.IncomingRequest{
			Body: io.NopCloser(strings.NewReader(`{
				"model":"gpt-4o-mini",
				"stream":true,
				"messages":[{"role":"user","content":"hello"}]
			}`)),
		},
	}

	err := NewOpenAIChatParser(1024).Parse(context.Background(), state)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !state.Stream {
		t.Fatal("stream flag was not parsed")
	}
}

func TestParserGeminiModelFromPath(t *testing.T) {
	state := &engine.RequestState{
		CanonicalAPI: engine.CanonicalGeminiGenerateContent,
		Incoming: engine.IncomingRequest{
			Path: "/v1beta/models/gemini-2.5-flash:streamGenerateContent",
			Body: io.NopCloser(strings.NewReader(`{"contents":[{"parts":[{"text":"hi"}]}]}`)),
		},
	}

	err := NewOpenAIChatParser(1024).Parse(context.Background(), state)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if state.RequestedModel != "gemini-2.5-flash" || !state.Stream {
		t.Fatalf("model = %q stream = %v", state.RequestedModel, state.Stream)
	}
}

func TestParserUnifiedMediaVideo(t *testing.T) {
	state := &engine.RequestState{
		CanonicalAPI: engine.CanonicalVideoGeneration,
		Incoming: engine.IncomingRequest{
			Path:   "/v1/videos/generations",
			Header: http.Header{"Idempotency-Key": []string{"idem-video"}},
			Body: io.NopCloser(strings.NewReader(`{
				"model":"seedance-2.0-text-to-video",
				"prompt":"camera move",
				"callback_url":"https://example.com/callback",
				"metadata":{"scene":"1"},
				"model_params":{"seed":123}
			}`)),
		},
	}

	err := NewOpenAIChatParser(1024).Parse(context.Background(), state)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !state.Async || state.IdempotencyKey != "idem-video" {
		t.Fatalf("async = %v idempotency = %q", state.Async, state.IdempotencyKey)
	}
	if state.RequestedModel != "seedance-2.0-text-to-video" {
		t.Fatalf("RequestedModel = %q", state.RequestedModel)
	}
	if state.Parsed.Media == nil || state.Parsed.Media.Kind != "video.generation" || state.Parsed.Media.Metadata["scene"] != "1" {
		t.Fatalf("media = %#v", state.Parsed.Media)
	}
	if state.Parsed.Media.ModelParams["seed"].(float64) != 123 {
		t.Fatalf("model_params = %#v", state.Parsed.Media.ModelParams)
	}
}

func TestParserFileBase64(t *testing.T) {
	state := &engine.RequestState{
		CanonicalAPI: engine.CanonicalFileUploadBase64,
		Incoming: engine.IncomingRequest{
			Path:   "/v1/files/upload/base64",
			Header: http.Header{"Idempotency-Key": []string{"idem-file"}},
			Body:   io.NopCloser(strings.NewReader(`{"base64_data":"aGk=","file_name":"hi.txt"}`)),
		},
	}

	err := NewOpenAIChatParser(1024).Parse(context.Background(), state)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if state.Parsed.File == nil || state.Parsed.File.SizeBytes != 2 || state.IdempotencyKey != "idem-file" {
		t.Fatalf("file = %#v idempotency = %q", state.Parsed.File, state.IdempotencyKey)
	}
}

func TestBodyStoreRejectsOversizeBody(t *testing.T) {
	_, err := (BodyStore{MaxBytes: 3}).Read(strings.NewReader("abcd"))
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeInvalidArgument {
		t.Fatalf("error = %v, want invalid argument", err)
	}
}
