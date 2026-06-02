package portalwebhttp

import (
	"net/http"
)

func (h *Handler) credits(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.Credits(r.Context(), sr.principal, r.URL.Query().Get("currency"))
	writeResult(w, sr.requestID, response, err)
}
