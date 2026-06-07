package consolehttp

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		{method: http.MethodGet, path: "/api/admin/v1/operators", status: http.StatusServiceUnavailable, body: "admin web service is unavailable"},
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

func TestConsoleStaticSecurityAndCacheHeaders(t *testing.T) {
	adminDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(adminDir, "assets"), 0o755); err != nil {
		t.Fatalf("create assets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adminDir, "index.html"), []byte("<html>admin</html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adminDir, "assets", "app.123.js"), []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	handler := testHandlerWithConfig(Config{AdminStaticDir: adminDir})

	tests := []struct {
		path  string
		cache string
	}{
		{path: "/portal/", cache: "no-cache"},
		{path: "/admin-ui/assets/app.123.js", cache: "public, max-age=31536000, immutable"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assertSecurityHeaders(t, rec.Header())
			if got := rec.Header().Get("Cache-Control"); got != tt.cache {
				t.Fatalf("Cache-Control = %q, want %q", got, tt.cache)
			}
		})
	}
}

func assertSecurityHeaders(t *testing.T, header http.Header) {
	t.Helper()
	required := map[string]string{
		"Content-Security-Policy":   "frame-ancestors 'none'",
		"Strict-Transport-Security": "max-age=31536000",
		"X-Content-Type-Options":    "nosniff",
		"Referrer-Policy":           "no-referrer",
		"X-Frame-Options":           "DENY",
	}
	for key, value := range required {
		if got := header.Get(key); !strings.Contains(got, value) {
			t.Fatalf("%s = %q, want to contain %q", key, got, value)
		}
	}
}

func testHandler() http.Handler {
	return testHandlerWithConfig(Config{})
}

func testHandlerWithConfig(cfg Config) http.Handler {
	readiness := func(context.Context) []httpserver.DependencyStatus {
		return []httpserver.DependencyStatus{{Name: "test", Status: "available"}}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	routes := NewHandler(cfg, logger)
	return httpserver.NewHandlerWithRoutes(readiness, prometheus.NewRegistry(), logger, []httpserver.RouteRegistrar{routes})
}
