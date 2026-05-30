package relay

import (
	"context"
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
	StatusCode int
	Code       string
	Message    string
	Retryable  bool
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ClassifyStatus maps an HTTP status to a safe provider error class.
func ClassifyStatus(status int) (code string, retryable bool) {
	switch {
	case status == http.StatusTooManyRequests:
		return "provider_rate_limited", true
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "provider_auth_failed", false
	case status >= 500:
		return "provider_unavailable", true
	default:
		return "provider_error", false
	}
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
