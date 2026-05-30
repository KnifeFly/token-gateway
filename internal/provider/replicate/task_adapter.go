package replicate

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
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const defaultTimeout = 30 * time.Second

// TaskAdapter maps Replicate predictions onto the Unified Media async task contract.
type TaskAdapter struct {
	client      *http.Client
	credentials tasksvc.ProviderCredentialResolver
}

// NewTaskAdapter returns a Replicate provider task adapter.
func NewTaskAdapter(client *http.Client, credentials tasksvc.ProviderCredentialResolver) *TaskAdapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &TaskAdapter{client: client, credentials: credentials}
}

// Submit creates a Replicate prediction.
func (a *TaskAdapter) Submit(ctx context.Context, request tasksvc.ProviderTaskRequest) (*tasksvc.ProviderTask, error) {
	if strings.HasPrefix(request.Channel.BaseURL, "mock://") {
		externalID := fmt.Sprintf("replicate_%s_%s", request.Candidate.ChannelID, request.Task.ID)
		return &tasksvc.ProviderTask{
			ExternalID: externalID,
			Status:     tasksvc.StatusRunning,
			Progress:   1,
			ProviderMetadata: map[string]string{
				"provider":      "replicate",
				"prediction_id": externalID,
			},
		}, nil
	}
	if strings.TrimSpace(request.Candidate.UpstreamModel) == "" {
		return nil, apperr.ConfigUnavailable("replicate version is required")
	}
	endpoint, err := predictionURL(request.Channel.BaseURL, "/predictions")
	if err != nil {
		return nil, err
	}
	input, err := replicateInput(request.Task.Input)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(predictionCreateRequest{
		Version:             request.Candidate.UpstreamModel,
		Input:               input,
		Webhook:             emptyAsOmitted(request.Task.CallbackURL),
		WebhookEventsFilter: webhookEvents(request.Task.CallbackURL),
	})
	if err != nil {
		return nil, apperr.Internal("replicate prediction request could not be encoded", apperr.WithCause(err))
	}
	var out predictionResponse
	if err := a.doJSON(ctx, http.MethodPost, endpoint, request.Channel, request.RequestID, body, &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.ID) == "" {
		return nil, apperr.ProviderError("replicate prediction response is missing id")
	}
	return &tasksvc.ProviderTask{
		ExternalID:       out.ID,
		Status:           replicateStatus(out.Status),
		Progress:         replicateProgress(out.Status),
		ProviderMetadata: replicateMetadata(out),
	}, nil
}

// Poll fetches a Replicate prediction and normalizes terminal results.
func (a *TaskAdapter) Poll(ctx context.Context, task tasksvc.Task, channel engine.ChannelView) (*tasksvc.ProviderTaskResult, error) {
	if strings.HasPrefix(channel.BaseURL, "mock://") {
		resultURL := "https://replicate.example/mock-results/" + task.ID
		asset := tasksvc.ResultAsset{URL: resultURL, Type: mediaType(task), Provider: "replicate"}
		metadata := map[string]string{"provider": "replicate", "prediction_id": task.ProviderTaskID}
		result, _ := json.Marshal(map[string]any{
			"provider":          "replicate",
			"id":                task.ProviderTaskID,
			"output":            []string{resultURL},
			"results":           []string{resultURL},
			"assets":            []tasksvc.ResultAsset{asset},
			"provider_metadata": metadata,
		})
		return &tasksvc.ProviderTaskResult{
			Status:           tasksvc.StatusSucceeded,
			Progress:         100,
			Result:           result,
			Assets:           []tasksvc.ResultAsset{asset},
			Usage:            estimatedUsage(task.Input, result),
			ProviderMetadata: metadata,
		}, nil
	}
	endpoint, err := predictionURL(channel.BaseURL, "/predictions/"+url.PathEscape(task.ProviderTaskID))
	if err != nil {
		return nil, err
	}
	var out predictionResponse
	if err := a.doJSON(ctx, http.MethodGet, endpoint, channel, task.RequestID, nil, &out); err != nil {
		return nil, err
	}
	status := replicateStatus(out.Status)
	if status == "" {
		return nil, apperr.ProviderError("replicate prediction response has invalid status")
	}
	assets := replicateResultAssets(task, out)
	metadata := replicateMetadata(out)
	result, err := normalizedResult(out, assets, metadata)
	if err != nil {
		return nil, err
	}
	return &tasksvc.ProviderTaskResult{
		Status:           status,
		Progress:         replicateProgress(out.Status),
		Result:           result,
		Assets:           assets,
		Usage:            estimatedUsage(task.Input, result),
		ErrorCode:        errorCode(status, out.Error),
		ErrorMessage:     errorMessage(out.Error),
		ProviderMetadata: metadata,
	}, nil
}

// Cancel asks Replicate to cancel a running prediction.
func (a *TaskAdapter) Cancel(ctx context.Context, task tasksvc.Task, channel engine.ChannelView) error {
	if task.ProviderTaskID == "" || strings.HasPrefix(channel.BaseURL, "mock://") {
		return nil
	}
	endpoint, err := predictionURL(channel.BaseURL, "/predictions/"+url.PathEscape(task.ProviderTaskID)+"/cancel")
	if err != nil {
		return err
	}
	return a.doJSON(ctx, http.MethodPost, endpoint, channel, task.RequestID, nil, nil)
}

func (a *TaskAdapter) doJSON(ctx context.Context, method string, endpoint string, channel engine.ChannelView, requestID string, body []byte, out any) error {
	timeout := channel.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
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
		return apperr.ProviderError("replicate prediction request failed", apperr.WithCause(err), apperr.WithTemporary())
	}
	defer res.Body.Close()
	content, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return apperr.ProviderError("replicate prediction response could not be read", apperr.WithCause(err), apperr.WithTemporary())
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return providerHTTPError(res.StatusCode, content)
	}
	if out == nil || len(content) == 0 {
		return nil
	}
	if err := json.Unmarshal(content, out); err != nil {
		return apperr.ProviderError("replicate prediction response is invalid", apperr.WithCause(err))
	}
	return nil
}

func predictionURL(base string, suffix string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", apperr.ProviderError("replicate base url is invalid")
	}
	if suffix == "/predictions" && strings.HasSuffix(parsed.Path, "/predictions") {
		return parsed.String(), nil
	}
	parsed.Path = "/v1" + suffix
	return parsed.String(), nil
}

func replicateInput(raw json.RawMessage) (map[string]any, error) {
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, apperr.InvalidArgument("replicate task input must be valid json", apperr.WithCause(err))
	}
	input := make(map[string]any)
	for key, value := range body {
		switch key {
		case "model", "callback_url", "metadata", "model_params":
			continue
		default:
			input[key] = value
		}
	}
	if params, ok := body["model_params"].(map[string]any); ok {
		for key, value := range params {
			input[key] = value
		}
	}
	return input, nil
}

func replicateStatus(status string) tasksvc.Status {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "starting", "processing":
		return tasksvc.StatusRunning
	case "succeeded", "successful":
		return tasksvc.StatusSucceeded
	case "failed":
		return tasksvc.StatusFailed
	case "canceled", "cancelled":
		return tasksvc.StatusCanceled
	default:
		return ""
	}
}

func replicateProgress(status string) int {
	switch replicateStatus(status) {
	case tasksvc.StatusSucceeded, tasksvc.StatusFailed, tasksvc.StatusCanceled:
		return 100
	case tasksvc.StatusRunning:
		return 50
	default:
		return 0
	}
}

func normalizedResult(out predictionResponse, assets []tasksvc.ResultAsset, metadata map[string]string) (json.RawMessage, error) {
	result, err := json.Marshal(map[string]any{
		"provider":          "replicate",
		"id":                out.ID,
		"output":            out.Output,
		"results":           replicateResultURLs(out.Output),
		"assets":            assets,
		"provider_metadata": metadata,
		"urls":              out.URLs,
		"metrics":           out.Metrics,
	})
	if err != nil {
		return nil, apperr.ProviderError("replicate prediction result could not be encoded", apperr.WithCause(err))
	}
	return result, nil
}

func replicateResultAssets(task tasksvc.Task, out predictionResponse) []tasksvc.ResultAsset {
	resultURLs := replicateResultURLs(out.Output)
	assets := make([]tasksvc.ResultAsset, 0, len(resultURLs))
	for _, resultURL := range resultURLs {
		assets = append(assets, tasksvc.ResultAsset{
			URL:      resultURL,
			Type:     mediaType(task),
			Provider: "replicate",
			Metadata: map[string]string{
				"prediction_id": out.ID,
			},
		})
	}
	return assets
}

func replicateResultURLs(output any) []string {
	var urls []string
	collectReplicateURLs(output, &urls)
	return urls
}

func collectReplicateURLs(value any, urls *[]string) {
	switch typed := value.(type) {
	case string:
		typed = strings.TrimSpace(typed)
		if typed != "" && hasURLScheme(typed) {
			*urls = append(*urls, typed)
		}
	case []string:
		for _, item := range typed {
			collectReplicateURLs(item, urls)
		}
	case []any:
		for _, item := range typed {
			collectReplicateURLs(item, urls)
		}
	case map[string]any:
		for _, item := range typed {
			collectReplicateURLs(item, urls)
		}
	}
}

func replicateMetadata(out predictionResponse) map[string]string {
	metadata := map[string]string{
		"provider":      "replicate",
		"prediction_id": out.ID,
		"status":        out.Status,
	}
	if getURL, ok := out.URLs["get"].(string); ok && strings.TrimSpace(getURL) != "" {
		metadata["get_url"] = strings.TrimSpace(getURL)
	}
	return metadata
}

func mediaType(task tasksvc.Task) string {
	if strings.TrimSpace(task.MediaType) != "" {
		return strings.TrimSpace(task.MediaType)
	}
	kind := string(task.Kind)
	if idx := strings.Index(kind, "."); idx >= 0 {
		return kind[:idx]
	}
	return ""
}

func hasURLScheme(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != ""
}

func estimatedUsage(input json.RawMessage, result json.RawMessage) tokenusage.Actual {
	usage := tokenusage.Actual{
		InputTokens:  int64(len(input) / 4),
		OutputTokens: int64(len(result) / 4),
	}
	if len(result) > 0 && usage.OutputTokens == 0 {
		usage.OutputTokens = 1
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	return usage
}

func emptyAsOmitted(value string) string {
	return strings.TrimSpace(value)
}

func webhookEvents(webhook string) []string {
	if strings.TrimSpace(webhook) == "" {
		return nil
	}
	return []string{"completed"}
}

func errorCode(status tasksvc.Status, err any) string {
	if status != tasksvc.StatusFailed || err == nil {
		return ""
	}
	return "replicate_prediction_failed"
}

func errorMessage(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		encoded, _ := json.Marshal(typed)
		return string(encoded)
	}
}

func providerHTTPError(status int, content []byte) error {
	message := fmt.Sprintf("replicate prediction returned status %d", status)
	if len(content) > 0 {
		message = fmt.Sprintf("%s: %s", message, strings.TrimSpace(string(content)))
	}
	switch status {
	case http.StatusTooManyRequests:
		return apperr.RateLimited("replicate prediction is rate limited", apperr.WithTemporary())
	case http.StatusUnauthorized, http.StatusForbidden:
		return apperr.ProviderError("replicate prediction authentication failed")
	default:
		opts := []apperr.Option{apperr.WithUnsafeMessage()}
		if status >= 500 {
			opts = append(opts, apperr.WithTemporary())
		}
		return apperr.ProviderError(message, opts...)
	}
}

type predictionCreateRequest struct {
	Version             string         `json:"version"`
	Input               map[string]any `json:"input"`
	Webhook             string         `json:"webhook,omitempty"`
	WebhookEventsFilter []string       `json:"webhook_events_filter,omitempty"`
}

type predictionResponse struct {
	ID      string         `json:"id"`
	Status  string         `json:"status"`
	Output  any            `json:"output"`
	Error   any            `json:"error"`
	URLs    map[string]any `json:"urls"`
	Metrics map[string]any `json:"metrics"`
}
