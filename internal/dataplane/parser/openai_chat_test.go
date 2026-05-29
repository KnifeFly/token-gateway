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

func TestBodyStoreRejectsOversizeBody(t *testing.T) {
	_, err := (BodyStore{MaxBytes: 3}).Read(strings.NewReader("abcd"))
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeInvalidArgument {
		t.Fatalf("error = %v, want invalid argument", err)
	}
}
