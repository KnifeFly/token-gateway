package controlhttp

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	redisinfra "github.com/KnifeFly/token-gateway/internal/infra/redis"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

const (
	maxAdminBodyBytes = 1 << 20
	idempotencyTTL    = 24 * time.Hour
)

// Handler serves M5 control-plane admin APIs.
type Handler struct {
	admin      *admin.Service
	publisher  *cpsnapshot.Publisher
	commercial *reporting.Service
	emergency  *redisinfra.EmergencyDisableStore
	token      string
	logger     *slog.Logger
	idem       *idempotencyStore
}

// NewHandler returns a control-plane HTTP handler.
func NewHandler(adminService *admin.Service, publisher *cpsnapshot.Publisher, token string, logger *slog.Logger, commercial ...*reporting.Service) http.Handler {
	return NewHandlerWithEmergency(adminService, publisher, token, logger, nil, commercial...)
}

// NewHandlerWithEmergency returns a control-plane HTTP handler with emergency hot-disable support.
func NewHandlerWithEmergency(adminService *admin.Service, publisher *cpsnapshot.Publisher, token string, logger *slog.Logger, emergency *redisinfra.EmergencyDisableStore, commercial ...*reporting.Service) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &Handler{
		admin:     adminService,
		publisher: publisher,
		emergency: emergency,
		token:     token,
		logger:    logger,
		idem:      newIdempotencyStore(idempotencyTTL),
	}
	if len(commercial) > 0 {
		h.commercial = commercial[0]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /admin/tenants", h.requireAdmin(h.upsertTenant))
	mux.HandleFunc("POST /admin/projects", h.requireAdmin(h.upsertProject))
	mux.HandleFunc("POST /admin/api-keys", h.requireAdmin(h.createAPIKey))
	mux.HandleFunc("GET /admin/api-keys", h.requireAdmin(h.listAPIKeys))
	mux.HandleFunc("POST /admin/api-keys/", h.requireAdmin(h.apiKeyAction))
	mux.HandleFunc("POST /admin/models", h.requireAdmin(h.upsertModel))
	mux.HandleFunc("POST /admin/channels", h.requireAdmin(h.upsertChannel))
	mux.HandleFunc("POST /admin/routes", h.requireAdmin(h.upsertRoute))
	mux.HandleFunc("POST /admin/prices", h.requireAdmin(h.upsertPrice))
	mux.HandleFunc("POST /admin/limits", h.requireAdmin(h.upsertLimit))
	mux.HandleFunc("POST /admin/plugin-bindings", h.requireAdmin(h.upsertPluginBinding))
	mux.HandleFunc("POST /admin/model-marketplace", h.requireAdmin(h.upsertModelMarketplace))
	mux.HandleFunc("GET /admin/model-marketplace", h.requireAdmin(h.listVisibleModels))
	mux.HandleFunc("GET /admin/reports/tenant-usage", h.requireAdmin(h.tenantUsageReport))
	mux.HandleFunc("GET /admin/reports/provider-profit", h.requireAdmin(h.providerProfitReport))
	mux.HandleFunc("GET /admin/reports/reconciliation", h.requireAdmin(h.reconciliationReport))
	mux.HandleFunc("GET /admin/reports/agent-metadata", h.requireAdmin(h.agentMetadataReport))
	mux.HandleFunc("POST /admin/provider-cost-profiles", h.requireAdmin(h.upsertProviderCostProfile))
	mux.HandleFunc("POST /admin/billing/adjustments", h.requireAdmin(h.createManualAdjustment))
	mux.HandleFunc("POST /admin/emergency/providers/", h.requireAdmin(h.providerEmergencyAction))
	mux.HandleFunc("POST /admin/emergency/channels/", h.requireAdmin(h.channelEmergencyAction))
	mux.HandleFunc("POST /admin/snapshots/publish", h.requireAdmin(h.publishSnapshot))
	mux.HandleFunc("POST /admin/snapshots/rollback", h.requireAdmin(h.rollbackSnapshot))
	return mux
}

func (h *Handler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(h.token) == "" {
			writeError(w, apperr.ConfigUnavailable("admin token is required"))
			return
		}
		if !constantTimeTokenEqual(adminToken(r), h.token) {
			writeError(w, apperr.Unauthorized("invalid admin token"))
			return
		}
		if r.ContentLength > maxAdminBodyBytes {
			writeError(w, requestBodyTooLargeError(nil))
			if isControlWrite(r) {
				h.logControlAudit(r, http.StatusRequestEntityTooLarge)
			}
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxAdminBodyBytes)
		}
		if isControlWrite(r) {
			h.handleControlWrite(w, r, next)
			return
		}
		next(w, r)
	}
}

func (h *Handler) handleControlWrite(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if key := strings.TrimSpace(r.Header.Get("Idempotency-Key")); key != "" {
		h.handleIdempotentControlWrite(w, r, next, key)
		return
	}
	recorder := newResponseCapture(w)
	next(recorder, r)
	h.logControlAudit(r, recorder.statusCode())
}

func (h *Handler) handleIdempotentControlWrite(w http.ResponseWriter, r *http.Request, next http.HandlerFunc, key string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeReadBodyError(w, err)
		h.logControlAudit(r, statusFromError(err))
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	scope := idempotencyScope(r, key)
	bodyHash := sha256.Sum256(body)
	if entry, hit, conflict := h.idem.lookup(scope, bodyHash); hit {
		writeCapturedResponse(w, entry.response)
		h.logControlAudit(r, entry.response.status)
		return
	} else if conflict {
		writeError(w, apperr.New(apperr.CodeIdempotencyConflict, "idempotency key was reused with different body", http.StatusConflict))
		h.logControlAudit(r, http.StatusConflict)
		return
	}

	recorder := newBufferedResponseCapture()
	next(recorder, r)
	response := recorder.response()
	writeCapturedResponse(w, response)
	if response.status < http.StatusInternalServerError {
		h.idem.store(scope, bodyHash, response)
	}
	h.logControlAudit(r, response.status)
}

func (h *Handler) upsertTenant(w http.ResponseWriter, r *http.Request) {
	var request admin.Tenant
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.admin.UpsertTenant(r.Context(), request)
	writeResult(w, result, err)
}

func (h *Handler) upsertProject(w http.ResponseWriter, r *http.Request) {
	var request admin.Project
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.admin.UpsertProject(r.Context(), request)
	writeResult(w, result, err)
}

func (h *Handler) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var request admin.APIKey
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.admin.CreateAPIKey(r.Context(), request)
	writeResult(w, result, err)
}

func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.admin.ListAPIKeys(r.Context(), r.URL.Query().Get("tenant_id"), r.URL.Query().Get("project_id"))
	writeResult(w, keys, err)
}

func (h *Handler) apiKeyAction(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/disable") {
		writeError(w, apperr.NotFound("admin API not found"))
		return
	}
	keyID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/api-keys/"), "/disable")
	keyID = strings.Trim(keyID, "/")
	if keyID == "" {
		writeError(w, apperr.InvalidArgument("api key id is required"))
		return
	}
	result, err := h.admin.DisableAPIKey(r.Context(), keyID)
	writeResult(w, result, err)
}

func (h *Handler) providerEmergencyAction(w http.ResponseWriter, r *http.Request) {
	h.emergencyAction(w, r, "/admin/emergency/providers/", "provider")
}

func (h *Handler) channelEmergencyAction(w http.ResponseWriter, r *http.Request) {
	h.emergencyAction(w, r, "/admin/emergency/channels/", "channel")
}

func (h *Handler) emergencyAction(w http.ResponseWriter, r *http.Request, prefix string, kind string) {
	if h.emergency == nil {
		writeError(w, apperr.ConfigUnavailable("emergency disable store is unavailable"))
		return
	}
	value, action, ok := parseEmergencyPath(r.URL.Path, prefix)
	if !ok {
		writeError(w, apperr.NotFound("admin API not found"))
		return
	}
	ttl, ok := parseEmergencyTTL(w, r)
	if !ok {
		return
	}
	var err error
	switch {
	case kind == "provider" && action == "disable":
		err = h.emergency.DisableProvider(r.Context(), value, ttl)
	case kind == "provider" && action == "enable":
		err = h.emergency.EnableProvider(r.Context(), value)
	case kind == "channel" && action == "disable":
		err = h.emergency.DisableChannel(r.Context(), value, ttl)
	case kind == "channel" && action == "enable":
		err = h.emergency.EnableChannel(r.Context(), value)
	default:
		writeError(w, apperr.NotFound("admin API not found"))
		return
	}
	writeResult(w, map[string]any{"kind": kind, "id": value, "action": action}, err)
}

func (h *Handler) upsertModel(w http.ResponseWriter, r *http.Request) {
	var request admin.ModelConfig
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.admin.UpsertModel(r.Context(), request)
	writeResult(w, result, err)
}

func (h *Handler) upsertChannel(w http.ResponseWriter, r *http.Request) {
	var request admin.ChannelConfig
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.admin.UpsertChannel(r.Context(), request)
	writeResult(w, result, err)
}

func (h *Handler) upsertRoute(w http.ResponseWriter, r *http.Request) {
	var request admin.RoutePolicyConfig
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.admin.UpsertRoute(r.Context(), request)
	writeResult(w, result, err)
}

func (h *Handler) upsertPrice(w http.ResponseWriter, r *http.Request) {
	var request admin.PriceRuleConfig
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.admin.UpsertPrice(r.Context(), request)
	writeResult(w, result, err)
}

func (h *Handler) upsertLimit(w http.ResponseWriter, r *http.Request) {
	var request admin.LimitRuleConfig
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.admin.UpsertLimit(r.Context(), request)
	writeResult(w, result, err)
}

func (h *Handler) upsertPluginBinding(w http.ResponseWriter, r *http.Request) {
	var request admin.PluginBindingConfig
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.admin.UpsertPluginBinding(r.Context(), request)
	writeResult(w, result, err)
}

func (h *Handler) upsertModelMarketplace(w http.ResponseWriter, r *http.Request) {
	var request admin.ModelMarketplaceConfig
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.admin.UpsertModelMarketplace(r.Context(), request)
	writeResult(w, result, err)
}

func (h *Handler) listVisibleModels(w http.ResponseWriter, r *http.Request) {
	result, err := h.admin.ListVisibleModels(r.Context(), r.URL.Query().Get("tenant_id"), r.URL.Query().Get("project_id"))
	writeResult(w, result, err)
}

func (h *Handler) tenantUsageReport(w http.ResponseWriter, r *http.Request) {
	if h.commercial == nil {
		writeError(w, apperr.ConfigUnavailable("commercial reporting is unavailable"))
		return
	}
	from, to, ok := parseTimeRange(w, r)
	if !ok {
		return
	}
	limit, ok := parseLimit(w, r)
	if !ok {
		return
	}
	result, err := h.commercial.TenantUsageReport(r.Context(), reporting.TenantUsageFilter{
		TenantID:  r.URL.Query().Get("tenant_id"),
		ProjectID: r.URL.Query().Get("project_id"),
		Currency:  r.URL.Query().Get("currency"),
		From:      from,
		To:        to,
		Limit:     limit,
	})
	writeResult(w, result, err)
}

func (h *Handler) providerProfitReport(w http.ResponseWriter, r *http.Request) {
	if h.commercial == nil {
		writeError(w, apperr.ConfigUnavailable("commercial reporting is unavailable"))
		return
	}
	from, to, ok := parseTimeRange(w, r)
	if !ok {
		return
	}
	result, err := h.commercial.ProviderProfitReport(r.Context(), reporting.ProviderProfitFilter{
		TenantID:  r.URL.Query().Get("tenant_id"),
		ProjectID: r.URL.Query().Get("project_id"),
		From:      from,
		To:        to,
	})
	writeResult(w, result, err)
}

func (h *Handler) reconciliationReport(w http.ResponseWriter, r *http.Request) {
	if h.commercial == nil {
		writeError(w, apperr.ConfigUnavailable("commercial reporting is unavailable"))
		return
	}
	result, err := h.commercial.ReconciliationReport(r.Context())
	writeResult(w, result, err)
}

func (h *Handler) agentMetadataReport(w http.ResponseWriter, r *http.Request) {
	if h.commercial == nil {
		writeError(w, apperr.ConfigUnavailable("commercial reporting is unavailable"))
		return
	}
	from, to, ok := parseTimeRange(w, r)
	if !ok {
		return
	}
	result, err := h.commercial.AgentMetadataReport(r.Context(), reporting.AgentMetadataFilter{
		TenantID:  r.URL.Query().Get("tenant_id"),
		ProjectID: r.URL.Query().Get("project_id"),
		From:      from,
		To:        to,
	})
	writeResult(w, result, err)
}

func (h *Handler) upsertProviderCostProfile(w http.ResponseWriter, r *http.Request) {
	if h.commercial == nil {
		writeError(w, apperr.ConfigUnavailable("commercial reporting is unavailable"))
		return
	}
	var request reporting.ProviderCostProfile
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.commercial.UpsertProviderCostProfile(r.Context(), request)
	writeResult(w, result, err)
}

func (h *Handler) createManualAdjustment(w http.ResponseWriter, r *http.Request) {
	if h.commercial == nil {
		writeError(w, apperr.ConfigUnavailable("commercial reporting is unavailable"))
		return
	}
	var request reporting.ManualAdjustmentRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := h.commercial.CreateManualAdjustment(r.Context(), request)
	writeResult(w, result, err)
}

func (h *Handler) publishSnapshot(w http.ResponseWriter, r *http.Request) {
	result, err := h.publisher.Publish(r.Context())
	writeResult(w, result, err)
}

func (h *Handler) rollbackSnapshot(w http.ResponseWriter, r *http.Request) {
	result, err := h.publisher.Rollback(r.Context())
	writeResult(w, result, err)
}

func (h *Handler) logControlAudit(r *http.Request, status int) {
	if h == nil || h.logger == nil || r == nil {
		return
	}
	h.logger.Info("control_plane_audit_event",
		"request_id", strings.TrimSpace(r.Header.Get("X-Request-ID")),
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"operator_id", strings.TrimSpace(r.Header.Get("X-Operator-ID")),
		"idempotency_key_present", strings.TrimSpace(r.Header.Get("Idempotency-Key")) != "",
	)
}

func adminToken(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Admin-Token")); value != "" {
		return value
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):])
	}
	return ""
}

func constantTimeTokenEqual(got string, want string) bool {
	gotHash := sha256.Sum256([]byte(strings.TrimSpace(got)))
	wantHash := sha256.Sum256([]byte(strings.TrimSpace(want)))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		if writeReadBodyError(w, err) {
			return false
		}
		writeError(w, apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err)))
		return false
	}
	return true
}

func writeReadBodyError(w http.ResponseWriter, err error) bool {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(w, requestBodyTooLargeError(err))
		return true
	}
	return false
}

func requestBodyTooLargeError(err error) *apperr.Error {
	if err == nil {
		return apperr.New(apperr.CodeInvalidArgument, "request body is too large", http.StatusRequestEntityTooLarge)
	}
	return apperr.New(apperr.CodeInvalidArgument, "request body is too large", http.StatusRequestEntityTooLarge, apperr.WithCause(err))
}

func statusFromError(err error) int {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func isControlWrite(r *http.Request) bool {
	return r != nil && r.Method != http.MethodGet && strings.HasPrefix(r.URL.Path, "/admin/")
}

func idempotencyScope(r *http.Request, key string) string {
	return r.Method + " " + r.URL.RequestURI() + " " + key
}

type capturedResponse struct {
	status int
	header http.Header
	body   []byte
}

type responseCapture struct {
	http.ResponseWriter
	status int
}

func newResponseCapture(w http.ResponseWriter) *responseCapture {
	return &responseCapture{ResponseWriter: w}
}

func (w *responseCapture) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseCapture) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func (w *responseCapture) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

type bufferedResponseCapture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newBufferedResponseCapture() *bufferedResponseCapture {
	return &bufferedResponseCapture{header: make(http.Header)}
}

func (w *bufferedResponseCapture) Header() http.Header {
	return w.header
}

func (w *bufferedResponseCapture) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *bufferedResponseCapture) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(body)
}

func (w *bufferedResponseCapture) response() capturedResponse {
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	return capturedResponse{
		status: status,
		header: w.header.Clone(),
		body:   append([]byte(nil), w.body.Bytes()...),
	}
}

func writeCapturedResponse(w http.ResponseWriter, response capturedResponse) {
	for key, values := range response.header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	status := response.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(response.body)
}

type idempotencyStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]idempotencyEntry
}

type idempotencyEntry struct {
	bodyHash  [32]byte
	response  capturedResponse
	expiresAt time.Time
}

func newIdempotencyStore(ttl time.Duration) *idempotencyStore {
	return &idempotencyStore{ttl: ttl, entries: make(map[string]idempotencyEntry)}
}

func (s *idempotencyStore) lookup(scope string, bodyHash [32]byte) (idempotencyEntry, bool, bool) {
	if s == nil {
		return idempotencyEntry{}, false, false
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[scope]
	if !ok {
		return idempotencyEntry{}, false, false
	}
	if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
		delete(s.entries, scope)
		return idempotencyEntry{}, false, false
	}
	if entry.bodyHash != bodyHash {
		return idempotencyEntry{}, false, true
	}
	return entry, true, false
}

func (s *idempotencyStore) store(scope string, bodyHash [32]byte, response capturedResponse) {
	if s == nil {
		return
	}
	expiresAt := time.Time{}
	if s.ttl > 0 {
		expiresAt = time.Now().UTC().Add(s.ttl)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[scope] = idempotencyEntry{
		bodyHash:  bodyHash,
		response:  response,
		expiresAt: expiresAt,
	}
}

func parseTimeRange(w http.ResponseWriter, r *http.Request) (time.Time, time.Time, bool) {
	from, ok := parseTimeQuery(w, r, "from")
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	to, ok := parseTimeQuery(w, r, "to")
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func parseTimeQuery(w http.ResponseWriter, r *http.Request, name string) (time.Time, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return time.Time{}, true
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		writeError(w, apperr.InvalidArgument(name+" must be RFC3339"))
		return time.Time{}, false
	}
	return parsed, true
}

func parseLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	value := strings.TrimSpace(r.URL.Query().Get("limit"))
	if value == "" {
		return 0, true
	}
	limit, err := strconv.Atoi(value)
	if err != nil {
		writeError(w, apperr.InvalidArgument("limit must be an integer"))
		return 0, false
	}
	return limit, true
}

func parseEmergencyPath(path string, prefix string) (value string, action string, ok bool) {
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	value = strings.TrimSpace(parts[0])
	action = strings.TrimSpace(parts[1])
	if value == "" || (action != "disable" && action != "enable") {
		return "", "", false
	}
	return value, action, true
}

func parseEmergencyTTL(w http.ResponseWriter, r *http.Request) (time.Duration, bool) {
	value := strings.TrimSpace(r.URL.Query().Get("ttl_seconds"))
	if value == "" {
		return 0, true
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		writeError(w, apperr.InvalidArgument("ttl_seconds must be a non-negative integer"))
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}

func writeResult[T any](w http.ResponseWriter, value T, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "service_error"
	message := "internal error"
	if appErr, ok := apperr.As(err); ok {
		status = appErr.HTTPStatus
		code = string(appErr.Code)
		message = appErr.SafeMessage()
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
