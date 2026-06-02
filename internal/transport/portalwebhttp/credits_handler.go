package portalwebhttp

import (
	"net/http"
)

func (h *Handler) credits(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.Credits(r.Context(), sr.principal, r.URL.Query().Get("currency"))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) creditLedger(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	filter, ok := parseUsageFilter(w, sr.requestID, r)
	if !ok {
		return
	}
	response, err := h.portal.CreditLedger(r.Context(), sr.principal, filter)
	writeResult(w, sr.requestID, response, err)
}
