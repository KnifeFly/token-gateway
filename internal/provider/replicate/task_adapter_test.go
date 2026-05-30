package replicate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

func TestTaskAdapterSubmitPollAndCancel(t *testing.T) {
	var createSeen bool
	var cancelSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/predictions":
			var body predictionCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			if body.Version != "replicate-version-1" {
				t.Fatalf("version = %q", body.Version)
			}
			if body.Input["prompt"] != "draw a release candidate" || body.Input["aspect_ratio"] != "16:9" {
				t.Fatalf("input = %#v", body.Input)
			}
			if body.Webhook != "https://hooks.example/task" || len(body.WebhookEventsFilter) != 1 {
				t.Fatalf("webhook = %q events = %#v", body.Webhook, body.WebhookEventsFilter)
			}
			createSeen = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"pred_1","status":"starting"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/predictions/pred_1":
			_, _ = w.Write([]byte(`{
				"id": "pred_1",
				"status": "succeeded",
				"output": ["https://cdn.example/result.png"],
				"urls": {"get":"https://api.replicate.com/v1/predictions/pred_1"},
				"metrics": {"predict_time": 1.25}
			}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/predictions/pred_1/cancel":
			cancelSeen = true
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := NewTaskAdapter(server.Client(), nil)
	channel := engine.ChannelView{
		ID:           "channel_replicate",
		ProviderType: "replicate",
		BaseURL:      server.URL,
		APIKey:       "test-token",
		Enabled:      true,
		Timeout:      time.Second,
	}
	task := tasksvc.Task{
		ID:          "task_1",
		RequestID:   "req_1",
		Kind:        tasksvc.KindImageGeneration,
		Input:       []byte(`{"model":"image-public","prompt":"draw a release candidate","callback_url":"https://client.example/ignored","metadata":{"team":"rc"},"model_params":{"aspect_ratio":"16:9"}}`),
		CallbackURL: "https://hooks.example/task",
	}
	submitted, err := adapter.Submit(context.Background(), tasksvc.ProviderTaskRequest{
		Task: task,
		Candidate: engine.ProviderCandidate{
			ChannelID:     "channel_replicate",
			ProviderType:  "replicate",
			PublicModel:   "image-public",
			UpstreamModel: "replicate-version-1",
		},
		Channel:   channel,
		RequestID: "req_1",
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if !createSeen || submitted.ExternalID != "pred_1" || submitted.Status != tasksvc.StatusRunning {
		t.Fatalf("submitted = %#v createSeen = %v", submitted, createSeen)
	}
	result, err := adapter.Poll(context.Background(), tasksvc.Task{
		ID:             "task_1",
		RequestID:      "req_1",
		ProviderTaskID: "pred_1",
		Input:          task.Input,
	}, channel)
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if result.Status != tasksvc.StatusSucceeded || result.Progress != 100 {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(string(result.Result), "https://cdn.example/result.png") {
		t.Fatalf("result json = %s", string(result.Result))
	}
	if result.Usage.TotalTokens == 0 {
		t.Fatalf("usage was not estimated: %#v", result.Usage)
	}
	if err := adapter.Cancel(context.Background(), tasksvc.Task{ProviderTaskID: "pred_1", RequestID: "req_1"}, channel); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if !cancelSeen {
		t.Fatal("cancel endpoint was not called")
	}
}

func TestReplicateStatusMapping(t *testing.T) {
	cases := map[string]tasksvc.Status{
		"starting":   tasksvc.StatusRunning,
		"processing": tasksvc.StatusRunning,
		"succeeded":  tasksvc.StatusSucceeded,
		"successful": tasksvc.StatusSucceeded,
		"failed":     tasksvc.StatusFailed,
		"canceled":   tasksvc.StatusCanceled,
	}
	for input, want := range cases {
		if got := replicateStatus(input); got != want {
			t.Fatalf("replicateStatus(%q) = %q, want %q", input, got, want)
		}
	}
}
