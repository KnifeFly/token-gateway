// Package consolehttp wires the Human Console Plane routes.
package consolehttp

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/transport/adminhttp"
	"github.com/KnifeFly/token-gateway/internal/transport/portalwebhttp"
)

// Config controls console route skeleton behavior.
type Config struct {
	PortalStaticDir string
	AdminStaticDir  string
}

// Handler registers console-owned browser BFF and optional static routes.
type Handler struct {
	cfg    Config
	logger *slog.Logger
}

// NewHandler returns a console route registrar.
func NewHandler(cfg Config, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{cfg: cfg, logger: logger}
}

// Register adds console routes without claiming gateway or machine-control paths.
func (h *Handler) Register(mux *http.ServeMux) {
	portalwebhttp.NewHandler(h.logger).Register(mux)
	adminhttp.NewHandler(h.logger).Register(mux)

	mux.HandleFunc("GET /portal", redirectToSlash)
	mux.HandleFunc("GET /portal/", h.staticApp("Portal Console", "/portal/", h.cfg.PortalStaticDir))
	mux.HandleFunc("GET /admin-ui", redirectToSlash)
	mux.HandleFunc("GET /admin-ui/", h.staticApp("Admin Console", "/admin-ui/", h.cfg.AdminStaticDir))
}

func redirectToSlash(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, r.URL.Path+"/", http.StatusPermanentRedirect)
}

func (h *Handler) staticApp(title string, basePath string, dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		relative := strings.TrimPrefix(r.URL.Path, basePath)
		if relative == "" || strings.HasSuffix(relative, "/") {
			serveIndexOrShell(w, r, title, basePath, dir)
			return
		}

		if strings.TrimSpace(dir) != "" {
			cleaned := strings.TrimPrefix(filepath.Clean("/"+relative), string(filepath.Separator))
			path := filepath.Join(dir, cleaned)
			if fileExists(path) {
				setStaticCache(w, path)
				http.ServeFile(w, r, path)
				return
			}
		}
		serveIndexOrShell(w, r, title, basePath, dir)
	}
}

func serveIndexOrShell(w http.ResponseWriter, r *http.Request, title string, basePath string, dir string) {
	indexPath := filepath.Join(dir, "index.html")
	if fileExists(indexPath) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, indexPath)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, fallbackHTML, title, title, basePath)
}

func setStaticCache(w http.ResponseWriter, path string) {
	if strings.HasSuffix(path, ".html") {
		w.Header().Set("Cache-Control", "no-cache")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

const fallbackHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>%s</title>
    <style>
      :root { color-scheme: light; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
      body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #f7f8fb; color: #111827; }
      main { width: min(520px, calc(100vw - 40px)); border: 1px solid #d8dee9; border-radius: 8px; background: #fff; padding: 28px; box-shadow: 0 12px 32px rgba(15, 23, 42, 0.08); }
      h1 { margin: 0 0 12px; font-size: 22px; line-height: 1.25; }
      p { margin: 0; color: #4b5563; line-height: 1.6; }
      code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 0.94em; }
    </style>
  </head>
  <body>
    <main>
      <h1>%s</h1>
      <p>Static assets for <code>%s</code> are not present in the configured dist directory.</p>
    </main>
  </body>
</html>
`
