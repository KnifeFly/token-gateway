package adminhttp

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Handler owns the Admin browser BFF route skeleton.
type Handler struct {
	logger *slog.Logger
}

// NewHandler builds an Admin Web BFF route registrar.
func NewHandler(logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{logger: logger}
}

// Register adds Admin Web BFF routes to the console mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/v1", h.notImplemented)
	mux.HandleFunc("/api/admin/v1/", h.notImplemented)
}

func (_ *Handler) notImplemented(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/admin/v1") {
		http.NotFound(w, r)
		return
	}
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	writeJSON(w, http.StatusNotImplemented, map[string]any{
		"error": map[string]any{
			"code":       "not_implemented",
			"message":    "admin web BFF is reserved for P21",
			"type":       "not_implemented",
			"request_id": requestID,
		},
	})
}

func requestID(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Request-ID")); value != "" {
		return value
	}
	return "req_" + time.Now().UTC().Format("20060102150405.000000000")
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
