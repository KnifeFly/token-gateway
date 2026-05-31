package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/internal/provider/relay"
	"github.com/KnifeFly/token-gateway/pkg/egressguard"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const defaultTimeout = 30 * time.Second

// Adapter relays requests to OpenAI-compatible providers.
type Adapter struct {
	client *http.Client
	egress *egressguard.Guard
}

// NewAdapter returns an OpenAI-compatible adapter.
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
	if a.egress != nil {
		if err := a.egress.ValidateURL(ctx, endpoint); err != nil {
			return nil, &relay.ProviderError{StatusCode: http.StatusBadGateway, Code: "provider_config_invalid", Message: "provider egress url is not allowed"}
		}
	}

	if request.UpstreamModel == "" {
		request.UpstreamModel = channel.UpstreamModel
	}
	body, contentType, err := rewriteRequestBody(request)
	if err != nil {
		return nil, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, err
	}
	if contentType == "" {
		contentType = "application/json"
	}
	httpReq.Header.Set("Content-Type", contentType)
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
		cancel()
		return nil, relay.ErrorFromRequestFailure(err)
	}
	if request.Stream {
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			defer cancel()
			defer res.Body.Close()
			content, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
			return nil, relay.ErrorFromStatus(res.StatusCode, content)
		}
		return &relay.Response{
			StatusCode: res.StatusCode,
			Header:     safeStreamHeaders(res.Header),
			Stream:     newHTTPStream(res.Body, cancel),
		}, nil
	}
	defer cancel()
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
		return nil, relay.ErrorFromStatus(res.StatusCode, content)
	}
	return &relay.Response{
		StatusCode: res.StatusCode,
		Header:     safeHeaders(res.Header),
		Body:       content,
		Usage:      parseUsage(content),
	}, nil
}

func rewriteRequestBody(request relay.Request) ([]byte, string, error) {
	contentType := request.ContentType
	if contentType == "" || strings.Contains(strings.ToLower(contentType), "json") {
		body, err := rewriteJSONModel(request.RawBody, request.UpstreamModel)
		return body, contentType, err
	}
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/") {
		return rewriteMultipartModel(request.RawBody, contentType, request.UpstreamModel)
	}
	return append([]byte(nil), request.RawBody...), contentType, nil
}

func rewriteJSONModel(body []byte, upstreamModel string) ([]byte, error) {
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

func rewriteMultipartModel(body []byte, contentType string, upstreamModel string) ([]byte, string, error) {
	if upstreamModel == "" {
		return append([]byte(nil), body...), contentType, nil
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil || params["boundary"] == "" {
		return nil, "", &relay.ProviderError{
			StatusCode: http.StatusBadRequest,
			Code:       "provider_request_invalid",
			Message:    "multipart request content type is invalid",
			Retryable:  false,
		}
	}
	form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(32 << 20)
	if err != nil {
		return nil, "", &relay.ProviderError{
			StatusCode: http.StatusBadRequest,
			Code:       "provider_request_invalid",
			Message:    "multipart request could not be decoded",
			Retryable:  false,
		}
	}
	defer func() { _ = form.RemoveAll() }()

	var rewritten bytes.Buffer
	writer := multipart.NewWriter(&rewritten)
	modelWritten := false
	for name, values := range form.Value {
		for _, value := range values {
			if name == "model" {
				value = upstreamModel
				modelWritten = true
			}
			if err := writer.WriteField(name, value); err != nil {
				_ = writer.Close()
				return nil, "", err
			}
		}
	}
	if !modelWritten {
		if err := writer.WriteField("model", upstreamModel); err != nil {
			_ = writer.Close()
			return nil, "", err
		}
	}
	for _, files := range form.File {
		for _, fileHeader := range files {
			if err := copyMultipartFile(writer, fileHeader); err != nil {
				_ = writer.Close()
				return nil, "", err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return rewritten.Bytes(), writer.FormDataContentType(), nil
}

func copyMultipartFile(writer *multipart.Writer, fileHeader *multipart.FileHeader) error {
	source, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	part, err := writer.CreatePart(fileHeader.Header)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, source)
	return err
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
	case "openai.moderations":
		return appendPath(base, "/v1/moderations")
	case "unified.image_generation":
		return appendPath(base, "/v1/images/generations")
	case "unified.image_edit":
		return appendPath(base, "/v1/images/edits")
	case "unified.audio_speech":
		return appendPath(base, "/v1/audio/speech")
	case "unified.audio_transcription":
		return appendPath(base, "/v1/audio/transcriptions")
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
	if request.CanonicalAPI == "openai.moderations" {
		body, _ := json.Marshal(map[string]any{
			"id":    fmt.Sprintf("modr-mock-%d", now),
			"model": request.PublicModel,
			"results": []map[string]any{{
				"flagged": false,
				"categories": map[string]bool{
					"violence": false,
					"hate":     false,
					"sexual":   false,
				},
				"category_scores": map[string]float64{
					"violence": 0,
					"hate":     0,
					"sexual":   0,
				},
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
			Usage: tokenusage.Actual{
				InputTokens: usage.InputTokens,
				TotalTokens: usage.InputTokens,
			},
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
	type detail struct {
		CachedTokens    int64 `json:"cached_tokens"`
		AudioTokens     int64 `json:"audio_tokens"`
		ReasoningTokens int64 `json:"reasoning_tokens"`
	}
	type usageShape struct {
		PromptTokens            int64  `json:"prompt_tokens"`
		CompletionTokens        int64  `json:"completion_tokens"`
		InputTokens             int64  `json:"input_tokens"`
		OutputTokens            int64  `json:"output_tokens"`
		TotalTokens             int64  `json:"total_tokens"`
		PromptTokensDetails     detail `json:"prompt_tokens_details"`
		CompletionTokensDetails detail `json:"completion_tokens_details"`
		InputTokensDetails      detail `json:"input_tokens_details"`
		OutputTokensDetails     detail `json:"output_tokens_details"`
	}
	var payload struct {
		Usage    usageShape `json:"usage"`
		Response struct {
			Usage usageShape `json:"usage"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return tokenusage.Actual{}
	}
	usage := payload.Usage
	if usage == (usageShape{}) {
		usage = payload.Response.Usage
	}
	inputTokens := usage.PromptTokens
	if inputTokens == 0 {
		inputTokens = usage.InputTokens
	}
	outputTokens := usage.CompletionTokens
	if outputTokens == 0 {
		outputTokens = usage.OutputTokens
	}
	totalTokens := usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = inputTokens + outputTokens
	}
	return tokenusage.Actual{
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		TotalTokens:       totalTokens,
		CachedInputTokens: usage.PromptTokensDetails.CachedTokens + usage.InputTokensDetails.CachedTokens,
		ReasoningTokens:   usage.CompletionTokensDetails.ReasoningTokens + usage.OutputTokensDetails.ReasoningTokens,
		AudioInputTokens:  usage.PromptTokensDetails.AudioTokens + usage.InputTokensDetails.AudioTokens,
		AudioOutputTokens: usage.CompletionTokensDetails.AudioTokens + usage.OutputTokensDetails.AudioTokens,
	}
}

type httpStream struct {
	body     io.ReadCloser
	cancel   context.CancelFunc
	buf      []byte
	eventBuf []byte
	actual   tokenusage.Actual
}

func newHTTPStream(body io.ReadCloser, cancel context.CancelFunc) *httpStream {
	return &httpStream{body: body, cancel: cancel}
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
	err := s.body.Close()
	if s.cancel != nil {
		s.cancel()
	}
	return err
}

func (s *httpStream) observeUsage(chunk []byte) {
	s.eventBuf = append(s.eventBuf, chunk...)
	for {
		event, rest, ok := nextSSEEvent(s.eventBuf)
		if !ok {
			return
		}
		s.eventBuf = rest
		data := sseEventData(event)
		if len(data) == 0 || strings.TrimSpace(string(data)) == "[DONE]" {
			continue
		}
		usage := parseUsage([]byte(data))
		if usage.TotalTokens > 0 {
			s.actual = tokenusage.Merge(s.actual, usage)
		}
	}
}

func nextSSEEvent(buffer []byte) ([]byte, []byte, bool) {
	index, separatorLen := nextSSEEventEnd(buffer)
	if index < 0 {
		return nil, buffer, false
	}
	event := append([]byte(nil), buffer[:index]...)
	rest := buffer[index+separatorLen:]
	return event, rest, true
}

func nextSSEEventEnd(buffer []byte) (int, int) {
	index := bytes.Index(buffer, []byte("\n\n"))
	separatorLen := 2
	if crlfIndex := bytes.Index(buffer, []byte("\r\n\r\n")); crlfIndex >= 0 && (index < 0 || crlfIndex < index) {
		index = crlfIndex
		separatorLen = 4
	}
	return index, separatorLen
}

func sseEventData(event []byte) []byte {
	normalized := strings.ReplaceAll(string(event), "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	var parts []string
	for _, line := range strings.Split(normalized, "\n") {
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimPrefix(line, "data:")
		if strings.HasPrefix(data, " ") {
			data = strings.TrimPrefix(data, " ")
		}
		parts = append(parts, data)
	}
	if len(parts) == 0 {
		return nil
	}
	return []byte(strings.Join(parts, "\n"))
}
