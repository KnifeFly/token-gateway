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

// ProviderTaskAdapter maps one provider's async media task contract.
type ProviderTaskAdapter interface {
	Submit(ctx context.Context, request ProviderTaskRequest) (*ProviderTask, error)
	Poll(ctx context.Context, task Task, channel engine.ChannelView) (*ProviderTaskResult, error)
	Cancel(ctx context.Context, task Task, channel engine.ChannelView) error
}

// ProviderTaskAdapterRegistry stores provider-specific async task adapters.
type ProviderTaskAdapterRegistry struct {
	adapters map[string]ProviderTaskAdapter
	fallback ProviderTaskAdapter
}

// NewProviderTaskAdapterRegistry returns a registry with the supplied fallback adapter.
func NewProviderTaskAdapterRegistry(fallback ProviderTaskAdapter) *ProviderTaskAdapterRegistry {
	return &ProviderTaskAdapterRegistry{adapters: map[string]ProviderTaskAdapter{}, fallback: fallback}
}

// Register adds or replaces a provider-specific async task adapter.
func (r *ProviderTaskAdapterRegistry) Register(providerType string, adapter ProviderTaskAdapter) {
	if r == nil || strings.TrimSpace(providerType) == "" || adapter == nil {
		return
	}
	r.adapters[providerType] = adapter
}

func (r *ProviderTaskAdapterRegistry) adapter(providerType string) ProviderTaskAdapter {
	if r == nil {
		return nil
	}
	if adapter := r.adapters[providerType]; adapter != nil {
		return adapter
	}
	return r.fallback
}

// HTTPProviderTaskDispatcher calls provider-specific async task adapters.
type HTTPProviderTaskDispatcher struct {
	registry *ProviderTaskAdapterRegistry
	channels ProviderChannelResolver
}

// GenericHTTPProviderTaskAdapter maps the default /v1/tasks async provider contract.
type GenericHTTPProviderTaskAdapter struct {
	client      *http.Client
	credentials ProviderCredentialResolver
}

// NewHTTPProviderTaskDispatcher returns a provider task dispatcher backed by HTTP.
func NewHTTPProviderTaskDispatcher(client *http.Client, credentials ProviderCredentialResolver, channels ProviderChannelResolver) *HTTPProviderTaskDispatcher {
	adapter := NewGenericHTTPProviderTaskAdapter(client, credentials)
	return &HTTPProviderTaskDispatcher{registry: NewProviderTaskAdapterRegistry(adapter), channels: channels}
}

// NewGenericHTTPProviderTaskAdapter returns the default HTTP async task adapter.
func NewGenericHTTPProviderTaskAdapter(client *http.Client, credentials ProviderCredentialResolver) *GenericHTTPProviderTaskAdapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &GenericHTTPProviderTaskAdapter{client: client, credentials: credentials}
}

// RegisterAdapter adds a provider-specific adapter to the dispatcher.
func (d *HTTPProviderTaskDispatcher) RegisterAdapter(providerType string, adapter ProviderTaskAdapter) {
	if d == nil || d.registry == nil {
		return
	}
	d.registry.Register(providerType, adapter)
}

// Submit creates an upstream provider task.
func (d *HTTPProviderTaskDispatcher) Submit(ctx context.Context, request ProviderTaskRequest) (*ProviderTask, error) {
	adapter := d.adapter(request.Candidate.ProviderType)
	if adapter == nil {
		return nil, apperr.ConfigUnavailable("provider task adapter is unavailable")
	}
	return adapter.Submit(ctx, request)
}

// Submit creates an upstream provider task using the default HTTP contract.
func (a *GenericHTTPProviderTaskAdapter) Submit(ctx context.Context, request ProviderTaskRequest) (*ProviderTask, error) {
	if strings.HasPrefix(request.Channel.BaseURL, "mock://") {
		externalID := fmt.Sprintf("external_%s_%s", request.Candidate.ChannelID, request.Task.ID)
		return &ProviderTask{
			ExternalID: externalID,
			Status:     StatusRunning,
			Progress:   1,
			ProviderMetadata: map[string]string{
				"external_task_id": externalID,
				"provider":         firstNonEmpty(request.Candidate.ProviderType, "mock_media"),
			},
		}, nil
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
	if err := a.doJSON(ctx, http.MethodPost, endpoint, request.Channel, request.RequestID, body, &out); err != nil {
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
	result := NormalizeProviderTaskResult(ProviderTaskResult{
		Status:           status,
		Progress:         out.Progress,
		Result:           out.Result,
		Assets:           assetsFromProviderTaskResponse(out, request.Task),
		Usage:            out.Usage.actual(),
		ErrorCode:        out.ErrorCode,
		ErrorMessage:     out.ErrorMessage,
		ProviderMetadata: out.ProviderMetadata,
	})
	return &ProviderTask{
		ExternalID:       externalID,
		Status:           result.Status,
		Progress:         result.Progress,
		Result:           result.Result,
		Assets:           result.Assets,
		Usage:            result.Usage,
		ErrorCode:        result.ErrorCode,
		ErrorMessage:     result.ErrorMessage,
		ProviderMetadata: result.ProviderMetadata,
	}, nil
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
	adapter := d.adapter(task.ProviderType)
	if adapter == nil {
		return nil, apperr.ConfigUnavailable("provider task adapter is unavailable")
	}
	return adapter.Poll(ctx, task, channel)
}

// Poll fetches the current upstream provider task status using the default HTTP contract.
func (a *GenericHTTPProviderTaskAdapter) Poll(ctx context.Context, task Task, channel engine.ChannelView) (*ProviderTaskResult, error) {
	if strings.HasPrefix(channel.BaseURL, "mock://") {
		return mockProviderTaskResult(task), nil
	}
	endpoint, err := providerTaskURL(channel.BaseURL, "/v1/tasks/"+url.PathEscape(task.ProviderTaskID))
	if err != nil {
		return nil, err
	}
	var out providerTaskResponse
	if err := a.doJSON(ctx, http.MethodGet, endpoint, channel, task.RequestID, nil, &out); err != nil {
		return nil, err
	}
	status := normalizeProviderStatus(out.Status)
	if status == "" {
		return nil, apperr.ProviderError("provider task response has invalid status")
	}
	result := ProviderTaskResult{
		Status:           status,
		Progress:         out.Progress,
		Result:           out.Result,
		Assets:           assetsFromProviderTaskResponse(out, task),
		Usage:            out.Usage.actual(),
		ErrorCode:        out.ErrorCode,
		ErrorMessage:     out.ErrorMessage,
		ProviderMetadata: out.ProviderMetadata,
	}
	result = NormalizeProviderTaskResult(result)
	return &result, nil
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
	adapter := d.adapter(task.ProviderType)
	if adapter == nil {
		return apperr.ConfigUnavailable("provider task adapter is unavailable")
	}
	return adapter.Cancel(ctx, task, channel)
}

// Cancel asks the upstream provider to cancel an async task using the default HTTP contract.
func (a *GenericHTTPProviderTaskAdapter) Cancel(ctx context.Context, task Task, channel engine.ChannelView) error {
	if strings.HasPrefix(channel.BaseURL, "mock://") {
		return nil
	}
	endpoint, err := providerTaskURL(channel.BaseURL, "/v1/tasks/"+url.PathEscape(task.ProviderTaskID)+"/cancel")
	if err != nil {
		return err
	}
	return a.doJSON(ctx, http.MethodPost, endpoint, channel, task.RequestID, nil, nil)
}

func (d *HTTPProviderTaskDispatcher) adapter(providerType string) ProviderTaskAdapter {
	if d == nil || d.registry == nil {
		return nil
	}
	return d.registry.adapter(providerType)
}

func (a *GenericHTTPProviderTaskAdapter) doJSON(ctx context.Context, method string, endpoint string, channel engine.ChannelView, requestID string, body []byte, out any) error {
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
	if a.credentials != nil && channel.ID != "" {
		resolved, err := a.credentials.ResolveProviderAPIKey(ctx, channel)
		if err != nil {
			return err
		}
		apiKey = resolved
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	res, err := a.client.Do(req)
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
	provider := firstNonEmpty(task.ProviderType, "mock_media")
	resultURL := fmt.Sprintf("https://provider.example/mock-results/%s/%s", strings.ReplaceAll(string(task.Kind), ".", "_"), task.ID)
	asset := ResultAsset{URL: resultURL, Type: task.MediaType, Provider: provider}
	metadata := map[string]string{
		"external_task_id": task.ProviderTaskID,
		"provider":         provider,
	}
	result, _ := json.Marshal(map[string]any{
		"results":           []string{resultURL},
		"assets":            []ResultAsset{asset},
		"provider_metadata": metadata,
	})
	usage := tokenusage.Actual{
		InputTokens:  int64(len(task.Input) / 4),
		OutputTokens: 32,
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	return &ProviderTaskResult{
		Status:           StatusSucceeded,
		Progress:         100,
		Result:           result,
		Assets:           []ResultAsset{asset},
		Usage:            usage,
		ProviderMetadata: metadata,
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
	ID               string            `json:"id"`
	ExternalTaskID   string            `json:"external_task_id"`
	Status           Status            `json:"status"`
	Progress         int               `json:"progress"`
	Result           json.RawMessage   `json:"result"`
	ResultURLs       []string          `json:"result_urls"`
	Assets           []ResultAsset     `json:"assets"`
	Usage            providerTaskUsage `json:"usage"`
	ErrorCode        string            `json:"error_code"`
	ErrorMessage     string            `json:"error_message"`
	ProviderMetadata map[string]string `json:"provider_metadata"`
}

func assetsFromProviderTaskResponse(out providerTaskResponse, task Task) []ResultAsset {
	if len(out.Assets) > 0 {
		return out.Assets
	}
	assets := make([]ResultAsset, 0, len(out.ResultURLs))
	provider := firstNonEmpty(task.ProviderType, "generic")
	for _, resultURL := range out.ResultURLs {
		assets = append(assets, ResultAsset{
			URL:      strings.TrimSpace(resultURL),
			Type:     task.MediaType,
			Provider: provider,
		})
	}
	return assets
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
