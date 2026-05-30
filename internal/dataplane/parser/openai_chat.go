package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const defaultMaxBodyBytes int64 = 4 << 20

// Parser parses M1/M3 request bodies into the shared RequestState.
type Parser struct {
	bodyStore BodyStore
}

// NewOpenAIChatParser returns the shared JSON body parser.
func NewOpenAIChatParser(maxBodyBytes int64) *Parser {
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}
	return &Parser{bodyStore: BodyStore{MaxBytes: maxBodyBytes}}
}

// Parse reads the request body and extracts normalized model and stream fields.
func (p *Parser) Parse(_ context.Context, state *engine.RequestState) error {
	switch state.CanonicalAPI {
	case engine.CanonicalTaskGet, engine.CanonicalTaskCancel:
		return p.parseTaskOperation(state)
	case engine.CanonicalFileQuota:
		return p.parseFileQuota(state)
	}
	body, err := p.bodyStore.Read(state.Incoming.Body)
	if err != nil {
		return err
	}
	switch state.CanonicalAPI {
	case engine.CanonicalOpenAIChatCompletions:
		return p.parseOpenAIChat(state, body)
	case engine.CanonicalOpenAIResponses:
		return p.parseOpenAIResponse(state, body)
	case engine.CanonicalOpenAIEmbeddings:
		return p.parseEmbedding(state, body)
	case engine.CanonicalOpenAIModerations:
		return p.parseModeration(state, body)
	case engine.CanonicalClaudeMessages:
		return p.parseClaudeMessage(state, body)
	case engine.CanonicalGeminiGenerateContent:
		return p.parseGemini(state, body)
	case engine.CanonicalImageGeneration,
		engine.CanonicalImageEdit,
		engine.CanonicalAudioSpeech:
		if state.ProtocolMode == engine.ProtocolNativeOpenAI {
			return p.parseNativeOpenAIMedia(state, body)
		}
		return p.parseUnifiedMedia(state, body)
	case engine.CanonicalVideoGeneration,
		engine.CanonicalMusicGeneration:
		return p.parseUnifiedMedia(state, body)
	case engine.CanonicalAudioTranscription:
		if state.ProtocolMode == engine.ProtocolNativeOpenAI {
			return p.parseNativeOpenAIMedia(state, body)
		}
		return p.parseAudioTranscription(state, body)
	case engine.CanonicalFileUploadBase64:
		return p.parseFileBase64(state, body)
	case engine.CanonicalFileUploadURL:
		return p.parseFileURL(state, body)
	case engine.CanonicalFileUploadStream:
		return p.parseFileStream(state, body)
	default:
		return apperr.InvalidArgument("unsupported request parser")
	}
}

func (p *Parser) parseOpenAIChat(state *engine.RequestState, body []byte) error {
	var req openAIChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err))
	}
	if req.Model == "" {
		return apperr.InvalidArgument("model is required")
	}
	if len(req.Messages) == 0 {
		return apperr.InvalidArgument("messages is required")
	}
	messages := make([]engine.OpenAIChatMessage, 0, len(req.Messages))
	for i, message := range req.Messages {
		if message.Role == "" {
			return apperr.InvalidArgument(fmt.Sprintf("messages[%d].role is required", i))
		}
		if len(message.Content) == 0 && len(message.ToolCalls) == 0 && len(message.FunctionCall) == 0 {
			return apperr.InvalidArgument(fmt.Sprintf("messages[%d].content or tool_calls is required", i))
		}
		var content any
		if len(message.Content) > 0 {
			if err := json.Unmarshal(message.Content, &content); err != nil {
				return apperr.InvalidArgument(fmt.Sprintf("messages[%d].content is invalid", i), apperr.WithCause(err))
			}
		}
		messages = append(messages, engine.OpenAIChatMessage{Role: message.Role, Content: content})
	}
	state.RequestedModel = req.Model
	state.Stream = req.Stream
	state.Parsed = engine.ParsedRequest{
		RawBody: body,
		Model:   req.Model,
		Stream:  req.Stream,
		OpenAIChat: &engine.OpenAIChatRequest{
			Model:    req.Model,
			Messages: messages,
			Stream:   req.Stream,
		},
	}
	state.EstimatedUsage = tokenusage.EstimateFromBytes(body)
	return nil
}

func (p *Parser) parseOpenAIResponse(state *engine.RequestState, body []byte) error {
	var req modelStreamRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err))
	}
	if req.Model == "" {
		return apperr.InvalidArgument("model is required")
	}
	state.RequestedModel = req.Model
	state.Stream = req.Stream
	state.Parsed = engine.ParsedRequest{
		RawBody: body,
		Model:   req.Model,
		Stream:  req.Stream,
		OpenAIResponse: &engine.OpenAIResponseRequest{
			Model:  req.Model,
			Stream: req.Stream,
		},
	}
	state.EstimatedUsage = tokenusage.EstimateFromBytes(body)
	return nil
}

func (p *Parser) parseEmbedding(state *engine.RequestState, body []byte) error {
	var req modelStreamRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err))
	}
	if req.Model == "" {
		return apperr.InvalidArgument("model is required")
	}
	state.RequestedModel = req.Model
	state.Stream = false
	state.Parsed = engine.ParsedRequest{
		RawBody:   body,
		Model:     req.Model,
		Embedding: &engine.EmbeddingRequest{Model: req.Model},
	}
	state.EstimatedUsage = tokenusage.EstimateFromBytes(body)
	return nil
}

func (p *Parser) parseModeration(state *engine.RequestState, body []byte) error {
	var req moderationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err))
	}
	if req.Model == "" {
		return apperr.InvalidArgument("model is required")
	}
	if len(req.Input) == 0 {
		return apperr.InvalidArgument("input is required")
	}
	state.RequestedModel = req.Model
	state.Stream = false
	state.Parsed = engine.ParsedRequest{
		RawBody:    body,
		Model:      req.Model,
		Moderation: &engine.ModerationRequest{Model: req.Model},
	}
	state.EstimatedUsage = tokenusage.EstimateFromBytes(body)
	return nil
}

func (p *Parser) parseNativeOpenAIMedia(state *engine.RequestState, body []byte) error {
	model, err := nativeOpenAIModelFromBody(body, state.Incoming.Header.Get("Content-Type"))
	if err != nil {
		return err
	}
	if model == "" {
		return apperr.InvalidArgument("model is required")
	}
	state.RequestedModel = model
	state.Stream = false
	state.Async = false
	state.Parsed = engine.ParsedRequest{
		RawBody: body,
		Model:   model,
	}
	state.EstimatedUsage = tokenusage.EstimateFromBytes(body)
	return nil
}

func (p *Parser) parseClaudeMessage(state *engine.RequestState, body []byte) error {
	var req modelStreamRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err))
	}
	if req.Model == "" {
		return apperr.InvalidArgument("model is required")
	}
	state.RequestedModel = req.Model
	state.Stream = req.Stream
	state.Parsed = engine.ParsedRequest{
		RawBody: body,
		Model:   req.Model,
		Stream:  req.Stream,
		ClaudeMessage: &engine.ClaudeMessageRequest{
			Model:  req.Model,
			Stream: req.Stream,
		},
	}
	state.EstimatedUsage = tokenusage.EstimateFromBytes(body)
	return nil
}

func (p *Parser) parseGemini(state *engine.RequestState, body []byte) error {
	var req geminiGenerateContentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err))
	}
	if len(bytes.TrimSpace(req.Contents)) == 0 || bytes.Equal(bytes.TrimSpace(req.Contents), []byte("null")) {
		return apperr.InvalidArgument("contents is required")
	}
	model := geminiModelFromPath(state.Incoming.Path)
	if model == "" {
		return apperr.InvalidArgument("model is required")
	}
	stream := isGeminiStreamPath(state.Incoming.Path)
	state.RequestedModel = model
	state.Stream = stream
	state.Parsed = engine.ParsedRequest{
		RawBody: body,
		Model:   model,
		Stream:  stream,
		Gemini: &engine.GeminiRequest{
			Model:  model,
			Stream: stream,
		},
	}
	state.EstimatedUsage = tokenusage.EstimateFromBytes(body)
	return nil
}

// BodyStore reads a bounded request body for parsers that need to inspect it once.
type BodyStore struct {
	MaxBytes int64
}

// Read reads a bounded request body into memory.
func (s BodyStore) Read(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, apperr.InvalidArgument("request body is required")
	}
	maxBytes := s.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBodyBytes
	}
	limited := io.LimitReader(body, maxBytes+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		return nil, apperr.InvalidArgument("request body could not be read", apperr.WithCause(err))
	}
	if int64(len(content)) > maxBytes {
		return nil, apperr.InvalidArgument("request body is too large")
	}
	if len(content) == 0 {
		return nil, apperr.InvalidArgument("request body is required")
	}
	return content, nil
}

type openAIChatRequest struct {
	Model    string              `json:"model"`
	Messages []openAIChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

type openAIChatMessage struct {
	Role         string          `json:"role"`
	Content      json.RawMessage `json:"content"`
	ToolCalls    json.RawMessage `json:"tool_calls"`
	FunctionCall json.RawMessage `json:"function_call"`
}

type modelStreamRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

type moderationRequest struct {
	Model string          `json:"model"`
	Input json.RawMessage `json:"input"`
}

type geminiGenerateContentRequest struct {
	Contents json.RawMessage `json:"contents"`
}

func nativeOpenAIModelFromBody(body []byte, contentType string) (string, error) {
	mediaType, params, _ := mime.ParseMediaType(contentType)
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return "", apperr.InvalidArgument("multipart boundary is required")
		}
		form, err := multipart.NewReader(bytes.NewReader(body), boundary).ReadForm(defaultMaxBodyBytes)
		if err != nil {
			return "", apperr.InvalidArgument("multipart body is invalid", apperr.WithCause(err))
		}
		defer func() { _ = form.RemoveAll() }()
		return firstFormValue(form, "model"), nil
	}
	var req modelStreamRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err))
	}
	return req.Model, nil
}

func geminiModelFromPath(path string) string {
	const prefix = "/v1beta/models/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	if idx := strings.IndexByte(rest, ':'); idx >= 0 {
		return rest[:idx]
	}
	return ""
}

func isGeminiStreamPath(path string) bool {
	return strings.HasSuffix(path, ":streamGenerateContent")
}
