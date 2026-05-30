package task

import (
	"context"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

func TestHTTPProviderTaskDispatcherUsesRegisteredAdapter(t *testing.T) {
	ctx := context.Background()
	adapter := &fakeProviderTaskAdapter{}
	dispatcher := NewHTTPProviderTaskDispatcher(nil, nil, staticChannelResolver{
		channel: engine.ChannelView{ID: "channel_custom", ProviderType: "custom_media", Enabled: true, BaseURL: "https://provider.example"},
	})
	dispatcher.RegisterAdapter("custom_media", adapter)

	submitted, err := dispatcher.Submit(ctx, ProviderTaskRequest{
		Task: Task{ID: "task_1", Kind: KindImageGeneration},
		Candidate: engine.ProviderCandidate{
			ProviderType: "custom_media",
			ChannelID:    "channel_custom",
			PublicModel:  "image-public",
		},
		Channel:   engine.ChannelView{ID: "channel_custom", ProviderType: "custom_media", Enabled: true, BaseURL: "https://provider.example"},
		RequestID: "req_1",
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if submitted.ExternalID != "custom_task" || adapter.submits != 1 {
		t.Fatalf("submitted = %#v submits = %d", submitted, adapter.submits)
	}

	result, err := dispatcher.Poll(ctx, Task{ProviderType: "custom_media", ChannelID: "channel_custom", ProviderTaskID: "custom_task"})
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if result.Status != StatusSucceeded || adapter.polls != 1 {
		t.Fatalf("result = %#v polls = %d", result, adapter.polls)
	}
	if err := dispatcher.Cancel(ctx, Task{ProviderType: "custom_media", ChannelID: "channel_custom", ProviderTaskID: "custom_task"}); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if adapter.cancels != 1 {
		t.Fatalf("cancels = %d", adapter.cancels)
	}
}

type fakeProviderTaskAdapter struct {
	submits int
	polls   int
	cancels int
}

func (a *fakeProviderTaskAdapter) Submit(context.Context, ProviderTaskRequest) (*ProviderTask, error) {
	a.submits++
	return &ProviderTask{ExternalID: "custom_task", Status: StatusRunning, Progress: 1}, nil
}

func (a *fakeProviderTaskAdapter) Poll(context.Context, Task, engine.ChannelView) (*ProviderTaskResult, error) {
	a.polls++
	return &ProviderTaskResult{Status: StatusSucceeded, Progress: 100}, nil
}

func (a *fakeProviderTaskAdapter) Cancel(context.Context, Task, engine.ChannelView) error {
	a.cancels++
	return nil
}

type staticChannelResolver struct {
	channel engine.ChannelView
}

func (r staticChannelResolver) ResolveProviderChannel(context.Context, string) (engine.ChannelView, bool, error) {
	return r.channel, true, nil
}
