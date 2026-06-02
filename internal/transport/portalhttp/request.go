package portalhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func parseLimit(w http.ResponseWriter, requestID string, r *http.Request) (int, bool) {
	value := strings.TrimSpace(r.URL.Query().Get("limit"))
	if value == "" {
		return 0, true
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 || limit > 200 {
		writeError(w, requestID, apperr.InvalidArgument("limit must be between 1 and 200"))
		return 0, false
	}
	return limit, true
}

func decodeJSON(w http.ResponseWriter, requestID string, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, requestID, apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err)))
		return false
	}
	return true
}

func requestID(r *http.Request) string {
	if value := r.Header.Get("X-Request-ID"); value != "" {
		return value
	}
	if value := r.Header.Get("X-Request-Id"); value != "" {
		return value
	}
	return fmt.Sprintf("req_%d", time.Now().UTC().UnixNano())
}
