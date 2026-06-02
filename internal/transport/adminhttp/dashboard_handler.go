package adminhttp

import (
	"net/http"
)

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.Dashboard(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}
