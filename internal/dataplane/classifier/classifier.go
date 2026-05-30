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

var endpoints = []engine.EndpointSpec{
	openAIChatEndpoint,
	{Method: http.MethodPost, Path: "/v1/responses", Canonical: engine.CanonicalOpenAIResponses, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolNativeOpenAI}},
	{Method: http.MethodPost, Path: "/v1/embeddings", Canonical: engine.CanonicalOpenAIEmbeddings, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolNativeOpenAI}},
	{Method: http.MethodPost, Path: "/v1/moderations", Canonical: engine.CanonicalOpenAIModerations, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolNativeOpenAI}},
	{Method: http.MethodPost, Path: "/v1/messages", Canonical: engine.CanonicalClaudeMessages, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolNativeClaude}},
	{Method: http.MethodPost, Path: "/v1/images/generations", Canonical: engine.CanonicalImageGeneration, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolUnified}},
	{Method: http.MethodPost, Path: "/v1/images/edits", Canonical: engine.CanonicalImageEdit, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolUnified}},
	{Method: http.MethodPost, Path: "/v1/videos/generations", Canonical: engine.CanonicalVideoGeneration, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolUnified}},
	{Method: http.MethodPost, Path: "/v1/audio/speech", Canonical: engine.CanonicalAudioSpeech, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolUnified}},
	{Method: http.MethodPost, Path: "/v1/audio/transcriptions", Canonical: engine.CanonicalAudioTranscription, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolUnified}},
	{Method: http.MethodPost, Path: "/v1/music/generations", Canonical: engine.CanonicalMusicGeneration, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolUnified}},
	{Method: http.MethodPost, Path: "/v1/files/upload/base64", Canonical: engine.CanonicalFileUploadBase64, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolUnified}},
	{Method: http.MethodPost, Path: "/v1/files/upload/url", Canonical: engine.CanonicalFileUploadURL, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolUnified}},
	{Method: http.MethodPost, Path: "/v1/files/upload/stream", Canonical: engine.CanonicalFileUploadStream, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolUnified}},
	{Method: http.MethodGet, Path: "/v1/files/quota", Canonical: engine.CanonicalFileQuota, AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolUnified}},
}

// DefaultClassifier classifies M1/M3 data-plane endpoints.
type DefaultClassifier struct{}

// NewDefault returns the default API classifier.
func NewDefault() *DefaultClassifier {
	return &DefaultClassifier{}
}

// Classify pins the endpoint and protocol mode for one request.
func (c *DefaultClassifier) Classify(_ context.Context, state *engine.RequestState) error {
	endpoint, ok := matchEndpoint(state.Incoming.Method, state.Incoming.Path)
	if !ok {
		return apperr.NotFound("endpoint not found")
	}
	mode := protocolFromHeader(state.Incoming.Header.Get(protocolHeader))
	if mode == "" || mode == engine.ProtocolAuto {
		mode = defaultProtocol(endpoint.Canonical)
	}
	if !allows(endpoint, mode) {
		return apperr.InvalidArgument("protocol is not allowed for endpoint")
	}
	state.Endpoint = endpoint
	state.CanonicalAPI = endpoint.Canonical
	return state.SetProtocol(mode)
}

func matchEndpoint(method, path string) (engine.EndpointSpec, bool) {
	if method == http.MethodGet && strings.HasPrefix(path, "/v1/tasks/") && !strings.HasSuffix(path, "/cancel") {
		return engine.EndpointSpec{
			Method:      http.MethodGet,
			Path:        "/v1/tasks/{task_id}",
			Canonical:   engine.CanonicalTaskGet,
			AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolUnified},
		}, true
	}
	if method == http.MethodPost && strings.HasPrefix(path, "/v1/tasks/") && strings.HasSuffix(path, "/cancel") {
		return engine.EndpointSpec{
			Method:      http.MethodPost,
			Path:        "/v1/tasks/{task_id}/cancel",
			Canonical:   engine.CanonicalTaskCancel,
			AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolUnified},
		}, true
	}
	if method == http.MethodPost && strings.HasPrefix(path, "/v1beta/models/") &&
		(strings.HasSuffix(path, ":generateContent") || strings.HasSuffix(path, ":streamGenerateContent")) {
		return engine.EndpointSpec{
			Method:      http.MethodPost,
			Path:        "/v1beta/models/{model}:generateContent",
			Canonical:   engine.CanonicalGeminiGenerateContent,
			AllowedMode: []engine.ProtocolMode{engine.ProtocolAuto, engine.ProtocolNativeGemini},
		}, true
	}
	for _, endpoint := range endpoints {
		if endpoint.Method == method && endpoint.Path == path {
			return endpoint, true
		}
	}
	return engine.EndpointSpec{}, false
}

func defaultProtocol(api engine.CanonicalAPI) engine.ProtocolMode {
	switch api {
	case engine.CanonicalClaudeMessages:
		return engine.ProtocolNativeClaude
	case engine.CanonicalGeminiGenerateContent:
		return engine.ProtocolNativeGemini
	case engine.CanonicalImageGeneration,
		engine.CanonicalImageEdit,
		engine.CanonicalVideoGeneration,
		engine.CanonicalAudioSpeech,
		engine.CanonicalAudioTranscription,
		engine.CanonicalMusicGeneration,
		engine.CanonicalTaskGet,
		engine.CanonicalTaskCancel,
		engine.CanonicalFileUploadBase64,
		engine.CanonicalFileUploadURL,
		engine.CanonicalFileUploadStream,
		engine.CanonicalFileQuota:
		return engine.ProtocolUnified
	default:
		return engine.ProtocolNativeOpenAI
	}
}

func allows(endpoint engine.EndpointSpec, mode engine.ProtocolMode) bool {
	for _, allowed := range endpoint.AllowedMode {
		if allowed == mode {
			return true
		}
	}
	return false
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
