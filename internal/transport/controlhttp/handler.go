package controlhttp

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// Handler serves M5 control-plane admin APIs.
type Handler struct {
	admin     *admin.Service
	publisher *cpsnapshot.Publisher
	token     string
	logger    *slog.Logger
}

// NewHandler returns a control-plane HTTP handler.
func NewHandler(adminService *admin.Service, publisher *cpsnapshot.Publisher, token string, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &Handler{admin: adminService, publisher: publisher, token: token, logger: logger}
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
	mux.HandleFunc("POST /admin/snapshots/publish", h.requireAdmin(h.publishSnapshot))
	mux.HandleFunc("POST /admin/snapshots/rollback", h.requireAdmin(h.rollbackSnapshot))
	return mux
}

func (h *Handler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.token != "" && adminToken(r) != h.token {
			writeError(w, apperr.Unauthorized("invalid admin token"))
			return
		}
		next(w, r)
	}
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

func (h *Handler) publishSnapshot(w http.ResponseWriter, r *http.Request) {
	result, err := h.publisher.Publish(r.Context())
	writeResult(w, result, err)
}

func (h *Handler) rollbackSnapshot(w http.ResponseWriter, r *http.Request) {
	result, err := h.publisher.Rollback(r.Context())
	writeResult(w, result, err)
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

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err)))
		return false
	}
	return true
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
