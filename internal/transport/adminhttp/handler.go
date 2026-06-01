package adminhttp

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	adminservice "github.com/KnifeFly/token-gateway/internal/app/admin/service"
	cpadmin "github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

const (
	sessionCookieName = "tg_admin_session"
	csrfHeaderName    = "X-CSRF-Token"
	reasonHeaderName  = "X-Reason"
)

// Handler owns the Admin browser BFF routes.
type Handler struct {
	admin  *adminservice.Service
	logger *slog.Logger
}

// NewHandler builds an Admin Web BFF route registrar without an attached service.
func NewHandler(logger *slog.Logger) *Handler {
	return NewHandlerWithService(nil, logger)
}

// NewHandlerWithService builds an Admin Web BFF route registrar.
func NewHandlerWithService(adminService *adminservice.Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{admin: adminService, logger: logger}
}

// Register adds Admin Web BFF routes to the console mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/admin/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/admin/v1/auth/logout", h.withSession(h.logout))
	mux.HandleFunc("GET /api/admin/v1/auth/me", h.withSession(h.me))
	mux.HandleFunc("GET /api/admin/v1/dashboard", h.withSession(h.dashboard))

	mux.HandleFunc("GET /api/admin/v1/tenants", h.withSession(h.listTenants))
	mux.HandleFunc("POST /api/admin/v1/tenants", h.withSession(h.upsertTenant))
	mux.HandleFunc("GET /api/admin/v1/tenants/", h.withSession(h.getTenant))
	mux.HandleFunc("GET /api/admin/v1/projects", h.withSession(h.listProjects))
	mux.HandleFunc("POST /api/admin/v1/projects", h.withSession(h.upsertProject))

	mux.HandleFunc("GET /api/admin/v1/api-keys", h.withSession(h.listAPIKeys))
	mux.HandleFunc("POST /api/admin/v1/api-keys", h.withSession(h.createAPIKey))
	mux.HandleFunc("POST /api/admin/v1/api-keys/", h.withSession(h.apiKeyAction))

	mux.HandleFunc("GET /api/admin/v1/models", h.withSession(h.listModels))
	mux.HandleFunc("POST /api/admin/v1/models", h.withSession(h.upsertModel))
	mux.HandleFunc("GET /api/admin/v1/channels", h.withSession(h.listChannels))
	mux.HandleFunc("POST /api/admin/v1/channels", h.withSession(h.upsertChannel))
	mux.HandleFunc("POST /api/admin/v1/channels/", h.withSession(h.channelAction))
	mux.HandleFunc("GET /api/admin/v1/routes", h.withSession(h.listRoutes))
	mux.HandleFunc("POST /api/admin/v1/routes", h.withSession(h.upsertRoute))
	mux.HandleFunc("GET /api/admin/v1/pricing", h.withSession(h.listPricing))
	mux.HandleFunc("POST /api/admin/v1/pricing", h.withSession(h.upsertPrice))
	mux.HandleFunc("GET /api/admin/v1/limits", h.withSession(h.listLimits))
	mux.HandleFunc("POST /api/admin/v1/limits", h.withSession(h.upsertLimit))

	mux.HandleFunc("GET /api/admin/v1/snapshots", h.withSession(h.snapshots))
	mux.HandleFunc("POST /api/admin/v1/snapshots/validate", h.withSession(h.validateSnapshot))
	mux.HandleFunc("POST /api/admin/v1/snapshots/publish", h.withSession(h.publishSnapshot))
	mux.HandleFunc("POST /api/admin/v1/snapshots/rollback", h.withSession(h.rollbackSnapshot))

	mux.HandleFunc("GET /api/admin/v1/operations/settlements", h.withSession(h.listSettlements))
	mux.HandleFunc("POST /api/admin/v1/operations/settlements/", h.withSession(h.settlementAction))
	mux.HandleFunc("GET /api/admin/v1/operations/callbacks", h.withSession(h.listCallbacks))
	mux.HandleFunc("POST /api/admin/v1/operations/callbacks/", h.withSession(h.callbackAction))
	mux.HandleFunc("GET /api/admin/v1/operations/workers", h.withSession(h.listWorkers))
	mux.HandleFunc("GET /api/admin/v1/operations/holds", h.withSession(h.listHolds))

	mux.HandleFunc("GET /api/admin/v1/audit", h.withSession(h.listAudit))
	mux.HandleFunc("GET /api/admin/v1/operators", h.withSession(h.listOperators))
	mux.HandleFunc("POST /api/admin/v1/operators", h.withSession(h.createOperator))
	mux.HandleFunc("POST /api/admin/v1/operators/", h.withSession(h.operatorAction))
	mux.HandleFunc("/api/admin/v1", h.notFound)
	mux.HandleFunc("/api/admin/v1/", h.notFound)
}

type sessionRequest struct {
	session   adminapp.Session
	actor     adminapp.Actor
	requestID string
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	if h.admin == nil {
		writeError(w, requestID, apperr.ConfigUnavailable("admin web service is unavailable"))
		return
	}
	var request adminapp.LoginRequest
	if !decodeJSON(w, requestID, r, &request) {
		return
	}
	response, err := h.admin.Login(r.Context(), request, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		writeError(w, requestID, err)
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
	if err := h.admin.Logout(r.Context(), sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.SessionResponse(r.Context(), sr.session.ID)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.Dashboard(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listTenants(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListTenants(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) getTenant(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	tenantID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/v1/tenants/"), "/")
	response, err := h.admin.GetTenant(r.Context(), sr.actor, tenantID)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertTenant(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request cpadmin.Tenant
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertTenant(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListProjects(r.Context(), sr.actor, r.URL.Query().Get("tenant_id"))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertProject(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request cpadmin.Project
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertProject(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListAPIKeys(r.Context(), sr.actor, r.URL.Query().Get("tenant_id"), r.URL.Query().Get("project_id"))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) createAPIKey(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request cpadmin.APIKey
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.CreateAPIKey(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) apiKeyAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	keyID, action, ok := parseActionPath(r.URL.Path, "/api/admin/v1/api-keys/")
	if !ok || action != "disable" {
		writeError(w, sr.requestID, apperr.NotFound("admin api key route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.DisableAPIKey(r.Context(), sr.actor, keyID, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listModels(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListModels(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertModel(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request cpadmin.ModelConfig
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertModel(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listChannels(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListChannels(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertChannel(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request cpadmin.ChannelConfig
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertChannel(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) channelAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	channelID, action, ok := parseActionPath(r.URL.Path, "/api/admin/v1/channels/")
	if !ok {
		writeError(w, sr.requestID, apperr.NotFound("admin channel route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	switch action {
	case "enable":
		response, err := h.admin.SetChannelEnabled(r.Context(), sr.actor, channelID, true, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "disable":
		response, err := h.admin.SetChannelEnabled(r.Context(), sr.actor, channelID, false, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "test":
		writeError(w, sr.requestID, apperr.FeatureNotEnabled("channel test workflow is unavailable"))
	default:
		writeError(w, sr.requestID, apperr.NotFound("admin channel route not found"))
	}
}

func (h *Handler) listRoutes(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListRoutes(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertRoute(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request cpadmin.RoutePolicyConfig
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertRoute(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listPricing(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListPricing(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertPrice(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request cpadmin.PriceRuleConfig
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertPrice(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listLimits(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListLimits(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertLimit(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request cpadmin.LimitRuleConfig
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertLimit(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) snapshots(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.SnapshotDiagnostics(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) validateSnapshot(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.ValidateSnapshot(r.Context(), sr.actor, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) publishSnapshot(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.PublishSnapshot(r.Context(), sr.actor, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) rollbackSnapshot(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.RollbackSnapshot(r.Context(), sr.actor, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listSettlements(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListFailedSettlements(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) settlementAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	settlementID, action, ok := parseActionPath(r.URL.Path, "/api/admin/v1/operations/settlements/")
	if !ok || action != "replay" {
		writeError(w, sr.requestID, apperr.NotFound("admin settlement route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.ReplayFailedSettlement(r.Context(), sr.actor, settlementID, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listCallbacks(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListCallbacks(r.Context(), sr.actor, queryLimit(r))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) callbackAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	callbackID, action, ok := parseActionPath(r.URL.Path, "/api/admin/v1/operations/callbacks/")
	if !ok || action != "retry" {
		writeError(w, sr.requestID, apperr.NotFound("admin callback route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.RetryCallback(r.Context(), sr.actor, callbackID, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listWorkers(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListWorkers(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listHolds(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListHolds(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listAudit(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	filter, ok := parseAuditFilter(w, sr.requestID, r)
	if !ok {
		return
	}
	response, err := h.admin.ListAuditEvents(r.Context(), sr.actor, filter)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) listOperators(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListOperators(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) createOperator(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request adminapp.OperatorCreateRequest
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.CreateOperator(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) operatorAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	operatorID, action, ok := parseActionPath(r.URL.Path, "/api/admin/v1/operators/")
	if !ok || action != "disable" {
		writeError(w, sr.requestID, apperr.NotFound("admin operator route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	response, err := h.admin.DisableOperator(r.Context(), sr.actor, operatorID, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) withSession(next func(http.ResponseWriter, *http.Request, sessionRequest)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := requestID(r)
		if h.admin == nil {
			writeError(w, requestID, apperr.ConfigUnavailable("admin web service is unavailable"))
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeError(w, requestID, apperr.Unauthorized("admin session is required"))
			return
		}
		session, actor, err := h.admin.Session(r.Context(), cookie.Value)
		if err != nil {
			writeError(w, requestID, err)
			return
		}
		next(w, r, sessionRequest{session: session, actor: actor, requestID: requestID})
	}
}

func (h *Handler) requireMutation(w http.ResponseWriter, r *http.Request, sr sessionRequest) bool {
	if err := h.requireCSRF(r, sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return false
	}
	return true
}

func (h *Handler) requireCSRF(r *http.Request, sessionID string) error {
	return h.admin.ValidateCSRF(r.Context(), sessionID, r.Header.Get(csrfHeaderName))
}

func (h *Handler) mutationOptions(r *http.Request, requestID string) adminapp.MutationOptions {
	return adminapp.MutationOptions{
		RequestID:      requestID,
		IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		Reason:         strings.TrimSpace(r.Header.Get(reasonHeaderName)),
		RemoteAddr:     r.RemoteAddr,
		UserAgent:      r.UserAgent(),
	}
}

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

func setSessionCookie(w http.ResponseWriter, r *http.Request, session adminapp.SessionResponse) {
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
