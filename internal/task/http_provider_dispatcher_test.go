package task

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestGenericHTTPProviderTaskAdapterPollNormalizesMediaResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/tasks/external_1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"id": "external_1",
			"status": "succeeded",
			"progress": 100,
			"result_urls": ["https://cdn.example/result.png"],
			"usage": {"input_tokens": 2, "output_tokens": 3},
			"provider_metadata": {"job_id": "external_1"}
		}`))
	}))
	defer server.Close()

	adapter := NewGenericHTTPProviderTaskAdapter(server.Client(), nil)
	result, err := adapter.Poll(context.Background(), Task{
		ID:             "task_1",
		ProviderType:   "generic_media",
		ProviderTaskID: "external_1",
		MediaType:      "image",
	}, engine.ChannelView{BaseURL: server.URL, Enabled: true})
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if result.Status != StatusSucceeded || result.Usage.TotalTokens != 5 {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Assets) != 1 || result.Assets[0].URL != "https://cdn.example/result.png" || result.Assets[0].Provider != "generic_media" {
		t.Fatalf("assets = %#v", result.Assets)
	}
	if result.ProviderMetadata["job_id"] != "external_1" {
		t.Fatalf("provider metadata = %#v", result.ProviderMetadata)
	}
	var payload map[string]any
	if err := json.Unmarshal(result.Result, &payload); err != nil {
		t.Fatalf("result json: %v", err)
	}
	if payload["results"] == nil || payload["assets"] == nil || payload["provider_metadata"] == nil {
		t.Fatalf("payload = %#v", payload)
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
