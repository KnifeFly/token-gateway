package parser

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
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

func TestOpenAIChatParserAcceptsAssistantToolCallsWithoutContent(t *testing.T) {
	state := &engine.RequestState{
		CanonicalAPI: engine.CanonicalOpenAIChatCompletions,
		Incoming: engine.IncomingRequest{
			Body: io.NopCloser(strings.NewReader(`{
				"model":"gpt-4o-mini",
				"messages":[
					{"role":"user","content":"call a tool"},
					{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]}
				],
				"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]
			}`)),
		},
	}

	err := NewOpenAIChatParser(2048).Parse(context.Background(), state)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if state.RequestedModel != "gpt-4o-mini" {
		t.Fatalf("RequestedModel = %q", state.RequestedModel)
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

func TestParserGeminiRequiresValidContents(t *testing.T) {
	state := &engine.RequestState{
		CanonicalAPI: engine.CanonicalGeminiGenerateContent,
		Incoming: engine.IncomingRequest{
			Path: "/v1beta/models/gemini-2.5-flash:generateContent",
			Body: io.NopCloser(strings.NewReader(`{"tools":[]}`)),
		},
	}

	err := NewOpenAIChatParser(1024).Parse(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeInvalidArgument {
		t.Fatalf("error = %v, want invalid argument", err)
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

func TestParserNativeOpenAIImageGenerationIsSynchronous(t *testing.T) {
	state := &engine.RequestState{
		CanonicalAPI: engine.CanonicalImageGeneration,
		ProtocolMode: engine.ProtocolNativeOpenAI,
		Incoming: engine.IncomingRequest{
			Path:   "/v1/images/generations",
			Header: http.Header{},
			Body:   io.NopCloser(strings.NewReader(`{"model":"gpt-image-1","prompt":"cat","size":"1024x1024"}`)),
		},
	}

	err := NewOpenAIChatParser(1024).Parse(context.Background(), state)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if state.Async || state.Parsed.Media != nil {
		t.Fatalf("native OpenAI media parsed as async media: async=%v media=%#v", state.Async, state.Parsed.Media)
	}
	if state.RequestedModel != "gpt-image-1" || state.Parsed.Model != "gpt-image-1" {
		t.Fatalf("model = %q parsed = %q", state.RequestedModel, state.Parsed.Model)
	}
}

func TestParserNativeOpenAIAudioTranscriptionMultipart(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", "whisper-1"); err != nil {
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
	state := &engine.RequestState{
		CanonicalAPI: engine.CanonicalAudioTranscription,
		ProtocolMode: engine.ProtocolNativeOpenAI,
		Incoming: engine.IncomingRequest{
			Path:   "/v1/audio/transcriptions",
			Header: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
			Body:   io.NopCloser(bytes.NewReader(body.Bytes())),
		},
	}

	err = NewOpenAIChatParser(4096).Parse(context.Background(), state)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if state.Async || state.RequestedModel != "whisper-1" {
		t.Fatalf("async = %v model = %q", state.Async, state.RequestedModel)
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
	if !strings.HasPrefix(state.Parsed.File.ContentHash, "sha256:") {
		t.Fatalf("content hash = %q", state.Parsed.File.ContentHash)
	}
}

func TestParserFileURLRequiresHTTPAndRecordsSourceURL(t *testing.T) {
	parser := NewOpenAIChatParser(1024)
	state := &engine.RequestState{
		CanonicalAPI: engine.CanonicalFileUploadURL,
		Incoming: engine.IncomingRequest{
			Path: "/v1/files/upload/url",
			Body: io.NopCloser(strings.NewReader(`{"url":"https://assets.example/input.png"}`)),
		},
	}

	if err := parser.Parse(context.Background(), state); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if state.Parsed.File == nil || state.Parsed.File.SourceURL != "https://assets.example/input.png" || state.Parsed.File.MIMEType != "image/png" {
		t.Fatalf("file = %#v", state.Parsed.File)
	}

	state = &engine.RequestState{
		CanonicalAPI: engine.CanonicalFileUploadURL,
		Incoming: engine.IncomingRequest{
			Path: "/v1/files/upload/url",
			Body: io.NopCloser(strings.NewReader(`{"url":"file:///tmp/input.png"}`)),
		},
	}
	err := parser.Parse(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeInvalidArgument {
		t.Fatalf("error = %v, want invalid argument", err)
	}
}

func TestParserFileStreamReturnsFeatureNotEnabled(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "image.png")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte("png-bytes")); err != nil {
		t.Fatalf("part.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	state := &engine.RequestState{
		CanonicalAPI: engine.CanonicalFileUploadStream,
		Incoming: engine.IncomingRequest{
			Path:   "/v1/files/upload",
			Header: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
			Body:   io.NopCloser(bytes.NewReader(body.Bytes())),
		},
	}

	err = NewOpenAIChatParser(4096).Parse(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeFeatureNotEnabled {
		t.Fatalf("error = %v, want feature_not_enabled", err)
	}
}

func TestBodyStoreRejectsOversizeBody(t *testing.T) {
	_, err := (BodyStore{MaxBytes: 3}).Read(strings.NewReader("abcd"))
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeInvalidArgument {
		t.Fatalf("error = %v, want invalid argument", err)
	}
}
