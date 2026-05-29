package plugin

import (
	"sort"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

// Resolver filters plugin bindings by request scope and returns execution order.
type Resolver struct{}

// Resolve applies tenant/project/model matching and stable specificity ordering.
func (Resolver) Resolve(_ Phase, state *engine.RequestState, bindings []engine.PluginBindingView) []engine.PluginBindingView {
	if len(bindings) == 0 {
		return nil
	}
	matches := make([]engine.PluginBindingView, 0, len(bindings))
	for _, binding := range bindings {
		if !binding.Enabled || !bindingMatches(state, binding) {
			continue
		}
		matches = append(matches, binding)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		left := bindingSpecificity(matches[i])
		right := bindingSpecificity(matches[j])
		if left != right {
			return left > right
		}
		if matches[i].Priority != matches[j].Priority {
			return matches[i].Priority < matches[j].Priority
		}
		if matches[i].Name != matches[j].Name {
			return matches[i].Name < matches[j].Name
		}
		return matches[i].ID < matches[j].ID
	})
	return matches
}

func bindingMatches(state *engine.RequestState, binding engine.PluginBindingView) bool {
	if binding.TenantID != "" && (state == nil || state.TenantID != binding.TenantID) {
		return false
	}
	if binding.ProjectID != "" && (state == nil || state.ProjectID != binding.ProjectID) {
		return false
	}
	model := requestModel(state)
	if binding.Model != "" && binding.Model != model {
		return false
	}
	return true
}

func bindingSpecificity(binding engine.PluginBindingView) int {
	score := 0
	if binding.TenantID != "" {
		score++
	}
	if binding.ProjectID != "" {
		score++
	}
	if binding.Model != "" {
		score++
	}
	return score
}

func requestModel(state *engine.RequestState) string {
	if state == nil {
		return ""
	}
	if state.ResolvedModel.PublicModel != "" {
		return state.ResolvedModel.PublicModel
	}
	if state.RequestedModel != "" {
		return state.RequestedModel
	}
	return state.Parsed.Model
}
