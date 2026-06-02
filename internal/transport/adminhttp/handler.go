package adminhttp

import (
	"log/slog"
	"net/http"

	adminservice "github.com/KnifeFly/token-gateway/internal/app/admin/service"
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
	mux.HandleFunc("POST /api/admin/v1/models/sync-preview", h.withSession(h.previewModelCatalogSync))
	mux.HandleFunc("GET /api/admin/v1/models/", h.withSession(h.modelRead))
	mux.HandleFunc("PATCH /api/admin/v1/models/", h.withSession(h.patchModel))
	mux.HandleFunc("POST /api/admin/v1/models/", h.withSession(h.modelAction))
	mux.HandleFunc("GET /api/admin/v1/channels", h.withSession(h.listChannels))
	mux.HandleFunc("GET /api/admin/v1/channels/", h.withSession(h.channelRead))
	mux.HandleFunc("POST /api/admin/v1/channels", h.withSession(h.upsertChannel))
	mux.HandleFunc("PATCH /api/admin/v1/channels/", h.withSession(h.patchChannel))
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
