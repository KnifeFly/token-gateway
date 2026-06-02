package portalhttp

import (
	"net/http"
	"strings"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
)

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	limit, ok := parseLimit(w, state.RequestID, r)
	if !ok {
		return
	}
	response, err := h.portal.ListTasks(r.Context(), principal, portalapp.TaskFilter{
		Status: r.URL.Query().Get("status"),
		Cursor: r.URL.Query().Get("cursor"),
		Limit:  limit,
	})
	writeResult(w, state.RequestID, response, err)
}

func (h *Handler) getTask(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	taskID := strings.TrimPrefix(r.URL.Path, "/v1/portal/tasks/")
	response, err := h.portal.GetTask(r.Context(), principal, taskID)
	writeResult(w, state.RequestID, response, err)
}
