package portalwebhttp

import (
	"net/http"
	"strings"
	"time"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	filter, ok := parseTaskFilter(w, sr.requestID, r)
	if !ok {
		return
	}
	response, err := h.portal.ListTasks(r.Context(), sr.principal, filter)
	writeResult(w, sr.requestID, response, err)
}

func parseTaskFilter(w http.ResponseWriter, requestID string, r *http.Request) (portalapp.TaskFilter, bool) {
	limit, err := parseLimitValue(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, requestID, err)
		return portalapp.TaskFilter{}, false
	}
	filter := portalapp.TaskFilter{
		APIKeyID:     strings.TrimSpace(r.URL.Query().Get("api_key_id")),
		RequestID:    strings.TrimSpace(r.URL.Query().Get("request_id")),
		Model:        strings.TrimSpace(r.URL.Query().Get("model")),
		ProviderType: strings.TrimSpace(r.URL.Query().Get("provider_type")),
		ChannelID:    strings.TrimSpace(r.URL.Query().Get("channel_id")),
		Status:       strings.TrimSpace(r.URL.Query().Get("status")),
		Cursor:       strings.TrimSpace(r.URL.Query().Get("cursor")),
		Limit:        limit,
	}
	if value := strings.TrimSpace(r.URL.Query().Get("from")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, requestID, apperr.InvalidArgument("from must be RFC3339"))
			return portalapp.TaskFilter{}, false
		}
		filter.From = parsed
	}
	if value := strings.TrimSpace(r.URL.Query().Get("to")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, requestID, apperr.InvalidArgument("to must be RFC3339"))
			return portalapp.TaskFilter{}, false
		}
		filter.To = parsed
	}
	return filter, true
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
