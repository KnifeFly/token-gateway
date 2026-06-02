package adminhttp

import (
	"net/http"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
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
	var request adminapp.APIKeyCreateRequest
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.CreateAPIKey(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) apiKeyAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	keyID, action, ok := parseActionPath(r.URL.Path, "/api/admin/v1/api-keys/")
	if !ok {
		writeError(w, sr.requestID, apperr.NotFound("admin api key route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	switch action {
	case "update":
		var request adminapp.APIKeyUpdateRequest
		if !decodeJSON(w, sr.requestID, r, &request) {
			return
		}
		response, err := h.admin.UpdateAPIKey(r.Context(), sr.actor, keyID, request, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "enable":
		response, err := h.admin.EnableAPIKey(r.Context(), sr.actor, keyID, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "disable":
		response, err := h.admin.DisableAPIKey(r.Context(), sr.actor, keyID, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "rotate":
		var request adminapp.APIKeyRotateRequest
		if r.Body != http.NoBody && r.ContentLength != 0 && !decodeJSON(w, sr.requestID, r, &request) {
			return
		}
		response, err := h.admin.RotateAPIKey(r.Context(), sr.actor, keyID, request, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	default:
		writeError(w, sr.requestID, apperr.NotFound("admin api key route not found"))
	}
}
