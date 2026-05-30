package publichttp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// Handler serves customer-facing local APIs that do not require provider dispatch.
type Handler struct {
	snapshot   engine.SnapshotProvider
	auth       engine.Authenticator
	commercial *reporting.Service
	logger     *slog.Logger
}

// NewHandler returns a public API route registrar.
func NewHandler(snapshot engine.SnapshotProvider, auth engine.Authenticator, commercial *reporting.Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{snapshot: snapshot, auth: auth, commercial: commercial, logger: logger}
}

// Register adds public routes to the shared data-plane mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/models", h.listModels)
	mux.HandleFunc("GET /v1/models/", h.getModelSchema)
	mux.HandleFunc("GET /v1/credits", h.getCredits)
}

func (h *Handler) listModels(w http.ResponseWriter, r *http.Request) {
	state, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	models := state.Snapshot.ListModels()
	out := make([]map[string]any, 0, len(models))
	for _, model := range models {
		if !model.Enabled || !modelAllowedForView(state.Principal.AllowedModels, model.PublicModel, model) {
			continue
		}
		out = append(out, modelObject(model))
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": out})
}

func (h *Handler) getModelSchema(w http.ResponseWriter, r *http.Request) {
	state, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	modelName := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/models/"), "/schema")
	modelName = strings.Trim(modelName, "/")
	if modelName == "" || strings.Contains(modelName, "/") {
		writeError(w, state.RequestID, apperr.NotFound("model schema not found"))
		return
	}
	model, found := state.Snapshot.LookupModel(modelName)
	if !found || !model.Enabled || !modelAllowedForView(state.Principal.AllowedModels, modelName, model) {
		writeError(w, state.RequestID, apperr.NotFound("model not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"model":   model.PublicModel,
		"version": state.SnapshotRef.Version,
		"schema":  modelSchema(model),
	})
}

func (h *Handler) getCredits(w http.ResponseWriter, r *http.Request) {
	state, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	if h.commercial == nil {
		writeError(w, state.RequestID, apperr.ConfigUnavailable("commercial reporting is unavailable"))
		return
	}
	report, err := h.commercial.TenantUsageReport(r.Context(), reporting.TenantUsageFilter{
		TenantID:  state.TenantID,
		ProjectID: state.ProjectID,
		Currency:  r.URL.Query().Get("currency"),
		Limit:     1,
	})
	if err != nil {
		writeError(w, state.RequestID, err)
		return
	}
	bucket := creditBucket(report)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "ok",
		"data": map[string]any{
			"token":   bucket,
			"user":    bucket,
			"account": bucket,
		},
	})
}

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request) (*engine.RequestState, bool) {
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
	if h.snapshot == nil || h.auth == nil {
		writeError(w, state.RequestID, apperr.ConfigUnavailable("runtime snapshot is unavailable"))
		return nil, false
	}
	if err := h.snapshot.Attach(r.Context(), state); err != nil {
		writeError(w, state.RequestID, err)
		return nil, false
	}
	if err := h.auth.Authenticate(r.Context(), state); err != nil {
		writeError(w, state.RequestID, err)
		return nil, false
	}
	return state, true
}

func modelObject(model engine.ModelView) map[string]any {
	capabilities := capabilities(model)
	return map[string]any{
		"id":                model.PublicModel,
		"object":            "model",
		"type":              modelType(model),
		"display_name":      displayName(model),
		"description":       model.Description,
		"aliases":           append([]string(nil), model.Aliases...),
		"owner":             "platform",
		"capabilities":      capabilities,
		"input_modalities":  inputModalities(model),
		"output_modalities": outputModalities(model),
		"async":             model.Protocol == engine.ProtocolUnified,
		"deprecated":        false,
	}
}

func capabilities(model engine.ModelView) []string {
	if model.Capability == "" {
		return []string{string(model.Protocol)}
	}
	parts := strings.FieldsFunc(model.Capability, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		out = append(out, model.Capability)
	}
	sort.Strings(out)
	return out
}

func modelType(model engine.ModelView) string {
	value := strings.ToLower(model.Capability + " " + model.PublicModel)
	switch {
	case strings.Contains(value, "moderation"):
		return "moderation"
	case strings.Contains(value, "embedding"):
		return "embedding"
	case strings.Contains(value, "image"):
		return "image"
	case strings.Contains(value, "video"):
		return "video"
	case strings.Contains(value, "audio") || strings.Contains(value, "speech") || strings.Contains(value, "transcription"):
		return "audio"
	case strings.Contains(value, "multi"):
		return "multimodal"
	default:
		return "text"
	}
}

func inputModalities(model engine.ModelView) []string {
	switch modelType(model) {
	case "image":
		return []string{"text", "image"}
	case "video":
		return []string{"text", "image"}
	case "audio":
		return []string{"text", "audio"}
	default:
		return []string{"text"}
	}
}

func outputModalities(model engine.ModelView) []string {
	switch modelType(model) {
	case "image":
		return []string{"image"}
	case "video":
		return []string{"video"}
	case "audio":
		return []string{"audio", "text"}
	case "embedding":
		return []string{"embedding"}
	case "moderation":
		return []string{"moderation"}
	default:
		return []string{"text"}
	}
}

func modelSchema(model engine.ModelView) map[string]any {
	if len(model.Schema) > 0 && string(model.Schema) != "{}" {
		var schema map[string]any
		if err := json.Unmarshal(model.Schema, &schema); err == nil && len(schema) > 0 {
			return schema
		}
	}
	return map[string]any{
		"type":     "object",
		"required": []string{"model"},
		"properties": map[string]any{
			"model": map[string]any{
				"type":  "string",
				"const": model.PublicModel,
			},
			"model_params": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
	}
}

func displayName(model engine.ModelView) string {
	if model.DisplayName != "" {
		return model.DisplayName
	}
	return model.PublicModel
}

func creditBucket(report *reporting.TenantUsageReport) map[string]any {
	var remaining, held int64
	currency := ""
	if report != nil {
		for _, balance := range report.Balances {
			remaining += balance.AvailableMicros
			held += balance.HeldMicros
			if currency == "" {
				currency = balance.Currency
			}
		}
		if currency == "" {
			currency = report.Totals.Currency
		}
	}
	if currency == "" {
		currency = "USD"
	}
	used := int64(0)
	if report != nil {
		used = report.Totals.RevenueMicros
	}
	return map[string]any{
		"remaining_credits": microsToCredits(remaining),
		"used_credits":      microsToCredits(used),
		"held_credits":      microsToCredits(held),
		"unlimited_credits": false,
		"currency":          currency,
	}
}

func microsToCredits(value int64) float64 {
	return float64(value) / 1_000_000
}

func modelAllowed(allowed []string, model string) bool {
	for _, value := range allowed {
		if value == "*" || value == model {
			return true
		}
	}
	return false
}

func modelAllowedForView(allowed []string, requested string, model engine.ModelView) bool {
	if modelAllowed(allowed, model.PublicModel) || modelAllowed(allowed, requested) {
		return true
	}
	for _, alias := range model.Aliases {
		if modelAllowed(allowed, alias) {
			return true
		}
	}
	return false
}

func requestID(r *http.Request) string {
	if value := r.Header.Get("X-Request-ID"); value != "" {
		return value
	}
	return fmt.Sprintf("req_%d", time.Now().UTC().UnixNano())
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
