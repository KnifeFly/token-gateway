package portalhttp

import (
	"net/http"
	"strings"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
)

func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	response, err := h.portal.ListAPIKeys(r.Context(), principal)
	writeResult(w, state.RequestID, response, err)
}

func (h *Handler) createAPIKey(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	var request portalapp.APIKeyCreateRequest
	if !decodeJSON(w, state.RequestID, r, &request) {
		return
	}
	response, err := h.portal.CreateAPIKey(r.Context(), principal, request)
	writeResult(w, state.RequestID, response, err)
}

func (h *Handler) disableAPIKey(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	keyID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/portal/api-keys/"), "/disable")
	response, err := h.portal.DisableAPIKey(r.Context(), principal, keyID)
	writeResult(w, state.RequestID, response, err)
}
