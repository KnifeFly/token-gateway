package task

import (
	"context"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

func TestBridgeCancelSettlesCanceledTask(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, 0)
	settlement := &bridgeRecordingSettlement{}
	bridge := NewBridge(service, NewMockProviderTaskDispatcher(), settlement)

	created, _, err := service.CreateMediaTask(ctx, CreateTaskRequest{
		TenantID:      "tenant_1",
		ProjectID:     "project_1",
		APIKeyID:      "key_1",
		RequestID:     "req_1",
		Endpoint:      "/v1/videos/generations",
		Kind:          KindVideoGeneration,
		MediaType:     "video",
		Model:         "seedance-2.0-text-to-video",
		Input:         []byte(`{"prompt":"hi"}`),
		BalanceHoldID: "hold_1",
	})
	if err != nil {
		t.Fatalf("CreateMediaTask() error = %v", err)
	}
	if _, err := service.MarkDispatched(ctx, created.ID, "mock_media", "channel_mock_media", "external_1"); err != nil {
		t.Fatalf("MarkDispatched() error = %v", err)
	}

	response, err := bridge.HandleTaskOperation(ctx, &engine.RequestState{
		TenantID:  "tenant_1",
		ProjectID: "project_1",
		Parsed: engine.ParsedRequest{Task: &engine.TaskRequest{
			Operation: engine.TaskOperationCancel,
			TaskID:    created.ID,
		}},
	})
	if err != nil {
		t.Fatalf("HandleTaskOperation() error = %v", err)
	}
	if response == nil {
		t.Fatal("missing response")
	}
	if len(settlement.tasks) != 1 || settlement.tasks[0].Status != StatusCanceled {
		t.Fatalf("settlement tasks = %#v", settlement.tasks)
	}
}

type bridgeRecordingSettlement struct {
	tasks []Task
}

func (s *bridgeRecordingSettlement) Settle(_ context.Context, task Task, _ tokenusage.Actual) error {
	s.tasks = append(s.tasks, task)
	return nil
}

func (s *bridgeRecordingSettlement) RecordFailed(context.Context, Task, tokenusage.Actual, error) error {
	return nil
}
