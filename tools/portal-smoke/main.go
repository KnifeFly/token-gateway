// Command portal-smoke validates the customer-facing Portal API contract
// against a running gateway without requiring admin credentials or external
// JSON tooling.
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
	"net/url"
	"os"
	"strings"
	"time"
)

type config struct {
	gatewayURL       string
	apiKey           string
	model            string
	createDerivedKey bool
	timeout          time.Duration
}

type smokeClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type modelListResponse struct {
	Object string `json:"object"`
	Data   []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type schemaResponse struct {
	Model  string         `json:"model"`
	Schema map[string]any `json:"schema"`
}

type creditsResponse struct {
	Success bool `json:"success"`
	Data    map[string]struct {
		RemainingCredits float64 `json:"remaining_credits"`
		UsedCredits      float64 `json:"used_credits"`
		Currency         string  `json:"currency"`
	} `json:"data"`
}

type usageResponse struct {
	Currency string `json:"currency"`
	Totals   struct {
		Requests    int64   `json:"requests"`
		CreditsUsed float64 `json:"credits_used"`
	} `json:"totals"`
	Items []map[string]any `json:"items"`
}

type apiKeyListResponse struct {
	Data []struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		Enabled       bool     `json:"enabled"`
		AllowedModels []string `json:"allowed_models"`
	} `json:"data"`
}

type apiKeyCreateResponse struct {
	APIKey struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	} `json:"api_key"`
	PlaintextKey string `json:"plaintext_key"`
}

type taskListResponse struct {
	Data []map[string]any `json:"data"`
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "portal_smoke=failed error=%q\n", err.Error())
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

	client, err := newSmokeClient(cfg)
	if err != nil {
		return err
	}

	model, err := client.checkModels(ctx, cfg.model)
	if err != nil {
		return err
	}
	if err := client.checkSchema(ctx, model); err != nil {
		return err
	}
	if err := client.checkCredits(ctx); err != nil {
		return err
	}
	if err := client.checkUsage(ctx); err != nil {
		return err
	}
	if err := client.checkAPIKeys(ctx); err != nil {
		return err
	}
	if cfg.createDerivedKey {
		if err := client.checkDerivedAPIKeyLifecycle(ctx, model); err != nil {
			return err
		}
	}
	if err := client.checkTasks(ctx); err != nil {
		return err
	}

	fmt.Printf("portal_smoke=passed gateway_url=%q model=%q\n", cfg.gatewayURL, model)
	return nil
}

func parseConfig(args []string) (config, error) {
	cfg := config{
		gatewayURL: getenvDefault("GATEWAY_URL", "http://127.0.0.1:9501"),
		apiKey:     getenvDefault("API_KEY", os.Getenv("TOKEN_GATEWAY_RC_API_KEY")),
		timeout:    30 * time.Second,
	}
	fs := flag.NewFlagSet("portal-smoke", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.gatewayURL, "gateway-url", cfg.gatewayURL, "base URL for the running gateway")
	fs.StringVar(&cfg.apiKey, "api-key", cfg.apiKey, "customer Bearer API key")
	fs.StringVar(&cfg.model, "model", "", "visible model to verify; defaults to the first Portal model")
	fs.BoolVar(&cfg.createDerivedKey, "create-derived-key", false, "create and disable a derived Portal API key")
	fs.DurationVar(&cfg.timeout, "timeout", cfg.timeout, "overall smoke timeout")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	cfg.gatewayURL = strings.TrimRight(strings.TrimSpace(cfg.gatewayURL), "/")
	cfg.apiKey = strings.TrimSpace(cfg.apiKey)
	cfg.model = strings.TrimSpace(cfg.model)
	if cfg.gatewayURL == "" {
		return config{}, errors.New("gateway URL is required")
	}
	if cfg.apiKey == "" {
		return config{}, errors.New("api key is required; pass -api-key or set API_KEY")
	}
	if cfg.timeout <= 0 {
		return config{}, errors.New("timeout must be positive")
	}
	return cfg, nil
}

func newSmokeClient(cfg config) (*smokeClient, error) {
	parsed, err := url.Parse(cfg.gatewayURL)
	if err != nil {
		return nil, fmt.Errorf("parse gateway URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("gateway URL must include scheme and host: %q", cfg.gatewayURL)
	}
	return &smokeClient{
		baseURL:    cfg.gatewayURL,
		apiKey:     cfg.apiKey,
		httpClient: &http.Client{Timeout: cfg.timeout},
	}, nil
}

func (c *smokeClient) checkModels(ctx context.Context, preferredModel string) (string, error) {
	var response modelListResponse
	raw, err := c.do(ctx, http.MethodGet, "/v1/portal/models", nil, &response)
	if err != nil {
		return "", err
	}
	if err := rejectLeakage("portal models", raw, "provider_secret", "credential", "key_hash", "plaintext_key"); err != nil {
		return "", err
	}
	if response.Object != "list" || len(response.Data) == 0 {
		return "", fmt.Errorf("portal models returned no visible models")
	}
	if preferredModel == "" {
		model := response.Data[0].ID
		fmt.Printf("portal_smoke=models count=%d model=%q\n", len(response.Data), model)
		return model, nil
	}
	for _, model := range response.Data {
		if model.ID == preferredModel {
			fmt.Printf("portal_smoke=models count=%d model=%q\n", len(response.Data), preferredModel)
			return preferredModel, nil
		}
	}
	return "", fmt.Errorf("preferred model %q is not visible in Portal model list", preferredModel)
}

func (c *smokeClient) checkSchema(ctx context.Context, model string) error {
	var response schemaResponse
	path := "/v1/portal/models/" + url.PathEscape(model) + "/schema"
	raw, err := c.do(ctx, http.MethodGet, path, nil, &response)
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal model schema", raw, "provider_secret", "credential", "key_hash", "plaintext_key"); err != nil {
		return err
	}
	if response.Model == "" || len(response.Schema) == 0 {
		return fmt.Errorf("portal model schema for %q is empty", model)
	}
	fmt.Printf("portal_smoke=schema model=%q\n", response.Model)
	return nil
}

func (c *smokeClient) checkCredits(ctx context.Context) error {
	var response creditsResponse
	raw, err := c.do(ctx, http.MethodGet, "/v1/portal/credits", nil, &response)
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal credits", raw, "provider_cost", "failed_settlement", "repair", "key_hash", "plaintext_key"); err != nil {
		return err
	}
	if !response.Success || len(response.Data) == 0 {
		return fmt.Errorf("portal credits returned no buckets")
	}
	fmt.Printf("portal_smoke=credits buckets=%d\n", len(response.Data))
	return nil
}

func (c *smokeClient) checkUsage(ctx context.Context) error {
	var response usageResponse
	raw, err := c.do(ctx, http.MethodGet, "/v1/portal/usage?limit=5", nil, &response)
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal usage", raw, "provider_cost", "failed_settlement", "repair", "key_hash", "plaintext_key"); err != nil {
		return err
	}
	fmt.Printf("portal_smoke=usage requests=%d items=%d\n", response.Totals.Requests, len(response.Items))
	return nil
}

func (c *smokeClient) checkAPIKeys(ctx context.Context) error {
	var response apiKeyListResponse
	raw, err := c.do(ctx, http.MethodGet, "/v1/portal/api-keys", nil, &response)
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal api key list", raw, "key_hash", "plaintext_key"); err != nil {
		return err
	}
	fmt.Printf("portal_smoke=api_keys count=%d\n", len(response.Data))
	return nil
}

func (c *smokeClient) checkDerivedAPIKeyLifecycle(ctx context.Context, model string) error {
	payload := map[string]any{
		"name":           fmt.Sprintf("p9-smoke-%d", time.Now().UTC().Unix()),
		"allowed_models": []string{model},
	}
	var created apiKeyCreateResponse
	if _, err := c.do(ctx, http.MethodPost, "/v1/portal/api-keys", payload, &created); err != nil {
		return err
	}
	if created.APIKey.ID == "" || created.PlaintextKey == "" {
		return fmt.Errorf("derived api key create response is missing id or plaintext key")
	}
	if err := c.checkAPIKeys(ctx); err != nil {
		return err
	}

	var disabled struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if _, err := c.do(ctx, http.MethodPost, "/v1/portal/api-keys/"+url.PathEscape(created.APIKey.ID)+"/disable", nil, &disabled); err != nil {
		return err
	}
	if disabled.ID != created.APIKey.ID || disabled.Enabled {
		return fmt.Errorf("derived api key disable response = id:%q enabled:%v", disabled.ID, disabled.Enabled)
	}
	fmt.Printf("portal_smoke=derived_key_lifecycle key_id=%q\n", created.APIKey.ID)
	return nil
}

func (c *smokeClient) checkTasks(ctx context.Context) error {
	var response taskListResponse
	raw, err := c.do(ctx, http.MethodGet, "/v1/portal/tasks?limit=5", nil, &response)
	if err != nil {
		return err
	}
	if err := rejectLeakage("portal tasks", raw, "secret-token", "provider_secret", "password", "credential"); err != nil {
		return err
	}
	fmt.Printf("portal_smoke=tasks count=%d\n", len(response.Data))
	return nil
}

func (c *smokeClient) do(ctx context.Context, method string, path string, payload any, out any) ([]byte, error) {
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
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
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
	if resp.StatusCode != http.StatusOK {
		return raw, fmt.Errorf("%s %s status=%d body=%s", method, path, resp.StatusCode, compactBody(raw))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return raw, fmt.Errorf("decode %s %s response: %w body=%s", method, path, err, compactBody(raw))
		}
	}
	return raw, nil
}

func rejectLeakage(label string, raw []byte, forbidden ...string) error {
	body := strings.ToLower(string(raw))
	for _, value := range forbidden {
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
