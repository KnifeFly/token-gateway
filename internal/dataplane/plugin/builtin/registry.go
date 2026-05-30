package builtin

import "github.com/KnifeFly/token-gateway/internal/dataplane/plugin"

// Registry returns a registry with all MVP built-in plugins.
func Registry() *plugin.Registry {
	registry := plugin.NewRegistry()
	RegisterAll(registry)
	return registry
}

// RegisterAll registers every MVP built-in plugin.
func RegisterAll(registry *plugin.Registry) {
	registry.Register(RequestSize{})
	registry.Register(IPAllowlist{})
	registry.Register(PromptTokenLimit{})
	registry.Register(ModelACL{})
	registry.Register(RouteOverride{})
	registry.Register(Callback{})
	registry.Register(PIIRedaction{})
	registry.Register(PromptGuard{})
	registry.Register(ResponseGuard{})
	registry.Register(CostGuard{})
	registry.Register(AuditLog{})
	registry.Register(LLMMetric{})
}
