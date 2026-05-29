package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	dpsnapshot "github.com/KnifeFly/token-gateway/internal/dataplane/snapshot"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func TestManagerSkipsUnboundPhase(t *testing.T) {
	counter := &countingPlugin{}
	registry := NewRegistry()
	registry.Register(counter)
	manager := NewManager(registry)
	state := &engine.RequestState{Snapshot: mustPluginSnapshot(t, nil)}

	if err := manager.Run(context.Background(), string(PhasePrePrompt), state); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if counter.calls != 0 {
		t.Fatalf("plugin calls = %d, want 0", counter.calls)
	}
}

func TestManagerMapsDenyToPolicyDenied(t *testing.T) {
	registry := NewRegistry()
	registry.Register(denyPlugin{})
	manager := NewManager(registry)
	state := &engine.RequestState{
		Parsed: engine.ParsedRequest{RawBody: []byte(`{"messages":[{"content":"blocked term"}]}`)},
		Snapshot: mustPluginSnapshot(t, []cpsnapshot.PluginBindingRuntime{{
			ID:            "guard",
			Name:          "deny",
			Phase:         string(PhasePrePrompt),
			Priority:      1,
			Enabled:       true,
			FailurePolicy: string(FailurePolicyFailClosed),
			Config:        json.RawMessage(`{}`),
		}}),
	}

	err := manager.Run(context.Background(), string(PhasePrePrompt), state)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Code != apperr.CodePolicyDenied {
		t.Fatalf("Run() error = %v, want policy_denied", err)
	}
}

func TestManagerFailOpenRecordsAuditEvent(t *testing.T) {
	registry := NewRegistry()
	registry.Register(failingPlugin{})
	manager := NewManager(registry)
	state := &engine.RequestState{
		RequestID: "req_1",
		TraceID:   "trace_1",
		Snapshot: mustPluginSnapshot(t, []cpsnapshot.PluginBindingRuntime{{
			ID:            "fail",
			Name:          "failing",
			Phase:         string(PhasePrePrompt),
			Priority:      1,
			Enabled:       true,
			FailurePolicy: string(FailurePolicyFailOpen),
			Config:        json.RawMessage(`{}`),
		}}),
		Internal: map[string]any{},
	}

	if err := manager.Run(context.Background(), string(PhasePrePrompt), state); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	events, _ := state.Internal[auditEventsKey].([]map[string]string)
	if len(events) != 1 || events[0]["action"] != "fail_open" {
		t.Fatalf("audit events = %#v", events)
	}
}

func TestManagerAppliesStateMutations(t *testing.T) {
	registry := NewRegistry()
	registry.Register(mutationPlugin{})
	manager := NewManager(registry)
	state := &engine.RequestState{
		Snapshot: mustPluginSnapshot(t, []cpsnapshot.PluginBindingRuntime{{
			ID:            "mutation",
			Name:          "mutation",
			Phase:         string(PhasePrePrompt),
			Priority:      1,
			Enabled:       true,
			FailurePolicy: string(FailurePolicyFailClosed),
			Config:        json.RawMessage(`{}`),
		}}),
		Metadata: map[string]string{},
		Internal: map[string]any{},
	}

	if err := manager.Run(context.Background(), string(PhasePrePrompt), state); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if state.Metadata["plugin.test"] != "metadata" {
		t.Fatalf("metadata = %#v", state.Metadata)
	}
	if state.Internal["plugin.test"] != "internal" {
		t.Fatalf("internal = %#v", state.Internal)
	}
}

func mustPluginSnapshot(t *testing.T, bindings []cpsnapshot.PluginBindingRuntime) *dpsnapshot.IndexedSnapshot {
	t.Helper()
	snapshot, err := dpsnapshot.Build(cpsnapshot.RuntimeSnapshot{
		Version:        "test",
		CreatedAt:      time.Now(),
		PluginBindings: bindings,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return snapshot
}

type countingPlugin struct {
	calls int
}

func (p *countingPlugin) Name() string {
	return "counting"
}

func (p *countingPlugin) Phase() Phase {
	return PhasePrePrompt
}

func (p *countingPlugin) Validate(json.RawMessage) error {
	return nil
}

func (p *countingPlugin) Execute(context.Context, Input) (Result, error) {
	p.calls++
	return Result{Action: ActionAllow}, nil
}

type failingPlugin struct{}

func (failingPlugin) Name() string {
	return "failing"
}

func (failingPlugin) Phase() Phase {
	return PhasePrePrompt
}

func (failingPlugin) Validate(json.RawMessage) error {
	return nil
}

func (failingPlugin) Execute(context.Context, Input) (Result, error) {
	return Result{}, errors.New("boom")
}

type denyPlugin struct{}

func (denyPlugin) Name() string {
	return "deny"
}

func (denyPlugin) Phase() Phase {
	return PhasePrePrompt
}

func (denyPlugin) Validate(json.RawMessage) error {
	return nil
}

func (denyPlugin) Execute(context.Context, Input) (Result, error) {
	return Result{Action: ActionDeny, Message: "blocked"}, nil
}

type mutationPlugin struct{}

func (mutationPlugin) Name() string {
	return "mutation"
}

func (mutationPlugin) Phase() Phase {
	return PhasePrePrompt
}

func (mutationPlugin) Validate(json.RawMessage) error {
	return nil
}

func (mutationPlugin) Execute(context.Context, Input) (Result, error) {
	return Result{Mutations: []StateMutation{
		{Target: MutationMetadata, Key: "plugin.test", Value: "metadata"},
		{Target: MutationInternal, Key: "plugin.test", Value: "internal"},
	}}, nil
}
