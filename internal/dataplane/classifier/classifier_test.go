package classifier

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	dpsnapshot "github.com/KnifeFly/token-gateway/internal/dataplane/snapshot"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func TestDefaultClassifierOpenAIChat(t *testing.T) {
	state := &engine.RequestState{Incoming: engine.IncomingRequest{
		Method: http.MethodPost,
		Path:   "/v1/chat/completions",
		Header: http.Header{},
		Body:   io.NopCloser(strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)),
	}}

	err := NewDefault().Classify(context.Background(), state)
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if state.CanonicalAPI != engine.CanonicalOpenAIChatCompletions {
		t.Fatalf("CanonicalAPI = %q", state.CanonicalAPI)
	}
	if state.ProtocolMode != engine.ProtocolNativeOpenAI {
		t.Fatalf("ProtocolMode = %q", state.ProtocolMode)
	}
}

func TestDefaultClassifierRejectsConflict(t *testing.T) {
	state := &engine.RequestState{Incoming: engine.IncomingRequest{
		Method: http.MethodPost,
		Path:   "/v1/chat/completions",
		Header: http.Header{"X-Gateway-Protocol": []string{"native_claude"}},
	}}

	err := NewDefault().Classify(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeInvalidArgument {
		t.Fatalf("error = %v, want invalid argument", err)
	}
}

func TestDefaultClassifierM3Endpoints(t *testing.T) {
	tests := []struct {
		path      string
		canonical engine.CanonicalAPI
		mode      engine.ProtocolMode
	}{
		{path: "/v1/responses", canonical: engine.CanonicalOpenAIResponses, mode: engine.ProtocolNativeOpenAI},
		{path: "/v1/embeddings", canonical: engine.CanonicalOpenAIEmbeddings, mode: engine.ProtocolNativeOpenAI},
		{path: "/v1/messages", canonical: engine.CanonicalClaudeMessages, mode: engine.ProtocolNativeClaude},
		{path: "/v1beta/models/gemini-2.5-flash:generateContent", canonical: engine.CanonicalGeminiGenerateContent, mode: engine.ProtocolNativeGemini},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			state := &engine.RequestState{Incoming: engine.IncomingRequest{
				Method: http.MethodPost,
				Path:   tt.path,
				Header: http.Header{},
			}}
			err := NewDefault().Classify(context.Background(), state)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			if state.CanonicalAPI != tt.canonical || state.ProtocolMode != tt.mode {
				t.Fatalf("canonical = %q mode = %q", state.CanonicalAPI, state.ProtocolMode)
			}
		})
	}
}

func TestDefaultClassifierM4Endpoints(t *testing.T) {
	tests := []struct {
		method    string
		path      string
		canonical engine.CanonicalAPI
		body      string
	}{
		{method: http.MethodPost, path: "/v1/videos/generations", canonical: engine.CanonicalVideoGeneration},
		{method: http.MethodPost, path: "/v1/images/generations", canonical: engine.CanonicalImageGeneration, body: `{"model":"image-model","prompt":"cat","callback_url":"https://example.com/cb"}`},
		{method: http.MethodPost, path: "/v1/files/upload/base64", canonical: engine.CanonicalFileUploadBase64},
		{method: http.MethodGet, path: "/v1/files/quota", canonical: engine.CanonicalFileQuota},
		{method: http.MethodGet, path: "/v1/tasks/task_123", canonical: engine.CanonicalTaskGet},
		{method: http.MethodPost, path: "/v1/tasks/task_123/cancel", canonical: engine.CanonicalTaskCancel},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			state := &engine.RequestState{Incoming: engine.IncomingRequest{
				Method: tt.method,
				Path:   tt.path,
				Header: http.Header{},
			}}
			if tt.body != "" {
				state.Incoming.Body = io.NopCloser(strings.NewReader(tt.body))
			}
			err := NewDefault().Classify(context.Background(), state)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			if state.CanonicalAPI != tt.canonical || state.ProtocolMode != engine.ProtocolUnified {
				t.Fatalf("canonical = %q mode = %q", state.CanonicalAPI, state.ProtocolMode)
			}
		})
	}
}

func TestDefaultClassifierUsesModelRegistryHint(t *testing.T) {
	state := &engine.RequestState{
		Snapshot: classifierSnapshot(t, []cpsnapshot.ModelRuntime{{
			PublicModel: "gpt-image-1",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Capability:  "image",
			Enabled:     true,
		}}),
		Incoming: engine.IncomingRequest{
			Method: http.MethodPost,
			Path:   "/v1/images/generations",
			Header: http.Header{},
			Body:   io.NopCloser(strings.NewReader(`{"model":"gpt-image-1","prompt":"cat","size":"1024x1024"}`)),
		},
	}
	if err := NewDefault().Classify(context.Background(), state); err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if state.ProtocolMode != engine.ProtocolNativeOpenAI {
		t.Fatalf("ProtocolMode = %q", state.ProtocolMode)
	}
	body, err := io.ReadAll(state.Incoming.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !strings.Contains(string(body), "gpt-image-1") {
		t.Fatalf("body was not restored: %q", string(body))
	}
}

func TestDefaultClassifierInfersUnifiedFromBodySchema(t *testing.T) {
	state := &engine.RequestState{Incoming: engine.IncomingRequest{
		Method: http.MethodPost,
		Path:   "/v1/images/generations",
		Header: http.Header{},
		Body:   io.NopCloser(strings.NewReader(`{"model":"unknown","prompt":"cat","model_params":{"seed":1}}`)),
	}}
	if err := NewDefault().Classify(context.Background(), state); err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if state.ProtocolMode != engine.ProtocolUnified {
		t.Fatalf("ProtocolMode = %q", state.ProtocolMode)
	}
}

func TestDefaultClassifierInfersNativeOpenAIAudioFromBodySchema(t *testing.T) {
	state := &engine.RequestState{Incoming: engine.IncomingRequest{
		Method: http.MethodPost,
		Path:   "/v1/audio/speech",
		Header: http.Header{},
		Body:   io.NopCloser(strings.NewReader(`{"model":"tts-1","input":"hi","voice":"alloy","response_format":"mp3"}`)),
	}}
	if err := NewDefault().Classify(context.Background(), state); err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if state.ProtocolMode != engine.ProtocolNativeOpenAI {
		t.Fatalf("ProtocolMode = %q", state.ProtocolMode)
	}
}

func TestDefaultClassifierInfersNativeOpenAIFromMultipart(t *testing.T) {
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
	state := &engine.RequestState{Incoming: engine.IncomingRequest{
		Method: http.MethodPost,
		Path:   "/v1/audio/transcriptions",
		Header: http.Header{"Content-Type": []string{writer.FormDataContentType()}},
		Body:   io.NopCloser(bytes.NewReader(body.Bytes())),
	}}
	if err := NewDefault().Classify(context.Background(), state); err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if state.ProtocolMode != engine.ProtocolNativeOpenAI {
		t.Fatalf("ProtocolMode = %q", state.ProtocolMode)
	}
}

func TestDefaultClassifierReturnsAmbiguousProtocol(t *testing.T) {
	state := &engine.RequestState{Incoming: engine.IncomingRequest{
		Method: http.MethodPost,
		Path:   "/v1/images/generations",
		Header: http.Header{},
		Body:   io.NopCloser(strings.NewReader(`{"model":"unknown","prompt":"cat"}`)),
	}}
	err := NewDefault().Classify(context.Background(), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeAmbiguousProtocol {
		t.Fatalf("error = %v, want ambiguous_protocol", err)
	}
}

func classifierSnapshot(t *testing.T, models []cpsnapshot.ModelRuntime) *dpsnapshot.IndexedSnapshot {
	t.Helper()
	indexed, err := dpsnapshot.Build(cpsnapshot.RuntimeSnapshot{
		Version: "test",
		Models:  models,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return indexed
}
