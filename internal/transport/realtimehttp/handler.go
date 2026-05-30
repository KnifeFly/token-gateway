package realtimehttp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/dataplane/realtime"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
)

const (
	operationCreateSession = "create_session"
	operationGetSession    = "get_session"
	operationWebSocket     = "websocket_stub"
)

// Handler serves the reserved realtime HTTP and WebSocket entry points.
type Handler struct {
	snapshot       engine.SnapshotProvider
	authenticator  engine.Authenticator
	realtimeEngine realtime.Engine
	observe        engine.ObserveRecorder
	metrics        *Metrics
	logger         *slog.Logger
	now            func() time.Time
}

// NewHandler builds a realtime handler that reuses data-plane auth and snapshot state.
func NewHandler(snapshot engine.SnapshotProvider, authenticator engine.Authenticator, realtimeEngine realtime.Engine, observe engine.ObserveRecorder, registry *prometheus.Registry, logger *slog.Logger) (*Handler, error) {
	metrics, err := NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	if realtimeEngine == nil {
		realtimeEngine = realtime.DisabledEngine{}
	}
	if observe == nil {
		observe = engine.NoopObserveRecorder{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		snapshot:       snapshot,
		authenticator:  authenticator,
		realtimeEngine: realtimeEngine,
		observe:        observe,
		metrics:        metrics,
		logger:         logger,
		now:            func() time.Time { return time.Now().UTC() },
	}, nil
}

// Register attaches realtime routes to the shared HTTP mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/realtime/sessions", h.createSession)
	mux.HandleFunc("GET /v1/realtime/sessions/{session_id}", h.getSession)
	mux.HandleFunc("GET /v1/realtime", h.websocketStub)
}

type sessionRequestBody struct {
	Model             string         `json:"model"`
	Modalities        []string       `json:"modalities"`
	Voice             string         `json:"voice"`
	Instructions      string         `json:"instructions"`
	InputAudioFormat  string         `json:"input_audio_format"`
	OutputAudioFormat string         `json:"output_audio_format"`
	Metadata          map[string]any `json:"metadata"`
}

type sessionResponseBody struct {
	ID           string `json:"id"`
	Object       string `json:"object"`
	ClientSecret string `json:"client_secret,omitempty"`
	ExpiresAt    int64  `json:"expires_at"`
	WebSocketURL string `json:"websocket_url,omitempty"`
	WebRTCURL    string `json:"webrtc_url,omitempty"`
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	state := h.newState(r, operationCreateSession)
	ctx, span := h.observe.StartSpan(r.Context(), "gateway.realtime.create_session", realtimeAttrs(operationCreateSession, state)...)
	defer span.End()

	if err := h.authenticate(ctx, state); err != nil {
		span.RecordError(err)
		h.finish(ctx, w, state, operationCreateSession, httpStatus(err), err)
		return
	}

	var body sessionRequestBody
	if err := decodeJSON(r.Body, &body); err != nil {
		err = apperr.InvalidArgument("invalid realtime session request", apperr.WithCause(err))
		span.RecordError(err)
		h.finish(ctx, w, state, operationCreateSession, httpStatus(err), err)
		return
	}

	req := realtime.SessionRequest{
		TenantID:          state.TenantID,
		ProjectID:         state.ProjectID,
		APIKeyID:          state.APIKeyID,
		RequestID:         state.RequestID,
		TraceID:           state.TraceID,
		Model:             strings.TrimSpace(body.Model),
		Modalities:        append([]string(nil), body.Modalities...),
		Voice:             strings.TrimSpace(body.Voice),
		Instructions:      body.Instructions,
		InputAudioFormat:  strings.TrimSpace(body.InputAudioFormat),
		OutputAudioFormat: strings.TrimSpace(body.OutputAudioFormat),
		Metadata:          cloneMetadata(body.Metadata),
		ExpiresIn:         15 * time.Minute,
	}
	if err := req.Validate(); err != nil {
		span.RecordError(err)
		h.finish(ctx, w, state, operationCreateSession, httpStatus(err), err)
		return
	}
	span.SetAttributes(attribute.String("gateway.model", req.Model))
	if err := h.authorizeModel(state, req.Model); err != nil {
		span.RecordError(err)
		h.finish(ctx, w, state, operationCreateSession, httpStatus(err), err)
		return
	}

	session, err := h.realtimeEngine.CreateSession(ctx, req)
	if err != nil {
		span.RecordError(err)
		h.finish(ctx, w, state, operationCreateSession, httpStatus(err), err)
		return
	}
	h.finish(ctx, w, state, operationCreateSession, http.StatusOK, nil, session)
}

func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	state := h.newState(r, operationGetSession)
	ctx, span := h.observe.StartSpan(r.Context(), "gateway.realtime.get_session", realtimeAttrs(operationGetSession, state)...)
	defer span.End()

	if err := h.authenticate(ctx, state); err != nil {
		span.RecordError(err)
		h.finish(ctx, w, state, operationGetSession, httpStatus(err), err)
		return
	}

	sessionID := strings.TrimSpace(r.PathValue("session_id"))
	if sessionID == "" {
		err := apperr.InvalidArgument("session_id is required")
		span.RecordError(err)
		h.finish(ctx, w, state, operationGetSession, httpStatus(err), err)
		return
	}

	session, err := h.realtimeEngine.GetSession(ctx, state.TenantID, state.ProjectID, sessionID)
	if err != nil {
		span.RecordError(err)
		h.finish(ctx, w, state, operationGetSession, httpStatus(err), err)
		return
	}
	h.finish(ctx, w, state, operationGetSession, http.StatusOK, nil, session)
}

func (h *Handler) websocketStub(w http.ResponseWriter, r *http.Request) {
	state := h.newState(r, operationWebSocket)
	ctx, span := h.observe.StartSpan(r.Context(), "gateway.realtime.websocket_stub", realtimeAttrs(operationWebSocket, state)...)
	defer span.End()

	if err := h.authenticate(ctx, state); err != nil {
		span.RecordError(err)
		h.finishConnection(ctx, w, state, httpStatus(err), err)
		return
	}

	conn := stubConnection{sessionID: strings.TrimSpace(r.URL.Query().Get("session_id"))}
	err := h.realtimeEngine.HandleConnection(ctx, conn)
	if err != nil {
		span.RecordError(err)
		h.finishConnection(ctx, w, state, httpStatus(err), err)
		return
	}
	h.finishConnection(ctx, w, state, http.StatusSwitchingProtocols, nil)
}

func (h *Handler) authenticate(ctx context.Context, state *engine.RequestState) error {
	if h.snapshot == nil {
		return apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	if err := h.snapshot.Attach(ctx, state); err != nil {
		return err
	}
	if h.authenticator == nil {
		return apperr.ConfigUnavailable("authenticator is unavailable")
	}
	return h.authenticator.Authenticate(ctx, state)
}

func (h *Handler) authorizeModel(state *engine.RequestState, model string) error {
	if state.Snapshot == nil {
		return apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	resolved, ok := state.Snapshot.LookupModel(model)
	if !ok || !resolved.Enabled {
		return apperr.NotFound("model not found")
	}
	if state.Principal == nil {
		return apperr.Unauthorized("authentication is required")
	}
	if !modelAllowed(state.Principal.AllowedModels, resolved.PublicModel) {
		return apperr.Forbidden("model is not allowed")
	}
	state.RequestedModel = resolved.PublicModel
	state.ResolvedModel = resolved
	if price, ok := state.Snapshot.LookupPrice(resolved.PublicModel); ok {
		state.PriceRule = price
	}
	if limit, ok := state.Snapshot.LookupLimit(resolved.PublicModel); ok {
		state.LimitRule = limit
	}
	return nil
}

func (h *Handler) finish(ctx context.Context, w http.ResponseWriter, state *engine.RequestState, operation string, status int, err error, sessions ...*realtime.Session) {
	h.metrics.recordSession(operation, err)
	h.logAudit(state, operation, status, err)
	if err != nil {
		h.observe.FinishRequest(ctx, state, &engine.GatewayResponse{StatusCode: status}, err)
		writeError(w, state, err)
		return
	}
	var session *realtime.Session
	if len(sessions) > 0 {
		session = sessions[0]
	}
	h.observe.FinishRequest(ctx, state, &engine.GatewayResponse{StatusCode: status}, nil)
	writeJSON(w, status, sessionResponse(session))
}

func (h *Handler) finishConnection(ctx context.Context, w http.ResponseWriter, state *engine.RequestState, status int, err error) {
	h.metrics.recordConnection(operationWebSocket, err)
	h.logAudit(state, operationWebSocket, status, err)
	if err != nil {
		h.observe.FinishRequest(ctx, state, &engine.GatewayResponse{StatusCode: status}, err)
		writeError(w, state, err)
		return
	}
	h.observe.FinishRequest(ctx, state, &engine.GatewayResponse{StatusCode: status}, nil)
	w.WriteHeader(status)
}

func (h *Handler) logAudit(state *engine.RequestState, operation string, status int, err error) {
	if h == nil || h.logger == nil || state == nil {
		return
	}
	outcome, errorCode := metricOutcome(err)
	h.logger.Info("realtime_audit_event",
		"request_id", state.RequestID,
		"trace_id", state.TraceID,
		"tenant_id", state.TenantID,
		"project_id", state.ProjectID,
		"api_key_id", state.APIKeyID,
		"operation", operation,
		"model", state.RequestedModel,
		"status", status,
		"outcome", outcome,
		"error_code", errorCode,
		"snapshot_version", state.SnapshotRef.Version,
	)
}

func (h *Handler) newState(r *http.Request, operation string) *engine.RequestState {
	requestID := headerValue(r.Header, "X-Request-ID")
	if requestID == "" {
		requestID = newID()
	}
	traceID := headerValue(r.Header, "X-Trace-ID")
	if traceID == "" {
		traceID = requestID
	}
	return &engine.RequestState{
		RequestID:       requestID,
		TraceID:         traceID,
		ClientRequestID: requestID,
		StartedAt:       h.now(),
		Incoming: engine.IncomingRequest{
			Method:        r.Method,
			Path:          r.URL.Path,
			RawQuery:      r.URL.RawQuery,
			Header:        r.Header.Clone(),
			Body:          r.Body,
			RemoteAddr:    r.RemoteAddr,
			ContentLength: r.ContentLength,
		},
		ProtocolMode: engine.ProtocolUnified,
		CanonicalAPI: engine.CanonicalAPI("realtime." + operation),
		ClientIP:     r.RemoteAddr,
		Metadata:     make(map[string]string),
		Internal:     make(map[string]any),
	}
}

func sessionResponse(session *realtime.Session) sessionResponseBody {
	if session == nil {
		return sessionResponseBody{}
	}
	object := session.Object
	if object == "" {
		object = "realtime.session"
	}
	return sessionResponseBody{
		ID:           session.ID,
		Object:       object,
		ClientSecret: session.ClientSecret,
		ExpiresAt:    session.ExpiresAt.Unix(),
		WebSocketURL: session.WebSocketURL,
		WebRTCURL:    session.WebRTCURL,
	}
}

func decodeJSON(body io.Reader, out any) error {
	if body == nil {
		return io.EOF
	}
	return json.NewDecoder(body).Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, state *engine.RequestState, err error) {
	status := http.StatusInternalServerError
	code := "service_error"
	message := "internal error"
	errType := "service_error"
	retryable := false
	if appErr, ok := apperr.As(err); ok {
		status = appErr.HTTPStatus
		code = string(appErr.Code)
		message = appErr.SafeMessage()
		errType = externalType(appErr.Code)
		retryable = appErr.Temporary
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":       code,
			"message":    message,
			"type":       errType,
			"request_id": state.RequestID,
			"retryable":  retryable,
		},
	})
}

func httpStatus(err error) int {
	if appErr, ok := apperr.As(err); ok {
		return appErr.HTTPStatus
	}
	return http.StatusInternalServerError
}

func externalType(code apperr.Code) string {
	switch code {
	case apperr.CodeUnauthorized:
		return "authentication_error"
	case apperr.CodeForbidden, apperr.CodePolicyDenied:
		return "permission_error"
	case apperr.CodeInternal, apperr.CodeConfigUnavailable, apperr.CodeServiceUnavailable:
		return "service_error"
	default:
		return "invalid_request_error"
	}
}

func metricOutcome(err error) (string, string) {
	if err == nil {
		return "success", "none"
	}
	if appErr, ok := apperr.As(err); ok {
		return "error", string(appErr.Code)
	}
	return "error", "internal_error"
}

func modelAllowed(allowed []string, model string) bool {
	for _, value := range allowed {
		if value == "*" || value == model {
			return true
		}
	}
	return false
}

func realtimeAttrs(operation string, state *engine.RequestState) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("gateway.realtime.operation", operation),
	}
	if state == nil {
		return attrs
	}
	attrs = append(attrs,
		attribute.String("gateway.request_id", state.RequestID),
		attribute.String("gateway.trace_id", state.TraceID),
		attribute.String("gateway.model", stringOrUnknown(state.RequestedModel)),
	)
	return attrs
}

func stringOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func headerValue(header http.Header, name string) string {
	if value := header.Get(name); value != "" {
		return value
	}
	for key, values := range header {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}

type stubConnection struct {
	sessionID string
}

func (c stubConnection) SessionID() string {
	return c.sessionID
}

func (stubConnection) Close(int, string) error {
	return nil
}
