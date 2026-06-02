package portalwebhttp

import (
	"net/http"
	"strings"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.ListAPIKeys(r.Context(), sr.principal)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) createAPIKey(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if err := h.requireCSRF(r, sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	var request portalapp.APIKeyCreateRequest
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.portal.CreateAPIKey(r.Context(), sr.principal, request)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) apiKeyAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	keyID, action, ok := portalAPIKeyIDAndAction(r.URL.Path)
	if !ok {
		writeError(w, sr.requestID, apperr.NotFound("portal api key route not found"))
		return
	}
	if err := h.requireCSRF(r, sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	switch action {
	case "disable":
		response, err := h.portal.DisableAPIKey(r.Context(), sr.principal, keyID)
		writeResult(w, sr.requestID, response, err)
	case "rotate":
		response, err := h.portal.RotateAPIKey(r.Context(), sr.principal, keyID)
		writeResult(w, sr.requestID, response, err)
	default:
		writeError(w, sr.requestID, apperr.NotFound("portal api key route not found"))
	}
}

func portalAPIKeyIDAndAction(path string) (string, string, bool) {
	value := strings.Trim(strings.TrimPrefix(path, "/api/portal/v1/api-keys/"), "/")
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
