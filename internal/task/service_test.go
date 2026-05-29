package task

import (
	"context"
	"testing"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
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
	}
	first, hit, err := service.CreateFile(ctx, request)
	if err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}
	if hit {
		t.Fatal("first create returned idempotency hit")
	}
	second, hit, err := service.CreateFile(ctx, request)
	if err != nil {
		t.Fatalf("CreateFile() duplicate error = %v", err)
	}
	if !hit || second.ID != first.ID {
		t.Fatalf("duplicate file = (%q, %v), want (%q, true)", second.ID, hit, first.ID)
	}
}
