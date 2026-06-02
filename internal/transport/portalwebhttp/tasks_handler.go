package portalwebhttp

import (
	"net/http"
	"strings"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	limit, ok := parseLimit(w, sr.requestID, r)
	if !ok {
		return
	}
	response, err := h.portal.ListTasks(r.Context(), sr.principal, r.URL.Query().Get("status"), limit, r.URL.Query().Get("cursor"))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) taskByID(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	taskID := strings.TrimPrefix(r.URL.Path, "/api/portal/v1/tasks/")
	if strings.Contains(taskID, "/") {
		writeError(w, sr.requestID, apperr.NotFound("portal task route not found"))
		return
	}
	response, err := h.portal.GetTask(r.Context(), sr.principal, taskID)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) taskAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !strings.HasSuffix(r.URL.Path, "/cancel") {
		writeError(w, sr.requestID, apperr.NotFound("portal task route not found"))
		return
	}
	if err := h.requireCSRF(r, sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	writeError(w, sr.requestID, apperr.New(apperr.CodeFeatureNotEnabled, "task cancel is not implemented", http.StatusNotImplemented))
}

func (h *Handler) notImplemented(message string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/portal/v1") {
			http.NotFound(w, r)
			return
		}
		writeError(w, requestID(r), apperr.New(apperr.CodeFeatureNotEnabled, message, http.StatusNotImplemented))
	}
}
