package portalhttp

import (
	"net/http"
	"strings"
)

func (h *Handler) listModels(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	response, err := h.portal.ListModels(r.Context(), principal)
	writeResult(w, state.RequestID, response, err)
}

func (h *Handler) getModelSchema(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	modelName := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/portal/models/"), "/schema")
	response, err := h.portal.GetModelSchema(r.Context(), principal, modelName)
	writeResult(w, state.RequestID, response, err)
}
