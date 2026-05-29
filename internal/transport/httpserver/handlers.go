package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
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

// Gateway handles data-plane requests.
type Gateway interface {
	Handle(context.Context, engine.IncomingRequest) (*engine.GatewayResponse, error)
}

// NewHandler builds gateway routes.
func NewHandler(readiness ReadinessFunc, registry *prometheus.Registry, logger *slog.Logger, gateways ...Gateway) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz(readiness))
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	if len(gateways) > 0 && gateways[0] != nil {
		mux.HandleFunc("POST /v1/chat/completions", chatCompletions(gateways[0]))
	}
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

func chatCompletions(gateway Gateway) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := gateway.Handle(r.Context(), engine.IncomingRequest{
			Method:        r.Method,
			Path:          r.URL.Path,
			RawQuery:      r.URL.RawQuery,
			Header:        r.Header.Clone(),
			Body:          r.Body,
			RemoteAddr:    r.RemoteAddr,
			ContentLength: r.ContentLength,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]any{
					"code":    "service_error",
					"message": "internal error",
					"type":    "service_error",
				},
			})
			return
		}
		for key, values := range response.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}
		if response.StatusCode == 0 {
			response.StatusCode = http.StatusOK
		}
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(response.Body)
	}
}
