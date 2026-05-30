package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

// ChannelConfig is the provider adapter input for one selected channel.
type ChannelConfig struct {
	ChannelID     string
	ProviderType  string
	BaseURL       string
	APIKey        string
	UpstreamModel string
	Timeout       time.Duration
}

// Request is the provider relay input after routing.
type Request struct {
	CanonicalAPI  string
	PublicModel   string
	UpstreamModel string
	RawBody       []byte
	ContentType   string
	Headers       http.Header
	RequestID     string
	Stream        bool
}

// Response is a provider adapter response.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Stream     ProviderStream
	Usage      tokenusage.Actual
}

// ProviderStream is a pull-based stream owned by provider adapters.
type ProviderStream interface {
	Recv(ctx context.Context) ([]byte, error)
	Usage() tokenusage.Actual
	Close() error
}

// DownstreamErrorReporter records errors after bytes have started flowing to the client.
type DownstreamErrorReporter interface {
	ReportDownstreamError(err error)
}

// Adapter relays provider requests.
type Adapter interface {
	Relay(ctx context.Context, channel ChannelConfig, request Request) (*Response, error)
}

// ProviderError is a safe, classified upstream error.
type ProviderError struct {
	StatusCode   int
	Code         string
	ProviderCode string
	Message      string
	Retryable    bool
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ClassifyStatus maps an HTTP status to a safe provider error class.
func ClassifyStatus(status int) (code string, retryable bool) {
	switch {
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return "provider_request_invalid", false
	case status == http.StatusTooManyRequests:
		return "provider_rate_limited", true
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "provider_auth_failed", false
	case status == http.StatusNotFound:
		return "provider_not_found", false
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		return "provider_timeout", true
	case status >= 500:
		return "provider_unavailable", true
	default:
		return "provider_error", false
	}
}

// ErrorFromStatus maps an upstream HTTP error response to a safe provider error.
func ErrorFromStatus(status int, body []byte) *ProviderError {
	code, retryable := ClassifyStatus(status)
	providerCode := providerErrorCode(body)
	message := fmt.Sprintf("provider returned status %d", status)
	if providerCode != "" {
		message = fmt.Sprintf("%s (%s)", message, providerCode)
	}
	return &ProviderError{
		StatusCode:   status,
		Code:         code,
		ProviderCode: providerCode,
		Message:      message,
		Retryable:    retryable,
	}
}

// ErrorFromRequestFailure maps transport and timeout errors to stable provider classes.
func ErrorFromRequestFailure(err error) *ProviderError {
	code := "provider_unavailable"
	if errors.Is(err, context.DeadlineExceeded) {
		code = "provider_timeout"
	} else {
		var timeout interface{ Timeout() bool }
		if errors.As(err, &timeout) && timeout.Timeout() {
			code = "provider_timeout"
		}
	}
	return &ProviderError{
		StatusCode: http.StatusBadGateway,
		Code:       code,
		Message:    "provider request failed",
		Retryable:  true,
	}
}

func providerErrorCode(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if payload.Error.Code != "" {
		return payload.Error.Code
	}
	return payload.Error.Type
}

// StaticStream emits a deterministic in-memory stream for local mocks.
type StaticStream struct {
	Chunks [][]byte
	Actual tokenusage.Actual
	index  int
}

// Recv returns the next stream chunk or io.EOF.
func (s *StaticStream) Recv(context.Context) ([]byte, error) {
	if s.index >= len(s.Chunks) {
		return nil, io.EOF
	}
	chunk := s.Chunks[s.index]
	s.index++
	return chunk, nil
}

// Usage returns final stream usage.
func (s *StaticStream) Usage() tokenusage.Actual {
	return s.Actual
}

// Close releases stream resources.
func (s *StaticStream) Close() error {
	return nil
}
