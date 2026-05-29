package relay

import (
	"context"
	"fmt"
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

// ChatCompletionRequest is the OpenAI-compatible chat relay input.
type ChatCompletionRequest struct {
	PublicModel   string
	UpstreamModel string
	RawBody       []byte
	RequestID     string
}

// Response is a provider adapter response.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Usage      tokenusage.Actual
}

// Adapter relays OpenAI-compatible chat completion requests.
type Adapter interface {
	ChatCompletions(ctx context.Context, channel ChannelConfig, request ChatCompletionRequest) (*Response, error)
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
