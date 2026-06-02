package adminhttp

import (
	"net/http"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) listSettlements(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListFailedSettlements(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) settlementAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	settlementID, action, ok := parseActionPath(r.URL.Path, "/api/admin/v1/operations/settlements/")
	if !ok || action != "replay" {
		writeError(w, sr.requestID, apperr.NotFound("admin settlement route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.ReplayFailedSettlement(r.Context(), sr.actor, settlementID, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listCallbacks(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListCallbacks(r.Context(), sr.actor, queryLimit(r))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) callbackAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	callbackID, action, ok := parseActionPath(r.URL.Path, "/api/admin/v1/operations/callbacks/")
	if !ok || action != "retry" {
		writeError(w, sr.requestID, apperr.NotFound("admin callback route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.RetryCallback(r.Context(), sr.actor, callbackID, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listWorkers(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListWorkers(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listHolds(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListHolds(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}
