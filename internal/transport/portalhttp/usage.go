package portalhttp

import (
	"net/http"
	"strings"
	"time"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) getUsage(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	from, to, ok := parseTimeRange(w, state.RequestID, r)
	if !ok {
		return
	}
	limit, ok := parseLimit(w, state.RequestID, r)
	if !ok {
		return
	}
	response, err := h.portal.Usage(r.Context(), principal, portalapp.UsageFilter{
		Currency: r.URL.Query().Get("currency"),
		From:     from,
		To:       to,
		Limit:    limit,
	})
	writeResult(w, state.RequestID, response, err)
}

func parseTimeRange(w http.ResponseWriter, requestID string, r *http.Request) (time.Time, time.Time, bool) {
	var from, to time.Time
	var err error
	if value := strings.TrimSpace(r.URL.Query().Get("from")); value != "" {
		from, err = time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, requestID, apperr.InvalidArgument("from must be RFC3339"))
			return time.Time{}, time.Time{}, false
		}
	}
	if value := strings.TrimSpace(r.URL.Query().Get("to")); value != "" {
		to, err = time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, requestID, apperr.InvalidArgument("to must be RFC3339"))
			return time.Time{}, time.Time{}, false
		}
	}
	return from, to, true
}
