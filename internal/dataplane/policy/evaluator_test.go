package policy

import (
	"context"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

func TestEvaluatorReturnsDenyDegradeAndRouteOverride(t *testing.T) {
	evaluator := NewEvaluator()
	tests := []struct {
		name string
		meta map[string]string
		want engine.PolicyAction
	}{
		{name: "deny", meta: map[string]string{metadataAction: string(engine.PolicyDeny), metadataReason: "blocked"}, want: engine.PolicyDeny},
		{name: "degrade", meta: map[string]string{metadataDegradeModel: "cheap-model"}, want: engine.PolicyDegrade},
		{name: "route_override", meta: map[string]string{
			metadataAction:           string(engine.PolicyRouteOverride),
			metadataOverrideChannel:  "channel_1",
			metadataOverrideProvider: "openai_compatible",
			metadataOverrideModel:    "upstream",
		}, want: engine.PolicyRouteOverride},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := evaluator.Evaluate(context.Background(), &engine.RequestState{
				RequestedModel: "gpt-4o-mini",
				Metadata:       tt.meta,
			})
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if decision.Action != tt.want {
				t.Fatalf("action = %q, want %q", decision.Action, tt.want)
			}
		})
	}
}

func TestEvaluatorAllowsByDefault(t *testing.T) {
	decision, err := NewEvaluator().Evaluate(context.Background(), &engine.RequestState{})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if decision.Action != engine.PolicyAllow {
		t.Fatalf("action = %q", decision.Action)
	}
}
