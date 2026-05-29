package task

import (
	"encoding/json"
	"net/http"
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
		"file_url":      file.FileURL,
		"download_url":  file.DownloadURL,
		"upload_time":   file.CreatedAt.Format(time.RFC3339),
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
	var payload struct {
		Results []string `json:"results"`
		URL     string   `json:"url"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	if len(payload.Results) > 0 {
		return payload.Results
	}
	if payload.URL != "" {
		return []string{payload.URL}
	}
	return nil
}
