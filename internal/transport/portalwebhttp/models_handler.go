package portalwebhttp

import (
	"net/http"
	"strings"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) models(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.ListModels(r.Context(), sr.principal)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) modelSchema(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	value := strings.TrimPrefix(r.URL.Path, "/api/portal/v1/models/")
	if !strings.HasSuffix(value, "/schema") {
		model := strings.Trim(value, "/")
		response, err := h.portal.GetModelDetail(r.Context(), sr.principal, model)
		writeResult(w, sr.requestID, response, err)
		return
	}
	model := strings.TrimSuffix(value, "/schema")
	if model == "" {
		writeError(w, sr.requestID, apperr.NotFound("portal model route not found"))
		return
	}
	response, err := h.portal.GetModelSchema(r.Context(), sr.principal, model)
	writeResult(w, sr.requestID, response, err)
}
