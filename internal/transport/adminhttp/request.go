package adminhttp

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/admin/v1") {
		http.NotFound(w, r)
		return
	}
	writeError(w, requestID(r), apperr.NotFound("admin web route not found"))
}

func parseActionPath(path string, prefix string) (string, string, bool) {
	value := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parseAuditFilter(w http.ResponseWriter, requestID string, r *http.Request) (adminapp.AuditFilter, bool) {
	filter := adminapp.AuditFilter{
		OperatorID: strings.TrimSpace(r.URL.Query().Get("operator_id")),
		Action:     strings.TrimSpace(r.URL.Query().Get("action")),
		Resource:   strings.TrimSpace(r.URL.Query().Get("resource")),
		Limit:      queryLimit(r),
	}
	if value := strings.TrimSpace(r.URL.Query().Get("from")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, requestID, apperr.InvalidArgument("from must be RFC3339"))
			return adminapp.AuditFilter{}, false
		}
		filter.From = parsed
	}
	if value := strings.TrimSpace(r.URL.Query().Get("to")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, requestID, apperr.InvalidArgument("to must be RFC3339"))
			return adminapp.AuditFilter{}, false
		}
		filter.To = parsed
	}
	return filter, true
}

func queryLimit(r *http.Request) int {
	value := strings.TrimSpace(r.URL.Query().Get("limit"))
	if value == "" {
		return 0
	}
	limit, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return limit
}

func decodeJSON(w http.ResponseWriter, requestID string, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, requestID, apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err)))
		return false
	}
	return true
}

func requestID(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Request-ID")); value != "" {
		return value
	}
	return "req_" + time.Now().UTC().Format("20060102150405.000000000")
}
