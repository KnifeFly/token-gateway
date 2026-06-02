package adminhttp

import (
	"net/http"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) listOperators(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListOperators(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) createOperator(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request adminapp.OperatorCreateRequest
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.CreateOperator(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) operatorAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	operatorID, action, ok := parseActionPath(r.URL.Path, "/api/admin/v1/operators/")
	if !ok || action != "disable" {
		writeError(w, sr.requestID, apperr.NotFound("admin operator route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.DisableOperator(r.Context(), sr.actor, operatorID, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}
