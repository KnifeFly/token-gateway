package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const defaultMaxBodyBytes int64 = 4 << 20

// OpenAIChatParser parses M1 OpenAI-compatible chat completion requests.
type OpenAIChatParser struct {
	bodyStore BodyStore
}

func NewOpenAIChatParser(maxBodyBytes int64) *OpenAIChatParser {
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}
	return &OpenAIChatParser{bodyStore: BodyStore{MaxBytes: maxBodyBytes}}
}

func (p *OpenAIChatParser) Parse(_ context.Context, state *engine.RequestState) error {
	if state.CanonicalAPI != engine.CanonicalOpenAIChatCompletions {
		return apperr.InvalidArgument("unsupported request parser")
	}
	body, err := p.bodyStore.Read(state.Incoming.Body)
	if err != nil {
		return err
	}
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
	if req.Stream {
		return apperr.InvalidArgument("streaming is not supported in M1")
	}
	messages := make([]engine.OpenAIChatMessage, 0, len(req.Messages))
	for i, message := range req.Messages {
		if message.Role == "" {
			return apperr.InvalidArgument(fmt.Sprintf("messages[%d].role is required", i))
		}
		if len(message.Content) == 0 {
			return apperr.InvalidArgument(fmt.Sprintf("messages[%d].content is required", i))
		}
		var content any
		if err := json.Unmarshal(message.Content, &content); err != nil {
			return apperr.InvalidArgument(fmt.Sprintf("messages[%d].content is invalid", i), apperr.WithCause(err))
		}
		messages = append(messages, engine.OpenAIChatMessage{Role: message.Role, Content: content})
	}
	state.RequestedModel = req.Model
	state.Stream = req.Stream
	state.Parsed = engine.ParsedRequest{
		RawBody: body,
		OpenAIChat: &engine.OpenAIChatRequest{
			Model:    req.Model,
			Messages: messages,
			Stream:   req.Stream,
		},
	}
	state.EstimatedUsage = tokenusage.EstimateFromBytes(body)
	return nil
}

// BodyStore reads a bounded request body for parsers that need to inspect it once.
type BodyStore struct {
	MaxBytes int64
}

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
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}
