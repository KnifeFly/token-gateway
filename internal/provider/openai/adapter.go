package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/internal/provider/relay"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const defaultTimeout = 30 * time.Second

// Adapter relays requests to OpenAI-compatible providers.
type Adapter struct {
	client *http.Client
}

// NewAdapter returns an OpenAI-compatible adapter.
func NewAdapter(client *http.Client) *Adapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &Adapter{client: client}
}

// Relay sends one OpenAI-compatible request to the selected provider channel.
func (a *Adapter) Relay(ctx context.Context, channel relay.ChannelConfig, request relay.Request) (*relay.Response, error) {
	if strings.HasPrefix(channel.BaseURL, "mock://") {
		return mockResponse(channel, request)
	}
	endpoint, err := endpointURL(channel.BaseURL, request.CanonicalAPI)
	if err != nil {
		return nil, err
	}
	return a.doJSON(ctx, channel, endpoint, request)
}

func (a *Adapter) doJSON(ctx context.Context, channel relay.ChannelConfig, endpoint string, request relay.Request) (*relay.Response, error) {
	timeout := channel.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := rewriteModel(request.RawBody, request.UpstreamModel)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if channel.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+channel.APIKey)
	}
	if request.RequestID != "" {
		httpReq.Header.Set("X-Request-ID", request.RequestID)
	}
	if request.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}
	res, err := a.client.Do(httpReq)
	if err != nil {
		return nil, &relay.ProviderError{
			StatusCode: http.StatusBadGateway,
			Code:       "provider_unavailable",
			Message:    "provider request failed",
			Retryable:  true,
		}
	}
	if request.Stream {
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			defer res.Body.Close()
			content, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
			code, retryable := relay.ClassifyStatus(res.StatusCode)
			return nil, &relay.ProviderError{
				StatusCode: res.StatusCode,
				Code:       code,
				Message:    fmt.Sprintf("provider returned status %d: %s", res.StatusCode, string(content)),
				Retryable:  retryable,
			}
		}
		return &relay.Response{
			StatusCode: res.StatusCode,
			Header:     safeStreamHeaders(res.Header),
			Stream:     newHTTPStream(res.Body),
		}, nil
	}
	defer res.Body.Close()
	content, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, &relay.ProviderError{
			StatusCode: http.StatusBadGateway,
			Code:       "provider_error",
			Message:    "provider response could not be read",
			Retryable:  true,
		}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		code, retryable := relay.ClassifyStatus(res.StatusCode)
		return nil, &relay.ProviderError{
			StatusCode: res.StatusCode,
			Code:       code,
			Message:    fmt.Sprintf("provider returned status %d", res.StatusCode),
			Retryable:  retryable,
		}
	}
	return &relay.Response{
		StatusCode: res.StatusCode,
		Header:     safeHeaders(res.Header),
		Body:       content,
		Usage:      parseUsage(content),
	}, nil
}

func rewriteModel(body []byte, upstreamModel string) ([]byte, error) {
	if upstreamModel == "" {
		return append([]byte(nil), body...), nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &relay.ProviderError{
			StatusCode: http.StatusBadRequest,
			Code:       "provider_request_invalid",
			Message:    "provider request could not be encoded",
			Retryable:  false,
		}
	}
	payload["model"] = upstreamModel
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, &relay.ProviderError{
			StatusCode: http.StatusBadRequest,
			Code:       "provider_request_invalid",
			Message:    "provider request could not be encoded",
			Retryable:  false,
		}
	}
	return encoded, nil
}

func chatCompletionsURL(base string) (string, error) {
	return appendPath(base, "/v1/chat/completions")
}

func endpointURL(base string, canonical string) (string, error) {
	switch canonical {
	case "openai.chat_completions":
		return appendPath(base, "/v1/chat/completions")
	case "openai.responses":
		return appendPath(base, "/v1/responses")
	case "openai.embeddings":
		return appendPath(base, "/v1/embeddings")
	default:
		return "", &relay.ProviderError{
			StatusCode: http.StatusBadRequest,
			Code:       "provider_request_invalid",
			Message:    "unsupported openai-compatible request",
			Retryable:  false,
		}
	}
}

func appendPath(base string, suffix string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", &relay.ProviderError{
			StatusCode: http.StatusBadGateway,
			Code:       "provider_config_invalid",
			Message:    "provider base url is invalid",
			Retryable:  false,
		}
	}
	if strings.HasSuffix(parsed.Path, suffix) {
		return parsed.String(), nil
	}
	if strings.HasSuffix(parsed.Path, "/v1") && strings.HasPrefix(suffix, "/v1/") {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + strings.TrimPrefix(suffix, "/v1")
	} else {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + suffix
	}
	return parsed.String(), nil
}

func mockResponse(channel relay.ChannelConfig, request relay.Request) (*relay.Response, error) {
	now := time.Now().Unix()
	usage := tokenusage.Actual{
		InputTokens:  int64(len(request.RawBody) / 4),
		OutputTokens: 5,
	}
	if usage.InputTokens == 0 {
		usage.InputTokens = 1
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	if request.Stream {
		chunks := openAIChatStreamChunks(now)
		if request.CanonicalAPI == "openai.responses" {
			chunks = openAIResponseStreamChunks(now)
		}
		return &relay.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Stream:     &relay.StaticStream{Chunks: chunks, Actual: usage},
			Usage:      usage,
		}, nil
	}
	if request.CanonicalAPI == "openai.embeddings" {
		body, _ := json.Marshal(map[string]any{
			"object": "list",
			"model":  request.PublicModel,
			"data": []map[string]any{{
				"object":    "embedding",
				"index":     0,
				"embedding": []float64{0.01, 0.02, 0.03},
			}},
			"usage": map[string]int64{
				"prompt_tokens": usage.InputTokens,
				"total_tokens":  usage.InputTokens,
			},
		})
		return &relay.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       append(body, '\n'),
			Usage:      usage,
		}, nil
	}
	if request.CanonicalAPI == "openai.responses" {
		body, _ := json.Marshal(map[string]any{
			"id":      fmt.Sprintf("resp-mock-%d", now),
			"object":  "response",
			"created": now,
			"model":   request.PublicModel,
			"output": []map[string]any{{
				"type": "message",
				"content": []map[string]string{{
					"type": "output_text",
					"text": "M3 mock response",
				}},
			}},
			"usage": map[string]int64{
				"input_tokens":  usage.InputTokens,
				"output_tokens": usage.OutputTokens,
				"total_tokens":  usage.TotalTokens,
			},
		})
		return &relay.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       append(body, '\n'),
			Usage:      usage,
		}, nil
	}
	body, _ := json.Marshal(map[string]any{
		"id":      fmt.Sprintf("chatcmpl-mock-%d", now),
		"object":  "chat.completion",
		"created": now,
		"model":   request.PublicModel,
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]string{
				"role":    "assistant",
				"content": "M1 mock response",
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]int64{
			"prompt_tokens":     usage.InputTokens,
			"completion_tokens": usage.OutputTokens,
			"total_tokens":      usage.TotalTokens,
		},
	})
	return &relay.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       append(body, '\n'),
		Usage:      usage,
	}, nil
}

func openAIChatStreamChunks(now int64) [][]byte {
	return [][]byte{
		[]byte(fmt.Sprintf("data: {\"id\":\"chatcmpl-mock-%d\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"M3\"}}]}\n\n", now)),
		[]byte("data: [DONE]\n\n"),
	}
}

func openAIResponseStreamChunks(now int64) [][]byte {
	return [][]byte{
		[]byte(fmt.Sprintf("event: response.output_text.delta\ndata: {\"id\":\"resp-mock-%d\",\"type\":\"response.output_text.delta\",\"delta\":\"M3\"}\n\n", now)),
		[]byte("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"),
	}
}

func safeHeaders(header http.Header) http.Header {
	out := http.Header{}
	if contentType := header.Get("Content-Type"); contentType != "" {
		out.Set("Content-Type", contentType)
	} else {
		out.Set("Content-Type", "application/json")
	}
	return out
}

func safeStreamHeaders(header http.Header) http.Header {
	out := safeHeaders(header)
	out.Set("Content-Type", "text/event-stream")
	out.Set("Cache-Control", "no-cache")
	return out
}

func parseUsage(body []byte) tokenusage.Actual {
	var payload struct {
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			InputTokens      int64 `json:"input_tokens"`
			OutputTokens     int64 `json:"output_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return tokenusage.Actual{}
	}
	inputTokens := payload.Usage.PromptTokens
	if inputTokens == 0 {
		inputTokens = payload.Usage.InputTokens
	}
	outputTokens := payload.Usage.CompletionTokens
	if outputTokens == 0 {
		outputTokens = payload.Usage.OutputTokens
	}
	totalTokens := payload.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = inputTokens + outputTokens
	}
	return tokenusage.Actual{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
	}
}

type httpStream struct {
	body   io.ReadCloser
	buf    []byte
	actual tokenusage.Actual
}

func newHTTPStream(body io.ReadCloser) *httpStream {
	return &httpStream{body: body}
}

func (s *httpStream) Recv(ctx context.Context) ([]byte, error) {
	if len(s.buf) == 0 {
		s.buf = make([]byte, 4096)
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	n, err := s.body.Read(s.buf)
	if n > 0 {
		chunk := append([]byte(nil), s.buf[:n]...)
		s.observeUsage(chunk)
		return chunk, nil
	}
	return nil, err
}

func (s *httpStream) Usage() tokenusage.Actual {
	return s.actual
}

func (s *httpStream) Close() error {
	if s == nil || s.body == nil {
		return nil
	}
	return s.body.Close()
}

func (s *httpStream) observeUsage(chunk []byte) {
	for _, line := range strings.Split(string(chunk), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		usage := parseUsage([]byte(data))
		if usage.TotalTokens > 0 {
			s.actual = usage
		}
	}
}
