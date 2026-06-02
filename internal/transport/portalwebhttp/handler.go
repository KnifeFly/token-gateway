package portalwebhttp

import (
	"log/slog"
	"net/http"

	portalservice "github.com/KnifeFly/token-gateway/internal/app/portal/service"
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
	mux.HandleFunc("POST /api/portal/v1/playground/run", h.withSession(h.runPlayground))
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
