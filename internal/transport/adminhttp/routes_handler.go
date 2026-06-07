package adminhttp

import (
	"net/http"

	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
)

func (h *Handler) listRoutes(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListRoutes(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertRoute(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request configadmin.RoutePolicyConfig
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertRoute(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}
