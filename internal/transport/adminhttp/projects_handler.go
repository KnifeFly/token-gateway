package adminhttp

import (
	"net/http"

	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
)

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListProjects(r.Context(), sr.actor, r.URL.Query().Get("tenant_id"))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertProject(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request configadmin.Project
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertProject(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}
