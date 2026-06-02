package adminhttp

import (
	"net/http"

	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListAPIKeys(r.Context(), sr.actor, r.URL.Query().Get("tenant_id"), r.URL.Query().Get("project_id"))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) createAPIKey(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request configadmin.APIKey
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.CreateAPIKey(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) apiKeyAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	keyID, action, ok := parseActionPath(r.URL.Path, "/api/admin/v1/api-keys/")
	if !ok || action != "disable" {
		writeError(w, sr.requestID, apperr.NotFound("admin api key route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.DisableAPIKey(r.Context(), sr.actor, keyID, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}
