package plugin

// Phase identifies one configured plugin execution point.
type Phase string

const (
	// PhasePreRequest runs before request classification and authentication.
	PhasePreRequest Phase = "pre_request"
	// PhasePostAuth runs after authentication and before prompt handling.
	PhasePostAuth Phase = "post_auth"
	// PhasePrePrompt runs before request prompt or media payload processing.
	PhasePrePrompt Phase = "pre_prompt"
	// PhasePreRoute runs before route planning.
	PhasePreRoute Phase = "pre_route"
	// PhasePostRoute runs after route planning.
	PhasePostRoute Phase = "post_route"
	// PhasePreProvider runs before provider dispatch.
	PhasePreProvider Phase = "pre_provider"
	// PhasePostProvider runs after provider response handling.
	PhasePostProvider Phase = "post_provider"
	// PhasePreSettlement runs before final billing settlement.
	PhasePreSettlement Phase = "pre_settlement"
	// PhaseAudit runs at the end of request handling.
	PhaseAudit Phase = "audit"
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
