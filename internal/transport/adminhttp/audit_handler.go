package adminhttp

import (
	"net/http"
)

func (h *Handler) listAudit(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	filter, ok := parseAuditFilter(w, sr.requestID, r)
	if !ok {
		return
	}
	response, err := h.admin.ListAuditEvents(r.Context(), sr.actor, filter)
	writeResult(w, sr.requestID, response, err)
}
