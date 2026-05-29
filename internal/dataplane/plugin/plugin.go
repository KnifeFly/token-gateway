package plugin

import (
	"context"
	"encoding/json"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

// Action describes the high-level effect requested by a plugin.
type Action string

const (
	ActionAllow   Action = "allow"
	ActionDeny    Action = "deny"
	ActionRedact  Action = "redact"
	ActionAudit   Action = "audit"
	ActionDegrade Action = "degrade"
)

// FailurePolicy controls how plugin execution errors affect the request.
type FailurePolicy string

const (
	FailurePolicyFailClosed FailurePolicy = "fail_closed"
	FailurePolicyFailOpen   FailurePolicy = "fail_open"
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
	AuditFields    map[string]string
	Metadata       map[string]string
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
