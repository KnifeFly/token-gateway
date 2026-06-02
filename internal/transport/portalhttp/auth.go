package portalhttp

import (
	"net/http"
	"time"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request) (*engine.RequestState, portalapp.Principal, bool) {
	state := &engine.RequestState{
		RequestID: requestID(r),
		StartedAt: time.Now().UTC(),
		Incoming: engine.IncomingRequest{
			Method:     r.Method,
			Path:       r.URL.Path,
			RawQuery:   r.URL.RawQuery,
			Header:     r.Header.Clone(),
			RemoteAddr: r.RemoteAddr,
		},
		Metadata: make(map[string]string),
		Internal: make(map[string]any),
	}
	if h.snapshot == nil || h.auth == nil || h.portal == nil {
		writeError(w, state.RequestID, apperr.ConfigUnavailable("portal service is unavailable"))
		return nil, portalapp.Principal{}, false
	}
	if err := h.snapshot.Attach(r.Context(), state); err != nil {
		writeError(w, state.RequestID, err)
		return nil, portalapp.Principal{}, false
	}
	if err := h.auth.Authenticate(r.Context(), state); err != nil {
		writeError(w, state.RequestID, err)
		return nil, portalapp.Principal{}, false
	}
	return state, portalapp.Principal{
		TenantID:      state.TenantID,
		ProjectID:     state.ProjectID,
		APIKeyID:      state.APIKeyID,
		AllowedModels: append([]string(nil), state.Principal.AllowedModels...),
	}, true
}
