package jobs

import (
	"context"
	"testing"

	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
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

func TestProviderTaskPollerSettlesFailedTerminalTask(t *testing.T) {
	ctx := context.Background()
	repo := tasksvc.NewMemoryRepository()
	service := tasksvc.NewService(repo, 0)
	dispatcher := staticProviderTaskDispatcher{result: &tasksvc.ProviderTaskResult{
		Status:       tasksvc.StatusFailed,
		Progress:     100,
		ErrorCode:    "provider_failed",
		ErrorMessage: "provider task failed",
	}}
	settlement := &recordingTaskSettlement{}

	created, _, err := service.CreateMediaTask(ctx, tasksvc.CreateTaskRequest{
		TenantID:      "tenant",
		ProjectID:     "project",
		APIKeyID:      "key",
		RequestID:     "req_failed",
		Endpoint:      "/v1/videos/generations",
		Kind:          tasksvc.KindVideoGeneration,
		MediaType:     "video",
		Model:         "seedance-2.0-text-to-video",
		Input:         []byte(`{"model":"seedance-2.0-text-to-video","prompt":"hi"}`),
		BalanceHoldID: "hold_1",
	})
	if err != nil {
		t.Fatalf("CreateMediaTask() error = %v", err)
	}
	if _, err := service.MarkDispatched(ctx, created.ID, "mock_media", "channel_mock_media", "external_1"); err != nil {
		t.Fatalf("MarkDispatched() error = %v", err)
	}

	poller := NewProviderTaskPoller(repo, dispatcher, service, settlement, 0, 10)
	if err := poller.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(settlement.tasks) != 1 || settlement.tasks[0].Status != tasksvc.StatusFailed {
		t.Fatalf("settlement tasks = %#v", settlement.tasks)
	}
	task, ok, err := repo.GetTask(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if !ok || task.Status != tasksvc.StatusFailed {
		t.Fatalf("task = %#v", task)
	}
}

type staticProviderTaskDispatcher struct {
	result *tasksvc.ProviderTaskResult
}

func (d staticProviderTaskDispatcher) Submit(context.Context, tasksvc.ProviderTaskRequest) (*tasksvc.ProviderTask, error) {
	return nil, nil
}

func (d staticProviderTaskDispatcher) Poll(context.Context, tasksvc.Task) (*tasksvc.ProviderTaskResult, error) {
	return d.result, nil
}

func (d staticProviderTaskDispatcher) Cancel(context.Context, tasksvc.Task) error {
	return nil
}

type recordingTaskSettlement struct {
	tasks []tasksvc.Task
}

func (s *recordingTaskSettlement) Settle(_ context.Context, task tasksvc.Task, _ tokenusage.Actual) error {
	s.tasks = append(s.tasks, task)
	return nil
}

func (s *recordingTaskSettlement) RecordFailed(context.Context, tasksvc.Task, tokenusage.Actual, error) error {
	return nil
}
