package adminhttp

import (
	"net/http"
)

func (h *Handler) snapshots(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.SnapshotDiagnostics(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) validateSnapshot(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.ValidateSnapshot(r.Context(), sr.actor, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) publishSnapshot(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.PublishSnapshot(r.Context(), sr.actor, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) rollbackSnapshot(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.RollbackSnapshot(r.Context(), sr.actor, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}
