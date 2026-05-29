package plugin

// Phase identifies one configured plugin execution point.
type Phase string

const (
	PhasePreRequest    Phase = "pre_request"
	PhasePostAuth      Phase = "post_auth"
	PhasePrePrompt     Phase = "pre_prompt"
	PhasePreRoute      Phase = "pre_route"
	PhasePostRoute     Phase = "post_route"
	PhasePreProvider   Phase = "pre_provider"
	PhasePostProvider  Phase = "post_provider"
	PhasePreSettlement Phase = "pre_settlement"
	PhaseAudit         Phase = "audit"
)

// ValidPhase reports whether phase is active in the MVP plugin chain.
func ValidPhase(phase Phase) bool {
	switch phase {
	case PhasePreRequest, PhasePostAuth, PhasePrePrompt, PhasePreRoute, PhasePostRoute,
		PhasePreProvider, PhasePostProvider, PhasePreSettlement, PhaseAudit:
		return true
	default:
		return false
	}
}
