package portalwebhttp

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	portalservice "github.com/KnifeFly/token-gateway/internal/app/portal/service"
	legacyportal "github.com/KnifeFly/token-gateway/internal/portal"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

const (
	sessionCookieName = "tg_portal_session"
	csrfHeaderName    = "X-CSRF-Token"
)

// Handler owns the Portal browser BFF routes.
type Handler struct {
	portal *portalservice.Service
	logger *slog.Logger
}

// NewHandler builds a Portal Web BFF route registrar.
func NewHandler(portalService *portalservice.Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{portal: portalService, logger: logger}
}

// Register adds Portal Web BFF routes to the console mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/portal/v1/auth/api-key-login", h.login)
	mux.HandleFunc("POST /api/portal/v1/auth/logout", h.withSession(h.logout))
	mux.HandleFunc("GET /api/portal/v1/auth/me", h.withSession(h.me))
	mux.HandleFunc("GET /api/portal/v1/dashboard", h.withSession(h.dashboard))
	mux.HandleFunc("GET /api/portal/v1/onboarding", h.withSession(h.onboarding))
	mux.HandleFunc("GET /api/portal/v1/models", h.withSession(h.models))
	mux.HandleFunc("GET /api/portal/v1/models/", h.withSession(h.modelSchema))
	mux.HandleFunc("GET /api/portal/v1/credits", h.withSession(h.credits))
	mux.HandleFunc("GET /api/portal/v1/usage", h.withSession(h.usage))
	mux.HandleFunc("GET /api/portal/v1/usage/export", h.notImplemented("usage export is reserved"))
	mux.HandleFunc("GET /api/portal/v1/api-keys", h.withSession(h.listAPIKeys))
	mux.HandleFunc("POST /api/portal/v1/api-keys", h.withSession(h.createAPIKey))
	mux.HandleFunc("POST /api/portal/v1/api-keys/", h.withSession(h.apiKeyAction))
	mux.HandleFunc("GET /api/portal/v1/tasks", h.withSession(h.listTasks))
	mux.HandleFunc("GET /api/portal/v1/tasks/", h.withSession(h.taskByID))
	mux.HandleFunc("POST /api/portal/v1/tasks/", h.withSession(h.taskAction))
	mux.HandleFunc("GET /api/portal/v1/settings/project", h.withSession(h.projectSettings))
	mux.HandleFunc("/api/portal/v1", h.notImplemented("portal web BFF route is not implemented"))
	mux.HandleFunc("/api/portal/v1/", h.notImplemented("portal web BFF route is not implemented"))
}

type sessionRequest struct {
	session   portalapp.Session
	principal portalapp.Principal
	requestID string
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if h.portal == nil {
		writeError(w, requestID(r), apperr.ConfigUnavailable("portal web service is unavailable"))
		return
	}
	var request portalapp.APIKeyLoginRequest
	if !decodeJSON(w, requestID(r), r, &request) {
		return
	}
	response, err := h.portal.LoginWithAPIKey(r.Context(), request, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		writeError(w, requestID(r), err)
		return
	}
	setSessionCookie(w, r, response.Session)
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if err := h.requireCSRF(r, sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	if err := h.portal.Logout(r.Context(), sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.SessionResponse(r.Context(), sr.session.ID)
	if err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.Dashboard(r.Context(), sr.principal)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) onboarding(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.Onboarding(r.Context(), sr.principal)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) models(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.ListModels(r.Context(), sr.principal)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) modelSchema(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	value := strings.TrimPrefix(r.URL.Path, "/api/portal/v1/models/")
	if !strings.HasSuffix(value, "/schema") {
		writeError(w, sr.requestID, apperr.NotFound("portal model route not found"))
		return
	}
	model := strings.TrimSuffix(value, "/schema")
	response, err := h.portal.GetModelSchema(r.Context(), sr.principal, model)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) credits(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.Credits(r.Context(), sr.principal, r.URL.Query().Get("currency"))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) usage(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	filter, ok := parseUsageFilter(w, sr.requestID, r)
	if !ok {
		return
	}
	response, err := h.portal.Usage(r.Context(), sr.principal, filter)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.ListAPIKeys(r.Context(), sr.principal)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) createAPIKey(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if err := h.requireCSRF(r, sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	var request legacyportal.APIKeyCreateRequest
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.portal.CreateAPIKey(r.Context(), sr.principal, request)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) apiKeyAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !strings.HasSuffix(r.URL.Path, "/disable") {
		writeError(w, sr.requestID, apperr.NotFound("portal api key route not found"))
		return
	}
	if err := h.requireCSRF(r, sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	keyID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/portal/v1/api-keys/"), "/disable")
	response, err := h.portal.DisableAPIKey(r.Context(), sr.principal, keyID)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	limit, ok := parseLimit(w, sr.requestID, r)
	if !ok {
		return
	}
	response, err := h.portal.ListTasks(r.Context(), sr.principal, r.URL.Query().Get("status"), limit, r.URL.Query().Get("cursor"))
	writeResult(w, sr.requestID, response, err)
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

func (h *Handler) projectSettings(w http.ResponseWriter, _ *http.Request, sr sessionRequest) {
	writeJSON(w, http.StatusOK, h.portal.ProjectSettings(sr.principal))
}

func (h *Handler) withSession(next func(http.ResponseWriter, *http.Request, sessionRequest)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := requestID(r)
		if h.portal == nil {
			writeError(w, requestID, apperr.ConfigUnavailable("portal web service is unavailable"))
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeError(w, requestID, apperr.Unauthorized("portal session is required"))
			return
		}
		session, principal, err := h.portal.Session(r.Context(), cookie.Value)
		if err != nil {
			writeError(w, requestID, err)
			return
		}
		next(w, r, sessionRequest{session: session, principal: principal, requestID: requestID})
	}
}

func (h *Handler) requireCSRF(r *http.Request, sessionID string) error {
	return h.portal.ValidateCSRF(r.Context(), sessionID, r.Header.Get(csrfHeaderName))
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

func setSessionCookie(w http.ResponseWriter, r *http.Request, session portalapp.SessionResponse) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.SessionID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookie(r),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookie(r),
	})
}

func secureCookie(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func requestID(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Request-ID")); value != "" {
		return value
	}
	return "req_" + time.Now().UTC().Format("20060102150405.000000000")
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
		errType = string(appErr.Code)
		retryable = appErr.Temporary
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":       code,
			"message":    message,
			"type":       errType,
			"retryable":  retryable,
			"request_id": requestID,
		},
	})
}
