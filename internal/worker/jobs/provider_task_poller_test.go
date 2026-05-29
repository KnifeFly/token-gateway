package jobs

import (
	"context"
	"testing"

	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

func TestProviderTaskPollerCompletesTaskAndEnqueuesCallback(t *testing.T) {
	ctx := context.Background()
	repo := tasksvc.NewMemoryRepository()
	service := tasksvc.NewService(repo, 0)
	dispatcher := tasksvc.NewMockProviderTaskDispatcher()

	created, _, err := service.CreateMediaTask(ctx, tasksvc.CreateTaskRequest{
		TenantID:      "tenant",
		ProjectID:     "project",
		APIKeyID:      "key",
		RequestID:     "req",
		Endpoint:      "/v1/videos/generations",
		Kind:          tasksvc.KindVideoGeneration,
		MediaType:     "video",
		Model:         "seedance-2.0-text-to-video",
		Input:         []byte(`{"model":"seedance-2.0-text-to-video","prompt":"hi"}`),
		CallbackURL:   "https://example.com/callback",
		BalanceHoldID: "",
	})
	if err != nil {
		t.Fatalf("CreateMediaTask() error = %v", err)
	}
	if _, err := service.MarkDispatched(ctx, created.ID, "mock_media", "channel_mock_media", "external_1"); err != nil {
		t.Fatalf("MarkDispatched() error = %v", err)
	}

	poller := NewProviderTaskPoller(repo, dispatcher, service, tasksvc.NoopSettlement{}, 0, 10)
	if err := poller.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	task, ok, err := repo.GetTask(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if !ok || task.Status != tasksvc.StatusSucceeded || task.Progress != 100 {
		t.Fatalf("task = %#v", task)
	}
	events, err := repo.ListDueCallbacks(ctx, 10, task.UpdatedAt.Add(1))
	if err != nil {
		t.Fatalf("ListDueCallbacks() error = %v", err)
	}
	if len(events) != 1 || events[0].TaskID != created.ID {
		t.Fatalf("events = %#v", events)
	}
}
