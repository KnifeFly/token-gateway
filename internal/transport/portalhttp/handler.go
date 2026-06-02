package portalhttp

import (
	"log/slog"
	"net/http"

	portalservice "github.com/KnifeFly/token-gateway/internal/app/portal/service"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

// Handler serves customer-facing Portal APIs with API-key scoped permissions.
type Handler struct {
	snapshot engine.SnapshotProvider
	auth     engine.Authenticator
	portal   *portalservice.Service
	logger   *slog.Logger
}

// NewHandler returns a Portal route registrar.
func NewHandler(snapshot engine.SnapshotProvider, auth engine.Authenticator, portalService *portalservice.Service, logger *slog.Logger) *Handler {
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
