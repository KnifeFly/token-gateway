package portalwebhttp

import (
	"net/http"
)

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.Dashboard(r.Context(), sr.principal)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) onboarding(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.Onboarding(r.Context(), sr.principal)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) projectSettings(w http.ResponseWriter, _ *http.Request, sr sessionRequest) {
	writeJSON(w, http.StatusOK, h.portal.ProjectSettings(sr.principal))
}
