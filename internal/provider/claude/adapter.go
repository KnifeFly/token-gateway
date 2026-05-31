package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/internal/provider/relay"
	"github.com/KnifeFly/token-gateway/pkg/egressguard"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const defaultTimeout = 30 * time.Second

// Adapter relays Claude Messages requests.
type Adapter struct {
	client *http.Client
	egress *egressguard.Guard
}

// NewAdapter returns a Claude adapter.
func NewAdapter(client *http.Client) *Adapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &Adapter{client: client}
}

// WithEgressGuard validates provider URLs before outbound HTTP calls.
func (a *Adapter) WithEgressGuard(guard *egressguard.Guard) *Adapter {
	if a != nil {
		a.egress = guard
	}
	return a
}

// Relay sends one Claude Messages request.
func (a *Adapter) Relay(ctx context.Context, channel relay.ChannelConfig, request relay.Request) (*relay.Response, error) {
	if request.CanonicalAPI != "claude.messages" {
		return nil, &relay.ProviderError{StatusCode: http.StatusBadRequest, Code: "provider_request_invalid", Message: "unsupported claude request"}
	}
	if strings.HasPrefix(channel.BaseURL, "mock://") {
		return mockMessage(request), nil
	}

	endpoint, err := endpointURL(channel.BaseURL)
	if err != nil {
		return nil, err
	}
	if a.egress != nil {
		if err := a.egress.ValidateURL(ctx, endpoint); err != nil {
			return nil, &relay.ProviderError{StatusCode: http.StatusBadGateway, Code: "provider_config_invalid", Message: "provider egress url is not allowed"}
		}
	}

	timeout := channel.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)

	upstreamModel := request.UpstreamModel
	if upstreamModel == "" {
		upstreamModel = channel.UpstreamModel
	}
	relayBody, err := rewriteJSONModel(request.RawBody, upstreamModel)
	if err != nil {
		cancel()
		return nil, err
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(relayBody))
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if request.Stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	req.Header.Set("Anthropic-Version", claudeHeader(request.Headers, "Anthropic-Version", "2023-06-01"))
	if beta := claudeHeader(request.Headers, "Anthropic-Beta", ""); beta != "" {
		req.Header.Set("Anthropic-Beta", beta)
	}
	if channel.APIKey != "" {
		req.Header.Set("X-API-Key", channel.APIKey)
	}

	res, err := a.client.Do(req)
	if err != nil {
		cancel()
		return nil, relay.ErrorFromRequestFailure(err)
	}

	if request.Stream && res.StatusCode >= 200 && res.StatusCode < 300 {
		return &relay.Response{StatusCode: res.StatusCode, Header: safeStreamHeaders(res.Header), Stream: &httpStream{body: res.Body, cancel: cancel}}, nil
	}
	defer cancel()
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, &relay.ProviderError{StatusCode: http.StatusBadGateway, Code: "provider_error", Message: "provider response could not be read", Retryable: true}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, relay.ErrorFromStatus(res.StatusCode, body)
	}
	return &relay.Response{StatusCode: res.StatusCode, Header: safeHeaders(res.Header), Body: body, Usage: parseUsage(body)}, nil
}

func rewriteJSONModel(body []byte, upstreamModel string) ([]byte, error) {
	if upstreamModel == "" {
		return append([]byte(nil), body...), nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &relay.ProviderError{StatusCode: http.StatusBadRequest, Code: "provider_request_invalid", Message: "provider request could not be encoded"}
	}
	payload["model"] = upstreamModel
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, &relay.ProviderError{StatusCode: http.StatusBadRequest, Code: "provider_request_invalid", Message: "provider request could not be encoded"}
	}
	return encoded, nil
}

func claudeHeader(headers http.Header, key string, fallback string) string {
	if headers == nil {
		return fallback
	}
	value := strings.TrimSpace(headers.Get(key))
	if value == "" {
		return fallback
	}
	return value
}

func endpointURL(base string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", &relay.ProviderError{StatusCode: http.StatusBadGateway, Code: "provider_config_invalid", Message: "provider base url is invalid"}
	}
	if strings.HasSuffix(parsed.Path, "/v1/messages") {
		return parsed.String(), nil
	}
	if strings.HasSuffix(parsed.Path, "/v1") {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/messages"
	} else {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v1/messages"
	}
	return parsed.String(), nil
}

func mockMessage(request relay.Request) *relay.Response {
	usage := tokenusage.EstimateFromBytes(request.RawBody)
	actual := tokenusage.Actual{InputTokens: usage.InputTokens, OutputTokens: 8, TotalTokens: usage.InputTokens + 8}
	if request.Stream {
		return &relay.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Stream: &relay.StaticStream{
				Chunks: [][]byte{
					[]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"M3 Claude mock response\"}}\n\n"),
					[]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"),
				},
				Actual: actual,
			},
			Usage: actual,
		}
	}
	body, _ := json.Marshal(map[string]any{
		"id":    "msg_mock",
		"type":  "message",
		"role":  "assistant",
		"model": request.PublicModel,
		"content": []map[string]string{{
			"type": "text",
			"text": "M3 Claude mock response",
		}},
		"usage": map[string]int64{"input_tokens": actual.InputTokens, "output_tokens": actual.OutputTokens},
	})
	return &relay.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: append(body, '\n'), Usage: actual}
}

func parseUsage(body []byte) tokenusage.Actual {
	var payload struct {
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
		Message struct {
			Usage struct {
				InputTokens              int64 `json:"input_tokens"`
				OutputTokens             int64 `json:"output_tokens"`
				CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
				CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			} `json:"usage"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return tokenusage.Actual{}
	}
	inputTokens := payload.Usage.InputTokens
	if inputTokens == 0 {
		inputTokens = payload.Message.Usage.InputTokens
	}
	outputTokens := payload.Usage.OutputTokens
	if outputTokens == 0 {
		outputTokens = payload.Message.Usage.OutputTokens
	}
	cacheCreation := payload.Usage.CacheCreationInputTokens
	if cacheCreation == 0 {
		cacheCreation = payload.Message.Usage.CacheCreationInputTokens
	}
	cacheRead := payload.Usage.CacheReadInputTokens
	if cacheRead == 0 {
		cacheRead = payload.Message.Usage.CacheReadInputTokens
	}
	return tokenusage.Actual{
		InputTokens:              inputTokens,
		OutputTokens:             outputTokens,
		TotalTokens:              inputTokens + outputTokens,
		CachedInputTokens:        cacheCreation + cacheRead,
		CacheCreationInputTokens: cacheCreation,
		CacheReadInputTokens:     cacheRead,
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

type httpStream struct {
	body   io.ReadCloser
	cancel context.CancelFunc
	actual tokenusage.Actual
}

func (s *httpStream) Recv(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	buf := make([]byte, 32*1024)
	n, err := s.body.Read(buf)
	if n > 0 {
		chunk := append([]byte(nil), buf[:n]...)
		s.observeUsage(chunk)
		return chunk, nil
	}
	return nil, err
}

func (s *httpStream) Usage() tokenusage.Actual {
	return s.actual
}

func (s *httpStream) Close() error {
	err := s.body.Close()
	if s.cancel != nil {
		s.cancel()
	}
	return err
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
			s.actual = tokenusage.Merge(s.actual, usage)
		}
	}
}
