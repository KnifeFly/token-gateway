package portalhttp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/portal"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// Handler serves customer-facing Portal APIs with API-key scoped permissions.
type Handler struct {
	snapshot engine.SnapshotProvider
	auth     engine.Authenticator
	portal   *portal.Service
	logger   *slog.Logger
}

// NewHandler returns a Portal route registrar.
func NewHandler(snapshot engine.SnapshotProvider, auth engine.Authenticator, portalService *portal.Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{snapshot: snapshot, auth: auth, portal: portalService, logger: logger}
}

// Register adds Portal routes to the shared data-plane mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/portal/models", h.listModels)
	mux.HandleFunc("GET /v1/portal/models/", h.getModelSchema)
	mux.HandleFunc("GET /v1/portal/credits", h.getCredits)
	mux.HandleFunc("GET /v1/portal/usage", h.getUsage)
	mux.HandleFunc("GET /v1/portal/api-keys", h.listAPIKeys)
	mux.HandleFunc("POST /v1/portal/api-keys", h.createAPIKey)
	mux.HandleFunc("POST /v1/portal/api-keys/", h.disableAPIKey)
	mux.HandleFunc("GET /v1/portal/tasks", h.listTasks)
	mux.HandleFunc("GET /v1/portal/tasks/", h.getTask)
}

func (h *Handler) listModels(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	response, err := h.portal.ListModels(state.Snapshot, principal)
	writeResult(w, state.RequestID, response, err)
}

func (h *Handler) getModelSchema(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	modelName := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/portal/models/"), "/schema")
	response, err := h.portal.GetModelSchema(state.Snapshot, principal, modelName)
	writeResult(w, state.RequestID, response, err)
}

func (h *Handler) getCredits(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	response, err := h.portal.Credits(r.Context(), principal, r.URL.Query().Get("currency"))
	writeResult(w, state.RequestID, response, err)
}

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
	response, err := h.portal.Usage(r.Context(), principal, reporting.TenantUsageFilter{
		Currency: r.URL.Query().Get("currency"),
		From:     from,
		To:       to,
		Limit:    limit,
	})
	writeResult(w, state.RequestID, response, err)
}

func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	response, err := h.portal.ListAPIKeys(r.Context(), principal)
	writeResult(w, state.RequestID, response, err)
}

func (h *Handler) createAPIKey(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	var request portal.APIKeyCreateRequest
	if !decodeJSON(w, state.RequestID, r, &request) {
		return
	}
	response, err := h.portal.CreateAPIKey(r.Context(), principal, request)
	writeResult(w, state.RequestID, response, err)
}

func (h *Handler) disableAPIKey(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	keyID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/portal/api-keys/"), "/disable")
	response, err := h.portal.DisableAPIKey(r.Context(), principal, keyID)
	writeResult(w, state.RequestID, response, err)
}

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	state, principal, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	limit, ok := parseLimit(w, state.RequestID, r)
	if !ok {
		return
	}
	response, err := h.portal.ListTasks(r.Context(), principal, r.URL.Query().Get("status"), limit, r.URL.Query().Get("cursor"))
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

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request) (*engine.RequestState, portal.Principal, bool) {
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
		return nil, portal.Principal{}, false
	}
	if err := h.snapshot.Attach(r.Context(), state); err != nil {
		writeError(w, state.RequestID, err)
		return nil, portal.Principal{}, false
	}
	if err := h.auth.Authenticate(r.Context(), state); err != nil {
		writeError(w, state.RequestID, err)
		return nil, portal.Principal{}, false
	}
	return state, portal.Principal{
		TenantID:      state.TenantID,
		ProjectID:     state.ProjectID,
		APIKeyID:      state.APIKeyID,
		AllowedModels: append([]string(nil), state.Principal.AllowedModels...),
	}, true
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

func writeResult(w http.ResponseWriter, requestID string, result any, err error) {
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, requestID string, err error) {
	status := http.StatusInternalServerError
	code := string(apperr.CodeInternal)
	message := "internal error"
	errType := "service_error"
	retryable := false
	if appErr, ok := apperr.As(err); ok {
		status = appErr.HTTPStatus
		code = string(appErr.Code)
		message = appErr.SafeMessage()
		retryable = appErr.Temporary
		errType = "invalid_request_error"
		if status >= 500 {
			errType = "service_error"
		}
		if status == http.StatusUnauthorized {
			errType = "authentication_error"
		}
		if status == http.StatusForbidden {
			errType = "permission_error"
		}
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":       code,
			"message":    message,
			"type":       errType,
			"request_id": requestID,
			"retryable":  retryable,
		},
	})
}
