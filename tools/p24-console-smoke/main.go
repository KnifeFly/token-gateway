// Command p24-console-smoke validates the P24 browser-facing Admin and Portal
// BFF workflows against a running console process.
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
	consoleURL       string
	apiKey           string
	model            string
	adminEmail       string
	adminPassword    string
	createDerivedKey bool
	seedFixtures     bool
	timeout          time.Duration
}

type consoleClient struct {
	baseURL    string
	csrfToken  string
	httpClient *http.Client
}

type requestOptions struct {
	CSRF           bool
	Status         int
	Reason         string
	IdempotencyKey string
}

type adminLoginResponse struct {
	Authenticated bool   `json:"authenticated"`
	CSRFToken     string `json:"csrf_token"`
	Session       struct {
		Authenticated bool   `json:"authenticated"`
		OperatorID    string `json:"operator_id"`
		Email         string `json:"email"`
	} `json:"session"`
}

type adminChannelListResponse struct {
	Data []struct {
		ID         string `json:"id"`
		ModelCount int    `json:"model_count"`
	} `json:"data"`
}

type adminModelListResponse struct {
	Data []struct {
		PublicModel string `json:"public_model"`
		DisplayName string `json:"display_name"`
	} `json:"data"`
}

type adminCustomerAccountListResponse struct {
	Data []struct {
		CustomerAccountID string `json:"customer_account_id"`
		TenantID          string `json:"tenant_id"`
		ProjectID         string `json:"project_id"`
	} `json:"data"`
}

type adminAPIKeyListResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	} `json:"data"`
}

type adminUsageLogListResponse struct {
	Data []struct {
		RequestID string `json:"request_id"`
	} `json:"data"`
}

type adminTaskLogListResponse struct {
	Data []struct {
		TaskID string `json:"task_id"`
	} `json:"data"`
}

type portalLoginResponse struct {
	Authenticated bool   `json:"authenticated"`
	CSRFToken     string `json:"csrf_token"`
	Session       struct {
		Authenticated bool   `json:"authenticated"`
		TenantID      string `json:"tenant_id"`
		ProjectID     string `json:"project_id"`
		APIKeyID      string `json:"api_key_id"`
	} `json:"session"`
}

type portalModelListResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type portalCreditsResponse struct {
	Data map[string]any `json:"data"`
}

type portalCreditLedgerResponse struct {
	Currency string           `json:"currency"`
	Items    []map[string]any `json:"items"`
}

type portalUsageResponse struct {
	Totals struct {
		Requests int64 `json:"requests"`
	} `json:"totals"`
	Items []map[string]any `json:"items"`
}

type portalUsageExportResponse struct {
	Filename   string   `json:"filename"`
	SafeFields []string `json:"safe_fields"`
}

type portalAPIKeyListResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	} `json:"data"`
}

type portalAPIKeyCreateResponse struct {
	APIKey struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	} `json:"api_key"`
	PlaintextKey string `json:"plaintext_key"`
}

type portalTaskListResponse struct {
	Data []map[string]any `json:"data"`
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "p24_console_smoke=failed error=%q\n", err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	adminClient, err := newConsoleClient(cfg.consoleURL, cfg.timeout)
	if err != nil {
		return err
	}
	portalClient, err := newConsoleClient(cfg.consoleURL, cfg.timeout)
	if err != nil {
		return err
	}

	channelID, adminModel, err := runAdminSmoke(ctx, adminClient, cfg)
	if err != nil {
		return err
	}
	portalModel, err := runPortalSmoke(ctx, portalClient, cfg)
	if err != nil {
		return err
	}

	fmt.Printf("p24_console_smoke=passed console_url=%q admin_model=%q portal_model=%q channel=%q\n", cfg.consoleURL, adminModel, portalModel, channelID)
	return nil
}

func parseConfig(args []string) (config, error) {
	cfg := config{
		consoleURL:    getenvDefault("CONSOLE_URL", "http://127.0.0.1:9505"),
		apiKey:        getenvDefault("API_KEY", "tg-local-dev-key"),
		adminEmail:    getenvDefault("ADMIN_EMAIL", "admin@example.com"),
		adminPassword: getenvDefault("ADMIN_PASSWORD", "admin-local"),
		seedFixtures:  true,
		timeout:       30 * time.Second,
	}
	fs := flag.NewFlagSet("p24-console-smoke", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.consoleURL, "console-url", cfg.consoleURL, "base URL for the running console")
	fs.StringVar(&cfg.apiKey, "api-key", cfg.apiKey, "customer API key used only for Portal session login")
	fs.StringVar(&cfg.model, "model", "", "model expected to be visible in Admin and Portal model lists")
	fs.StringVar(&cfg.adminEmail, "admin-email", cfg.adminEmail, "Admin operator email")
	fs.StringVar(&cfg.adminPassword, "admin-password", cfg.adminPassword, "Admin operator password")
	fs.BoolVar(&cfg.createDerivedKey, "create-derived-key", false, "create and disable a derived Portal API key")
	fs.BoolVar(&cfg.seedFixtures, "seed-fixtures", cfg.seedFixtures, "create minimal Admin fixtures when lists are empty")
	fs.DurationVar(&cfg.timeout, "timeout", cfg.timeout, "overall smoke timeout")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	cfg.consoleURL = strings.TrimRight(strings.TrimSpace(cfg.consoleURL), "/")
	cfg.apiKey = strings.TrimSpace(cfg.apiKey)
	cfg.model = strings.TrimSpace(cfg.model)
	cfg.adminEmail = strings.TrimSpace(cfg.adminEmail)
	cfg.adminPassword = strings.TrimSpace(cfg.adminPassword)
	if cfg.consoleURL == "" {
		return config{}, errors.New("console URL is required")
	}
	if cfg.apiKey == "" {
		return config{}, errors.New("api key is required; pass -api-key or set API_KEY")
	}
	if cfg.adminEmail == "" || cfg.adminPassword == "" {
		return config{}, errors.New("admin email and password are required")
	}
	if cfg.timeout <= 0 {
		return config{}, errors.New("timeout must be positive")
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

func runAdminSmoke(ctx context.Context, c *consoleClient, cfg config) (string, string, error) {
	if err := c.loginAdmin(ctx, cfg.adminEmail, cfg.adminPassword); err != nil {
		return "", "", err
	}
	model, err := c.checkAdminModels(ctx, cfg.model, cfg.seedFixtures)
	if err != nil {
		return "", "", err
	}
	channelID, err := c.checkAdminChannels(ctx, model, cfg.seedFixtures)
	if err != nil {
		return "", "", err
	}
	if err := c.checkAdminCustomers(ctx, model, cfg.seedFixtures); err != nil {
		return "", "", err
	}
	if err := c.checkAdminAPIKeys(ctx); err != nil {
		return "", "", err
	}
	if err := c.checkAdminLogs(ctx); err != nil {
		return "", "", err
	}
	if err := c.checkAdminPlayground(ctx, model, channelID); err != nil {
		return "", "", err
	}
	if err := c.logoutAdmin(ctx); err != nil {
		return "", "", err
	}
	return channelID, model, nil
}

func runPortalSmoke(ctx context.Context, c *consoleClient, cfg config) (string, error) {
	if err := c.loginPortal(ctx, cfg.apiKey); err != nil {
		return "", err
	}
	if err := c.checkPortalDashboard(ctx); err != nil {
		return "", err
	}
	model, err := c.checkPortalModels(ctx, cfg.model)
	if err != nil {
		return "", err
	}
	if err := c.checkPortalPlayground(ctx, model); err != nil {
		return "", err
	}
	if err := c.checkPortalCredits(ctx); err != nil {
		return "", err
	}
	if err := c.checkPortalAPIKeys(ctx, cfg.createDerivedKey, model); err != nil {
		return "", err
	}
	if err := c.checkPortalUsage(ctx); err != nil {
		return "", err
	}
	if err := c.checkPortalTasks(ctx); err != nil {
		return "", err
	}
	if err := c.logoutPortal(ctx, cfg.apiKey); err != nil {
		return "", err
	}
	return model, nil
}

func (c *consoleClient) loginAdmin(ctx context.Context, email string, password string) error {
	var response adminLoginResponse
	raw, err := c.do(ctx, http.MethodPost, "/api/admin/v1/auth/login", map[string]string{"email": email, "password": password}, &response, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("admin login", raw, password, "key_hash", "plaintext_key"); err != nil {
		return err
	}
	if !response.Authenticated || !response.Session.Authenticated || response.CSRFToken == "" {
		return errors.New("admin login response missing authenticated session or csrf token")
	}
	c.csrfToken = response.CSRFToken
	fmt.Printf("p24_console_smoke=admin_login operator=%q\n", response.Session.Email)
	return nil
}

func (c *consoleClient) checkAdminChannels(ctx context.Context, model string, seedFixtures bool) (string, error) {
	var response adminChannelListResponse
	raw, err := c.do(ctx, http.MethodGet, "/api/admin/v1/channels", nil, &response, requestOptions{})
	if err != nil {
		return "", err
	}
	if err := rejectLeakage("admin channels", raw, "plaintext_key", "encrypted_api_key", "key_hash"); err != nil {
		return "", err
	}
	if len(response.Data) == 0 || response.Data[0].ID == "" {
		if !seedFixtures {
			return "", errors.New("admin channel list is empty")
		}
		if err := c.seedAdminChannel(ctx, model); err != nil {
			return "", err
		}
		raw, err = c.do(ctx, http.MethodGet, "/api/admin/v1/channels", nil, &response, requestOptions{})
		if err != nil {
			return "", err
		}
		if len(response.Data) == 0 || response.Data[0].ID == "" {
			return "", errors.New("admin channel list is empty after fixture seed")
		}
	}
	channelID := response.Data[0].ID
	path := "/api/admin/v1/channels/" + url.PathEscape(channelID)
	if raw, err = c.do(ctx, http.MethodGet, path, nil, nil, requestOptions{}); err != nil {
		return "", err
	}
	if err := rejectLeakage("admin channel detail", raw, "plaintext_key", "encrypted_api_key", "key_hash"); err != nil {
		return "", err
	}
	path += "/test"
	if raw, err = c.do(ctx, http.MethodPost, path, nil, nil, adminMutation("p24-smoke-channel-test")); err != nil {
		return "", err
	}
	if err := rejectLeakage("admin channel test", raw, "plaintext_key", "encrypted_api_key", "key_hash"); err != nil {
		return "", err
	}
	fmt.Printf("p24_console_smoke=admin_channels count=%d channel=%q\n", len(response.Data), channelID)
	return channelID, nil
}

func (c *consoleClient) checkAdminModels(ctx context.Context, preferredModel string, seedFixtures bool) (string, error) {
	var response adminModelListResponse
	raw, err := c.do(ctx, http.MethodGet, "/api/admin/v1/models", nil, &response, requestOptions{})
	if err != nil {
		return "", err
	}
	if err := rejectLeakage("admin models", raw, "group_ratio", "model_ratio", "key_hash", "plaintext_key"); err != nil {
		return "", err
	}
	model := chooseAdminModel(response, preferredModel)
	if model == "" {
		if !seedFixtures {
			return "", fmt.Errorf("preferred model %q is not visible in Admin model list", preferredModel)
		}
		model = emptyAs(preferredModel, "gpt-4o-mini")
		if err := c.seedAdminModel(ctx, model); err != nil {
			return "", err
		}
		raw, err = c.do(ctx, http.MethodGet, "/api/admin/v1/models", nil, &response, requestOptions{})
		if err != nil {
			return "", err
		}
		if err := rejectLeakage("admin models", raw, "group_ratio", "model_ratio", "key_hash", "plaintext_key"); err != nil {
			return "", err
		}
		if chooseAdminModel(response, model) == "" {
			return "", fmt.Errorf("model %q is not visible in Admin model list after fixture seed", model)
		}
	}
	encoded := url.PathEscape(model)
	for _, path := range []string{
		"/api/admin/v1/models/" + encoded,
		"/api/admin/v1/models/" + encoded + "/schema-preview",
	} {
		if raw, err = c.do(ctx, http.MethodGet, path, nil, nil, requestOptions{}); err != nil {
			return "", err
		}
		if err := rejectLeakage("admin model read", raw, "group_ratio", "model_ratio", "key_hash", "plaintext_key"); err != nil {
			return "", err
		}
	}
	fmt.Printf("p24_console_smoke=admin_models count=%d model=%q\n", len(response.Data), model)
	return model, nil
}

func (c *consoleClient) checkAdminCustomers(ctx context.Context, model string, seedFixtures bool) error {
	var response adminCustomerAccountListResponse
	raw, err := c.do(ctx, http.MethodGet, "/api/admin/v1/customer-accounts", nil, &response, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("admin customers", raw, "user_group", "plaintext_key", "key_hash"); err != nil {
		return err
	}
	if len(response.Data) == 0 || response.Data[0].CustomerAccountID == "" {
		if !seedFixtures {
			return errors.New("admin customer account list is empty")
		}
		if err := c.seedAdminCustomer(ctx, model); err != nil {
			return err
		}
		raw, err = c.do(ctx, http.MethodGet, "/api/admin/v1/customer-accounts", nil, &response, requestOptions{})
		if err != nil {
			return err
		}
		if len(response.Data) == 0 || response.Data[0].CustomerAccountID == "" {
			return errors.New("admin customer account list is empty after fixture seed")
		}
	}
	accountID := response.Data[0].CustomerAccountID
	encoded := url.PathEscape(accountID)
	for _, path := range []string{
		"/api/admin/v1/customer-accounts/" + encoded,
		"/api/admin/v1/customer-accounts/" + encoded + "/credit-report",
		"/api/admin/v1/customer-accounts/" + encoded + "/usage/export",
		"/api/admin/v1/customer-accounts/" + encoded + "/ledger/export",
	} {
		if raw, err = c.do(ctx, http.MethodGet, path, nil, nil, requestOptions{}); err != nil {
			return err
		}
		if err := rejectLeakage("admin customer account read", raw, "user_group", "plaintext_key", "key_hash", "raw_prompt", "raw_response", "callback_url"); err != nil {
			return err
		}
	}
	fmt.Printf("p24_console_smoke=admin_customers count=%d account=%q\n", len(response.Data), accountID)
	return nil
}

func (c *consoleClient) seedAdminModel(ctx context.Context, model string) error {
	payload := map[string]any{
		"public_model":    model,
		"display_name":    "P24 Smoke Model",
		"protocol":        "native_openai",
		"capability":      "chat",
		"category":        "chat",
		"modalities":      []string{"text"},
		"capabilities":    []string{"chat"},
		"provider_family": "openai_compatible",
		"schema": map[string]any{
			"type":     "object",
			"required": []string{"model"},
			"properties": map[string]any{
				"model": map[string]string{"type": "string"},
			},
		},
		"enabled": true,
	}
	raw, err := c.do(ctx, http.MethodPost, "/api/admin/v1/models", payload, nil, adminMutation("p24-smoke-model"))
	if err != nil {
		return err
	}
	if err := rejectLeakage("admin model fixture", raw, "group_ratio", "model_ratio", "key_hash", "plaintext_key"); err != nil {
		return err
	}
	fmt.Printf("p24_console_smoke=admin_seed_model model=%q\n", model)
	return nil
}

func (c *consoleClient) seedAdminChannel(ctx context.Context, model string) error {
	payload := map[string]any{
		"id":            "channel_p24_smoke",
		"provider_type": "openai_compatible",
		"base_url":      "mock://openai",
		"api_key":       "sk_p24_smoke_should_not_return",
		"enabled":       true,
		"models": []map[string]any{{
			"public_model":       model,
			"upstream_model":     model,
			"health_status":      "healthy",
			"test_status":        "passed",
			"cost_config_status": "configured",
		}},
	}
	raw, err := c.do(ctx, http.MethodPost, "/api/admin/v1/channels", payload, nil, adminMutation("p24-smoke-channel"))
	if err != nil {
		return err
	}
	if err := rejectLeakage("admin channel fixture", raw, "sk_p24_smoke_should_not_return", "encrypted_api_key", "key_hash"); err != nil {
		return err
	}
	fmt.Printf("p24_console_smoke=admin_seed_channel model=%q\n", model)
	return nil
}

func (c *consoleClient) seedAdminCustomer(ctx context.Context, model string) error {
	payload := map[string]any{
		"tenant_id":             "tenant_p24_smoke",
		"tenant_name":           "P24 Smoke Tenant",
		"project_id":            "project_p24_smoke",
		"project_name":          "P24 Smoke Project",
		"display_name":          "P24 Smoke Account",
		"email":                 "p24-smoke@example.com",
		"role":                  "owner",
		"api_key_name":          "P24 Smoke Key",
		"allowed_models":        []string{model},
		"currency":              "USD",
		"initial_credit_micros": 1000000,
		"initial_credit_reason": "P24 smoke fixture",
	}
	raw, err := c.do(ctx, http.MethodPost, "/api/admin/v1/customer-accounts", payload, nil, adminMutation("p24-smoke-customer"))
	if err != nil {
		return err
	}
	if err := rejectLeakage("admin customer fixture", raw, "user_group", "plaintext_key", "key_hash"); err != nil {
		return err
	}
	fmt.Printf("p24_console_smoke=admin_seed_customer model=%q\n", model)
	return nil
}

func (c *consoleClient) checkAdminAPIKeys(ctx context.Context) error {
	var response adminAPIKeyListResponse
	raw, err := c.do(ctx, http.MethodGet, "/api/admin/v1/api-keys", nil, &response, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("admin api keys", raw, "plaintext_key", "key_hash"); err != nil {
		return err
	}
	if len(response.Data) == 0 {
		return errors.New("admin api key list is empty")
	}
	fmt.Printf("p24_console_smoke=admin_api_keys count=%d\n", len(response.Data))
	return nil
}

func (c *consoleClient) checkAdminLogs(ctx context.Context) error {
	var usage adminUsageLogListResponse
	raw, err := c.do(ctx, http.MethodGet, "/api/admin/v1/usage-logs?limit=5", nil, &usage, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("admin usage logs", raw, "raw_prompt", "raw_response", "callback_url", "plaintext_key", "key_hash"); err != nil {
		return err
	}
	if len(usage.Data) > 0 && usage.Data[0].RequestID != "" {
		path := "/api/admin/v1/usage-logs/" + url.PathEscape(usage.Data[0].RequestID)
		if raw, err = c.do(ctx, http.MethodGet, path, nil, nil, requestOptions{}); err != nil {
			return err
		}
		if err := rejectLeakage("admin usage log detail", raw, "raw_prompt", "raw_response", "callback_url", "plaintext_key", "key_hash"); err != nil {
			return err
		}
	}

	var tasks adminTaskLogListResponse
	raw, err = c.do(ctx, http.MethodGet, "/api/admin/v1/task-logs?limit=5", nil, &tasks, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("admin task logs", raw, "raw_prompt", "raw_response", "callback_url", "plaintext_key", "key_hash"); err != nil {
		return err
	}
	if len(tasks.Data) > 0 && tasks.Data[0].TaskID != "" {
		path := "/api/admin/v1/task-logs/" + url.PathEscape(tasks.Data[0].TaskID)
		if raw, err = c.do(ctx, http.MethodGet, path, nil, nil, requestOptions{}); err != nil {
			return err
		}
		if err := rejectLeakage("admin task log detail", raw, "raw_prompt", "raw_response", "callback_url", "plaintext_key", "key_hash"); err != nil {
			return err
		}
	}
	fmt.Printf("p24_console_smoke=admin_logs usage=%d tasks=%d\n", len(usage.Data), len(tasks.Data))
	return nil
}

func (c *consoleClient) checkAdminPlayground(ctx context.Context, model string, channelID string) error {
	payload := map[string]any{
		"model":      model,
		"channel_id": channelID,
		"mode":       "chat",
		"debug":      true,
		"payload": map[string]any{
			"messages": []map[string]string{{"role": "user", "content": "p24 smoke prompt must not echo"}},
		},
	}
	raw, err := c.do(ctx, http.MethodPost, "/api/admin/v1/playground/run", payload, nil, adminMutation("p24-smoke-playground"))
	if err != nil {
		return err
	}
	if err := rejectLeakage("admin playground", raw, "p24 smoke prompt must not echo", "plaintext_key", "key_hash"); err != nil {
		return err
	}
	fmt.Println("p24_console_smoke=admin_playground")
	return nil
}

func (c *consoleClient) logoutAdmin(ctx context.Context) error {
	if _, err := c.do(ctx, http.MethodPost, "/api/admin/v1/auth/logout", nil, nil, requestOptions{CSRF: true}); err != nil {
		return err
	}
	fmt.Println("p24_console_smoke=admin_logout")
	return nil
}

func (c *consoleClient) loginPortal(ctx context.Context, apiKey string) error {
	var response portalLoginResponse
	raw, err := c.do(ctx, http.MethodPost, "/api/portal/v1/auth/api-key-login", map[string]string{"api_key": apiKey}, &response, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal login", raw, apiKey, "key_hash", "plaintext_key"); err != nil {
		return err
	}
	if !response.Authenticated || !response.Session.Authenticated || response.CSRFToken == "" {
		return errors.New("portal login response missing authenticated session or csrf token")
	}
	c.csrfToken = response.CSRFToken
	fmt.Printf("p24_console_smoke=portal_login tenant=%q project=%q api_key_id=%q\n", response.Session.TenantID, response.Session.ProjectID, response.Session.APIKeyID)
	return nil
}

func (c *consoleClient) checkPortalDashboard(ctx context.Context) error {
	raw, err := c.do(ctx, http.MethodGet, "/api/portal/v1/dashboard", nil, nil, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal dashboard", raw, "provider_secret", "credential", "key_hash", "plaintext_key"); err != nil {
		return err
	}
	fmt.Println("p24_console_smoke=portal_dashboard")
	return nil
}

func (c *consoleClient) checkPortalModels(ctx context.Context, preferredModel string) (string, error) {
	var response portalModelListResponse
	raw, err := c.do(ctx, http.MethodGet, "/api/portal/v1/models", nil, &response, requestOptions{})
	if err != nil {
		return "", err
	}
	if err := rejectLeakage("portal models", raw, "model_group", "model_ratio", "key_hash", "plaintext_key"); err != nil {
		return "", err
	}
	model := choosePortalModel(response, preferredModel)
	if model == "" {
		return "", fmt.Errorf("preferred model %q is not visible in Portal model list", preferredModel)
	}
	encoded := url.PathEscape(model)
	for _, path := range []string{
		"/api/portal/v1/models/" + encoded,
		"/api/portal/v1/models/" + encoded + "/schema",
	} {
		if raw, err = c.do(ctx, http.MethodGet, path, nil, nil, requestOptions{}); err != nil {
			return "", err
		}
		if err := rejectLeakage("portal model read", raw, "model_group", "model_ratio", "key_hash", "plaintext_key"); err != nil {
			return "", err
		}
	}
	fmt.Printf("p24_console_smoke=portal_models count=%d model=%q\n", len(response.Data), model)
	return model, nil
}

func (c *consoleClient) checkPortalPlayground(ctx context.Context, model string) error {
	payload := map[string]any{
		"model": model,
		"mode":  "chat",
		"debug": true,
		"payload": map[string]any{
			"messages": []map[string]string{{"role": "user", "content": "p24 portal prompt must not echo"}},
		},
	}
	raw, err := c.do(ctx, http.MethodPost, "/api/portal/v1/playground/run", payload, nil, requestOptions{CSRF: true})
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal playground", raw, "p24 portal prompt must not echo", "plaintext_key", "key_hash"); err != nil {
		return err
	}
	fmt.Println("p24_console_smoke=portal_playground")
	return nil
}

func (c *consoleClient) checkPortalCredits(ctx context.Context) error {
	var credits portalCreditsResponse
	raw, err := c.do(ctx, http.MethodGet, "/api/portal/v1/credits", nil, &credits, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal credits", raw, "payment", "subscription", "redemption", "key_hash", "plaintext_key"); err != nil {
		return err
	}
	if len(credits.Data) == 0 {
		return errors.New("portal credits returned no buckets")
	}
	var ledger portalCreditLedgerResponse
	raw, err = c.do(ctx, http.MethodGet, "/api/portal/v1/credits/ledger?limit=5", nil, &ledger, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal credit ledger", raw, "payment", "subscription", "redemption", "key_hash", "plaintext_key"); err != nil {
		return err
	}
	fmt.Printf("p24_console_smoke=portal_credits buckets=%d ledger=%d\n", len(credits.Data), len(ledger.Items))
	return nil
}

func (c *consoleClient) checkPortalAPIKeys(ctx context.Context, createDerived bool, model string) error {
	var response portalAPIKeyListResponse
	raw, err := c.do(ctx, http.MethodGet, "/api/portal/v1/api-keys", nil, &response, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal api keys", raw, "key_hash", "plaintext_key"); err != nil {
		return err
	}
	if createDerived {
		payload := map[string]any{
			"name":           fmt.Sprintf("p24-console-smoke-%d", time.Now().UTC().Unix()),
			"allowed_models": []string{model},
		}
		var created portalAPIKeyCreateResponse
		raw, err = c.do(ctx, http.MethodPost, "/api/portal/v1/api-keys", payload, &created, requestOptions{CSRF: true})
		if err != nil {
			return err
		}
		if created.APIKey.ID == "" || created.PlaintextKey == "" {
			return errors.New("portal derived key create response missing id or plaintext key")
		}
		if strings.Contains(strings.ToLower(string(raw)), "key_hash") {
			return errors.New("portal derived key create leaked key hash")
		}
		path := "/api/portal/v1/api-keys/" + url.PathEscape(created.APIKey.ID) + "/disable"
		raw, err = c.do(ctx, http.MethodPost, path, nil, nil, requestOptions{CSRF: true})
		if err != nil {
			return err
		}
		if err := rejectLeakage("portal derived key disable", raw, "key_hash", created.PlaintextKey); err != nil {
			return err
		}
		fmt.Printf("p24_console_smoke=portal_derived_key key_id=%q\n", created.APIKey.ID)
	}
	fmt.Printf("p24_console_smoke=portal_api_keys count=%d\n", len(response.Data))
	return nil
}

func (c *consoleClient) checkPortalUsage(ctx context.Context) error {
	var usage portalUsageResponse
	raw, err := c.do(ctx, http.MethodGet, "/api/portal/v1/usage?limit=5", nil, &usage, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal usage", raw, "provider_cost", "failed_settlement", "repair", "key_hash", "plaintext_key"); err != nil {
		return err
	}
	var exported portalUsageExportResponse
	raw, err = c.do(ctx, http.MethodGet, "/api/portal/v1/usage/export?limit=5", nil, &exported, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal usage export", raw, "provider_cost", "raw_prompt", "raw_response", "callback_url", "key_hash", "plaintext_key"); err != nil {
		return err
	}
	if exported.Filename == "" || len(exported.SafeFields) == 0 {
		return errors.New("portal usage export missing filename or safe fields")
	}
	fmt.Printf("p24_console_smoke=portal_usage requests=%d items=%d export=%q\n", usage.Totals.Requests, len(usage.Items), exported.Filename)
	return nil
}

func (c *consoleClient) checkPortalTasks(ctx context.Context) error {
	var response portalTaskListResponse
	raw, err := c.do(ctx, http.MethodGet, "/api/portal/v1/tasks?limit=5", nil, &response, requestOptions{})
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal tasks", raw, "raw_prompt", "callback_url", "secret-token", "provider_secret", "password", "credential"); err != nil {
		return err
	}
	fmt.Printf("p24_console_smoke=portal_tasks count=%d\n", len(response.Data))
	return nil
}

func (c *consoleClient) logoutPortal(ctx context.Context, apiKey string) error {
	if _, err := c.do(ctx, http.MethodPost, "/api/portal/v1/auth/logout", nil, nil, requestOptions{CSRF: true}); err != nil {
		return err
	}
	raw, err := c.do(ctx, http.MethodGet, "/api/portal/v1/dashboard", nil, nil, requestOptions{Status: http.StatusUnauthorized})
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal unauthorized response", raw, apiKey, "key_hash", "plaintext_key"); err != nil {
		return err
	}
	fmt.Println("p24_console_smoke=portal_logout")
	return nil
}

func (c *consoleClient) do(ctx context.Context, method string, path string, payload any, out any, opts requestOptions) ([]byte, error) {
	status := opts.Status
	if status == 0 {
		status = http.StatusOK
	}
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal %s %s: %w", method, path, err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create %s %s request: %w", method, path, err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if opts.CSRF {
		req.Header.Set(csrfHeaderName, c.csrfToken)
	}
	if opts.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", opts.IdempotencyKey)
	}
	if opts.Reason != "" {
		req.Header.Set("X-Reason", opts.Reason)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s failed: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read %s %s response: %w", method, path, err)
	}
	if resp.StatusCode != status {
		return raw, fmt.Errorf("%s %s status=%d want=%d body=%s", method, path, resp.StatusCode, status, compactBody(raw))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return raw, fmt.Errorf("decode %s %s response: %w body=%s", method, path, err, compactBody(raw))
		}
	}
	return raw, nil
}

func adminMutation(name string) requestOptions {
	return requestOptions{
		CSRF:           true,
		Reason:         "P24 console smoke",
		IdempotencyKey: fmt.Sprintf("%s-%d", name, time.Now().UTC().UnixNano()),
	}
}

func chooseAdminModel(response adminModelListResponse, preferredModel string) string {
	if preferredModel == "" {
		if len(response.Data) == 0 {
			return ""
		}
		return response.Data[0].PublicModel
	}
	for _, model := range response.Data {
		if model.PublicModel == preferredModel {
			return preferredModel
		}
	}
	return ""
}

func choosePortalModel(response portalModelListResponse, preferredModel string) string {
	if preferredModel == "" {
		if len(response.Data) == 0 {
			return ""
		}
		return response.Data[0].ID
	}
	for _, model := range response.Data {
		if model.ID == preferredModel {
			return preferredModel
		}
	}
	return ""
}

func rejectLeakage(label string, raw []byte, forbidden ...string) error {
	body := strings.ToLower(string(raw))
	for _, value := range forbidden {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.Contains(body, strings.ToLower(value)) {
			return fmt.Errorf("%s response contains forbidden marker %q", label, value)
		}
	}
	return nil
}

func compactBody(raw []byte) string {
	body := strings.TrimSpace(string(raw))
	if len(body) > 500 {
		return body[:500] + "..."
	}
	return body
}

func getenvDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func emptyAs(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
