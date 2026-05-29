package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DependencyStatus is a safe readiness dependency report.
type DependencyStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ReadinessFunc returns dependency readiness.
type ReadinessFunc func(context.Context) []DependencyStatus

// NewHandler builds M0 routes.
func NewHandler(readiness ReadinessFunc, registry *prometheus.Registry, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz(readiness))
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	return RequestIDMiddleware(RecoveryMiddleware(AccessLogMiddleware(mux, logger)))
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readyz(readiness ReadinessFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statuses := readiness(r.Context())
		ready := true
		for _, status := range statuses {
			if status.Status == "unavailable" {
				ready = false
				break
			}
		}
		status := "ready"
		code := http.StatusOK
		if !ready {
			status = "not_ready"
			code = http.StatusServiceUnavailable
		}
		writeJSON(w, code, map[string]any{
			"status":       status,
			"dependencies": statuses,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
