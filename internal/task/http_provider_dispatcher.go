package task

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

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const defaultProviderTaskTimeout = 30 * time.Second

// ProviderCredentialResolver resolves provider API keys for async task calls.
type ProviderCredentialResolver interface {
	ResolveProviderAPIKey(ctx context.Context, channel engine.ChannelView) (string, error)
}

// ProviderChannelResolver resolves provider channel config for background task polling.
type ProviderChannelResolver interface {
	ResolveProviderChannel(ctx context.Context, channelID string) (engine.ChannelView, bool, error)
}

// HTTPProviderTaskDispatcher calls a provider's HTTP async task API.
type HTTPProviderTaskDispatcher struct {
	client      *http.Client
	credentials ProviderCredentialResolver
	channels    ProviderChannelResolver
}

// NewHTTPProviderTaskDispatcher returns a provider task dispatcher backed by HTTP.
func NewHTTPProviderTaskDispatcher(client *http.Client, credentials ProviderCredentialResolver, channels ProviderChannelResolver) *HTTPProviderTaskDispatcher {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPProviderTaskDispatcher{client: client, credentials: credentials, channels: channels}
}

// Submit creates an upstream provider task.
func (d *HTTPProviderTaskDispatcher) Submit(ctx context.Context, request ProviderTaskRequest) (*ProviderTask, error) {
	if strings.HasPrefix(request.Channel.BaseURL, "mock://") {
		externalID := fmt.Sprintf("external_%s_%s", request.Candidate.ChannelID, request.Task.ID)
		return &ProviderTask{ExternalID: externalID, Status: StatusRunning, Progress: 1}, nil
	}
	endpoint, err := providerTaskURL(request.Channel.BaseURL, "/v1/tasks")
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(providerSubmitRequest{
		TaskID:        request.Task.ID,
		Kind:          string(request.Task.Kind),
		MediaType:     request.Task.MediaType,
		Model:         request.Candidate.PublicModel,
		UpstreamModel: request.Candidate.UpstreamModel,
		Input:         request.Task.Input,
		Metadata:      request.Task.Metadata,
		CallbackURL:   request.Task.CallbackURL,
	})
	if err != nil {
		return nil, apperr.Internal("provider task request could not be encoded", apperr.WithCause(err))
	}
	var out providerTaskResponse
	if err := d.doJSON(ctx, http.MethodPost, endpoint, request.Channel, request.RequestID, body, &out); err != nil {
		return nil, err
	}
	externalID := firstNonEmpty(out.ExternalTaskID, out.ID)
	if externalID == "" {
		return nil, apperr.ProviderError("provider task response is missing external task id")
	}
	status := normalizeProviderStatus(out.Status)
	if status == "" {
		status = StatusRunning
	}
	return &ProviderTask{ExternalID: externalID, Status: status, Progress: out.Progress}, nil
}

// Poll fetches the current upstream provider task status.
func (d *HTTPProviderTaskDispatcher) Poll(ctx context.Context, task Task) (*ProviderTaskResult, error) {
	if task.ProviderTaskID == "" {
		return nil, apperr.InvalidArgument("provider task id is required")
	}
	channel, err := d.resolveChannel(ctx, task.ChannelID)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(channel.BaseURL, "mock://") {
		return mockProviderTaskResult(task), nil
	}
	endpoint, err := providerTaskURL(channel.BaseURL, "/v1/tasks/"+url.PathEscape(task.ProviderTaskID))
	if err != nil {
		return nil, err
	}
	var out providerTaskResponse
	if err := d.doJSON(ctx, http.MethodGet, endpoint, channel, task.RequestID, nil, &out); err != nil {
		return nil, err
	}
	status := normalizeProviderStatus(out.Status)
	if status == "" {
		return nil, apperr.ProviderError("provider task response has invalid status")
	}
	return &ProviderTaskResult{
		Status:       status,
		Progress:     out.Progress,
		Result:       out.Result,
		Usage:        out.Usage.actual(),
		ErrorCode:    out.ErrorCode,
		ErrorMessage: out.ErrorMessage,
	}, nil
}

// Cancel asks the upstream provider to cancel an async task.
func (d *HTTPProviderTaskDispatcher) Cancel(ctx context.Context, task Task) error {
	if task.ProviderTaskID == "" {
		return nil
	}
	channel, err := d.resolveChannel(ctx, task.ChannelID)
	if err != nil {
		return err
	}
	if strings.HasPrefix(channel.BaseURL, "mock://") {
		return nil
	}
	endpoint, err := providerTaskURL(channel.BaseURL, "/v1/tasks/"+url.PathEscape(task.ProviderTaskID)+"/cancel")
	if err != nil {
		return err
	}
	return d.doJSON(ctx, http.MethodPost, endpoint, channel, task.RequestID, nil, nil)
}

func (d *HTTPProviderTaskDispatcher) doJSON(ctx context.Context, method string, endpoint string, channel engine.ChannelView, requestID string, body []byte, out any) error {
	timeout := channel.Timeout
	if timeout <= 0 {
		timeout = defaultProviderTaskTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	apiKey := channel.APIKey
	if d.credentials != nil && channel.ID != "" {
		resolved, err := d.credentials.ResolveProviderAPIKey(ctx, channel)
		if err != nil {
			return err
		}
		apiKey = resolved
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	res, err := d.client.Do(req)
	if err != nil {
		return apperr.ProviderError("provider task request failed", apperr.WithCause(err), apperr.WithTemporary())
	}
	defer res.Body.Close()
	content, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return apperr.ProviderError("provider task response could not be read", apperr.WithCause(err), apperr.WithTemporary())
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return providerTaskHTTPError(res.StatusCode, content)
	}
	if out == nil || len(content) == 0 {
		return nil
	}
	if err := json.Unmarshal(content, out); err != nil {
		return apperr.ProviderError("provider task response is invalid", apperr.WithCause(err))
	}
	return nil
}

func providerTaskURL(base string, suffix string) (string, error) {
	base = strings.TrimRight(base, "/")
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", apperr.ProviderError("provider base url is invalid")
	}
	if strings.HasSuffix(parsed.Path, suffix) {
		return parsed.String(), nil
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + suffix
	return parsed.String(), nil
}

func providerTaskHTTPError(status int, content []byte) error {
	message := fmt.Sprintf("provider task returned status %d", status)
	if len(content) > 0 {
		message = fmt.Sprintf("%s: %s", message, strings.TrimSpace(string(content)))
	}
	switch status {
	case http.StatusTooManyRequests:
		return apperr.RateLimited("provider task is rate limited", apperr.WithTemporary())
	case http.StatusUnauthorized, http.StatusForbidden:
		return apperr.ProviderError("provider task authentication failed")
	default:
		opts := []apperr.Option{apperr.WithUnsafeMessage()}
		if status >= 500 {
			opts = append(opts, apperr.WithTemporary())
		}
		return apperr.ProviderError(message, opts...)
	}
}

func (d *HTTPProviderTaskDispatcher) resolveChannel(ctx context.Context, channelID string) (engine.ChannelView, error) {
	if strings.TrimSpace(channelID) == "" {
		return engine.ChannelView{}, apperr.ConfigUnavailable("provider channel is unavailable")
	}
	if d.channels == nil {
		return engine.ChannelView{}, apperr.ConfigUnavailable("provider channel resolver is unavailable")
	}
	channel, ok, err := d.channels.ResolveProviderChannel(ctx, channelID)
	if err != nil {
		return engine.ChannelView{}, err
	}
	if !ok || !channel.Enabled {
		return engine.ChannelView{}, apperr.ServiceUnavailable("provider channel is unavailable", apperr.WithTemporary())
	}
	return channel, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mockProviderTaskResult(task Task) *ProviderTaskResult {
	resultURL := fmt.Sprintf("mock://%s/%s", strings.ReplaceAll(string(task.Kind), ".", "_"), task.ID)
	result, _ := json.Marshal(map[string]any{"results": []string{resultURL}})
	usage := tokenusage.Actual{
		InputTokens:  int64(len(task.Input) / 4),
		OutputTokens: 32,
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	return &ProviderTaskResult{
		Status:   StatusSucceeded,
		Progress: 100,
		Result:   result,
		Usage:    usage,
	}
}

func normalizeProviderStatus(status Status) Status {
	switch status {
	case StatusQueued, StatusRunning, StatusSucceeded, StatusFailed, StatusCanceled, StatusExpired:
		return status
	default:
		return ""
	}
}

type providerSubmitRequest struct {
	TaskID        string            `json:"task_id"`
	Kind          string            `json:"kind"`
	MediaType     string            `json:"media_type"`
	Model         string            `json:"model"`
	UpstreamModel string            `json:"upstream_model"`
	Input         json.RawMessage   `json:"input"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CallbackURL   string            `json:"callback_url,omitempty"`
}

type providerTaskResponse struct {
	ID             string            `json:"id"`
	ExternalTaskID string            `json:"external_task_id"`
	Status         Status            `json:"status"`
	Progress       int               `json:"progress"`
	Result         json.RawMessage   `json:"result"`
	Usage          providerTaskUsage `json:"usage"`
	ErrorCode      string            `json:"error_code"`
	ErrorMessage   string            `json:"error_message"`
}

type providerTaskUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

func (u providerTaskUsage) actual() tokenusage.Actual {
	total := u.TotalTokens
	if total == 0 {
		total = u.InputTokens + u.OutputTokens
	}
	return tokenusage.Actual{InputTokens: u.InputTokens, OutputTokens: u.OutputTokens, TotalTokens: total}
}
