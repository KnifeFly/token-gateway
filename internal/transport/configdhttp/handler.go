package configdhttp

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// Handler exposes configd-owned runtime snapshot operations.
type Handler struct {
	publisher *cpsnapshot.Publisher
	token     string
	logger    *slog.Logger
}

// NewHandler returns a configd HTTP route registrar.
func NewHandler(publisher *cpsnapshot.Publisher, token string, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{publisher: publisher, token: token, logger: logger}
}

// Register adds configd routes to a shared mux.
func (h *Handler) Register(mux *http.ServeMux) {
	if h == nil || mux == nil {
		return
	}
	mux.HandleFunc("POST /configd/snapshots/publish", h.requireAdmin(h.publishSnapshot))
	mux.HandleFunc("POST /configd/snapshots/rollback", h.requireAdmin(h.rollbackSnapshot))
	mux.HandleFunc("GET /configd/snapshots/diagnostics", h.requireAdmin(h.snapshotDiagnostics))
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
		next(w, r)
	}
}

func (h *Handler) publishSnapshot(w http.ResponseWriter, r *http.Request) {
	if h.publisher == nil {
		writeError(w, apperr.ConfigUnavailable("snapshot publisher is unavailable"))
		return
	}
	result, err := h.publisher.Publish(r.Context())
	writeResult(w, result, err)
}

func (h *Handler) rollbackSnapshot(w http.ResponseWriter, r *http.Request) {
	if h.publisher == nil {
		writeError(w, apperr.ConfigUnavailable("snapshot publisher is unavailable"))
		return
	}
	result, err := h.publisher.Rollback(r.Context())
	writeResult(w, result, err)
}

func (h *Handler) snapshotDiagnostics(w http.ResponseWriter, r *http.Request) {
	if h.publisher == nil {
		writeError(w, apperr.ConfigUnavailable("snapshot publisher is unavailable"))
		return
	}
	result, err := h.publisher.Diagnostics(r.Context())
	writeResult(w, result, err)
}

func writeResult(w http.ResponseWriter, result any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := string(apperr.CodeInternal)
	message := "internal error"
	if appErr, ok := apperr.As(err); ok {
		status = appErr.HTTPStatus
		code = string(appErr.Code)
		message = appErr.SafeMessage()
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
			"type":    "configd_error",
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
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
