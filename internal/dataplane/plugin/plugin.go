package plugin

import (
	"context"
	"encoding/json"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

// Action describes the high-level effect requested by a plugin.
type Action string

const (
	// ActionAllow permits the request to continue.
	ActionAllow Action = "allow"
	// ActionDeny blocks the request.
	ActionDeny Action = "deny"
	// ActionRedact indicates the plugin changed sensitive payload content.
	ActionRedact Action = "redact"
	// ActionAudit emits audit metadata without changing flow.
	ActionAudit Action = "audit"
	// ActionDegrade suggests a cheaper or safer model.
	ActionDegrade Action = "degrade"
)

// MutationTarget identifies the request-state surface a plugin may mutate.
type MutationTarget string

const (
	// MutationMetadata writes a key into request metadata.
	MutationMetadata MutationTarget = "metadata"
	// MutationInternal writes a key into internal request state.
	MutationInternal MutationTarget = "internal"
	// MutationRequestedModel changes the requested public model.
	MutationRequestedModel MutationTarget = "requested_model"
)

// FailurePolicy controls how plugin execution errors affect the request.
type FailurePolicy string

const (
	// FailurePolicyFailClosed makes plugin errors block the request.
	FailurePolicyFailClosed FailurePolicy = "fail_closed"
	// FailurePolicyFailOpen makes plugin errors auditable but non-blocking.
	FailurePolicyFailOpen FailurePolicy = "fail_open"
)

// Plugin is the interface implemented by built-in data-plane plugins.
type Plugin interface {
	Name() string
	Phase() Phase
	Validate(config json.RawMessage) error
	Execute(ctx context.Context, input Input) (Result, error)
}

// Input is passed to each plugin execution.
type Input struct {
	Phase   Phase
	State   *engine.RequestState
	Binding engine.PluginBindingView
	Config  json.RawMessage
}

// Result is the normalized output of one plugin execution.
type Result struct {
	Action         Action
	ErrorCode      string
	Message        string
	SuggestedModel string
	Mutations      []StateMutation
	AuditFields    map[string]string
	Metadata       map[string]string
}

// StateMutation explicitly records one plugin-requested state mutation.
type StateMutation struct {
	Target MutationTarget
	Key    string
	Value  string
}

// Registry stores built-in plugins by name.
type Registry struct {
	plugins map[string]Plugin
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{plugins: map[string]Plugin{}}
}

// Register adds a built-in plugin implementation.
func (r *Registry) Register(plugin Plugin) {
	if r == nil || plugin == nil || plugin.Name() == "" {
		return
	}
	r.plugins[plugin.Name()] = plugin
}

// Lookup returns a plugin by name.
func (r *Registry) Lookup(name string) (Plugin, bool) {
	if r == nil {
		return nil, false
	}
	plugin, ok := r.plugins[name]
	return plugin, ok
}
