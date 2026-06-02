package adminhttp

import (
	"net/http"

	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
)

func (h *Handler) listModels(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListModels(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertModel(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request configadmin.ModelConfig
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertModel(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}
