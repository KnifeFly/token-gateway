package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"

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

// RouteRegistrar adds non-gateway routes to the shared HTTP mux.
type RouteRegistrar interface {
	Register(*http.ServeMux)
}

// HandlerConfig customizes shared gateway HTTP routing.
type HandlerConfig struct {
	TrustedProxyCIDRs []string
}

// NewHandler builds gateway routes.
func NewHandler(readiness ReadinessFunc, registry *prometheus.Registry, logger *slog.Logger, gateways ...Gateway) http.Handler {
	return NewHandlerWithRoutesConfig(readiness, registry, logger, HandlerConfig{}, nil, gateways...)
}

// NewHandlerWithRoutes builds gateway routes plus optional extension routes.
func NewHandlerWithRoutes(readiness ReadinessFunc, registry *prometheus.Registry, logger *slog.Logger, extensions []RouteRegistrar, gateways ...Gateway) http.Handler {
	return NewHandlerWithRoutesConfig(readiness, registry, logger, HandlerConfig{}, extensions, gateways...)
}

// NewHandlerWithRoutesConfig builds gateway routes plus optional extension routes and HTTP config.
func NewHandlerWithRoutesConfig(readiness ReadinessFunc, registry *prometheus.Registry, logger *slog.Logger, config HandlerConfig, extensions []RouteRegistrar, gateways ...Gateway) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz(readiness))
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	for _, extension := range extensions {
		if extension != nil {
			extension.Register(mux)
		}
	}
	if len(gateways) > 0 && gateways[0] != nil {
		mux.HandleFunc("POST /v1/chat/completions", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/responses", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/embeddings", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/moderations", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/messages", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1beta/models/", dataPlane(gateways[0]))
		mux.HandleFunc("GET /v1/tasks/", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/tasks/", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/files/upload/base64", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/files/upload/url", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/files/upload/stream", dataPlane(gateways[0]))
		mux.HandleFunc("GET /v1/files/quota", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/images/generations", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/images/edits", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/videos/generations", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/audio/speech", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/audio/transcriptions", dataPlane(gateways[0]))
		mux.HandleFunc("POST /v1/music/generations", dataPlane(gateways[0]))
	}
	handler := RequestIDMiddleware(RecoveryMiddleware(AccessLogMiddleware(mux, logger)))
	return trustedProxyMiddleware(config.TrustedProxyCIDRs, handler)
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func trustedProxyMiddleware(cidrs []string, next http.Handler) http.Handler {
	resolver := newClientIPResolver(cidrs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if resolver != nil {
			cloned := r.Clone(r.Context())
			cloned.RemoteAddr = resolver.clientIP(r)
			r = cloned
		}
		next.ServeHTTP(w, r)
	})
}

type clientIPResolver struct {
	trusted []*net.IPNet
}

func newClientIPResolver(cidrs []string) *clientIPResolver {
	resolver := &clientIPResolver{}
	for _, value := range cidrs {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		_, network, err := net.ParseCIDR(value)
		if err == nil {
			resolver.trusted = append(resolver.trusted, network)
		}
	}
	return resolver
}

func (r *clientIPResolver) clientIP(req *http.Request) string {
	direct := remoteIP(req.RemoteAddr)
	if direct == nil {
		return strings.TrimSpace(req.RemoteAddr)
	}
	if !r.isTrusted(direct) {
		return direct.String()
	}
	if ip := firstForwardedIP(req.Header.Get("X-Forwarded-For")); ip != nil {
		return ip.String()
	}
	if ip := net.ParseIP(strings.TrimSpace(req.Header.Get("X-Real-IP"))); ip != nil {
		return ip.String()
	}
	return direct.String()
}

func (r *clientIPResolver) isTrusted(ip net.IP) bool {
	if r == nil || len(r.trusted) == 0 || ip == nil {
		return false
	}
	for _, network := range r.trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func remoteIP(remoteAddr string) net.IP {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return nil
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		remoteAddr = host
	}
	return net.ParseIP(remoteAddr)
}

func firstForwardedIP(value string) net.IP {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		return net.ParseIP(part)
	}
	return nil
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

func dataPlane(gateway Gateway) http.HandlerFunc {
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
		if response.Stream != nil {
			for key, values := range response.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			writeStream(r.Context(), w, response.StatusCode, response.Stream)
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
