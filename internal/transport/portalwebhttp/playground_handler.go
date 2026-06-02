package portalwebhttp

import (
	"net/http"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
)

func (h *Handler) runPlayground(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if err := h.requireCSRF(r, sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	var request portalapp.PlaygroundRunRequest
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.portal.RunPlayground(r.Context(), sr.principal, request)
	writeResult(w, sr.requestID, response, err)
}
