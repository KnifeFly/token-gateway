package classifier

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func TestDefaultClassifierOpenAIChat(t *testing.T) {
	state := &engine.RequestState{Incoming: engine.IncomingRequest{
		Method: http.MethodPost,
		Path:   "/v1/chat/completions",
		Header: http.Header{},
		Body:   io.NopCloser(nil),
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
	}{
		{method: http.MethodPost, path: "/v1/videos/generations", canonical: engine.CanonicalVideoGeneration},
		{method: http.MethodPost, path: "/v1/images/generations", canonical: engine.CanonicalImageGeneration},
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
