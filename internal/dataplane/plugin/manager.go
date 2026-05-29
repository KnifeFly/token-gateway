package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/KnifeFly/token-gateway/pkg/redaction"
)

const auditEventsKey = "audit_events"

// Manager executes snapshot-bound built-in plugin chains.
type Manager struct {
	registry *Registry
	resolver Resolver
}

// NewManager returns a data-plane plugin manager.
func NewManager(registry *Registry) *Manager {
	if registry == nil {
		registry = NewRegistry()
	}
	return &Manager{registry: registry, resolver: Resolver{}}
}

// Run executes all bindings for phase. Empty phases skip through one map lookup.
func (m *Manager) Run(ctx context.Context, phase string, state *engine.RequestState) error {
	if m == nil || state == nil || state.Snapshot == nil {
		return nil
	}
	currentPhase := Phase(phase)
	if !ValidPhase(currentPhase) {
		return nil
	}
	bindings := state.Snapshot.LookupPluginBindings(phase)
	if len(bindings) == 0 {
		return nil
	}
	for _, binding := range m.resolver.Resolve(currentPhase, state, bindings) {
		if err := m.executeBinding(ctx, currentPhase, state, binding); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) executeBinding(ctx context.Context, phase Phase, state *engine.RequestState, binding engine.PluginBindingView) error {
	impl, ok := m.registry.Lookup(binding.Name)
	if !ok {
		return m.handlePluginError(state, binding, fmt.Errorf("plugin %q is not registered", binding.Name))
	}
	config := binding.Config
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	if err := impl.Validate(config); err != nil {
		return m.handlePluginError(state, binding, err)
	}
	result, err := impl.Execute(ctx, Input{
		Phase:   phase,
		State:   state,
		Binding: binding,
		Config:  config,
	})
	if err != nil {
		return m.handlePluginError(state, binding, err)
	}
	return applyResult(state, result)
}

func (m *Manager) handlePluginError(state *engine.RequestState, binding engine.PluginBindingView, err error) error {
	if FailurePolicy(binding.FailurePolicy) == FailurePolicyFailOpen {
		appendAuditEvent(state, map[string]string{
			"plugin": binding.Name,
			"phase":  binding.Phase,
			"error":  err.Error(),
			"action": "fail_open",
		})
		return nil
	}
	if appErr, ok := apperr.As(err); ok {
		return appErr
	}
	return apperr.ConfigUnavailable("plugin execution failed", apperr.WithCause(err))
}

func applyResult(state *engine.RequestState, result Result) error {
	if result.Metadata != nil {
		if state.Metadata == nil {
			state.Metadata = map[string]string{}
		}
		for key, value := range result.Metadata {
			state.Metadata[key] = value
		}
	}
	if result.SuggestedModel != "" {
		if state.Metadata == nil {
			state.Metadata = map[string]string{}
		}
		state.Metadata["plugin.suggested_model"] = result.SuggestedModel
	}
	if len(result.AuditFields) > 0 {
		appendAuditEvent(state, result.AuditFields)
	}
	switch result.Action {
	case "", ActionAllow, ActionRedact, ActionAudit, ActionDegrade:
		return nil
	case ActionDeny:
		message := result.Message
		if message == "" {
			message = "request denied by policy"
		}
		return apperr.PolicyDenied(message)
	default:
		return apperr.ConfigUnavailable("plugin returned unsupported action", apperr.WithCause(errors.New(string(result.Action))))
	}
}

func appendAuditEvent(state *engine.RequestState, fields map[string]string) {
	if state == nil || len(fields) == 0 {
		return
	}
	event := make(map[string]string, len(fields)+2)
	event["request_id"] = state.RequestID
	event["trace_id"] = state.TraceID
	for key, value := range fields {
		event[key] = redaction.RedactPII(value)
	}
	if state.Internal == nil {
		state.Internal = map[string]any{}
	}
	existing, _ := state.Internal[auditEventsKey].([]map[string]string)
	state.Internal[auditEventsKey] = append(existing, event)
}
