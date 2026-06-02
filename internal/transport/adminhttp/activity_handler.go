package adminhttp

import (
	"net/http"
	"strings"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) listUsageLogs(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	filter, ok := parseUsageLogFilter(w, sr.requestID, r)
	if !ok {
		return
	}
	response, err := h.admin.ListUsageLogs(r.Context(), sr.actor, filter)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) usageLogByRequestID(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	requestID := strings.TrimPrefix(r.URL.Path, "/api/admin/v1/usage-logs/")
	if requestID == "" || strings.Contains(requestID, "/") {
		writeError(w, sr.requestID, apperr.NotFound("admin usage log route not found"))
		return
	}
	response, err := h.admin.GetUsageLog(r.Context(), sr.actor, requestID)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listTaskLogs(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	filter, ok := parseTaskLogFilter(w, sr.requestID, r)
	if !ok {
		return
	}
	response, err := h.admin.ListTaskLogs(r.Context(), sr.actor, filter)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) taskLogByTaskID(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	taskID := strings.TrimPrefix(r.URL.Path, "/api/admin/v1/task-logs/")
	if taskID == "" || strings.Contains(taskID, "/") {
		writeError(w, sr.requestID, apperr.NotFound("admin task log route not found"))
		return
	}
	response, err := h.admin.GetTaskLog(r.Context(), sr.actor, taskID)
	writeResult(w, sr.requestID, response, err)
}

func parseUsageLogFilter(w http.ResponseWriter, requestID string, r *http.Request) (adminapp.UsageLogFilter, bool) {
	filter := adminapp.UsageLogFilter{
		TenantID:     strings.TrimSpace(r.URL.Query().Get("tenant_id")),
		ProjectID:    strings.TrimSpace(r.URL.Query().Get("project_id")),
		APIKeyID:     strings.TrimSpace(r.URL.Query().Get("api_key_id")),
		RequestID:    strings.TrimSpace(r.URL.Query().Get("request_id")),
		Model:        strings.TrimSpace(r.URL.Query().Get("model")),
		ProviderType: strings.TrimSpace(r.URL.Query().Get("provider_type")),
		ChannelID:    strings.TrimSpace(r.URL.Query().Get("channel_id")),
		Status:       strings.TrimSpace(r.URL.Query().Get("status")),
		Currency:     strings.TrimSpace(r.URL.Query().Get("currency")),
		Limit:        queryLimit(r),
	}
	if !parseActivityTimeRange(w, requestID, r, &filter.From, &filter.To) {
		return adminapp.UsageLogFilter{}, false
	}
	return filter, true
}

func parseTaskLogFilter(w http.ResponseWriter, requestID string, r *http.Request) (adminapp.TaskLogFilter, bool) {
	filter := adminapp.TaskLogFilter{
		TaskID:       strings.TrimSpace(r.URL.Query().Get("task_id")),
		TenantID:     strings.TrimSpace(r.URL.Query().Get("tenant_id")),
		ProjectID:    strings.TrimSpace(r.URL.Query().Get("project_id")),
		APIKeyID:     strings.TrimSpace(r.URL.Query().Get("api_key_id")),
		RequestID:    strings.TrimSpace(r.URL.Query().Get("request_id")),
		Model:        strings.TrimSpace(r.URL.Query().Get("model")),
		ProviderType: strings.TrimSpace(r.URL.Query().Get("provider_type")),
		ChannelID:    strings.TrimSpace(r.URL.Query().Get("channel_id")),
		Status:       strings.TrimSpace(r.URL.Query().Get("status")),
		Cursor:       strings.TrimSpace(r.URL.Query().Get("cursor")),
		Limit:        queryLimit(r),
	}
	if !parseActivityTimeRange(w, requestID, r, &filter.From, &filter.To) {
		return adminapp.TaskLogFilter{}, false
	}
	return filter, true
}

func parseActivityTimeRange(w http.ResponseWriter, requestID string, r *http.Request, from *time.Time, to *time.Time) bool {
	if value := strings.TrimSpace(r.URL.Query().Get("from")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, requestID, apperr.InvalidArgument("from must be RFC3339"))
			return false
		}
		*from = parsed
	}
	if value := strings.TrimSpace(r.URL.Query().Get("to")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, requestID, apperr.InvalidArgument("to must be RFC3339"))
			return false
		}
		*to = parsed
	}
	return true
}
