// Command p22-console-smoke validates the production Console browser contract.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

const csrfHeaderName = "X-CSRF-Token"

type config struct {
	consoleURL    string
	apiKey        string
	adminEmail    string
	adminPassword string
	timeout       time.Duration
}

type consoleClient struct {
	baseURL    string
	httpClient *http.Client
	csrfToken  string
}

type adminLoginResponse struct {
	Authenticated bool `json:"authenticated"`
	Session       struct {
		Authenticated bool   `json:"authenticated"`
		Email         string `json:"email"`
	} `json:"session"`
	CSRFToken string `json:"csrf_token"`
}

type portalLoginResponse struct {
	Authenticated bool `json:"authenticated"`
	Session       struct {
		Authenticated bool   `json:"authenticated"`
		TenantID      string `json:"tenant_id"`
		ProjectID     string `json:"project_id"`
	} `json:"session"`
	CSRFToken string `json:"csrf_token"`
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()
	if err := run(ctx, cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "p22 console smoke failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config) error {
	adminClient, err := newConsoleClient(cfg.consoleURL, cfg.timeout)
	if err != nil {
		return err
	}
	portalClient, err := newConsoleClient(cfg.consoleURL, cfg.timeout)
	if err != nil {
		return err
	}
	if err := adminClient.checkStaticHeaders(ctx); err != nil {
		return err
	}
	if err := adminClient.runAdmin(ctx, cfg); err != nil {
		return err
	}
	if err := portalClient.runPortal(ctx, cfg); err != nil {
		return err
	}
	fmt.Printf("p22_console_smoke=passed console_url=%q\n", cfg.consoleURL)
	return nil
}

func parseConfig(args []string) (config, error) {
	cfg := config{
		consoleURL:    getenvDefault("CONSOLE_URL", "http://127.0.0.1:9505"),
		apiKey:        getenvDefault("API_KEY", "tg-local-dev-key"),
		adminEmail:    getenvDefault("ADMIN_EMAIL", "admin@example.com"),
		adminPassword: getenvDefault("ADMIN_PASSWORD", "admin-local"),
		timeout:       30 * time.Second,
	}
	fs := flag.NewFlagSet("p22-console-smoke", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.consoleURL, "console-url", cfg.consoleURL, "base URL for the running console")
	fs.StringVar(&cfg.apiKey, "api-key", cfg.apiKey, "customer API key for Portal session login")
	fs.StringVar(&cfg.adminEmail, "admin-email", cfg.adminEmail, "Admin operator email")
	fs.StringVar(&cfg.adminPassword, "admin-password", cfg.adminPassword, "Admin operator password")
	fs.DurationVar(&cfg.timeout, "timeout", cfg.timeout, "overall smoke timeout")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	cfg.consoleURL = strings.TrimRight(strings.TrimSpace(cfg.consoleURL), "/")
	cfg.apiKey = strings.TrimSpace(cfg.apiKey)
	cfg.adminEmail = strings.TrimSpace(cfg.adminEmail)
	cfg.adminPassword = strings.TrimSpace(cfg.adminPassword)
	if cfg.consoleURL == "" {
		return config{}, errors.New("console URL is required")
	}
	if cfg.apiKey == "" {
		return config{}, errors.New("api key is required")
	}
	if cfg.adminEmail == "" || cfg.adminPassword == "" {
		return config{}, errors.New("admin email and password are required")
	}
	return cfg, nil
}

func newConsoleClient(baseURL string, timeout time.Duration) (*consoleClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse console URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("console URL must include scheme and host: %q", baseURL)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	return &consoleClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: timeout,
		},
	}, nil
}

func (c *consoleClient) checkStaticHeaders(ctx context.Context) error {
	resp, raw, err := c.do(ctx, http.MethodGet, "/admin-ui/", nil, nil, requestOptions{})
	if err != nil {
		return err
	}
	if err := requireSecurityHeaders("admin static", resp.Header); err != nil {
		return err
	}
	if cache := resp.Header.Get("Cache-Control"); cache != "no-cache" {
		return fmt.Errorf("admin static Cache-Control=%q, want no-cache", cache)
	}
	if !bytes.Contains(raw, []byte("Admin Console")) && !bytes.Contains(raw, []byte("<!doctype html")) && !bytes.Contains(raw, []byte("<html")) {
		return fmt.Errorf("admin static did not return an HTML shell: %s", string(raw))
	}
	fmt.Println("p22_console_smoke=static_headers")
	return nil
}

func (c *consoleClient) runAdmin(ctx context.Context, cfg config) error {
	var login adminLoginResponse
	_, raw, err := c.do(ctx, http.MethodPost, "/api/admin/v1/auth/login", map[string]string{
		"email":    cfg.adminEmail,
		"password": cfg.adminPassword,
	}, &login, requestOptions{})
	if err != nil {
		return err
	}
	if strings.Contains(string(raw), cfg.adminPassword) {
		return errors.New("admin login response leaked password")
	}
	if !login.Authenticated || !login.Session.Authenticated || login.CSRFToken == "" {
		return errors.New("admin login response missing session or csrf token")
	}
	c.csrfToken = login.CSRFToken

	adminReads := []string{
		"/api/admin/v1/dashboard",
		"/api/admin/v1/tenants",
		"/api/admin/v1/projects",
		"/api/admin/v1/models",
		"/api/admin/v1/channels",
		"/api/admin/v1/routes",
		"/api/admin/v1/pricing",
		"/api/admin/v1/limits",
		"/api/admin/v1/snapshots",
		"/api/admin/v1/operations/settlements",
		"/api/admin/v1/operations/callbacks?limit=5",
		"/api/admin/v1/operations/workers",
		"/api/admin/v1/operations/holds",
		"/api/admin/v1/audit?limit=5",
		"/api/admin/v1/operators",
	}
	for _, path := range adminReads {
		if _, _, err := c.do(ctx, http.MethodGet, path, nil, nil, requestOptions{}); err != nil {
			return fmt.Errorf("admin read %s: %w", path, err)
		}
	}
	if err := c.expectCSRFDeny(ctx, "/api/admin/v1/snapshots/validate"); err != nil {
		return err
	}
	if _, _, err := c.do(ctx, http.MethodPost, "/api/admin/v1/snapshots/validate", map[string]string{}, nil, adminMutation("p22 snapshot validate")); err != nil {
		return fmt.Errorf("admin snapshot validate: %w", err)
	}
	if _, _, err := c.do(ctx, http.MethodGet, "/api/admin/v1/audit?resource=snapshot&limit=5", nil, nil, requestOptions{}); err != nil {
		return fmt.Errorf("admin audit after snapshot validate: %w", err)
	}
	if _, _, err := c.do(ctx, http.MethodPost, "/api/admin/v1/auth/logout", nil, nil, requestOptions{CSRF: true}); err != nil {
		return fmt.Errorf("admin logout: %w", err)
	}
	fmt.Println("p22_console_smoke=admin_e2e")
	return nil
}

func (c *consoleClient) runPortal(ctx context.Context, cfg config) error {
	var login portalLoginResponse
	_, raw, err := c.do(ctx, http.MethodPost, "/api/portal/v1/auth/api-key-login", map[string]string{"api_key": cfg.apiKey}, &login, requestOptions{})
	if err != nil {
		return err
	}
	if strings.Contains(string(raw), cfg.apiKey) {
		return errors.New("portal login response leaked api key")
	}
	if !login.Authenticated || !login.Session.Authenticated || login.CSRFToken == "" {
		return errors.New("portal login response missing session or csrf token")
	}
	c.csrfToken = login.CSRFToken

	portalReads := []string{
		"/api/portal/v1/dashboard",
		"/api/portal/v1/api-keys",
		"/api/portal/v1/usage?limit=5",
		"/api/portal/v1/tasks?limit=5",
	}
	for _, path := range portalReads {
		if _, _, err := c.do(ctx, http.MethodGet, path, nil, nil, requestOptions{}); err != nil {
			return fmt.Errorf("portal read %s: %w", path, err)
		}
	}
	if err := c.expectCSRFDeny(ctx, "/api/portal/v1/auth/logout"); err != nil {
		return err
	}
	if _, _, err := c.do(ctx, http.MethodPost, "/api/portal/v1/auth/logout", nil, nil, requestOptions{CSRF: true}); err != nil {
		return fmt.Errorf("portal logout: %w", err)
	}
	fmt.Println("p22_console_smoke=portal_e2e")
	return nil
}

func (c *consoleClient) expectCSRFDeny(ctx context.Context, path string) error {
	resp, raw, err := c.do(ctx, http.MethodPost, path, map[string]string{}, nil, requestOptions{AllowError: true})
	if err != nil {
		return err
	}
	if resp.StatusCode < 400 {
		return fmt.Errorf("%s without csrf status=%d body=%s", path, resp.StatusCode, string(raw))
	}
	if !bytes.Contains(raw, []byte("csrf")) {
		return fmt.Errorf("%s csrf deny body missing csrf message: %s", path, string(raw))
	}
	return nil
}

type requestOptions struct {
	CSRF       bool
	Reason     string
	AllowError bool
}

func adminMutation(reason string) requestOptions {
	return requestOptions{CSRF: true, Reason: reason}
}

func (c *consoleClient) do(ctx context.Context, method string, path string, body any, dst any, opts requestOptions) (*http.Response, []byte, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if opts.CSRF {
		req.Header.Set(csrfHeaderName, c.csrfToken)
	}
	if opts.Reason != "" {
		req.Header.Set("X-Reason", opts.Reason)
		req.Header.Set("Idempotency-Key", "p22-console-smoke-"+strings.NewReplacer("/", "-", " ", "-").Replace(strings.ToLower(opts.Reason)))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, raw, err
	}
	if err := requireSecurityHeaders(path, resp.Header); err != nil {
		return resp, raw, err
	}
	if resp.StatusCode >= 400 && !opts.AllowError {
		return resp, raw, fmt.Errorf("%s %s status=%d body=%s", method, path, resp.StatusCode, string(raw))
	}
	if dst != nil && len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, dst); err != nil {
			return resp, raw, fmt.Errorf("decode %s: %w body=%s", path, err, string(raw))
		}
	}
	return resp, raw, nil
}

func requireSecurityHeaders(label string, header http.Header) error {
	required := map[string]string{
		"Content-Security-Policy":   "frame-ancestors 'none'",
		"Strict-Transport-Security": "max-age=31536000",
		"X-Content-Type-Options":    "nosniff",
		"Referrer-Policy":           "no-referrer",
		"X-Frame-Options":           "DENY",
	}
	for key, value := range required {
		if got := header.Get(key); !strings.Contains(got, value) {
			return fmt.Errorf("%s %s=%q, want to contain %q", label, key, got, value)
		}
	}
	return nil
}

func getenvDefault(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
