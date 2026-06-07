package portalhttp

import (
	"net/http"
)

func (h *Handler) getCredits(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	response, err := h.portal.Credits(r.Context(), principal, r.URL.Query().Get("currency"))
	writeResult(w, state.RequestID, response, err)
}
