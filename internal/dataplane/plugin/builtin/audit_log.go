package builtin

import (
	"context"
	"encoding/json"

	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
)

// AuditLog emits a redacted audit event into request-local state.
type AuditLog struct{}

type auditLogConfig struct {
	Enabled bool `json:"enabled"`
}

// Name returns the audit log plugin name.
func (AuditLog) Name() string {
	return "audit_log"
}

// Phase returns the audit phase for audit logging.
func (AuditLog) Phase() plugin.Phase {
	return plugin.PhaseAudit
}

// Validate verifies audit log plugin configuration.
func (AuditLog) Validate(config json.RawMessage) error {
	var cfg auditLogConfig
	return decodeConfig(config, &cfg)
}

// Execute emits redacted audit fields into the plugin result.
func (AuditLog) Execute(_ context.Context, input plugin.Input) (plugin.Result, error) {
	if input.State == nil {
		return plugin.Result{}, nil
	}
	fields := map[string]string{
		"plugin":        "audit_log",
		"tenant_id":     input.State.TenantID,
		"project_id":    input.State.ProjectID,
		"api_key_id":    input.State.APIKeyID,
		"model":         input.State.RequestedModel,
		"canonical_api": string(input.State.CanonicalAPI),
		"snapshot":      input.State.SnapshotRef.Version,
	}
	return plugin.Result{Action: plugin.ActionAudit, AuditFields: fields}, nil
}
