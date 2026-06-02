package adminhttp

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	adminrepo "github.com/KnifeFly/token-gateway/internal/app/admin/repository"
	adminservice "github.com/KnifeFly/token-gateway/internal/app/admin/service"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

func TestAdminAuthSessionAndCSRF(t *testing.T) {
	mux, _ := testMux(t)
	cookie, csrf := loginOperator(t, mux, "admin@example.com", "admin-local")

	me := request(t, mux, http.MethodGet, "/api/admin/v1/auth/me", "", cookie, "")
	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d body=%s", me.Code, me.Body.String())
	}
	var meBody struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("decode me body: %v", err)
	}
	csrf = meBody.CSRFToken

	missingCSRF := request(t, mux, http.MethodPost, "/api/admin/v1/tenants", `{"name":"Denied"}`, cookie, "")
	if missingCSRF.Code != http.StatusUnauthorized || !strings.Contains(missingCSRF.Body.String(), "csrf token is required") {
		t.Fatalf("missing csrf status=%d body=%s", missingCSRF.Code, missingCSRF.Body.String())
	}

	create := httptest.NewRequest(http.MethodPost, "/api/admin/v1/tenants", strings.NewReader(`{"name":"Acme"}`))
	create.Header.Set("Content-Type", "application/json")
	create.Header.Set(csrfHeaderName, csrf)
	create.Header.Set("Idempotency-Key", "idem_tenant")
	create.Header.Set("X-Reason", "create tenant")
	create.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, create)
	if rec.Code != http.StatusOK {
		t.Fatalf("create tenant status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminRBACDenyAndAuditQuery(t *testing.T) {
	mux, svc := testMux(t)
	if _, err := svc.EnsureBootstrapOperator(context.Background(), "viewer@example.com", "viewer-local", []adminapp.Role{adminapp.RoleReadOnly}); err != nil {
		t.Fatalf("EnsureBootstrapOperator(viewer) error = %v", err)
	}
	cookie, csrf := loginOperator(t, mux, "viewer@example.com", "viewer-local")

	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/tenants", strings.NewReader(`{"name":"Denied"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeaderName, csrf)
	req.Header.Set("Idempotency-Key", "idem_denied")
	req.Header.Set("X-Reason", "permission test")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("deny status=%d body=%s", rec.Code, rec.Body.String())
	}

	adminCookie, _ := loginOperator(t, mux, "admin@example.com", "admin-local")
	audit := request(t, mux, http.MethodGet, "/api/admin/v1/audit?resource=tenant", "", adminCookie, "")
	if audit.Code != http.StatusOK || !strings.Contains(audit.Body.String(), `"error_code":"forbidden"`) {
		t.Fatalf("audit status=%d body=%s", audit.Code, audit.Body.String())
	}
}

func TestAdminChannelWorkflows(t *testing.T) {
	mux, _ := testMux(t)
	cookie, csrf := loginOperator(t, mux, "admin@example.com", "admin-local")

	create := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels", `{
		"id":"channel_primary",
		"provider_type":"openai",
		"base_url":"https://provider.example/v1",
		"api_key":"sk_should_not_return",
		"enabled":true,
		"models":[{"public_model":"gpt-public","upstream_model":"gpt-upstream","health_status":"healthy","test_status":"passed","cost_config_status":"configured"}]
	}`, cookie, csrf, "channel_create")
	if create.Code != http.StatusOK {
		t.Fatalf("create channel status=%d body=%s", create.Code, create.Body.String())
	}
	if strings.Contains(create.Body.String(), "sk_should_not_return") || strings.Contains(create.Body.String(), "encrypted_api_key") {
		t.Fatalf("create channel leaked credential material: %s", create.Body.String())
	}

	detail := request(t, mux, http.MethodGet, "/api/admin/v1/channels/channel_primary", "", cookie, "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"model_count":1`) {
		t.Fatalf("channel detail status=%d body=%s", detail.Code, detail.Body.String())
	}

	testResult := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels/channel_primary/test", "", cookie, csrf, "channel_test")
	if testResult.Code != http.StatusOK || !strings.Contains(testResult.Body.String(), `"status":"ready"`) {
		t.Fatalf("channel test status=%d body=%s", testResult.Code, testResult.Body.String())
	}

	rotate := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels/channel_primary/rotate-credential", `{"api_key":"sk_rotated_should_not_return"}`, cookie, csrf, "channel_rotate")
	if rotate.Code != http.StatusOK {
		t.Fatalf("rotate status=%d body=%s", rotate.Code, rotate.Body.String())
	}
	if strings.Contains(rotate.Body.String(), "sk_rotated_should_not_return") || strings.Contains(rotate.Body.String(), "encrypted_api_key") {
		t.Fatalf("rotate leaked credential material: %s", rotate.Body.String())
	}

	preview := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels/channel_primary/sync-preview", `{
		"upstream_models":[{"public_model":"gpt-public","upstream_model":"gpt-upstream-v2","cost_config_status":"configured"}]
	}`, cookie, csrf, "channel_sync_preview")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), `"changed"`) {
		t.Fatalf("sync preview status=%d body=%s", preview.Code, preview.Body.String())
	}

	apply := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels/channel_primary/sync-apply", `{
		"upstream_models":[{"public_model":"gpt-public","upstream_model":"gpt-upstream-v2","cost_config_status":"configured"}]
	}`, cookie, csrf, "channel_sync_apply")
	if apply.Code != http.StatusOK || !strings.Contains(apply.Body.String(), `"gpt-upstream-v2"`) {
		t.Fatalf("sync apply status=%d body=%s", apply.Code, apply.Body.String())
	}

	health := request(t, mux, http.MethodGet, "/api/admin/v1/channels/channel_primary/health-events", "", cookie, "")
	if health.Code != http.StatusOK || !strings.Contains(health.Body.String(), `"channel_id":"channel_primary"`) {
		t.Fatalf("health status=%d body=%s", health.Code, health.Body.String())
	}
}

func TestAdminModelWorkflows(t *testing.T) {
	mux, _ := testMux(t)
	cookie, csrf := loginOperator(t, mux, "admin@example.com", "admin-local")

	create := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/models", `{
		"public_model":"gpt-public",
		"display_name":"GPT Public",
		"protocol":"native_openai",
		"capability":"chat",
		"category":"chat",
		"modalities":["text"],
		"capabilities":["chat","tool"],
		"metadata":{"credential":"model-secret"},
		"schema":{"type":"object","required":["model"]},
		"enabled":true
	}`, cookie, csrf, "model_create")
	if create.Code != http.StatusOK {
		t.Fatalf("create model status=%d body=%s", create.Code, create.Body.String())
	}
	if strings.Contains(create.Body.String(), "model-secret") || strings.Contains(create.Body.String(), "ratio") {
		t.Fatalf("create model exposed secret or cut-scope field: %s", create.Body.String())
	}

	price := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/pricing", `{
		"public_model":"gpt-public",
		"category":"chat",
		"currency":"CNY",
		"components":[{"unit":"input_token","micros_per_unit":2}],
		"enabled":true
	}`, cookie, csrf, "model_price")
	if price.Code != http.StatusOK {
		t.Fatalf("price status=%d body=%s", price.Code, price.Body.String())
	}

	channel := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels", `{
		"id":"channel_primary",
		"provider_type":"openai",
		"base_url":"https://provider.example/v1",
		"api_key":"sk_should_not_return",
		"enabled":true,
		"models":[{"public_model":"gpt-public","upstream_model":"gpt-upstream","health_status":"healthy","test_status":"passed","cost_config_status":"configured"}]
	}`, cookie, csrf, "model_channel")
	if channel.Code != http.StatusOK {
		t.Fatalf("channel status=%d body=%s", channel.Code, channel.Body.String())
	}

	detail := request(t, mux, http.MethodGet, "/api/admin/v1/models/gpt-public", "", cookie, "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"pricing_summary"`) || !strings.Contains(detail.Body.String(), `"channel_coverage"`) {
		t.Fatalf("model detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	if strings.Contains(detail.Body.String(), "sk_should_not_return") || strings.Contains(detail.Body.String(), "ratio") {
		t.Fatalf("model detail exposed secret or cut-scope field: %s", detail.Body.String())
	}

	channels := request(t, mux, http.MethodGet, "/api/admin/v1/models/gpt-public/channels", "", cookie, "")
	if channels.Code != http.StatusOK || !strings.Contains(channels.Body.String(), `"upstream_model":"gpt-upstream"`) {
		t.Fatalf("model channels status=%d body=%s", channels.Code, channels.Body.String())
	}

	schema := request(t, mux, http.MethodGet, "/api/admin/v1/models/gpt-public/schema-preview", "", cookie, "")
	if schema.Code != http.StatusOK || !strings.Contains(schema.Body.String(), `"model":"gpt-public"`) {
		t.Fatalf("schema status=%d body=%s", schema.Code, schema.Body.String())
	}

	preview := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/models/sync-preview", `{
		"models":[{"public_model":"gpt-public","display_name":"GPT Public Updated","protocol":"native_openai","capability":"chat"},{"public_model":"image-public","protocol":"unified","capability":"image"}]
	}`, cookie, csrf, "model_sync_preview")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), `"added"`) || !strings.Contains(preview.Body.String(), `"changed"`) {
		t.Fatalf("model sync preview status=%d body=%s", preview.Code, preview.Body.String())
	}

	deprecate := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/models/gpt-public/deprecate", "", cookie, csrf, "model_deprecate")
	if deprecate.Code != http.StatusOK || !strings.Contains(deprecate.Body.String(), `"deprecated":true`) {
		t.Fatalf("deprecate status=%d body=%s", deprecate.Code, deprecate.Body.String())
	}

	disable := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/models/gpt-public/disable", "", cookie, csrf, "model_disable")
	if disable.Code != http.StatusOK || !strings.Contains(disable.Body.String(), `"enabled":false`) {
		t.Fatalf("disable status=%d body=%s", disable.Code, disable.Body.String())
	}
}

func TestAdminPlaygroundWorkflows(t *testing.T) {
	mux, _ := testMux(t)
	cookie, csrf := loginOperator(t, mux, "admin@example.com", "admin-local")

	model := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/models", `{
		"public_model":"gpt-public",
		"display_name":"GPT Public",
		"protocol":"native_openai",
		"capability":"chat",
		"schema":{"type":"object","required":["model","input"],"properties":{"model":{"type":"string"},"input":{"type":"string"}}},
		"enabled":true
	}`, cookie, csrf, "playground_model")
	if model.Code != http.StatusOK {
		t.Fatalf("playground model status=%d body=%s", model.Code, model.Body.String())
	}

	channel := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/channels", `{
		"id":"channel_primary",
		"provider_type":"openai",
		"base_url":"https://provider.example/v1",
		"api_key":"sk_should_not_return",
		"enabled":true,
		"models":[{"public_model":"gpt-public","upstream_model":"gpt-upstream","health_status":"healthy","test_status":"passed","cost_config_status":"configured"}]
	}`, cookie, csrf, "playground_channel")
	if channel.Code != http.StatusOK {
		t.Fatalf("playground channel status=%d body=%s", channel.Code, channel.Body.String())
	}

	run := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/playground/run", `{
		"model":"gpt-public",
		"channel_id":"channel_primary",
		"mode":"chat",
		"debug":true,
		"payload":{"model":"gpt-public","input":"do not return prompt","api_key":"sk_leak"}
	}`, cookie, csrf, "playground_run")
	if run.Code != http.StatusOK || !strings.Contains(run.Body.String(), `"scope":"admin"`) || !strings.Contains(run.Body.String(), `"channel_id":"channel_primary"`) || !strings.Contains(run.Body.String(), `"status":"ready"`) {
		t.Fatalf("playground run status=%d body=%s", run.Code, run.Body.String())
	}
	if strings.Contains(run.Body.String(), "do not return prompt") || strings.Contains(run.Body.String(), "sk_leak") || strings.Contains(run.Body.String(), "sk_should_not_return") {
		t.Fatalf("playground run leaked unsafe content: %s", run.Body.String())
	}

	preview := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/playground/import-preview", `{
		"payload":{"model":"gpt-public","input":"secret prompt","api_key":"sk_import"}
	}`, cookie, csrf, "playground_import")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), `"redacted_fields"`) {
		t.Fatalf("playground import status=%d body=%s", preview.Code, preview.Body.String())
	}
	if strings.Contains(preview.Body.String(), "secret prompt") || strings.Contains(preview.Body.String(), "sk_import") {
		t.Fatalf("playground import leaked unsafe content: %s", preview.Body.String())
	}

	exported := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/playground/export", `{
		"model":"gpt-public",
		"mode":"chat",
		"payload":{"model":"gpt-public","input":"secret export","credential":"private"}
	}`, cookie, csrf, "playground_export")
	if exported.Code != http.StatusOK || !strings.Contains(exported.Body.String(), `"omitted_fields"`) {
		t.Fatalf("playground export status=%d body=%s", exported.Code, exported.Body.String())
	}
	if strings.Contains(exported.Body.String(), "secret export") || strings.Contains(exported.Body.String(), "private") {
		t.Fatalf("playground export leaked unsafe content: %s", exported.Body.String())
	}
}

func TestAdminCustomerAccountWorkflows(t *testing.T) {
	mux, _ := testMux(t)
	cookie, csrf := loginOperator(t, mux, "admin@example.com", "admin-local")

	create := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/customer-accounts", `{
		"tenant_name":"Acme",
		"project_name":"Production",
		"api_key_name":"primary",
		"allowed_models":["gpt-public"],
		"currency":"CNY",
		"initial_credit_micros":5000000,
		"initial_credit_reason":"seed credits"
	}`, cookie, csrf, "customer_create")
	if create.Code != http.StatusOK {
		t.Fatalf("create customer status=%d body=%s", create.Code, create.Body.String())
	}
	if strings.Contains(create.Body.String(), "plaintext_key") || strings.Contains(create.Body.String(), "key_hash") || strings.Contains(create.Body.String(), "user_group") {
		t.Fatalf("create customer exposed secret or cut-scope field: %s", create.Body.String())
	}
	var created adminapp.CustomerAccountDetail
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode customer detail: %v", err)
	}
	accountID := created.Account.CustomerAccountID
	if accountID == "" || created.Account.ActiveAPIKeyCount != 1 {
		t.Fatalf("created customer = %#v", created)
	}

	list := request(t, mux, http.MethodGet, "/api/admin/v1/customer-accounts?keyword=acme", "", cookie, "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), accountID) {
		t.Fatalf("customer list status=%d body=%s", list.Code, list.Body.String())
	}

	detail := request(t, mux, http.MethodGet, "/api/admin/v1/customer-accounts/"+accountID, "", cookie, "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"api_keys"`) {
		t.Fatalf("customer detail status=%d body=%s", detail.Code, detail.Body.String())
	}

	adjust := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/customer-accounts/"+accountID+"/manual-adjustment", `{
		"currency":"CNY",
		"amount_micros":2000000,
		"reason":"support credit"
	}`, cookie, csrf, "customer_adjust")
	if adjust.Code != http.StatusOK || !strings.Contains(adjust.Body.String(), `"amount_micros":2000000`) {
		t.Fatalf("adjust status=%d body=%s", adjust.Code, adjust.Body.String())
	}

	creditReport := request(t, mux, http.MethodGet, "/api/admin/v1/customer-accounts/"+accountID+"/credit-report?currency=CNY", "", cookie, "")
	if creditReport.Code != http.StatusOK || !strings.Contains(creditReport.Body.String(), `"active_holds"`) || !strings.Contains(creditReport.Body.String(), `"failed_settlements"`) || !strings.Contains(creditReport.Body.String(), `"exports"`) {
		t.Fatalf("credit report status=%d body=%s", creditReport.Code, creditReport.Body.String())
	}
	if strings.Contains(creditReport.Body.String(), "plaintext_key") || strings.Contains(creditReport.Body.String(), "key_hash") || strings.Contains(creditReport.Body.String(), "payment") {
		t.Fatalf("credit report exposed unsafe or cut-scope data: %s", creditReport.Body.String())
	}

	usageExport := request(t, mux, http.MethodGet, "/api/admin/v1/customer-accounts/"+accountID+"/usage/export?currency=CNY", "", cookie, "")
	if usageExport.Code != http.StatusOK || !strings.Contains(usageExport.Body.String(), `"kind":"usage"`) || !strings.Contains(usageExport.Body.String(), `"safe_fields"`) {
		t.Fatalf("usage export status=%d body=%s", usageExport.Code, usageExport.Body.String())
	}
	if strings.Contains(usageExport.Body.String(), "plaintext_key") || strings.Contains(usageExport.Body.String(), "raw_prompt") || strings.Contains(usageExport.Body.String(), "callback_url") {
		t.Fatalf("usage export exposed unsafe data: %s", usageExport.Body.String())
	}

	ledgerExport := request(t, mux, http.MethodGet, "/api/admin/v1/customer-accounts/"+accountID+"/ledger/export?currency=CNY", "", cookie, "")
	if ledgerExport.Code != http.StatusOK || !strings.Contains(ledgerExport.Body.String(), `"kind":"ledger"`) || !strings.Contains(ledgerExport.Body.String(), `"manual_adjustment"`) {
		t.Fatalf("ledger export status=%d body=%s", ledgerExport.Code, ledgerExport.Body.String())
	}
	if strings.Contains(ledgerExport.Body.String(), "key_hash") || strings.Contains(ledgerExport.Body.String(), "subscription") || strings.Contains(ledgerExport.Body.String(), "redemption") {
		t.Fatalf("ledger export exposed unsafe or cut-scope data: %s", ledgerExport.Body.String())
	}

	reset := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/customer-accounts/"+accountID+"/reset-session", `{}`, cookie, csrf, "customer_reset")
	if reset.Code != http.StatusOK || !strings.Contains(reset.Body.String(), `"revoked_sessions":1`) {
		t.Fatalf("reset status=%d body=%s", reset.Code, reset.Body.String())
	}

	disable := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/customer-accounts/"+accountID+"/disable", "", cookie, csrf, "customer_disable")
	if disable.Code != http.StatusOK || !strings.Contains(disable.Body.String(), `"status":"disabled"`) {
		t.Fatalf("disable status=%d body=%s", disable.Code, disable.Body.String())
	}
}

func TestAdminAPIKeyWorkflows(t *testing.T) {
	mux, _ := testMux(t)
	cookie, csrf := loginOperator(t, mux, "admin@example.com", "admin-local")

	create := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/api-keys", `{
		"tenant_id":"tenant_1",
		"project_id":"project_1",
		"name":"ops key",
		"allowed_models":["gpt-public"],
		"ip_allowlist":["203.0.113.10","2001:db8::/32"],
		"expires_at":"2026-06-30T00:00:00Z"
	}`, cookie, csrf, "api_key_create")
	if create.Code != http.StatusOK {
		t.Fatalf("create api key status=%d body=%s", create.Code, create.Body.String())
	}
	if !strings.Contains(create.Body.String(), `"plaintext_key"`) || strings.Contains(create.Body.String(), "key_hash") {
		t.Fatalf("create api key missing plaintext or leaked hash: %s", create.Body.String())
	}
	var created adminapp.APIKeyCreateResponse
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created api key: %v", err)
	}
	if created.APIKey.ID == "" || created.PlaintextKey == "" || created.APIKey.Fingerprint == "" {
		t.Fatalf("created api key = %#v", created)
	}
	if len(created.APIKey.AllowedModels) != 1 || created.APIKey.AllowedModels[0] != "gpt-public" {
		t.Fatalf("created allowed models = %#v", created.APIKey.AllowedModels)
	}
	if len(created.APIKey.IPAllowlist) != 2 || created.APIKey.ExpiresAt == nil {
		t.Fatalf("created api key metadata = %#v", created.APIKey)
	}

	list := request(t, mux, http.MethodGet, "/api/admin/v1/api-keys?tenant_id=tenant_1&project_id=project_1", "", cookie, "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), created.APIKey.ID) {
		t.Fatalf("list api key status=%d body=%s", list.Code, list.Body.String())
	}
	if strings.Contains(list.Body.String(), created.PlaintextKey) || strings.Contains(list.Body.String(), "key_hash") {
		t.Fatalf("list api key leaked secret material: %s", list.Body.String())
	}

	update := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/api-keys/"+created.APIKey.ID+"/update", `{"name":"ops renamed"}`, cookie, csrf, "api_key_update")
	if update.Code != http.StatusOK {
		t.Fatalf("update api key status=%d body=%s", update.Code, update.Body.String())
	}
	var updated adminapp.APIKeyView
	if err := json.Unmarshal(update.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated api key: %v", err)
	}
	if updated.Name != "ops renamed" || len(updated.AllowedModels) != 1 || updated.AllowedModels[0] != "gpt-public" {
		t.Fatalf("updated api key unexpectedly expanded or lost scope: %#v", updated)
	}
	if strings.Contains(update.Body.String(), created.PlaintextKey) || strings.Contains(update.Body.String(), "key_hash") {
		t.Fatalf("update api key leaked secret material: %s", update.Body.String())
	}

	rotate := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/api-keys/"+created.APIKey.ID+"/rotate", `{}`, cookie, csrf, "api_key_rotate")
	if rotate.Code != http.StatusOK {
		t.Fatalf("rotate api key status=%d body=%s", rotate.Code, rotate.Body.String())
	}
	var rotated adminapp.APIKeyRotateResponse
	if err := json.Unmarshal(rotate.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("decode rotated api key: %v", err)
	}
	if rotated.PlaintextKey == "" || rotated.PlaintextKey == created.PlaintextKey || rotated.APIKey.Fingerprint == "" {
		t.Fatalf("rotated api key = %#v", rotated)
	}
	if strings.Contains(rotate.Body.String(), "key_hash") {
		t.Fatalf("rotate api key leaked hash: %s", rotate.Body.String())
	}

	disable := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/api-keys/"+created.APIKey.ID+"/disable", "", cookie, csrf, "api_key_disable")
	if disable.Code != http.StatusOK || !strings.Contains(disable.Body.String(), `"enabled":false`) {
		t.Fatalf("disable api key status=%d body=%s", disable.Code, disable.Body.String())
	}
	if strings.Contains(disable.Body.String(), rotated.PlaintextKey) || strings.Contains(disable.Body.String(), "key_hash") {
		t.Fatalf("disable api key leaked secret material: %s", disable.Body.String())
	}

	enable := mutationRequest(t, mux, http.MethodPost, "/api/admin/v1/api-keys/"+created.APIKey.ID+"/enable", "", cookie, csrf, "api_key_enable")
	if enable.Code != http.StatusOK || !strings.Contains(enable.Body.String(), `"enabled":true`) {
		t.Fatalf("enable api key status=%d body=%s", enable.Code, enable.Body.String())
	}
	if strings.Contains(enable.Body.String(), rotated.PlaintextKey) || strings.Contains(enable.Body.String(), "key_hash") {
		t.Fatalf("enable api key leaked secret material: %s", enable.Body.String())
	}
}

func TestAdminActivityLogs(t *testing.T) {
	mux, _ := testMux(t)
	cookie, _ := loginOperator(t, mux, "admin@example.com", "admin-local")

	usage := request(t, mux, http.MethodGet, "/api/admin/v1/usage-logs?tenant_id=tenant_1&request_id=req_usage_1&model=gpt-public", "", cookie, "")
	if usage.Code != http.StatusOK || !strings.Contains(usage.Body.String(), `"request_id":"req_usage_1"`) || !strings.Contains(usage.Body.String(), `"channel_id":"channel_primary"`) {
		t.Fatalf("usage log status=%d body=%s", usage.Code, usage.Body.String())
	}
	if strings.Contains(usage.Body.String(), "prompt") || strings.Contains(usage.Body.String(), "secret") || strings.Contains(usage.Body.String(), "ratio") {
		t.Fatalf("usage log exposed unsafe or cut-scope field: %s", usage.Body.String())
	}

	usageDetail := request(t, mux, http.MethodGet, "/api/admin/v1/usage-logs/req_usage_1", "", cookie, "")
	if usageDetail.Code != http.StatusOK || !strings.Contains(usageDetail.Body.String(), `"ledger"`) || strings.Contains(usageDetail.Body.String(), "prompt") {
		t.Fatalf("usage detail status=%d body=%s", usageDetail.Code, usageDetail.Body.String())
	}

	tasks := request(t, mux, http.MethodGet, "/api/admin/v1/task-logs?request_id=req_task_1&provider_type=openai&channel_id=channel_primary", "", cookie, "")
	if tasks.Code != http.StatusOK || !strings.Contains(tasks.Body.String(), `"request_id":"req_task_1"`) || !strings.Contains(tasks.Body.String(), `"provider_task_id":"provider_task_1"`) {
		t.Fatalf("task logs status=%d body=%s", tasks.Code, tasks.Body.String())
	}
	var taskList struct {
		Data []adminapp.TaskLogView `json:"data"`
	}
	if err := json.Unmarshal(tasks.Body.Bytes(), &taskList); err != nil {
		t.Fatalf("decode task logs: %v", err)
	}
	if len(taskList.Data) != 1 || taskList.Data[0].TaskID == "" {
		t.Fatalf("task log data = %#v", taskList.Data)
	}

	taskDetail := request(t, mux, http.MethodGet, "/api/admin/v1/task-logs/"+taskList.Data[0].TaskID, "", cookie, "")
	if taskDetail.Code != http.StatusOK || !strings.Contains(taskDetail.Body.String(), `"workflow":"wf_1"`) {
		t.Fatalf("task detail status=%d body=%s", taskDetail.Code, taskDetail.Body.String())
	}
	if strings.Contains(taskDetail.Body.String(), "secret-token") || strings.Contains(taskDetail.Body.String(), "callback_url") || strings.Contains(taskDetail.Body.String(), `"input"`) {
		t.Fatalf("task detail exposed unsafe field: %s", taskDetail.Body.String())
	}
}

func testMux(t *testing.T) (*http.ServeMux, *adminservice.Service) {
	t.Helper()
	ctx := context.Background()
	repo := adminrepo.NewMemoryRepository()
	owner := configadmin.NewService(configadmin.NewMemoryRepository(), configadmin.NewCredentialCodec("test-secret"), nil)
	reportRepo := reporting.NewMemoryRepository()
	reportRepo.SeedUsageRecord(reporting.UsageLogRow{
		RequestID:    "req_usage_1",
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		APIKeyID:     "key_current",
		Model:        "gpt-public",
		ProviderType: "openai",
		ChannelID:    "channel_primary",
		InputTokens:  10,
		OutputTokens: 20,
		TotalTokens:  30,
		AmountMicros: 1200,
		Currency:     "CNY",
	})
	taskRepo := tasksvc.NewMemoryRepository()
	taskService := tasksvc.NewService(taskRepo, 0)
	task, _, err := taskService.CreateMediaTask(ctx, tasksvc.CreateTaskRequest{
		TenantID:    "tenant_1",
		ProjectID:   "project_1",
		APIKeyID:    "key_current",
		RequestID:   "req_task_1",
		Endpoint:    "/v1/images/generations",
		Kind:        tasksvc.KindImageGeneration,
		MediaType:   "image",
		Model:       "gpt-public",
		Input:       []byte(`{"prompt":"do not return"}`),
		CallbackURL: "https://customer.example/callback",
		Metadata:    map[string]string{"workflow": "wf_1", "api_secret": "secret-token"},
	})
	if err != nil {
		t.Fatalf("CreateMediaTask() error = %v", err)
	}
	if _, err := taskService.MarkDispatched(ctx, task.ID, "openai", "channel_primary", "provider_task_1"); err != nil {
		t.Fatalf("MarkDispatched() error = %v", err)
	}
	svc := adminservice.New(
		repo,
		owner,
		adminservice.WithCommercialReporting(reporting.NewService(reportRepo)),
		adminservice.WithTaskRepository(taskRepo),
		adminservice.WithPortalSessionResetter(fakePortalSessionResetter{count: 1}),
	)
	if _, err := svc.EnsureBootstrapOperator(ctx, "admin@example.com", "admin-local", []adminapp.Role{adminapp.RoleSuperAdmin}); err != nil {
		t.Fatalf("EnsureBootstrapOperator() error = %v", err)
	}
	mux := http.NewServeMux()
	NewHandlerWithService(svc, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	return mux, svc
}

func loginOperator(t *testing.T, mux http.Handler, email string, password string) (*http.Cookie, string) {
	t.Helper()
	rec := request(t, mux, http.MethodPost, "/api/admin/v1/auth/login", `{"email":"`+email+`","password":"`+password+`"}`, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode login body: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != sessionCookieName {
		t.Fatalf("missing session cookie: %#v", cookies)
	}
	return cookies[0], body.CSRFToken
}

func request(t *testing.T, mux http.Handler, method string, path string, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if csrf != "" {
		req.Header.Set(csrfHeaderName, csrf)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

type fakePortalSessionResetter struct {
	count int
}

func (f fakePortalSessionResetter) ResetPortalSessions(context.Context, adminservice.PortalSessionResetFilter) (int, error) {
	return f.count, nil
}

func mutationRequest(t *testing.T, mux http.Handler, method string, path string, body string, cookie *http.Cookie, csrf string, seed string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(csrfHeaderName, csrf)
	req.Header.Set("Idempotency-Key", "idem_"+seed)
	req.Header.Set("X-Reason", "test "+seed)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}
