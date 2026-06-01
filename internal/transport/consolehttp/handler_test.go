package consolehttp

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/transport/httpserver"
	"github.com/prometheus/client_golang/prometheus"
)

func TestConsoleRouteBoundaries(t *testing.T) {
	handler := testHandler()

	tests := []struct {
		method string
		path   string
		status int
		body   string
	}{
		{method: http.MethodGet, path: "/healthz", status: http.StatusOK, body: `"status":"ok"`},
		{method: http.MethodGet, path: "/api/portal/v1/dashboard", status: http.StatusServiceUnavailable, body: "portal web service is unavailable"},
		{method: http.MethodPost, path: "/api/admin/v1/operators", status: http.StatusNotImplemented, body: "admin web BFF is reserved for P21"},
		{method: http.MethodGet, path: "/portal/", status: http.StatusOK, body: "Portal Console"},
		{method: http.MethodGet, path: "/admin-ui/", status: http.StatusOK, body: "Admin Console"},
		{method: http.MethodGet, path: "/admin/snapshots/publish", status: http.StatusNotFound, body: "404 page not found"},
		{method: http.MethodGet, path: "/v1/portal/models", status: http.StatusNotFound, body: "404 page not found"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.status, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.body) {
				t.Fatalf("body %q does not contain %q", rec.Body.String(), tt.body)
			}
		})
	}
}

func testHandler() http.Handler {
	readiness := func(context.Context) []httpserver.DependencyStatus {
		return []httpserver.DependencyStatus{{Name: "test", Status: "available"}}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	routes := NewHandler(Config{}, logger)
	return httpserver.NewHandlerWithRoutes(readiness, prometheus.NewRegistry(), logger, []httpserver.RouteRegistrar{routes})
}
