package portalwebhttp

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func parseLimit(w http.ResponseWriter, requestID string, r *http.Request) (int, bool) {
	limit, err := parseLimitValue(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, requestID, err)
		return 0, false
	}
	return limit, true
}

func parseLimitValue(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	var limit int
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, apperr.InvalidArgument("limit must be between 1 and 200")
		}
		limit = limit*10 + int(ch-'0')
	}
	if limit <= 0 || limit > 200 {
		return 0, apperr.InvalidArgument("limit must be between 1 and 200")
	}
	return limit, nil
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
