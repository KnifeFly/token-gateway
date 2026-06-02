package portalwebhttp

import (
	"net/http"
	"strings"
	"time"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) usage(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	filter, ok := parseUsageFilter(w, sr.requestID, r)
	if !ok {
		return
	}
	response, err := h.portal.Usage(r.Context(), sr.principal, filter)
	writeResult(w, sr.requestID, response, err)
}

func parseUsageFilter(w http.ResponseWriter, requestID string, r *http.Request) (portalapp.UsageFilter, bool) {
	var filter portalapp.UsageFilter
	filter.Currency = r.URL.Query().Get("currency")
	limit, err := parseLimitValue(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, requestID, err)
		return portalapp.UsageFilter{}, false
	}
	filter.Limit = limit
	if value := strings.TrimSpace(r.URL.Query().Get("from")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, requestID, apperr.InvalidArgument("from must be RFC3339"))
			return portalapp.UsageFilter{}, false
		}
		filter.From = parsed
	}
	if value := strings.TrimSpace(r.URL.Query().Get("to")); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, requestID, apperr.InvalidArgument("to must be RFC3339"))
			return portalapp.UsageFilter{}, false
		}
		filter.To = parsed
	}
	return filter, true
}
