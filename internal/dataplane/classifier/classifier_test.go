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
