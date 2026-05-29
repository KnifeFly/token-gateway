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

func NewAdapter(client *http.Client) *Adapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &Adapter{client: client}
}

func (a *Adapter) ChatCompletions(ctx context.Context, channel relay.ChannelConfig, request relay.ChatCompletionRequest) (*relay.Response, error) {
	if strings.HasPrefix(channel.BaseURL, "mock://") {
		return mockChatCompletion(channel, request)
	}
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
	endpoint, err := chatCompletionsURL(channel.BaseURL)
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
	res, err := a.client.Do(httpReq)
	if err != nil {
		return nil, &relay.ProviderError{
			StatusCode: http.StatusBadGateway,
			Code:       "provider_unavailable",
			Message:    "provider request failed",
			Retryable:  true,
		}
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
	parsed, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", &relay.ProviderError{
			StatusCode: http.StatusBadGateway,
			Code:       "provider_config_invalid",
			Message:    "provider base url is invalid",
			Retryable:  false,
		}
	}
	if strings.HasSuffix(parsed.Path, "/v1/chat/completions") {
		return parsed.String(), nil
	}
	if strings.HasSuffix(parsed.Path, "/v1") {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
		return parsed.String(), nil
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v1/chat/completions"
	return parsed.String(), nil
}

func mockChatCompletion(channel relay.ChannelConfig, request relay.ChatCompletionRequest) (*relay.Response, error) {
	now := time.Now().Unix()
	usage := tokenusage.Actual{
		InputTokens:  int64(len(request.RawBody) / 4),
		OutputTokens: 5,
	}
	if usage.InputTokens == 0 {
		usage.InputTokens = 1
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens
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

func safeHeaders(header http.Header) http.Header {
	out := http.Header{}
	if contentType := header.Get("Content-Type"); contentType != "" {
		out.Set("Content-Type", contentType)
	} else {
		out.Set("Content-Type", "application/json")
	}
	return out
}

func parseUsage(body []byte) tokenusage.Actual {
	var payload struct {
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return tokenusage.Actual{}
	}
	return tokenusage.Actual{
		InputTokens:  payload.Usage.PromptTokens,
		OutputTokens: payload.Usage.CompletionTokens,
		TotalTokens:  payload.Usage.TotalTokens,
	}
}
