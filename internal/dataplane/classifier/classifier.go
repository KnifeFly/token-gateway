package classifier

import (
	"context"
	"net/http"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

const protocolHeader = "X-Gateway-Protocol"

var openAIChatEndpoint = engine.EndpointSpec{
	Method:      http.MethodPost,
	Path:        "/v1/chat/completions",
	Canonical:   engine.CanonicalOpenAIChatCompletions,
	AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolNativeOpenAI},
}

// DefaultClassifier classifies the M1 OpenAI-compatible chat endpoint.
type DefaultClassifier struct{}

func NewDefault() *DefaultClassifier {
	return &DefaultClassifier{}
}

func (c *DefaultClassifier) Classify(_ context.Context, state *engine.RequestState) error {
	if state.Incoming.Method != openAIChatEndpoint.Method || state.Incoming.Path != openAIChatEndpoint.Path {
		return apperr.NotFound("endpoint not found")
	}
	mode := protocolFromHeader(state.Incoming.Header.Get(protocolHeader))
	if mode == "" || mode == engine.ProtocolAuto {
		mode = engine.ProtocolNativeOpenAI
	}
	if mode != engine.ProtocolNativeOpenAI {
		return apperr.InvalidArgument("protocol is not allowed for endpoint")
	}
	state.Endpoint = openAIChatEndpoint
	state.CanonicalAPI = openAIChatEndpoint.Canonical
	return state.SetProtocol(mode)
}

func protocolFromHeader(value string) engine.ProtocolMode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case string(engine.ProtocolAuto):
		return engine.ProtocolAuto
	case string(engine.ProtocolUnified):
		return engine.ProtocolUnified
	case string(engine.ProtocolNativeOpenAI):
		return engine.ProtocolNativeOpenAI
	case string(engine.ProtocolNativeClaude):
		return engine.ProtocolNativeClaude
	case string(engine.ProtocolNativeGemini):
		return engine.ProtocolNativeGemini
	default:
		return engine.ProtocolMode(value)
	}
}
