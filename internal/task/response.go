package task

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

// TaskResponse serializes a task into a gateway JSON response.
func TaskResponse(task *Task) (*engine.GatewayResponse, error) {
	body, err := json.Marshal(TaskObject(task))
	if err != nil {
		return nil, err
	}
	return jsonResponse(http.StatusOK, body), nil
}

// FileResponse serializes a file upload response.
func FileResponse(file *FileAsset) (*engine.GatewayResponse, error) {
	body, err := json.Marshal(map[string]any{
		"success": true,
		"code":    200,
		"msg":     "ok",
		"data":    FileObject(file),
	})
	if err != nil {
		return nil, err
	}
	return jsonResponse(http.StatusOK, body), nil
}

// FileQuotaResponse serializes file quota usage.
func FileQuotaResponse(quota FileQuota) (*engine.GatewayResponse, error) {
	body, err := json.Marshal(map[string]any{
		"success": true,
		"data": map[string]any{
			"max_files":       quota.MaxFiles,
			"used_files":      quota.UsedFiles,
			"remaining_files": quota.RemainingFiles,
			"max_bytes":       quota.MaxBytes,
			"used_bytes":      quota.UsedBytes,
			"quota_kind":      "transient_input_asset",
		},
	})
	if err != nil {
		return nil, err
	}
	return jsonResponse(http.StatusOK, body), nil
}

// TaskObject returns the OpenAPI-compatible task object.
func TaskObject(task *Task) map[string]any {
	if task == nil {
		return map[string]any{}
	}
	object := string(task.Kind) + ".task"
	if task.Kind == KindAudioSpeech || task.Kind == KindAudioTranscription {
		object = "audio.generation.task"
	}
	result := resultURLs(task.Result)
	assets := resultAssets(task.Result)
	providerMetadata := resultProviderMetadata(task.Result)
	payload := map[string]any{
		"id":       task.ID,
		"object":   object,
		"created":  task.CreatedAt.Unix(),
		"model":    task.Model,
		"status":   externalTaskStatus(task.Status),
		"progress": task.Progress,
		"type":     task.MediaType,
		"results":  result,
		"task_info": map[string]any{
			"can_cancel": !IsTerminal(task.Status),
		},
		"metadata": task.Metadata,
	}
	if task.ProviderTaskID != "" {
		payload["provider_task_id"] = task.ProviderTaskID
	}
	if task.ProviderType != "" {
		payload["provider_type"] = task.ProviderType
	}
	if task.ChannelID != "" {
		payload["channel_id"] = task.ChannelID
	}
	if len(assets) > 0 {
		payload["assets"] = assets
	}
	if len(providerMetadata) > 0 {
		payload["provider_metadata"] = providerMetadata
	}
	if task.ErrorCode != "" || task.ErrorMessage != "" {
		payload["error"] = map[string]any{
			"code":      task.ErrorCode,
			"message":   task.ErrorMessage,
			"type":      "task_error",
			"retryable": false,
		}
	}
	if task.Usage.TotalTokens > 0 {
		payload["usage"] = map[string]any{
			"billing_rule":  "per_token",
			"input_tokens":  task.Usage.InputTokens,
			"output_tokens": task.Usage.OutputTokens,
			"total_tokens":  task.Usage.TotalTokens,
		}
	}
	return payload
}

// FileObject returns the OpenAPI-compatible file object.
func FileObject(file *FileAsset) map[string]any {
	if file == nil {
		return map[string]any{}
	}
	payload := map[string]any{
		"file_id":       file.ID,
		"file_name":     file.FileName,
		"original_name": file.OriginalName,
		"file_size":     file.SizeBytes,
		"mime_type":     file.MIMEType,
		"upload_path":   file.UploadPath,
		"source":        file.Source,
		"content_hash":  file.ContentHash,
		"source_url":    file.SourceURL,
		"transient":     file.Transient,
		"upload_time":   file.CreatedAt.Format(time.RFC3339),
	}
	if file.FileURL != "" {
		payload["file_url"] = file.FileURL
	}
	if file.DownloadURL != "" {
		payload["download_url"] = file.DownloadURL
	}
	if file.ExpiresAt != nil {
		payload["expires_at"] = file.ExpiresAt.Format(time.RFC3339)
	}
	return payload
}

func jsonResponse(status int, body []byte) *engine.GatewayResponse {
	return &engine.GatewayResponse{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       append(body, '\n'),
	}
}

func externalTaskStatus(status Status) string {
	switch status {
	case StatusQueued:
		return "queued"
	case StatusRunning:
		return "processing"
	case StatusSucceeded:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusCanceled:
		return "cancelled"
	case StatusExpired:
		return "expired"
	default:
		return "pending"
	}
}

func resultURLs(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	if urls := stringList(payload["results"]); len(urls) > 0 {
		return urls
	}
	if url := stringValue(payload["url"]); url != "" {
		return []string{url}
	}
	if assets := resultAssets(raw); len(assets) > 0 {
		return urlsFromAssets(assets)
	}
	return collectURLs(payload["output"])
}

func resultAssets(raw json.RawMessage) []ResultAsset {
	if len(raw) == 0 {
		return nil
	}
	var payload struct {
		Assets []ResultAsset `json:"assets"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return normalizeResultAssets(payload.Assets)
}

func resultProviderMetadata(raw json.RawMessage) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var payload struct {
		ProviderMetadata map[string]string `json:"provider_metadata"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return cleanMetadata(payload.ProviderMetadata)
}

func stringList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func stringValue(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func collectURLs(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	var urls []string
	collectURLValues(value, &urls)
	return urls
}

func collectURLValues(value any, urls *[]string) {
	switch typed := value.(type) {
	case string:
		typed = strings.TrimSpace(typed)
		if typed != "" && hasURLScheme(typed) {
			*urls = append(*urls, typed)
		}
	case []any:
		for _, item := range typed {
			collectURLValues(item, urls)
		}
	case map[string]any:
		for _, item := range typed {
			collectURLValues(item, urls)
		}
	}
}
