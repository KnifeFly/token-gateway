package gemini

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

// Adapter relays Gemini GenerateContent requests.
type Adapter struct {
	client *http.Client
	egress *egressguard.Guard
}

// NewAdapter returns a Gemini adapter.
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

// Relay sends one Gemini GenerateContent request.
func (a *Adapter) Relay(ctx context.Context, channel relay.ChannelConfig, request relay.Request) (*relay.Response, error) {
	if request.CanonicalAPI != "gemini.generate_content" {
		return nil, &relay.ProviderError{StatusCode: http.StatusBadRequest, Code: "provider_request_invalid", Message: "unsupported gemini request"}
	}
	if strings.HasPrefix(channel.BaseURL, "mock://") {
		return mockGenerateContent(request), nil
	}

	upstreamModel := request.UpstreamModel
	if upstreamModel == "" {
		upstreamModel = channel.UpstreamModel
	}
	endpoint, err := endpointURL(channel.BaseURL, upstreamModel, request.Stream)
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

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(request.RawBody))
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	q := req.URL.Query()
	if channel.APIKey != "" {
		q.Set("key", channel.APIKey)
	}
	if request.Stream {
		q.Set("alt", "sse")
	}
	req.URL.RawQuery = q.Encode()

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

func endpointURL(base string, model string, stream bool) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", &relay.ProviderError{StatusCode: http.StatusBadGateway, Code: "provider_config_invalid", Message: "provider base url is invalid"}
	}
	method := "generateContent"
	if stream {
		method = "streamGenerateContent"
	}
	if strings.Contains(parsed.Path, ":generateContent") || strings.Contains(parsed.Path, ":streamGenerateContent") {
		return parsed.String(), nil
	}
	if strings.HasSuffix(parsed.Path, "/v1beta") {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/models/" + model + ":" + method
	} else {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v1beta/models/" + model + ":" + method
	}
	return parsed.String(), nil
}

func mockGenerateContent(request relay.Request) *relay.Response {
	usage := tokenusage.EstimateFromBytes(request.RawBody)
	actual := tokenusage.Actual{InputTokens: usage.InputTokens, OutputTokens: 6, TotalTokens: usage.InputTokens + 6}
	if request.Stream {
		return &relay.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Stream: &relay.StaticStream{
				Chunks: [][]byte{
					[]byte("data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"M3 Gemini mock response\"}]}}]}\n\n"),
					[]byte("data: {\"usageMetadata\":{\"promptTokenCount\":1,\"candidatesTokenCount\":6,\"totalTokenCount\":7}}\n\n"),
				},
				Actual: actual,
			},
			Usage: actual,
		}
	}
	body, _ := json.Marshal(map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{
				"role":  "model",
				"parts": []map[string]string{{"text": "M3 Gemini mock response"}},
			},
			"finishReason": "STOP",
		}},
		"usageMetadata": map[string]int64{
			"promptTokenCount":     actual.InputTokens,
			"candidatesTokenCount": actual.OutputTokens,
			"totalTokenCount":      actual.TotalTokens,
		},
	})
	return &relay.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: append(body, '\n'), Usage: actual}
}

func parseUsage(body []byte) tokenusage.Actual {
	var payload struct {
		Usage struct {
			PromptTokenCount     int64 `json:"promptTokenCount"`
			CandidatesTokenCount int64 `json:"candidatesTokenCount"`
			TotalTokenCount      int64 `json:"totalTokenCount"`
			CachedContentTokens  int64 `json:"cachedContentTokenCount"`
			ThoughtsTokenCount   int64 `json:"thoughtsTokenCount"`
			PromptTokensDetails  []struct {
				Modality   string `json:"modality"`
				TokenCount int64  `json:"tokenCount"`
			} `json:"promptTokensDetails"`
			CandidatesTokensDetails []struct {
				Modality   string `json:"modality"`
				TokenCount int64  `json:"tokenCount"`
			} `json:"candidatesTokensDetails"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return tokenusage.Actual{}
	}
	actual := tokenusage.Actual{
		InputTokens:       payload.Usage.PromptTokenCount,
		OutputTokens:      payload.Usage.CandidatesTokenCount,
		TotalTokens:       payload.Usage.TotalTokenCount,
		CachedInputTokens: payload.Usage.CachedContentTokens,
		ReasoningTokens:   payload.Usage.ThoughtsTokenCount,
	}
	for _, detail := range payload.Usage.PromptTokensDetails {
		addModalityInput(&actual, detail.Modality, detail.TokenCount)
	}
	for _, detail := range payload.Usage.CandidatesTokensDetails {
		addModalityOutput(&actual, detail.Modality, detail.TokenCount)
	}
	if actual.TotalTokens == 0 {
		actual.TotalTokens = actual.InputTokens + actual.OutputTokens
	}
	return actual
}

func addModalityInput(actual *tokenusage.Actual, modality string, tokens int64) {
	switch strings.ToUpper(modality) {
	case "AUDIO":
		actual.AudioInputTokens += tokens
	case "IMAGE":
		actual.ImageInputTokens += tokens
	case "VIDEO":
		actual.VideoInputTokens += tokens
	}
}

func addModalityOutput(actual *tokenusage.Actual, modality string, tokens int64) {
	switch strings.ToUpper(modality) {
	case "AUDIO":
		actual.AudioOutputTokens += tokens
	case "IMAGE":
		actual.ImageOutputTokens += tokens
	case "VIDEO":
		actual.VideoOutputTokens += tokens
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
