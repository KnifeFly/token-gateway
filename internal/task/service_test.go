package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	metricnames "github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/KnifeFly/token-gateway/pkg/egressguard"
	"github.com/prometheus/client_golang/prometheus"
)

func TestServiceCreateMediaTaskIdempotency(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryRepository(), 0)
	request := CreateTaskRequest{
		TenantID:       "tenant",
		ProjectID:      "project",
		APIKeyID:       "key",
		RequestID:      "req_1",
		Endpoint:       "/v1/videos/generations",
		IdempotencyKey: "idem_1",
		Kind:           KindVideoGeneration,
		MediaType:      "video",
		Model:          "seedance-2.0-text-to-video",
		Input:          []byte(`{"model":"seedance-2.0-text-to-video","prompt":"hello"}`),
	}

	first, hit, err := service.CreateMediaTask(ctx, request)
	if err != nil {
		t.Fatalf("CreateMediaTask() error = %v", err)
	}
	if hit {
		t.Fatal("first create returned idempotency hit")
	}
	second, hit, err := service.CreateMediaTask(ctx, request)
	if err != nil {
		t.Fatalf("CreateMediaTask() duplicate error = %v", err)
	}
	if !hit {
		t.Fatal("duplicate did not return idempotency hit")
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate task id = %q, want %q", second.ID, first.ID)
	}
}

func TestServiceCreateMediaTaskIdempotencyConflict(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryRepository(), 0)
	request := CreateTaskRequest{
		TenantID:       "tenant",
		ProjectID:      "project",
		APIKeyID:       "key",
		RequestID:      "req_1",
		Endpoint:       "/v1/videos/generations",
		IdempotencyKey: "idem_1",
		Kind:           KindVideoGeneration,
		MediaType:      "video",
		Model:          "seedance-2.0-text-to-video",
		Input:          []byte(`{"model":"seedance-2.0-text-to-video","prompt":"hello"}`),
	}
	if _, _, err := service.CreateMediaTask(ctx, request); err != nil {
		t.Fatalf("CreateMediaTask() error = %v", err)
	}
	request.Input = []byte(`{"model":"seedance-2.0-text-to-video","prompt":"changed"}`)
	_, _, err := service.CreateMediaTask(ctx, request)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeIdempotencyConflict {
		t.Fatalf("error = %v, want idempotency_conflict", err)
	}
}

func TestServiceRecordsTaskMetrics(t *testing.T) {
	ctx := context.Background()
	registry := prometheus.NewRegistry()
	metrics, err := NewMetrics(registry)
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}
	service := NewServiceWithMetrics(NewMemoryRepository(), 0, metrics)
	task, _, err := service.CreateMediaTask(ctx, CreateTaskRequest{
		TenantID:       "tenant",
		ProjectID:      "project",
		APIKeyID:       "key",
		RequestID:      "req_1",
		Endpoint:       "/v1/videos/generations",
		IdempotencyKey: "idem_1",
		Kind:           KindVideoGeneration,
		MediaType:      "video",
		Model:          "seedance-2.0-text-to-video",
		Input:          []byte(`{"model":"seedance-2.0-text-to-video","prompt":"hello"}`),
	})
	if err != nil {
		t.Fatalf("CreateMediaTask() error = %v", err)
	}
	if _, err := service.MarkDispatched(ctx, task.ID, "mock_media", "channel_1", "provider_task_1"); err != nil {
		t.Fatalf("MarkDispatched() error = %v", err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range families {
		if family.GetName() == metricnames.MetricTaskLifecycleTransitions {
			return
		}
	}
	t.Fatalf("metric %q was not gathered", metricnames.MetricTaskLifecycleTransitions)
}

func TestServiceEnqueuesCallbackWhenTaskCompletes(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, 0)
	task, _, err := service.CreateMediaTask(ctx, CreateTaskRequest{
		TenantID:    "tenant",
		ProjectID:   "project",
		APIKeyID:    "key",
		RequestID:   "req_1",
		Endpoint:    "/v1/videos/generations",
		Kind:        KindVideoGeneration,
		MediaType:   "video",
		Model:       "seedance-2.0-text-to-video",
		Input:       []byte(`{"model":"seedance-2.0-text-to-video","prompt":"hello"}`),
		CallbackURL: "https://hooks.example/task",
	})
	if err != nil {
		t.Fatalf("CreateMediaTask() error = %v", err)
	}
	if _, err := service.CompleteTask(ctx, *task, ProviderTaskResult{Status: StatusSucceeded}); err != nil {
		t.Fatalf("CompleteTask() error = %v", err)
	}
	callbacks, err := repo.ListDueCallbacks(ctx, 10, service.now())
	if err != nil {
		t.Fatalf("ListDueCallbacks() error = %v", err)
	}
	if len(callbacks) != 1 || callbacks[0].URL != "https://hooks.example/task" {
		t.Fatalf("callbacks = %#v", callbacks)
	}
}

func TestCompleteTaskNormalizesProviderAssetsAndCallbackPayload(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, 0)
	created, _, err := service.CreateMediaTask(ctx, CreateTaskRequest{
		TenantID:    "tenant",
		ProjectID:   "project",
		APIKeyID:    "key",
		RequestID:   "req_1",
		Endpoint:    "/v1/images/generations",
		Kind:        KindImageGeneration,
		MediaType:   "image",
		Model:       "image-public",
		Input:       []byte(`{"model":"image-public","prompt":"hello"}`),
		CallbackURL: "https://hooks.example/task",
		Metadata:    map[string]string{"customer": "team-a"},
	})
	if err != nil {
		t.Fatalf("CreateMediaTask() error = %v", err)
	}
	dispatched, err := service.MarkDispatched(ctx, created.ID, "replicate", "channel_replicate", "pred_1")
	if err != nil {
		t.Fatalf("MarkDispatched() error = %v", err)
	}
	updated, err := service.CompleteTask(ctx, *dispatched, ProviderTaskResult{
		Status:   StatusSucceeded,
		Progress: 100,
		Assets: []ResultAsset{{
			URL:      "https://cdn.example/result.png",
			Type:     "image",
			Provider: "replicate",
		}},
		ProviderMetadata: map[string]string{"prediction_id": "pred_1"},
	})
	if err != nil {
		t.Fatalf("CompleteTask() error = %v", err)
	}
	if updated.Metadata["provider.prediction_id"] != "pred_1" {
		t.Fatalf("metadata = %#v", updated.Metadata)
	}
	object := TaskObject(updated)
	if object["provider_task_id"] != "pred_1" || object["provider_type"] != "replicate" {
		t.Fatalf("task object = %#v", object)
	}
	results, ok := object["results"].([]string)
	if !ok || len(results) != 1 || results[0] != "https://cdn.example/result.png" {
		t.Fatalf("results = %#v", object["results"])
	}
	assets, ok := object["assets"].([]ResultAsset)
	if !ok || len(assets) != 1 || assets[0].Provider != "replicate" {
		t.Fatalf("assets = %#v", object["assets"])
	}

	callbacks, err := repo.ListDueCallbacks(ctx, 10, service.now())
	if err != nil {
		t.Fatalf("ListDueCallbacks() error = %v", err)
	}
	if len(callbacks) != 1 {
		t.Fatalf("callbacks = %#v", callbacks)
	}
	var payload map[string]any
	if err := json.Unmarshal(callbacks[0].Payload, &payload); err != nil {
		t.Fatalf("callback payload json: %v", err)
	}
	if payload["provider_task_id"] != "pred_1" || payload["provider_type"] != "replicate" {
		t.Fatalf("callback payload = %#v", payload)
	}
	rawResults, ok := payload["results"].([]any)
	if !ok || len(rawResults) != 1 || rawResults[0] != "https://cdn.example/result.png" {
		t.Fatalf("callback results = %#v", payload["results"])
	}
	if _, ok := payload["provider_metadata"].(map[string]any); !ok {
		t.Fatalf("callback provider metadata = %#v", payload["provider_metadata"])
	}
}

func TestFileServiceIdempotency(t *testing.T) {
	ctx := context.Background()
	service := NewFileService(NewMemoryRepository(), 0)
	request := FileCreateRequest{
		TenantID:       "tenant",
		ProjectID:      "project",
		APIKeyID:       "key",
		RequestID:      "req_1",
		Endpoint:       "/v1/files/upload/base64",
		IdempotencyKey: "file_idem",
		RequestBody:    []byte(`{"base64_data":"aGk="}`),
		FileName:       "hi.txt",
		OriginalName:   "hi.txt",
		SizeBytes:      2,
		MIMEType:       "text/plain",
		Source:         "upload_base64",
		ContentHash:    "sha256:8f434346648f6b96df89dda901c5176b10a6d83961dd3c1ac88b59b2dc327aa4",
	}
	first, hit, err := service.CreateFile(ctx, request)
	if err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}
	if hit {
		t.Fatal("first create returned idempotency hit")
	}
	if first.FileURL != "" || first.DownloadURL != "" || !first.Transient || first.ExpiresAt == nil {
		t.Fatalf("file storage fields = %#v", first)
	}
	if !strings.HasPrefix(first.ContentHash, "sha256:") {
		t.Fatalf("content hash = %q", first.ContentHash)
	}
	second, hit, err := service.CreateFile(ctx, request)
	if err != nil {
		t.Fatalf("CreateFile() duplicate error = %v", err)
	}
	if !hit || second.ID != first.ID {
		t.Fatalf("duplicate file = (%q, %v), want (%q, true)", second.ID, hit, first.ID)
	}
}

func TestFileServiceRejectsUnsafeSourceURL(t *testing.T) {
	ctx := context.Background()
	guard, err := egressguard.New(egressguard.Config{})
	if err != nil {
		t.Fatalf("egressguard.New() error = %v", err)
	}
	service := NewFileService(NewMemoryRepository(), 0, WithFileEgressGuard(guard))
	_, _, err = service.CreateFile(ctx, FileCreateRequest{
		TenantID:    "tenant",
		ProjectID:   "project",
		APIKeyID:    "key",
		RequestID:   "req_1",
		Endpoint:    "/v1/files/upload/url",
		RequestBody: []byte(`{"url":"http://127.0.0.1/input.png"}`),
		FileName:    "input.png",
		Source:      "upload_url",
		SourceURL:   "http://127.0.0.1/input.png",
	})
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodeInvalidArgument {
		t.Fatalf("error = %v, want invalid_argument", err)
	}
}
