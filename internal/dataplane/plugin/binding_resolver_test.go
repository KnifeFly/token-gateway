package plugin

import (
	"reflect"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

func TestResolverOrdersBySpecificityPriorityAndName(t *testing.T) {
	state := &engine.RequestState{
		TenantID:       "tenant_1",
		ProjectID:      "project_1",
		RequestedModel: "gpt-4o-mini",
	}
	bindings := []engine.PluginBindingView{
		{ID: "global", Name: "z", Phase: string(PhasePrePrompt), Priority: 1, Enabled: true},
		{ID: "tenant", Name: "a", Phase: string(PhasePrePrompt), TenantID: "tenant_1", Priority: 50, Enabled: true},
		{ID: "wrong-tenant", Name: "a", Phase: string(PhasePrePrompt), TenantID: "tenant_2", Priority: 1, Enabled: true},
		{ID: "project-b", Name: "b", Phase: string(PhasePrePrompt), TenantID: "tenant_1", ProjectID: "project_1", Priority: 20, Enabled: true},
		{ID: "project-a", Name: "a", Phase: string(PhasePrePrompt), TenantID: "tenant_1", ProjectID: "project_1", Priority: 20, Enabled: true},
		{ID: "model", Name: "m", Phase: string(PhasePrePrompt), TenantID: "tenant_1", ProjectID: "project_1", Model: "gpt-4o-mini", Priority: 100, Enabled: true},
	}

	got := Resolver{}.Resolve(PhasePrePrompt, state, bindings)
	var ids []string
	for _, binding := range got {
		ids = append(ids, binding.ID)
	}
	want := []string{"model", "project-a", "project-b", "tenant", "global"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("order = %#v, want %#v", ids, want)
	}
}
