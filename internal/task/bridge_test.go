package task

import (
	"context"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
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

func TestBridgeCreateAndDispatchReportsIdempotencyHit(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, 0)
	dispatcher := &submitRecordingDispatcher{}
	bridge := NewBridge(service, dispatcher)
	_, _, err := service.CreateMediaTask(ctx, CreateTaskRequest{
		TenantID:       "tenant_1",
		ProjectID:      "project_1",
		APIKeyID:       "key_1",
		RequestID:      "req_original",
		Endpoint:       "/v1/images/generations",
		IdempotencyKey: "idem_1",
		Kind:           KindImageGeneration,
		MediaType:      "image",
		Model:          "image-public",
		Input:          []byte(`{"model":"image-public","prompt":"hi"}`),
		BalanceHoldID:  "hold_original",
	})
	if err != nil {
		t.Fatalf("CreateMediaTask() error = %v", err)
	}

	response, hit, err := bridge.CreateAndDispatch(ctx, bridgeMediaState("req_replay", "hold_replay"))
	if err != nil {
		t.Fatalf("CreateAndDispatch() error = %v", err)
	}
	if !hit {
		t.Fatal("hit = false, want true")
	}
	if response == nil {
		t.Fatal("missing response")
	}
	if dispatcher.submits != 0 {
		t.Fatalf("provider submits = %d, want 0", dispatcher.submits)
	}
}

func TestBridgeSettlesTerminalSubmitBeforeCompletingTask(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, 0)
	settlement := &bridgeRecordingSettlement{}
	dispatcher := &submitRecordingDispatcher{task: &ProviderTask{
		ExternalID: "external_1",
		Status:     StatusSucceeded,
		Progress:   100,
		Result:     []byte(`{"results":["https://provider.example/result.png"]}`),
		Usage:      tokenusage.Actual{InputTokens: 2, OutputTokens: 3, TotalTokens: 5},
	}}
	bridge := NewBridge(service, dispatcher, settlement)
	state := bridgeMediaState("req_terminal", "hold_terminal")
	state.IdempotencyKey = ""

	response, hit, err := bridge.CreateAndDispatch(ctx, state)
	if err != nil {
		t.Fatalf("CreateAndDispatch() error = %v", err)
	}
	if hit {
		t.Fatal("hit = true, want false")
	}
	if response == nil {
		t.Fatal("missing response")
	}
	if len(settlement.tasks) != 1 {
		t.Fatalf("settlement tasks = %#v", settlement.tasks)
	}
	if settlement.tasks[0].Status != StatusSucceeded || settlement.usages[0].TotalTokens != 5 {
		t.Fatalf("settlement task = %#v usage = %#v", settlement.tasks[0], settlement.usages[0])
	}
	task, err := service.GetTask(ctx, "tenant_1", "project_1", settlement.tasks[0].ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if task.Status != StatusSucceeded || task.Usage.TotalTokens != 5 {
		t.Fatalf("task = %#v", task)
	}
}

func TestBridgePinsAsyncTaskPriceSnapshot(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, 0)
	dispatcher := &submitRecordingDispatcher{}
	bridge := NewBridge(service, dispatcher).WithDefaultPrice(pricing.TokenPrice{Currency: "USD", InputMicrosPerToken: 7, OutputMicrosPerToken: 11})
	state := bridgeMediaState("req_price", "hold_price")
	state.IdempotencyKey = ""
	state.SnapshotRef = engine.SnapshotRef{Version: "snap_1"}
	state.PriceRule = engine.PriceRuleView{
		PublicModel:           "image-public",
		Currency:              "CNY",
		InputMicrosPerToken:   13,
		OutputMicrosPerToken:  17,
		EstimatedOutputTokens: 42,
		Enabled:               true,
	}
	state.EstimatedUsage = tokenusage.Estimate{InputTokens: 3, OutputTokens: 42}
	state.EstimatedChargeMicros = 753
	state.RoutePlan.PolicyID = "route_1"

	if _, _, err := bridge.CreateAndDispatch(ctx, state); err != nil {
		t.Fatalf("CreateAndDispatch() error = %v", err)
	}
	if len(dispatcher.requests) != 1 {
		t.Fatalf("requests = %#v", dispatcher.requests)
	}
	snapshot := dispatcher.requests[0].Task.PriceSnapshot
	if snapshot.Currency != "CNY" || snapshot.InputMicrosPerToken != 13 || snapshot.OutputMicrosPerToken != 17 {
		t.Fatalf("price snapshot = %#v", snapshot)
	}
	if snapshot.EstimatedOutputTokens != 42 || snapshot.EstimatedChargeMicros != 753 || snapshot.RouteSnapshotVersion != "snap_1" || snapshot.RoutePolicyID != "route_1" {
		t.Fatalf("price snapshot audit fields = %#v", snapshot)
	}
}

func TestBridgeFallsBackWhenAsyncSubmitFailsBeforeExternalTask(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, 0)
	dispatcher := &submitRecordingDispatcher{
		errors: []error{apperr.ServiceUnavailable("temporary submit failure", apperr.WithTemporary())},
		tasks:  []*ProviderTask{nil, &ProviderTask{ExternalID: "external_2", Status: StatusRunning, Progress: 1}},
	}
	bridge := NewBridge(service, dispatcher)
	state := bridgeMediaState("req_fallback", "hold_fallback")
	state.IdempotencyKey = ""
	state.RoutePlan = &engine.RoutePlan{Candidates: []engine.ProviderCandidate{
		{ProviderType: "mock_media", ChannelID: "channel_1", PublicModel: "image-public"},
		{ProviderType: "mock_media", ChannelID: "channel_2", PublicModel: "image-public"},
	}}

	if _, _, err := bridge.CreateAndDispatch(ctx, state); err != nil {
		t.Fatalf("CreateAndDispatch() error = %v", err)
	}
	if dispatcher.submits != 2 {
		t.Fatalf("submits = %d, want 2", dispatcher.submits)
	}
	if dispatcher.requests[1].Candidate.ChannelID != "channel_2" {
		t.Fatalf("requests = %#v", dispatcher.requests)
	}
	if len(state.Attempts) != 2 || !state.Attempts[1].Success || state.Attempts[1].FallbackFromChannelID != "channel_1" {
		t.Fatalf("attempts = %#v", state.Attempts)
	}
	tasks, err := repo.ListTasks(ctx, TaskListFilter{TenantID: "tenant_1", ProjectID: "project_1", Limit: 10})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(tasks) != 1 || tasks[0].ChannelID != "channel_2" || tasks[0].ProviderTaskID != "external_2" {
		t.Fatalf("tasks = %#v", tasks)
	}
}

type bridgeRecordingSettlement struct {
	tasks  []Task
	usages []tokenusage.Actual
}

func (s *bridgeRecordingSettlement) Settle(_ context.Context, task Task, usage tokenusage.Actual) error {
	s.tasks = append(s.tasks, task)
	s.usages = append(s.usages, usage)
	return nil
}

func (s *bridgeRecordingSettlement) RecordFailed(context.Context, Task, tokenusage.Actual, error) error {
	return nil
}

type submitRecordingDispatcher struct {
	submits  int
	task     *ProviderTask
	tasks    []*ProviderTask
	errors   []error
	requests []ProviderTaskRequest
}

func (d *submitRecordingDispatcher) Submit(_ context.Context, request ProviderTaskRequest) (*ProviderTask, error) {
	d.requests = append(d.requests, request)
	index := d.submits
	d.submits++
	if index < len(d.errors) && d.errors[index] != nil {
		return nil, d.errors[index]
	}
	if index < len(d.tasks) && d.tasks[index] != nil {
		return d.tasks[index], nil
	}
	if d.task != nil {
		return d.task, nil
	}
	return &ProviderTask{ExternalID: "external_1", Status: StatusRunning, Progress: 1}, nil
}

func (d *submitRecordingDispatcher) Poll(context.Context, Task) (*ProviderTaskResult, error) {
	return nil, nil
}

func (d *submitRecordingDispatcher) Cancel(context.Context, Task) error {
	return nil
}

func bridgeMediaState(requestID, holdID string) *engine.RequestState {
	return &engine.RequestState{
		RequestID:      requestID,
		TenantID:       "tenant_1",
		ProjectID:      "project_1",
		APIKeyID:       "key_1",
		RequestedModel: "image-public",
		IdempotencyKey: "idem_1",
		BalanceHoldID:  holdID,
		Endpoint:       engine.EndpointSpec{Path: "/v1/images/generations"},
		Snapshot:       bridgeSnapshot{},
		RoutePlan:      &engine.RoutePlan{Candidates: []engine.ProviderCandidate{{ProviderType: "mock_media", ChannelID: "channel_1", PublicModel: "image-public"}}},
		Parsed: engine.ParsedRequest{
			RawBody: []byte(`{"model":"image-public","prompt":"hi"}`),
			Media: &engine.UnifiedMediaRequest{
				Kind:      string(KindImageGeneration),
				MediaType: "image",
				Model:     "image-public",
			},
		},
	}
}

type bridgeSnapshot struct{}

func (bridgeSnapshot) Ref() engine.SnapshotRef        { return engine.SnapshotRef{Version: "test"} }
func (bridgeSnapshot) ListModels() []engine.ModelView { return nil }
func (bridgeSnapshot) LookupAPIKeyHash(string) (engine.APIKeyView, bool) {
	return engine.APIKeyView{}, false
}
func (bridgeSnapshot) LookupModel(string) (engine.ModelView, bool) {
	return engine.ModelView{}, false
}
func (bridgeSnapshot) LookupRoute(string) (engine.RoutePolicyView, bool) {
	return engine.RoutePolicyView{}, false
}
func (bridgeSnapshot) LookupChannel(channelID string) (engine.ChannelView, bool) {
	if channelID != "channel_1" && channelID != "channel_2" {
		return engine.ChannelView{}, false
	}
	return engine.ChannelView{ID: channelID, ProviderType: "mock_media", Enabled: true}, true
}
func (bridgeSnapshot) LookupPrice(string) (engine.PriceRuleView, bool) {
	return engine.PriceRuleView{}, false
}
func (bridgeSnapshot) LookupLimit(string) (engine.LimitRuleView, bool) {
	return engine.LimitRuleView{}, false
}
func (bridgeSnapshot) LookupLimits(engine.LimitScope) []engine.LimitRuleView  { return nil }
func (bridgeSnapshot) LookupPluginBindings(string) []engine.PluginBindingView { return nil }
func (bridgeSnapshot) IsAPIKeyRevoked(string) bool                            { return false }
